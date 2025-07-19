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
	displayRegistrationUC       *application.DisplayRegistrationUseCase
	displayDisconnectionUC      *application.DisplayDisconnectionUseCase
	controllerConnectionUC      *application.ControllerConnectionUseCase
	controllerDisconnectionUC   *application.ControllerDisconnectionUseCase
	displayMessageHandlingUC    *application.DisplayMessageHandlingUseCase
	controllerMessageHandlingUC *application.ControllerMessageHandlingUseCase
	wsGateway                   *infrastructure.GorillaWebSocketGateway
}

func NewWsHandler(displayRegistrationUC *application.DisplayRegistrationUseCase,
	displayDisconnectionUC *application.DisplayDisconnectionUseCase,
	controllerConnectionUC *application.ControllerConnectionUseCase,
	controllerDisconnectionUC *application.ControllerDisconnectionUseCase,
	displayMessageHandlingUC *application.DisplayMessageHandlingUseCase,
	controllerMessageHandlingUC *application.ControllerMessageHandlingUseCase,
	wsGateway *infrastructure.GorillaWebSocketGateway,
) *WsHandler {
	return &WsHandler{
		displayRegistrationUC:       displayRegistrationUC,
		displayDisconnectionUC:      displayDisconnectionUC,
		controllerConnectionUC:      controllerConnectionUC,
		controllerDisconnectionUC:   controllerDisconnectionUC,
		displayMessageHandlingUC:    displayMessageHandlingUC,
		controllerMessageHandlingUC: controllerMessageHandlingUC,
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
	displayID, err := h.displayRegistrationUC.Execute(conn, displayIDParam, commandURL, token)
	if err != nil {
		log.Printf("Display registration failed: %v", err)
		// No cleanup needed as registration failed before any state was stored.
		return
	}

	// Send set_id message to the display
	setIDPayload, _ := json.Marshal(map[string]string{"id": displayID})
	// Here we use wsGateway directly as it's a simple message send, could be a use case too.
	err = h.wsGateway.SendMessage(displayID, "server", "set_id", setIDPayload)
	if err != nil {
		log.Printf("Failed to send set_id to display '%s': %v", displayID, err)
		// If we can't send the ID, the display is useless. Trigger disconnection logic.
		h.displayDisconnectionUC.Execute(displayID)
		return
	}

	for {
		_, p, err := h.wsGateway.ReadMessage(conn)
		if err != nil {
			log.Printf("Display '%s' disconnected: %v", displayID, err)
			// A single call to the disconnection use case handles all cleanup.
			h.displayDisconnectionUC.Execute(displayID)
			return
		}

		err = h.displayMessageHandlingUC.Execute(displayID, p)
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
	controllerID, err := h.controllerConnectionUC.Execute(conn)
	if err != nil {
		log.Printf("Controller connection failed: %v", err)
		// The use case should handle sending errors back to the client if registration fails.
		return
	}
	defer h.controllerDisconnectionUC.Execute(controllerID)

	// No initial command_list sent here. Controller will subscribe and receive command_list.
	setIDPayload, _ := json.Marshal(map[string]string{"id": controllerID})
	// Send set_id message to the controller
	err = h.wsGateway.SendMessage(controllerID, "server", "set_id", setIDPayload)
	if err != nil {
		log.Printf("Failed to send set_id to controller '%s': %v", controllerID, err)
		// If we can't send the ID, the controller is useless. Trigger disconnection logic.
		return
	}

	for {
		_, p, err := h.wsGateway.ReadMessage(conn)
		if err != nil {
			log.Printf("Controller '%s' disconnected: %v", controllerID, err)
			// A single call to the disconnection use case handles all cleanup.
			return
		}

		err = h.controllerMessageHandlingUC.Execute(controllerID, p)
		if err != nil {
			log.Printf("Error handling controller message from '%s': %v", controllerID, err)
			// The use case should handle sending errors back to the controller.
		}
	}
}
