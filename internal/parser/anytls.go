package parser

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/xiaobei/singbox-manager/internal/storage"
)

// AnyTLSParser AnyTLS 解析器
type AnyTLSParser struct{}

// Protocol 返回协议名称
func (p *AnyTLSParser) Protocol() string {
	return "anytls"
}

// Parse 解析 AnyTLS URL
// 格式: anytls://password@server:port?sni=...&fp=...&params#name
func (p *AnyTLSParser) Parse(rawURL string) (*storage.Node, error) {
	addressPart, params, name, err := parseURLParams(rawURL)
	if err != nil {
		return nil, err
	}

	// 分离 password 和服务器信息
	atIdx := strings.Index(addressPart, "@")
	if atIdx == -1 {
		return nil, fmt.Errorf("无效的 AnyTLS URL 格式")
	}

	password, _ := url.QueryUnescape(addressPart[:atIdx])
	serverPart := addressPart[atIdx+1:]

	// 解析服务器地址
	server, port, err := parseServerInfo(serverPart)
	if err != nil {
		return nil, fmt.Errorf("解析服务器地址失败: %w", err)
	}

	if password == "" {
		return nil, fmt.Errorf("缺少认证密码")
	}

	// 设置默认名称
	if name == "" {
		name = fmt.Sprintf("%s:%d", server, port)
	}

	// 构建 Extra
	extra := map[string]interface{}{
		"password": password,
	}

	// TLS 配置
	tls := map[string]interface{}{
		"enabled": true,
	}

	// SNI
	if sni := params.Get("sni"); sni != "" {
		tls["server_name"] = sni
	}

	// 跳过证书验证
	if getParamBool(params, "insecure") || getParamBool(params, "allowInsecure") {
		tls["insecure"] = true
	}

	// ALPN
	if alpn := params.Get("alpn"); alpn != "" {
		tls["alpn"] = strings.Split(alpn, ",")
	}

	// uTLS fingerprint（默认 chrome，与 trojan 一致）
	fp := getParamString(params, "fp", "chrome")
	tls["utls"] = map[string]interface{}{
		"enabled":     true,
		"fingerprint": fp,
	}

	extra["tls"] = tls

	// 空闲会话管理
	if v := params.Get("idle-check-interval"); v != "" {
		extra["idle_session_check_interval"] = getParamInt(params, "idle-check-interval", 0)
	}
	if v := params.Get("idle-timeout"); v != "" {
		extra["idle_session_timeout"] = getParamInt(params, "idle-timeout", 0)
	}
	if v := params.Get("min-idle-session"); v != "" {
		extra["min_idle_session"] = getParamInt(params, "min-idle-session", 0)
	}

	node := &storage.Node{
		Tag:        name,
		Type:       "anytls",
		Server:     server,
		ServerPort: port,
		Extra:      extra,
	}

	return node, nil
}
