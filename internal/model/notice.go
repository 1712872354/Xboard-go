package model

// Notice represents a system notification
type Notice struct {
	Model
	Title   string `gorm:"type:varchar(256)" json:"title"`
	Content string `gorm:"type:text" json:"content"`
	ImgURL  string `gorm:"type:varchar(256)" json:"img_url"`
	Show    int    `gorm:"type:tinyint(1);default:1" json:"show"`
	Sort    int    `gorm:"type:int(11);default:0" json:"sort"`
}

// TableName returns the table name
func (Notice) TableName() string {
	return "v2_notice"
}
