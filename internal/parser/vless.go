package parser

import (
	"fmt"
	"strings"

	"github.com/structName/sing-box-manager-gui/internal/storage"
)

// VlessParser VLESS 解析器
type VlessParser struct{}

// Protocol 返回协议名称
func (p *VlessParser) Protocol() string {
	return "vless"
}

// Parse 解析 VLESS URL
// 格式: vless://uuid@server:port?params#name
func (p *VlessParser) Parse(rawURL string) (*storage.Node, error) {
	addressPart, params, name, err := parseURLParams(rawURL)
	if err != nil {
		return nil, err
	}

	// 分离 uuid 和服务器信息
	atIdx := strings.Index(addressPart, "@")
	if atIdx == -1 {
		return nil, fmt.Errorf("无效的 VLESS URL 格式")
	}

	uuid := addressPart[:atIdx]
	serverPart := addressPart[atIdx+1:]

	// 解析服务器地址
	server, port, err := parseServerInfo(serverPart)
	if err != nil {
		return nil, fmt.Errorf("解析服务器地址失败: %w", err)
	}

	// 设置默认名称
	if name == "" {
		name = fmt.Sprintf("%s:%d", server, port)
	}

	// 构建 Extra
	extra := map[string]interface{}{
		"uuid": uuid,
	}

	// Flow 配置
	if flow := params.Get("flow"); flow != "" {
		extra["flow"] = flow
	}

	// 传输层配置
	transportType := getParamString(params, "type", "tcp")
	if transportType != "tcp" {
		transport := map[string]interface{}{
			"type": transportType,
		}

		switch transportType {
		case "ws":
			if path := params.Get("path"); path != "" {
				transport["path"] = path
			}
			if host := params.Get("host"); host != "" {
				transport["headers"] = map[string]string{
					"Host": host,
				}
			}
		case "http", "h2":
			if path := params.Get("path"); path != "" {
				transport["path"] = path
			}
			if host := params.Get("host"); host != "" {
				transport["host"] = strings.Split(host, ",")
			}
		case "grpc":
			if serviceName := params.Get("serviceName"); serviceName != "" {
				transport["service_name"] = serviceName
			}
			if mode := params.Get("mode"); mode != "" {
				transport["mode"] = mode
			}
		case "quic":
			if security := params.Get("quicSecurity"); security != "" {
				transport["security"] = security
			}
		}

		extra["transport"] = transport
	}

	// TLS/Reality 配置
	security := getParamString(params, "security", "none")
	if security != "none" {
		tls := map[string]interface{}{
			"enabled": true,
		}

		// SNI
		if sni := params.Get("sni"); sni != "" {
			tls["server_name"] = sni
		} else if host := params.Get("host"); host != "" {
			tls["server_name"] = host
		}

		// 跳过证书验证
		if getParamBool(params, "allowInsecure") || getParamBool(params, "insecure") {
			tls["insecure"] = true
		}

		// ALPN
		if alpn := params.Get("alpn"); alpn != "" {
			tls["alpn"] = strings.Split(alpn, ",")
		}

		// Reality 配置
		if security == "reality" {
			reality := map[string]interface{}{
				"enabled": true,
			}
			if pbk := params.Get("pbk"); pbk != "" {
				reality["public_key"] = pbk
			}
			if sid := params.Get("sid"); sid != "" {
				reality["short_id"] = sid
			}
			tls["reality"] = reality

			// uTLS fingerprint
			fp := getParamString(params, "fp", "chrome")
			tls["utls"] = map[string]interface{}{
				"enabled":     true,
				"fingerprint": fp,
			}
		} else if fp := params.Get("fp"); fp != "" {
			// 普通 TLS 的 uTLS
			tls["utls"] = map[string]interface{}{
				"enabled":     true,
				"fingerprint": fp,
			}
		}

		extra["tls"] = tls
	}

	node := &storage.Node{
		Tag:        name,
		Type:       "vless",
		Server:     server,
		ServerPort: port,
		Extra:      extra,
	}

	return node, nil
}
