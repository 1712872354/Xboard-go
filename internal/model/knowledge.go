package model

// Knowledge represents a knowledge base article
type Knowledge struct {
	Model
	Title   string `gorm:"type:varchar(256)" json:"title"`
	Content string `gorm:"type:text" json:"content"`
	Sort    int    `gorm:"type:int(11);default:0" json:"sort"`
	Show    int    `gorm:"type:tinyint(1);default:1" json:"show"`
}

// TableName returns the table name
func (Knowledge) TableName() string {
	return "v2_knowledge"
}
