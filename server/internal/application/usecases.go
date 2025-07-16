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
	if actualDisplay.Controller != nil {
		controllerID := actualDisplay.Controller.ID
		// Notify controller, unregister it, and delete it from repo
		uc.ConnManager.SendError(controllerID, domain.ErrTargetDisplayNotFound, "Target display disconnected.")
		uc.ConnManager.UnregisterControllerConnection(controllerID)
		uc.ControllerRepo.Delete(controllerID)
		actualDisplay.Controller = nil
	}
	actualDisplay.Mu.Unlock()

	uc.DisplayRepo.Delete(displayID)
	log.Printf("Removed display '%s' from repository.", displayID)
}

// ControllerConnectionUseCase handles a new Controller connection.
type ControllerConnectionUseCase struct {
	DisplayRepo      DisplayRepository
	ControllerRepo   ControllerRepository
	WebSocketService WebSocketConnectionManager
}

// Execute connects a controller to a target display and registers its connection.
func (uc *ControllerConnectionUseCase) Execute(conn *websocket.Conn, targetID string) (string, *domain.Display, error) {
	// Generate a temporary controller ID for error reporting before actual assignment
	controllerIDCounter++
	tempControllerID := fmt.Sprintf("temp-controller-%d", controllerIDCounter)

	if targetID == "" {
		uc.WebSocketService.SendError(tempControllerID, domain.ErrInvalidQueryParams, "Missing required query parameter: target_id")
		return "", nil, fmt.Errorf("missing target_id")
	}

	display, found := uc.DisplayRepo.FindByID(targetID)
	if !found {
		uc.WebSocketService.SendError(tempControllerID, domain.ErrTargetDisplayNotFound, fmt.Sprintf("Target display '%s' not found or offline.", targetID))
		return "", nil, fmt.Errorf("target display not found: %s", targetID)
	}

	actualDisplay := display.(*domain.Display) // Type assertion

	actualDisplay.Mu.Lock()
	defer actualDisplay.Mu.Unlock()

	if actualDisplay.Controller != nil {
		uc.WebSocketService.SendError(tempControllerID, domain.ErrTargetDisplayAlreadyControlled, fmt.Sprintf("Display '%s' is already controlled by another client.", targetID))
		return "", nil, fmt.Errorf("target display already controlled: %s", targetID)
	}

	controllerIDCounter++
	controllerID := fmt.Sprintf("controller-%d", controllerIDCounter) // Simple ID generation
	controller := &domain.Controller{
		ID:            controllerID,
		TargetDisplay: actualDisplay,
	}
	uc.ControllerRepo.Save(controller)
	actualDisplay.Controller = controller
	uc.WebSocketService.RegisterControllerConnection(controllerID, conn)

	log.Printf("Controller '%s' connected to Display '%s'.", controllerID, targetID)

	return controllerID, actualDisplay, nil
}

// ControllerDisconnectionUseCase handles the disconnection of a Controller.
type ControllerDisconnectionUseCase struct {
	ControllerRepo ControllerRepository
	ConnManager    WebSocketConnectionManager
}

// Execute performs all cleanup tasks when a controller disconnects.
func (uc *ControllerDisconnectionUseCase) Execute(controllerID string) {
	// Unregister connection first
	uc.ConnManager.UnregisterControllerConnection(controllerID)

	// Find the controller to get the target display
	controller, found := uc.ControllerRepo.FindByID(controllerID)
	if found && controller.TargetDisplay != nil {
		// Release the display from being controlled
		controller.TargetDisplay.Mu.Lock()
		if controller.TargetDisplay.Controller == controller {
			controller.TargetDisplay.Controller = nil
		}
		controller.TargetDisplay.Mu.Unlock()
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
	if actualDisplay.Controller != nil {
		controllerID := actualDisplay.Controller.ID
		// Notify controller, unregister it, and delete it from repo
		uc.ConnManager.SendError(controllerID, domain.ErrTargetDisplayNotFound, "Target display disconnected.") // Or a more specific error for deletion
		uc.ConnManager.UnregisterControllerConnection(controllerID)
		uc.ControllerRepo.Delete(controllerID)
		actualDisplay.Controller = nil
	}
	actualDisplay.Mu.Unlock()

	uc.ConnManager.UnregisterDisplayConnection(displayID)
	uc.DisplayRepo.Delete(displayID)
	log.Printf("Deleted display '%s' from repository.", displayID)
	return nil
}

// DeleteControllerUseCase handles the deletion of a Controller.
type DeleteControllerUseCase struct {
	ControllerRepo ControllerRepository
	ConnManager    WebSocketConnectionManager
}

// Execute deletes a controller and unregisters its connection.
func (uc *DeleteControllerUseCase) Execute(controllerID string) error {
	controller, found := uc.ControllerRepo.FindByID(controllerID)
	if !found {
		return fmt.Errorf("controller '%s' not found", controllerID)
	}
	// If the controller was controlling a display, release it
	if controller.TargetDisplay != nil {
		controller.TargetDisplay.Mu.Lock()
		if controller.TargetDisplay.Controller == controller {
			controller.TargetDisplay.Controller = nil
		}
		controller.TargetDisplay.Mu.Unlock()
	}
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
		if actualDisplay.Controller != nil {
			// Forward status to the controlling controller
			if err := uc.WebSocketService.SendMessage(actualDisplay.Controller.ID, "status", msg.Payload); err != nil {
				log.Printf("Error forwarding status to controller for display '%s': %v", displayID, err)
				return fmt.Errorf("failed to forward status: %w", err)
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
		// This error should ideally be sent back to the controller
		return fmt.Errorf("invalid message format from controller '%s': %w", controllerID, err)
	}

	if msg.Type == "command" {
		controller.TargetDisplay.Mu.Lock()
		defer controller.TargetDisplay.Mu.Unlock()

		if controller.TargetDisplay.Controller == controller { // Only forward if this controller is still active
			// Forward command to the target display
			if err := uc.WebSocketService.SendMessage(controller.TargetDisplay.ID, "command", msg.Payload); err != nil {
				log.Printf("Error forwarding command to display '%s': %v", controller.TargetDisplay.ID, err)
				return fmt.Errorf("failed to forward command: %w", err)
			}
		} else {
			// This case should ideally be handled by the interface layer sending an error back to the controller
			return fmt.Errorf("controller '%s' no longer controls display '%s'", controllerID, controller.TargetDisplay.ID)
		}
	} else {
		return fmt.Errorf("unknown message type from controller '%s': %s", controllerID, msg.Type)
	}
	return nil
}
