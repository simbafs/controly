package main

import (
	"github.com/gin-gonic/gin"
	"github.com/simbafs/controly/server/internal/api"
	"github.com/simbafs/controly/server/internal/hub"
	"github.com/simbafs/controly/server/internal/repository"
)

func main() {
	hub := hub.NewHub()
	go hub.Run()

	r := gin.Default()

	appRepo := repository.NewAppInMemory()
	appAPI := api.NewAppAPI(appRepo)
	appAPI.Setup(r)

	wsAPI := api.NewWebsocketAPI(hub)
	wsAPI.Setup(r)

	r.Run()
}
