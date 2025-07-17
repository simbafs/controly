package application

import (
	"encoding/json"

	"github.com/gorilla/websocket"
	"github.com/simbafs/controly/server/internal/domain"
)

// sendErrorToConn sends an error message directly to the given WebSocket connection.
func sendErrorToConn(conn *websocket.Conn, code int, message string) {
	payload, _ := json.Marshal(domain.ErrorPayload{Code: code, Message: message})
	msg := domain.OutgoingMessage{
		Type:    "error",
		From:    "server",
		Payload: payload,
	}

	conn.WriteJSON(msg)
}
