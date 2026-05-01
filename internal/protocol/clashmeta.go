package protocol

import "fmt"

// ClashMetaProtocol handles Clash.Meta config generation
type ClashMetaProtocol struct{}

func init() {
	Register("clashmeta", &ClashMetaProtocol{})
}

func (p *ClashMetaProtocol) Flags() []Flag {
	return []Flag{
		{Flag: "meta", Attribute: 1},
		{Flag: "verge", Attribute: 1},
		{Flag: "flclash", Attribute: 1},
		{Flag: "nekobox", Attribute: 1},
		{Flag: "clashmetaforandroid", Attribute: 1},
	}
}

func (p *ClashMetaProtocol) GenerateConfig(user *ClientInfo, server *ServerConfig) interface{} {
	// Clash.Meta extends Clash format with additional protocol support
	switch server.Protocol {
	case "shadowsocks":
		return p.buildShadowsocks(server)
	case "vmess":
		return p.buildVmess(server, user)
	case "trojan":
		return p.buildTrojan(server)
	case "vless":
		return p.buildVless(server, user)
	case "hysteria":
		return p.buildHysteria(server, user)
	case "tuic":
		return p.buildTUIC(server, user)
	case "anytls":
		return p.buildAnyTLS(server, user)
	case "socks":
		return p.buildSocks(server)
	case "http":
		return p.buildHTTP(server)
	case "mieru":
		return p.buildMieru(server)
	default:
		return nil
	}
}

// ClashMetaProxy extends ClashProxy with additional fields supported by Clash.Meta
type ClashMetaProxy struct {
	ClashProxy
	Flow            string                 `json:"flow,omitempty" yaml:"flow,omitempty"`
	Hysteria2Opts   map[string]interface{} `json:"hysteria2-opts,omitempty" yaml:"hysteria2-opts,omitempty"`
	RealityOpts     map[string]interface{} `json:"reality-opts,omitempty" yaml:"reality-opts,omitempty"`
	VLESSOpts       map[string]interface{} `json:"vless-opts,omitempty" yaml:"vless-opts,omitempty"`
}

func (p *ClashMetaProtocol) buildShadowsocks(server *ServerConfig) ClashMetaProxy {
	cp := (&ClashProtocol{}).buildShadowsocks(server)
	return ClashMetaProxy{ClashProxy: cp}
}

func (p *ClashMetaProtocol) buildVmess(server *ServerConfig, user *ClientInfo) ClashMetaProxy {
	cp := (&ClashProtocol{}).buildVmess(server, user)
	return ClashMetaProxy{ClashProxy: cp}
}

func (p *ClashMetaProtocol) buildTrojan(server *ServerConfig) ClashMetaProxy {
	cp := (&ClashProtocol{}).buildTrojan(server)
	return ClashMetaProxy{ClashProxy: cp}
}

func (p *ClashMetaProtocol) buildVless(server *ServerConfig, user *ClientInfo) ClashMetaProxy {
	ps := getProtocolSettings(server)
	proxy := ClashMetaProxy{}
	proxy.Name = server.Name
	proxy.Type = "vless"
	proxy.Server = server.Host
	proxy.Port = server.Port
	proxy.UUID = user.UUID
	proxy.UDP = true

	if flow, ok := ps["flow"].(string); ok {
		proxy.Flow = flow
	}

	if tlsVal, ok := ps["tls"]; ok {
		tlsMode := 0
		switch v := tlsVal.(type) {
		case float64:
			tlsMode = int(v)
		case int:
			tlsMode = v
		case bool:
			if v {
				tlsMode = 1
			}
		}
		if tlsMode > 0 {
			proxy.TLS = true
			if tlsMode == 2 {
				proxy.SkipCertVerify = true
				if rs, ok := ps["reality_settings"].(map[string]interface{}); ok {
					proxy.RealityOpts = map[string]interface{}{
						"public-key": getStr(rs, "public_key"),
						"short-id":   getStr(rs, "short-id"),
					}
				}
			}
		}
	}

	proxy.Network = getStr(ps, "network")
	return proxy
}

func (p *ClashMetaProtocol) buildHysteria(server *ServerConfig, user *ClientInfo) ClashMetaProxy {
	ps := getProtocolSettings(server)
	proxy := ClashMetaProxy{}
	proxy.Name = server.Name
	proxy.Server = server.Host
	proxy.UDP = true

	version := 2
	if v, ok := ps["version"]; ok {
		switch val := v.(type) {
		case float64:
			version = int(val)
		case int:
			version = val
		}
	}

	if version == 2 {
		proxy.Type = "hysteria2"
		proxy.Password = user.UUID
		proxy.Port = server.Port
		h2Opts := make(map[string]interface{})
		if up, ok := ps["bandwidth"].(map[string]interface{}); ok {
			if upMbps, ok := up["up"]; ok {
				h2Opts["up"] = fmt.Sprintf("%v", upMbps)
			}
			if downMbps, ok := up["down"]; ok {
				h2Opts["down"] = fmt.Sprintf("%v", downMbps)
			}
		}
		if len(h2Opts) > 0 {
			proxy.Hysteria2Opts = h2Opts
		}
	} else {
		proxy.Type = "hysteria"
		proxy.Port = server.Port
		proxy.Password = user.UUID
	}
	return proxy
}

func (p *ClashMetaProtocol) buildTUIC(server *ServerConfig, user *ClientInfo) ClashMetaProxy {
	proxy := ClashMetaProxy{}
	proxy.Name = server.Name
	proxy.Type = "tuic"
	proxy.Server = server.Host
	proxy.Port = server.Port
	proxy.UUID = user.UUID
	proxy.Password = user.UUID
	proxy.UDP = true
	return proxy
}

func (p *ClashMetaProtocol) buildAnyTLS(server *ServerConfig, user *ClientInfo) ClashMetaProxy {
	proxy := ClashMetaProxy{}
	proxy.Name = server.Name
	proxy.Type = "anytls"
	proxy.Server = server.Host
	proxy.Port = server.Port
	proxy.Password = user.UUID
	return proxy
}

func (p *ClashMetaProtocol) buildSocks(server *ServerConfig) ClashMetaProxy {
	cp := (&ClashProtocol{}).buildSocks5(server)
	return ClashMetaProxy{ClashProxy: cp}
}

func (p *ClashMetaProtocol) buildHTTP(server *ServerConfig) ClashMetaProxy {
	cp := (&ClashProtocol{}).buildHTTP(server)
	return ClashMetaProxy{ClashProxy: cp}
}

func (p *ClashMetaProtocol) buildMieru(server *ServerConfig) ClashMetaProxy {
	proxy := ClashMetaProxy{}
	proxy.Name = server.Name
	proxy.Type = "mieru"
	proxy.Server = server.Host
	proxy.Port = server.Port
	return proxy
}
