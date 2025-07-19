package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/simbafs/controly/server/internal/application"
	"github.com/simbafs/controly/server/internal/config"
	"github.com/simbafs/controly/server/internal/delivery"
	"github.com/simbafs/controly/server/internal/infrastructure"
)

//go:embed all:controller/*
var files embed.FS

func main() {
	// Initialize Configuration
	cfg := config.NewConfig()

	// Initialize Infrastructure Adapters
	displayRepo := infrastructure.NewInMemoryDisplayRepository()
	controllerRepo := infrastructure.NewInMemoryControllerRepository()
	commandFetcher := infrastructure.NewHTTPCommandFetcher()
	wsGateway := infrastructure.NewGorillaWebSocketGateway()
	idGenerator := infrastructure.NewBase58IDGenerator()

	// Initialize Use Cases
	displayRegistrationUC := &application.DisplayRegistrationUseCase{
		DisplayRepo:      displayRepo,
		CommandFetcher:   commandFetcher,
		WebSocketService: wsGateway,
		IDGenerator:      idGenerator,
		ServerToken:      cfg.Token,
	}

	displayDisconnectionUC := &application.DisplayDisconnectionUseCase{
		DisplayRepo:    displayRepo,
		ControllerRepo: controllerRepo,
		ConnManager:    wsGateway,
	}

	controllerConnectionUC := &application.ControllerConnectionUseCase{
		ControllerRepo:   controllerRepo,
		WebSocketService: wsGateway,
		IDGenerator:      idGenerator,
	}

	controllerDisconnectionUC := &application.ControllerDisconnectionUseCase{
		ControllerRepo: controllerRepo,
		DisplayRepo:    displayRepo,
		ConnManager:    wsGateway,
	}

	displayMessageHandlingUC := &application.DisplayMessageHandlingUseCase{
		DisplayRepo:      displayRepo,
		WebSocketService: wsGateway,
	}
	controllerMessageHandlingUC := &application.ControllerMessageHandlingUseCase{
		ControllerRepo:   controllerRepo,
		DisplayRepo:      displayRepo,
		WebSocketService: wsGateway,
	}

	// New delete use cases
	deleteDisplayUC := &application.DeleteDisplayUseCase{
		DisplayRepo:    displayRepo,
		ControllerRepo: controllerRepo,
		ConnManager:    wsGateway,
	}
	deleteControllerUC := &application.DeleteControllerUseCase{
		ControllerRepo: controllerRepo,
		DisplayRepo:    displayRepo,
		ConnManager:    wsGateway,
	}

	// Create dependencies struct for wsHandler
	wsHandler := delivery.NewWsHandler(
		displayRegistrationUC,
		displayDisconnectionUC,
		controllerConnectionUC,
		controllerDisconnectionUC,
		displayMessageHandlingUC,
		controllerMessageHandlingUC,
		wsGateway,
	)

	// Initialize ConnectionsHandler
	connectionsHandler := delivery.NewConnectionsHandler(
		displayRepo,
		controllerRepo,
	)

	// Initialize Delete Handlers
	deleteDisplayHandler := delivery.NewDeleteDisplayHandler(deleteDisplayUC)
	deleteControllerHandler := delivery.NewDeleteControllerHandler(deleteControllerUC)

	// Initialize Frontend Handler
	contentFs, err := fs.Sub(files, "controller/dist")
	if err != nil {
		panic(err)
	}
	frontendHandler := delivery.NewFrontendHandler(contentFs)

	// Setup router
	router := mux.NewRouter()

	// Register WebSocket route
	router.Handle("/ws", wsHandler)

	// Register REST API routes
	router.Handle("/api/connections", connectionsHandler).Methods("GET")
	router.Handle("/api/displays/{id}", deleteDisplayHandler).Methods("DELETE")
	router.Handle("/api/controllers/{id}", deleteControllerHandler).Methods("DELETE")

	// Serve embedded frontend files
	router.PathPrefix("/").Handler(frontendHandler)

	log.Println("Relay Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}
