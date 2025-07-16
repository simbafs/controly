package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Error Codes
const (
	// Connection Errors (1xxx)
	ErrInvalidQueryParams = 1001
	ErrInvalidClientType  = 1002

	// Display Registration Errors (2xxx)
	ErrCommandURLUnreachable = 2001
	ErrInvalidCommandJSON    = 2002
	ErrDisplayIDConflict     = 2003

	// Controller Connection Errors (3xxx)
	ErrTargetDisplayNotFound        = 3001
	ErrTargetDisplayAlreadyControlled = 3002

	// Communication Errors (4xxx)
	ErrInvalidMessageFormat = 4001
	ErrUnknownCommand       = 4002
	ErrInvalidCommandArgs   = 4003
)

// WebSocketMessage represents the generic WebSocket message format
type WebSocketMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// ErrorPayload represents the payload for an error message
type ErrorPayload struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Command represents a single command definition from command.json
// DisplayClient represents a connected Display
type DisplayClient struct {
	ID          string
	Conn        *websocket.Conn
	CommandList json.RawMessage // Store raw command.json content
	Controller  *ControllerClient // Pointer to the controlling Controller, if any
	mu          sync.Mutex        // Mutex to protect access to Controller
}

// ControllerClient represents a connected Controller
type ControllerClient struct {
	ID          string
	Conn        *websocket.Conn
	TargetDisplay *DisplayClient // Pointer to the Display being controlled
}

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for now
		},
	}

	displays    = make(map[string]*DisplayClient)
	controllers = make(map[string]*ControllerClient)
	mu          sync.Mutex // Mutex to protect displays and controllers maps
)

func sendError(conn *websocket.Conn, code int, message string) {
	payload, _ := json.Marshal(ErrorPayload{Code: code, Message: message})
	msg := WebSocketMessage{Type: "error", Payload: payload}
	if err := conn.WriteJSON(msg); err != nil {
		log.Printf("Error sending error message: %v", err)
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}
	defer conn.Close()

	params := r.URL.Query()
	clientType := params.Get("type")

	switch clientType {
	case "display":
		handleDisplayConnection(conn, params)
	case "controller":
		handleControllerConnection(conn, params)
	default:
		sendError(conn, ErrInvalidClientType, "Invalid client type. Must be 'display' or 'controller'.")
		return
	}
}

func handleDisplayConnection(conn *websocket.Conn, params url.Values) {
	displayID := params.Get("id")
	commandURL := params.Get("command_url")

	if commandURL == "" {
		sendError(conn, ErrInvalidQueryParams, "Missing required query parameter: command_url")
		return
	}

	// Fetch and parse command.json
	resp, err := http.Get(commandURL)
	if err != nil {
		sendError(conn, ErrCommandURLUnreachable, fmt.Sprintf("Failed to fetch command URL: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		sendError(conn, ErrCommandURLUnreachable, fmt.Sprintf("Command URL returned status code: %d", resp.StatusCode))
		return
	}

	commandData, err := io.ReadAll(resp.Body)
	if err != nil {
		sendError(conn, ErrInvalidCommandJSON, fmt.Sprintf("Failed to read command JSON: %v", err))
		return
	}
	// Basic validation: check if it's valid JSON
	if !json.Valid(commandData) {
		sendError(conn, ErrInvalidCommandJSON, "Invalid command JSON format.")
		return
	}

	mu.Lock()
	if displayID == "" {
		// Generate UUID for displayID if not provided
		displayID = uuid.New().String()
		// Send set_id message to the display
		setIDPayload, _ := json.Marshal(map[string]string{"id": displayID}) // Payload is a JSON object with "id" key
		setIDMsg := WebSocketMessage{Type: "set_id", Payload: setIDPayload}
		if err := conn.WriteJSON(setIDMsg); err != nil {
			log.Printf("Error sending set_id message to new display: %v", err)
			// Consider closing connection if this critical message fails
		}
	} else {
		if _, exists := displays[displayID]; exists {
			mu.Unlock()
			sendError(conn, ErrDisplayIDConflict, fmt.Sprintf("Display ID '%s' is already in use.", displayID))
			return
		}
	}

	display := &DisplayClient{
		ID:          displayID,
		Conn:        conn,
		CommandList: commandData,
	}
	displays[displayID] = display
	mu.Unlock()

	log.Printf("Display '%s' connected.", displayID)

	// Send success message (optional, but good for confirmation)
	// For now, just log and proceed. The spec doesn't explicitly define a success message for display registration.

	for {
		_, p, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Display '%s' disconnected: %v", displayID, err)
			mu.Lock()
			delete(displays, displayID)
			// If this display was controlled, disconnect the controller
			if display.Controller != nil {
				display.Controller.TargetDisplay = nil
				delete(controllers, display.Controller.ID) // Remove controller from map
				sendError(display.Controller.Conn, ErrTargetDisplayNotFound, "Target display disconnected.")
				display.Controller.Conn.Close()
			}
			mu.Unlock()
			return
		}

		var msg WebSocketMessage
		if err := json.Unmarshal(p, &msg); err != nil {
			sendError(conn, ErrInvalidMessageFormat, "Invalid message format.")
			continue
		}

		if msg.Type == "status" {
			display.mu.Lock()
			if display.Controller != nil {
				if err := display.Controller.Conn.WriteMessage(websocket.TextMessage, p); err != nil {
					log.Printf("Error forwarding status to controller for display '%s': %v", displayID, err)
				}
			}
			display.mu.Unlock()
		} else {
			sendError(conn, ErrInvalidMessageFormat, fmt.Sprintf("Unknown message type from display: %s", msg.Type))
		}
	}
}

func handleControllerConnection(conn *websocket.Conn, params url.Values) {
	targetID := params.Get("target_id")

	if targetID == "" {
		sendError(conn, ErrInvalidQueryParams, "Missing required query parameter: target_id")
		return
	}

	mu.Lock()
	display, found := displays[targetID]
	if !found {
		mu.Unlock()
		sendError(conn, ErrTargetDisplayNotFound, fmt.Sprintf("Target display '%s' not found or offline.", targetID))
		return
	}

	display.mu.Lock()
	if display.Controller != nil {
		display.mu.Unlock()
		mu.Unlock()
		sendError(conn, ErrTargetDisplayAlreadyControlled, fmt.Sprintf("Display '%s' is already controlled by another client.", targetID))
		return
	}

	controllerID := fmt.Sprintf("controller-%d", len(controllers)+1) // Simple ID generation
	controller := &ControllerClient{
		ID:          controllerID,
		Conn:        conn,
		TargetDisplay: display,
	}
	controllers[controllerID] = controller
	display.Controller = controller
	display.mu.Unlock()
	mu.Unlock()

	log.Printf("Controller '%s' connected to Display '%s'.", controllerID, targetID)

	// Send command list to controller
	commandListMsg := WebSocketMessage{Type: "command_list", Payload: display.CommandList}
	if err := conn.WriteJSON(commandListMsg); err != nil {
		log.Printf("Error sending command list to controller '%s': %v", controllerID, err)
		// Consider disconnecting controller if this fails
		mu.Lock()
		display.mu.Lock()
		display.Controller = nil
		delete(controllers, controllerID)
		display.mu.Unlock()
		mu.Unlock()
		return
	}

	for {
		_, p, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Controller '%s' disconnected: %v", controllerID, err)
			mu.Lock()
			display.mu.Lock()
			if display.Controller == controller { // Only clear if this controller was the active one
				display.Controller = nil
			}
			delete(controllers, controllerID)
			display.mu.Unlock()
			mu.Unlock()
			return
		}

		var msg WebSocketMessage
		if err := json.Unmarshal(p, &msg); err != nil {
			sendError(conn, ErrInvalidMessageFormat, "Invalid message format.")
			continue
		}

		if msg.Type == "command" {
			// Validate command against display's command list (basic validation for now)
			// Forward command to display directly without validation
			display.mu.Lock()
			if display.Controller == controller { // Only forward if this controller is still active
				if err := display.Conn.WriteMessage(websocket.TextMessage, p); err != nil {
					log.Printf("Error forwarding command to display '%s': %v", targetID, err)
					// Consider disconnecting controller if display connection is bad
				}
			} else {
				sendError(conn, ErrTargetDisplayAlreadyControlled, "You are no longer controlling this display.")
			}
			display.mu.Unlock()
		} else {
			sendError(conn, ErrInvalidMessageFormat, fmt.Sprintf("Unknown message type from controller: %s", msg.Type))
		}
	}
}

func main() {
	http.HandleFunc("/ws", wsHandler)
	log.Println("Relay Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
