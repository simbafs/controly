package delivery

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"

	"github.com/gorilla/mux"
	"github.com/simbafs/controly/server/internal/application"
)

// API response structs
type connectionsResponse struct {
	Displays    []displayInfo    `json:"displays"`
	Controllers []controllerInfo `json:"controllers"`
}

type displayInfo struct {
	ID          string   `json:"id"`
	Subscribers []string `json:"subscribers"`
}

type controllerInfo struct {
	ID            string   `json:"id"`
	Subscriptions []string `json:"subscriptions"`
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

	log.Println("collecting display")
	// Get all displays
	for display := range h.DisplayRepo.All() {
		info := displayInfo{ID: display.ID}
		log.Printf("waiting %s", display.ID)
		display.Mu.Lock() // Lock to safely read Subscribers map
		log.Printf("get %s", display.ID)
		for subscriberID := range display.Subscribers {
			info.Subscribers = append(info.Subscribers, subscriberID)
		}
		display.Mu.Unlock()
		sort.Strings(info.Subscribers) // Sort for consistent output
		displays = append(displays, info)
	}

	log.Println("collecting controller")
	// Get all controllers
	for controller := range h.ControllerRepo.All() {
		info := controllerInfo{ID: controller.ID}
		controller.Mu.Lock() // Lock to safely read Subscriptions map
		for subscriptionID := range controller.Subscriptions {
			info.Subscriptions = append(info.Subscriptions, subscriptionID)
		}
		controller.Mu.Unlock()
		sort.Strings(info.Subscriptions) // Sort for consistent output
		controllers = append(controllers, info)
	}

	log.Println("preparing response")
	response := connectionsResponse{
		Displays:    displays,
		Controllers: controllers,
	}

	log.Println("sending response")
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding connections response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
	log.Println("response sent successfully")
}

// DeleteDisplayHandler holds the dependency for the delete display API endpoint.
type DeleteDisplayHandler struct {
	DeleteDisplay *application.DeleteDisplay
}

// NewDeleteDisplayHandler creates a new DeleteDisplayHandler.
func NewDeleteDisplayHandler(deleteDisplay *application.DeleteDisplay) *DeleteDisplayHandler {
	return &DeleteDisplayHandler{
		DeleteDisplay: deleteDisplay,
	}
}

// ServeHTTP handles the DELETE /api/displays/{id} endpoint.
func (h *DeleteDisplayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		http.Error(w, "Missing display ID", http.StatusBadRequest)
		return
	}

	if err := h.DeleteDisplay.Execute(id); err != nil {
		log.Printf("Error deleting display %s: %v", id, err)
		http.Error(w, err.Error(), http.StatusNotFound) // Assuming error indicates not found or internal issue
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeleteControllerHandler holds the dependency for the delete controller API endpoint.
type DeleteControllerHandler struct {
	DeleteController *application.DeleteController
}

// NewDeleteControllerHandler creates a new DeleteControllerHandler.
func NewDeleteControllerHandler(deleteController *application.DeleteController) *DeleteControllerHandler {
	return &DeleteControllerHandler{
		DeleteController: deleteController,
	}
}

// ServeHTTP handles the DELETE /api/controllers/{id} endpoint.
func (h *DeleteControllerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		http.Error(w, "Missing controller ID", http.StatusBadRequest)
		return
	}

	if err := h.DeleteController.Execute(id); err != nil {
		log.Printf("Error deleting controller %s: %v", id, err)
		http.Error(w, err.Error(), http.StatusNotFound) // Assuming error indicates not found or internal issue
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
