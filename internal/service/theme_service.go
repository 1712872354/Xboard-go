package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xboard/xboard/internal/model"
	"gorm.io/gorm"
)

// ThemeService handles theme management (list, config, switch, delete)
type ThemeService struct {
	db *gorm.DB
}

func NewThemeService(db *gorm.DB) *ThemeService {
	return &ThemeService{db: db}
}

// GetList returns all available themes with their metadata
func (s *ThemeService) GetList() ([]map[string]interface{}, error) {
	themesDir := "theme"
	entries, err := os.ReadDir(themesDir)
	if err != nil {
		return nil, fmt.Errorf("read themes dir failed: %w", err)
	}

	var themes []map[string]interface{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		themeName := entry.Name()
		configPath := filepath.Join(themesDir, themeName, "config.json")

		info := map[string]interface{}{
			"name": themeName,
			"path": filepath.Join(themesDir, themeName),
		}

		if data, err := os.ReadFile(configPath); err == nil {
			var config model.ThemeConfig
			if err := json.Unmarshal(data, &config); err == nil {
				info["config"] = config
			}
		}

		themes = append(themes, info)
	}

	return themes, nil
}

// GetConfig retrieves the configuration for a specific theme
func (s *ThemeService) GetConfig(theme string) (*model.ThemeConfig, error) {
	configPath := filepath.Join("theme", theme, "config.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("theme config not found: %w", err)
	}

	var config model.ThemeConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse theme config failed: %w", err)
	}

	return &config, nil
}

// Switch activates a theme by setting the current_theme setting
func (s *ThemeService) Switch(theme string) error {
	// Validate theme exists
	themePath := filepath.Join("theme", theme)
	if _, err := os.Stat(themePath); os.IsNotExist(err) {
		return fmt.Errorf("theme not found: %s", theme)
	}

	// Check that theme has required files
	configPath := filepath.Join(themePath, "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return errors.New("theme config file not found")
	}

	dashboardPath := filepath.Join(themePath, "dashboard.blade.php")
	if _, err := os.Stat(dashboardPath); os.IsNotExist(err) {
		return errors.New("theme view file not found")
	}

	// Update the current_theme setting
	var setting model.Setting
	result := s.db.Where("`key` = ?", "current_theme").First(&setting)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return s.db.Create(&model.Setting{Key: "current_theme", Value: theme}).Error
		}
		return result.Error
	}

	return s.db.Model(&setting).Update("value", theme).Error
}

// Delete removes a theme directory (non-system themes only)
func (s *ThemeService) Delete(theme string) error {
	if theme == "Xboard" || theme == "v2board" {
		return errors.New("system theme cannot be deleted")
	}

	// Check if it's the current theme
	var setting model.Setting
	if err := s.db.Where("`key` = ? AND `value` = ?", "current_theme", theme).First(&setting).Error; err == nil {
		return errors.New("current theme cannot be deleted")
	}

	themePath := filepath.Join("theme", theme)
	if _, err := os.Stat(themePath); os.IsNotExist(err) {
		return fmt.Errorf("theme not found: %s", theme)
	}

	return os.RemoveAll(themePath)
}
