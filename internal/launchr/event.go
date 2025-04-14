package launchr

import (
	"sync/atomic"

	"github.com/gookit/event"
)

var eventDispatcher atomic.Pointer[event.Manager]

func init() {
	eventDispatcher.Store(event.NewManager("event_dispatcher"))
}

// EventDispatcher returns the global event dispatcher.
func EventDispatcher() *event.Manager {
	return eventDispatcher.Load()
}

// NewEvent returns new event instance.
func NewEvent(name string, data map[string]any) *event.BasicEvent {
	return event.New(name, data)
}
