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
	}
	log.Printf("Client registered: %s (%s)", client.id, client.clientType)
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
	msg := domain.OutgoingMessage{
		Type:    msgType,
		From:    from,
		Payload: payload,
	}
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshalling outgoing message: %v", err)
		return
	}

	var targetClient *Client
	if c, ok := h.displays.Load(to); ok {
		targetClient = c.(*Client)
	} else if c, ok := h.controllers.Load(to); ok {
		targetClient = c.(*Client)
	}

	if targetClient != nil {
		select {
		case targetClient.send <- msgBytes:
		default:
			log.Printf("Send channel full for client %s, message dropped.", to)
			// Optionally unregister the client if the channel is consistently full
			// h.unregister <- targetClient
		}
	} else {
		log.Printf("Client %s not found for sending message.", to)
	}
}

func (h *Hub) broadcast(targets []string, from, msgType string, payload any) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling broadcast payload: %v", err)
		return
	}
	for _, targetID := range targets {
		h.sendRaw(targetID, from, msgType, payloadBytes)
	}
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

	h.send(clientID, "server", "set_id", map[string]string{"id": clientID})

	if clientTypeEnum == domain.ClientTypeDisplay {
		h.postDisplayRegistration(clientID)
	}
}
