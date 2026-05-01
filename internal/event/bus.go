package event

import (
	"fmt"
	"log"
	"sync"
)

// ---------------------------------------------------------------------------
// Event types
// ---------------------------------------------------------------------------

// Event constants are defined in events.go

// Event represents a domain event
type Event struct {
	Type string
	Data map[string]interface{}
}

// Listener is a callback that processes events
type Listener func(event Event)

// ---------------------------------------------------------------------------
// EventBus — thread-safe publish/subscribe
// ---------------------------------------------------------------------------

// EventBus provides a simple in-process event bus
type EventBus struct {
	mu        sync.RWMutex
	listeners map[string][]Listener
}

var globalBus = NewEventBus()

// NewEventBus creates a new event bus
func NewEventBus() *EventBus {
	return &EventBus{
		listeners: make(map[string][]Listener),
	}
}

// Subscribe registers a listener for an event type
func (bus *EventBus) Subscribe(eventType string, listener Listener) {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	bus.listeners[eventType] = append(bus.listeners[eventType], listener)
}

// Publish dispatches an event to all registered listeners asynchronously
func (bus *EventBus) Publish(event Event) {
	bus.mu.RLock()
	listeners := make([]Listener, 0, len(bus.listeners[event.Type]))
	listeners = append(listeners, bus.listeners[event.Type]...)
	bus.mu.RUnlock()

	for _, listener := range listeners {
		go func(l Listener) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[EventBus] panic in listener for %s: %v", event.Type, r)
				}
			}()
			l(event)
		}(listener)
	}
}

// HasListeners returns whether an event type has any listeners
func (bus *EventBus) HasListeners(eventType string) bool {
	bus.mu.RLock()
	defer bus.mu.RUnlock()
	return len(bus.listeners[eventType]) > 0
}

// ---------------------------------------------------------------------------
// Global convenience functions
// ---------------------------------------------------------------------------

// Subscribe registers a listener on the global event bus
func Subscribe(eventType string, listener Listener) {
	globalBus.Subscribe(eventType, listener)
}

// Publish dispatches an event on the global event bus
func Publish(eventType string, data map[string]interface{}) {
	globalBus.Publish(Event{Type: eventType, Data: data})
}

// GetBus returns the global event bus
func GetBus() *EventBus {
	return globalBus
}

// ---------------------------------------------------------------------------
// Built-in listeners
// ---------------------------------------------------------------------------

// RegisterBuiltinListeners sets up default event handlers
func RegisterBuiltinListeners() {
	// Order events
	Subscribe(EventOrderCreated, func(e Event) {
		log.Printf("[Event] Order created: trade_no=%s user_id=%v amount=%v",
			getStr(e.Data, "trade_no"),
			e.Data["user_id"],
			e.Data["total_amount"],
		)
	})

	Subscribe(EventOrderPaid, func(e Event) {
		log.Printf("[Event] Order paid: trade_no=%s amount=%v",
			getStr(e.Data, "trade_no"),
			e.Data["total_amount"],
		)
	})

	Subscribe(EventOrderOpened, func(e Event) {
		log.Printf("[Event] Order opened: trade_no=%s user_id=%v plan_id=%v",
			getStr(e.Data, "trade_no"),
			e.Data["user_id"],
			e.Data["plan_id"],
		)
	})

	Subscribe(EventUserRegistered, func(e Event) {
		log.Printf("[Event] User registered: user_id=%v email=%s",
			e.Data["user_id"],
			getStr(e.Data, "email"),
		)
	})

	Subscribe(EventTrafficReset, func(e Event) {
		log.Printf("[Event] Traffic reset: user_id=%v",
			e.Data["user_id"],
		)
	})

	Subscribe(EventTicketCreated, func(e Event) {
		log.Printf("[Event] Ticket created: ticket_id=%v user_id=%v subject=%s",
			e.Data["ticket_id"],
			e.Data["user_id"],
			getStr(e.Data, "subject"),
		)
	})
}

func getStr(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}
