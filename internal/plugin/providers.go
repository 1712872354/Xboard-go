package plugin

import "sync"

var (
	registry   = make(map[string]Plugin)
	registryMu sync.RWMutex
)

// Register adds a plugin to the global registry. Called during init() or app bootstrap.
// Built-in plugins use this; external plugins are loaded via configuration at runtime.
func Register(p Plugin) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[p.Code()] = p
}

// GetRegistered returns all registered plugins as a copy-safe map.
func GetRegistered() map[string]Plugin {
	registryMu.RLock()
	defer registryMu.RUnlock()
	result := make(map[string]Plugin, len(registry))
	for k, v := range registry {
		result[k] = v
	}
	return result
}

// GetRegisteredByCode returns a specific registered plugin by its code.
// Returns nil if no plugin with the given code is registered.
func GetRegisteredByCode(code string) Plugin {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registry[code]
}

// GetRegisteredByType returns all registered plugins of a given type.
func GetRegisteredByType(pt PluginType) []Plugin {
	registryMu.RLock()
	defer registryMu.RUnlock()
	var result []Plugin
	for _, p := range registry {
		if p.Type() == pt {
			result = append(result, p)
		}
	}
	return result
}

// RegisteredCount returns the number of registered plugins.
func RegisteredCount() int {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return len(registry)
}

// Unregister removes a plugin from the registry by code.
// Returns true if the plugin was found and removed.
func Unregister(code string) bool {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, ok := registry[code]; ok {
		delete(registry, code)
		return true
	}
	return false
}
