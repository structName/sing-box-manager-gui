package parser

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/structName/sing-box-manager-gui/internal/storage"
)

// TuicParser TUIC 解析器
type TuicParser struct{}

// Protocol 返回协议名称
func (p *TuicParser) Protocol() string {
	return "tuic"
}

// Parse 解析 TUIC URL
// 格式: tuic://uuid:password@server:port?params#name
func (p *TuicParser) Parse(rawURL string) (*storage.Node, error) {
	addressPart, params, name, err := parseURLParams(rawURL)
	if err != nil {
		return nil, err
	}

	// 分离 userinfo 和服务器信息
	atIdx := strings.LastIndex(addressPart, "@")
	if atIdx == -1 {
		return nil, fmt.Errorf("无效的 TUIC URL 格式")
	}

	userInfo, _ := url.QueryUnescape(addressPart[:atIdx])
	serverPart := addressPart[atIdx+1:]

	// 解析服务器地址
	server, port, err := parseServerInfo(serverPart)
	if err != nil {
		return nil, fmt.Errorf("解析服务器地址失败: %w", err)
	}

	// 解析 uuid:password
	var uuid, password string
	colonIdx := strings.Index(userInfo, ":")
	if colonIdx == -1 {
		uuid = userInfo
		password = params.Get("password")
	} else {
		uuid = userInfo[:colonIdx]
		password = userInfo[colonIdx+1:]
	}

	// 设置默认名称
	if name == "" {
		name = fmt.Sprintf("%s:%d", server, port)
	}

	// 构建 Extra
	extra := map[string]interface{}{
		"uuid":     uuid,
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
	if getParamBool(params, "insecure") || getParamBool(params, "allowInsecure") || getParamBool(params, "skip-cert-verify") {
		tls["insecure"] = true
	}

	// ALPN
	if alpn := params.Get("alpn"); alpn != "" {
		tls["alpn"] = strings.Split(alpn, ",")
	}

	// 禁用 SNI
	if getParamBool(params, "disable-sni") {
		tls["disable_sni"] = true
	}

	extra["tls"] = tls

	// 拥塞控制
	if cc := params.Get("congestion_control"); cc != "" {
		extra["congestion_control"] = cc
	} else if cc := params.Get("congestion-control"); cc != "" {
		extra["congestion_control"] = cc
	}

	// UDP 中继模式
	if mode := params.Get("udp-relay-mode"); mode != "" {
		extra["udp_relay_mode"] = mode
	} else if mode := params.Get("udp_relay_mode"); mode != "" {
		extra["udp_relay_mode"] = mode
	}

	// 零 RTT
	if getParamBool(params, "zero-rtt") || getParamBool(params, "reduce-rtt") {
		extra["zero_rtt_handshake"] = true
	}

	// 心跳
	if heartbeat := params.Get("heartbeat"); heartbeat != "" {
		extra["heartbeat"] = heartbeat
	}

	node := &storage.Node{
		Tag:        name,
		Type:       "tuic",
		Server:     server,
		ServerPort: port,
		Extra:      extra,
	}

	return node, nil
}
