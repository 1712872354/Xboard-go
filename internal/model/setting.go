package model

// Setting represents a system configuration key-value pair
type Setting struct {
	Model
	Key   string `gorm:"type:varchar(128);uniqueIndex" json:"key"`
	Value string `gorm:"type:text" json:"value"`
}

// Get retrieves a setting value by key
func (Setting) Get(key string) string {
	var setting Setting
	result := DB.Where("`key` = ?", key).First(&setting)
	if result.Error != nil {
		return ""
	}
	return setting.Value
}

// Set updates or creates a setting value by key
func (Setting) Set(key, value string) error {
	var setting Setting
	result := DB.Where("`key` = ?", key).First(&setting)
	if result.Error != nil {
		// Create new
		setting = Setting{Key: key, Value: value}
		return DB.Create(&setting).Error
	}
	setting.Value = value
	return DB.Save(&setting).Error
}

// TableName returns the table name
func (Setting) TableName() string {
	return "v2_settings"
}
