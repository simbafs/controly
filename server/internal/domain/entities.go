package domain

import (
	"encoding/json"
	"sync"
)

// Display represents a connected Display device.
type Display struct {
	ID          string
	CommandList json.RawMessage // Store raw command.json content
	Controller  *Controller     // Pointer to the controlling Controller, if any
	Mu          sync.Mutex      // Mutex to protect access to Controller
}

// Controller represents a connected Controller client.
type Controller struct {
	ID          string
	TargetDisplay *Display // Pointer to the Display being controlled
}
