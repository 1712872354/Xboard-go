package model

// SubscribeTemplate represents a subscription template
type SubscribeTemplate struct {
	Model
	Name         string `gorm:"type:varchar(128)" json:"name"`
	Template     string `gorm:"type:text" json:"template"`
	CoveredTable string `gorm:"type:varchar(128)" json:"covered_table"`
}

// TableName returns the table name
func (SubscribeTemplate) TableName() string {
	return "v2_subscribe_templates"
}
