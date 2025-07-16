package main

import (
	"encoding/json"
	"fmt"
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
	controllerConnectionUC      *application.ControllerConnectionUseCase
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
	// Defer closing the connection here. Unregistering will happen in the specific handlers.
	// defer conn.Close()

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
	displayIDParam := params.Get("id")
	commandURL := params.Get("command_url")

	// Use the DisplayRegistrationUseCase
	displayID, err := deps.displayRegistrationUC.Execute(conn, displayIDParam, commandURL)
	if err != nil {
		log.Printf("Display registration failed: %v", err)
		conn.Close()
		return
	}

	deps.wsGateway.RegisterDisplayConnection(displayID, conn)
	defer func() {
		deps.wsGateway.UnregisterDisplayConnection(displayID)
		conn.Close() // Ensure the connection is closed when handler exits
		log.Printf("Display '%s' connection closed.", displayID)
	}()

	for {
		_, p, err := deps.wsGateway.ReadMessage(conn)
		if err != nil {
			log.Printf("Display '%s' disconnected: %v", displayID, err)
			// Clean up display and associated controller on disconnect
			displayIface, found := deps.displayRegistrationUC.DisplayRepo.FindByID(displayID)
			if found {
				actualDisplay := displayIface.(*domain.Display) // Type assertion
				actualDisplay.Mu.Lock()
				if actualDisplay.Controller != nil {
					// Notify controller that display disconnected
					deps.wsGateway.SendError(actualDisplay.Controller.ID, domain.ErrTargetDisplayNotFound, "Target display disconnected.")
					deps.wsGateway.UnregisterControllerConnection(actualDisplay.Controller.ID)
					deps.controllerConnectionUC.ControllerRepo.Delete(actualDisplay.Controller.ID)
					actualDisplay.Controller = nil
				}
				actualDisplay.Mu.Unlock()
				deps.displayRegistrationUC.DisplayRepo.Delete(displayID)
			}
			return
		}

		err = deps.displayMessageHandlingUC.Execute(displayID, p)
		if err != nil {
			log.Printf("Error handling display message from '%s': %v", displayID, err)
			// Send error back to display if message format is invalid
			var msg domain.WebSocketMessage
			if json.Unmarshal(p, &msg) == nil && msg.Type != "status" { // Only send error if it's not a status message (which is expected)
				deps.wsGateway.SendError(displayID, domain.ErrInvalidMessageFormat, fmt.Sprintf("Unknown message type or invalid format: %s", msg.Type))
			} else if json.Unmarshal(p, &msg) != nil { // If unmarshalling itself failed
				deps.wsGateway.SendError(displayID, domain.ErrInvalidMessageFormat, "Invalid message format.")
			}
		}
	}
}

func handleControllerConnection(deps *wsHandlerDependencies, conn *websocket.Conn, params url.Values) {
	targetID := params.Get("target_id")

	// Use the ControllerConnectionUseCase
	controllerID, err := deps.controllerConnectionUC.Execute(conn, targetID)
	if err != nil {
		log.Printf("Controller connection failed: %v", err)
		conn.Close()
		return
	}

	deps.wsGateway.RegisterControllerConnection(controllerID, conn)
	defer func() {
		deps.wsGateway.UnregisterControllerConnection(controllerID)
		conn.Close() // Ensure the connection is closed when handler exits
		log.Printf("Controller '%s' connection closed.", controllerID)
	}()

	for {
		_, p, err := deps.wsGateway.ReadMessage(conn)
		if err != nil {
			log.Printf("Controller '%s' disconnected: %v", controllerID, err)
			// Clean up controller and release display
			controller, found := deps.controllerConnectionUC.ControllerRepo.FindByID(controllerID)
			if found && controller.TargetDisplay != nil {
				controller.TargetDisplay.Mu.Lock()
				if controller.TargetDisplay.Controller == controller {
					controller.TargetDisplay.Controller = nil
				}
				controller.TargetDisplay.Mu.Unlock()
			}
			deps.controllerConnectionUC.ControllerRepo.Delete(controllerID)
			return
		}

		err = deps.controllerMessageHandlingUC.Execute(controllerID, p)
		if err != nil {
			log.Printf("Error handling controller message from '%s': %v", controllerID, err)
			// Send error back to controller
			var msg domain.WebSocketMessage
			if json.Unmarshal(p, &msg) == nil && msg.Type != "command" { // Only send error if it's not a command message (which is expected)
				deps.wsGateway.SendError(controllerID, domain.ErrInvalidMessageFormat, fmt.Sprintf("Unknown message type or invalid format: %s", msg.Type))
			} else if json.Unmarshal(p, &msg) != nil { // If unmarshalling itself failed
				deps.wsGateway.SendError(controllerID, domain.ErrInvalidMessageFormat, "Invalid message format.")
			} else if msg.Type == "command" { // Specific error for command forwarding issues
				deps.wsGateway.SendError(controllerID, domain.ErrInvalidCommandFormat, "Failed to forward command or no longer controlling display.")
			}
		}
	}
}

func main() {
	// Initialize Infrastructure Adapters
	displayRepo := infrastructure.NewInMemoryDisplayRepository()
	controllerRepo := infrastructure.NewInMemoryControllerRepository()
	commandFetcher := infrastructure.NewHTTPCommandFetcher()
	wsGateway := infrastructure.NewGorillaWebSocketGateway()

	// Initialize Use Cases
	// Initialize ID Generator
	idGenerator := infrastructure.NewBase58IDGenerator()

	// Initialize Use Cases
	displayRegistrationUC := &application.DisplayRegistrationUseCase{
		DisplayRepo:      displayRepo,
		CommandFetcher:   commandFetcher,
		WebSocketService: wsGateway, // WebSocketGateway implements WebSocketMessenger
		IDGenerator:      idGenerator,
	}
	controllerConnectionUC := &application.ControllerConnectionUseCase{
		DisplayRepo:      displayRepo,
		ControllerRepo:   controllerRepo,
		WebSocketService: wsGateway, // WebSocketGateway implements WebSocketMessenger
	}
	displayMessageHandlingUC := &application.DisplayMessageHandlingUseCase{
		DisplayRepo:      displayRepo,
		WebSocketService: wsGateway, // WebSocketGateway implements WebSocketMessenger
	}
	controllerMessageHandlingUC := &application.ControllerMessageHandlingUseCase{
		ControllerRepo:   controllerRepo,
		DisplayRepo:      displayRepo, // Needed for display.Controller access in cleanup
		WebSocketService: wsGateway,   // WebSocketGateway implements WebSocketMessenger
	}

	// Create dependencies struct for wsHandler
	deps := &wsHandlerDependencies{
		displayRegistrationUC:       displayRegistrationUC,
		controllerConnectionUC:      controllerConnectionUC,
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
