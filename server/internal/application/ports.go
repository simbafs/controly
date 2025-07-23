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
	GetControllersWaitingFor(displayID string) []*domain.Controller
}

// CommandFetcher defines the interface for fetching command.json from a URL.
type CommandFetcher interface {
	FetchCommands(url string) ([]byte, error)
}

// ClientNotifier defines the interface for sending WebSocket messages.
type ClientNotifier interface {
	SendMessage(to, from, msgType string, payload json.RawMessage)
	SendError(clientID string, code int, message string)
}
