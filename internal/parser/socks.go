package parser

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"unicode"

	"github.com/structName/sing-box-manager-gui/internal/storage"
)

// SocksParser SOCKS 解析器
type SocksParser struct{}

// Protocol 返回协议名称
func (p *SocksParser) Protocol() string {
	return "socks"
}

// Parse 解析 SOCKS URL
// 格式1: socks://username:password@server:port#name
// 格式2: socks://base64(username:password)@server:port#name
// 格式3: socks://server:port#name (无认证)
// 也支持 socks5:// 和 socks4:// 前缀
func (p *SocksParser) Parse(rawURL string) (*storage.Node, error) {
	addressPart, params, name, err := parseURLParams(rawURL)
	if err != nil {
		return nil, err
	}

	var username, password, serverPart string
	version := "5" // 默认 SOCKS5

	// 检测版本（从协议名称）
	idx := strings.Index(rawURL, "://")
	if idx != -1 {
		protocol := strings.ToLower(rawURL[:idx])
		if protocol == "socks4" || protocol == "socks4a" {
			version = "4"
		}
	}

	// 分离认证信息和服务器
	atIdx := strings.LastIndex(addressPart, "@")
	if atIdx != -1 {
		// 有认证信息
		authPart := addressPart[:atIdx]
		serverPart = addressPart[atIdx+1:]

		// 尝试解析 username:password 格式
		if colonIdx := strings.Index(authPart, ":"); colonIdx != -1 {
			// 直接格式: username:password
			username, _ = url.QueryUnescape(authPart[:colonIdx])
			password, _ = url.QueryUnescape(authPart[colonIdx+1:])
		} else {
			// 没有冒号，可能是 Base64 编码的 username:password 或者只是用户名
			// 尝试 Base64 解码
			decoded := tryBase64Decode(authPart)

			if decoded != "" && strings.Contains(decoded, ":") {
				// 解码成功且包含冒号，说明是 Base64 编码的 username:password
				colonIdx := strings.Index(decoded, ":")
				username = decoded[:colonIdx]
				password = decoded[colonIdx+1:]
			} else {
				// 解码失败或不包含冒号，当作普通用户名处理
				username, _ = url.QueryUnescape(authPart)
			}
		}
	} else {
		// 无认证信息
		serverPart = addressPart
	}

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
		"version": version,
	}

	// 添加认证信息
	if username != "" {
		extra["username"] = username
	}
	if password != "" {
		extra["password"] = password
	}

	// 处理 URL 参数中可能的额外配置
	if v := params.Get("version"); v != "" {
		extra["version"] = v
	}
	if u := params.Get("username"); u != "" {
		extra["username"] = u
	}
	if pw := params.Get("password"); pw != "" {
		extra["password"] = pw
	}

	// UoT (UDP over TCP) 配置
	if getParamBool(params, "udp-over-tcp") || getParamBool(params, "uot") {
		extra["udp_over_tcp"] = map[string]interface{}{
			"enabled": true,
		}
	}

	node := &storage.Node{
		Tag:        name,
		Type:       "socks",
		Server:     server,
		ServerPort: port,
		Extra:      extra,
	}

	return node, nil
}

// isValidUsername 检查字符串是否是有效的用户名（只包含可打印字符）
func isValidUsername(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

// tryBase64Decode 尝试多种 Base64 解码方式
func tryBase64Decode(s string) string {
	// 尝试标准 Base64
	if decoded, err := base64.StdEncoding.DecodeString(s); err == nil {
		if isValidUsername(string(decoded)) {
			return string(decoded)
		}
	}
	// 尝试 URL 安全 Base64
	if decoded, err := base64.URLEncoding.DecodeString(s); err == nil {
		if isValidUsername(string(decoded)) {
			return string(decoded)
		}
	}
	// 尝试无填充 Base64
	if decoded, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		if isValidUsername(string(decoded)) {
			return string(decoded)
		}
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		if isValidUsername(string(decoded)) {
			return string(decoded)
		}
	}
	return ""
}
