package domain

import "encoding/json"

// WebSocketMessage represents the generic WebSocket message format
type WebSocketMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// ErrorPayload represents the payload for an error message
type ErrorPayload struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
