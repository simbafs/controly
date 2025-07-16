package infrastructure

import (
	"sync"

	"github.com/simbafs/controly/server/internal/domain"
)

// InMemoryDisplayRepository implements application.DisplayRepository for in-memory storage.
type InMemoryDisplayRepository struct {
	displays sync.Map // map[string]*domain.Display
}

// NewInMemoryDisplayRepository creates a new InMemoryDisplayRepository.
func NewInMemoryDisplayRepository() *InMemoryDisplayRepository {
	return &InMemoryDisplayRepository{}
}

// Save stores a display.
func (r *InMemoryDisplayRepository) Save(display *domain.Display) {
	r.displays.Store(display.ID, display)
}

// FindByID retrieves a display by its ID.
func (r *InMemoryDisplayRepository) FindByID(id string) (any, bool) {
	iface, ok := r.displays.Load(id)
	return iface, ok
}

// Delete removes a display by its ID.
func (r *InMemoryDisplayRepository) Delete(id string) {
	r.displays.Delete(id)
}

// Range calls f for each key and value in the map.
func (r *InMemoryDisplayRepository) Range(f func(key, value any) bool) {
	r.displays.Range(f)
}

// InMemoryControllerRepository implements application.ControllerRepository for in-memory storage.
type InMemoryControllerRepository struct {
	controllers sync.Map // map[string]*domain.Controller
}

// NewInMemoryControllerRepository creates a new InMemoryControllerRepository.
func NewInMemoryControllerRepository() *InMemoryControllerRepository {
	return &InMemoryControllerRepository{}
}

// Save stores a controller.
func (r *InMemoryControllerRepository) Save(controller *domain.Controller) {
	r.controllers.Store(controller.ID, controller)
}

// FindByID retrieves a controller by its ID.
func (r *InMemoryControllerRepository) FindByID(id string) (*domain.Controller, bool) {
	iface, ok := r.controllers.Load(id)
	if !ok {
		return nil, false
	}
	return iface.(*domain.Controller), true
}

// Delete removes a controller by its ID.
func (r *InMemoryControllerRepository) Delete(id string) {
	r.controllers.Delete(id)
}

// Range calls f for each key and value in the map.
func (r *InMemoryControllerRepository) Range(f func(key, value any) bool) {
	r.controllers.Range(f)
}
