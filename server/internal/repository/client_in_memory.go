package repository

import (
	"context"
	"sync"

	"github.com/simbafs/controly/server/internal/entity"
)

var _ Client = (*clientInMeomry)(nil)

type clientInMeomry struct {
	sync.RWMutex
	clients map[string]*entity.Client
}

func NewClientInMemory() *clientInMeomry {
	return &clientInMeomry{
		clients: make(map[string]*entity.Client),
	}
}

func (c *clientInMeomry) Get(ctx context.Context, id string) (*entity.Client, error) {
	c.RLock()
	defer c.RUnlock()

	client, exists := c.clients[id]
	if !exists {
		return nil, ErrNotFound
	}
	return client, nil
}

func (c *clientInMeomry) Put(ctx context.Context, id string, client *entity.Client) error {
	c.Lock()
	defer c.Unlock()

	c.clients[id] = client
	return nil
}
