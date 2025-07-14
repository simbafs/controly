
package hub

import (
	"github.com/gorilla/websocket"
)

type Hub struct {
	clients    map[*websocket.Conn]bool
	admins     map[*websocket.Conn]bool
	broadcast  chan []byte
	registerClient   chan *websocket.Conn
	unregisterClient chan *websocket.Conn
	registerAdmin   chan *websocket.Conn
	unregisterAdmin chan *websocket.Conn
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*websocket.Conn]bool),
		admins:     make(map[*websocket.Conn]bool),
		broadcast:  make(chan []byte),
		registerClient:   make(chan *websocket.Conn),
		unregisterClient: make(chan *websocket.Conn),
		registerAdmin:   make(chan *websocket.Conn),
		unregisterAdmin: make(chan *websocket.Conn),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case connection := <-h.registerClient:
			h.clients[connection] = true
		case connection := <-h.unregisterClient:
			if _, ok := h.clients[connection]; ok {
				delete(h.clients, connection)
				connection.Close()
			}
		case connection := <-h.registerAdmin:
			h.admins[connection] = true
		case connection := <-h.unregisterAdmin:
			if _, ok := h.admins[connection]; ok {
				delete(h.admins, connection)
				connection.Close()
			}
		case message := <-h.broadcast:
			// TODO: Implement targeted broadcast based on message content (e.g., uuid)
			for connection := range h.clients {
				if err := connection.WriteMessage(websocket.TextMessage, message); err != nil {
					// TODO: Handle websocket write error (e.g., log, unregister client)
					panic("not implemented")
				}
			}
			for connection := range h.admins {
				if err := connection.WriteMessage(websocket.TextMessage, message); err != nil {
					// TODO: Handle websocket write error (e.g., log, unregister admin)
					panic("not implemented")
				}
			}
		}
	}
}

func (h *Hub) RegisterClient(conn *websocket.Conn) {
	h.registerClient <- conn
}

func (h *Hub) UnregisterClient(conn *websocket.Conn) {
	h.unregisterClient <- conn
}

func (h *Hub) RegisterAdmin(conn *websocket.Conn) {
	h.registerAdmin <- conn
}

func (h *Hub) UnregisterAdmin(conn *websocket.Conn) {
	h.unregisterAdmin <- conn
}
