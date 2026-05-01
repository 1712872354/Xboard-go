package plugin

import (
	"encoding/json"
	"time"

	pluginModel "github.com/xboard/xboard/internal/model"
	"gorm.io/gorm"
)

// GORMPluginStore implements PluginStore using GORM and model.Plugin
type GORMPluginStore struct {
	db *gorm.DB
}

func NewGORMPluginStore(db *gorm.DB) *GORMPluginStore {
	return &GORMPluginStore{db: db}
}

func (s *GORMPluginStore) FindByCode(code string) (*PluginRecord, error) {
	var p pluginModel.Plugin
	err := s.db.Where("code = ?", code).First(&p).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return toRecord(&p), nil
}

func (s *GORMPluginStore) Save(record *PluginRecord) error {
	var existing pluginModel.Plugin
	err := s.db.Where("code = ?", record.Code).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return s.db.Create(fromRecord(record)).Error
	}
	if err != nil {
		return err
	}

	updates := map[string]interface{}{
		"name":       record.Name,
		"version":    record.Version,
		"type":       string(record.Type),
		"is_enabled": 0,
		"updated_at": time.Now(),
	}
	if record.Status == StatusEnabled {
		updates["is_enabled"] = 1
	}
	return s.db.Model(&existing).Updates(updates).Error
}

func (s *GORMPluginStore) FindAll() ([]*PluginRecord, error) {
	var plugins []pluginModel.Plugin
	if err := s.db.Find(&plugins).Error; err != nil {
		return nil, err
	}
	records := make([]*PluginRecord, len(plugins))
	for i, p := range plugins {
		records[i] = toRecord(&p)
	}
	return records, nil
}

func (s *GORMPluginStore) Delete(code string) error {
	return s.db.Where("code = ?", code).Delete(&pluginModel.Plugin{}).Error
}

func toRecord(p *pluginModel.Plugin) *PluginRecord {
	status := StatusDisabled
	if p.IsEnabled == 1 {
		status = StatusEnabled
	}
	settings := "{}"
	if p.Config != nil {
		if b, err := json.Marshal(p.Config); err == nil {
			settings = string(b)
		}
	}
	return &PluginRecord{
		Code:       p.Code,
		Name:       p.Name,
		Version:    p.Version,
		Type:       PluginType(p.Type),
		Status:     status,
		Settings:   settings,
		InstalledAt: p.InstalledAt.Unix(),
		UpdatedAt:  p.UpdatedAt.Unix(),
	}
}

func fromRecord(r *PluginRecord) *pluginModel.Plugin {
	isEnabled := 0
	if r.Status == StatusEnabled {
		isEnabled = 1
	}
	settings := make(pluginModel.JSON)
	if r.Settings != "" {
		json.Unmarshal([]byte(r.Settings), &settings)
	}
	return &pluginModel.Plugin{
		Code:      r.Code,
		Name:      r.Name,
		Version:   r.Version,
		Type:      string(r.Type),
		IsEnabled: isEnabled,
		Config:    settings,
		InstalledAt: time.Unix(r.InstalledAt, 0),
	}
}
