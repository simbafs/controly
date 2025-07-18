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
	var err error
	// If displayID is empty, generate a new unique ID
	if displayID == "" {
		displayID, err = uc.IDGenerator.GenerateUniqueDisplayID(uc.DisplayRepo)
		if err != nil {
			return "", fmt.Errorf("failed to generate unique display ID: %w", err)
		}
	}

	// Check for ID conflict
	if _, exists := uc.DisplayRepo.FindByID(displayID); exists {
		// Use the low-level SendErrorToConn since the connection is not yet registered.
		sendErrorToConn(conn, domain.ErrDisplayIDConflict, fmt.Sprintf("Display ID '%s' is already in use.", displayID))
		return "", fmt.Errorf("display ID conflict: %s", displayID)
	}

	if commandURL == "" {
		// Use the low-level SendErrorToConn as we might not have a displayID to associate with.
		sendErrorToConn(conn, domain.ErrInvalidQueryParams, "Missing required query parameter: command_url")
		return "", fmt.Errorf("missing command_url")
	}

	commandData, err := uc.CommandFetcher.FetchCommands(commandURL)
	if err != nil {
		sendErrorToConn(conn, domain.ErrCommandURLUnreachable, fmt.Sprintf("Failed to fetch command URL: %v", err))
		return "", fmt.Errorf("command URL unreachable: %w", err)
	}

	if !json.Valid(commandData) {
		sendErrorToConn(conn, domain.ErrInvalidCommandJSON, "Invalid command JSON format.")
		return "", fmt.Errorf("invalid command JSON format")
	}

	display := domain.NewDisplay(displayID, commandData)
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

	// Find the display to get its subscribers
	displayIface, found := uc.DisplayRepo.FindByID(displayID)
	if !found {
		// Already cleaned up or never existed.
		return
	}

	actualDisplay := displayIface.(*domain.Display)

	// Lock the display, copy its subscribers, and then unlock immediately
	// to avoid holding the lock while performing other operations.
	actualDisplay.Mu.Lock()
	subscribersToNotify := make([]string, 0, len(actualDisplay.Subscribers))
	for controllerID := range actualDisplay.Subscribers {
		subscribersToNotify = append(subscribersToNotify, controllerID)
	}
	actualDisplay.Subscribers = make(map[string]bool) // Clear subscribers
	actualDisplay.Mu.Unlock()

	// Now, notify subscribers and update their subscriptions without holding the display lock.
	for _, controllerID := range subscribersToNotify {
		// uc.ConnManager.SendError(controllerID, domain.ErrTargetDisplayNotFound, fmt.Sprintf("Display '%s' disconnected.", displayID))
		if controller, controllerFound := uc.ControllerRepo.FindByID(controllerID); controllerFound {
			controller.Mu.Lock()
			delete(controller.Subscriptions, displayID)
			controller.Mu.Unlock()
		}
	}

	// Finally, delete the display from the repository.
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
func (uc *ControllerConnectionUseCase) Execute(conn *websocket.Conn) (string, error) {
	var controllerID string

	controllerIDCounter++
	controllerID = fmt.Sprintf("controller-%d", controllerIDCounter)

	// Check for ID conflict
	if _, exists := uc.ControllerRepo.FindByID(controllerID); exists {
		uc.WebSocketService.SendError(controllerID, domain.ErrControllerIDConflict, fmt.Sprintf("Controller ID '%s' is already in use.", controllerID))
		return "", fmt.Errorf("controller ID conflict: %s", controllerID)
	}

	controller := domain.NewController(controllerID)
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
	if !found {
		// Already cleaned up or never existed.
		return
	}

	// Lock the controller, copy its subscriptions, and then unlock immediately
	// to avoid holding the lock while performing other operations.
	controller.Mu.Lock()
	subscriptionsToUpdate := make([]string, 0, len(controller.Subscriptions))
	for displayID := range controller.Subscriptions {
		subscriptionsToUpdate = append(subscriptionsToUpdate, displayID)
	}
	controller.Mu.Unlock()

	// Now, update the subscribed displays without holding the controller lock.
	for _, displayID := range subscriptionsToUpdate {
		if displayIface, displayFound := uc.DisplayRepo.FindByID(displayID); displayFound {
			actualDisplay := displayIface.(*domain.Display)
			actualDisplay.RemoveSubscriber(controllerID)

			// Notify the display that a controller has unsubscribed
			unsubscribedCount := len(actualDisplay.Subscribers)
			unsubscribedPayload, _ := json.Marshal(domain.UnsubscribedPayload{Count: unsubscribedCount})
			if err := uc.ConnManager.SendMessage(displayID, "server", "unsubscribed", unsubscribedPayload); err != nil {
				log.Printf("Error sending 'unsubscribed' message to display '%s': %v", displayID, err)
			}
		}
	}

	// Finally, delete the controller from the repository.
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

	var msg domain.IncomingMessage
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
			// When forwarding, `from` is the display, `to` is the controller.
			if err := uc.WebSocketService.SendMessage(controllerID, displayID, "status", msg.Payload); err != nil {
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

	var msg domain.IncomingMessage
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

			// Send command list to controller. `from` is the display, `to` is the controller.
			if err := uc.WebSocketService.SendMessage(controllerID, displayID, "command_list", actualDisplay.CommandList); err != nil {
				log.Printf("Error sending command_list to controller '%s' for display '%s': %v", controllerID, displayID, err)
				// Continue to next display even if one fails
			}

			// Notify the display that a new controller has subscribed
			subscribedCount := len(actualDisplay.Subscribers)
			subscribedPayload, _ := json.Marshal(domain.SubscribedPayload{Count: subscribedCount})
			if err := uc.WebSocketService.SendMessage(displayID, "server", "subscribed", subscribedPayload); err != nil {
				log.Printf("Error sending 'subscribed' message to display '%s': %v", displayID, err)
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

				// Notify the display that a controller has unsubscribed
				unsubscribedCount := len(actualDisplay.Subscribers)
				unsubscribedPayload, _ := json.Marshal(domain.UnsubscribedPayload{Count: unsubscribedCount})
				if err := uc.WebSocketService.SendMessage(displayID, "server", "unsubscribed", unsubscribedPayload); err != nil {
					log.Printf("Error sending 'unsubscribed' message to display '%s': %v", displayID, err)
				}
			}
			delete(controller.Subscriptions, displayID) // Remove display from controller's subscriptions
			uc.ControllerRepo.Save(controller)          // Persist controller changes
		}
		log.Printf("Controller '%s' unsubscribed from displays: %v", controllerID, payload.DisplayIDs)

	case "command":
		// Command messages must have a `to` field specifying the target display.
		if msg.To == "" {
			uc.WebSocketService.SendError(controllerID, domain.ErrInvalidMessageFormat, "Command message missing target 'to' field.")
			return fmt.Errorf("command message from controller '%s' missing target 'to' field", controllerID)
		}

		controller.Mu.Lock()
		_, isSubscribed := controller.Subscriptions[msg.To]
		controller.Mu.Unlock()

		if !isSubscribed {
			uc.WebSocketService.SendError(controllerID, domain.ErrNotSubscribedToDisplay, fmt.Sprintf("Not subscribed to display '%s'.", msg.To))
			return fmt.Errorf("controller '%s' not subscribed to display '%s'", controllerID, msg.To)
		}

		// Forward command to the target display. `from` is the controller, `to` is the display.
		if err := uc.WebSocketService.SendMessage(msg.To, controllerID, "command", msg.Payload); err != nil {
			log.Printf("Error forwarding command to display '%s' from controller '%s': %v", msg.To, controllerID, err)
			return fmt.Errorf("failed to forward command: %w", err)
		}
		log.Printf("Controller '%s' sent command to display '%s'.", controllerID, msg.To)

	default:
		uc.WebSocketService.SendError(controllerID, domain.ErrInvalidMessageFormat, fmt.Sprintf("Unknown message type: %s", msg.Type))
		return fmt.Errorf("unknown message type from controller '%s': %s", controllerID, msg.Type)
	}
	return nil
}
