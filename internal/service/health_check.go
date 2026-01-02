package service

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/xiaobei/singbox-manager/internal/storage"
)

// HealthCheckService 健康检测服务
type HealthCheckService struct {
	store *storage.JSONStore

	healthCache map[string]*storage.ChainHealthStatus
	cacheMu     sync.RWMutex

	stopCh  chan struct{}
	running bool
	mu      sync.Mutex

	alertCallback func(chainID, message string)
}

// NewHealthCheckService 创建健康检测服务
func NewHealthCheckService(store *storage.JSONStore) *HealthCheckService {
	return &HealthCheckService{
		store:       store,
		healthCache: make(map[string]*storage.ChainHealthStatus),
		stopCh:      make(chan struct{}),
	}
}

// SetAlertCallback 设置告警回调
func (h *HealthCheckService) SetAlertCallback(callback func(chainID, message string)) {
	h.alertCallback = callback
}

// CheckChain 检测单个链路（级联测试）
// 严格按照链路节点顺序进行测试，避免直接连接后续节点导致 IP 泄露
func (h *HealthCheckService) CheckChain(chainID string) (*storage.ChainHealthStatus, error) {
	chain := h.store.GetProxyChain(chainID)
	if chain == nil {
		return nil, fmt.Errorf("chain not found: %s", chainID)
	}

	settings := h.store.GetSettings()
	config := chain.HealthConfig
	if config == nil {
		config = settings.ChainHealthConfig
	}
	if config == nil {
		config = &storage.ChainHealthConfig{
			Timeout: 10,
			URL:     "https://www.gstatic.com/generate_204",
		}
	}

	status := &storage.ChainHealthStatus{
		ChainID:      chainID,
		LastCheck:    time.Now(),
		NodeStatuses: make([]storage.NodeHealthStatus, 0, len(chain.Nodes)),
	}

	allNodes := h.store.GetAllNodes()
	nodeMap := make(map[string]storage.Node)
	for _, n := range allNodes {
		nodeMap[n.Tag] = n
	}

	// 收集链路中的节点
	var chainNodes []storage.Node
	for _, nodeTag := range chain.Nodes {
		node, exists := nodeMap[nodeTag]
		if !exists {
			nodeStatus := storage.NodeHealthStatus{
				Tag:    nodeTag,
				Status: "unhealthy",
				Error:  "node not found",
			}
			status.NodeStatuses = append(status.NodeStatuses, nodeStatus)
			status.Status = "unhealthy"
			h.cacheStatus(chainID, status)
			return status, nil
		}
		chainNodes = append(chainNodes, node)
	}

	if len(chainNodes) == 0 {
		status.Status = "unhealthy"
		h.cacheStatus(chainID, status)
		return status, nil
	}

	// 级联测试：按顺序逐个测试节点
	// 第一个节点：直接 TCP 连接测试
	// 后续节点：通过前一个节点作为代理进行测试
	timeout := time.Duration(config.Timeout) * time.Second
	totalLatency := 0
	unhealthyCount := 0
	var lastProxyConn net.Conn

	for i, node := range chainNodes {
		var nodeStatus storage.NodeHealthStatus

		if i == 0 {
			// 第一个节点：直接 TCP 连接
			nodeStatus = h.testNodeDirect(node, timeout)
		} else {
			// 后续节点：通过前面的代理链路连接
			// 使用前一个节点建立的连接作为代理
			nodeStatus = h.testNodeViaChain(chainNodes[:i], node, timeout)
		}

		status.NodeStatuses = append(status.NodeStatuses, nodeStatus)

		if nodeStatus.Status != "healthy" {
			unhealthyCount++
			// 如果某个节点不可用，后续节点都无法测试
			for j := i + 1; j < len(chainNodes); j++ {
				status.NodeStatuses = append(status.NodeStatuses, storage.NodeHealthStatus{
					Tag:    chainNodes[j].Tag,
					Status: "unknown",
					Error:  "previous node unavailable",
				})
			}
			break
		}
		totalLatency += nodeStatus.Latency
	}

	if lastProxyConn != nil {
		lastProxyConn.Close()
	}

	status.Latency = totalLatency

	// 确定整体状态
	if unhealthyCount == 0 {
		status.Status = "healthy"
	} else if unhealthyCount < len(chain.Nodes) {
		status.Status = "degraded"
	} else {
		status.Status = "unhealthy"
	}

	// 缓存结果
	h.cacheStatus(chainID, status)

	// 如果不健康且启用告警，发送告警
	if status.Status == "unhealthy" && config.AlertEnabled && h.alertCallback != nil {
		h.alertCallback(chainID, fmt.Sprintf("链路 %s 不可用", chain.Name))
	}

	return status, nil
}

// cacheStatus 缓存健康状态
func (h *HealthCheckService) cacheStatus(chainID string, status *storage.ChainHealthStatus) {
	h.cacheMu.Lock()
	h.healthCache[chainID] = status
	h.cacheMu.Unlock()
}

// testNodeDirect 直接测试节点连通性（TCP 连接测试）
func (h *HealthCheckService) testNodeDirect(node storage.Node, timeout time.Duration) storage.NodeHealthStatus {
	start := time.Now()

	address := fmt.Sprintf("%s:%d", node.Server, node.ServerPort)
	conn, err := net.DialTimeout("tcp", address, timeout)

	latency := int(time.Since(start).Milliseconds())

	if err != nil {
		return storage.NodeHealthStatus{
			Tag:     node.Tag,
			Status:  "unhealthy",
			Latency: latency,
			Error:   err.Error(),
		}
	}
	conn.Close()

	return storage.NodeHealthStatus{
		Tag:     node.Tag,
		Status:  "healthy",
		Latency: latency,
	}
}

// testNodeViaChain 通过代理链路测试节点
// proxyChain: 前置代理节点列表
// targetNode: 要测试的目标节点
func (h *HealthCheckService) testNodeViaChain(proxyChain []storage.Node, targetNode storage.Node, timeout time.Duration) storage.NodeHealthStatus {
	start := time.Now()

	// 连接到第一个代理节点
	firstNode := proxyChain[0]
	firstAddr := fmt.Sprintf("%s:%d", firstNode.Server, firstNode.ServerPort)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", firstAddr)
	if err != nil {
		return storage.NodeHealthStatus{
			Tag:     targetNode.Tag,
			Status:  "unhealthy",
			Latency: int(time.Since(start).Milliseconds()),
			Error:   fmt.Sprintf("connect to first proxy failed: %s", err.Error()),
		}
	}
	defer conn.Close()

	// 通过代理链路逐级连接
	currentConn := conn
	for i := 0; i < len(proxyChain); i++ {
		var nextAddr string
		if i < len(proxyChain)-1 {
			// 连接到下一个代理节点
			nextNode := proxyChain[i+1]
			nextAddr = fmt.Sprintf("%s:%d", nextNode.Server, nextNode.ServerPort)
		} else {
			// 最后一个代理，连接到目标节点
			nextAddr = fmt.Sprintf("%s:%d", targetNode.Server, targetNode.ServerPort)
		}

		// 根据节点类型执行代理握手
		node := proxyChain[i]
		err = h.proxyHandshake(currentConn, node, nextAddr, timeout)
		if err != nil {
			return storage.NodeHealthStatus{
				Tag:     targetNode.Tag,
				Status:  "unhealthy",
				Latency: int(time.Since(start).Milliseconds()),
				Error:   fmt.Sprintf("proxy handshake failed at %s: %s", node.Tag, err.Error()),
			}
		}
	}

	latency := int(time.Since(start).Milliseconds())

	return storage.NodeHealthStatus{
		Tag:     targetNode.Tag,
		Status:  "healthy",
		Latency: latency,
	}
}

// proxyHandshake 执行代理握手
func (h *HealthCheckService) proxyHandshake(conn net.Conn, node storage.Node, targetAddr string, timeout time.Duration) error {
	conn.SetDeadline(time.Now().Add(timeout))

	switch node.Type {
	case "shadowsocks":
		// Shadowsocks 不需要握手，直接可以发送数据
		// 但我们需要验证连接是否有效，发送一个简单的连接请求
		return h.shadowsocksConnect(conn, node, targetAddr)
	case "trojan":
		return h.trojanConnect(conn, node, targetAddr)
	case "vmess", "vless":
		// VMess/VLESS 协议较复杂，这里简化为验证 TLS 握手（如果启用）
		return h.vmessConnect(conn, node, targetAddr)
	case "hysteria2", "tuic":
		// QUIC 协议，需要特殊处理
		// 这里简化为 TCP 连接测试
		return nil
	case "socks", "socks5":
		return h.socks5Connect(conn, targetAddr)
	case "http":
		return h.httpProxyConnect(conn, node, targetAddr)
	default:
		// 未知协议，假设可以直接使用
		return nil
	}
}

// socks5Connect SOCKS5 代理连接
func (h *HealthCheckService) socks5Connect(conn net.Conn, targetAddr string) error {
	host, portStr, err := net.SplitHostPort(targetAddr)
	if err != nil {
		return err
	}
	port, err := net.LookupPort("tcp", portStr)
	if err != nil {
		return err
	}

	// SOCKS5 握手
	// 1. 发送认证方法
	_, err = conn.Write([]byte{0x05, 0x01, 0x00}) // SOCKS5, 1 method, no auth
	if err != nil {
		return err
	}

	// 2. 读取服务器响应
	buf := make([]byte, 2)
	_, err = io.ReadFull(conn, buf)
	if err != nil {
		return err
	}
	if buf[0] != 0x05 || buf[1] != 0x00 {
		return fmt.Errorf("SOCKS5 auth failed")
	}

	// 3. 发送连接请求
	req := []byte{0x05, 0x01, 0x00, 0x03} // SOCKS5, CONNECT, RSV, DOMAIN
	req = append(req, byte(len(host)))
	req = append(req, []byte(host)...)
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(port))
	req = append(req, portBytes...)

	_, err = conn.Write(req)
	if err != nil {
		return err
	}

	// 4. 读取连接响应
	resp := make([]byte, 10)
	_, err = io.ReadFull(conn, resp[:4])
	if err != nil {
		return err
	}
	if resp[1] != 0x00 {
		return fmt.Errorf("SOCKS5 connect failed: %d", resp[1])
	}

	// 跳过地址部分
	switch resp[3] {
	case 0x01: // IPv4
		_, err = io.ReadFull(conn, resp[4:10])
	case 0x03: // Domain
		_, err = io.ReadFull(conn, resp[4:5])
		if err == nil {
			domainLen := int(resp[4])
			_, err = io.ReadFull(conn, make([]byte, domainLen+2))
		}
	case 0x04: // IPv6
		_, err = io.ReadFull(conn, make([]byte, 18))
	}

	return err
}

// httpProxyConnect HTTP 代理连接
func (h *HealthCheckService) httpProxyConnect(conn net.Conn, node storage.Node, targetAddr string) error {
	req := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n", targetAddr, targetAddr)

	// 添加认证（如果有）
	if username, ok := node.Extra["username"].(string); ok && username != "" {
		password, _ := node.Extra["password"].(string)
		auth := fmt.Sprintf("%s:%s", username, password)
		// Base64 编码
		encoded := base64Encode([]byte(auth))
		req += fmt.Sprintf("Proxy-Authorization: Basic %s\r\n", encoded)
	}
	req += "\r\n"

	_, err := conn.Write([]byte(req))
	if err != nil {
		return err
	}

	// 读取响应
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	// 检查 HTTP 状态码
	if len(line) < 12 || line[9:12] != "200" {
		return fmt.Errorf("HTTP CONNECT failed: %s", line)
	}

	// 读取剩余的响应头
	for {
		line, err = reader.ReadString('\n')
		if err != nil {
			return err
		}
		if line == "\r\n" {
			break
		}
	}

	return nil
}

// shadowsocksConnect Shadowsocks 连接测试
// Shadowsocks 是加密协议，无法直接验证连接
// 这里只验证 TCP 连接是否建立
func (h *HealthCheckService) shadowsocksConnect(conn net.Conn, node storage.Node, targetAddr string) error {
	// Shadowsocks 协议需要加密，这里简化处理
	// 实际上我们已经连接到 SS 服务器，认为连接成功
	return nil
}

// trojanConnect Trojan 连接测试
func (h *HealthCheckService) trojanConnect(conn net.Conn, node storage.Node, targetAddr string) error {
	// Trojan 通常需要 TLS
	tlsConn := tls.Client(conn, &tls.Config{
		InsecureSkipVerify: true, // 测试用，跳过证书验证
	})
	err := tlsConn.Handshake()
	if err != nil {
		return fmt.Errorf("TLS handshake failed: %s", err.Error())
	}
	// TLS 握手成功即认为节点可用
	return nil
}

// vmessConnect VMess/VLESS 连接测试
func (h *HealthCheckService) vmessConnect(conn net.Conn, node storage.Node, targetAddr string) error {
	// 检查是否需要 TLS
	if tls_, ok := node.Extra["tls"].(map[string]interface{}); ok {
		if enabled, ok := tls_["enabled"].(bool); ok && enabled {
			tlsConn := tls.Client(conn, &tls.Config{
				InsecureSkipVerify: true,
			})
			err := tlsConn.Handshake()
			if err != nil {
				return fmt.Errorf("TLS handshake failed: %s", err.Error())
			}
		}
	}
	// VMess/VLESS 协议复杂，这里简化为 TLS 握手或 TCP 连接成功
	return nil
}

// base64Encode 简单的 Base64 编码
func base64Encode(data []byte) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var result []byte

	for i := 0; i < len(data); i += 3 {
		var n uint32
		remaining := len(data) - i
		if remaining >= 3 {
			n = uint32(data[i])<<16 | uint32(data[i+1])<<8 | uint32(data[i+2])
			result = append(result, alphabet[n>>18&0x3F], alphabet[n>>12&0x3F], alphabet[n>>6&0x3F], alphabet[n&0x3F])
		} else if remaining == 2 {
			n = uint32(data[i])<<16 | uint32(data[i+1])<<8
			result = append(result, alphabet[n>>18&0x3F], alphabet[n>>12&0x3F], alphabet[n>>6&0x3F], '=')
		} else {
			n = uint32(data[i]) << 16
			result = append(result, alphabet[n>>18&0x3F], alphabet[n>>12&0x3F], '=', '=')
		}
	}
	return string(result)
}

// Start 启动定时健康检测
func (h *HealthCheckService) Start() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.running {
		return
	}

	settings := h.store.GetSettings()
	if settings.ChainHealthConfig == nil || !settings.ChainHealthConfig.Enabled {
		return
	}

	h.running = true
	h.stopCh = make(chan struct{})

	go h.runScheduledChecks()
}

// Stop 停止定时检测
func (h *HealthCheckService) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.running {
		return
	}

	close(h.stopCh)
	h.running = false
}

// Restart 重启定时检测
func (h *HealthCheckService) Restart() {
	h.Stop()
	h.Start()
}

func (h *HealthCheckService) runScheduledChecks() {
	settings := h.store.GetSettings()
	interval := time.Duration(settings.ChainHealthConfig.Interval) * time.Second
	if interval < 30*time.Second {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-h.stopCh:
			return
		case <-ticker.C:
			h.checkAllChains()
		}
	}
}

func (h *HealthCheckService) checkAllChains() {
	chains := h.store.GetProxyChains()
	for _, chain := range chains {
		if chain.Enabled {
			h.CheckChain(chain.ID)
		}
	}
}

// GetCachedStatus 获取缓存的健康状态
func (h *HealthCheckService) GetCachedStatus(chainID string) *storage.ChainHealthStatus {
	h.cacheMu.RLock()
	defer h.cacheMu.RUnlock()
	return h.healthCache[chainID]
}

// GetAllCachedStatuses 获取所有缓存状态
func (h *HealthCheckService) GetAllCachedStatuses() map[string]*storage.ChainHealthStatus {
	h.cacheMu.RLock()
	defer h.cacheMu.RUnlock()

	result := make(map[string]*storage.ChainHealthStatus)
	for k, v := range h.healthCache {
		result[k] = v
	}
	return result
}

// IsRunning 检查服务是否运行中
func (h *HealthCheckService) IsRunning() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.running
}

// Unused imports placeholder to avoid compile error
var _ = http.StatusOK
