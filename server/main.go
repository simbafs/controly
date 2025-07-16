package main

import (
	"log"
	"net/http"

	"github.com/simbafs/controly/server/internal/application"
	"github.com/simbafs/controly/server/internal/delivery" // New import
	"github.com/simbafs/controly/server/internal/infrastructure"
)

// wsHandlerDependencies holds the dependencies for the wsHandler
type wsHandlerDependencies struct {
	displayRegistrationUC       *application.DisplayRegistrationUseCase
	displayDisconnectionUC      *application.DisplayDisconnectionUseCase
	controllerConnectionUC      *application.ControllerConnectionUseCase
	controllerDisconnectionUC   *application.ControllerDisconnectionUseCase
	displayMessageHandlingUC    *application.DisplayMessageHandlingUseCase
	controllerMessageHandlingUC *application.ControllerMessageHandlingUseCase
	wsGateway                   *infrastructure.GorillaWebSocketGateway
}

func main() {
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
	}

	displayDisconnectionUC := &application.DisplayDisconnectionUseCase{
		DisplayRepo:    displayRepo,
		ControllerRepo: controllerRepo,
		ConnManager:    wsGateway,
	}

	controllerConnectionUC := &application.ControllerConnectionUseCase{
		DisplayRepo:      displayRepo,
		ControllerRepo:   controllerRepo,
		WebSocketService: wsGateway,
	}

	controllerDisconnectionUC := &application.ControllerDisconnectionUseCase{
		ControllerRepo: controllerRepo,
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
	http.Handle("/ws", wsHandler)

	// New: Initialize ConnectionsHandler and register its method
	connectionsHandler := delivery.NewConnectionsHandler(
		displayRepo,
		controllerRepo,
	)
	http.Handle("/api/connections", connectionsHandler)

	log.Println("Relay Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
