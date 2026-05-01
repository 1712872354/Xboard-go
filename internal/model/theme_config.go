package model

import "encoding/json"

// ThemeConfig represents the configuration of a theme
type ThemeConfig struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Author  string `json:"author"`
	Config  JSON   `gorm:"type:json" json:"config"`
}

// TableName returns the table name
func (ThemeConfig) TableName() string {
	return "v2_theme_configs"
}

// GetConfig returns the config as a map
func (t *ThemeConfig) GetConfig() map[string]interface{} {
	if t.Config == nil {
		return make(map[string]interface{})
	}
	return t.Config
}

// SetConfig stores config from a map
func (t *ThemeConfig) SetConfig(config map[string]interface{}) error {
	if config == nil {
		t.Config = nil
		return nil
	}
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &t.Config)
}
