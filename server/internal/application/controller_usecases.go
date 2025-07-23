package application

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/gorilla/websocket"
	"github.com/simbafs/controly/server/internal/domain"
)

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
		SendJSON(to, from, msgType string, payload any)
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
			uc.ConnManager.SendJSON(displayID, "server", "unsubscribed", domain.UnsubscribedPayload{Count: unsubscribedCount})
		}
	}

	// Finally, delete the controller from the repository.
	uc.ControllerRepo.Delete(controllerID)
	log.Printf("Removed controller '%s' from repository.", controllerID)
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

// ProcessControllerMessage handles messages received from a Controller.
type ProcessControllerMessage struct {
	ControllerRepo   ControllerRepository
	DisplayRepo      DisplayRepository
	WebSocketService ClientNotifier
}

// Execute handles an incoming message from a controller.
func (uc *ProcessControllerMessage) Execute(controllerID string, message []byte) error {
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
			return fmt.Errorf("invalid subscribe payload: %w", err)
		}
		usecase := &SubscribeToDisplays{
			ControllerRepo: uc.ControllerRepo,
			DisplayRepo:    uc.DisplayRepo,
			Notifier:       uc.WebSocketService,
		}
		return usecase.Execute(controllerID, payload.DisplayIDs)

	case "unsubscribe":
		var payload struct {
			DisplayIDs []string `json:"display_ids"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			uc.WebSocketService.SendError(controllerID, domain.ErrInvalidMessageFormat, "Invalid unsubscribe payload format.")
			return fmt.Errorf("invalid unsubscribe payload: %w", err)
		}
		usecase := &UnsubscribeFromDisplays{
			ControllerRepo: uc.ControllerRepo,
			DisplayRepo:    uc.DisplayRepo,
			Notifier:       uc.WebSocketService,
		}
		return usecase.Execute(controllerID, payload.DisplayIDs)

	case "command":
		if msg.To == "" {
			uc.WebSocketService.SendError(controllerID, domain.ErrInvalidMessageFormat, "Command message missing target 'to' field.")
			return fmt.Errorf("command message missing target")
		}
		usecase := &SendCommandToDisplay{
			ControllerRepo: uc.ControllerRepo,
			Notifier:       uc.WebSocketService,
		}
		return usecase.Execute(controllerID, msg.To, msg.Payload)

	case "waiting":
		var displayIDs []string
		if err := json.Unmarshal(msg.Payload, &displayIDs); err != nil {
			uc.WebSocketService.SendError(controllerID, domain.ErrInvalidMessageFormat, "Invalid waiting payload format, expected a string array.")
			return fmt.Errorf("invalid waiting payload: %w", err)
		}
		usecase := &UpdateWaitingList{
			ControllerRepo: uc.ControllerRepo,
			DisplayRepo:    uc.DisplayRepo,
			Notifier:       uc.WebSocketService,
		}
		return usecase.Execute(controllerID, displayIDs)

	default:
		uc.WebSocketService.SendError(controllerID, domain.ErrInvalidMessageFormat, fmt.Sprintf("Unknown message type: %s", msg.Type))
		return fmt.Errorf("unknown message type: %s", msg.Type)
	}
}

// SubscribeToDisplays handles the logic for a controller to subscribe to displays.
type SubscribeToDisplays struct {
	ControllerRepo ControllerRepository
	DisplayRepo    DisplayRepository
	Notifier       ClientNotifier
}

// Execute subscribes the controller to the given display IDs.
func (uc *SubscribeToDisplays) Execute(controllerID string, displayIDs []string) error {
	controller, found := uc.ControllerRepo.FindByID(controllerID)
	if !found {
		return fmt.Errorf("controller '%s' not found", controllerID)
	}

	controller.Mu.Lock()
	defer controller.Mu.Unlock()

	for _, displayID := range displayIDs {
		displayIface, displayFound := uc.DisplayRepo.FindByID(displayID)
		if !displayFound {
			controller.WaitingFor[displayID] = true
			log.Printf("Controller '%s' is now waiting for offline display '%s'", controllerID, displayID)
			continue
		}

		actualDisplay := displayIface.(*domain.Display)
		actualDisplay.Mu.Lock()
		actualDisplay.Subscribers[controllerID] = true
		actualDisplay.Mu.Unlock()

		controller.Subscriptions[displayID] = true
		uc.DisplayRepo.Save(actualDisplay)

		uc.Notifier.SendMessage(controllerID, displayID, "command_list", actualDisplay.CommandList)
		subscribedCount := len(actualDisplay.Subscribers)
		uc.Notifier.SendJSON(displayID, "server", "subscribed", domain.SubscribedPayload{Count: subscribedCount})
	}

	waitingList := make([]string, 0, len(controller.WaitingFor))
	for id := range controller.WaitingFor {
		waitingList = append(waitingList, id)
	}
	uc.ControllerRepo.Save(controller)
	uc.Notifier.SendJSON(controllerID, "server", "waiting", waitingList)
	log.Printf("Controller '%s' subscribed to displays: %v", controllerID, displayIDs)
	return nil
}

// UnsubscribeFromDisplays handles the logic for a controller to unsubscribe from displays.
type UnsubscribeFromDisplays struct {
	ControllerRepo ControllerRepository
	DisplayRepo    DisplayRepository
	Notifier       ClientNotifier
}

// Execute unsubscribes the controller from the given display IDs.
func (uc *UnsubscribeFromDisplays) Execute(controllerID string, displayIDs []string) error {
	controller, found := uc.ControllerRepo.FindByID(controllerID)
	if !found {
		return fmt.Errorf("controller '%s' not found", controllerID)
	}

	controller.Mu.Lock()
	defer controller.Mu.Unlock()

	for _, displayID := range displayIDs {
		if _, ok := controller.Subscriptions[displayID]; ok {
			delete(controller.Subscriptions, displayID)
			if displayIface, displayFound := uc.DisplayRepo.FindByID(displayID); displayFound {
				actualDisplay := displayIface.(*domain.Display)
				actualDisplay.RemoveSubscriber(controllerID)
				uc.DisplayRepo.Save(actualDisplay)

				unsubscribedCount := len(actualDisplay.Subscribers)
				uc.Notifier.SendJSON(displayID, "server", "unsubscribed", domain.UnsubscribedPayload{Count: unsubscribedCount})
			}
		}
		delete(controller.WaitingFor, displayID)
	}

	waitingList := make([]string, 0, len(controller.WaitingFor))
	for id := range controller.WaitingFor {
		waitingList = append(waitingList, id)
	}
	uc.ControllerRepo.Save(controller)
	uc.Notifier.SendJSON(controllerID, "server", "waiting", waitingList)
	log.Printf("Controller '%s' unsubscribed from displays: %v", controllerID, displayIDs)
	return nil
}

// SendCommandToDisplay handles the logic for a controller to send a command to a display.
type SendCommandToDisplay struct {
	ControllerRepo ControllerRepository
	Notifier       ClientNotifier
}

// Execute sends the command from the controller to the target display.
func (uc *SendCommandToDisplay) Execute(controllerID, targetDisplayID string, payload json.RawMessage) error {
	controller, found := uc.ControllerRepo.FindByID(controllerID)
	if !found {
		return fmt.Errorf("controller '%s' not found", controllerID)
	}

	controller.Mu.Lock()
	_, isSubscribed := controller.Subscriptions[targetDisplayID]
	controller.Mu.Unlock()

	if !isSubscribed {
		uc.Notifier.SendError(controllerID, domain.ErrNotSubscribedToDisplay, fmt.Sprintf("Not subscribed to display '%s'.", targetDisplayID))
		return fmt.Errorf("controller '%s' not subscribed to display '%s'", controllerID, targetDisplayID)
	}

	uc.Notifier.SendMessage(targetDisplayID, controllerID, "command", payload)
	log.Printf("Controller '%s' sent command to display '%s'.", controllerID, targetDisplayID)
	return nil
}

// UpdateWaitingList handles the business logic for a controller updating its waiting list.
type UpdateWaitingList struct {
	ControllerRepo ControllerRepository
	DisplayRepo    DisplayRepository
	Notifier       ClientNotifier
}

// Execute updates the controller's waiting list based on the provided display IDs.
func (uc *UpdateWaitingList) Execute(controllerID string, displayIDs []string) error {
	controller, found := uc.ControllerRepo.FindByID(controllerID)
	if !found {
		return fmt.Errorf("controller '%s' not found", controllerID)
	}

	isDisplayOnline := func(displayID string) bool {
		_, found := uc.DisplayRepo.FindByID(displayID)
		return found
	}

	finalWaitingList := controller.SetWaitingList(displayIDs, isDisplayOnline)
	uc.ControllerRepo.Save(controller)
	uc.Notifier.SendJSON(controllerID, "server", "waiting", finalWaitingList)
	log.Printf("Updated waiting list for controller '%s'. Now waiting for: %v", controllerID, finalWaitingList)
	return nil
}
