package application

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/gorilla/websocket"
	"github.com/simbafs/controly/server/internal/domain"
	"github.com/simbafs/controly/server/internal/infrastructure"
)

// Global counter for simple controller ID generation (temporary, for build fix)
var controllerIDCounter int

// IDGenerator defines the interface for generating unique IDs.
type IDGenerator interface {
	GenerateUniqueDisplayID(checker infrastructure.DisplayExistenceChecker) (string, error)
}

// DisplayRegistrationUseCase handles the registration of a new Display.
type DisplayRegistrationUseCase struct {
	DisplayRepo      DisplayRepository
	CommandFetcher   CommandFetcher
	WebSocketService WebSocketConnectionManager
	IDGenerator      IDGenerator // New dependency
}

// Execute registers a new display and its connection.
func (uc *DisplayRegistrationUseCase) Execute(conn *websocket.Conn, displayID, commandURL string) (string, error) {
	if commandURL == "" {
		// We don't have a displayID yet, so we can't use it for SendError. Send to a placeholder.
		uc.WebSocketService.SendError("", domain.ErrInvalidQueryParams, "Missing required query parameter: command_url")
		return "", fmt.Errorf("missing command_url")
	}

	commandData, err := uc.CommandFetcher.FetchCommands(commandURL)
	if err != nil {
		uc.WebSocketService.SendError(displayID, domain.ErrCommandURLUnreachable, fmt.Sprintf("Failed to fetch command URL: %v", err))
		return "", fmt.Errorf("command URL unreachable: %w", err)
	}

	if !json.Valid(commandData) {
		uc.WebSocketService.SendError(displayID, domain.ErrInvalidCommandJSON, "Invalid command JSON format.")
		return "", fmt.Errorf("invalid command JSON format")
	}

	// If displayID is empty, generate a new unique ID
	if displayID == "" {
		displayID, err = uc.IDGenerator.GenerateUniqueDisplayID(uc.DisplayRepo)
		if err != nil {
			return "", fmt.Errorf("failed to generate unique display ID: %w", err)
		}
	}

	// Check for ID conflict
	if _, exists := uc.DisplayRepo.FindByID(displayID); exists {
		uc.WebSocketService.SendError(displayID, domain.ErrDisplayIDConflict, fmt.Sprintf("Display ID '%s' is already in use.", displayID))
		return "", fmt.Errorf("display ID conflict: %s", displayID)
	}

	display := &domain.Display{
		ID:          displayID,
		CommandList: commandData,
	}
	uc.DisplayRepo.Save(display)
	uc.WebSocketService.RegisterDisplayConnection(display.ID, conn)

	log.Printf("Display '%s' connected and registered.", displayID)
	return displayID, nil
}

// DisplayDisconnectionUseCase handles the disconnection of a Display.
type DisplayDisconnectionUseCase struct {
	DisplayRepo    DisplayRepository
	ControllerRepo ControllerRepository
	ConnManager    WebSocketConnectionManager
}

// Execute performs all cleanup tasks when a display disconnects.
func (uc *DisplayDisconnectionUseCase) Execute(displayID string) {
	// Unregister connection first
	uc.ConnManager.UnregisterDisplayConnection(displayID)

	// Clean up repositories
	displayIface, found := uc.DisplayRepo.FindByID(displayID)
	if !found {
		// Already cleaned up, or never existed.
		return
	}

	actualDisplay := displayIface.(*domain.Display)
	actualDisplay.Mu.Lock()
	// Notify all subscribers and remove this display from their subscriptions
	for controllerID := range actualDisplay.Subscribers {
		uc.ConnManager.SendError(controllerID, domain.ErrTargetDisplayNotFound, fmt.Sprintf("Display '%s' disconnected.", displayID))
		controller, controllerFound := uc.ControllerRepo.FindByID(controllerID)
		if controllerFound {
			controller.Mu.Lock()
			delete(controller.Subscriptions, displayID)
			controller.Mu.Unlock()
		}
		// No need to unregister controller connection or delete controller here,
		// as the controller might be subscribed to other displays.
	}
	actualDisplay.Subscribers = make(map[string]bool) // Clear subscribers
	actualDisplay.Mu.Unlock()

	uc.DisplayRepo.Delete(displayID)
	log.Printf("Removed display '%s' from repository.", displayID)
}

// ControllerConnectionUseCase handles a new Controller connection.
type ControllerConnectionUseCase struct {
	ControllerRepo   ControllerRepository
	WebSocketService WebSocketConnectionManager
	IDGenerator      IDGenerator // New dependency
}

// Execute registers a new controller and its connection.
func (uc *ControllerConnectionUseCase) Execute(conn *websocket.Conn, controllerIDParam string) (string, error) {
	var controllerID string

	// If controllerIDParam is empty, generate a new unique ID
	if controllerIDParam == "" {
		// For controllers, we don't have a specific checker like for displays.
		// We'll just generate and then check against the ControllerRepo.
		// This might need a more robust ID generation for controllers if conflicts are frequent.
		// For now, let's use a simple counter or a similar Base58 generator if we adapt it.
		// Let's use the Base58IDGenerator for controllers too, but it needs a ControllerExistenceChecker.
		// For now, I'll use a simple counter for controller IDs as in the original code,
		// but ensure it's unique by checking the repo.
		controllerIDCounter++
		controllerID = fmt.Sprintf("controller-%d", controllerIDCounter)
		// In a real scenario, this should use a robust ID generator and check for uniqueness.
		// For now, assuming simple counter is unique enough for temporary IDs.
	} else {
		controllerID = controllerIDParam
	}

	// Check for ID conflict
	if _, exists := uc.ControllerRepo.FindByID(controllerID); exists {
		uc.WebSocketService.SendError(controllerID, domain.ErrControllerIDConflict, fmt.Sprintf("Controller ID '%s' is already in use.", controllerID))
		return "", fmt.Errorf("controller ID conflict: %s", controllerID)
	}

	controller := &domain.Controller{
		ID:            controllerID,
		Subscriptions: make(map[string]bool), // Initialize the map
	}
	uc.ControllerRepo.Save(controller)
	uc.WebSocketService.RegisterControllerConnection(controllerID, conn)

	log.Printf("Controller '%s' connected and registered.", controllerID)

	return controllerID, nil
}

// ControllerDisconnectionUseCase handles the disconnection of a Controller.
type ControllerDisconnectionUseCase struct {
	ControllerRepo ControllerRepository
	DisplayRepo    DisplayRepository // Add DisplayRepo dependency
	ConnManager    WebSocketConnectionManager
}

// Execute performs all cleanup tasks when a controller disconnects.
func (uc *ControllerDisconnectionUseCase) Execute(controllerID string) {
	// Unregister connection first
	uc.ConnManager.UnregisterControllerConnection(controllerID)

	// Find the controller to get its subscriptions
	controller, found := uc.ControllerRepo.FindByID(controllerID)
	if found {
		controller.Mu.Lock()
		for displayID := range controller.Subscriptions {
			displayIface, displayFound := uc.DisplayRepo.FindByID(displayID)
			if displayFound {
				actualDisplay := displayIface.(*domain.Display)
				actualDisplay.Mu.Lock()
				delete(actualDisplay.Subscribers, controllerID)
				actualDisplay.Mu.Unlock()
			}
		}
		controller.Mu.Unlock()
	}
	// Delete the controller from the repository
	uc.ControllerRepo.Delete(controllerID)
	log.Printf("Removed controller '%s' from repository.", controllerID)
}

// DeleteDisplayUseCase handles the deletion of a Display.
type DeleteDisplayUseCase struct {
	DisplayRepo    DisplayRepository
	ControllerRepo ControllerRepository // New dependency
	ConnManager    WebSocketConnectionManager
}

// Execute deletes a display and unregisters its connection.
func (uc *DeleteDisplayUseCase) Execute(displayID string) error {
	displayIface, found := uc.DisplayRepo.FindByID(displayID)
	if !found {
		return fmt.Errorf("display '%s' not found", displayID)
	}

	actualDisplay := displayIface.(*domain.Display)
	actualDisplay.Mu.Lock()
	// Notify all subscribers and remove this display from their subscriptions
	for controllerID := range actualDisplay.Subscribers {
		uc.ConnManager.SendError(controllerID, domain.ErrTargetDisplayNotFound, fmt.Sprintf("Display '%s' deleted.", displayID)) // More specific error
		controller, controllerFound := uc.ControllerRepo.FindByID(controllerID)
		if controllerFound {
			controller.Mu.Lock()
			delete(controller.Subscriptions, displayID)
			controller.Mu.Unlock()
		}
	}
	actualDisplay.Subscribers = make(map[string]bool) // Clear subscribers
	actualDisplay.Mu.Unlock()

	uc.ConnManager.UnregisterDisplayConnection(displayID)
	uc.DisplayRepo.Delete(displayID)
	log.Printf("Deleted display '%s' from repository.", displayID)
	return nil
}

// DeleteControllerUseCase handles the deletion of a Controller.
type DeleteControllerUseCase struct {
	ControllerRepo ControllerRepository
	DisplayRepo    DisplayRepository // Add DisplayRepo dependency
	ConnManager    WebSocketConnectionManager
}

// Execute deletes a controller and unregisters its connection.
func (uc *DeleteControllerUseCase) Execute(controllerID string) error {
	controller, found := uc.ControllerRepo.FindByID(controllerID)
	if !found {
		return fmt.Errorf("controller '%s' not found", controllerID)
	}

	controller.Mu.Lock()
	for displayID := range controller.Subscriptions {
		displayIface, displayFound := uc.DisplayRepo.FindByID(displayID)
		if displayFound {
			actualDisplay := displayIface.(*domain.Display)
			actualDisplay.Mu.Lock()
			delete(actualDisplay.Subscribers, controllerID)
			actualDisplay.Mu.Unlock()
		}
	}
	controller.Subscriptions = make(map[string]bool) // Clear subscriptions
	controller.Mu.Unlock()

	uc.ConnManager.UnregisterControllerConnection(controllerID)
	uc.ControllerRepo.Delete(controllerID)
	log.Printf("Deleted controller '%s' from repository.", controllerID)
	return nil
}

// DisplayMessageHandlingUseCase handles messages received from a Display.
type DisplayMessageHandlingUseCase struct {
	DisplayRepo      DisplayRepository
	WebSocketService WebSocketMessenger
}

// Execute handles an incoming message from a display.
func (uc *DisplayMessageHandlingUseCase) Execute(displayID string, message []byte) error {
	displayIface, found := uc.DisplayRepo.FindByID(displayID)
	if !found {
		return fmt.Errorf("display '%s' not found for message handling", displayID)
	}

	actualDisplay := displayIface.(*domain.Display) // Type assertion

	var msg domain.WebSocketMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		// This error should ideally be sent back to the display, but we don't have the conn here.
		// The interface layer will handle sending errors back to the client.
		return fmt.Errorf("invalid message format from display '%s': %w", displayID, err)
	}

	if msg.Type == "status" {
		actualDisplay.Mu.Lock()
		defer actualDisplay.Mu.Unlock()
		// Forward status to all subscribing controllers
		for controllerID := range actualDisplay.Subscribers {
			if err := uc.WebSocketService.SendMessage(controllerID, "status", msg.Payload); err != nil {
				log.Printf("Error forwarding status to controller '%s' for display '%s': %v", controllerID, displayID, err)
				// Continue to next subscriber even if one fails
			}
		}
	} else {
		return fmt.Errorf("unknown message type from display '%s': %s", displayID, msg.Type)
	}
	return nil
}

// ControllerMessageHandlingUseCase handles messages received from a Controller.
type ControllerMessageHandlingUseCase struct {
	ControllerRepo   ControllerRepository
	DisplayRepo      DisplayRepository
	WebSocketService WebSocketMessenger
}

// Execute handles an incoming message from a controller.
func (uc *ControllerMessageHandlingUseCase) Execute(controllerID string, message []byte) error {
	controller, found := uc.ControllerRepo.FindByID(controllerID)
	if !found {
		return fmt.Errorf("controller '%s' not found for message handling", controllerID)
	}

	var msg domain.WebSocketMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		uc.WebSocketService.SendError(controllerID, domain.ErrInvalidMessageFormat, "Invalid message format.")
		return fmt.Errorf("invalid message format from controller '%s': %w", controllerID, err)
	}

	switch msg.Type {
	case "subscribe":
		var payload struct {
			DisplayIDs []string `json:"display_ids"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			uc.WebSocketService.SendError(controllerID, domain.ErrInvalidMessageFormat, "Invalid subscribe payload format.")
			return fmt.Errorf("invalid subscribe payload format from controller '%s': %w", controllerID, err)
		}

		controller.Mu.Lock()
		defer controller.Mu.Unlock()

		for _, displayID := range payload.DisplayIDs {
			displayIface, displayFound := uc.DisplayRepo.FindByID(displayID)
			if !displayFound {
				uc.WebSocketService.SendError(controllerID, domain.ErrTargetDisplayNotFound, fmt.Sprintf("Display '%s' not found for subscription.", displayID))
				continue // Continue to next display even if one fails
			}
			actualDisplay := displayIface.(*domain.Display)

			actualDisplay.Mu.Lock()
			actualDisplay.Subscribers[controllerID] = true // Add controller to display's subscribers
			actualDisplay.Mu.Unlock()

			controller.Subscriptions[displayID] = true // Add display to controller's subscriptions
			uc.DisplayRepo.Save(actualDisplay)         // Persist display changes
			uc.ControllerRepo.Save(controller)         // Persist controller changes

			// Send command list to controller for the newly subscribed display
			if err := uc.WebSocketService.SendMessage(controllerID, "command_list", actualDisplay.CommandList); err != nil {
				log.Printf("Error sending command_list to controller '%s' for display '%s': %v", controllerID, displayID, err)
				// Continue to next display even if one fails
			}
		}
		log.Printf("Controller '%s' subscribed to displays: %v", controllerID, payload.DisplayIDs)

	case "unsubscribe":
		var payload struct {
			DisplayIDs []string `json:"display_ids"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			uc.WebSocketService.SendError(controllerID, domain.ErrInvalidMessageFormat, "Invalid unsubscribe payload format.")
			return fmt.Errorf("invalid unsubscribe payload format from controller '%s': %w", controllerID, err)
		}

		controller.Mu.Lock()
		defer controller.Mu.Unlock()

		for _, displayID := range payload.DisplayIDs {
			displayIface, displayFound := uc.DisplayRepo.FindByID(displayID)
			if displayFound {
				actualDisplay := displayIface.(*domain.Display)
				actualDisplay.Mu.Lock()
				delete(actualDisplay.Subscribers, controllerID) // Remove controller from display's subscribers
				actualDisplay.Mu.Unlock()
				uc.DisplayRepo.Save(actualDisplay) // Persist display changes
			}
			delete(controller.Subscriptions, displayID) // Remove display from controller's subscriptions
			uc.ControllerRepo.Save(controller)          // Persist controller changes
		}
		log.Printf("Controller '%s' unsubscribed from displays: %v", controllerID, payload.DisplayIDs)

	case "command":
		// Command messages must have a display_id at the top level
		if msg.DisplayID == "" {
			uc.WebSocketService.SendError(controllerID, domain.ErrInvalidMessageFormat, "Command message missing target display_id.")
			return fmt.Errorf("command message from controller '%s' missing target display_id", controllerID)
		}

		controller.Mu.Lock()
		_, isSubscribed := controller.Subscriptions[msg.DisplayID]
		controller.Mu.Unlock()

		if !isSubscribed {
			uc.WebSocketService.SendError(controllerID, domain.ErrNotSubscribedToDisplay, fmt.Sprintf("Not subscribed to display '%s'.", msg.DisplayID))
			return fmt.Errorf("controller '%s' not subscribed to display '%s'", controllerID, msg.DisplayID)
		}

		// Forward command to the target display
		if err := uc.WebSocketService.SendMessage(msg.DisplayID, "command", msg.Payload); err != nil {
			log.Printf("Error forwarding command to display '%s' from controller '%s': %v", msg.DisplayID, controllerID, err)
			return fmt.Errorf("failed to forward command: %w", err)
		}
		log.Printf("Controller '%s' sent command to display '%s'.", controllerID, msg.DisplayID)

	default:
		uc.WebSocketService.SendError(controllerID, domain.ErrInvalidMessageFormat, fmt.Sprintf("Unknown message type: %s", msg.Type))
		return fmt.Errorf("unknown message type from controller '%s': %s", controllerID, msg.Type)
	}
	return nil
}
