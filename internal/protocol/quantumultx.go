package protocol

import (
	"strconv"
	"strings"
)

// QuantumultXProtocol handles QuantumultX config generation
type QuantumultXProtocol struct{}

func init() {
	Register("quantumultx", &QuantumultXProtocol{})
}

func (p *QuantumultXProtocol) Flags() []Flag {
	return []Flag{
		{Flag: "quantumult%20x", Attribute: 1},
		{Flag: "quantumult-x", Attribute: 1},
	}
}

func (p *QuantumultXProtocol) GenerateConfig(user *ClientInfo, server *ServerConfig) interface{} {
	switch server.Protocol {
	case "shadowsocks":
		return p.buildShadowsocks(server)
	case "vmess":
		return p.buildVmess(server, user)
	case "vless":
		return p.buildVless(server, user)
	case "trojan":
		return p.buildTrojan(server)
	case "anytls":
		return p.buildAnyTLS(server)
	case "socks":
		return p.buildSocks(server)
	case "http":
		return p.buildHTTP(server)
	default:
		return ""
	}
}

func (p *QuantumultXProtocol) buildShadowsocks(server *ServerConfig) string {
	ps := getProtocolSettings(server)
	method := server.Cipher
	if c, ok := ps["cipher"].(string); ok && c != "" {
		method = c
	}
	password := server.Password
	if pwd, ok := ps["password"].(string); ok && pwd != "" {
		password = pwd
	}

	var parts []string
	parts = append(parts, "shadowsocks="+server.Host+":"+strconv.Itoa(server.Port))
	parts = append(parts, "method="+method)
	parts = append(parts, "password="+password)

	if plugin, ok := ps["plugin"].(string); ok && plugin != "" {
		parts = append(parts, "plugin="+plugin)
		if pluginOpts, ok := ps["plugin_opts"].(string); ok {
			parts = append(parts, "plugin-opts="+pluginOpts)
		}
	}

	return strings.Join(parts, ", ") + "\r\n"
}

func (p *QuantumultXProtocol) buildVmess(server *ServerConfig, user *ClientInfo) string {
	ps := getProtocolSettings(server)
	parts := []string{
		"vmess=" + server.Host + ":" + strconv.Itoa(server.Port),
		"method=chacha20-ietf-poly1305",
		"password=" + user.UUID,
		"fast-open=true",
		"udp-relay=true",
	}

	if tls, ok := ps["tls"].(bool); ok && tls {
		parts = append(parts, "over-tls=true")
		if sni, ok := ps["tls_settings"].(map[string]interface{}); ok {
			if serverName, ok := sni["server_name"].(string); ok {
				parts = append(parts, "tls-host="+serverName)
			}
		}
	}

	if network, ok := ps["network"].(string); ok {
		switch network {
		case "ws":
			parts = append(parts, "obfs=ws")
			if ns, ok := ps["network_settings"].(map[string]interface{}); ok {
				if path, ok := ns["path"].(string); ok {
					parts = append(parts, "obfs-uri="+path)
				}
				if headers, ok := ns["headers"].(map[string]interface{}); ok {
					if host, ok := headers["Host"].(string); ok {
						parts = append(parts, "obfs-host="+host)
					}
				}
			}
		case "grpc":
			parts = append(parts, "obfs=grpc")
			if ns, ok := ps["network_settings"].(map[string]interface{}); ok {
				if sn, ok := ns["serviceName"].(string); ok {
					parts = append(parts, "obfs-uri="+sn)
				}
			}
		}
	}

	return strings.Join(parts, ", ") + "\r\n"
}

func (p *QuantumultXProtocol) buildVless(server *ServerConfig, user *ClientInfo) string {
	return p.buildVMessLike("vless", server, user)
}

func (p *QuantumultXProtocol) buildTrojan(server *ServerConfig) string {
	parts := []string{
		"trojan=" + server.Host + ":" + strconv.Itoa(server.Port),
		"password=" + server.Password,
		"fast-open=true",
		"udp-relay=true",
	}
	ps := getProtocolSettings(server)
	if sni, ok := ps["tls_settings"].(map[string]interface{}); ok {
		if serverName, ok := sni["server_name"].(string); ok {
			parts = append(parts, "tls-host="+serverName)
		}
	}
	return strings.Join(parts, ", ") + "\r\n"
}

func (p *QuantumultXProtocol) buildAnyTLS(server *ServerConfig) string {
	return "anytls=" + server.Host + ":" + strconv.Itoa(server.Port) +
		", password=" + server.Password + "\r\n"
}

func (p *QuantumultXProtocol) buildSocks(server *ServerConfig) string {
	return "socks5=" + server.Host + ":" + strconv.Itoa(server.Port) +
		", username=" + server.Password +
		", password=" + server.Password + "\r\n"
}

func (p *QuantumultXProtocol) buildHTTP(server *ServerConfig) string {
	ps := getProtocolSettings(server)
	address := server.Host + ":" + strconv.Itoa(server.Port)
	if tls, ok := ps["tls"].(bool); ok && tls {
		return "https=" + address +
			", username=" + server.Password +
			", password=" + server.Password + "\r\n"
	}
	return "http=" + address +
		", username=" + server.Password +
		", password=" + server.Password + "\r\n"
}

func (p *QuantumultXProtocol) buildVMessLike(prefix string, server *ServerConfig, user *ClientInfo) string {
	return prefix + "=" + server.Host + ":" + strconv.Itoa(server.Port) +
		", method=none, password=" + user.UUID + "\r\n"
}
