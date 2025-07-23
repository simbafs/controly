package infrastructure

import (
	"iter"
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

func (r *InMemoryDisplayRepository) All() iter.Seq[*domain.Display] {
	return func(yield func(*domain.Display) bool) {
		r.displays.Range(func(key any, value any) bool {
			display, ok := value.(*domain.Display)
			if !ok {
				return true // continue iteration
			}
			return yield(display)
		})
	}
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

func (r *InMemoryControllerRepository) All() iter.Seq[*domain.Controller] {
	return func(yield func(*domain.Controller) bool) {
		r.controllers.Range(func(key any, value any) bool {
			display, ok := value.(*domain.Controller)
			if !ok {
				return true // continue iteration
			}
			return yield(display)
		})
	}
}

// GetControllersWaitingFor retrieves all controllers waiting for a specific display ID.
func (r *InMemoryControllerRepository) GetControllersWaitingFor(displayID string) []*domain.Controller {
	var waitingControllers []*domain.Controller
	r.controllers.Range(func(key, value any) bool {
		controller, ok := value.(*domain.Controller)
		if !ok {
			return true // continue
		}

		controller.Mu.Lock()
		isWaiting := controller.WaitingFor[displayID]
		controller.Mu.Unlock()

		if isWaiting {
			waitingControllers = append(waitingControllers, controller)
		}
		return true
	})
	return waitingControllers
}
