package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/xiaobei/singbox-manager/internal/storage"
	"golang.org/x/net/proxy"
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

// ClashDelayResponse Clash API 延迟测试响应
type ClashDelayResponse struct {
	Delay   int    `json:"delay"`
	Message string `json:"message,omitempty"`
}

// CheckChain 检测单个链路
// 使用 Clash API 测试每个节点的延迟
// 参考 v2rayN/Clash 的实现：通过代理发送 HTTP 请求到测试 URL
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

	// 验证链路节点
	if len(chain.Nodes) < 1 {
		status.Status = "unhealthy"
		status.NodeStatuses = append(status.NodeStatuses, storage.NodeHealthStatus{
			Tag:    "chain",
			Status: "unhealthy",
			Error:  "链路至少需要1个节点",
		})
		h.cacheStatus(chainID, status)
		return status, nil
	}

	// 检查 Clash API 是否可用
	clashAPIPort := settings.ClashAPIPort
	if clashAPIPort <= 0 {
		// Clash API 未启用，回退到简单测试
		return h.checkChainSimple(chain, config, status)
	}

	timeout := time.Duration(config.Timeout) * time.Second
	testURL := config.URL
	if testURL == "" {
		testURL = "https://www.gstatic.com/generate_204"
	}

	// 分别测试每个节点的延迟
	// 每个节点的副本 Tag 格式为：{链路名}-{节点Tag}
	// 入口节点直接连接，后续节点通过前置节点 detour
	unhealthyCount := 0
	var totalLatency int

	for i, nodeTag := range chain.Nodes {
		copyTag := storage.GenerateChainNodeCopyTag(chain.Name, nodeTag)

		// 通过 Clash API 测试该节点的延迟
		latency, err := h.testViaClashAPI(clashAPIPort, copyTag, testURL, timeout)

		nodeStatus := storage.NodeHealthStatus{
			Tag: nodeTag,
		}

		if err != nil {
			nodeStatus.Status = "unhealthy"
			nodeStatus.Error = err.Error()
			unhealthyCount++
		} else {
			nodeStatus.Status = "healthy"
			nodeStatus.Latency = latency
			// 最后一个节点的延迟就是端到端延迟
			if i == len(chain.Nodes)-1 {
				totalLatency = latency
			}
		}

		status.NodeStatuses = append(status.NodeStatuses, nodeStatus)
	}

	// 设置整体状态
	status.Latency = totalLatency

	if unhealthyCount == 0 {
		status.Status = "healthy"
	} else if unhealthyCount < len(chain.Nodes) {
		status.Status = "degraded"
	} else {
		status.Status = "unhealthy"
	}

	h.cacheStatus(chainID, status)

	if status.Status == "unhealthy" && config.AlertEnabled && h.alertCallback != nil {
		h.alertCallback(chainID, fmt.Sprintf("链路 %s 不可用", chain.Name))
	}

	return status, nil
}

// CheckChainSpeed 测试链路下载速度
// 通过代理下载文件测量带宽
func (h *HealthCheckService) CheckChainSpeed(chainID string) (*storage.ChainSpeedResult, error) {
	chain := h.store.GetProxyChain(chainID)
	if chain == nil {
		return nil, fmt.Errorf("chain not found: %s", chainID)
	}

	settings := h.store.GetSettings()
	mixedPort := settings.MixedPort
	if mixedPort <= 0 {
		return nil, fmt.Errorf("代理端口未配置")
	}

	// 测速配置
	// 使用 Cloudflare 的测速服务，下载 10MB 数据
	speedTestURL := "https://speed.cloudflare.com/__down?bytes=10000000"
	timeout := 30 * time.Second

	// 创建通过代理的 HTTP 客户端
	proxyDialer, err := proxy.SOCKS5("tcp", fmt.Sprintf("127.0.0.1:%d", mixedPort), nil, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("创建代理失败: %w", err)
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return proxyDialer.Dial(network, addr)
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	// 开始测速
	start := time.Now()

	resp, err := client.Get(speedTestURL)
	if err != nil {
		return nil, fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取数据并计算速度
	var totalBytes int64
	buf := make([]byte, 32*1024) // 32KB buffer

	for {
		n, err := resp.Body.Read(buf)
		totalBytes += int64(n)
		if err != nil {
			if err == io.EOF {
				break
			}
			// 超时或其他错误，使用已下载的数据计算速度
			break
		}
	}

	duration := time.Since(start)

	// 计算速度 (Mbps)
	speedMbps := float64(totalBytes*8) / duration.Seconds() / 1000000

	result := &storage.ChainSpeedResult{
		ChainID:    chainID,
		TestTime:   time.Now(),
		SpeedMbps:  speedMbps,
		BytesTotal: totalBytes,
		Duration:   duration.Milliseconds(),
	}

	return result, nil
}

// testViaClashAPI 通过 Clash API 测试代理延迟
// 这是 v2rayN/Clash 使用的标准测速方式
func (h *HealthCheckService) testViaClashAPI(port int, proxyName, testURL string, timeout time.Duration) (int, error) {
	// 构建 Clash API URL
	// GET /proxies/{name}/delay?url=xxx&timeout=xxx
	apiURL := fmt.Sprintf("http://127.0.0.1:%d/proxies/%s/delay", port, url.PathEscape(proxyName))

	params := url.Values{}
	params.Set("url", testURL)
	params.Set("timeout", fmt.Sprintf("%d", int(timeout.Milliseconds())))

	fullURL := apiURL + "?" + params.Encode()

	client := &http.Client{
		Timeout: timeout + 2*time.Second, // 额外留 2 秒给 API 响应
	}

	resp, err := client.Get(fullURL)
	if err != nil {
		return 0, fmt.Errorf("Clash API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// 尝试解析错误消息
		var errResp ClashDelayResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Message != "" {
			return 0, fmt.Errorf("测试失败: %s", errResp.Message)
		}
		return 0, fmt.Errorf("测试失败: HTTP %d", resp.StatusCode)
	}

	var result ClashDelayResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Delay <= 0 {
		return 0, fmt.Errorf("测试超时或失败")
	}

	return result.Delay, nil
}

// checkChainSimple 简单测试（当 Clash API 不可用时）
// 只测试各节点的 TCP 连通性
func (h *HealthCheckService) checkChainSimple(chain *storage.ProxyChain, config *storage.ChainHealthConfig, status *storage.ChainHealthStatus) (*storage.ChainHealthStatus, error) {
	allNodes := h.store.GetAllNodes()
	nodeMap := make(map[string]storage.Node)
	for _, n := range allNodes {
		nodeMap[n.Tag] = n
	}

	timeout := time.Duration(config.Timeout) * time.Second
	unhealthyCount := 0

	for _, nodeTag := range chain.Nodes {
		node, exists := nodeMap[nodeTag]
		if !exists {
			status.NodeStatuses = append(status.NodeStatuses, storage.NodeHealthStatus{
				Tag:    nodeTag,
				Status: "unhealthy",
				Error:  "节点不存在",
			})
			unhealthyCount++
			continue
		}

		// 简单的 TCP 连接测试
		nodeStatus := h.testNodeTCP(node, timeout)
		status.NodeStatuses = append(status.NodeStatuses, nodeStatus)

		if nodeStatus.Status != "healthy" {
			unhealthyCount++
		}
	}

	if unhealthyCount == 0 {
		status.Status = "healthy"
	} else if unhealthyCount < len(chain.Nodes) {
		status.Status = "degraded"
	} else {
		status.Status = "unhealthy"
	}

	// 简单测试无法获取端到端延迟
	status.Latency = 0

	h.cacheStatus(chain.ID, status)

	if status.Status == "unhealthy" && config.AlertEnabled && h.alertCallback != nil {
		h.alertCallback(chain.ID, fmt.Sprintf("链路 %s 不可用（简单测试）", chain.Name))
	}

	return status, nil
}

// testNodeTCP 简单的 TCP 连接测试
func (h *HealthCheckService) testNodeTCP(node storage.Node, timeout time.Duration) storage.NodeHealthStatus {
	start := time.Now()

	address := fmt.Sprintf("%s:%d", node.Server, node.ServerPort)

	// 尝试建立 TCP 连接
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

// cacheStatus 缓存健康状态
func (h *HealthCheckService) cacheStatus(chainID string, status *storage.ChainHealthStatus) {
	h.cacheMu.Lock()
	h.healthCache[chainID] = status
	h.cacheMu.Unlock()
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
