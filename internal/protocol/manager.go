package protocol

import (
	"fmt"
	"strings"
	"sync"
)

// Flag represents a protocol identification flag
type Flag struct {
	Name      string `json:"name"`
	Flag      string `json:"flag"`
	Attribute int    `json:"attribute"`
}

// Protocol defines the interface for all protocol implementations
type Protocol interface {
	// Flags returns protocol identification flags
	Flags() []Flag
	// GenerateConfig creates server-side config for a single server
	GenerateConfig(user *ClientInfo, server *ServerConfig) interface{}
}

// ClientInfo holds user information for config generation
type ClientInfo struct {
	ID             uint          `json:"id"`
	UUID           string        `json:"uuid"`
	SpeedLimit     int           `json:"speed_limit"`
	DeviceLimit    int           `json:"device_limit"`
	Email          string        `json:"email"`
	TransferEnable int64         `json:"transfer_enable"`
	U              int64         `json:"u"`
	D              int64         `json:"d"`
	ExpiredAt      int64         `json:"expired_at"`
	NodeList       []interface{} `json:"node_list,omitempty"`
}

// ServerConfig holds server configuration for config generation
type ServerConfig struct {
	ID            uint                   `json:"id"`
	Name          string                 `json:"name"`
	Host          string                 `json:"host"`
	Port          int                    `json:"port"`
	ServerPort    int                    `json:"server_port"`
	PortRange     string                 `json:"port_range,omitempty"`
	ServerKey     string                 `json:"server_key,omitempty"`
	Password      string                 `json:"password,omitempty"`
	Cipher        string                 `json:"cipher,omitempty"`
	Rate          float64                `json:"rate"`
	Network       string                 `json:"network,omitempty"`
	Protocol      string                 `json:"protocol,omitempty"`
	TLS           bool                   `json:"tls"`
	TLSProvider   string                 `json:"tls_provider,omitempty"`
	TLSHost       string                 `json:"tls_host,omitempty"`
	Reality       bool                   `json:"reality,omitempty"`
	RealityConfig map[string]interface{} `json:"reality_config,omitempty"`
	Flow          string                 `json:"flow,omitempty"`
	CustomConfig  map[string]interface{} `json:"custom_config,omitempty"`
	TrafficUsed   int64                  `json:"traffic_used"`
	TrafficLimit  int64                  `json:"traffic_limit"`
	RouteIDs      []uint                 `json:"route_ids,omitempty"`
}

// ProtocolManager manages protocol discovery and matching
type ProtocolManager struct {
	mu       sync.RWMutex
	registry map[string]Protocol
}

// NewProtocolManager creates a new protocol manager
func NewProtocolManager() *ProtocolManager {
	return &ProtocolManager{
		registry: make(map[string]Protocol),
	}
}

// Register registers a protocol implementation
func (pm *ProtocolManager) Register(name string, p Protocol) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.registry[name] = p
}

// GetProtocol returns a protocol by name
func (pm *ProtocolManager) GetProtocol(name string) Protocol {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.registry[name]
}

// GetAllFlags aggregates flags from all registered protocols
func (pm *ProtocolManager) GetAllFlags() []Flag {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var allFlags []Flag
	for name, p := range pm.registry {
		flags := p.Flags()
		for _, f := range flags {
			if f.Name == "" {
				f.Name = name
			}
			allFlags = append(allFlags, f)
		}
	}
	return allFlags
}

// MatchProtocol matches a flag string to a protocol (longest flag match wins)
func (pm *ProtocolManager) MatchProtocol(flag string) Protocol {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var bestMatch Protocol
	longestLen := 0

	flagLower := strings.ToLower(flag)

	for name, p := range pm.registry {
		flags := p.Flags()
		for _, f := range flags {
			fLower := strings.ToLower(f.Flag)
			if strings.Contains(flagLower, fLower) {
				if len(fLower) > longestLen {
					longestLen = len(fLower)
					bestMatch = p
				}
			}
		}
		_ = name
	}
	return bestMatch
}

// Global registry for package-level registration
var (
	globalRegistry   = make(map[string]Protocol)
	globalRegistryMu sync.RWMutex
)

// Register registers a protocol in the global registry (for use in init())
func Register(name string, p Protocol) {
	globalRegistryMu.Lock()
	defer globalRegistryMu.Unlock()
	globalRegistry[name] = p
}

// Get returns a protocol from the global registry
func Get(name string) Protocol {
	globalRegistryMu.RLock()
	defer globalRegistryMu.RUnlock()
	return globalRegistry[name]
}

// GetAllFlagsFromGlobal returns all flags from globally registered protocols
func GetAllFlagsFromGlobal() []Flag {
	globalRegistryMu.RLock()
	defer globalRegistryMu.RUnlock()

	var allFlags []Flag
	for name, p := range globalRegistry {
		flags := p.Flags()
		for _, f := range flags {
			if f.Name == "" {
				f.Name = name
			}
			allFlags = append(allFlags, f)
		}
	}
	return allFlags
}

// MatchProtocolFromGlobal matches a flag from the global registry
func MatchProtocolFromGlobal(flag string) Protocol {
	globalRegistryMu.RLock()
	defer globalRegistryMu.RUnlock()

	flagLower := strings.ToLower(flag)
	longestLen := 0
	var bestMatch Protocol

	for name, p := range globalRegistry {
		flags := p.Flags()
		for _, f := range flags {
			fLower := strings.ToLower(f.Flag)
			if strings.Contains(flagLower, fLower) && len(fLower) > longestLen {
				longestLen = len(fLower)
				bestMatch = p
			}
		}
		_ = name
	}
	return bestMatch
}

// LoadAllFromGlobal loads all globally registered protocols into the manager
func (pm *ProtocolManager) LoadAllFromGlobal() {
	globalRegistryMu.RLock()
	defer globalRegistryMu.RUnlock()

	for name, p := range globalRegistry {
		pm.Register(name, p)
	}
}

// String returns a string representation of the manager
func (pm *ProtocolManager) String() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	names := make([]string, 0, len(pm.registry))
	for name := range pm.registry {
		names = append(names, name)
	}
	return fmt.Sprintf("ProtocolManager{protocols: %s}", strings.Join(names, ", "))
}
