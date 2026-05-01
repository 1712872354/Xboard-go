package model

// ServerGroupRelation is the pivot table for server-group many-to-many relationship
type ServerGroupRelation struct {
	ID            uint `gorm:"primarykey" json:"id"`
	ServerID      uint `gorm:"type:int(11);index;uniqueIndex:idx_server_group" json:"server_id"`
	ServerGroupID uint `gorm:"type:int(11);index;uniqueIndex:idx_server_group" json:"server_group_id"`
}

// TableName returns the table name
func (ServerGroupRelation) TableName() string {
	return "v2_server_group_relation"
}
