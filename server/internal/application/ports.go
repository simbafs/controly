package application

import (
	"encoding/json"
	"iter"

	"github.com/simbafs/controly/server/internal/domain"
)

// DisplayRepository defines the interface for managing Display entities.
type DisplayRepository interface {
	Save(display *domain.Display)
	FindByID(id string) (any, bool)
	Delete(id string)
	All() iter.Seq[*domain.Display]
	// Add other necessary methods like GetAll, Update, etc.
}

// ControllerRepository defines the interface for managing Controller entities.
type ControllerRepository interface {
	Save(controller *domain.Controller)
	FindByID(id string) (*domain.Controller, bool)
	Delete(id string)
	All() iter.Seq[*domain.Controller]
	// Add other necessary methods
}

// CommandFetcher defines the interface for fetching command.json from a URL.
type CommandFetcher interface {
	FetchCommands(url string) ([]byte, error)
}

// WebSocketMessenger defines the interface for sending WebSocket messages.
type WebSocketMessenger interface {
	SendMessage(to, from, msgType string, payload json.RawMessage) error
	SendError(clientID string, code int, message string) error
}

// WebSocketConnectionManager defines the interface for managing WebSocket connections from the application layer.
type WebSocketConnectionManager interface {
	WebSocketMessenger
	RegisterDisplayConnection(displayID string, conn any)
	UnregisterDisplayConnection(displayID string)
	RegisterControllerConnection(controllerID string, conn any)
	UnregisterControllerConnection(controllerID string)
}
