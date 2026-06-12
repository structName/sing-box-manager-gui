package parser

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/structName/sing-box-manager-gui/internal/storage"
)

// Hysteria2Parser Hysteria2 解析器
type Hysteria2Parser struct{}

// Protocol 返回协议名称
func (p *Hysteria2Parser) Protocol() string {
	return "hysteria2"
}

// Parse 解析 Hysteria2 URL
// 格式1: hysteria2://password@server:port?params#name
// 格式2: hysteria2://server:port?auth=password&params#name
func (p *Hysteria2Parser) Parse(rawURL string) (*storage.Node, error) {
	addressPart, params, name, err := parseURLParams(rawURL)
	if err != nil {
		return nil, err
	}

	var password, server string
	var port int

	// 判断格式
	if strings.Contains(addressPart, "@") {
		// 格式1: password@server:port
		atIdx := strings.Index(addressPart, "@")
		password, _ = url.QueryUnescape(addressPart[:atIdx])
		serverPart := addressPart[atIdx+1:]

		server, port, err = parseServerInfo(serverPart)
		if err != nil {
			return nil, fmt.Errorf("解析服务器地址失败: %w", err)
		}
	} else {
		// 格式2: server:port (password 在参数中)
		server, port, err = parseServerInfo(addressPart)
		if err != nil {
			return nil, fmt.Errorf("解析服务器地址失败: %w", err)
		}
		password = params.Get("auth")
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

	extra["tls"] = tls

	// 混淆配置
	if obfsPassword := params.Get("obfs-password"); obfsPassword != "" {
		obfs := map[string]interface{}{
			"type":     getParamString(params, "obfs", "salamander"),
			"password": obfsPassword,
		}
		extra["obfs"] = obfs
	}

	// 带宽配置
	if up := params.Get("upmbps"); up != "" {
		extra["up_mbps"] = getParamInt(params, "upmbps", 0)
	} else if up := params.Get("up"); up != "" {
		extra["up"] = up
	}

	if down := params.Get("downmbps"); down != "" {
		extra["down_mbps"] = getParamInt(params, "downmbps", 0)
	} else if down := params.Get("down"); down != "" {
		extra["down"] = down
	}

	// 端口跳跃
	if ports := params.Get("mport"); ports != "" {
		extra["ports"] = ports
	}

	// 跳跃间隔
	if hopInterval := params.Get("hop-interval"); hopInterval != "" {
		extra["hop_interval"] = hopInterval
	}

	node := &storage.Node{
		Tag:        name,
		Type:       "hysteria2",
		Server:     server,
		ServerPort: port,
		Extra:      extra,
	}

	return node, nil
}
