package parser

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/structName/sing-box-manager-gui/internal/storage"
	"github.com/structName/sing-box-manager-gui/pkg/utils"
)

// VmessParser VMess 解析器
type VmessParser struct{}

// Protocol 返回协议名称
func (p *VmessParser) Protocol() string {
	return "vmess"
}

// vmessConfig VMess 配置结构 (v2rayN Base64 JSON)
type vmessConfig struct {
	V    interface{} `json:"v"`                // 版本
	Ps   string      `json:"ps"`               // 节点名称
	Add  string      `json:"add"`              // 服务器地址
	Port interface{} `json:"port"`             // 端口
	ID   string      `json:"id"`               // UUID
	Aid  interface{} `json:"aid"`              // Alter ID
	Scy  string      `json:"scy"`              // 加密方式
	Net  string      `json:"net"`              // 传输协议
	Type string      `json:"type"`             // 伪装类型
	Host string      `json:"host"`             // 伪装域名
	Path string      `json:"path"`             // 路径
	TLS  string      `json:"tls"`              // TLS
	SNI  string      `json:"sni"`              // SNI
	ALPN string      `json:"alpn"`             // ALPN
	Fp   string      `json:"fp"`               // Fingerprint
	Skip bool        `json:"skip-cert-verify"` // 跳过证书验证
}

// Parse 解析 VMess URL
// 格式1 (v2rayN): vmess://BASE64(json)#name
// 格式2 (VMessAEAD / Xray #716): vmess://uuid@server:port?params#name
func (p *VmessParser) Parse(rawURL string) (*storage.Node, error) {
	// 去除协议头
	body := strings.TrimPrefix(rawURL, "vmess://")

	// 分离 fragment (#name)
	var fragmentName string
	if idx := strings.Index(body, "#"); idx != -1 {
		fragmentName, _ = url.QueryUnescape(body[idx+1:])
		body = body[:idx]
	}

	// Prefer classic Base64 JSON; fall back to VMessAEAD URI when decode/JSON fails.
	if node, err := p.parseBase64JSON(body, fragmentName); err == nil {
		return node, nil
	}

	if strings.Contains(body, "@") {
		return p.parseAEADURI(body, fragmentName)
	}

	return nil, fmt.Errorf("无效的 VMess URL 格式")
}

func (p *VmessParser) parseBase64JSON(body, fragmentName string) (*storage.Node, error) {
	decoded, err := utils.DecodeBase64(body)
	if err != nil {
		return nil, fmt.Errorf("Base64 解码失败: %w", err)
	}

	var config vmessConfig
	if err := json.Unmarshal([]byte(decoded), &config); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}

	// 获取端口
	var port int
	switch v := config.Port.(type) {
	case float64:
		port = int(v)
	case string:
		port, _ = strconv.Atoi(v)
	case int:
		port = v
	}

	// 获取 Alter ID
	var alterId int
	switch v := config.Aid.(type) {
	case float64:
		alterId = int(v)
	case string:
		alterId, _ = strconv.Atoi(v)
	case int:
		alterId = v
	}

	// 设置名称
	name := config.Ps
	if fragmentName != "" {
		name = fragmentName
	}
	if name == "" {
		name = fmt.Sprintf("%s:%d", config.Add, port)
	}

	// 构建 Extra
	extra := map[string]interface{}{
		"uuid":     config.ID,
		"alter_id": alterId,
		"security": config.Scy,
	}

	// 设置默认加密方式
	if config.Scy == "" {
		extra["security"] = "auto"
	}

	// 传输层配置
	network := config.Net
	if network == "" {
		network = "tcp"
	}

	// 构建传输配置
	if network != "tcp" || config.Type == "http" {
		transport := map[string]interface{}{
			"type": network,
		}

		switch network {
		case "ws":
			if config.Path != "" {
				transport["path"] = config.Path
			}
			if config.Host != "" {
				transport["headers"] = map[string]string{
					"Host": config.Host,
				}
			}
		case "http", "h2":
			if config.Path != "" {
				transport["path"] = config.Path
			}
			if config.Host != "" {
				transport["host"] = strings.Split(config.Host, ",")
			}
		case "grpc":
			if config.Path != "" {
				transport["service_name"] = config.Path
			}
		case "quic":
			if config.Type != "" {
				transport["security"] = config.Type
			}
		}

		extra["transport"] = transport
	}

	// TLS 配置
	if config.TLS == "tls" {
		tls := map[string]interface{}{
			"enabled": true,
		}
		// 设置 server_name（按优先级：SNI > Host > 服务器地址）
		if config.SNI != "" {
			tls["server_name"] = config.SNI
		} else if config.Host != "" {
			tls["server_name"] = config.Host
		} else {
			// 如果 SNI 和 Host 都为空，使用服务器地址作为默认 server_name
			// 这是为了确保 TLS 握手时有正确的 SNI
			tls["server_name"] = config.Add
		}
		if config.Skip {
			tls["insecure"] = true
		}
		if config.Fp != "" {
			tls["utls"] = map[string]interface{}{
				"enabled":     true,
				"fingerprint": config.Fp,
			}
		}
		if config.ALPN != "" {
			tls["alpn"] = strings.Split(config.ALPN, ",")
		}
		extra["tls"] = tls
	}

	node := &storage.Node{
		Tag:        name,
		Type:       "vmess",
		Server:     config.Add,
		ServerPort: port,
		Extra:      extra,
	}

	return node, nil
}

// parseAEADURI parses Xray VMessAEAD share links (Discussion #716):
// vmess://uuid@host:port?encryption=auto&type=ws&security=tls&...#name
func (p *VmessParser) parseAEADURI(body, fragmentName string) (*storage.Node, error) {
	// Reassemble a synthetic URL so parseURLParams can split query/fragment.
	// fragmentName was already stripped; attach query from body.
	raw := "vmess://" + body
	addressPart, params, name, err := parseURLParams(raw)
	if err != nil {
		return nil, err
	}
	if fragmentName != "" {
		name = fragmentName
	}

	atIdx := strings.Index(addressPart, "@")
	if atIdx == -1 {
		return nil, fmt.Errorf("无效的 VMessAEAD URL 格式")
	}

	uuid, _ := url.QueryUnescape(addressPart[:atIdx])
	serverPart := addressPart[atIdx+1:]
	// Common form uses a trailing slash before '?'
	serverPart = strings.TrimSuffix(serverPart, "/")

	server, port, err := parseServerInfo(serverPart)
	if err != nil {
		return nil, fmt.Errorf("解析服务器地址失败: %w", err)
	}
	if uuid == "" {
		return nil, fmt.Errorf("缺少 UUID")
	}

	if name == "" {
		name = fmt.Sprintf("%s:%d", server, port)
	}

	securityMethod := getParamString(params, "encryption", "auto")
	if securityMethod == "" {
		securityMethod = "auto"
	}

	extra := map[string]interface{}{
		"uuid":     uuid,
		"alter_id": 0, // VMessAEAD always uses alterId 0
		"security": securityMethod,
	}

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
		case "httpupgrade", "http_upgrade":
			transport["type"] = "httpupgrade"
			if path := params.Get("path"); path != "" {
				transport["path"] = path
			}
			if host := params.Get("host"); host != "" {
				transport["host"] = host
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
			} else if serviceName := params.Get("service_name"); serviceName != "" {
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

	security := getParamString(params, "security", "none")
	if security != "none" {
		tls := map[string]interface{}{
			"enabled": true,
		}

		if sni := params.Get("sni"); sni != "" {
			tls["server_name"] = sni
		} else if host := params.Get("host"); host != "" {
			tls["server_name"] = host
		} else {
			tls["server_name"] = server
		}

		if getParamBool(params, "allowInsecure") || getParamBool(params, "insecure") || getParamBool(params, "skip-cert-verify") {
			tls["insecure"] = true
		}

		if alpn := params.Get("alpn"); alpn != "" {
			tls["alpn"] = strings.Split(alpn, ",")
		}

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

			fp := getParamString(params, "fp", "chrome")
			tls["utls"] = map[string]interface{}{
				"enabled":     true,
				"fingerprint": fp,
			}
		} else if fp := params.Get("fp"); fp != "" {
			tls["utls"] = map[string]interface{}{
				"enabled":     true,
				"fingerprint": fp,
			}
		}

		extra["tls"] = tls
	}

	return &storage.Node{
		Tag:        name,
		Type:       "vmess",
		Server:     server,
		ServerPort: port,
		Extra:      extra,
	}, nil
}
