package repository

import (
	"context"
	"errors"

	"github.com/simbafs/controly/server/internal/entity"
)

var ErrNotFound = errors.New("not found")

type Client interface {
	Get(ctx context.Context, id string) (*entity.Client, error)
	Put(ctx context.Context, client *entity.Client) error
}

type App interface {
	Get(ctx context.Context, id string) (*entity.App, error)
	List(ctx context.Context) ([]*entity.App, error)
	Put(ctx context.Context, app *entity.App) error
	Delete(ctx context.Context, id string) error
}
