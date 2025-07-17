package domain

import (
	"encoding/json"
	"sync"
)

// Display represents a connected Display device.
type Display struct {
	ID          string
	CommandList json.RawMessage // Store raw command.json content
	Subscribers map[string]bool // Map of Controller IDs subscribed to this Display
	Mu          sync.Mutex      // Mutex to protect access to Subscribers
}

// Controller represents a connected Controller client.
type Controller struct {
	ID            string
	Subscriptions map[string]bool // Map of Display IDs this Controller is subscribed to
	Mu            sync.Mutex      // Mutex to protect access to Subscriptions
}
