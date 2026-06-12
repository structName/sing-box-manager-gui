package parser

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/structName/sing-box-manager-gui/internal/storage"
	"github.com/structName/sing-box-manager-gui/pkg/utils"
)

// ShadowsocksParser Shadowsocks 解析器
type ShadowsocksParser struct{}

// Protocol 返回协议名称
func (p *ShadowsocksParser) Protocol() string {
	return "shadowsocks"
}

// Parse 解析 Shadowsocks URL
// 格式1 (SIP002): ss://BASE64(method:password)@server:port#name
// 格式2 (Legacy): ss://BASE64(method:password@server:port)#name
func (p *ShadowsocksParser) Parse(rawURL string) (*storage.Node, error) {
	// 去除协议头
	rawURL = strings.TrimPrefix(rawURL, "ss://")

	// 分离 fragment (#name)
	var name string
	if idx := strings.Index(rawURL, "#"); idx != -1 {
		name, _ = url.QueryUnescape(rawURL[idx+1:])
		rawURL = rawURL[:idx]
	}

	var method, password, server string
	var port int

	// 尝试 SIP002 格式: BASE64@server:port
	if atIdx := strings.LastIndex(rawURL, "@"); atIdx != -1 {
		// 新格式
		userInfo := rawURL[:atIdx]
		serverPart := rawURL[atIdx+1:]

		// 解析服务器信息
		var err error
		server, port, err = parseServerInfo(serverPart)
		if err != nil {
			return nil, fmt.Errorf("解析服务器地址失败: %w", err)
		}

		// 解码用户信息
		decoded, err := utils.DecodeBase64(userInfo)
		if err != nil {
			// 可能是 URL 编码的
			decoded, err = url.QueryUnescape(userInfo)
			if err != nil {
				return nil, fmt.Errorf("解码用户信息失败: %w", err)
			}
		}

		// 分离 method:password
		colonIdx := strings.Index(decoded, ":")
		if colonIdx == -1 {
			return nil, fmt.Errorf("无效的用户信息格式")
		}
		method = decoded[:colonIdx]
		password = decoded[colonIdx+1:]
	} else {
		// 旧格式: BASE64(method:password@server:port)
		decoded, err := utils.DecodeBase64(rawURL)
		if err != nil {
			return nil, fmt.Errorf("解码失败: %w", err)
		}

		// 分离 method:password@server:port
		atIdx := strings.LastIndex(decoded, "@")
		if atIdx == -1 {
			return nil, fmt.Errorf("无效的 URL 格式")
		}

		userInfo := decoded[:atIdx]
		serverPart := decoded[atIdx+1:]

		// 解析服务器信息
		server, port, err = parseServerInfo(serverPart)
		if err != nil {
			return nil, fmt.Errorf("解析服务器地址失败: %w", err)
		}

		// 分离 method:password
		colonIdx := strings.Index(userInfo, ":")
		if colonIdx == -1 {
			return nil, fmt.Errorf("无效的用户信息格式")
		}
		method = userInfo[:colonIdx]
		password = userInfo[colonIdx+1:]
	}

	// 设置默认名称
	if name == "" {
		name = fmt.Sprintf("%s:%d", server, port)
	}

	node := &storage.Node{
		Tag:        name,
		Type:       "shadowsocks",
		Server:     server,
		ServerPort: port,
		Extra: map[string]interface{}{
			"method":   method,
			"password": password,
		},
	}

	return node, nil
}
