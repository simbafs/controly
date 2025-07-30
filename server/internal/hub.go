package internal

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
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins
	},
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub        *Hub
	conn       *websocket.Conn
	send       chan []byte
	id         string
	clientType domain.ClientType
}

// readPump pumps messages from the websocket connection to the hub.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		c.hub.handleMessage(c, message)
	}
}

// writePump pumps messages from the hub to the websocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Hub maintains the set of active clients and broadcasts messages.
type Hub struct {
	displays    sync.Map // map[string]*Client
	controllers sync.Map // map[string]*Client
	inspectors  sync.Map // map[string]*Client

	displayEntities    sync.Map // map[string]*domain.Display
	controllerEntities sync.Map // map[string]*domain.Controller

	register   chan *Client
	unregister chan *Client

	serverToken string
}

func NewHub(serverToken string) *Hub {
	return &Hub{
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		serverToken: serverToken,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)
		case client := <-h.unregister:
			h.unregisterClient(client)
		}
	}
}

func (h *Hub) registerClient(client *Client) {
	switch client.clientType {
	case domain.ClientTypeDisplay:
		h.displays.Store(client.id, client)
	case domain.ClientTypeController:
		h.controllers.Store(client.id, client)
	case domain.ClientTypeInspector:
		h.inspectors.Store(client.id, client)
	}
	log.Printf("Client registered: %s (%s)", client.id, client.clientType)
	if client.clientType != domain.ClientTypeInspector {
		h.send(client.id, "server", "set_id", domain.SetIDPayload{
			ID: client.id,
		})
	}
}

func (h *Hub) unregisterClient(client *Client) {
	switch client.clientType {
	case domain.ClientTypeDisplay:
		h.displays.Delete(client.id)
		h.displayEntities.Delete(client.id)
		h.handleDisplayDisconnection(client.id)
		log.Printf("Display unregistered and removed: %s", client.id)
	case domain.ClientTypeController:
		h.controllers.Delete(client.id)
		h.controllerEntities.Delete(client.id)
		h.handleControllerDisconnection(client.id)
		log.Printf("Controller unregistered and removed: %s", client.id)
	case domain.ClientTypeInspector:
		h.inspectors.Delete(client.id)
		log.Printf("Inspector unregistered and removed: %s", client.id)
	}
	close(client.send)
}

func (h *Hub) handleMessage(client *Client, message []byte) {
	var msg domain.IncomingMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Printf("Error unmarshalling message from %s: %v", client.id, err)
		return
	}

	switch client.clientType {
	case domain.ClientTypeDisplay:
		h.handleDisplayMessage(client, &msg)
	case domain.ClientTypeController:
		h.handleControllerMessage(client, &msg)
	}
}

func (h *Hub) send(to, from, msgType string, payload any) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling payload for type '%s': %v", msgType, err)
		return
	}
	h.sendRaw(to, from, msgType, payloadBytes)
}

func (h *Hub) sendRaw(to, from, msgType string, payload json.RawMessage) {
	h.broadcast([]string{to}, from, msgType, payload)
}

func (h *Hub) broadcast(targets []string, from, msgType string, payload any) {
	if len(targets) == 0 {
		return
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling broadcast payload: %v", err)
		return
	}
	msg := domain.OutgoingMessage{
		Type:    msgType,
		From:    from,
		Payload: payloadBytes,
	}
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshalling outgoing message for broadcast: %v", err)
		return
	}

	// Forward outgoing broadcast to inspector
	h.broadcastToInspectors(from, targets, msgBytes)

	for _, targetID := range targets {
		var targetClient *Client
		if c, ok := h.displays.Load(targetID); ok {
			targetClient = c.(*Client)
		} else if c, ok := h.controllers.Load(targetID); ok {
			targetClient = c.(*Client)
		}

		if targetClient != nil {
			select {
			case targetClient.send <- msgBytes:
			default:
				log.Printf("Send channel full for client %s, message dropped.", targetID)
			}
		} else {
			log.Printf("Client %s not found for sending message.", targetID)
		}
	}
}

func (h *Hub) isInspectorConnected() bool {
	var isConnected bool
	h.inspectors.Range(func(key, value any) bool {
		isConnected = true
		return false // stop iteration
	})
	return isConnected
}

func (h *Hub) broadcastToInspectors(source string, targets []string, originalMessage json.RawMessage) {
	if !h.isInspectorConnected() {
		return
	}

	var temp any
	msgToSend := originalMessage
	if err := json.Unmarshal(originalMessage, &temp); err != nil {
		// Not a valid JSON, treat as a raw string and marshal it into a JSON string value.
		quoted, wrapErr := json.Marshal(string(originalMessage))
		if wrapErr != nil {
			log.Printf("Error wrapping non-JSON message for inspector: %v", wrapErr)
		} else {
			msgToSend = quoted
		}
	}

	inspectionMessage := domain.InspectionMessage{
		Source:          source,
		Targets:         targets,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
		OriginalMessage: msgToSend,
	}

	messageBytes, err := json.Marshal(inspectionMessage)
	if err != nil {
		log.Printf("Error marshalling inspection message: %v", err)
		return
	}

	h.inspectors.Range(func(key, value any) bool {
		client := value.(*Client)
		select {
		case client.send <- messageBytes:
		default:
			log.Printf("Inspector send channel full for client %s, message dropped.", client.id)
		}
		return true
	})
}

func (h *Hub) ServeWs(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	clientTypeStr := r.URL.Query().Get("type")
	var clientID string
	var clientTypeEnum domain.ClientType

	switch clientTypeStr {
	case "display":
		clientTypeEnum = domain.ClientTypeDisplay
		displayIDParam := r.URL.Query().Get("id")
		commandURL := r.URL.Query().Get("command_url")
		token := r.URL.Query().Get("token")
		clientID, err = h.handleNewDisplay(displayIDParam, commandURL, token)
		if err != nil {
			log.Printf("Display registration failed: %v", err)
			conn.Close()
			return
		}
	case "controller":
		clientTypeEnum = domain.ClientTypeController
		clientID, err = h.handleNewController()
		if err != nil {
			log.Printf("Controller registration failed: %v", err)
			conn.Close()
			return
		}
	default:
		log.Println("Invalid client type")
		conn.Close()
		return
	}

	client := &Client{
		hub:        h,
		conn:       conn,
		send:       make(chan []byte, 256),
		id:         clientID,
		clientType: clientTypeEnum,
	}
	h.register <- client

	go client.writePump()
	go client.readPump()

	if clientTypeEnum == domain.ClientTypeDisplay {
		h.postDisplayRegistration(clientID)
	}
}
