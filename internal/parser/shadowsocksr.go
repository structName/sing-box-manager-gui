package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xiaobei/singbox-manager/internal/storage"
	"github.com/xiaobei/singbox-manager/pkg/utils"
)

// ShadowsocksRParser ShadowsocksR 解析器
type ShadowsocksRParser struct{}

// Protocol 返回协议名称
func (p *ShadowsocksRParser) Protocol() string {
	return "shadowsocksr"
}

// Parse 解析 ShadowsocksR URL
// 格式: ssr://BASE64(server:port:protocol:method:obfs:BASE64(password)/?params)
// 参数: obfsparam, protoparam, remarks, group (均为 BASE64 编码)
func (p *ShadowsocksRParser) Parse(rawURL string) (*storage.Node, error) {
	// 去除协议头
	rawURL = strings.TrimPrefix(rawURL, "ssr://")

	// 整体 Base64 解码
	decoded, err := utils.DecodeBase64(rawURL)
	if err != nil {
		return nil, fmt.Errorf("解码 SSR URL 失败: %w", err)
	}

	// 分离主体和参数部分: server:port:protocol:method:obfs:base64pass/?params
	var mainPart, paramPart string
	if idx := strings.Index(decoded, "/?"); idx != -1 {
		mainPart = decoded[:idx]
		paramPart = decoded[idx+2:]
	} else if idx := strings.Index(decoded, "?"); idx != -1 {
		mainPart = decoded[:idx]
		paramPart = decoded[idx+1:]
	} else {
		mainPart = decoded
	}

	// 解析主体: server:port:protocol:method:obfs:base64password
	// 注意 server 可能是 IPv6 地址，所以需要特殊处理
	var parts []string
	if strings.HasPrefix(mainPart, "[") {
		// IPv6 地址以 [ 开头，必须用 splitSSRMain 处理
		parts = splitSSRMain(mainPart)
	} else {
		parts = strings.SplitN(mainPart, ":", 6)
	}
	if len(parts) < 6 {
		// 回退：尝试 splitSSRMain 从右侧分割
		parts = splitSSRMain(mainPart)
		if len(parts) < 6 {
			return nil, fmt.Errorf("无效的 SSR URL 格式，字段数不足: %d", len(parts))
		}
	}

	server := parts[0]
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("无效的端口: %s", parts[1])
	}
	protocol := parts[2]
	method := parts[3]
	obfs := parts[4]
	passwordBase64 := parts[5]

	// 解码密码 (URL-safe Base64)
	password, err := utils.DecodeBase64(passwordBase64)
	if err != nil {
		return nil, fmt.Errorf("解码密码失败: %w", err)
	}

	// 解析可选参数
	params := parseSSRParams(paramPart)

	obfsParam := decodeSSRParam(params["obfsparam"])
	protocolParam := decodeSSRParam(params["protoparam"])
	remarks := decodeSSRParam(params["remarks"])
	// group := decodeSSRParam(params["group"])

	// 设置默认名称
	if remarks == "" {
		remarks = fmt.Sprintf("%s:%d", server, port)
	}

	extra := map[string]interface{}{
		"method":   method,
		"password": password,
	}
	if protocol != "" {
		extra["protocol"] = protocol
	}
	if protocolParam != "" {
		extra["protocol_param"] = protocolParam
	}
	if obfs != "" {
		extra["obfs"] = obfs
	}
	if obfsParam != "" {
		extra["obfs_param"] = obfsParam
	}

	node := &storage.Node{
		Tag:        remarks,
		Type:       "shadowsocksr",
		Server:     server,
		ServerPort: port,
		Extra:      extra,
	}

	return node, nil
}

// splitSSRMain 处理包含 IPv6 地址的 SSR 主体部分
// IPv6 示例: [::1]:port:protocol:method:obfs:password
func splitSSRMain(main string) []string {
	// 如果以 [ 开头，说明是 IPv6
	if strings.HasPrefix(main, "[") {
		bracketEnd := strings.Index(main, "]")
		if bracketEnd == -1 {
			return nil
		}
		server := main[1:bracketEnd]
		rest := main[bracketEnd+1:]
		// rest 应该是 :port:protocol:method:obfs:password
		rest = strings.TrimPrefix(rest, ":")
		parts := strings.SplitN(rest, ":", 5)
		if len(parts) < 5 {
			return nil
		}
		return append([]string{server}, parts...)
	}

	// 非 IPv6，尝试从右侧反向分割
	// 格式: server:port:protocol:method:obfs:password
	// server 可能包含多个冒号（虽然不常见）
	// 标准做法：从右往左取 5 个冒号分隔的字段，剩余的全是 server
	result := make([]string, 6)
	remaining := main
	for i := 5; i > 0; i-- {
		lastColon := strings.LastIndex(remaining, ":")
		if lastColon == -1 {
			return strings.SplitN(main, ":", 6) // 回退到简单分割
		}
		result[i] = remaining[lastColon+1:]
		remaining = remaining[:lastColon]
	}
	result[0] = remaining
	return result
}

// parseSSRParams 解析 SSR URL 中的查询参数
func parseSSRParams(paramStr string) map[string]string {
	params := make(map[string]string)
	if paramStr == "" {
		return params
	}

	pairs := strings.Split(paramStr, "&")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			params[kv[0]] = kv[1]
		}
	}
	return params
}

// decodeSSRParam 解码 SSR 参数值 (URL-safe Base64)
func decodeSSRParam(value string) string {
	if value == "" {
		return ""
	}
	decoded, err := utils.DecodeBase64(value)
	if err != nil {
		return value // 解码失败则返回原值
	}
	return decoded
}
