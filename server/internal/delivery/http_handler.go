package delivery

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/simbafs/controly/server/internal/application"
	"github.com/simbafs/controly/server/internal/domain"
	"github.com/simbafs/controly/server/internal/infrastructure" // Needed for type assertion to InMemoryDisplayRepository/InMemoryControllerRepository
)

// API response structs
type connectionsResponse struct {
	Displays    []displayInfo    `json:"displays"`
	Controllers []controllerInfo `json:"controllers"`
}

type displayInfo struct {
	ID           string `json:"id"`
	ControlledBy string `json:"controlled_by,omitempty"` // Omit if empty
}

type controllerInfo struct {
	ID          string `json:"id"`
	Controlling string `json:"controlling,omitempty"` // Omit if empty
}

// ConnectionsHandler holds the dependencies for the connections API endpoint.
type ConnectionsHandler struct {
	DisplayRepo    application.DisplayRepository
	ControllerRepo application.ControllerRepository
}

// NewConnectionsHandler creates a new ConnectionsHandler.
func NewConnectionsHandler(displayRepo application.DisplayRepository, controllerRepo application.ControllerRepository) *ConnectionsHandler {
	return &ConnectionsHandler{
		DisplayRepo:    displayRepo,
		ControllerRepo: controllerRepo,
	}
}

// ServeHTTP handles the GET /api/connections endpoint.
func (h *ConnectionsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	displays := []displayInfo{}
	controllers := []controllerInfo{}

	// Get all displays
	h.DisplayRepo.(*infrastructure.InMemoryDisplayRepository).Range(func(key, value any) bool {
		display := value.(*domain.Display)
		info := displayInfo{ID: display.ID}
		display.Mu.Lock() // Lock to safely read Controller field
		if display.Controller != nil {
			info.ControlledBy = display.Controller.ID
		}
		display.Mu.Unlock()
		displays = append(displays, info)
		return true
	})

	// Get all controllers
	h.ControllerRepo.(*infrastructure.InMemoryControllerRepository).Range(func(key, value any) bool {
		controller := value.(*domain.Controller)
		info := controllerInfo{ID: controller.ID}
		if controller.TargetDisplay != nil {
			info.Controlling = controller.TargetDisplay.ID
		}
		controllers = append(controllers, info)
		return true
	})

	response := connectionsResponse{
		Displays:    displays,
		Controllers: controllers,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding connections response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
