package domain

import "encoding/json"

// IncomingMessage represents a message received from a client.
type IncomingMessage struct {
	Type    string          `json:"type"`
	To      string          `json:"to,omitempty"` // e.g., for 'command' messages
	Payload json.RawMessage `json:"payload"`
}

// OutgoingMessage represents a message sent from the server.
type OutgoingMessage struct {
	Type    string          `json:"type"`
	From    string          `json:"from,omitempty"` // Source (e.g., a display ID, or "server")
	Payload json.RawMessage `json:"payload"`
}

// ErrorPayload represents the payload for an error message
type ErrorPayload struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// SubscribedPayload represents the payload for a "subscribed" message
type SubscribedPayload struct {
	Count int `json:"count"`
}

// UnsubscribedPayload represents the payload for an "unsubscribed" message
type UnsubscribedPayload struct {
	Count int `json:"count"`
}

// DisplayDisconnectedPayload represents the payload for a "display_disconnected" message
type DisplayDisconnectedPayload struct {
	DisplayID string `json:"display_id"`
}
