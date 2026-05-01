package plugin

import (
	"fmt"
	"sync"
)

// ---------------------------------------------------------------------------
// Hook constants
// ---------------------------------------------------------------------------

const (
	HookPaymentNotifySuccess      = "payment.notify.success"
	HookOrderCreateBefore         = "order.create.before"
	HookOrderCreateAfter          = "order.create.after"
	HookOrderOpenBefore           = "order.open.before"
	HookOrderOpenAfter            = "order.open.after"
	HookUserRegisterAfter         = "user.register.after"
	HookAvailablePaymentMethods   = "available_payment_methods"
	HookTicketCreateAfter         = "ticket.create.after"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// Listener is a callback that receives event arguments without returning a value.
type Listener func(args ...interface{})

// Filter is a callback that receives a result and arguments, and returns a
// (possibly modified) result.
type Filter func(result interface{}, args ...interface{}) interface{}

// HookManager provides a thread-safe event and filter registration/dispatch
// system, mirroring the PHP HookManager pattern.
type HookManager struct {
	mu        sync.RWMutex
	listeners map[string][]Listener
	filters   map[string][]Filter
}

// NewHookManager creates a new empty HookManager.
func NewHookManager() *HookManager {
	return &HookManager{
		listeners: make(map[string][]Listener),
		filters:   make(map[string][]Filter),
	}
}

// Global singleton for simple access patterns.
var globalHookManager = NewHookManager()

// ---------------------------------------------------------------------------
// Event registration & dispatch
// ---------------------------------------------------------------------------

// Listen registers a Listener for the given event.
func (hm *HookManager) Listen(event string, callback Listener) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.listeners[event] = append(hm.listeners[event], callback)
}

// Dispatch triggers all listeners registered for the given event, passing the
// provided arguments. Each listener is called sequentially; if one panics, the
// panic is recovered and wrapped in an error message, but execution continues.
func (hm *HookManager) Dispatch(event string, args ...interface{}) {
	hm.mu.RLock()
	cbs := hm.listeners[event]
	hm.mu.RUnlock()

	for _, cb := range cbs {
		safeCall(func() {
			cb(args...)
		}, event)
	}
}

// ---------------------------------------------------------------------------
// Filter registration & application
// ---------------------------------------------------------------------------

// Register registers a Filter callback for the given filter name.
func (hm *HookManager) Register(filter string, callback Filter) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.filters[filter] = append(hm.filters[filter], callback)
}

// ApplyFilter runs all registered filters for the given name sequentially.
// Each filter receives the output of the previous one as its result argument.
// The initial result is the `result` parameter, and the final result is returned.
func (hm *HookManager) ApplyFilter(filter string, result interface{}, args ...interface{}) interface{} {
	hm.mu.RLock()
	cbs := hm.filters[filter]
	hm.mu.RUnlock()

	current := result
	for _, cb := range cbs {
		current = safeFilterCall(cb, current, filter, args...)
	}
	return current
}

// ---------------------------------------------------------------------------
// Global convenience functions
// ---------------------------------------------------------------------------

// Listen registers a global Listener.
func Listen(event string, callback Listener) {
	globalHookManager.Listen(event, callback)
}

// Dispatch triggers global event dispatch.
func Dispatch(event string, args ...interface{}) {
	globalHookManager.Dispatch(event, args...)
}

// RegisterFilter registers a global Filter.
func RegisterFilter(filter string, callback Filter) {
	globalHookManager.Register(filter, callback)
}

// ApplyFilter applies global filters.
func ApplyFilter(filter string, result interface{}, args ...interface{}) interface{} {
	return globalHookManager.ApplyFilter(filter, result, args...)
}

// ---------------------------------------------------------------------------
// Inspect
// ---------------------------------------------------------------------------

// HasEvent returns true if there are any listeners for the given event.
func (hm *HookManager) HasEvent(event string) bool {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return len(hm.listeners[event]) > 0
}

// HasFilter returns true if there are any filters registered for the given name.
func (hm *HookManager) HasFilter(filter string) bool {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return len(hm.filters[filter]) > 0
}

// ListenerCount returns the number of listeners for a given event.
func (hm *HookManager) ListenerCount(event string) int {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return len(hm.listeners[event])
}

// FilterCount returns the number of filters for a given name.
func (hm *HookManager) FilterCount(filter string) int {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return len(hm.filters[filter])
}

// RemoveAllListeners removes all listeners for the given event.
func (hm *HookManager) RemoveAllListeners(event string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	delete(hm.listeners, event)
}

// RemoveAllFilters removes all filters for the given name.
func (hm *HookManager) RemoveAllFilters(filter string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	delete(hm.filters, filter)
}

// GetHookManager returns the global HookManager singleton.
func GetHookManager() *HookManager {
	return globalHookManager
}

// InitHookManager creates a fresh global HookManager, discarding any existing one.
func InitHookManager() {
	globalHookManager = NewHookManager()
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// safeCall executes fn inside a recovery wrapper. If fn panics, the panic is
// caught and formatted as an error message referencing the hook name.
func safeCall(fn func(), hookName string) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("hook: panic recovered in %q: %v\n", hookName, r)
		}
	}()
	fn()
}

// safeFilterCall wraps a Filter call in a panic recovery.
func safeFilterCall(cb Filter, current interface{}, filter string, args ...interface{}) (ret interface{}) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("hook: panic recovered in filter %q: %v\n", filter, r)
			ret = current // return the previous value on panic
		}
	}()
	return cb(current, args...)
}
