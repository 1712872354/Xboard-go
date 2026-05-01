package protocol

import "strconv"

// LoonProtocol handles Loon config generation
type LoonProtocol struct{}

func init() {
	Register("loon", &LoonProtocol{})
}

func (p *LoonProtocol) Flags() []Flag {
	return []Flag{
		{Flag: "loon", Attribute: 1},
	}
}

func (p *LoonProtocol) GenerateConfig(user *ClientInfo, server *ServerConfig) interface{} {
	switch server.Protocol {
	case "shadowsocks":
		return p.buildShadowsocks(server)
	case "vmess":
		return p.buildVmess(server, user)
	case "trojan":
		return p.buildTrojan(server)
	case "hysteria":
		return p.buildHysteria(server, user)
	case "vless":
		return p.buildVless(server, user)
	case "anytls":
		return p.buildAnyTLS(server)
	default:
		return ""
	}
}

func (p *LoonProtocol) buildShadowsocks(server *ServerConfig) string {
	ps := getProtocolSettings(server)
	method := server.Cipher
	if c, ok := ps["cipher"].(string); ok && c != "" {
		method = c
	}
	password := server.Password
	return "shadowsocks=" + server.Host + ":" + strconv.Itoa(server.Port) +
		", method=" + method +
		", password=" + password +
		", fast-open=true, udp=true" +
		"\r\n"
}

func (p *LoonProtocol) buildVmess(server *ServerConfig, user *ClientInfo) string {
	ps := getProtocolSettings(server)
	net := "tcp"
	if n, ok := ps["network"].(string); ok {
		net = n
	}
	result := "vmess=" + server.Host + ":" + strconv.Itoa(server.Port) +
		", method=auto, username=" + user.UUID +
		", fast-open=true, udp=true"
	if tls, ok := ps["tls"].(bool); ok && tls {
		result += ", over-tls=true"
	}
	if net == "ws" {
		result += ", ws=true"
	}
	result += "\r\n"
	return result
}

func (p *LoonProtocol) buildTrojan(server *ServerConfig) string {
	result := "trojan=" + server.Host + ":" + strconv.Itoa(server.Port) +
		", password=" + server.Password +
		", fast-open=true, udp=true"
	ps := getProtocolSettings(server)
	if sni, ok := ps["tls_settings"].(map[string]interface{}); ok {
		if serverName, ok := sni["server_name"].(string); ok {
			result += ", sni=" + serverName
		}
	}
	result += "\r\n"
	return result
}

func (p *LoonProtocol) buildHysteria(server *ServerConfig, user *ClientInfo) string {
	ps := getProtocolSettings(server)
	sni := ""
	if tls, ok := ps["tls"].(map[string]interface{}); ok {
		if serverName, ok := tls["server_name"].(string); ok {
			sni = serverName
		}
	}
	result := "hysteria2=" + server.Host + ":" + strconv.Itoa(server.Port) +
		", password=" + user.UUID
	if sni != "" {
		result += ", sni=" + sni
	}
	result += "\r\n"
	return result
}

func (p *LoonProtocol) buildVless(server *ServerConfig, user *ClientInfo) string {
	return "vless=" + server.Host + ":" + strconv.Itoa(server.Port) +
		", uuid=" + user.UUID +
		"\r\n"
}

func (p *LoonProtocol) buildAnyTLS(server *ServerConfig) string {
	return "anytls=" + server.Host + ":" + strconv.Itoa(server.Port) +
		", password=" + server.Password +
		"\r\n"
}
