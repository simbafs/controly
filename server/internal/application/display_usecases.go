package application

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/gorilla/websocket"
	"github.com/simbafs/controly/server/internal/domain"
)

// RegisterDisplay handles the registration of a new Display.
type RegisterDisplay struct {
	DisplayRepo      DisplayRepository
	CommandFetcher   CommandFetcher
	WebSocketService interface {
		RegisterDisplayConnection(displayID string, conn any)
	}
	IDGenerator IDGenerator // New dependency
	ServerToken string      // Server-wide token for authentication
}

// Execute registers a new display and its connection.
func (uc *RegisterDisplay) Execute(conn *websocket.Conn, displayID, commandURL, token string) (string, error) {
	// Token validation
	if uc.ServerToken != "" {
		if token == "" {
			sendErrorToConn(conn, domain.ErrAuthenticationFailed, "Token required.")
			return "", fmt.Errorf("token required")
		}
		if uc.ServerToken != token {
			sendErrorToConn(conn, domain.ErrAuthenticationFailed, "Invalid token.")
			return "", fmt.Errorf("invalid token")
		}
	}
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

// HandleDisplayConnection handles logic after a display is registered.
type HandleDisplayConnection struct {
	ControllerRepo   ControllerRepository
	DisplayRepo      DisplayRepository
	WebSocketService ClientNotifier
}

// Execute checks for waiting controllers and establishes subscriptions.
func (uc *HandleDisplayConnection) Execute(displayID string) {
	displayIface, ok := uc.DisplayRepo.FindByID(displayID)
	if !ok {
		return // Should not happen if called right after registration
	}
	display := displayIface.(*domain.Display)

	waitingControllers := uc.ControllerRepo.GetControllersWaitingFor(displayID)

	for _, controller := range waitingControllers {
		controller.Mu.Lock()
		// Check if still waiting, might have changed
		if !controller.WaitingFor[displayID] {
			controller.Mu.Unlock()
			continue
		}

		// Establish subscription
		delete(controller.WaitingFor, displayID)
		controller.Subscriptions[displayID] = true
		display.Subscribers[controller.ID] = true

		// Get updated waiting list to send
		newWaitingList := make([]string, 0, len(controller.WaitingFor))
		for id := range controller.WaitingFor {
			newWaitingList = append(newWaitingList, id)
		}
		controller.Mu.Unlock()

		// Persist changes
		uc.ControllerRepo.Save(controller)

		// Notify controller
		uc.WebSocketService.SendMessage(controller.ID, displayID, "command_list", display.CommandList)
		uc.WebSocketService.SendJSON(controller.ID, "server", "waiting", newWaitingList)

		log.Printf("Automatically subscribed waiting controller '%s' to new display '%s'", controller.ID, displayID)
	}
	// Persist display changes after all controllers have been processed
	uc.DisplayRepo.Save(display)
}

// HandleDisplayDisconnection handles the disconnection of a Display.
type HandleDisplayDisconnection struct {
	DisplayRepo    DisplayRepository
	ControllerRepo ControllerRepository
	ConnManager    interface {
		UnregisterDisplayConnection(displayID string)
		SendMessage(to, from, msgType string, payload json.RawMessage)
		SendJSON(to, from, msgType string, payload any)
	}
}

// Execute performs all cleanup tasks when a display disconnects.
func (uc *HandleDisplayDisconnection) Execute(displayID string) {
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
		if controller, controllerFound := uc.ControllerRepo.FindByID(controllerID); controllerFound {
			controller.Mu.Lock()
			delete(controller.Subscriptions, displayID)
			controller.WaitingFor[displayID] = true
			// Create a slice of the waiting list to send
			waitingList := make([]string, 0, len(controller.WaitingFor))
			for id := range controller.WaitingFor {
				waitingList = append(waitingList, id)
			}
			controller.Mu.Unlock()

			// Notify controller that the display has disconnected
			uc.ConnManager.SendJSON(controllerID, "server", "display_disconnected", domain.DisplayDisconnectedPayload{DisplayID: displayID})

			// Send the updated waiting list
			uc.ConnManager.SendJSON(controllerID, "server", "waiting", waitingList)
		}
	}

	// Finally, delete the display from the repository.
	uc.DisplayRepo.Delete(displayID)
	log.Printf("Removed display '%s' from repository.", displayID)
}

// DeleteDisplay handles the deletion of a Display.
type DeleteDisplay struct {
	DisplayRepo    DisplayRepository
	ControllerRepo ControllerRepository // New dependency
	ConnManager    interface {
		UnregisterDisplayConnection(displayID string)
		SendError(clientID string, code int, message string)
	}
}

// Execute deletes a display and unregisters its connection.
func (uc *DeleteDisplay) Execute(displayID string) error {
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

// ProcessDisplayMessage handles messages received from a Display.
type ProcessDisplayMessage struct {
	DisplayRepo      DisplayRepository
	WebSocketService interface {
		BroadcastMessage(targets []string, from, msgType string, payload json.RawMessage)
	}
}

// Execute handles an incoming message from a display.
func (uc *ProcessDisplayMessage) Execute(displayID string, message []byte) error {
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
		subscribers := make([]string, 0, len(actualDisplay.Subscribers))
		for controllerID := range actualDisplay.Subscribers {
			subscribers = append(subscribers, controllerID)
		}
		actualDisplay.Mu.Unlock()

		if len(subscribers) > 0 {
			// Forward status to all subscribing controllers in a single broadcast
			uc.WebSocketService.BroadcastMessage(subscribers, displayID, "status", msg.Payload)
		}
	} else {
		return fmt.Errorf("unknown message type from display '%s': %s", displayID, msg.Type)
	}
	return nil
}
