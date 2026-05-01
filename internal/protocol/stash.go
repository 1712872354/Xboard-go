package protocol

import "fmt"

// StashProtocol handles Stash config generation (Clash-compatible YAML)
type StashProtocol struct{}

func init() {
	Register("stash", &StashProtocol{})
}

func (p *StashProtocol) Flags() []Flag {
	return []Flag{
		{Flag: "stash", Attribute: 1},
	}
}

func (p *StashProtocol) GenerateConfig(user *ClientInfo, server *ServerConfig) interface{} {
	// Stash uses Clash-compatible format, delegate to Clash
	clash := &ClashProtocol{}
	switch server.Protocol {
	case "shadowsocks":
		return clash.GenerateConfig(user, server)
	case "vmess":
		return clash.GenerateConfig(user, server)
	case "trojan":
		return clash.GenerateConfig(user, server)
	case "hysteria":
		return p.buildHysteria(server, user)
	case "vless":
		return p.buildVless(server, user)
	case "tuic":
		return p.buildTUIC(server, user)
	case "anytls":
		return p.buildAnyTLS(server, user)
	case "socks":
		return clash.GenerateConfig(user, server)
	case "http":
		return clash.GenerateConfig(user, server)
	default:
		return nil
	}
}

func (p *StashProtocol) buildHysteria(server *ServerConfig, user *ClientInfo) map[string]interface{} {
	ps := getProtocolSettings(server)
	cfg := map[string]interface{}{
		"name":     server.Name,
		"type":     "hysteria2",
		"server":   server.Host,
		"port":     server.Port,
		"password": user.UUID,
		"udp":      true,
	}
	if up, ok := ps["bandwidth"].(map[string]interface{}); ok {
		if upMbps, ok := up["up"]; ok {
			cfg["up"] = fmt.Sprintf("%v", upMbps)
		}
		if downMbps, ok := up["down"]; ok {
			cfg["down"] = fmt.Sprintf("%v", downMbps)
		}
	}
	return cfg
}

func (p *StashProtocol) buildVless(server *ServerConfig, user *ClientInfo) map[string]interface{} {
	ps := getProtocolSettings(server)
	cfg := map[string]interface{}{
		"name":   server.Name,
		"type":   "vless",
		"server": server.Host,
		"port":   server.Port,
		"uuid":   user.UUID,
		"udp":    true,
	}
	if flow, ok := ps["flow"].(string); ok {
		cfg["flow"] = flow
	}
	if tls, ok := ps["tls"].(bool); ok && tls {
		cfg["tls"] = true
	}
	return cfg
}

func (p *StashProtocol) buildTUIC(server *ServerConfig, user *ClientInfo) map[string]interface{} {
	return map[string]interface{}{
		"name":     server.Name,
		"type":     "tuic",
		"server":   server.Host,
		"port":     server.Port,
		"uuid":     user.UUID,
		"password": user.UUID,
		"udp":      true,
	}
}

func (p *StashProtocol) buildAnyTLS(server *ServerConfig, user *ClientInfo) map[string]interface{} {
	return map[string]interface{}{
		"name":     server.Name,
		"type":     "anytls",
		"server":   server.Host,
		"port":     server.Port,
		"password": user.UUID,
	}
}
