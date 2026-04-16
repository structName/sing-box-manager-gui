package speedtest

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/constant"
	"github.com/xiaobei/singbox-manager/internal/database/models"
	"github.com/xiaobei/singbox-manager/internal/logger"
	"gopkg.in/yaml.v3"
)

// float64ToBits 将 float64 转换为 uint64 位模式
func float64ToBits(f float64) uint64 {
	return math.Float64bits(f)
}

// float64FromBits 将 uint64 位模式转换为 float64
func float64FromBits(bits uint64) float64 {
	return math.Float64frombits(bits)
}

// TestResult 测速结果
type TestResult struct {
	NodeID    uint    `json:"node_id"`
	Delay     int     `json:"delay"`      // 延迟 (ms), -1 表示超时
	Speed     float64 `json:"speed"`      // 速度 (MB/s)
	Status    string  `json:"status"`     // success/timeout/error
	LandingIP string  `json:"landing_ip"` // 落地 IP
	Error     string  `json:"error,omitempty"`
}

// Tester 测速器
type Tester struct {
	LatencyURL         string
	SpeedURL           string
	Timeout            time.Duration
	IncludeHandshake   bool
	DetectCountry      bool
	LandingIPURL       string
	SpeedRecordMode    string // average/peak
	PeakSampleInterval int    // 峰值采样间隔 (ms)
}

// validateTestURL 验证测速 URL 安全性（防止 SSRF）
func validateTestURL(rawURL string) (string, error) {
	if rawURL == "" {
		return "", fmt.Errorf("URL 不能为空")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("无效的 URL: %w", err)
	}

	// 只允许 http/https 协议
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("不支持的协议: %s，仅支持 http/https", scheme)
	}

	// 检查是否为内网地址
	host := parsed.Hostname()
	if isPrivateAddress(host) {
		return "", fmt.Errorf("不允许访问内网地址: %s", host)
	}

	return rawURL, nil
}

// isPrivateAddress 检查是否为内网地址
func isPrivateAddress(host string) bool {
	// 常见的内网域名
	lowerHost := strings.ToLower(host)
	if lowerHost == "localhost" || strings.HasSuffix(lowerHost, ".local") {
		return true
	}

	// 解析 IP 地址
	ip := net.ParseIP(host)
	if ip == nil {
		// 不是 IP，可能是域名，允许通过
		return false
	}

	// 检查是否为私有 IP
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

// NewTester 创建测速器
func NewTester(profile *models.SpeedTestProfile) *Tester {
	timeout := time.Duration(profile.Timeout) * time.Second
	if timeout == 0 {
		timeout = 7 * time.Second
	}

	// 默认 URL
	latencyURL := "https://cp.cloudflare.com/generate_204"
	speedURL := "https://speed.cloudflare.com/__down?bytes=5000000"
	landingIPURL := "https://api.ipify.org"

	// 验证自定义 URL
	if profile.LatencyURL != "" {
		if validated, err := validateTestURL(profile.LatencyURL); err != nil {
			logger.Warn("延迟测试 URL 无效: %v, 使用默认值", err)
		} else {
			latencyURL = validated
		}
	}

	if profile.SpeedURL != "" {
		if validated, err := validateTestURL(profile.SpeedURL); err != nil {
			logger.Warn("速度测试 URL 无效: %v, 使用默认值", err)
		} else {
			speedURL = validated
		}
	}

	if profile.LandingIPURL != "" {
		if validated, err := validateTestURL(profile.LandingIPURL); err != nil {
			logger.Warn("落地 IP URL 无效: %v, 使用默认值", err)
		} else {
			landingIPURL = validated
		}
	}

	speedRecordMode := profile.SpeedRecordMode
	if speedRecordMode == "" {
		speedRecordMode = "average"
	}

	peakSampleInterval := profile.PeakSampleInterval
	if peakSampleInterval == 0 {
		peakSampleInterval = 100
	}

	return &Tester{
		LatencyURL:         latencyURL,
		SpeedURL:           speedURL,
		Timeout:            timeout,
		IncludeHandshake:   profile.IncludeHandshake,
		DetectCountry:      profile.DetectCountry,
		LandingIPURL:       landingIPURL,
		SpeedRecordMode:    speedRecordMode,
		PeakSampleInterval: peakSampleInterval,
	}
}

// nodeToMihomoProxy 将节点转换为 mihomo 代理配置
func nodeToMihomoProxy(node *models.Node) (map[string]interface{}, error) {
	extra := map[string]interface{}(node.Extra)

	proxy := map[string]interface{}{
		"name":   node.Tag,
		"server": node.Server,
		"port":   node.ServerPort,
		"udp":    true,
	}

	switch node.Type {
	case "shadowsocks", "ss":
		proxy["type"] = "ss"
		if method, ok := extra["method"].(string); ok {
			proxy["cipher"] = method
		}
		if password, ok := extra["password"].(string); ok {
			proxy["password"] = password
		}
		if err := applyShadowsocksPluginToMihomo(proxy, extra); err != nil {
			return nil, err
		}

	case "vmess":
		proxy["type"] = "vmess"
		if uuid, ok := extra["uuid"].(string); ok {
			proxy["uuid"] = uuid
		}
		if alterId, ok := extra["alter_id"]; ok {
			switch v := alterId.(type) {
			case float64:
				proxy["alterId"] = int(v)
			case int:
				proxy["alterId"] = v
			}
		} else {
			proxy["alterId"] = 0
		}
		if security, ok := extra["security"].(string); ok && security != "" {
			proxy["cipher"] = security
		} else {
			proxy["cipher"] = "auto"
		}
		// TLS
		if tls, ok := extra["tls"].(map[string]interface{}); ok {
			if enabled, ok := tls["enabled"].(bool); ok && enabled {
				proxy["tls"] = true
				if sni, ok := tls["server_name"].(string); ok {
					proxy["servername"] = sni
				}
				if insecure, ok := tls["insecure"].(bool); ok {
					proxy["skip-cert-verify"] = insecure
				}
			}
		}
		// Transport
		if transport, ok := extra["transport"].(map[string]interface{}); ok {
			if tType, ok := transport["type"].(string); ok {
				proxy["network"] = tType
				switch tType {
				case "ws":
					wsOpts := map[string]interface{}{}
					if path, ok := transport["path"].(string); ok {
						wsOpts["path"] = path
					}
					if headers, ok := transport["headers"].(map[string]interface{}); ok {
						wsOpts["headers"] = headers
					}
					if len(wsOpts) > 0 {
						proxy["ws-opts"] = wsOpts
					}
				case "grpc":
					grpcOpts := map[string]interface{}{}
					if serviceName, ok := transport["service_name"].(string); ok {
						grpcOpts["grpc-service-name"] = serviceName
					}
					if len(grpcOpts) > 0 {
						proxy["grpc-opts"] = grpcOpts
					}
				}
			}
		}

	case "vless":
		proxy["type"] = "vless"
		if uuid, ok := extra["uuid"].(string); ok {
			proxy["uuid"] = uuid
		}
		if flow, ok := extra["flow"].(string); ok && flow != "" {
			proxy["flow"] = flow
		}
		// TLS/Reality
		if tls, ok := extra["tls"].(map[string]interface{}); ok {
			if enabled, ok := tls["enabled"].(bool); ok && enabled {
				proxy["tls"] = true
				if sni, ok := tls["server_name"].(string); ok {
					proxy["servername"] = sni
				}
				if insecure, ok := tls["insecure"].(bool); ok {
					proxy["skip-cert-verify"] = insecure
				}
				// Reality
				if reality, ok := tls["reality"].(map[string]interface{}); ok {
					if enabled, ok := reality["enabled"].(bool); ok && enabled {
						realityOpts := map[string]interface{}{}
						if pubKey, ok := reality["public_key"].(string); ok {
							realityOpts["public-key"] = pubKey
						}
						if shortID, ok := reality["short_id"].(string); ok {
							realityOpts["short-id"] = shortID
						}
						proxy["reality-opts"] = realityOpts
						// REALITY 必须设置 client-fingerprint，先设默认值
						proxy["client-fingerprint"] = "chrome"
						// REALITY 必须有 servername，如果 TLS 没设置则使用服务器地址
						if _, hasSNI := proxy["servername"]; !hasSNI {
							proxy["servername"] = node.Server
						}
					}
				}
				// uTLS fingerprint（覆盖默认值）
				if utls, ok := tls["utls"].(map[string]interface{}); ok {
					if fp, ok := utls["fingerprint"].(string); ok {
						proxy["client-fingerprint"] = fp
					}
				}
			}
		}
		// Transport
		if transport, ok := extra["transport"].(map[string]interface{}); ok {
			if tType, ok := transport["type"].(string); ok {
				proxy["network"] = tType
				switch tType {
				case "ws":
					wsOpts := map[string]interface{}{}
					if path, ok := transport["path"].(string); ok {
						wsOpts["path"] = path
					}
					if headers, ok := transport["headers"].(map[string]interface{}); ok {
						wsOpts["headers"] = headers
					}
					if len(wsOpts) > 0 {
						proxy["ws-opts"] = wsOpts
					}
				case "grpc":
					grpcOpts := map[string]interface{}{}
					if serviceName, ok := transport["service_name"].(string); ok {
						grpcOpts["grpc-service-name"] = serviceName
					}
					if len(grpcOpts) > 0 {
						proxy["grpc-opts"] = grpcOpts
					}
				}
			}
		}

	case "trojan":
		proxy["type"] = "trojan"
		if password, ok := extra["password"].(string); ok {
			proxy["password"] = password
		}
		// TLS
		proxy["tls"] = true
		if tls, ok := extra["tls"].(map[string]interface{}); ok {
			if sni, ok := tls["server_name"].(string); ok {
				proxy["sni"] = sni
			}
			if insecure, ok := tls["insecure"].(bool); ok {
				proxy["skip-cert-verify"] = insecure
			}
		}
		// Transport
		if transport, ok := extra["transport"].(map[string]interface{}); ok {
			if tType, ok := transport["type"].(string); ok {
				proxy["network"] = tType
			}
		}

	case "hysteria2", "hy2":
		proxy["type"] = "hysteria2"
		if password, ok := extra["password"].(string); ok {
			proxy["password"] = password
		}
		if auth, ok := extra["auth"].(string); ok && auth != "" {
			proxy["password"] = auth
		}
		// TLS
		if tls, ok := extra["tls"].(map[string]interface{}); ok {
			if sni, ok := tls["server_name"].(string); ok {
				proxy["sni"] = sni
			}
			if insecure, ok := tls["insecure"].(bool); ok {
				proxy["skip-cert-verify"] = insecure
			}
		}
		// Obfs
		if obfs, ok := extra["obfs"].(map[string]interface{}); ok {
			if obfsType, ok := obfs["type"].(string); ok && obfsType != "" {
				proxy["obfs"] = obfsType
			}
			if obfsPassword, ok := obfs["password"].(string); ok {
				proxy["obfs-password"] = obfsPassword
			}
		}

	case "tuic":
		proxy["type"] = "tuic"
		if uuid, ok := extra["uuid"].(string); ok {
			proxy["uuid"] = uuid
		}
		if password, ok := extra["password"].(string); ok {
			proxy["password"] = password
		}
		// TLS
		if tls, ok := extra["tls"].(map[string]interface{}); ok {
			if sni, ok := tls["server_name"].(string); ok {
				proxy["sni"] = sni
			}
			if insecure, ok := tls["insecure"].(bool); ok {
				proxy["skip-cert-verify"] = insecure
			}
			if alpn, ok := tls["alpn"].([]interface{}); ok {
				alpnStrs := make([]string, len(alpn))
				for i, a := range alpn {
					if s, ok := a.(string); ok {
						alpnStrs[i] = s
					}
				}
				proxy["alpn"] = alpnStrs
			}
		}
		if congestion, ok := extra["congestion_control"].(string); ok {
			proxy["congestion-controller"] = congestion
		}

	case "shadowsocksr", "ssr":
		proxy["type"] = "ssr"
		if method, ok := extra["method"].(string); ok {
			proxy["cipher"] = method
		}
		if password, ok := extra["password"].(string); ok {
			proxy["password"] = password
		}
		if protocol, ok := extra["protocol"].(string); ok {
			proxy["protocol"] = protocol
		}
		if protocolParam, ok := extra["protocol_param"].(string); ok {
			proxy["protocol-param"] = protocolParam
		}
		if obfs, ok := extra["obfs"].(string); ok {
			proxy["obfs"] = obfs
		}
		if obfsParam, ok := extra["obfs_param"].(string); ok {
			proxy["obfs-param"] = obfsParam
		}

	default:
		return nil, fmt.Errorf("不支持的协议类型: %s", node.Type)
	}

	return proxy, nil
}

func applyShadowsocksPluginToMihomo(proxy map[string]interface{}, extra map[string]interface{}) error {
	plugin, _ := extra["plugin"].(string)
	if plugin == "" {
		return nil
	}

	proxy["plugin"] = normalizeMihomoShadowsocksPluginName(plugin)

	opts, err := normalizeMihomoShadowsocksPluginOpts(plugin, extra["plugin_opts"])
	if err != nil {
		return fmt.Errorf("解析 shadowsocks plugin_opts 失败: %w", err)
	}
	if len(opts) > 0 {
		proxy["plugin-opts"] = opts
	}

	return nil
}

func normalizeMihomoShadowsocksPluginName(plugin string) string {
	switch strings.ToLower(plugin) {
	case "obfs-local":
		return "obfs"
	default:
		return plugin
	}
}

func normalizeMihomoShadowsocksPluginOpts(plugin string, raw interface{}) (map[string]interface{}, error) {
	switch opts := raw.(type) {
	case nil:
		return nil, nil
	case map[string]interface{}:
		return opts, nil
	case string:
		return parseShadowsocksPluginOptsString(plugin, opts), nil
	default:
		return nil, fmt.Errorf("不支持的 plugin_opts 类型: %T", raw)
	}
}

func parseShadowsocksPluginOptsString(plugin, raw string) map[string]interface{} {
	if raw == "" {
		return nil
	}

	pluginName := strings.ToLower(plugin)
	parsed := make(map[string]interface{})
	for _, segment := range strings.Split(raw, ";") {
		part := strings.TrimSpace(segment)
		if part == "" {
			continue
		}

		key, value, hasValue := strings.Cut(part, "=")
		key = strings.TrimSpace(key)
		if !hasValue {
			parsed[key] = true
			continue
		}
		value = strings.TrimSpace(value)

		switch pluginName {
		case "obfs", "obfs-local":
			switch key {
			case "obfs":
				parsed["mode"] = value
			case "obfs-host":
				parsed["host"] = value
			case "obfs-uri":
				parsed["path"] = value
			default:
				parsed[key] = value
			}
		default:
			parsed[key] = value
		}
	}

	if len(parsed) == 0 {
		return nil
	}

	return parsed
}

// GetMihomoAdapter 从节点创建 mihomo adapter
func GetMihomoAdapter(node *models.Node) (constant.Proxy, error) {
	proxyMap, err := nodeToMihomoProxy(node)
	if err != nil {
		return nil, err
	}

	// 使用 YAML 进行转换以确保类型正确
	yamlBytes, err := yaml.Marshal(proxyMap)
	if err != nil {
		return nil, fmt.Errorf("序列化代理配置失败: %w", err)
	}

	var finalMap map[string]interface{}
	if err := yaml.Unmarshal(yamlBytes, &finalMap); err != nil {
		return nil, fmt.Errorf("反序列化代理配置失败: %w", err)
	}

	proxyAdapter, err := adapter.ParseProxy(finalMap)
	if err != nil {
		return nil, fmt.Errorf("创建 mihomo adapter 失败: %w", err)
	}

	return proxyAdapter, nil
}

// TestDelay 测试节点延迟
func (t *Tester) TestDelay(node *models.Node) TestResult {
	result := TestResult{
		NodeID: node.ID,
		Delay:  -1,
		Speed:  0,
		Status: "error",
	}

	proxyAdapter, err := GetMihomoAdapter(node)
	if err != nil {
		result.Error = err.Error()
		logger.Debug("节点 [%s] 创建 adapter 失败: %v", node.Tag, err)
		return result
	}

	// 设置 UnifiedDelay 模式
	adapter.UnifiedDelay.Store(!t.IncludeHandshake)

	ctx, cancel := context.WithTimeout(context.Background(), t.Timeout)
	defer cancel()

	delay, err := proxyAdapter.URLTest(ctx, t.LatencyURL, nil)
	if err != nil {
		result.Status = "timeout"
		result.Delay = -1
		result.Error = err.Error()
		logger.Debug("节点 [%s] 延迟测试失败: %v", node.Tag, err)
		return result
	}

	result.Delay = int(delay)
	result.Status = "success"

	// 如果需要检测落地 IP
	if t.DetectCountry {
		result.LandingIP = t.fetchLandingIP(proxyAdapter)
	}

	logger.Debug("节点 [%s] 延迟测试成功: %d ms", node.Tag, result.Delay)
	return result
}

// TestSpeed 测试节点速度 (包含延迟测试)
func (t *Tester) TestSpeed(node *models.Node) TestResult {
	result := TestResult{
		NodeID: node.ID,
		Delay:  -1,
		Speed:  0,
		Status: "error",
	}

	proxyAdapter, err := GetMihomoAdapter(node)
	if err != nil {
		result.Error = err.Error()
		logger.Debug("节点 [%s] 创建 adapter 失败: %v", node.Tag, err)
		return result
	}

	// 解析测速 URL
	parsedURL, err := url.Parse(t.SpeedURL)
	if err != nil {
		result.Error = fmt.Sprintf("无效的测速 URL: %v", err)
		return result
	}

	portStr := parsedURL.Port()
	if portStr == "" {
		if parsedURL.Scheme == "https" {
			portStr = "443"
		} else {
			portStr = "80"
		}
	}

	portInt, _ := strconv.Atoi(portStr)
	if portInt < 0 || portInt > 65535 {
		result.Error = fmt.Sprintf("无效的端口: %d", portInt)
		return result
	}

	ctx, cancel := context.WithTimeout(context.Background(), t.Timeout)
	defer cancel()

	metadata := &constant.Metadata{
		Host:    parsedURL.Hostname(),
		DstPort: uint16(portInt),
		Type:    constant.HTTP,
	}

	start := time.Now()
	conn, err := proxyAdapter.DialContext(ctx, metadata)
	if err != nil {
		result.Status = "timeout"
		result.Error = err.Error()
		return result
	}
	defer func() {
		go func() { _ = conn.Close() }()
	}()

	result.Delay = int(time.Since(start).Milliseconds())

	// 创建 HTTP 客户端
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(dialCtx context.Context, network, addr string) (net.Conn, error) {
				h, pStr, _ := net.SplitHostPort(addr)
				pInt, _ := strconv.Atoi(pStr)
				if pInt < 0 || pInt > 65535 {
					return nil, fmt.Errorf("port out of range: %d", pInt)
				}
				md := &constant.Metadata{
					Host:    h,
					DstPort: uint16(pInt),
					Type:    constant.HTTP,
				}
				return proxyAdapter.DialContext(dialCtx, md)
			},
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: t.Timeout,
	}

	resp, err := client.Get(t.SpeedURL)
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()

	// 读取数据测速
	buf := make([]byte, 32*1024)
	var totalRead int64 // 使用 atomic 操作
	readStart := time.Now()

	// 峰值速度采样 (使用 atomic 避免数据竞争)
	var peakSpeedBits uint64 // 存储 float64 的位模式
	var lastSampleBytes int64
	var sampleTicker *time.Ticker
	var sampleDone chan struct{}

	if t.SpeedRecordMode == "peak" {
		sampleTicker = time.NewTicker(time.Duration(t.PeakSampleInterval) * time.Millisecond)
		sampleDone = make(chan struct{})
		lastSampleTime := readStart

		go func() {
			defer sampleTicker.Stop()
			for {
				select {
				case <-sampleTicker.C:
					now := time.Now()
					currentBytes := atomic.LoadInt64(&totalRead)
					elapsed := now.Sub(lastSampleTime).Seconds()
					if elapsed > 0 {
						instantSpeed := float64(currentBytes-lastSampleBytes) / 1024 / 1024 / elapsed
						// 原子更新峰值速度
						for {
							oldBits := atomic.LoadUint64(&peakSpeedBits)
							oldSpeed := float64FromBits(oldBits)
							if instantSpeed <= oldSpeed {
								break
							}
							newBits := float64ToBits(instantSpeed)
							if atomic.CompareAndSwapUint64(&peakSpeedBits, oldBits, newBits) {
								break
							}
						}
					}
					lastSampleBytes = currentBytes
					lastSampleTime = now
				case <-sampleDone:
					return
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	for {
		n, readErr := resp.Body.Read(buf)
		atomic.AddInt64(&totalRead, int64(n))
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			if ctx.Err() == context.DeadlineExceeded {
				break
			}
			if netErr, ok := readErr.(net.Error); ok && netErr.Timeout() {
				break
			}
			if sampleDone != nil {
				close(sampleDone)
			}
			result.Error = readErr.Error()
			return result
		}
		select {
		case <-ctx.Done():
			goto CalculateSpeed
		default:
		}
	}

CalculateSpeed:
	if sampleDone != nil {
		close(sampleDone)
	}

	finalTotalRead := atomic.LoadInt64(&totalRead)
	duration := time.Since(readStart)
	if duration.Seconds() == 0 {
		result.Status = "success"
		return result
	}

	// 最小有效下载量校验 (10KB)
	const minValidBytes int64 = 10 * 1024
	if finalTotalRead < minValidBytes {
		result.Status = "error"
		result.Error = fmt.Sprintf("下载量过小 (%d 字节 < %d 字节)", finalTotalRead, minValidBytes)
		return result
	}

	peakSpeed := float64FromBits(atomic.LoadUint64(&peakSpeedBits))
	if t.SpeedRecordMode == "peak" && peakSpeed > 0 {
		result.Speed = peakSpeed
	} else {
		result.Speed = float64(finalTotalRead) / 1024 / 1024 / duration.Seconds()
	}

	result.Status = "success"

	// 检测落地 IP
	if t.DetectCountry && result.Speed > 0 {
		result.LandingIP = t.fetchLandingIP(proxyAdapter)
	}

	logger.Debug("节点 [%s] 测速成功: 速度 %.2f MB/s, 延迟 %d ms", node.Tag, result.Speed, result.Delay)
	return result
}

// fetchLandingIP 获取落地 IP
func (t *Tester) fetchLandingIP(proxyAdapter constant.Proxy) string {
	defer func() {
		if r := recover(); r != nil {
			logger.Debug("获取落地 IP 时发生 panic: %v", r)
		}
	}()

	ipURL := t.LandingIPURL
	if ipURL == "" {
		ipURL = "https://api.ipify.org"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(dialCtx context.Context, network, addr string) (net.Conn, error) {
				h, pStr, _ := net.SplitHostPort(addr)
				pInt, _ := strconv.Atoi(pStr)
				if pInt < 0 || pInt > 65535 {
					return nil, fmt.Errorf("port out of range: %d", pInt)
				}
				md := &constant.Metadata{
					Host:    h,
					DstPort: uint16(pInt),
					Type:    constant.HTTP,
				}
				return proxyAdapter.DialContext(dialCtx, md)
			},
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 3 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", ipURL, nil)
	if err != nil {
		return ""
	}

	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body := make([]byte, 64)
	n, _ := resp.Body.Read(body)
	return strings.TrimSpace(string(body[:n]))
}
