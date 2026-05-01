package model

import (
	"encoding/json"
	"time"
)

// Plugin represents an installed plugin
type Plugin struct {
	Model
	Code        string    `gorm:"type:varchar(64);uniqueIndex" json:"code"`
	Name        string    `gorm:"type:varchar(128)" json:"name"`
	Version     string    `gorm:"type:varchar(32)" json:"version"`
	Type        string    `gorm:"type:varchar(32)" json:"type"`
	IsEnabled   int       `gorm:"type:tinyint(1);default:0" json:"is_enabled"`
	Config      JSON      `gorm:"type:json" json:"config"`
	InstalledAt time.Time `gorm:"type:datetime" json:"installed_at"`
}

// GetConfig returns the config as a map
func (p *Plugin) GetConfig() map[string]interface{} {
	if p.Config == nil {
		return make(map[string]interface{})
	}
	return p.Config
}

// SetConfig stores config from a map
func (p *Plugin) SetConfig(config map[string]interface{}) error {
	if config == nil {
		p.Config = nil
		return nil
	}
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &p.Config)
}

// MarshalBinary implements encoding.BinaryMarshaler
func (p *Plugin) MarshalBinary() ([]byte, error) {
	return json.Marshal(p)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler
func (p *Plugin) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, p)
}

// TableName returns the table name
func (Plugin) TableName() string {
	return "v2_plugins"
}
