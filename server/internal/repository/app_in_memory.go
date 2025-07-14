package repository

import (
	"context"
	"maps"
	"slices"
	"sync"

	"github.com/simbafs/controly/server/internal/entity"
)

var _ App = (*appInMemory)(nil)

type appInMemory struct {
	apps map[string]*entity.App
	sync.RWMutex
}

func NewAppInMemory() *appInMemory {
	return &appInMemory{
		apps: make(map[string]*entity.App),
	}
}

func (a *appInMemory) Get(ctx context.Context, id string) (*entity.App, error) {
	a.RLock()
	defer a.RUnlock()

	app, exists := a.apps[id]
	if !exists {
		return nil, ErrNotFound
	}
	return app, nil
}

func (a *appInMemory) List(ctx context.Context) ([]*entity.App, error) {
	a.RLock()
	defer a.RUnlock()

	apps := maps.Values(a.apps)
	return slices.SortedFunc(apps, func(a, b *entity.App) int {
		if a.Name() < b.Name() {
			return -1
		}
		if a.Name() > b.Name() {
			return 1
		}
		return 0
	}), nil
}

func (a *appInMemory) Put(ctx context.Context, app *entity.App) error {
	a.Lock()
	defer a.Unlock()

	a.apps[app.Name()] = app
	return nil
}

func (a *appInMemory) Delete(ctx context.Context, id string) error {
	a.Lock()
	defer a.Unlock()

	delete(a.apps, id)
	return nil
}
