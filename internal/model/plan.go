package model

// Plan represents a subscription plan
type Plan struct {
	Model
	GroupID              uint    `gorm:"type:int(11);default:0" json:"group_id"`
	Name                 string  `gorm:"type:varchar(128)" json:"name"`
	Content              string  `gorm:"type:text" json:"content"`
	TransferEnable       int64   `gorm:"type:bigint(20);default:0" json:"transfer_enable"`
	DeviceLimit          int     `gorm:"type:int(11);default:0" json:"device_limit"`
	SpeedLimit           int     `gorm:"type:int(11);default:0" json:"speed_limit"`
	DeviceLimitMode      int     `gorm:"type:int(11);default:0" json:"device_limit_mode"`
	DeviceExclusiveIP    int     `gorm:"type:int(11);default:0" json:"device_exclusive_ip"`
	DailyTraffic         int64   `gorm:"type:bigint(20);default:0" json:"daily_traffic"`
	MonthlyTrafficEnable int     `gorm:"type:tinyint(1);default:0" json:"monthly_traffic_enable"`
	Price                float64 `gorm:"type:decimal(10,2);default:0" json:"price"`
	MonthlyPrice         float64 `gorm:"type:decimal(10,2);default:0" json:"monthly_price"`
	QuarterPrice         float64 `gorm:"type:decimal(10,2);default:0" json:"quarter_price"`
	HalfYearPrice        float64 `gorm:"type:decimal(10,2);default:0" json:"half_year_price"`
	YearPrice            float64 `gorm:"type:decimal(10,2);default:0" json:"year_price"`
	TwoYearPrice         float64 `gorm:"type:decimal(10,2);default:0" json:"two_year_price"`
	ThreeYearPrice       float64 `gorm:"type:decimal(10,2);default:0" json:"three_year_price"`
	OnetimePrice         float64 `gorm:"type:decimal(10,2);default:0" json:"onetime_price"`
	ResetPrice           float64 `gorm:"type:decimal(10,2);default:0" json:"reset_price"`
	ResetTrafficMethod   int     `gorm:"type:int(11);default:0" json:"reset_traffic_method"`
	CapacityLimit        int64   `gorm:"type:bigint(20);default:0" json:"capacity_limit"`
	Sort                 int     `gorm:"type:int(11);default:0" json:"sort"`
	CurrencyCode         string  `gorm:"type:varchar(16)" json:"currency_code"`
	CurrencySymbol       string  `gorm:"type:varchar(16)" json:"currency_symbol"`
	Show                 int     `gorm:"type:tinyint(1);default:1" json:"show"`
	RechargePrice        float64 `gorm:"type:decimal(10,2);default:0" json:"recharge_price"`
	RechargeEnable       int     `gorm:"type:tinyint(1);default:0" json:"recharge_enable"`
	Enable               int     `gorm:"type:tinyint(1);default:1" json:"enable"`
	Categories           string  `gorm:"type:text" json:"categories"`
}

// TableName returns the table name
func (Plan) TableName() string {
	return "v2_plan"
}
