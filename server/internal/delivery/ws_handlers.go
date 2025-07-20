package delivery

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

type WsHandler struct {
	registerDisplay             *application.RegisterDisplay
	handleDisplayDisconnection  *application.HandleDisplayDisconnection
	registerController          *application.RegisterController
	handleControllerDisconnection *application.HandleControllerDisconnection
	processDisplayMessage       *application.ProcessDisplayMessage
	processControllerMessage    *application.ProcessControllerMessage
	wsGateway                   *infrastructure.GorillaWebSocketGateway
}

func NewWsHandler(
	registerDisplay *application.RegisterDisplay,
	handleDisplayDisconnection *application.HandleDisplayDisconnection,
	registerController *application.RegisterController,
	handleControllerDisconnection *application.HandleControllerDisconnection,
	processDisplayMessage *application.ProcessDisplayMessage,
	processControllerMessage *application.ProcessControllerMessage,
	wsGateway *infrastructure.GorillaWebSocketGateway,
) *WsHandler {
	return &WsHandler{
		registerDisplay:             registerDisplay,
		handleDisplayDisconnection:  handleDisplayDisconnection,
		registerController:          registerController,
		handleControllerDisconnection: handleControllerDisconnection,
		processDisplayMessage:       processDisplayMessage,
		processControllerMessage:    processControllerMessage,
		wsGateway:                   wsGateway,
	}
}

func (h *WsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.wsGateway.Upgrade(w, r)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}
	// The connection is closed by the respective handlers

	params := r.URL.Query()
	clientType := params.Get("type")

	switch clientType {
	case "display":
		h.handleDisplayConnection(conn, params)
	case "controller":
		h.handleControllerConnection(conn)
	default:
		h.wsGateway.SendError("", domain.ErrInvalidClientType, "Invalid client type. Must be 'display' or 'controller'.")
		conn.Close()
		return
	}
}

func (h *WsHandler) handleDisplayConnection(conn *websocket.Conn, params url.Values) {
	// Defer closing the connection to ensure it's always closed on exit.
	defer conn.Close()

	displayIDParam := params.Get("id")
	commandURL := params.Get("command_url")
	token := params.Get("token")

	// Use case handles registration in both repo and gateway.
	displayID, err := h.registerDisplay.Execute(conn, displayIDParam, commandURL, token)
	if err != nil {
		log.Printf("Display registration failed: %v", err)
		// No cleanup needed as registration failed before any state was stored.
		return
	}

	// Send set_id message to the display
	setIDPayload, _ := json.Marshal(map[string]string{"id": displayID})
	// Here we use wsGateway directly as it's a simple message send, could be a use case too.
	h.wsGateway.SendMessage(displayID, "server", "set_id", setIDPayload)

	for {
		_, p, err := h.wsGateway.ReadMessage(conn)
		if err != nil {
			log.Printf("Display '%s' disconnected: %v", displayID, err)
			// A single call to the disconnection use case handles all cleanup.
			h.handleDisplayDisconnection.Execute(displayID)
			return
		}

		err = h.processDisplayMessage.Execute(displayID, p)
		if err != nil {
			log.Printf("Error handling display message from '%s': %v", displayID, err)
			// Optionally send an error message back to the display
			h.wsGateway.SendError(displayID, domain.ErrInvalidMessageFormat, "Invalid message format or type.")
		}
	}
}

func (h *WsHandler) handleControllerConnection(conn *websocket.Conn) {
	defer conn.Close()

	// Use case handles connection and registration.
	controllerID, err := h.registerController.Execute(conn)
	if err != nil {
		log.Printf("Controller connection failed: %v", err)
		// The use case should handle sending errors back to the client if registration fails.
		return
	}
	defer h.handleControllerDisconnection.Execute(controllerID)

	// No initial command_list sent here. Controller will subscribe and receive command_list.
	setIDPayload, _ := json.Marshal(map[string]string{"id": controllerID})
	// Send set_id message to the controller
	h.wsGateway.SendMessage(controllerID, "server", "set_id", setIDPayload)

	for {
		_, p, err := h.wsGateway.ReadMessage(conn)
		if err != nil {
			log.Printf("Controller '%s' disconnected: %v", controllerID, err)
			// A single call to the disconnection use case handles all cleanup.
			return
		}

		err = h.processControllerMessage.Execute(controllerID, p)
		if err != nil {
			log.Printf("Error handling controller message from '%s': %v", controllerID, err)
			// The use case should handle sending errors back to the controller.
		}
	}
}

// InspectorWsHandler handles WebSocket connections for the /ws/inspect endpoint.
type InspectorWsHandler struct {
	inspectorGateway *infrastructure.InspectorGateway
}

// NewInspectorWsHandler creates a new InspectorWsHandler.
func NewInspectorWsHandler(inspectorGateway *infrastructure.InspectorGateway) *InspectorWsHandler {
	return &InspectorWsHandler{
		inspectorGateway: inspectorGateway,
	}
}

// ServeHTTP upgrades the connection and registers it with the InspectorGateway.
// It then blocks, waiting for the connection to close.
func (h *InspectorWsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.inspectorGateway.Upgrade(w, r)
	if err != nil {
		log.Printf("Failed to upgrade inspector connection: %v", err)
		return
	}
	// Defer unregistering to ensure cleanup.
	defer h.inspectorGateway.Unregister(conn)

	h.inspectorGateway.Register(conn)

	// The inspector client is a passive listener. We just need to keep the
	// connection alive and detect when it closes. Reading from the connection
	// is a standard way to do this. Any message received is ignored.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			// This error will be triggered on connection close.
			break // Exit the loop, defer will handle unregistering.
		}
	}
}
