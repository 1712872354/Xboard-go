package payment

import (
	"sync"

	"github.com/xboard/xboard/internal/plugin"
)

func init() {
	plugin.Register(&AlipayF2F{})
	plugin.Register(&Epay{})
	plugin.Register(&MGate{})
	plugin.Register(&Coinbase{})
	plugin.Register(&CoinPayments{})
	plugin.Register(&BTCPay{})
}

// ---------------------------------------------------------------------------
// Shared helpers for payment plugins
// ---------------------------------------------------------------------------

var (
	currentSettings = make(map[string]interface{})
	settingsMu      sync.RWMutex
)

// SetPluginSettings stores plugin settings (called by Boot or admin handler)
func SetPluginSettings(settings map[string]interface{}) {
	settingsMu.Lock()
	currentSettings = settings
	settingsMu.Unlock()
}

// getPluginSettings returns a copy of current plugin settings
func getPluginSettings() map[string]interface{} {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	cp := make(map[string]interface{}, len(currentSettings))
	for k, v := range currentSettings {
		cp[k] = v
	}
	return cp
}

// getString safely extracts a string from a settings map
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case string:
			return val
		case float64:
			return ""
		default:
			return ""
		}
	}
	return ""
}

// getStringMap extracts a string from map[string]interface{} by key
func getStringMap(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok2 := v.(string); ok2 {
			return s
		}
	}
	return ""
}

// getFloat64Map extracts a float64 from map[string]interface{} by key
func getFloat64Map(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case string:
			return 0
		default:
			return 0
		}
	}
	return 0
}
