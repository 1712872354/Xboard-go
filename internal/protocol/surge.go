package protocol

import (
	"fmt"
	"strings"
)

// SurgeProtocol handles Surge config generation (text-based format)
type SurgeProtocol struct{}

func init() {
	Register("surge", &SurgeProtocol{})
}

func (p *SurgeProtocol) Flags() []Flag {
	return []Flag{
		{Flag: "surge", Attribute: 1},
	}
}

func (p *SurgeProtocol) GenerateConfig(user *ClientInfo, server *ServerConfig) interface{} {
	switch server.Protocol {
	case "shadowsocks":
		return p.buildShadowsocks(server)
	case "vmess":
		return p.buildVmess(server, user)
	case "trojan":
		return p.buildTrojan(server)
	case "hysteria":
		return p.buildHysteria(server, user)
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

func (p *SurgeProtocol) buildShadowsocks(server *ServerConfig) string {
	ps := getProtocolSettings(server)
	parts := []string{
		fmt.Sprintf("%s = ss", server.Name),
		server.Host,
		fmt.Sprintf("%d", server.Port),
		fmt.Sprintf("encrypt-method=%s", server.Cipher),
		fmt.Sprintf("password=%s", server.Password),
		"tfo=true",
		"udp-relay=true",
	}

	if c, ok := ps["cipher"].(string); ok && c != "" {
		parts[3] = fmt.Sprintf("encrypt-method=%s", c)
	}

	if plugin, ok := ps["plugin"].(string); ok && plugin != "" {
		pluginOpts, _ := ps["plugin_opts"].(string)
		opts := parsePluginOpts(pluginOpts)
		if plugin == "obfs" {
			parts = append(parts, "obfs="+getOptStr(opts, "obfs", "http"))
			if obfsHost, ok := opts["obfs-host"]; ok {
				parts = append(parts, "obfs-host="+obfsHost)
			}
			if path, ok := opts["path"]; ok {
				parts = append(parts, "obfs-uri="+path)
			}
		}
	}

	// Filter empty parts
	var filtered []string
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	return strings.Join(filtered, ",") + "\r\n"
}

func (p *SurgeProtocol) buildVmess(server *ServerConfig, user *ClientInfo) string {
	ps := getProtocolSettings(server)
	parts := []string{
		fmt.Sprintf("%s = vmess", server.Name),
		server.Host,
		fmt.Sprintf("%d", server.Port),
		"username=" + user.UUID,
		"vmess-aead=true",
		"tfo=true",
		"udp-relay=true",
	}

	if tls, ok := ps["tls"].(bool); ok && tls {
		parts = append(parts, "tls=true")
		if sni, ok := ps["tls_settings"].(map[string]interface{}); ok {
			if insecure, ok := sni["allow_insecure"].(bool); ok && insecure {
				parts = append(parts, "skip-cert-verify=true")
			}
			if serverName, ok := sni["server_name"].(string); ok {
				parts = append(parts, "sni="+serverName)
			}
		}
	}

	if network, ok := ps["network"].(string); ok && network == "ws" {
		parts = append(parts, "ws=true")
		if ns, ok := ps["network_settings"].(map[string]interface{}); ok {
			if path, ok := ns["path"].(string); ok {
				parts = append(parts, "ws-path="+path)
			}
			if headers, ok := ns["headers"].(map[string]interface{}); ok {
				if host, ok := headers["Host"].(string); ok {
					parts = append(parts, "ws-headers=Host:"+host)
				}
			}
		}
	}

	var filtered []string
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	return strings.Join(filtered, ",") + "\r\n"
}

func (p *SurgeProtocol) buildTrojan(server *ServerConfig) string {
	ps := getProtocolSettings(server)
	sni := ""
	if tls, ok := ps["tls_settings"].(map[string]interface{}); ok {
		if serverName, ok := tls["server_name"].(string); ok {
			sni = serverName
		}
	}

	parts := []string{
		fmt.Sprintf("%s = trojan", server.Name),
		server.Host,
		fmt.Sprintf("%d", server.Port),
		"password=" + server.Password,
		sni,
		"tfo=true",
		"udp-relay=true",
	}

	if tls, ok := ps["tls_settings"].(map[string]interface{}); ok {
		if insecure, ok := tls["allow_insecure"].(bool); ok && insecure {
			parts = append(parts, "skip-cert-verify=true")
		}
	}

	var filtered []string
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	return strings.Join(filtered, ",") + "\r\n"
}

func (p *SurgeProtocol) buildHysteria(server *ServerConfig, user *ClientInfo) string {
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

	if version != 2 {
		return ""
	}

	sni := ""
	if tls, ok := ps["tls"].(map[string]interface{}); ok {
		if serverName, ok := tls["server_name"].(string); ok {
			sni = serverName
		}
	}

	parts := []string{
		fmt.Sprintf("%s = hysteria2", server.Name),
		server.Host,
		fmt.Sprintf("%d", server.Port),
		"password=" + user.UUID,
		sni,
		"udp-relay=true",
	}

	if bw, ok := ps["bandwidth"].(map[string]interface{}); ok {
		if up, ok := bw["up"]; ok {
			parts = append(parts, "upload-bandwidth="+fmt.Sprintf("%v", up))
		}
		if down, ok := bw["down"]; ok {
			parts = append(parts, "download-bandwidth="+fmt.Sprintf("%v", down))
		}
	}

	if tls, ok := ps["tls"].(map[string]interface{}); ok {
		if insecure, ok := tls["allow_insecure"].(bool); ok {
			if insecure {
				parts = append(parts, "skip-cert-verify=true")
			} else {
				parts = append(parts, "skip-cert-verify=false")
			}
		}
	}

	var filtered []string
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	return strings.Join(filtered, ",") + "\r\n"
}

func (p *SurgeProtocol) buildAnyTLS(server *ServerConfig) string {
	ps := getProtocolSettings(server)
	parts := []string{
		fmt.Sprintf("%s = anytls", server.Name),
		server.Host,
		fmt.Sprintf("%d", server.Port),
		"password=" + server.Password,
	}

	if sni, ok := ps["tls"].(map[string]interface{}); ok {
		if serverName, ok := sni["server_name"].(string); ok {
			parts = append(parts, "sni="+serverName)
		}
		if insecure, ok := sni["allow_insecure"].(bool); ok && insecure {
			parts = append(parts, "skip-cert-verify=true")
		}
	}

	var filtered []string
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	return strings.Join(filtered, ",") + "\r\n"
}

func (p *SurgeProtocol) buildSocks(server *ServerConfig) string {
	ps := getProtocolSettings(server)
	typeName := "socks5"
	if tls, ok := ps["tls"].(bool); ok && tls {
		typeName = "socks5-tls"
	}

	parts := []string{
		fmt.Sprintf("%s = %s", server.Name, typeName),
		server.Host,
		fmt.Sprintf("%d", server.Port),
		server.Password,
		server.Password,
	}

	if tls, ok := ps["tls"].(bool); ok && tls {
		if sni, ok := ps["tls_settings"].(map[string]interface{}); ok {
			if serverName, ok := sni["server_name"].(string); ok {
				parts = append(parts, "sni="+serverName)
			}
			if insecure, ok := sni["allow_insecure"].(bool); ok && insecure {
				parts = append(parts, "skip-cert-verify=true")
			}
		}
	}
	parts = append(parts, "udp-relay=true")

	var filtered []string
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	return strings.Join(filtered, ",") + "\r\n"
}

func (p *SurgeProtocol) buildHTTP(server *ServerConfig) string {
	ps := getProtocolSettings(server)
	typeName := "http"
	if tls, ok := ps["tls"].(bool); ok && tls {
		typeName = "https"
	}

	parts := []string{
		fmt.Sprintf("%s = %s", server.Name, typeName),
		server.Host,
		fmt.Sprintf("%d", server.Port),
		server.Password,
		server.Password,
	}

	if tls, ok := ps["tls"].(bool); ok && tls {
		if sni, ok := ps["tls_settings"].(map[string]interface{}); ok {
			if serverName, ok := sni["server_name"].(string); ok {
				parts = append(parts, "sni="+serverName)
			}
			if insecure, ok := sni["allow_insecure"].(bool); ok && insecure {
				parts = append(parts, "skip-cert-verify=true")
			}
		}
	}

	var filtered []string
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	return strings.Join(filtered, ",") + "\r\n"
}
