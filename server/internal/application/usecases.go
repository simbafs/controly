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

		payload, err := json.Marshal(newWaitingList)
		if err != nil {
			log.Printf("Error marshalling waiting list for controller '%s': %v", controller.ID, err)
			continue
		}
		uc.WebSocketService.SendMessage(controller.ID, "server", "waiting", payload)

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
			payload, _ := json.Marshal(domain.DisplayDisconnectedPayload{DisplayID: displayID})
			uc.ConnManager.SendMessage(controllerID, "server", "display_disconnected", payload)

			// Send the updated waiting list
			payload, err := json.Marshal(waitingList)
			if err != nil {
				log.Printf("Error marshalling waiting list for controller '%s': %v", controllerID, err)
				continue
			}
			uc.ConnManager.SendMessage(controllerID, "server", "waiting", payload)
		}
	}

	// Finally, delete the display from the repository.
	uc.DisplayRepo.Delete(displayID)
	log.Printf("Removed display '%s' from repository.", displayID)
}

// RegisterController handles a new Controller connection.
type RegisterController struct {
	ControllerRepo   ControllerRepository
	WebSocketService interface {
		RegisterControllerConnection(controllerID string, conn any)
		SendError(clientID string, code int, message string)
	}
	IDGenerator IDGenerator // New dependency
}

// Execute registers a new controller and its connection.
func (uc *RegisterController) Execute(conn *websocket.Conn) (string, error) {
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

// HandleControllerDisconnection handles the disconnection of a Controller.
type HandleControllerDisconnection struct {
	ControllerRepo ControllerRepository
	DisplayRepo    DisplayRepository // Add DisplayRepo dependency
	ConnManager    interface {
		UnregisterControllerConnection(controllerID string)
		SendMessage(to, from, msgType string, payload json.RawMessage)
	}
}

// Execute performs all cleanup tasks when a controller disconnects.
func (uc *HandleControllerDisconnection) Execute(controllerID string) {
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
			uc.ConnManager.SendMessage(displayID, "server", "unsubscribed", unsubscribedPayload)
		}
	}

	// Finally, delete the controller from the repository.
	uc.ControllerRepo.Delete(controllerID)
	log.Printf("Removed controller '%s' from repository.", controllerID)
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

// DeleteController handles the deletion of a Controller.
type DeleteController struct {
	ControllerRepo ControllerRepository
	DisplayRepo    DisplayRepository
	ConnManager    interface {
		UnregisterControllerConnection(controllerID string)
	}
}

// Execute deletes a controller and unregisters its connection.
func (uc *DeleteController) Execute(controllerID string) error {
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

// ProcessControllerMessage handles messages received from a Controller.
type ProcessControllerMessage struct {
	ControllerRepo   ControllerRepository
	DisplayRepo      DisplayRepository
	WebSocketService ClientNotifier
}

// Execute handles an incoming message from a controller.
func (uc *ProcessControllerMessage) Execute(controllerID string, message []byte) error {
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

		for _, displayID := range payload.DisplayIDs {
			displayIface, displayFound := uc.DisplayRepo.FindByID(displayID)
			if !displayFound {
				// Display is offline, add to waiting list
				controller.WaitingFor[displayID] = true
				log.Printf("Controller '%s' is now waiting for offline display '%s'", controllerID, displayID)
				continue
			}

			// Display is online, proceed with subscription
			actualDisplay := displayIface.(*domain.Display)

			actualDisplay.Mu.Lock()
			actualDisplay.Subscribers[controllerID] = true // Add controller to display's subscribers
			actualDisplay.Mu.Unlock()

			controller.Subscriptions[displayID] = true // Add display to controller's subscriptions
			uc.DisplayRepo.Save(actualDisplay)         // Persist display changes

			// Send command list to controller. `from` is the display, `to` is the controller.
			uc.WebSocketService.SendMessage(controllerID, displayID, "command_list", actualDisplay.CommandList)

			// Notify the display that a new controller has subscribed
			subscribedCount := len(actualDisplay.Subscribers)
			subscribedPayload, _ := json.Marshal(domain.SubscribedPayload{Count: subscribedCount})
			uc.WebSocketService.SendMessage(displayID, "server", "subscribed", subscribedPayload)
		}

		// After processing all subscriptions, get the current waiting list and send it
		waitingList := make([]string, 0, len(controller.WaitingFor))
		for id := range controller.WaitingFor {
			waitingList = append(waitingList, id)
		}
		uc.ControllerRepo.Save(controller) // Persist controller changes (subscriptions and waiting list)
		controller.Mu.Unlock()

		msg, err := json.Marshal(waitingList)
		if err != nil {
			log.Printf("Error marshalling waiting list for controller '%s': %v", controllerID, err)
			return fmt.Errorf("failed to marshal waiting list for controller '%s': %w", controllerID, err)
		}
		uc.WebSocketService.SendMessage(controllerID, "server", "waiting", msg)
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

		for _, displayID := range payload.DisplayIDs {
			// Remove from subscriptions
			if _, ok := controller.Subscriptions[displayID]; ok {
				delete(controller.Subscriptions, displayID)
				if displayIface, displayFound := uc.DisplayRepo.FindByID(displayID); displayFound {
					actualDisplay := displayIface.(*domain.Display)
					actualDisplay.RemoveSubscriber(controllerID)
					uc.DisplayRepo.Save(actualDisplay)

					unsubscribedCount := len(actualDisplay.Subscribers)
					unsubscribedPayload, _ := json.Marshal(domain.UnsubscribedPayload{Count: unsubscribedCount})
					uc.WebSocketService.SendMessage(displayID, "server", "unsubscribed", unsubscribedPayload)
				}
			}
			// Also remove from waiting list
			delete(controller.WaitingFor, displayID)
		}

		// After processing all unsubscriptions, get the current waiting list and send it
		waitingList := make([]string, 0, len(controller.WaitingFor))
		for id := range controller.WaitingFor {
			waitingList = append(waitingList, id)
		}
		uc.ControllerRepo.Save(controller)
		controller.Mu.Unlock()

		msg, err := json.Marshal(waitingList)
		if err != nil {
			log.Printf("Error marshalling waiting list for controller '%s': %v", controllerID, err)
			return fmt.Errorf("failed to marshal waiting list for controller '%s': %w", controllerID, err)
		}
		uc.WebSocketService.SendMessage(controllerID, "server", "waiting", msg)
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
		uc.WebSocketService.SendMessage(msg.To, controllerID, "command", msg.Payload)
		log.Printf("Controller '%s' sent command to display '%s'.", controllerID, msg.To)

	default:
		uc.WebSocketService.SendError(controllerID, domain.ErrInvalidMessageFormat, fmt.Sprintf("Unknown message type: %s", msg.Type))
		return fmt.Errorf("unknown message type from controller '%s': %s", controllerID, msg.Type)
	}
	return nil
}
