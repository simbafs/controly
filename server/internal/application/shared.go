package application

import (
	"encoding/json"
	"log"

	"github.com/gorilla/websocket"
	"github.com/simbafs/controly/server/internal/domain"
	"github.com/simbafs/controly/server/internal/infrastructure"
)

// IDGenerator defines the interface for generating unique IDs.
type IDGenerator interface {
	GenerateUniqueDisplayID(checker infrastructure.DisplayExistenceChecker) (string, error)
}

// Global counter for simple controller ID generation (temporary, for build fix)
var controllerIDCounter int

// sendErrorToConn is a utility function to send an error message over a WebSocket connection.
// It should be used when an error occurs before a client is fully registered in the system.
func sendErrorToConn(conn *websocket.Conn, errCode int, message string) {
	// This is a low-level send, not using the full ClientNotifier interface
	// because the connection might not be registered yet.

	payload, err := json.Marshal(domain.ErrorPayload{
		Code:    errCode,
		Message: message,
	})
	if err != nil {
		log.Printf("Error marshalling error payload: %v", err)
		return
	}

	conn.WriteJSON(domain.OutgoingMessage{
		Type:    "error",
		From:    "server",
		Payload: payload,
	})
}
