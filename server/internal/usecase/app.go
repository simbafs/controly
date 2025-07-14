package usecase

import (
	"context"
	"fmt"

	"github.com/simbafs/controly/server/internal/entity"
	"github.com/simbafs/controly/server/internal/repository"
)

type App interface {
	CreateApp(ctx context.Context, name, password string) (*entity.App, error)
	UpdateApp(ctx context.Context, name string, controls []map[string]any) (*entity.App, error)
	ListApps(ctx context.Context) ([]*entity.App, error)
	GetApp(ctx context.Context, name string) (*entity.App, error)
	DeleteApp(ctx context.Context, name string) error
}

type appUsecase struct {
	appRepo repository.App
}

func NewAppUsecase(appRepo repository.App) App {
	return &appUsecase{
		appRepo: appRepo,
	}
}

func (uc *appUsecase) CreateApp(ctx context.Context, name, password string) (*entity.App, error) {
	app, err := entity.NewApp(name, password)
	if err != nil {
		return nil, fmt.Errorf("failed to create app entity: %w", err)
	}

	if err := uc.appRepo.Put(ctx, app); err != nil {
		return nil, fmt.Errorf("failed to save app: %w", err)
	}

	return app, nil
}

func (uc *appUsecase) UpdateApp(ctx context.Context, name string, controls []map[string]any) (*entity.App, error) {
	app, err := uc.appRepo.Get(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("app not found: %w", err)
	}

	app.ClearControls()
	for _, controlMap := range controls {
		control, err := entity.NewControl(controlMap)
		if err != nil {
			return nil, fmt.Errorf("failed to create control entity: %w", err)
		}

		app.AppendControl(control)
	}

	if err := uc.appRepo.Put(ctx, app); err != nil {
		return nil, fmt.Errorf("failed to update app: %w", err)
	}

	return app, nil
}

func (uc *appUsecase) ListApps(ctx context.Context) ([]*entity.App, error) {
	apps, err := uc.appRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list apps: %w", err)
	}
	return apps, nil
}

func (uc *appUsecase) GetApp(ctx context.Context, name string) (*entity.App, error) {
	app, err := uc.appRepo.Get(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}
	return app, nil
}

func (uc *appUsecase) DeleteApp(ctx context.Context, name string) error {
	if err := uc.appRepo.Delete(ctx, name); err != nil {
		return fmt.Errorf("failed to delete app: %w", err)
	}
	return nil
}
