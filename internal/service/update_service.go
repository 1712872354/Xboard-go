package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/xboard/xboard/internal/model"
	"gorm.io/gorm"
)

const (
	currentVersion = "1.0.0"
	updateCheckURL = "https://api.github.com/repos/xboard/xboard/releases/latest"
)

// UpdateService handles checking for application updates
type UpdateService struct {
	db *gorm.DB
}

func NewUpdateService(db *gorm.DB) *UpdateService {
	return &UpdateService{db: db}
}

type githubRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Prerelease  bool      `json:"prerelease"`
}

// CheckUpdate checks GitHub for the latest release
func (s *UpdateService) CheckUpdate() (map[string]interface{}, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(updateCheckURL)
	if err != nil {
		return nil, fmt.Errorf("check update failed: %w", err)
	}
	defer resp.Body.Close()

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("parse release failed: %w", err)
	}

	hasUpdate := release.TagName != "" && release.TagName != "v"+currentVersion && release.TagName != currentVersion

	return map[string]interface{}{
		"current_version": currentVersion,
		"latest_version":  release.TagName,
		"has_update":      hasUpdate,
		"release_name":    release.Name,
		"release_notes":   release.Body,
		"published_at":    release.PublishedAt,
		"release_url":     release.HTMLURL,
		"is_prerelease":   release.Prerelease,
	}, nil
}

// GetCurrentVersion returns the current application version
func (s *UpdateService) GetCurrentVersion() string {
	return currentVersion
}

// GetUpdateLog returns the update history from the database
func (s *UpdateService) GetUpdateLog() []map[string]interface{} {
	var settings []model.Setting
	s.db.Where("`key` LIKE ?", "update_log_%").Order("id DESC").Find(&settings)

	var logs []map[string]interface{}
	for _, setting := range settings {
		var logEntry map[string]interface{}
		if err := json.Unmarshal([]byte(setting.Value), &logEntry); err == nil {
			logEntry["key"] = setting.Key
			logs = append(logs, logEntry)
		}
	}

	if logs == nil {
		logs = make([]map[string]interface{}, 0)
	}

	return logs
}
