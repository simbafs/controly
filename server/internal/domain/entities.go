package domain

import (
	"encoding/json"
	"sync"
	// sync "github.com/linkdata/deadlock"
)

// Display represents a connected Display device.
type Display struct {
	ID          string
	CommandList json.RawMessage // Store raw command.json content
	Subscribers map[string]bool // Map of Controller IDs subscribed to this Display
	Mu          sync.Mutex      // Mutex to protect access to Subscribers
}

func NewDisplay(id string, commandList json.RawMessage) *Display {
	return &Display{
		ID:          id,
		CommandList: commandList,
		Subscribers: make(map[string]bool),
	}
}

// Controller represents a connected Controller client.
type Controller struct {
	ID            string
	Subscriptions map[string]bool // Map of Display IDs this Controller is subscribed to
	Mu            sync.Mutex      // Mutex to protect access to Subscriptions
}

func NewController(id string) *Controller {
	return &Controller{
		ID:            id,
		Subscriptions: make(map[string]bool),
	}
}
