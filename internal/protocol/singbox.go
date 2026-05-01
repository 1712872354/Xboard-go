package protocol

import "fmt"

// SingBoxProtocol handles Sing-box config generation
type SingBoxProtocol struct{}

func init() {
	Register("singbox", &SingBoxProtocol{})
}

func (p *SingBoxProtocol) Flags() []Flag {
	return []Flag{
		{Flag: "sing-box", Attribute: 1},
		{Flag: "hiddify", Attribute: 1},
		{Flag: "sfm", Attribute: 1},
	}
}

// SingBoxOutbound represents a sing-box outbound configuration
type SingBoxOutbound struct {
	Tag            string                 `json:"tag"`
	Type           string                 `json:"type"`
	Server         string                 `json:"server,omitempty"`
	ServerPort     int                    `json:"server_port,omitempty"`
	Method         string                 `json:"method,omitempty"`
	Password       string                 `json:"password,omitempty"`
	Plugin         string                 `json:"plugin,omitempty"`
	PluginOpts     string                 `json:"plugin_opts,omitempty"`
	UUID           string                 `json:"uuid,omitempty"`
	Security       string                 `json:"security,omitempty"`
	AlterID        int                    `json:"alter_id,omitempty"`
	Flow           string                 `json:"flow,omitempty"`
	PacketEncoding string                 `json:"packet_encoding,omitempty"`
	AuthStr        string                 `json:"auth_str,omitempty"`
	Obfs           map[string]interface{} `json:"obfs,omitempty"`
	DisableMTU     bool                   `json:"disable_mtu_discovery,omitempty"`
	UpMbps         int                    `json:"up_mbps,omitempty"`
	DownMbps       int                    `json:"down_mbps,omitempty"`
	Token          string                 `json:"token,omitempty"`
	Username       string                 `json:"username,omitempty"`
	Version        string                 `json:"version,omitempty"`
	Congestion     string                 `json:"congestion_control,omitempty"`
	UDPRelayMode   string                 `json:"udp_relay_mode,omitempty"`
	ZeroRTT        bool                   `json:"zero_rtt_handshake,omitempty"`
	Heartbeat      string                 `json:"heartbeat,omitempty"`
	Path           string                 `json:"path,omitempty"`
	Headers        map[string]string      `json:"headers,omitempty"`
	TLS            *SingBoxTLS            `json:"tls,omitempty"`
	Transport      map[string]interface{} `json:"transport,omitempty"`
	Multiplex      map[string]interface{} `json:"multiplex,omitempty"`
}

// SingBoxTLS represents TLS config for sing-box
type SingBoxTLS struct {
	Enabled    bool                   `json:"enabled"`
	Insecure   bool                   `json:"insecure,omitempty"`
	ServerName string                 `json:"server_name,omitempty"`
	ALPN       []string               `json:"alpn,omitempty"`
	UTLS       map[string]interface{} `json:"utls,omitempty"`
	ECH        map[string]interface{} `json:"ech,omitempty"`
	Reality    map[string]interface{} `json:"reality,omitempty"`
}

func (p *SingBoxProtocol) GenerateConfig(user *ClientInfo, server *ServerConfig) interface{} {
	ps := getProtocolSettings(server)

	switch server.Protocol {
	case "shadowsocks":
		return p.buildShadowsocks(server, user, ps)
	case "vmess":
		return p.buildVmess(server, user, ps)
	case "vless":
		return p.buildVless(server, user, ps)
	case "trojan":
		return p.buildTrojan(server, user, ps)
	case "hysteria":
		return p.buildHysteria(server, user, ps)
	case "tuic":
		return p.buildTUIC(server, user, ps)
	case "anytls":
		return p.buildAnyTLS(server, user, ps)
	case "socks":
		return p.buildSocks(server, user, ps)
	case "http":
		return p.buildHTTP(server, user, ps)
	default:
		return nil
	}
}

func (p *SingBoxProtocol) buildShadowsocks(server *ServerConfig, user *ClientInfo, ps map[string]interface{}) SingBoxOutbound {
	outbound := SingBoxOutbound{
		Tag:        server.Name,
		Type:       "shadowsocks",
		Server:      server.Host,
		ServerPort: server.Port,
		Method:     server.Cipher,
		Password:   server.Password,
	}
	if c, ok := ps["cipher"].(string); ok && c != "" {
		outbound.Method = c
	}
	if plugin, ok := ps["plugin"].(string); ok && plugin != "" {
		outbound.Plugin = plugin
		if pluginOpts, ok := ps["plugin_opts"].(string); ok {
			outbound.PluginOpts = pluginOpts
		}
	}
	return outbound
}

func (p *SingBoxProtocol) buildVmess(server *ServerConfig, user *ClientInfo, ps map[string]interface{}) SingBoxOutbound {
	outbound := SingBoxOutbound{
		Tag:        server.Name,
		Type:       "vmess",
		Server:      server.Host,
		ServerPort: server.Port,
		UUID:       user.UUID,
		Security:   "auto",
		AlterID:    0,
	}

	p.appendTLS(&outbound, server, ps)
	p.appendMultiplex(&outbound, ps)
	if transport := buildTransport(ps, server); transport != nil {
		outbound.Transport = transport
	}
	return outbound
}

func (p *SingBoxProtocol) buildVless(server *ServerConfig, user *ClientInfo, ps map[string]interface{}) SingBoxOutbound {
	outbound := SingBoxOutbound{
		Tag:            server.Name,
		Type:           "vless",
		Server:          server.Host,
		ServerPort:     server.Port,
		UUID:           user.UUID,
		PacketEncoding: "xudp",
	}
	if flow, ok := ps["flow"].(string); ok {
		outbound.Flow = flow
	}

	p.appendTLS(&outbound, server, ps)
	p.appendMultiplex(&outbound, ps)
	if transport := buildTransport(ps, server); transport != nil {
		outbound.Transport = transport
	}
	return outbound
}

func (p *SingBoxProtocol) buildTrojan(server *ServerConfig, user *ClientInfo, ps map[string]interface{}) SingBoxOutbound {
	outbound := SingBoxOutbound{
		Tag:        server.Name,
		Type:       "trojan",
		Server:      server.Host,
		ServerPort: server.Port,
		Password:   user.UUID,
	}

	tlsMode := 1
	if t, ok := ps["tls"]; ok {
		switch v := t.(type) {
		case float64:
			tlsMode = int(v)
		case int:
			tlsMode = v
		case bool:
			if !v {
				tlsMode = 0
			}
		}
	}

	if tlsMode > 0 {
		tls := &SingBoxTLS{Enabled: true}
		if tlsMode == 2 {
			tls.Insecure = getBool(ps, "reality_settings.allow_insecure")
			tls.ServerName = getStr(ps, "reality_settings.server_name")
			tls.Reality = map[string]interface{}{
				"enabled":    true,
				"public_key": getStr(ps, "reality_settings.public_key"),
				"short_id":   getStr(ps, "reality_settings.short_id"),
			}
		} else {
			tls.Insecure = getBool(ps, "tls_settings.allow_insecure")
			if sni, ok := ps["tls_settings"].(map[string]interface{}); ok {
				if serverName, ok := sni["server_name"].(string); ok {
					tls.ServerName = serverName
				}
			}
		}
		outbound.TLS = tls
	}

	p.appendMultiplex(&outbound, ps)
	if transport := buildTransport(ps, server); transport != nil {
		outbound.Transport = transport
	}
	return outbound
}

func (p *SingBoxProtocol) buildHysteria(server *ServerConfig, user *ClientInfo, ps map[string]interface{}) SingBoxOutbound {
	version := 2
	if v, ok := ps["version"]; ok {
		switch val := v.(type) {
		case float64:
			version = int(val)
		case int:
			version = val
		}
	}

	outbound := SingBoxOutbound{
		Server:      server.Host,
		ServerPort: server.Port,
		Tag:        server.Name,
		TLS: &SingBoxTLS{
			Enabled:  true,
			Insecure: false,
		},
	}

	if sni, ok := ps["tls"].(map[string]interface{}); ok {
		if insecure, ok := sni["allow_insecure"].(bool); ok {
			outbound.TLS.Insecure = insecure
		}
		if serverName, ok := sni["server_name"].(string); ok {
			outbound.TLS.ServerName = serverName
		}
	}

	if up, ok := ps["bandwidth"].(map[string]interface{}); ok {
		if upMbps, ok := up["up"]; ok {
			outbound.UpMbps = toInt(upMbps)
		}
		if downMbps, ok := up["down"]; ok {
			outbound.DownMbps = toInt(downMbps)
		}
	}

	if version == 2 {
		outbound.Type = "hysteria2"
		outbound.Password = user.UUID
		if obfs, ok := ps["obfs"].(map[string]interface{}); ok {
			if open, ok := obfs["open"].(bool); ok && open {
				outbound.Obfs = map[string]interface{}{
					"type":     getStr(obfs, "type"),
					"password": getStr(obfs, "password"),
				}
			}
		}
	} else {
		outbound.Type = "hysteria"
		outbound.AuthStr = user.UUID
		outbound.DisableMTU = true
		if obfs, ok := ps["obfs"].(map[string]interface{}); ok {
			if pwd, ok := obfs["password"].(string); ok {
				outbound.Obfs = map[string]interface{}{
					"password": pwd,
				}
			}
		}
	}
	return outbound
}

func (p *SingBoxProtocol) buildTUIC(server *ServerConfig, user *ClientInfo, ps map[string]interface{}) SingBoxOutbound {
	congestion := "cubic"
	if cc, ok := ps["congestion_control"].(string); ok {
		congestion = cc
	}
	udpRelay := "native"
	if ur, ok := ps["udp_relay_mode"].(string); ok {
		udpRelay = ur
	}

	outbound := SingBoxOutbound{
		Type:         "tuic",
		Tag:          server.Name,
		Server:        server.Host,
		ServerPort:   server.Port,
		Congestion:   congestion,
		UDPRelayMode: udpRelay,
		ZeroRTT:      true,
		Heartbeat:    "10s",
		TLS: &SingBoxTLS{
			Enabled:  true,
			Insecure: false,
			ALPN:     []string{"h3"},
		},
	}

	if sni, ok := ps["tls"].(map[string]interface{}); ok {
		if insecure, ok := sni["allow_insecure"].(bool); ok {
			outbound.TLS.Insecure = insecure
		}
		if serverName, ok := sni["server_name"].(string); ok {
			outbound.TLS.ServerName = serverName
		}
	}

	ver := 0
	if v, ok := ps["version"]; ok {
		switch val := v.(type) {
		case float64:
			ver = int(val)
		case int:
			ver = val
		}
	}

	if ver == 4 {
		outbound.Token = user.UUID
	} else {
		outbound.UUID = user.UUID
		outbound.Password = user.UUID
	}

	return outbound
}

func (p *SingBoxProtocol) buildAnyTLS(server *ServerConfig, user *ClientInfo, ps map[string]interface{}) SingBoxOutbound {
	outbound := SingBoxOutbound{
		Type:       "anytls",
		Tag:        server.Name,
		Server:      server.Host,
		Password:   user.UUID,
		ServerPort: server.Port,
		TLS: &SingBoxTLS{
			Enabled:  true,
			Insecure: false,
			ALPN:     []string{"h3"},
		},
	}
	if sni, ok := ps["tls"].(map[string]interface{}); ok {
		if insecure, ok := sni["allow_insecure"].(bool); ok {
			outbound.TLS.Insecure = insecure
		}
		if serverName, ok := sni["server_name"].(string); ok {
			outbound.TLS.ServerName = serverName
		}
	}
	return outbound
}

func (p *SingBoxProtocol) buildSocks(server *ServerConfig, user *ClientInfo, ps map[string]interface{}) SingBoxOutbound {
	outbound := SingBoxOutbound{
		Type:       "socks",
		Tag:        server.Name,
		Server:      server.Host,
		ServerPort: server.Port,
		Version:    "5",
		Username:   user.UUID,
		Password:   user.UUID,
	}
	if udpOverTCP, ok := ps["udp_over_tcp"].(bool); ok && udpOverTCP {
		outbound.ZeroRTT = true
	}
	return outbound
}

func (p *SingBoxProtocol) buildHTTP(server *ServerConfig, user *ClientInfo, ps map[string]interface{}) SingBoxOutbound {
	outbound := SingBoxOutbound{
		Type:       "http",
		Tag:        server.Name,
		Server:      server.Host,
		ServerPort: server.Port,
		Username:   user.UUID,
		Password:   user.UUID,
	}
	if path, ok := ps["path"].(string); ok {
		outbound.Path = path
	}
	if tls, ok := ps["tls"].(bool); ok && tls {
		tlsCfg := &SingBoxTLS{Enabled: true, Insecure: false}
		if sni, ok := ps["tls_settings"].(map[string]interface{}); ok {
			if insecure, ok := sni["allow_insecure"].(bool); ok {
				tlsCfg.Insecure = insecure
			}
			if serverName, ok := sni["server_name"].(string); ok {
				tlsCfg.ServerName = serverName
			}
		}
		outbound.TLS = tlsCfg
	}
	return outbound
}

func (p *SingBoxProtocol) appendTLS(outbound *SingBoxOutbound, server *ServerConfig, ps map[string]interface{}) {
	tlsVal, hasTLS := ps["tls"]
	if !hasTLS {
		return
	}

	tlsMode := 0
	switch v := tlsVal.(type) {
	case bool:
		if v {
			tlsMode = 1
		}
	case float64:
		tlsMode = int(v)
	case int:
		tlsMode = v
	}

	if tlsMode == 0 {
		return
	}

	tls := &SingBoxTLS{Enabled: true}

	switch tlsMode {
	case 2: // Reality
		tls.Insecure = getBool(ps, "reality_settings.allow_insecure")
		tls.ServerName = getStr(ps, "reality_settings.server_name")
		tls.Reality = map[string]interface{}{
			"enabled":    true,
			"public_key": getStr(ps, "reality_settings.public_key"),
			"short_id":   getStr(ps, "reality_settings.short_id"),
		}
	default: // Standard TLS
		tls.Insecure = getBool(ps, "tls_settings.allow_insecure")
		if sni, ok := ps["tls_settings"].(map[string]interface{}); ok {
			if serverName, ok := sni["server_name"].(string); ok {
				tls.ServerName = serverName
			}
		}
	}

	outbound.TLS = tls
}

func (p *SingBoxProtocol) appendMultiplex(outbound *SingBoxOutbound, ps map[string]interface{}) {
	multiplexVal, ok := ps["multiplex"]
	if !ok {
		return
	}
	multiplex, ok := multiplexVal.(map[string]interface{})
	if !ok {
		return
	}
	enabled, ok := multiplex["enabled"].(bool)
	if !ok || !enabled {
		return
	}

	m := map[string]interface{}{
		"enabled":  true,
		"protocol": "yamux",
	}
	if proto, ok := multiplex["protocol"].(string); ok {
		m["protocol"] = proto
	}
	if mc, ok := multiplex["max_connections"]; ok {
		m["max_connections"] = mc
	}
	if ms, ok := multiplex["min_streams"]; ok {
		m["min_streams"] = ms
	}
	if ms, ok := multiplex["max_streams"]; ok {
		m["max_streams"] = ms
	}
	if padding, ok := multiplex["padding"].(bool); ok {
		m["padding"] = padding
	}
	if brutal, ok := multiplex["brutal"].(map[string]interface{}); ok {
		if enabled, ok := brutal["enabled"].(bool); ok && enabled {
			brutalConfig := map[string]interface{}{
				"enabled": true,
			}
			if up, ok := brutal["up_mbps"]; ok {
				brutalConfig["up_mbps"] = up
			}
			if down, ok := brutal["down_mbps"]; ok {
				brutalConfig["down_mbps"] = down
			}
			m["brutal"] = brutalConfig
		}
	}

	// Remove nil values
	for k, v := range m {
		if v == nil {
			delete(m, k)
		}
	}
	outbound.Multiplex = m
}

// buildTransport creates transport config for VMess/VLESS/Trojan
func buildTransport(ps map[string]interface{}, server *ServerConfig) map[string]interface{} {
	network, ok := ps["network"].(string)
	if !ok || network == "" {
		return nil
	}

	ns, _ := ps["network_settings"].(map[string]interface{})

	var transport map[string]interface{}

	switch network {
	case "tcp":
		if ns != nil {
			if header, ok := ns["header"].(map[string]interface{}); ok {
				if hType, ok := header["type"].(string); ok && hType == "http" {
					paths := []string{"/"}
					var req map[string]interface{}
					if r, ok := header["request"].(map[string]interface{}); ok {
						req = r
						if p, ok := r["path"].([]interface{}); ok && len(p) > 0 {
							paths = make([]string, len(p))
							for i, v := range p {
								paths[i] = fmt.Sprintf("%v", v)
							}
						}
					}
					transport = map[string]interface{}{
						"type": "http",
						"path": paths[0],
					}
					if req != nil {
						if reqHeaders, ok := req["headers"].(map[string]interface{}); ok {
							if host, ok := reqHeaders["Host"].([]interface{}); ok && len(host) > 0 {
								hosts := make([]string, len(host))
								for i, v := range host {
									hosts[i] = fmt.Sprintf("%v", v)
								}
								transport["host"] = hosts
							}
						}
					}
				}
			}
		}
	case "ws":
		t := map[string]interface{}{"type": "ws"}
		if ns != nil {
			if path, ok := ns["path"].(string); ok {
				t["path"] = path
			}
			if headers, ok := ns["headers"].(map[string]interface{}); ok {
				if host, ok := headers["Host"].(string); ok {
					t["headers"] = map[string]string{"Host": host}
				}
			}
			t["max_early_data"] = 0
		}
		transport = t
	case "grpc":
		t := map[string]interface{}{"type": "grpc"}
		if ns != nil {
			if sn, ok := ns["serviceName"].(string); ok {
				t["service_name"] = sn
			}
		}
		transport = t
	case "h2":
		t := map[string]interface{}{"type": "http"}
		if ns != nil {
			if host, ok := ns["host"]; ok {
				t["host"] = host
			}
			if path, ok := ns["path"].(string); ok {
				t["path"] = path
			}
		}
		transport = t
	case "httpupgrade":
		t := map[string]interface{}{"type": "httpupgrade"}
		if ns != nil {
			if path, ok := ns["path"].(string); ok {
				t["path"] = path
			}
			host := server.Host
			if h, ok := ns["host"].(string); ok && h != "" {
				host = h
			}
			t["host"] = host
			if headers, ok := ns["headers"]; ok {
				t["headers"] = headers
			}
		}
		transport = t
	case "quic":
		transport = map[string]interface{}{"type": "quic"}
	}

	if transport == nil {
		return nil
	}
	// Remove nil values
	for k, v := range transport {
		if v == nil {
			delete(transport, k)
		}
	}
	return transport
}

func toInt(v interface{}) int {
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case int64:
		return int(val)
	}
	return 0
}
