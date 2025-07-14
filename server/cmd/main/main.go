package main

import (
	"context"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/simbafs/controly/server/internal/api"
	"github.com/simbafs/controly/server/internal/entity"
	"github.com/simbafs/controly/server/internal/hub"
	"github.com/simbafs/controly/server/internal/repository"
	"github.com/simbafs/controly/server/internal/usecase"
)

func insertTestData(appRepo repository.App) {
	c, err := entity.NewApp("countdown", "countdown")
	if err != nil {
		panic(err)
	}

	c.AppendControl(entity.NewButtonControl("start"))
	c.AppendControl(entity.NewButtonControl("stop"))
	c.AppendControl(entity.NewButtonControl("reset"))
	c.AppendControl(entity.NewNumberControl("setTime", true, 0, 6000))

	appRepo.Put(context.Background(), c)
}

func main() {
	slog.SetLogLoggerLevel(slog.LevelDebug)

	hub := hub.NewHub()
	go hub.Run()

	r := gin.Default()

	appRepo := repository.NewAppInMemory()
	appUsecase := usecase.NewAppUsecase(appRepo)
	appAPI := api.NewAppAPI(appUsecase)
	appAPI.Setup(r)

	insertTestData(appRepo)

	wsAPI := api.NewWebsocketAPI(hub)
	wsAPI.Setup(r)

	r.Run()
}
