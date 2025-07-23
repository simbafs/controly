package infrastructure

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/simbafs/controly/server/internal/domain"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
)

// client is a middleman between the websocket connection and the hub.
type client struct {
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan any
}

func newClient(conn *websocket.Conn) *client {
	return &client{
		conn: conn,
		send: make(chan any, 256), // Buffered channel to hold messages
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *client) writePump() {
	defer c.conn.Close()
	for message := range c.send {
		c.conn.WriteJSON(message)
	}
}

// GorillaWebSocketGateway implements application.WebSocketMessenger and handles WebSocket connections.
type GorillaWebSocketGateway struct {
	upgrader              websocket.Upgrader
	displayConnections    sync.Map // map[string]*client (displayID -> conn)
	controllerConnections sync.Map // map[string]*client (controllerID -> conn)
	inspector             *InspectorGateway
}

// NewGorillaWebSocketGateway creates a new GorillaWebSocketGateway.
func NewGorillaWebSocketGateway(inspector *InspectorGateway) *GorillaWebSocketGateway {
	return &GorillaWebSocketGateway{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for now
			},
		},
		inspector: inspector,
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
func (g *GorillaWebSocketGateway) RegisterDisplayConnection(displayID string, conn any) {
	wsConn, ok := conn.(*websocket.Conn)
	if !ok {
		log.Printf("error: connection is not of type *websocket.Conn")
		return
	}
	c := newClient(wsConn)
	g.displayConnections.Store(displayID, c)
	go c.writePump()
}

// UnregisterDisplayConnection removes a display's WebSocket connection.
func (g *GorillaWebSocketGateway) UnregisterDisplayConnection(displayID string) {
	iface, ok := g.displayConnections.Load(displayID)
	if !ok {
		return
	}
	c := iface.(*client)
	close(c.send)
	g.displayConnections.Delete(displayID)
}

// RegisterControllerConnection stores a controller's WebSocket connection.
func (g *GorillaWebSocketGateway) RegisterControllerConnection(controllerID string, conn any) {
	wsConn, ok := conn.(*websocket.Conn)
	if !ok {
		log.Printf("error: connection is not of type *websocket.Conn")
		return
	}
	c := newClient(wsConn)
	g.controllerConnections.Store(controllerID, c)
	go c.writePump()
}

// UnregisterControllerConnection removes a controller's WebSocket connection.
func (g *GorillaWebSocketGateway) UnregisterControllerConnection(controllerID string) {
	iface, ok := g.controllerConnections.Load(controllerID)
	if !ok {
		return
	}
	c := iface.(*client)
	close(c.send)
	g.controllerConnections.Delete(controllerID)
}

// BroadcastMessage sends a generic WebSocket message to a list of clients and logs it for inspection.
func (g *GorillaWebSocketGateway) BroadcastMessage(targets []string, from, msgType string, payload json.RawMessage) {
	if len(targets) == 0 {
		return
	}

	// 1. Construct the OutgoingMessage
	msg := domain.OutgoingMessage{
		Type:    msgType,
		From:    from,
		Payload: payload,
	}

	// 2. Marshal it for the inspection message
	originalMsgBytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshalling original message for inspection: %v", err)
		// We can still proceed with sending the message to the target
	} else {
		// 3. Construct and broadcast the InspectionMessage
		inspectionMsg := &domain.InspectionMessage{
			Source:          from,
			Targets:         targets,
			Timestamp:       time.Now().UTC().Format("2006-01-02T15:04:05.999Z07:00"), // RFC3339Nano
			OriginalMessage: originalMsgBytes,
		}
		g.inspector.Broadcast(inspectionMsg)
	}

	// 4. Send the message to all targets
	for _, targetID := range targets {
		var c *client
		iface, ok := g.displayConnections.Load(targetID)
		if !ok {
			iface, ok = g.controllerConnections.Load(targetID)
			if !ok {
				log.Printf("Connection for client ID '%s' not found for broadcast", targetID)
				continue
			}
		}
		c = iface.(*client)
		c.send <- msg
	}
}

// SendMessage sends a generic WebSocket message to a specific client.
func (g *GorillaWebSocketGateway) SendMessage(to, from, msgType string, payload json.RawMessage) {
	if to == "" {
		log.Printf("cannot send message to empty recipient")
		return
	}
	g.BroadcastMessage([]string{to}, from, msgType, payload)
}

// SendJSON marshals a payload object to JSON and sends it to a single recipient.
func (g *GorillaWebSocketGateway) SendJSON(to, from, msgType string, payload any) {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling payload for type '%s' to '%s': %v", msgType, to, err)
		return
	}
	g.SendMessage(to, from, msgType, jsonPayload)
}

// SendError sends an error WebSocket message to a specific client.
func (g *GorillaWebSocketGateway) SendError(clientID string, code int, message string) {
	// Errors are from the server
	g.SendJSON(clientID, "server", "error", domain.ErrorPayload{Code: code, Message: message})
}
