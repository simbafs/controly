package infrastructure

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/simbafs/controly/server/internal/domain"
)

// GorillaWebSocketGateway implements application.WebSocketMessenger and handles WebSocket connections.
type GorillaWebSocketGateway struct {
	upgrader              websocket.Upgrader
	displayConnections    sync.Map // map[string]*websocket.Conn (displayID -> conn)
	controllerConnections sync.Map // map[string]*websocket.Conn (controllerID -> conn)
}

// NewGorillaWebSocketGateway creates a new GorillaWebSocketGateway.
func NewGorillaWebSocketGateway() *GorillaWebSocketGateway {
	return &GorillaWebSocketGateway{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for now
			},
		},
	}
}

// Upgrade upgrades an HTTP connection to a WebSocket connection.
func (g *GorillaWebSocketGateway) Upgrade(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	return g.upgrader.Upgrade(w, r, nil)
}

// ReadMessage reads a WebSocket message from the connection.
func (g *GorillaWebSocketGateway) ReadMessage(conn *websocket.Conn) (messageType int, p []byte, err error) {
	return conn.ReadMessage()
}

// WriteMessage writes a WebSocket message to the connection.
func (g *GorillaWebSocketGateway) WriteMessage(conn *websocket.Conn, messageType int, data []byte) error {
	return conn.WriteMessage(messageType, data)
}

// RegisterDisplayConnection stores a display's WebSocket connection.
func (g *GorillaWebSocketGateway) RegisterDisplayConnection(displayID string, conn *websocket.Conn) {
	g.displayConnections.Store(displayID, conn)
}

// UnregisterDisplayConnection removes a display's WebSocket connection.
func (g *GorillaWebSocketGateway) UnregisterDisplayConnection(displayID string) {
	g.displayConnections.Delete(displayID)
}

// RegisterControllerConnection stores a controller's WebSocket connection.
func (g *GorillaWebSocketGateway) RegisterControllerConnection(controllerID string, conn *websocket.Conn) {
	g.controllerConnections.Store(controllerID, conn)
}

// UnregisterControllerConnection removes a controller's WebSocket connection.
func (g *GorillaWebSocketGateway) UnregisterControllerConnection(controllerID string) {
	g.controllerConnections.Delete(controllerID)
}

// SendMessage sends a generic WebSocket message to a specific client.
func (g *GorillaWebSocketGateway) SendMessage(clientID string, msgType string, payload json.RawMessage) error {
	var conn *websocket.Conn

	iface, ok := g.displayConnections.Load(clientID)
	if ok {
		conn = iface.(*websocket.Conn)
	} else {
		iface, ok = g.controllerConnections.Load(clientID)
		if ok {
			conn = iface.(*websocket.Conn)
		} else {
			return fmt.Errorf("connection for client ID '%s' not found", clientID)
		}
	}

	msg := domain.WebSocketMessage{Type: msgType, Payload: payload}
	if err := conn.WriteJSON(msg); err != nil {
		log.Printf("Error sending message to client '%s': %v", clientID, err)
		return fmt.Errorf("failed to send message to client '%s': %w", clientID, err)
	}
	return nil
}

// SendError sends an error WebSocket message to a specific client.
func (g *GorillaWebSocketGateway) SendError(clientID string, code int, message string) error {
	payload, _ := json.Marshal(domain.ErrorPayload{Code: code, Message: message})
	return g.SendMessage(clientID, "error", payload)
}
