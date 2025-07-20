package infrastructure

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/simbafs/controly/server/internal/domain"
)

// InspectorGateway manages WebSocket connections for the /ws/inspect endpoint.
type InspectorGateway struct {
	upgrader   websocket.Upgrader
	clients    map[*websocket.Conn]bool
	clientsMux sync.Mutex
}

// NewInspectorGateway creates a new InspectorGateway.
func NewInspectorGateway() *InspectorGateway {
	return &InspectorGateway{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins
			},
		},
		clients: make(map[*websocket.Conn]bool),
	}
}

// Upgrade upgrades an HTTP connection to a WebSocket connection for an inspector.
func (g *InspectorGateway) Upgrade(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	return g.upgrader.Upgrade(w, r, nil)
}

// Register adds a new inspector client to the pool.
func (g *InspectorGateway) Register(conn *websocket.Conn) {
	g.clientsMux.Lock()
	defer g.clientsMux.Unlock()
	g.clients[conn] = true
	log.Println("Inspector client connected.")
}

// Unregister removes an inspector client from the pool.
func (g *InspectorGateway) Unregister(conn *websocket.Conn) {
	g.clientsMux.Lock()
	defer g.clientsMux.Unlock()
	if _, ok := g.clients[conn]; ok {
		delete(g.clients, conn)
		conn.Close()
		log.Println("Inspector client disconnected.")
	}
}

// Broadcast sends a message to all connected inspector clients.
func (g *InspectorGateway) Broadcast(msg *domain.InspectionMessage) {
	g.clientsMux.Lock()
	defer g.clientsMux.Unlock()

	if len(g.clients) == 0 {
		return
	}

	// Create a list of clients to remove to avoid modifying the map while iterating
	var clientsToRemove []*websocket.Conn

	for client := range g.clients {
		err := client.WriteJSON(msg)
		if err != nil {
			log.Printf("Error writing to inspector client, will unregister: %v", err)
			clientsToRemove = append(clientsToRemove, client)
		}
	}

	// Remove dead clients
	for _, client := range clientsToRemove {
		delete(g.clients, client)
		client.Close()
	}
}
