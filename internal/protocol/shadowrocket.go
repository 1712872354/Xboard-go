package protocol

import (
	"encoding/base64"
	"fmt"
	"net/url"
)

// ShadowrocketProtocol handles Shadowrocket config generation
type ShadowrocketProtocol struct{}

func init() {
	Register("shadowrocket", &ShadowrocketProtocol{})
}

func (p *ShadowrocketProtocol) Flags() []Flag {
	return []Flag{
		{Flag: "shadowrocket", Attribute: 1},
	}
}

func (p *ShadowrocketProtocol) GenerateConfig(user *ClientInfo, server *ServerConfig) interface{} {
	switch server.Protocol {
	case "shadowsocks":
		return p.buildShadowsocks(server)
	case "vmess":
		return p.buildVmess(server, user)
	case "vless":
		return p.buildVless(server, user)
	case "trojan":
		return p.buildTrojan(server)
	case "hysteria":
		return p.buildHysteria(server, user)
	case "tuic":
		return p.buildTUIC(server, user)
	case "anytls":
		return p.buildAnyTLS(server, user)
	case "socks":
		return p.buildSocks(server)
	default:
		return ""
	}
}

func (p *ShadowrocketProtocol) buildShadowsocks(server *ServerConfig) string {
	ps := getProtocolSettings(server)
	method := server.Cipher
	if c, ok := ps["cipher"].(string); ok && c != "" {
		method = c
	}
	password := server.Password
	if pwd, ok := ps["password"].(string); ok && pwd != "" {
		password = pwd
	}

	b64 := base64URLEncode([]byte(method + ":" + password))
	addr := wrapIPv6(server.Host)
	link := fmt.Sprintf("ss://%s@%s:%d", b64, addr, server.Port)

	if plugin, ok := ps["plugin"].(string); ok && plugin != "" {
		pluginOpts, _ := ps["plugin_opts"].(string)
		link += "/?plugin=" + url.QueryEscape(plugin+";"+pluginOpts)
	}

	link += "#" + url.PathEscape(server.Name) + "\r\n"
	return link
}

func (p *ShadowrocketProtocol) buildVmess(server *ServerConfig, user *ClientInfo) string {
	// Shadowrocket uses standard vmess:// links
	return BuildVMessShareLink(user.UUID, server)
}

func (p *ShadowrocketProtocol) buildVless(server *ServerConfig, user *ClientInfo) string {
	return BuildVLessShareLink(user.UUID, server)
}

func (p *ShadowrocketProtocol) buildTrojan(server *ServerConfig) string {
	return BuildTrojanShareLink(server.Password, server)
}

func (p *ShadowrocketProtocol) buildHysteria(server *ServerConfig, user *ClientInfo) string {
	return BuildHysteriaShareLink(user.UUID, server)
}

func (p *ShadowrocketProtocol) buildTUIC(server *ServerConfig, user *ClientInfo) string {
	return BuildTUICShareLink(user.UUID, server)
}

func (p *ShadowrocketProtocol) buildAnyTLS(server *ServerConfig, user *ClientInfo) string {
	ps := getProtocolSettings(server)
	addr := wrapIPv6(server.Host)
	config := url.Values{}
	if sni, ok := ps["tls"].(map[string]interface{}); ok {
		if serverName, ok := sni["server_name"].(string); ok {
			config.Set("sni", serverName)
		}
	}
	query := config.Encode()
	return fmt.Sprintf("anytls://%s@%s:%d?%s#%s\r\n", user.UUID, addr, server.Port, query, url.PathEscape(server.Name))
}

func (p *ShadowrocketProtocol) buildSocks(server *ServerConfig) string {
	credentials := base64.StdEncoding.EncodeToString([]byte(server.Password + ":" + server.Password))
	addr := wrapIPv6(server.Host)
	return fmt.Sprintf("socks://%s@%s:%d#%s\r\n", credentials, addr, server.Port, url.PathEscape(server.Name))
}
