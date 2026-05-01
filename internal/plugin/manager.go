package plugin

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

var (
	// ErrPluginNotFound is returned when a plugin is not found.
	ErrPluginNotFound = fmt.Errorf("plugin not found")
	// ErrPluginAlreadyInstalled is returned when attempting to install an already-installed plugin.
	ErrPluginAlreadyInstalled = fmt.Errorf("plugin already installed")
	// ErrPluginNotInstalled is returned when operating on a plugin that is not installed.
	ErrPluginNotInstalled = fmt.Errorf("plugin not installed")
	// ErrPluginDependency is returned when a plugin dependency is missing.
	ErrPluginDependency = fmt.Errorf("plugin dependency not satisfied")
	// ErrPluginConfig is returned when plugin configuration is invalid.
	ErrPluginConfig = fmt.Errorf("plugin configuration error")
)

// PluginStatus represents the lifecycle status of a plugin.
type PluginStatus string

const (
	StatusDisabled PluginStatus = "disabled"
	StatusEnabled  PluginStatus = "enabled"
	StatusInstalled PluginStatus = "installed"
)

// PluginRecord represents a plugin's persisted state in the database.
type PluginRecord struct {
	Code       string       `json:"code"`
	Name       string       `json:"name"`
	Version    string       `json:"version"`
	Type       PluginType   `json:"type"`
	Status     PluginStatus `json:"status"`
	Settings   string       `json:"settings"` // JSON-encoded map[string]interface{}
	InstalledAt int64       `json:"installed_at"`
	UpdatedAt  int64        `json:"updated_at"`
}

// PluginStore defines the database operations the manager needs.
type PluginStore interface {
	// FindByCode retrieves a plugin record by its code.
	FindByCode(code string) (*PluginRecord, error)
	// Save persists a plugin record (insert or update).
	Save(record *PluginRecord) error
	// FindAll returns all plugin records.
	FindAll() ([]*PluginRecord, error)
	// Delete removes a plugin record by code.
	Delete(code string) error
}

// Manager handles the lifecycle of registered plugins.
type Manager struct {
	mu     sync.RWMutex
	store  PluginStore
	ctx    *Context

	// cached holds runtime state for installed+loaded plugins keyed by code.
	cached map[string]Plugin
}

// NewManager creates a new plugin manager with the given store and runtime context.
func NewManager(store PluginStore, ctx *Context) *Manager {
	return &Manager{
		store:  store,
		ctx:    ctx,
		cached: make(map[string]Plugin),
	}
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

// Initialize loads all enabled plugins from the database and boots them.
// It should be called during application startup.
func (m *Manager) Initialize() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	records, err := m.store.FindAll()
	if err != nil {
		return fmt.Errorf("failed to load plugin records: %w", err)
	}

	var firstErr error
	for _, rec := range records {
		if rec.Status != StatusEnabled {
			continue
		}
		p, ok := registry[rec.Code]
		if !ok {
			// Plugin was removed from the registry; skip.
			continue
		}
		if err := m.bootPluginLocked(p, rec); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

// Install installs a registered plugin by its code. It calls Install() on the
// plugin and persists the record. Returns ErrPluginAlreadyInstalled if the
// plugin is already recorded.
func (m *Manager) Install(code string) error {
	p := GetRegisteredByCode(code)
	if p == nil {
		return fmt.Errorf("%w: %s", ErrPluginNotFound, code)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, err := m.store.FindByCode(code)
	if err != nil {
		return fmt.Errorf("failed to check plugin existence: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("%w: %s", ErrPluginAlreadyInstalled, code)
	}

	if err := p.Install(); err != nil {
		return fmt.Errorf("install failed for %s: %w", code, err)
	}

	rec := &PluginRecord{
		Code:    p.Code(),
		Name:    p.Name(),
		Version: p.Version(),
		Type:    p.Type(),
		Status:  StatusInstalled,
	}

	if err := m.store.Save(rec); err != nil {
		return fmt.Errorf("failed to persist plugin record: %w", err)
	}

	return nil
}

// Uninstall removes a plugin from the database and calls Uninstall().
func (m *Manager) Uninstall(code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rec, err := m.store.FindByCode(code)
	if err != nil {
		return fmt.Errorf("failed to find plugin: %w", err)
	}
	if rec == nil {
		return fmt.Errorf("%w: %s", ErrPluginNotInstalled, code)
	}

	p, ok := registry[code]
	if ok {
		if err := p.Uninstall(); err != nil {
			return fmt.Errorf("uninstall failed for %s: %w", code, err)
		}
	}

	if err := m.store.Delete(code); err != nil {
		return fmt.Errorf("failed to delete plugin record: %w", err)
	}

	delete(m.cached, code)
	return nil
}

// Enable enables a previously installed plugin. If the plugin isn't yet cached
// it will be booted.
func (m *Manager) Enable(code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rec, err := m.store.FindByCode(code)
	if err != nil {
		return fmt.Errorf("failed to find plugin: %w", err)
	}
	if rec == nil {
		return fmt.Errorf("%w: %s", ErrPluginNotInstalled, code)
	}

	p, ok := registry[code]
	if !ok {
		return fmt.Errorf("%w: %s", ErrPluginNotFound, code)
	}

	rec.Status = StatusEnabled
	if err := m.store.Save(rec); err != nil {
		return fmt.Errorf("failed to update plugin status: %w", err)
	}

	return m.bootPluginLocked(p, rec)
}

// Disable disables a running plugin without removing it.
func (m *Manager) Disable(code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rec, err := m.store.FindByCode(code)
	if err != nil {
		return fmt.Errorf("failed to find plugin: %w", err)
	}
	if rec == nil {
		return fmt.Errorf("%w: %s", ErrPluginNotInstalled, code)
	}

	rec.Status = StatusDisabled
	if err := m.store.Save(rec); err != nil {
		return fmt.Errorf("failed to update plugin status: %w", err)
	}

	delete(m.cached, code)
	return nil
}

// Update performs a version upgrade for a plugin and persists the new version.
func (m *Manager) Update(code, newVersion string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rec, err := m.store.FindByCode(code)
	if err != nil {
		return fmt.Errorf("failed to find plugin: %w", err)
	}
	if rec == nil {
		return fmt.Errorf("%w: %s", ErrPluginNotInstalled, code)
	}

	p, ok := registry[code]
	if !ok {
		return fmt.Errorf("%w: %s", ErrPluginNotFound, code)
	}

	oldVersion := rec.Version
	if oldVersion == newVersion {
		return nil // no-op
	}

	if err := p.Update(oldVersion, newVersion); err != nil {
		return fmt.Errorf("update failed for %s: %w", code, err)
	}

	rec.Version = newVersion
	if err := m.store.Save(rec); err != nil {
		return fmt.Errorf("failed to persist version update: %w", err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Queries
// ---------------------------------------------------------------------------

// GetEnabledPaymentPlugins returns all currently enabled payment plugins.
func (m *Manager) GetEnabledPaymentPlugins() []PaymentPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []PaymentPlugin
	for _, p := range m.cached {
		if pp, ok := p.(PaymentPlugin); ok {
			result = append(result, pp)
		}
	}
	return result
}

// GetByType returns all cached (enabled) plugins of the specified type.
func (m *Manager) GetByType(pt PluginType) []Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Plugin
	for _, p := range m.cached {
		if p.Type() == pt {
			result = append(result, p)
		}
	}
	return result
}

// GetEnabled returns all enabled plugin codes.
func (m *Manager) GetEnabled() []Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Plugin, 0, len(m.cached))
	for _, p := range m.cached {
		result = append(result, p)
	}
	return result
}

// IsEnabled returns true if the given plugin code is currently enabled.
func (m *Manager) IsEnabled(code string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.cached[code]
	return ok
}

// GetPlugin returns the cached plugin instance for the given code.
func (m *Manager) GetPlugin(code string) Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cached[code]
}

// ---------------------------------------------------------------------------
// Config helpers
// ---------------------------------------------------------------------------

// GetConfigTypes returns the registered FormField definitions for a plugin.
// These describe the expected config schema for the admin panel.
func (m *Manager) GetConfigTypes(code string) ([]FormField, error) {
	p := GetRegisteredByCode(code)
	if p == nil {
		return nil, fmt.Errorf("%w: %s", ErrPluginNotFound, code)
	}
	if pp, ok := p.(PaymentPlugin); ok {
		return pp.Form(), nil
	}
	return nil, nil
}

// CastConfigValues converts a raw map[string]interface{} config into
// concrete Go types based on the plugin's FormField schema. For example,
// "number" fields are cast to float64, "switch" to bool.
func (m *Manager) CastConfigValues(code string, raw map[string]interface{}) (map[string]interface{}, error) {
	fields, err := m.GetConfigTypes(code)
	if err != nil {
		return nil, err
	}

	result := make(map[string]interface{}, len(raw))
	fieldMap := make(map[string]FormField, len(fields))
	for _, f := range fields {
		fieldMap[f.Key] = f
	}

	for key, val := range raw {
		ff, hasField := fieldMap[key]
		if !hasField {
			result[key] = val
			continue
		}

		cv, err := castValue(val, ff.Type)
		if err != nil {
			return nil, fmt.Errorf("%w: field %q: %v", ErrPluginConfig, key, err)
		}
		result[key] = cv
	}

	return result, nil
}

func castValue(val interface{}, fieldType string) (interface{}, error) {
	switch fieldType {
	case "number":
		switch v := val.(type) {
		case float64:
			return v, nil
		case json.Number:
			return v.Float64()
		case string:
			f, err := json.Number(v).Float64()
			if err != nil {
				return nil, fmt.Errorf("cannot parse as number: %s", v)
			}
			return f, nil
		default:
			return nil, fmt.Errorf("unexpected type %T for number field", val)
		}
	case "switch":
		switch v := val.(type) {
		case bool:
			return v, nil
		case string:
			return v == "true" || v == "1" || v == "on", nil
		case float64:
			return v != 0, nil
		default:
			return nil, fmt.Errorf("unexpected type %T for switch field", val)
		}
	default:
		return fmt.Sprintf("%v", val), nil
	}
}

// ---------------------------------------------------------------------------
// Private helpers
// ---------------------------------------------------------------------------

// bootPluginLocked boots a plugin (calls Boot) and caches it.
// Caller MUST hold m.mu write lock.
func (m *Manager) bootPluginLocked(p Plugin, rec *PluginRecord) error {
	// Merge persisted settings into the context config
	settings := make(map[string]interface{})
	if rec.Settings != "" {
		if err := json.Unmarshal([]byte(rec.Settings), &settings); err != nil {
			return fmt.Errorf("failed to decode settings for %s: %w", rec.Code, err)
		}
	}

	ctx := *m.ctx
	ctx.Config = PluginConfig{
		Code:        rec.Code,
		Name:        rec.Name,
		Version:     rec.Version,
		Description: p.Description(),
		Author:      p.Author(),
		Type:        rec.Type,
		Settings:    settings,
	}

	if err := p.Boot(&ctx); err != nil {
		return fmt.Errorf("failed to boot plugin %s: %w", rec.Code, err)
	}

	m.cached[rec.Code] = p
	return nil
}

// ---------------------------------------------------------------------------
// Dependency helpers
// ---------------------------------------------------------------------------

// sortByDependency performs a topological sort of plugin codes based on their
// declared dependencies. The dependencies are expressed as an adjacency list
// where deps[code] lists the codes that 'code' depends on.
func sortByDependency(deps map[string][]string) ([]string, error) {
	inDegree := make(map[string]int, len(deps))
	graph := make(map[string][]string)

	for node, depList := range deps {
		if _, ok := inDegree[node]; !ok {
			inDegree[node] = 0
		}
		for _, dep := range depList {
			graph[dep] = append(graph[dep], node)
			inDegree[node]++
		}
	}

	// Collect nodes with in-degree zero
	var queue []string
	for node := range deps {
		if inDegree[node] == 0 {
			queue = append(queue, node)
		}
	}
	sort.Strings(queue)

	var sorted []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		sorted = append(sorted, node)

		for _, neighbor := range graph[node] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
		sort.Strings(queue)
	}

	if len(sorted) != len(deps) {
		return nil, fmt.Errorf("circular dependency detected: %w", ErrPluginDependency)
	}

	return sorted, nil
}

// ---------------------------------------------------------------------------
// PluginStore implementations
// ---------------------------------------------------------------------------

// MemoryPluginStore is an in-memory implementation of PluginStore for testing.
type MemoryPluginStore struct {
	mu      sync.RWMutex
	records map[string]*PluginRecord
	order   []string
}

func NewMemoryPluginStore() *MemoryPluginStore {
	return &MemoryPluginStore{
		records: make(map[string]*PluginRecord),
	}
}

func (s *MemoryPluginStore) FindByCode(code string) (*PluginRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.records[code]
	if !ok {
		return nil, nil
	}
	cp := *r
	return &cp, nil
}

func (s *MemoryPluginStore) Save(rec *PluginRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *rec
	if _, ok := s.records[rec.Code]; !ok {
		s.order = append(s.order, rec.Code)
	}
	s.records[rec.Code] = &cp
	return nil
}

func (s *MemoryPluginStore) FindAll() ([]*PluginRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*PluginRecord, 0, len(s.order))
	for _, code := range s.order {
		if r, ok := s.records[code]; ok {
			cp := *r
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (s *MemoryPluginStore) Delete(code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.records, code)
	filtered := make([]string, 0, len(s.order))
	for _, c := range s.order {
		if c != code {
			filtered = append(filtered, c)
		}
	}
	s.order = filtered
	return nil
}

// Ensure MemoryPluginStore implements PluginStore.
var _ PluginStore = (*MemoryPluginStore)(nil)

// ---------------------------------------------------------------------------
// Stringify helpers
// ---------------------------------------------------------------------------

// PluginTypeStrings returns all registered plugin types as a comma-separated string.
func PluginTypeStrings() string {
	types := []string{string(TypePayment), string(TypeFeature), string(TypeNotify)}
	return strings.Join(types, ", ")
}
