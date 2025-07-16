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
	WebSocketService WebSocketMessenger
	IDGenerator      IDGenerator // New dependency
}

// Execute registers a new display.
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
		// Send set_id message to the display
		setIDPayload, _ := json.Marshal(map[string]string{"id": displayID})
		uc.WebSocketService.SendMessage(displayID, "set_id", setIDPayload)
	}

	// Check for ID conflict (this check is still needed even with unique ID generation
	// in case of external ID provision or race conditions, though less likely now)
	if _, exists := uc.DisplayRepo.FindByID(displayID); exists {
		uc.WebSocketService.SendError(displayID, domain.ErrDisplayIDConflict, fmt.Sprintf("Display ID '%s' is already in use.", displayID))
		return "", fmt.Errorf("display ID conflict: %s", displayID)
	}

	display := &domain.Display{
		ID:          displayID,
		CommandList: commandData,
	}
	uc.DisplayRepo.Save(display)

	log.Printf("Display '%s' connected.", displayID)
	return displayID, nil
}

// ControllerConnectionUseCase handles a new Controller connection.
type ControllerConnectionUseCase struct {
	DisplayRepo      DisplayRepository
	ControllerRepo   ControllerRepository
	WebSocketService WebSocketMessenger
}

// Execute connects a controller to a target display.
func (uc *ControllerConnectionUseCase) Execute(conn *websocket.Conn, targetID string) (string, error) {
	// Generate a temporary controller ID for error reporting before actual assignment
	// This ID is not yet saved to the repository.
	controllerIDCounter++
	tempControllerID := fmt.Sprintf("temp-controller-%d", controllerIDCounter)

	if targetID == "" {
		uc.WebSocketService.SendError(tempControllerID, domain.ErrInvalidQueryParams, "Missing required query parameter: target_id")
		return "", fmt.Errorf("missing target_id")
	}

	display, found := uc.DisplayRepo.FindByID(targetID)
	if !found {
		uc.WebSocketService.SendError(tempControllerID, domain.ErrTargetDisplayNotFound, fmt.Sprintf("Target display '%s' not found or offline.", targetID))
		return "", fmt.Errorf("target display not found: %s", targetID)
	}

	actualDisplay := display.(*domain.Display) // Type assertion

	actualDisplay.Mu.Lock()
	defer actualDisplay.Mu.Unlock()

	if actualDisplay.Controller != nil {
		uc.WebSocketService.SendError(tempControllerID, domain.ErrTargetDisplayAlreadyControlled, fmt.Sprintf("Display '%s' is already controlled by another client.", targetID))
		return "", fmt.Errorf("target display already controlled: %s", targetID)
	}

	controllerIDCounter++
	controllerID := fmt.Sprintf("controller-%d", controllerIDCounter) // Simple ID generation
	controller := &domain.Controller{
		ID:            controllerID,
		TargetDisplay: actualDisplay,
	}
	uc.ControllerRepo.Save(controller)
	actualDisplay.Controller = controller

	log.Printf("Controller '%s' connected to Display '%s'.", controllerID, targetID)

	// Send command list to controller
	commandListMsgPayload := actualDisplay.CommandList
	if err := uc.WebSocketService.SendMessage(controllerID, "command_list", commandListMsgPayload); err != nil {
		log.Printf("Error sending command list to controller '%s': %v", controllerID, err)
		// Clean up on send error
		uc.ControllerRepo.Delete(controllerID)
		actualDisplay.Controller = nil
		return "", fmt.Errorf("failed to send command list: %w", err)
	}

	return controllerID, nil
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
