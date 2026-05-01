package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Model is the base model with common fields
type Model struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SoftDelete adds soft delete support
type SoftDelete struct {
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// JSON is a generic type for storing JSON data in MySQL
type JSON map[string]interface{}

// Scan implements the sql.Scanner interface
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan JSON: unexpected type %T", value)
	}
	return json.Unmarshal(bytes, j)
}

// Value implements the driver.Valuer interface
func (j JSON) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Strings is a type for storing JSON string arrays
type Strings []string

// Scan implements the sql.Scanner interface
func (s *Strings) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan Strings: unexpected type %T", value)
	}
	return json.Unmarshal(bytes, s)
}

// Value implements the driver.Valuer interface
func (s Strings) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return json.Marshal(s)
}

// Ints is a type for storing JSON integer arrays
type Ints []int

// Scan implements the sql.Scanner interface
func (s *Ints) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan Ints: unexpected type %T", value)
	}
	return json.Unmarshal(bytes, s)
}

// Value implements the driver.Valuer interface
func (s Ints) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return json.Marshal(s)
}

// DB is the global database connection
var DB *gorm.DB
