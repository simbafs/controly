package main

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"

	"github.com/simbafs/controly/server/internal/application"
	"github.com/simbafs/controly/server/internal/domain"
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

func wsHandler(deps *wsHandlerDependencies, w http.ResponseWriter, r *http.Request) {
	conn, err := deps.wsGateway.Upgrade(w, r)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}
	// The connection is closed by the respective handlers

	params := r.URL.Query()
	clientType := params.Get("type")

	switch clientType {
	case "display":
		handleDisplayConnection(deps, conn, params)
	case "controller":
		handleControllerConnection(deps, conn, params)
	default:
		deps.wsGateway.SendError("", domain.ErrInvalidClientType, "Invalid client type. Must be 'display' or 'controller'.")
		conn.Close()
		return
	}
}

func handleDisplayConnection(deps *wsHandlerDependencies, conn *websocket.Conn, params url.Values) {
	// Defer closing the connection to ensure it's always closed on exit.
	defer conn.Close()

	displayIDParam := params.Get("id")
	commandURL := params.Get("command_url")

	// Use case handles registration in both repo and gateway.
	displayID, err := deps.displayRegistrationUC.Execute(conn, displayIDParam, commandURL)
	if err != nil {
		log.Printf("Display registration failed: %v", err)
		// No cleanup needed as registration failed before any state was stored.
		return
	}

	// Send set_id message to the display
	setIDPayload, _ := json.Marshal(map[string]string{"id": displayID})
	// Here we use wsGateway directly as it's a simple message send, could be a use case too.
	err = deps.wsGateway.SendMessage(displayID, "set_id", setIDPayload)
	if err != nil {
		log.Printf("Failed to send set_id to display '%s': %v", displayID, err)
		// If we can't send the ID, the display is useless. Trigger disconnection logic.
		deps.displayDisconnectionUC.Execute(displayID)
		return
	}

	for {
		_, p, err := deps.wsGateway.ReadMessage(conn)
		if err != nil {
			log.Printf("Display '%s' disconnected: %v", displayID, err)
			// A single call to the disconnection use case handles all cleanup.
			deps.displayDisconnectionUC.Execute(displayID)
			return
		}

		err = deps.displayMessageHandlingUC.Execute(displayID, p)
		if err != nil {
			log.Printf("Error handling display message from '%s': %v", displayID, err)
			// Optionally send an error message back to the display
			deps.wsGateway.SendError(displayID, domain.ErrInvalidMessageFormat, "Invalid message format or type.")
		}
	}
}

func handleControllerConnection(deps *wsHandlerDependencies, conn *websocket.Conn, params url.Values) {
	defer conn.Close()

	targetID := params.Get("target_id")

	// Use case handles connection and registration.
	controllerID, display, err := deps.controllerConnectionUC.Execute(conn, targetID)
	if err != nil {
		log.Printf("Controller connection failed: %v", err)
		return
	}

	// Send command list to controller
	commandListMsgPayload := display.CommandList
	err = deps.wsGateway.SendMessage(controllerID, "command_list", commandListMsgPayload)
	if err != nil {
		log.Printf("Failed to send command_list to controller '%s': %v", controllerID, err)
		deps.controllerDisconnectionUC.Execute(controllerID) // Cleanup
		return
	}

	for {
		_, p, err := deps.wsGateway.ReadMessage(conn)
		if err != nil {
			log.Printf("Controller '%s' disconnected: %v", controllerID, err)
			// A single call to the disconnection use case handles all cleanup.
			deps.controllerDisconnectionUC.Execute(controllerID)
			return
		}

		err = deps.controllerMessageHandlingUC.Execute(controllerID, p)
		if err != nil {
			log.Printf("Error handling controller message from '%s': %v", controllerID, err)
			// Send error back to controller
			deps.wsGateway.SendError(controllerID, domain.ErrInvalidMessageFormat, "Invalid message format or type.")
		}
	}
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
	deps := &wsHandlerDependencies{
		displayRegistrationUC:       displayRegistrationUC,
		displayDisconnectionUC:      displayDisconnectionUC,
		controllerConnectionUC:      controllerConnectionUC,
		controllerDisconnectionUC:   controllerDisconnectionUC,
		displayMessageHandlingUC:    displayMessageHandlingUC,
		controllerMessageHandlingUC: controllerMessageHandlingUC,
		wsGateway:                   wsGateway,
	}

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		wsHandler(deps, w, r)
	})

	log.Println("Relay Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
