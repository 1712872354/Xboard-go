package model

import "time"

// User represents a user in the system
type User struct {
	Model
	SoftDelete
	InviteUserID              uint      `gorm:"type:int(11);default:0" json:"invite_user_id"`
	ParentID                  uint      `gorm:"type:int(11);default:0" json:"parent_id"`
	PlanID                    uint      `gorm:"type:int(11);default:0;index" json:"plan_id"`
	GroupID                   uint      `gorm:"type:int(11);default:0;index" json:"group_id"`
	Email                     string    `gorm:"type:varchar(64);index" json:"email"`
	Password                  string    `gorm:"type:varchar(256)" json:"password"`
	Token                     string    `gorm:"type:varchar(64);uniqueIndex" json:"token"`
	Balance                   float64   `gorm:"type:decimal(10,2);default:0" json:"balance"`
	TransferEnable            int64     `gorm:"type:bigint(20);default:0" json:"transfer_enable"`
	U                         int64     `gorm:"type:bigint(20);default:0" json:"u"`
	D                         int64     `gorm:"type:bigint(20);default:0" json:"d"`
	LastTrafficResetDay       int       `gorm:"type:int(11);default:0" json:"last_traffic_reset_day"`
	TrafficResetDay           int       `gorm:"type:int(11);default:0" json:"traffic_reset_day"`
	Discount                  int       `gorm:"type:int(11);default:0" json:"discount"`
	CommissionType            int       `gorm:"type:int(11);default:0" json:"commission_type"`
	CommissionRate            int       `gorm:"type:int(11);default:0" json:"commission_rate"`
	CommissionBalance         float64   `gorm:"type:decimal(10,2);default:0" json:"commission_balance"`
	CommissionDisplay         int       `gorm:"type:int(11);default:0" json:"commission_display"`
	Staff                     int       `gorm:"type:int(11);default:0" json:"staff"`
	IsAdmin                   int       `gorm:"type:tinyint(1);default:0" json:"is_admin"`
	LastLoginAt               time.Time `gorm:"type:datetime" json:"last_login_at"`
	LastLoginIP               string    `gorm:"type:varchar(64)" json:"last_login_ip"`
	UUID                      string    `gorm:"type:varchar(64);index" json:"uuid"`
	DeviceLimit               int       `gorm:"type:int(11);default:0" json:"device_limit"`
	SpeedLimit                int       `gorm:"type:int(11);default:0" json:"speed_limit"`
	SubscribeURL              string    `gorm:"type:varchar(128)" json:"subscribe_url"`
	SubscribeStatus           int       `gorm:"type:tinyint(1);default:1" json:"subscribe_status"`
	SubscribeBandwidthUsage   int64     `gorm:"type:bigint(20);default:0" json:"subscribe_bandwidth_usage"`
	SubscribeBandwidthLimit   int64     `gorm:"type:bigint(20);default:0" json:"subscribe_bandwidth_limit"`
	Banned                    int       `gorm:"type:tinyint(1);default:0" json:"banned"`
	Remarks                   string    `gorm:"type:varchar(256)" json:"remarks"`
	ExpiredAt                 time.Time `gorm:"type:datetime" json:"expired_at"`
	TrialExpiredAt            time.Time `gorm:"type:datetime" json:"trial_expired_at"`
	PlanExpiredAt             time.Time `gorm:"type:datetime" json:"plan_expired_at"`
	OnlineIPCount             int       `gorm:"type:int(11);default:0" json:"online_ip_count"`
	TelegramID                int64     `gorm:"type:bigint(20);default:0;index" json:"telegram_id"`
	TokenVersion              int       `gorm:"type:int(11);default:0" json:"token_version"`

	// Relations
	Plan             *Plan             `gorm:"foreignKey:PlanID" json:"plan,omitempty"`
	ServerGroup      *ServerGroup      `gorm:"foreignKey:GroupID" json:"server_group,omitempty"`
	Orders           []Order           `gorm:"foreignKey:UserID" json:"orders,omitempty"`
	Tickets          []Ticket          `gorm:"foreignKey:UserID" json:"tickets,omitempty"`
	InviteCodes      []InviteCode      `gorm:"foreignKey:UserID" json:"invite_codes,omitempty"`
	CommissionLogs   []CommissionLog   `gorm:"foreignKey:InviteUserID" json:"commission_logs,omitempty"`
	StatUsers        []StatUser        `gorm:"foreignKey:UserID" json:"stat_users,omitempty"`
	TrafficResetLogs []TrafficResetLog `gorm:"foreignKey:UserID" json:"traffic_reset_logs,omitempty"`
}

// TableName returns the table name
func (User) TableName() string {
	return "v2_user"
}
