package internal

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/big"
	"net/http"
	"sort"

	"github.com/gorilla/mux"
	"github.com/simbafs/controly/server/internal/domain"
)

// --- HTTP Handlers ---

func (h *Hub) ConnectionsHandler(w http.ResponseWriter, r *http.Request) {
	displays := []map[string]any{}
	h.displayEntities.Range(func(key, value any) bool {
		display := value.(*domain.Display)
		display.Mu.Lock()
		subscribers := make([]string, 0, len(display.Subscribers))
		for sub := range display.Subscribers {
			subscribers = append(subscribers, sub)
		}
		display.Mu.Unlock()
		sort.Strings(subscribers)
		displays = append(displays, map[string]any{"id": display.ID, "subscribers": subscribers})
		return true
	})

	controllers := []map[string]any{}
	h.controllerEntities.Range(func(key, value any) bool {
		controller := value.(*domain.Controller)
		controller.Mu.Lock()
		subscriptions := make([]string, 0, len(controller.Subscriptions))
		for sub := range controller.Subscriptions {
			subscriptions = append(subscriptions, sub)
		}
		controller.Mu.Unlock()
		sort.Strings(subscriptions)
		controllers = append(controllers, map[string]any{"id": controller.ID, "subscriptions": subscriptions})
		return true
	})

	response := map[string]any{
		"displays":    displays,
		"controllers": controllers,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// --- WebSocket Message Handlers ---

func (h *Hub) handleDisplayMessage(client *Client, msg *domain.IncomingMessage) {
	if msg.Type == "status" {
		if d, ok := h.displayEntities.Load(client.id); ok {
			display := d.(*domain.Display)
			display.Mu.Lock()
			subscribers := make([]string, 0, len(display.Subscribers))
			for id := range display.Subscribers {
				subscribers = append(subscribers, id)
			}
			display.Mu.Unlock()
			h.broadcast(subscribers, client.id, "status", msg.Payload)
		}
	}
}

func (h *Hub) handleControllerMessage(client *Client, msg *domain.IncomingMessage) {
	switch msg.Type {
	case "subscribe":
		var payload struct {
			DisplayIDs []string `json:"display_ids"`
		}
		json.Unmarshal(msg.Payload, &payload)
		h.handleSubscribe(client.id, payload.DisplayIDs)
	case "unsubscribe":
		var payload struct {
			DisplayIDs []string `json:"display_ids"`
		}
		json.Unmarshal(msg.Payload, &payload)
		h.handleUnsubscribe(client.id, payload.DisplayIDs)
	case "command":
		h.sendRaw(msg.To, client.id, "command", msg.Payload)
	case "waiting":
		var displayIDs []string
		json.Unmarshal(msg.Payload, &displayIDs)
		h.handleWaitingList(client.id, displayIDs)
	}
}

// --- Business Logic (previously use cases) ---

func (h *Hub) handleNewDisplay(displayID, commandURL, token string) (string, error) {
	if h.serverToken != "" && h.serverToken != token {
		return "", fmt.Errorf("invalid token")
	}

	if displayID == "" {
		var err error
		displayID, err = h.generateUniqueDisplayID()
		if err != nil {
			return "", err
		}
	}

	if _, exists := h.displayEntities.Load(displayID); exists {
		return "", fmt.Errorf("display ID conflict: %s", displayID)
	}

	commandData, err := fetchCommands(commandURL)
	if err != nil {
		return "", err
	}

	display := domain.NewDisplay(displayID, commandData)
	h.displayEntities.Store(displayID, display)
	return displayID, nil
}

func (h *Hub) handleNewController() (string, error) {
	// Simple incremental ID for controllers
	var controllerID string
	var err error
	for {
		controllerID, err = generateRandomString(8, "controller-")
		if err != nil {
			return "", err
		}
		if _, exists := h.controllerEntities.Load(controllerID); !exists {
			break
		}
	}
	controller := domain.NewController(controllerID)
	h.controllerEntities.Store(controllerID, controller)
	return controllerID, nil
}

func (h *Hub) postDisplayRegistration(displayID string) {
	display, _ := h.displayEntities.Load(displayID)
	if display == nil {
		return
	}

	h.controllerEntities.Range(func(key, value any) bool {
		controller := value.(*domain.Controller)
		controller.Mu.Lock()
		isWaiting := controller.WaitingFor[displayID]
		controller.Mu.Unlock()

		if isWaiting {
			h.handleSubscribe(controller.ID, []string{displayID})
		}
		return true
	})
}

func (h *Hub) handleDisplayDisconnection(displayID string) {
	h.controllerEntities.Range(func(key, value any) bool {
		controller := value.(*domain.Controller)
		controller.Mu.Lock()
		if _, ok := controller.Subscriptions[displayID]; ok {
			delete(controller.Subscriptions, displayID)
			controller.WaitingFor[displayID] = true

			waitingList := make([]string, 0, len(controller.WaitingFor))
			for id := range controller.WaitingFor {
				waitingList = append(waitingList, id)
			}

			h.send(controller.ID, "server", "display_disconnected", domain.DisplayDisconnectedPayload{DisplayID: displayID})
			h.send(controller.ID, "server", "waiting", waitingList)
		}
		controller.Mu.Unlock()
		return true
	})
}

func (h *Hub) handleControllerDisconnection(controllerID string) {
	if c, ok := h.controllerEntities.Load(controllerID); ok {
		controller := c.(*domain.Controller)
		controller.Mu.Lock()
		subscriptions := make([]string, 0, len(controller.Subscriptions))
		for subID := range controller.Subscriptions {
			subscriptions = append(subscriptions, subID)
		}
		controller.Mu.Unlock()

		for _, displayID := range subscriptions {
			if d, ok := h.displayEntities.Load(displayID); ok {
				display := d.(*domain.Display)
				display.RemoveSubscriber(controllerID)
				h.send(displayID, "server", "unsubscribed", domain.UnsubscribedPayload{Count: len(display.Subscribers)})
			}
		}
	}
}

func (h *Hub) handleSubscribe(controllerID string, displayIDs []string) {
	c, _ := h.controllerEntities.Load(controllerID)
	if c == nil {
		return
	}
	controller := c.(*domain.Controller)

	for _, displayID := range displayIDs {
		d, ok := h.displayEntities.Load(displayID)
		if !ok {
			controller.Mu.Lock()
			controller.WaitingFor[displayID] = true
			controller.Mu.Unlock()
			continue
		}
		display := d.(*domain.Display)

		controller.Mu.Lock()
		delete(controller.WaitingFor, displayID)
		controller.Subscriptions[displayID] = true
		controller.Mu.Unlock()

		display.Mu.Lock()
		display.Subscribers[controllerID] = true
		display.Mu.Unlock()

		h.sendRaw(controllerID, displayID, "command_list", display.CommandList)
		h.send(displayID, "server", "subscribed", domain.SubscribedPayload{Count: len(display.Subscribers)})
	}

	controller.Mu.Lock()
	waitingList := make([]string, 0, len(controller.WaitingFor))
	for id := range controller.WaitingFor {
		waitingList = append(waitingList, id)
	}
	controller.Mu.Unlock()
	h.send(controllerID, "server", "waiting", waitingList)
}

func (h *Hub) handleUnsubscribe(controllerID string, displayIDs []string) {
	c, _ := h.controllerEntities.Load(controllerID)
	if c == nil {
		return
	}
	controller := c.(*domain.Controller)

	controller.Mu.Lock()
	for _, displayID := range displayIDs {
		delete(controller.Subscriptions, displayID)
		if d, ok := h.displayEntities.Load(displayID); ok {
			display := d.(*domain.Display)
			display.RemoveSubscriber(controllerID)
			h.send(displayID, "server", "unsubscribed", domain.UnsubscribedPayload{Count: len(display.Subscribers)})
		}
	}
	controller.Mu.Unlock()
}

func (h *Hub) handleWaitingList(controllerID string, displayIDs []string) {
	c, _ := h.controllerEntities.Load(controllerID)
	if c == nil {
		return
	}
	controller := c.(*domain.Controller)

	isDisplayOnline := func(id string) bool {
		_, ok := h.displayEntities.Load(id)
		return ok
	}

	finalList := controller.SetWaitingList(displayIDs, isDisplayOnline)
	h.send(controllerID, "server", "waiting", finalList)
}

// --- Utility Functions ---

const (
	alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	idLength = 8
)

func generateRandomString(length int, prefix string) (string, error) {
	bytes := make([]byte, length)
	for i := range length {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", fmt.Errorf("failed to generate random number: %w", err)
		}
		bytes[i] = alphabet[num.Int64()]
	}
	return prefix + string(bytes), nil
}

func (h *Hub) generateUniqueDisplayID() (string, error) {
	for {
		id, err := generateRandomString(idLength, "")
		if err != nil {
			return "", err
		}
		if _, exists := h.displayEntities.Load(id); !exists {
			return id, nil
		}
	}
}

func fetchCommands(url string) ([]byte, error) {
	if url == "" {
		return nil, fmt.Errorf("command_url is required")
	}
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch command URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("command URL returned status code: %d", resp.StatusCode)
	}

	commandData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read command JSON: %w", err)
	}
	return commandData, nil
}

// Inspector Handler
func (h *Hub) InspectorWsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade inspector connection: %v", err)
		return
	}

	// Generate a unique ID for the inspector client
	inspectorID, err := generateRandomString(8, "inspector-")
	if err != nil {
		log.Printf("Failed to generate inspector ID: %v", err)
		conn.Close()
		return
	}

	client := &Client{
		hub:        h,
		conn:       conn,
		send:       make(chan []byte, 256),
		id:         inspectorID,
		clientType: domain.ClientTypeInspector,
	}
	h.register <- client

	go client.writePump()
	go client.readPump()
}

func (h *Hub) FrontendHandler(contentFs fs.FS) http.Handler {
	return http.FileServer(http.FS(contentFs))
}

func (h *Hub) DeleteDisplayHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if c, ok := h.displays.Load(id); ok {
		client := c.(*Client)
		h.unregister <- client
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Hub) DeleteControllerHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if c, ok := h.controllers.Load(id); ok {
		client := c.(*Client)
		h.unregister <- client
	}
	w.WriteHeader(http.StatusNoContent)
}
