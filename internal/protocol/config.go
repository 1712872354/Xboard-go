package protocol

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// ==================== SIP008 (Shadowsocks) ====================

// SIP008Config represents a Shadowsocks server in SIP008 format
type SIP008Config struct {
	ID          int    `json:"id"`
	Remarks     string `json:"remarks"`
	Server      string `json:"server"`
	ServerPort  int    `json:"server_port"`
	Password    string `json:"password"`
	Method      string `json:"method"`
	Plugin      string `json:"plugin,omitempty"`
	PluginOpts  string `json:"plugin_opts,omitempty"`
}

// BuildShadowsocksSIP008 builds a SIP008 config for Shadowsocks
func BuildShadowsocksSIP008(server *ServerConfig, user *ClientInfo) SIP008Config {
	config := SIP008Config{
		ID:         int(server.ID),
		Remarks:    server.Name,
		Server:     server.Host,
		ServerPort: server.Port,
		Password:   server.Password,
		Method:     server.Cipher,
	}

	if plugin, ok := server.CustomConfig["plugin"].(string); ok && plugin != "" {
		config.Plugin = plugin
		if pluginOpts, ok := server.CustomConfig["plugin_opts"].(string); ok {
			config.PluginOpts = pluginOpts
		}
	}

	return config
}

// ==================== VMess Share Link ====================

// VMessShareConfig represents the JSON body of a vmess:// share link
type VMessShareConfig struct {
	V    string `json:"v"`
	PS   string `json:"ps"`
	Add  string `json:"add"`
	Port string `json:"port"`
	ID   string `json:"id"`
	Aid  string `json:"aid"`
	Net  string `json:"net,omitempty"`
	Type string `json:"type"`
	Host string `json:"host"`
	Path string `json:"path"`
	TLS  string `json:"tls,omitempty"`
	SNI  string `json:"sni,omitempty"`
	FP   string `json:"fp,omitempty"`
}

// BuildVMessShareLink builds a vmess:// share link
func BuildVMessShareLink(uuid string, server *ServerConfig) string {
	cfg := VMessShareConfig{
		V:    "2",
		PS:   server.Name,
		Add:  server.Host,
		Port: fmt.Sprintf("%d", server.Port),
		ID:   uuid,
		Aid:  "0",
		Net:  "tcp",
		Type: "none",
		Host: "",
		Path: "",
	}

	ps := getProtocolSettings(server)
	if network, ok := ps["network"].(string); ok {
		cfg.Net = network
	}

	if tlsVal, ok := ps["tls"]; ok {
		if tlsBool, ok := tlsVal.(bool); ok && tlsBool {
			cfg.TLS = "tls"
		}
	}

	if sni, ok := ps["tls_settings"].(map[string]interface{}); ok {
		if serverName, ok := sni["server_name"].(string); ok {
			cfg.SNI = serverName
		}
	}

	switch cfg.Net {
	case "tcp":
		if ns, ok := ps["network_settings"].(map[string]interface{}); ok {
			if header, ok := ns["header"].(map[string]interface{}); ok {
				if hType, ok := header["type"].(string); ok && hType != "none" {
					cfg.Type = hType
					if request, ok := header["request"].(map[string]interface{}); ok {
						if path, ok := request["path"].([]interface{}); ok && len(path) > 0 {
							cfg.Path = fmt.Sprintf("%v", path[0])
						}
						if headers, ok := request["headers"].(map[string]interface{}); ok {
							if host, ok := headers["Host"].([]interface{}); ok && len(host) > 0 {
								cfg.Host = fmt.Sprintf("%v", host[0])
							}
						}
					}
				}
			}
		}
	case "ws":
		cfg.Type = "ws"
		if ns, ok := ps["network_settings"].(map[string]interface{}); ok {
			if path, ok := ns["path"].(string); ok {
				cfg.Path = path
			}
			if headers, ok := ns["headers"].(map[string]interface{}); ok {
				if host, ok := headers["Host"].(string); ok {
					cfg.Host = host
				}
			}
		}
	case "grpc":
		cfg.Type = "grpc"
		if ns, ok := ps["network_settings"].(map[string]interface{}); ok {
			if sn, ok := ns["serviceName"].(string); ok {
				cfg.Path = sn
			}
		}
	}

	jsonBytes, _ := json.Marshal(cfg)
	b64 := base64.StdEncoding.EncodeToString(jsonBytes)
	return fmt.Sprintf("vmess://%s\r\n", b64)
}

// ==================== VLess Share Link ====================

// BuildVLessShareLink builds a vless:// share link
func BuildVLessShareLink(uuid string, server *ServerConfig) string {
	ps := getProtocolSettings(server)
	config := url.Values{}

	network := "tcp"
	if n, ok := ps["network"].(string); ok {
		network = n
	}
	config.Set("type", network)
	config.Set("encryption", "none")

	if flow, ok := ps["flow"].(string); ok {
		config.Set("flow", flow)
	}

	tlsMode := 0
	if t, ok := ps["tls"]; ok {
		switch v := t.(type) {
		case bool:
			if v {
				tlsMode = 1
			}
		case float64:
			tlsMode = int(v)
		case int:
			tlsMode = v
		}
	}

	switch tlsMode {
	case 1:
		config.Set("security", "tls")
		if sni, ok := ps["tls_settings"].(map[string]interface{}); ok {
			if serverName, ok := sni["server_name"].(string); ok {
				config.Set("sni", serverName)
			}
			if insecure, ok := sni["allow_insecure"].(bool); ok && insecure {
				config.Set("allowInsecure", "1")
			}
		}
	case 2:
		config.Set("security", "reality")
		if rs, ok := ps["reality_settings"].(map[string]interface{}); ok {
			if pk, ok := rs["public_key"].(string); ok {
				config.Set("pbk", pk)
			}
			if sid, ok := rs["short_id"].(string); ok {
				config.Set("sid", sid)
			}
			if sn, ok := rs["server_name"].(string); ok {
				config.Set("sni", sn)
				config.Set("servername", sn)
			}
		}
		config.Set("spx", "/")
	}

	switch network {
	case "ws":
		if ns, ok := ps["network_settings"].(map[string]interface{}); ok {
			if path, ok := ns["path"].(string); ok {
				config.Set("path", path)
			}
			if headers, ok := ns["headers"].(map[string]interface{}); ok {
				if host, ok := headers["Host"].(string); ok {
					config.Set("host", host)
				}
			}
		}
	case "grpc":
		if ns, ok := ps["network_settings"].(map[string]interface{}); ok {
			if sn, ok := ns["serviceName"].(string); ok {
				config.Set("serviceName", sn)
			}
		}
	case "httpupgrade":
		if ns, ok := ps["network_settings"].(map[string]interface{}); ok {
			if path, ok := ns["path"].(string); ok {
				config.Set("path", path)
			}
			if host, ok := ns["host"].(string); ok {
				config.Set("host", host)
			} else {
				config.Set("host", server.Host)
			}
		}
	case "xhttp":
		if ns, ok := ps["network_settings"].(map[string]interface{}); ok {
			if path, ok := ns["path"].(string); ok {
				config.Set("path", path)
			}
			if host, ok := ns["host"].(string); ok {
				config.Set("host", host)
			} else {
				config.Set("host", server.Host)
			}
			if mode, ok := ns["mode"].(string); ok {
				config.Set("mode", mode)
			}
		}
	}

	addr := wrapIPv6(server.Host)
	query := config.Encode()
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s\r\n", uuid, addr, server.Port, query, url.PathEscape(server.Name))
}

// ==================== Trojan Share Link ====================

// BuildTrojanShareLink builds a trojan:// share link
func BuildTrojanShareLink(password string, server *ServerConfig) string {
	ps := getProtocolSettings(server)
	config := url.Values{}

	tlsMode := 1
	if t, ok := ps["tls"]; ok {
		switch v := t.(type) {
		case float64:
			tlsMode = int(v)
		case int:
			tlsMode = v
		case bool:
			if v {
				tlsMode = 1
			}
		}
	}

	switch tlsMode {
	case 2:
		config.Set("security", "reality")
		if rs, ok := ps["reality_settings"].(map[string]interface{}); ok {
			if pk, ok := rs["public_key"].(string); ok {
				config.Set("pbk", pk)
			}
			if sid, ok := rs["short_id"].(string); ok {
				config.Set("sid", sid)
			}
			if sn, ok := rs["server_name"].(string); ok {
				config.Set("sni", sn)
			}
		}
	default:
		if sni, ok := ps["tls_settings"].(map[string]interface{}); ok {
			if insecure, ok := sni["allow_insecure"].(bool); ok && insecure {
				config.Set("allowInsecure", "1")
			}
			if serverName, ok := sni["server_name"].(string); ok {
				config.Set("peer", serverName)
				config.Set("sni", serverName)
			}
		}
	}

	if network, ok := ps["network"].(string); ok {
		switch network {
		case "ws":
			config.Set("type", "ws")
			if ns, ok := ps["network_settings"].(map[string]interface{}); ok {
				if path, ok := ns["path"].(string); ok {
					config.Set("path", path)
				}
				if headers, ok := ns["headers"].(map[string]interface{}); ok {
					if host, ok := headers["Host"].(string); ok {
						config.Set("host", host)
					}
				}
			}
		case "grpc":
			config.Set("type", "grpc")
			if ns, ok := ps["network_settings"].(map[string]interface{}); ok {
				if sn, ok := ns["serviceName"].(string); ok {
					config.Set("serviceName", sn)
				}
			}
		case "httpupgrade":
			config.Set("type", "httpupgrade")
			if ns, ok := ps["network_settings"].(map[string]interface{}); ok {
				if path, ok := ns["path"].(string); ok {
					config.Set("path", path)
				}
				if host, ok := ns["host"].(string); ok {
					config.Set("host", host)
				} else {
					config.Set("host", server.Host)
				}
			}
		case "xhttp":
			config.Set("type", "xhttp")
			if ns, ok := ps["network_settings"].(map[string]interface{}); ok {
				if path, ok := ns["path"].(string); ok {
					config.Set("path", path)
				}
				if host, ok := ns["host"].(string); ok {
					config.Set("host", host)
				} else {
					config.Set("host", server.Host)
				}
				if mode, ok := ns["mode"].(string); ok {
					config.Set("mode", mode)
				}
			}
		}
	}

	addr := wrapIPv6(server.Host)
	query := config.Encode()
	return fmt.Sprintf("trojan://%s@%s:%d?%s#%s\r\n", password, addr, server.Port, query, url.PathEscape(server.Name))
}

// ==================== Hysteria Share Link ====================

// BuildHysteriaShareLink builds a hysteria:// or hysteria2:// share link
func BuildHysteriaShareLink(password string, server *ServerConfig) string {
	ps := getProtocolSettings(server)
	version := 2
	if v, ok := ps["version"]; ok {
		switch val := v.(type) {
		case float64:
			version = int(val)
		case int:
			version = val
		}
	}

	config := url.Values{}

	if sni, ok := ps["tls"].(map[string]interface{}); ok {
		if serverName, ok := sni["server_name"].(string); ok {
			config.Set("sni", serverName)
		}
		if insecure, ok := sni["allow_insecure"].(bool); ok {
			if insecure {
				config.Set("insecure", "1")
			} else {
				config.Set("insecure", "0")
			}
		}
	}

	name := url.PathEscape(server.Name)
	addr := wrapIPv6(server.Host)

	if version == 2 {
		if obfs, ok := ps["obfs"].(map[string]interface{}); ok {
			if open, ok := obfs["open"].(bool); ok && open {
				config.Set("obfs", "salamander")
				if obfsPwd, ok := obfs["password"].(string); ok {
					config.Set("obfs-password", obfsPwd)
				}
			}
		}
		if server.PortRange != "" {
			config.Set("mport", server.PortRange)
		}
		query := config.Encode()
		return fmt.Sprintf("hysteria2://%s@%s:%d?%s#%s\r\n", password, addr, server.Port, query, name)
	}

	config.Set("protocol", "udp")
	config.Set("auth", password)
	if up, ok := ps["bandwidth"].(map[string]interface{}); ok {
		if upMbps, ok := up["up"]; ok {
			config.Set("upmbps", fmt.Sprintf("%v", upMbps))
		}
		if downMbps, ok := up["down"]; ok {
			config.Set("downmbps", fmt.Sprintf("%v", downMbps))
		}
	}
	if obfs, ok := ps["obfs"].(map[string]interface{}); ok {
		if open, ok := obfs["open"].(bool); ok && open {
			if obfsPwd, ok := obfs["password"].(string); ok {
				config.Set("obfs", "xplus")
				config.Set("obfsParam", obfsPwd)
			}
		}
	}
	query := config.Encode()
	return fmt.Sprintf("hysteria://%s:%d?%s#%s\r\n", addr, server.Port, query, name)
}

// ==================== TUIC Share Link ====================

// BuildTUICShareLink builds a tuic:// share link
func BuildTUICShareLink(password string, server *ServerConfig) string {
	ps := getProtocolSettings(server)
	config := url.Values{}

	if sni, ok := ps["tls"].(map[string]interface{}); ok {
		if serverName, ok := sni["server_name"].(string); ok {
			config.Set("sni", serverName)
		}
		if insecure, ok := sni["allow_insecure"].(bool); ok && insecure {
			config.Set("insecure", "1")
		}
	}

	if alpn, ok := ps["alpn"]; ok {
		switch v := alpn.(type) {
		case []interface{}:
			var parts []string
			for _, a := range v {
				parts = append(parts, fmt.Sprintf("%v", a))
			}
			config.Set("alpn", strings.Join(parts, ","))
		case string:
			config.Set("alpn", v)
		}
	}

	congestion := "cubic"
	if cc, ok := ps["congestion_control"].(string); ok {
		congestion = cc
	}
	config.Set("congestion_control", congestion)

	udpRelay := "native"
	if ur, ok := ps["udp_relay_mode"].(string); ok {
		udpRelay = ur
	}
	config.Set("udp-relay-mode", udpRelay)

	addr := wrapIPv6(server.Host)
	query := config.Encode()
	uri := fmt.Sprintf("tuic://%s:%s@%s:%d", password, password, addr, server.Port)
	if query != "" {
		uri += "?" + query
	}
	uri += "#" + url.PathEscape(server.Name) + "\r\n"
	return uri
}

// ==================== AnyTLS Share Link ====================

// BuildAnyTLSShareLink builds an anytls:// share link
func BuildAnyTLSShareLink(password string, server *ServerConfig) string {
	ps := getProtocolSettings(server)
	config := url.Values{}

	if sni, ok := ps["tls"].(map[string]interface{}); ok {
		if serverName, ok := sni["server_name"].(string); ok {
			config.Set("sni", serverName)
		}
		if insecure, ok := sni["allow_insecure"].(bool); ok {
			config.Set("insecure", fmt.Sprintf("%v", insecure))
		}
	}

	addr := wrapIPv6(server.Host)
	query := config.Encode()
	return fmt.Sprintf("anytls://%s@%s:%d?%s#%s\r\n", password, addr, server.Port, query, url.PathEscape(server.Name))
}

// ==================== Socks Share Link ====================

// BuildSocksShareLink builds a socks:// share link
func BuildSocksShareLink(password string, server *ServerConfig) string {
	credentials := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", password, password)))
	addr := wrapIPv6(server.Host)
	return fmt.Sprintf("socks://%s@%s:%d#%s\r\n", credentials, addr, server.Port, url.PathEscape(server.Name))
}

// ==================== HTTP Share Link ====================

// BuildHTTPShareLink builds an http:// share link
func BuildHTTPShareLink(password string, server *ServerConfig) string {
	ps := getProtocolSettings(server)
	credentials := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", password, password)))
	addr := wrapIPv6(server.Host)

	params := url.Values{}
	if tls, ok := ps["tls"].(bool); ok && tls {
		params.Set("security", "tls")
		if sni, ok := ps["tls_settings"].(map[string]interface{}); ok {
			if serverName, ok := sni["server_name"].(string); ok {
				params.Set("sni", serverName)
			}
			if insecure, ok := sni["allow_insecure"].(bool); ok {
				if insecure {
					params.Set("allowInsecure", "1")
				} else {
					params.Set("allowInsecure", "0")
				}
			}
		}
	}

	uri := fmt.Sprintf("http://%s@%s:%d", credentials, addr, server.Port)
	if params.Encode() != "" {
		uri += "?" + params.Encode()
	}
	uri += "#" + url.PathEscape(server.Name) + "\r\n"
	return uri
}

// ==================== Mieru Config ====================

// BuildMieruConfig builds a mieru compatible config
func BuildMieruConfig(password string, server *ServerConfig) map[string]interface{} {
	return map[string]interface{}{
		"port":    server.Port,
		"server":  server.Host,
		"password": password,
	}
}

// ==================== Helpers ====================

// getProtocolSettings extracts protocol_settings from CustomConfig
func getProtocolSettings(server *ServerConfig) map[string]interface{} {
	if server.CustomConfig == nil {
		return make(map[string]interface{})
	}
	if ps, ok := server.CustomConfig["protocol_settings"].(map[string]interface{}); ok {
		return ps
	}
	return server.CustomConfig
}

// wrapIPv6 wraps IPv6 addresses in brackets
func wrapIPv6(host string) string {
	if strings.Contains(host, ":") {
		if !strings.HasPrefix(host, "[") {
			return "[" + host + "]"
		}
	}
	return host
}
