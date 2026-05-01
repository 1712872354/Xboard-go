package model

import "encoding/json"

// Server represents a proxy server node
type Server struct {
	Model
	GroupID       uint   `gorm:"type:int(11);default:0;index" json:"group_id"`
	ParentID      uint   `gorm:"type:int(11);default:0" json:"parent_id"`
	MachineID     uint   `gorm:"type:int(11);default:0" json:"machine_id"`
	RouteIDs      Strings `gorm:"type:json" json:"route_ids"`
	Name          string `gorm:"type:varchar(128)" json:"name"`
	Network       string `gorm:"type:varchar(16)" json:"network"`
	Protocol      string `gorm:"type:varchar(32)" json:"protocol"`
	Host          string `gorm:"type:varchar(256)" json:"host"`
	Port          int    `gorm:"type:int(11);default:0" json:"port"`
	ServerPort    int    `gorm:"type:int(11);default:0" json:"server_port"`
	PortRange     string `gorm:"type:varchar(64)" json:"port_range"`
	ServerKey     string `gorm:"type:varchar(256)" json:"server_key"`
	CIPHER        string `gorm:"type:varchar(32)" json:"cipher"`
	Rate          string `gorm:"type:varchar(16);default:1" json:"rate"`
	Show          int    `gorm:"type:tinyint(1);default:1" json:"show"`
	Sort          int    `gorm:"type:int(11);default:0" json:"sort"`
	Area          string `gorm:"type:varchar(64)" json:"area"`
	TrafficRatio  string `gorm:"type:varchar(16);default:1" json:"traffic_ratio"`
	TrafficUsed   int64  `gorm:"type:bigint(20);default:0" json:"traffic_used"`
	TrafficLimit  int64  `gorm:"type:bigint(20);default:0" json:"traffic_limit"`
	DNS           string `gorm:"type:varchar(256)" json:"dns"`
	DNSJSON       JSON   `gorm:"type:json" json:"dns_json"`
	TLS           int    `gorm:"type:tinyint(1);default:0" json:"tls"`
	TLSProvider   string `gorm:"type:varchar(64)" json:"tls_provider"`
	TLSHost       string `gorm:"type:varchar(256)" json:"tls_host"`
	TLSCert       string `gorm:"type:text" json:"tls_cert"`
	TLSKey        string `gorm:"type:text" json:"tls_key"`
	TLSInsecure   int    `gorm:"type:tinyint(1);default:0" json:"tls_insecure"`
	Reality       int    `gorm:"type:tinyint(1);default:0" json:"reality"`
	RealityConfig JSON   `gorm:"type:json" json:"reality_config"`
	Flow          string `gorm:"type:varchar(64)" json:"flow"`
	Enable        int    `gorm:"type:tinyint(1);default:1" json:"enable"`
	CustomConfig  JSON   `gorm:"type:json" json:"custom_config"`
	OnlineCount   int    `gorm:"type:int(11);default:0" json:"online_count"`
	LastPushAt    int64  `gorm:"type:bigint(20);default:0" json:"last_push_at"`
}

// GetTLSConfig returns the parsed TLS config
func (s *Server) GetTLSConfig() map[string]interface{} {
	if s.TLS == 0 {
		return nil
	}
	return map[string]interface{}{
		"provider": s.TLSProvider,
		"host":     s.TLSHost,
		"cert":     s.TLSCert,
		"key":      s.TLSKey,
		"insecure": s.TLSInsecure,
	}
}

// GetRealityConfig returns the parsed reality config
func (s *Server) GetRealityConfig() map[string]interface{} {
	if s.Reality == 0 || s.RealityConfig == nil {
		return nil
	}
	return s.RealityConfig
}

// GetCustomConfig returns the parsed custom config
func (s *Server) GetCustomConfig() map[string]interface{} {
	if s.CustomConfig == nil {
		return nil
	}
	return s.CustomConfig
}

// GetRouteIDs returns the parsed route IDs
func (s *Server) GetRouteIDs() []string {
	return s.RouteIDs
}

// MarshalBinary implements encoding.BinaryMarshaler
func (s *Server) MarshalBinary() ([]byte, error) {
	return json.Marshal(s)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler
func (s *Server) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, s)
}

// TableName returns the table name
func (Server) TableName() string {
	return "v2_server"
}
