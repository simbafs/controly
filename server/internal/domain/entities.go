package domain

import (
	"encoding/json"
	"sync"
	// sync "github.com/linkdata/deadlock"
)

// ClientType defines the type of a connected client.
type ClientType int

const (
	_ ClientType = iota
	ClientTypeDisplay
	ClientTypeController
)

// String returns the string representation of a ClientType.
func (ct ClientType) String() string {
	switch ct {
	case ClientTypeDisplay:
		return "display"
	case ClientTypeController:
		return "controller"
	default:
		return "unknown"
	}
}

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

func (d *Display) RemoveSubscriber(controllerID string) {
	d.Mu.Lock()
	defer d.Mu.Unlock()
	delete(d.Subscribers, controllerID)
}

// Controller represents a connected Controller client.
type Controller struct {
	ID            string
	Subscriptions map[string]bool // Map of Display IDs this Controller is subscribed to
	WaitingFor    map[string]bool // Map of Display IDs this Controller is waiting for
	Mu            sync.Mutex      // Mutex to protect access to Subscriptions and WaitingFor
}

func NewController(id string) *Controller {
	return &Controller{
		ID:            id,
		Subscriptions: make(map[string]bool),
		WaitingFor:    make(map[string]bool),
	}
}

// SetWaitingList clears the existing waiting list and sets it to the new list of display IDs.
// It returns the final list of display IDs that were actually added to the waiting list.
func (c *Controller) SetWaitingList(displayIDs []string, isDisplayOnline func(string) bool) []string {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	c.WaitingFor = make(map[string]bool)
	finalWaitingList := []string{}

	for _, id := range displayIDs {
		if !isDisplayOnline(id) {
			c.WaitingFor[id] = true
			finalWaitingList = append(finalWaitingList, id)
		}
	}
	return finalWaitingList
}
