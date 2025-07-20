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
	inspectorGateway := infrastructure.NewInspectorGateway()
	wsGateway := infrastructure.NewGorillaWebSocketGateway(inspectorGateway)
	idGenerator := infrastructure.NewBase58IDGenerator()

	// Initialize Use Cases
	registerDisplay := &application.RegisterDisplay{
		DisplayRepo:      displayRepo,
		CommandFetcher:   commandFetcher,
		WebSocketService: wsGateway,
		IDGenerator:      idGenerator,
		ServerToken:      cfg.Token,
	}

	handleDisplayDisconnection := &application.HandleDisplayDisconnection{
		DisplayRepo:    displayRepo,
		ControllerRepo: controllerRepo,
		ConnManager:    wsGateway,
	}

	registerController := &application.RegisterController{
		ControllerRepo:   controllerRepo,
		WebSocketService: wsGateway,
		IDGenerator:      idGenerator,
	}

	handleControllerDisconnection := &application.HandleControllerDisconnection{
		ControllerRepo: controllerRepo,
		DisplayRepo:    displayRepo,
		ConnManager:    wsGateway,
	}

	processDisplayMessage := &application.ProcessDisplayMessage{
		DisplayRepo:      displayRepo,
		WebSocketService: wsGateway,
	}
	processControllerMessage := &application.ProcessControllerMessage{
		ControllerRepo:   controllerRepo,
		DisplayRepo:      displayRepo,
		WebSocketService: wsGateway,
	}

	deleteDisplay := &application.DeleteDisplay{
		DisplayRepo:    displayRepo,
		ControllerRepo: controllerRepo,
		ConnManager:    wsGateway,
	}
	deleteController := &application.DeleteController{
		ControllerRepo: controllerRepo,
		DisplayRepo:    displayRepo,
		ConnManager:    wsGateway,
	}

	// Create dependencies struct for wsHandler
	wsHandler := delivery.NewWsHandler(
		registerDisplay,
		handleDisplayDisconnection,
		registerController,
		handleControllerDisconnection,
		processDisplayMessage,
		processControllerMessage,
		wsGateway,
	)

	// Initialize ConnectionsHandler
	connectionsHandler := delivery.NewConnectionsHandler(
		displayRepo,
		controllerRepo,
	)

	// Initialize Delete Handlers
	deleteDisplayHandler := delivery.NewDeleteDisplayHandler(deleteDisplay)
	deleteControllerHandler := delivery.NewDeleteControllerHandler(deleteController)

	// Initialize Inspector Handler
	inspectorWsHandler := delivery.NewInspectorWsHandler(inspectorGateway)

	// Initialize Frontend Handler
	contentFs, err := fs.Sub(files, "controller/dist")
	if err != nil {
		panic(err)
	}
	frontendHandler := delivery.NewFrontendHandler(contentFs)

	// Setup router
	router := mux.NewRouter()

	// Register WebSocket routes
	router.Handle("/ws", wsHandler)
	router.Handle("/ws/inspect", inspectorWsHandler)

	// Register REST API routes
	router.Handle("/api/connections", connectionsHandler).Methods("GET")
	router.Handle("/api/displays/{id}", deleteDisplayHandler).Methods("DELETE")
	router.Handle("/api/controllers/{id}", deleteControllerHandler).Methods("DELETE")

	// Serve embedded frontend files
	router.PathPrefix("/").Handler(frontendHandler)

	log.Printf("Relay Server started on %s", cfg.Addr)
	log.Fatal(http.ListenAndServe(cfg.Addr, router))
}
