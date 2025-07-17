package infrastructure

import (
	"encoding/json"
	"fmt"
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

// SendMessage sends a generic WebSocket message to a specific client.
func (g *GorillaWebSocketGateway) SendMessage(to, from, msgType string, payload json.RawMessage) error {
	var c *client

	if to == "" {
		return fmt.Errorf("cannot send message to empty recipient")
	}

	iface, ok := g.displayConnections.Load(to)
	if !ok {
		iface, ok = g.controllerConnections.Load(to)
		if !ok {
			return fmt.Errorf("connection for client ID '%s' not found", to)
		}
	}
	c = iface.(*client)

	msg := domain.OutgoingMessage{
		Type:    msgType,
		From:    from,
		Payload: payload,
	}

	c.send <- msg
	return nil
}

// SendError sends an error WebSocket message to a specific client.
func (g *GorillaWebSocketGateway) SendError(clientID string, code int, message string) error {
	payload, _ := json.Marshal(domain.ErrorPayload{Code: code, Message: message})
	// Errors are from the server
	return g.SendMessage(clientID, "server", "error", payload)
}
