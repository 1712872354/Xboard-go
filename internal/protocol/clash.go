package protocol

import (
	"strings"
)

// ClashProtocol handles Clash config generation
type ClashProtocol struct{}

func init() {
	Register("clash", &ClashProtocol{})
}

func (p *ClashProtocol) Flags() []Flag {
	return []Flag{
		{Flag: "clash", Attribute: 1},
	}
}

// ClashProxy represents a single proxy entry in Clash YAML
type ClashProxy struct {
	Name           string                 `json:"name" yaml:"name"`
	Type           string                 `json:"type" yaml:"type"`
	Server         string                 `json:"server" yaml:"server"`
	Port           int                    `json:"port" yaml:"port"`
	Cipher         string                 `json:"cipher,omitempty" yaml:"cipher,omitempty"`
	Password       string                 `json:"password,omitempty" yaml:"password,omitempty"`
	UUID           string                 `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	AlterID        int                    `json:"alterId,omitempty" yaml:"alterId,omitempty"`
	SNI            string                 `json:"sni,omitempty" yaml:"sni,omitempty"`
	SkipCertVerify bool                   `json:"skip-cert-verify,omitempty" yaml:"skip-cert-verify,omitempty"`
	UDP            bool                   `json:"udp,omitempty" yaml:"udp,omitempty"`
	TLS            bool                   `json:"tls,omitempty" yaml:"tls,omitempty"`
	Servername     string                 `json:"servername,omitempty" yaml:"servername,omitempty"`
	Network        string                 `json:"network,omitempty" yaml:"network,omitempty"`
	Plugin         string                 `json:"plugin,omitempty" yaml:"plugin,omitempty"`
	PluginOpts     map[string]interface{} `json:"plugin-opts,omitempty" yaml:"plugin-opts,omitempty"`
	WSOpts         map[string]interface{} `json:"ws-opts,omitempty" yaml:"ws-opts,omitempty"`
	HTTPOpts       map[string]interface{} `json:"http-opts,omitempty" yaml:"http-opts,omitempty"`
	GRPCOpts       map[string]interface{} `json:"grpc-opts,omitempty" yaml:"grpc-opts,omitempty"`
	Username       string                 `json:"username,omitempty" yaml:"username,omitempty"`
}

func (p *ClashProtocol) GenerateConfig(user *ClientInfo, server *ServerConfig) interface{} {
	switch server.Protocol {
	case "shadowsocks":
		return p.buildShadowsocks(server)
	case "vmess":
		return p.buildVmess(server, user)
	case "trojan":
		return p.buildTrojan(server)
	case "socks":
		return p.buildSocks5(server)
	case "http":
		return p.buildHTTP(server)
	default:
		return nil
	}
}

func (p *ClashProtocol) buildShadowsocks(server *ServerConfig) ClashProxy {
	ps := getProtocolSettings(server)
	proxy := ClashProxy{
		Name:     server.Name,
		Type:     "ss",
		Server:   server.Host,
		Port:     server.Port,
		Cipher:   server.Cipher,
		Password: server.Password,
		UDP:      true,
	}

	if c, ok := ps["cipher"].(string); ok && c != "" {
		proxy.Cipher = c
	}

	if plugin, ok := ps["plugin"].(string); ok && plugin != "" {
		proxy.Plugin = plugin
		pluginOpts, _ := ps["plugin_opts"].(string)

		opts := parsePluginOpts(pluginOpts)
		switch plugin {
		case "obfs":
			proxy.PluginOpts = map[string]interface{}{
				"mode": getOptStr(opts, "obfs", "http"),
				"host": getOptStr(opts, "obfs-host", ""),
			}
			if path, ok := opts["path"]; ok {
				proxy.PluginOpts["path"] = path
			}
		case "v2ray-plugin":
			proxy.PluginOpts = map[string]interface{}{
				"mode": getOptStr(opts, "mode", "websocket"),
				"tls":  opts["tls"] == "true",
				"host": getOptStr(opts, "host", ""),
				"path": getOptStr(opts, "path", "/"),
			}
		default:
			converted := make(map[string]interface{})
			for k, v := range opts {
				converted[k] = v
			}
			proxy.PluginOpts = converted
		}
	}
	return proxy
}

func (p *ClashProtocol) buildVmess(server *ServerConfig, user *ClientInfo) ClashProxy {
	ps := getProtocolSettings(server)
	proxy := ClashProxy{
		Name:     server.Name,
		Type:     "vmess",
		Server:   server.Host,
		Port:     server.Port,
		UUID:     user.UUID,
		AlterID:  0,
		Cipher:   "auto",
		UDP:      true,
	}

	if tls, ok := ps["tls"].(bool); ok && tls {
		proxy.TLS = true
		if sni, ok := ps["tls_settings"].(map[string]interface{}); ok {
			proxy.SkipCertVerify = getBool(sni, "allow_insecure")
			if serverName, ok := sni["server_name"].(string); ok {
				proxy.Servername = serverName
			}
		}
	}

	switch getStr(ps, "network") {
	case "tcp":
		proxy.Network = "tcp"
		if ns, ok := ps["network_settings"].(map[string]interface{}); ok {
			if header, ok := ns["header"].(map[string]interface{}); ok {
				if hType, ok := header["type"].(string); ok && hType == "http" {
					proxy.Network = "http"
					httpOpts := make(map[string]interface{})
					if request, ok := header["request"].(map[string]interface{}); ok {
						if paths, ok := request["path"].([]interface{}); ok && len(paths) > 0 {
							httpOpts["path"] = paths
						}
						if reqHeaders, ok := request["headers"].(map[string]interface{}); ok {
							httpOpts["headers"] = reqHeaders
						}
					}
					if len(httpOpts) > 0 {
						proxy.HTTPOpts = httpOpts
					}
				}
			}
		}
	case "ws":
		proxy.Network = "ws"
		if ns, ok := ps["network_settings"].(map[string]interface{}); ok {
			wsOpts := make(map[string]interface{})
			if path, ok := ns["path"].(string); ok {
				wsOpts["path"] = path
			}
			if headers, ok := ns["headers"].(map[string]interface{}); ok {
				if host, ok := headers["Host"].(string); ok {
					wsOpts["headers"] = map[string]string{"Host": host}
				}
			}
			if len(wsOpts) > 0 {
				proxy.WSOpts = wsOpts
			}
		}
	case "grpc":
		proxy.Network = "grpc"
		if ns, ok := ps["network_settings"].(map[string]interface{}); ok {
			grpcOpts := make(map[string]interface{})
			if sn, ok := ns["serviceName"].(string); ok {
				grpcOpts["grpc-service-name"] = sn
			}
			if len(grpcOpts) > 0 {
				proxy.GRPCOpts = grpcOpts
			}
		}
	}
	return proxy
}

func (p *ClashProtocol) buildTrojan(server *ServerConfig) ClashProxy {
	ps := getProtocolSettings(server)
	proxy := ClashProxy{
		Name:     server.Name,
		Type:     "trojan",
		Server:   server.Host,
		Port:     server.Port,
		Password: server.Password,
		UDP:      true,
	}

	if sni, ok := ps["tls_settings"].(map[string]interface{}); ok {
		if serverName, ok := sni["server_name"].(string); ok {
			proxy.SNI = serverName
		}
		proxy.SkipCertVerify = getBool(sni, "allow_insecure")
	}

	switch getStr(ps, "network") {
	case "tcp":
		proxy.Network = "tcp"
	case "ws":
		proxy.Network = "ws"
		if ns, ok := ps["network_settings"].(map[string]interface{}); ok {
			wsOpts := make(map[string]interface{})
			if path, ok := ns["path"].(string); ok {
				wsOpts["path"] = path
			}
			if headers, ok := ns["headers"].(map[string]interface{}); ok {
				if host, ok := headers["Host"].(string); ok {
					wsOpts["headers"] = map[string]string{"Host": host}
				}
			}
			if len(wsOpts) > 0 {
				proxy.WSOpts = wsOpts
			}
		}
	case "grpc":
		proxy.Network = "grpc"
		if ns, ok := ps["network_settings"].(map[string]interface{}); ok {
			grpcOpts := make(map[string]interface{})
			if sn, ok := ns["serviceName"].(string); ok {
				grpcOpts["grpc-service-name"] = sn
			}
			if len(grpcOpts) > 0 {
				proxy.GRPCOpts = grpcOpts
			}
		}
	default:
		proxy.Network = "tcp"
	}
	return proxy
}

func (p *ClashProtocol) buildSocks5(server *ServerConfig) ClashProxy {
	ps := getProtocolSettings(server)
	proxy := ClashProxy{
		Name:     server.Name,
		Type:     "socks5",
		Server:   server.Host,
		Port:     server.Port,
		Username: server.Password,
		Password: server.Password,
		UDP:      true,
	}
	if tls, ok := ps["tls"].(bool); ok && tls {
		proxy.TLS = true
		proxy.SkipCertVerify = getBool(ps, "tls_settings.allow_insecure")
	}
	return proxy
}

func (p *ClashProtocol) buildHTTP(server *ServerConfig) ClashProxy {
	ps := getProtocolSettings(server)
	proxy := ClashProxy{
		Name:     server.Name,
		Type:     "http",
		Server:   server.Host,
		Port:     server.Port,
		Username: server.Password,
		Password: server.Password,
	}
	if tls, ok := ps["tls"].(bool); ok && tls {
		proxy.TLS = true
		proxy.SkipCertVerify = getBool(ps, "tls_settings.allow_insecure")
	}
	return proxy
}

// Helpers

func parsePluginOpts(opts string) map[string]string {
	result := make(map[string]string)
	if opts == "" {
		return result
	}
	pairs := strings.Split(opts, ";")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			result[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return result
}

func getOptStr(m map[string]string, key, def string) string {
	if v, ok := m[key]; ok && v != "" {
		return v
	}
	return def
}

func getStr(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	s, ok := m[key].(string)
	if ok {
		return s == "true" || s == "1"
	}
	return false
}
