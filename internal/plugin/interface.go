package plugin

// PluginType defines the category of a plugin
type PluginType string

const (
	TypePayment     PluginType = "payment"
	TypeFeature     PluginType = "feature"
	TypeNotify      PluginType = "notification"
)

// Plugin is the core interface all plugins must implement
type Plugin interface {
	// Code returns unique plugin identifier (e.g., "alipay_f2f")
	Code() string
	// Name returns human-readable plugin name
	Name() string
	// Version returns plugin version
	Version() string
	// Description returns plugin description
	Description() string
	// Author returns plugin author
	Author() string
	// Type returns plugin type
	Type() PluginType
	// Boot is called when the plugin is initialized
	Boot(ctx *Context) error
	// Install is called when plugin is first installed
	Install() error
	// Uninstall is called when plugin is removed
	Uninstall() error
	// Update handles version migration (oldVersion -> newVersion)
	Update(oldVersion, newVersion string) error
}

// PaymentPlugin extends Plugin for payment gateways
type PaymentPlugin interface {
	Plugin
	// Pay initiates a payment, returns payment URL or data
	Pay(order *PaymentOrder) (*PaymentResult, error)
	// Notify handles payment callback/notification
	Notify(request interface{}) (*PaymentNotification, error)
	// Form returns configuration form schema for admin panel
	Form() []FormField
}

// FeaturePlugin extends Plugin for feature/functionality plugins
type FeaturePlugin interface {
	Plugin
	// RegisterRoutes allows plugin to register HTTP routes
	RegisterRoutes(router interface{})
	// RegisterCommands allows plugin to register CLI commands
	RegisterCommands() []Command
}

// Context provides plugin runtime environment
type Context struct {
	Config  PluginConfig
	Logger  Logger
	Cache   Cache
	DB      interface{} // GORM DB
	Setting SettingStore
}

// PluginConfig holds plugin-specific configuration
type PluginConfig struct {
	Code        string                 `json:"code"`
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	Description string                 `json:"description"`
	Author      string                 `json:"author"`
	Type        PluginType             `json:"type"`
	Settings    map[string]interface{} `json:"settings"`
}

// PaymentOrder represents a payment request
type PaymentOrder struct {
	OrderID     uint
	TradeNo     string
	TotalAmount float64
	Subject     string
	Description string
	NotifyURL   string
	ReturnURL   string
	UserID      uint
	UserEmail   string
	Currency    string
}

// PaymentResult represents payment gateway response
type PaymentResult struct {
	Type           string // "redirect" | "url" | "form" | "qrcode"
	RedirectURL    string
	PayURL         string
	HTML           string
	QRCode         string
	TradeNo        string
	GatewayOrderID string
}

// PaymentNotification represents payment callback data
type PaymentNotification struct {
	TradeNo        string                 // System trade number
	GatewayTradeNo string                 // Gateway trade number
	Amount         float64
	Status         string                 // "success" | "failed"
	RawData        map[string]interface{}
}

// FormField represents a configuration field in admin panel
type FormField struct {
	Key         string        `json:"key"`
	Type        string        `json:"type"` // "text", "password", "number", "select", "switch"
	Label       string        `json:"label"`
	Description string        `json:"description"`
	Required    bool          `json:"required"`
	Default     interface{}   `json:"default"`
	Options     []FieldOption `json:"options,omitempty"`
}

// FieldOption represents a select option in a form field
type FieldOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Command represents a CLI command a plugin can register
type Command struct {
	Name        string
	Description string
	Handler     func(args []string) error
}

// Logger interface for plugin logging
type Logger interface {
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// Cache interface for plugin cache access
type Cache interface {
	Get(key string) (string, error)
	Set(key string, value string, ttl int) error
	Delete(key string) error
}

// SettingStore interface for plugin settings access
type SettingStore interface {
	Get(key string) (string, error)
	Set(key, value string) error
}
