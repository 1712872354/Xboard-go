package model

// ServerGroup represents a group of servers
type ServerGroup struct {
	Model
	Name string `gorm:"type:varchar(128)" json:"name"`

	// Relations
	Servers []Server `gorm:"many2many:v2_server_group_relation;foreignKey:ID;joinForeignKey:ServerGroupID;References:ID;joinReferences:ServerID" json:"servers,omitempty"`
}

// TableName returns the table name
func (ServerGroup) TableName() string {
	return "v2_server_group"
}
