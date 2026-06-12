package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xiaobei/singbox-manager/internal/builder"
	"github.com/xiaobei/singbox-manager/internal/storage"
	"golang.org/x/net/proxy"
)

const defaultTorDiagnosticURL = "https://check.torproject.org/api/ip"

// TorDiagnosticCheck 表示 Tor 诊断中的单段检查结果。
type TorDiagnosticCheck struct {
	Status  string `json:"status"`
	Method  string `json:"method,omitempty"`
	Latency int    `json:"latency,omitempty"`
	Error   string `json:"error,omitempty"`
}

// TorDiagnosticResult 分离展示 Tor 段与完整链路诊断结果。
type TorDiagnosticResult struct {
	ChainID    string             `json:"chain_id"`
	CheckedAt  time.Time          `json:"checked_at"`
	TorSegment TorDiagnosticCheck `json:"tor_segment"`
	FullChain  TorDiagnosticCheck `json:"full_chain"`
}

type torSegmentChecker func(context.Context, storage.ProxyChain) (TorDiagnosticCheck, error)
type torFullChainChecker func(context.Context, string, string) (TorDiagnosticCheck, error)
type torSegmentProcessLauncher func(context.Context, string, string, string) (torDiagnosticProcess, error)
type torProxyWaiter func(context.Context, string) error

type torDiagnosticProcess interface {
	Stop() error
}

// TorDiagnosticsService 提供 Tor 链路手动诊断。
type TorDiagnosticsService struct {
	store                  *storage.JSONStore
	segmentMu              sync.Mutex
	segmentTimeout         time.Duration
	fullChainTimeout       time.Duration
	segmentChecker         torSegmentChecker
	fullChainChecker       torFullChainChecker
	segmentProcessLauncher torSegmentProcessLauncher
	proxyWaiter            torProxyWaiter
}

// NewTorDiagnosticsService 创建 Tor 诊断服务。
func NewTorDiagnosticsService(store *storage.JSONStore) *TorDiagnosticsService {
	svc := &TorDiagnosticsService{
		store:            store,
		segmentTimeout:   30 * time.Second,
		fullChainTimeout: 15 * time.Second,
	}
	svc.segmentChecker = svc.defaultSegmentChecker
	svc.fullChainChecker = svc.defaultFullChainChecker
	svc.segmentProcessLauncher = svc.defaultSegmentProcessLauncher
	svc.proxyWaiter = waitForProxyPort
	return svc
}

// CheckTorChain 诊断 Tor 段与完整链路。
func (s *TorDiagnosticsService) CheckTorChain(ctx context.Context, chainID string) (*TorDiagnosticResult, error) {
	chain := s.store.GetProxyChain(chainID)
	if chain == nil {
		return nil, fmt.Errorf("chain not found: %s", chainID)
	}
	if !storage.ChainContainsTor(chain.Nodes) {
		return nil, fmt.Errorf("链路不是 Tor 链路")
	}

	result := &TorDiagnosticResult{
		ChainID:   chainID,
		CheckedAt: time.Now(),
	}

	result.TorSegment = s.checkTorSegment(ctx, *chain)
	result.FullChain = s.checkFullChain(ctx, *chain)

	return result, nil
}

func (s *TorDiagnosticsService) checkTorSegment(ctx context.Context, chain storage.ProxyChain) TorDiagnosticCheck {
	if !s.segmentMu.TryLock() {
		return TorDiagnosticCheck{
			Status: "unhealthy",
			Method: "temporary_segment",
			Error:  "已有 Tor 段诊断正在运行，请稍后再试",
		}
	}
	defer s.segmentMu.Unlock()

	segmentCtx, cancel := context.WithTimeout(ctx, s.segmentTimeout)
	defer cancel()

	check, err := s.segmentChecker(segmentCtx, chain)
	if err != nil {
		return TorDiagnosticCheck{
			Status: "unhealthy",
			Method: "temporary_segment",
			Error:  fmt.Sprintf("Tor bootstrap/segment failure: %v", err),
		}
	}
	if check.Status == "" {
		check.Status = "healthy"
	}
	if check.Method == "" {
		check.Method = "temporary_segment"
	}
	return check
}

func (s *TorDiagnosticsService) checkFullChain(ctx context.Context, chain storage.ProxyChain) TorDiagnosticCheck {
	proxyAddr, err := s.findBoundTorInbound(chain.ID)
	if err != nil {
		return TorDiagnosticCheck{
			Status: "unhealthy",
			Method: "main_inbound",
			Error:  fmt.Sprintf("post-proxy/full-chain failure: %v", err),
		}
	}

	fullCtx, cancel := context.WithTimeout(ctx, s.fullChainTimeout)
	defer cancel()

	check, err := s.fullChainChecker(fullCtx, proxyAddr, defaultTorDiagnosticURL)
	if err != nil {
		return TorDiagnosticCheck{
			Status: "unhealthy",
			Method: "main_inbound",
			Error:  fmt.Sprintf("post-proxy/full-chain failure: %v", err),
		}
	}
	if check.Status == "" {
		check.Status = "healthy"
	}
	if check.Method == "" {
		check.Method = "main_inbound"
	}
	return check
}

func (s *TorDiagnosticsService) findBoundTorInbound(chainID string) (string, error) {
	for _, port := range s.store.GetInboundPorts() {
		if !port.Enabled || !port.UseTorExit || port.TorChainID != chainID {
			continue
		}
		if port.Type != "mixed" && port.Type != "socks" {
			continue
		}
		listen := port.Listen
		if listen == "" || listen == "0.0.0.0" || listen == "::" {
			listen = "127.0.0.1"
		}
		return net.JoinHostPort(listen, strconv.Itoa(port.Port)), nil
	}
	return "", fmt.Errorf("未找到绑定该 Tor 链路的启用 mixed/socks 入站端口")
}

func (s *TorDiagnosticsService) defaultSegmentChecker(ctx context.Context, chain storage.ProxyChain) (TorDiagnosticCheck, error) {
	configPath, workDir, proxyAddr, cleanup, err := s.writeTemporarySegmentConfig(chain)
	if err != nil {
		return TorDiagnosticCheck{}, err
	}
	defer cleanup()

	settings := s.store.GetSettings()
	executablePath, err := s.resolveSingBoxPath(settings)
	if err != nil {
		return TorDiagnosticCheck{}, err
	}

	process, err := s.segmentProcessLauncher(ctx, executablePath, configPath, workDir)
	if err != nil {
		return TorDiagnosticCheck{}, err
	}
	defer process.Stop()

	if err := s.proxyWaiter(ctx, proxyAddr); err != nil {
		return TorDiagnosticCheck{}, err
	}

	check, err := s.fullChainChecker(ctx, proxyAddr, defaultTorDiagnosticURL)
	if err != nil {
		return TorDiagnosticCheck{}, err
	}
	if check.Method == "" {
		check.Method = "temporary_segment"
	}
	return check, nil
}

func (s *TorDiagnosticsService) defaultFullChainChecker(ctx context.Context, proxyAddr string, targetURL string) (TorDiagnosticCheck, error) {
	proxyDialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		return TorDiagnosticCheck{}, fmt.Errorf("创建代理失败: %w", err)
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return proxyDialer.Dial(network, addr)
		},
	}
	client := &http.Client{
		Transport: transport,
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return TorDiagnosticCheck{}, err
	}

	start := time.Now()
	response, err := client.Do(request)
	if err != nil {
		return TorDiagnosticCheck{}, err
	}
	defer response.Body.Close()

	if err := validateTorHTTPResponse(targetURL, response); err != nil {
		return TorDiagnosticCheck{}, err
	}

	return TorDiagnosticCheck{
		Status:  "healthy",
		Latency: int(time.Since(start).Milliseconds()),
	}, nil
}

func validateTorHTTPResponse(targetURL string, response *http.Response) error {
	if response.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("HTTP 状态码 %d", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("读取 Tor 检查响应失败: %w", err)
	}
	return validateTorProjectResponse(targetURL, body)
}

func validateTorProjectResponse(targetURL string, body []byte) error {
	var response struct {
		IsTor bool `json:"IsTor"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("解析 Tor 检查响应失败: %w", err)
	}
	if !response.IsTor {
		return fmt.Errorf("Tor 检查未确认出口来自 Tor 网络: IsTor=false")
	}
	return nil
}

func (s *TorDiagnosticsService) writeTemporarySegmentConfig(chain storage.ProxyChain) (string, string, string, func(), error) {
	segmentChain, err := truncateChainAtTor(chain)
	if err != nil {
		return "", "", "", nil, err
	}

	port, err := allocateLocalPort()
	if err != nil {
		return "", "", "", nil, err
	}

	workDir, err := os.MkdirTemp("", "sbm-tor-diagnostic-*")
	if err != nil {
		return "", "", "", nil, fmt.Errorf("创建临时诊断目录失败: %w", err)
	}
	cleanup := func() {
		_ = os.RemoveAll(workDir)
	}

	settings := *s.store.GetSettings()
	settings.TunEnabled = false
	settings.ClashAPIPort = 0
	settings.ClashUIEnabled = false

	inbound := storage.InboundPort{
		ID:         "tor-diagnostic",
		Name:       "Tor diagnostic",
		Type:       "mixed",
		Listen:     "127.0.0.1",
		Port:       port,
		Enabled:    true,
		UseTorExit: true,
		TorChainID: segmentChain.ID,
	}

	configBuilder := builder.NewConfigBuilder(
		&settings,
		s.store.GetAllNodes(),
		nil,
		[]storage.InboundPort{inbound},
		[]storage.ProxyChain{segmentChain},
	)
	configBuilder.SetDataDir(workDir)
	configJSON, err := configBuilder.BuildJSON()
	if err != nil {
		cleanup()
		return "", "", "", nil, fmt.Errorf("生成临时 Tor 段配置失败: %w", err)
	}

	configPath := filepath.Join(workDir, "config.json")
	if err := os.WriteFile(configPath, []byte(configJSON), 0600); err != nil {
		cleanup()
		return "", "", "", nil, fmt.Errorf("写入临时 Tor 段配置失败: %w", err)
	}

	return configPath, workDir, net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), cleanup, nil
}

func truncateChainAtTor(chain storage.ProxyChain) (storage.ProxyChain, error) {
	for index, tag := range chain.Nodes {
		if storage.IsChainTorNodeTag(tag) {
			chain.Nodes = append([]string(nil), chain.Nodes[:index+1]...)
			chain.ChainNodes = nil
			return chain, nil
		}
	}
	return storage.ProxyChain{}, fmt.Errorf("链路不是 Tor 链路")
}

func allocateLocalPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("分配临时诊断端口失败: %w", err)
	}
	defer listener.Close()

	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("无法读取临时诊断端口")
	}
	return address.Port, nil
}

func (s *TorDiagnosticsService) resolveSingBoxPath(settings *storage.Settings) (string, error) {
	if settings == nil {
		return "", fmt.Errorf("sing-box 路径未配置")
	}
	singBoxPath := strings.TrimSpace(settings.SingBoxPath)
	if singBoxPath == "" {
		return "", fmt.Errorf("sing-box 路径未配置")
	}
	if filepath.IsAbs(singBoxPath) {
		return singBoxPath, nil
	}
	if resolvedPath, err := exec.LookPath(singBoxPath); err == nil {
		return resolvedPath, nil
	}

	dataDir := s.store.GetDataDir()
	candidates := []string{
		filepath.Join(dataDir, singBoxPath),
		filepath.Join(filepath.Dir(dataDir), singBoxPath),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("sing-box 不存在或不可执行: %s", singBoxPath)
}

type execTorDiagnosticProcess struct {
	cmd      *exec.Cmd
	done     chan error
	stopOnce sync.Once
}

func (s *TorDiagnosticsService) defaultSegmentProcessLauncher(ctx context.Context, executablePath, configPath, workDir string) (torDiagnosticProcess, error) {
	var output bytes.Buffer
	cmd := exec.CommandContext(ctx, executablePath, "run", "-c", configPath)
	cmd.Dir = workDir
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动临时 sing-box 诊断进程失败: %w", err)
	}

	process := &execTorDiagnosticProcess{
		cmd:  cmd,
		done: make(chan error, 1),
	}
	go func() {
		process.done <- cmd.Wait()
	}()

	return process, nil
}

func (p *execTorDiagnosticProcess) Stop() error {
	p.stopOnce.Do(func() {
		if p.cmd != nil && p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
		}
		select {
		case <-p.done:
		case <-time.After(2 * time.Second):
		}
	})
	return nil
}

func waitForProxyPort(ctx context.Context, proxyAddr string) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		dialer := net.Dialer{Timeout: 250 * time.Millisecond}
		conn, err := dialer.DialContext(ctx, "tcp", proxyAddr)
		if err == nil {
			_ = conn.Close()
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("等待临时 Tor 段代理端口就绪超时: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}
