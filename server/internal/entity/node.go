package entity

import (
	"errors"
	"fmt"
	"sync"

	"github.com/gorilla/websocket"
)

var ErrNoConnection = errors.New("no websocket connection")

type EventHandler func(data any) error

type wsMsg struct {
	Event string `json:"event"`
	Data  any    `json:"data"`
}

type Node struct {
	id          string
	ws          *websocket.Conn
	isConnected bool
	handlers    map[string][]EventHandler
	mutex       sync.Mutex
}

func NewNode(id string, ws *websocket.Conn) *Node {
	return &Node{
		id:          id,
		ws:          ws,
		isConnected: false, // TODO:
	}
}

func (n *Node) ID() string {
	return n.id
}

func (n *Node) IsConnected() bool {
	return n.isConnected
}

// Emit sends data to the websocket connection
func (n *Node) Emit(event string, data any) error {
	if n.ws == nil {
		return ErrNoConnection
	}

	msg := wsMsg{
		Event: event,
		Data:  data,
	}

	err := n.ws.WriteJSON(msg)
	if err != nil {
		return fmt.Errorf("failed to write JSON to ws: %w", err)
	}
	return nil
}

// On registers an event handler for incomming events
func (n *Node) On(event string, handler EventHandler) error {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if n.ws == nil {
		return ErrNoConnection
	}

	if n.handlers == nil {
		n.handlers = make(map[string][]EventHandler)
	}

	n.handlers[event] = append(n.handlers[event], handler)
	return nil
}
