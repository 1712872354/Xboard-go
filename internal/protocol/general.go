package protocol

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
)

// GeneralProtocol handles general subscription format (v2rayN, v2rayNG, PassWall, etc.)
type GeneralProtocol struct{}

func init() {
	Register("general", &GeneralProtocol{})
}

func (p *GeneralProtocol) Flags() []Flag {
	return []Flag{
		{Flag: "general", Attribute: 1},
		{Flag: "v2rayn", Attribute: 1},
		{Flag: "v2rayng", Attribute: 1},
		{Flag: "passwall", Attribute: 1},
		{Flag: "ssrplus", Attribute: 1},
		{Flag: "sagernet", Attribute: 1},
	}
}

func (p *GeneralProtocol) GenerateConfig(user *ClientInfo, server *ServerConfig) interface{} {
	switch server.Protocol {
	case "shadowsocks":
		return BuildShadowsocksShareLink(server, user)
	case "vmess":
		return BuildVMessShareLink(user.UUID, server)
	case "vless":
		return BuildVLessShareLink(user.UUID, server)
	case "trojan":
		return BuildTrojanShareLink(server.Password, server)
	case "hysteria":
		return BuildHysteriaShareLink(user.UUID, server)
	case "tuic":
		return BuildTUICShareLink(user.UUID, server)
	case "anytls":
		return BuildAnyTLSShareLink(user.UUID, server)
	case "socks":
		return BuildSocksShareLink(server.Password, server)
	case "http":
		return BuildHTTPShareLink(server.Password, server)
	default:
		return ""
	}
}

// BuildShadowsocksShareLink builds a ss:// share link for general subscription
func BuildShadowsocksShareLink(server *ServerConfig, user *ClientInfo) string {
	ps := getProtocolSettings(server)
	password := server.Password
	if pwd, ok := ps["password"].(string); ok && pwd != "" {
		password = pwd
	}
	method := server.Cipher
	if c, ok := ps["cipher"].(string); ok && c != "" {
		method = c
	}
	payload := []byte(method + ":" + password)
	b64 := base64URLEncode(payload)
	addr := wrapIPv6(server.Host)
	link := "ss://" + b64 + "@" + addr + ":" + fmt.Sprintf("%d", server.Port)

	plugin, _ := ps["plugin"].(string)
	pluginOpts, _ := ps["plugin_opts"].(string)
	if plugin != "" && pluginOpts != "" {
		link += "/?plugin=" + url.QueryEscape(plugin+";"+pluginOpts)
	}

	link += "#" + url.PathEscape(server.Name) + "\r\n"
	return link
}

// base64URLEncode performs base64 encoding with URL-safe characters (no padding variations)
func base64URLEncode(data []byte) string {
	s := base64.StdEncoding.EncodeToString(data)
	s = strings.ReplaceAll(s, "+", "-")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.TrimRight(s, "=")
	return s
}
