package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/structName/sing-box-manager-gui/internal/builder"
	"github.com/structName/sing-box-manager-gui/internal/database/models"
	"github.com/structName/sing-box-manager-gui/internal/deploy"
	"github.com/structName/sing-box-manager-gui/internal/kernel"
	"github.com/structName/sing-box-manager-gui/internal/storage"
)

type deploymentManagedEntrypointStarter interface {
	Start(ctx context.Context, input deploymentManagedEntrypointInput) (deploymentManagedEntrypoint, error)
}

type deploymentManagedEntrypointInput struct {
	Candidate   deploy.ConnectionCandidate
	Settings    *storage.Settings
	Nodes       []storage.Node
	ProxyChains []storage.ProxyChain
	DataDir     string
}

type deploymentManagedEntrypoint struct {
	LocalEndpoint string
	Cleanup       func()
}

type singBoxDeploymentManagedEntrypointStarter struct{}

func (s *Server) prepareDeploymentManagedConnection(ctx context.Context, req *deploymentConnectionReq) (func(), error) {
	if req == nil || req.Mode != models.DeploymentConnectionManagedProxy {
		return func() {}, nil
	}
	if req.ProxyType == "socks5" && strings.TrimSpace(req.ProxyHost) != "" && req.ProxyPort > 0 {
		return func() {}, nil
	}
	candidate, ok := deploymentCandidateByID(s.deploymentConnectionCandidates(), req.ManagedCandidateID)
	if !ok {
		return nil, fmt.Errorf("托管路线不存在")
	}
	if !candidate.Available {
		if candidate.UnavailableReason != "" {
			return nil, fmt.Errorf("托管路线不可用: %s", candidate.UnavailableReason)
		}
		return nil, fmt.Errorf("托管路线不可用")
	}
	if strings.TrimSpace(candidate.LocalEndpoint) != "" {
		return configureDeploymentManagedEndpoint(req, candidate.LocalEndpoint)
	}
	if !candidate.RequiresTemporaryEntrypoint {
		return nil, fmt.Errorf("托管路线缺少本地入口")
	}
	starter := s.deployEntrypointStarter
	if starter == nil {
		starter = singBoxDeploymentManagedEntrypointStarter{}
	}
	entrypoint, err := starter.Start(ctx, deploymentManagedEntrypointInput{
		Candidate:   candidate,
		Settings:    s.store.GetSettings(),
		Nodes:       s.store.GetAllNodes(),
		ProxyChains: s.store.GetProxyChains(),
		DataDir:     s.deploymentDataDir(),
	})
	if err != nil {
		return nil, fmt.Errorf("启动托管路线临时入口失败: %w", err)
	}
	cleanup, err := configureDeploymentManagedEndpoint(req, entrypoint.LocalEndpoint)
	if err != nil {
		if entrypoint.Cleanup != nil {
			entrypoint.Cleanup()
		}
		return nil, err
	}
	return func() {
		cleanup()
		if entrypoint.Cleanup != nil {
			entrypoint.Cleanup()
		}
	}, nil
}

func deploymentCandidateByID(candidates []deploy.ConnectionCandidate, id string) (deploy.ConnectionCandidate, bool) {
	for _, candidate := range candidates {
		if candidate.ID == id {
			return candidate, true
		}
	}
	return deploy.ConnectionCandidate{}, false
}

func configureDeploymentManagedEndpoint(req *deploymentConnectionReq, endpoint string) (func(), error) {
	host, portText, err := net.SplitHostPort(strings.TrimSpace(endpoint))
	if err != nil {
		return nil, fmt.Errorf("托管路线入口无效")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("托管路线入口端口无效")
	}
	previous := *req
	req.ProxyType = "socks5"
	req.ProxyHost = strings.TrimSpace(host)
	req.ProxyPort = port
	req.ProxyUsername = ""
	req.ProxyPassword = ""
	return func() {
		*req = previous
	}, nil
}

func (singBoxDeploymentManagedEntrypointStarter) Start(ctx context.Context, input deploymentManagedEntrypointInput) (deploymentManagedEntrypoint, error) {
	if input.Settings == nil {
		input.Settings = storage.DefaultSettings()
	}
	if strings.TrimSpace(input.Candidate.Outbound) == "" {
		return deploymentManagedEntrypoint{}, fmt.Errorf("托管路线缺少出站目标")
	}
	binPath := kernel.DefaultBinPath(input.DataDir)
	if _, err := os.Stat(binPath); err != nil {
		if _, installErr := kernel.EnsureBundledInstalled(input.DataDir); installErr != nil {
			return deploymentManagedEntrypoint{}, installErr
		}
	}
	if _, err := os.Stat(binPath); err != nil {
		return deploymentManagedEntrypoint{}, fmt.Errorf("sing-box 内核不可用: %w", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return deploymentManagedEntrypoint{}, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	workDir, err := os.MkdirTemp("", "sbm-deploy-entrypoint-*")
	if err != nil {
		return deploymentManagedEntrypoint{}, err
	}
	cleanupWorkDir := true
	defer func() {
		if cleanupWorkDir {
			_ = os.RemoveAll(workDir)
		}
	}()

	configJSON, err := temporaryDeploymentEntrypointConfig(input, workDir, port)
	if err != nil {
		return deploymentManagedEntrypoint{}, err
	}
	configPath := filepath.Join(workDir, "config.json")
	if err := os.WriteFile(configPath, []byte(configJSON), 0o600); err != nil {
		return deploymentManagedEntrypoint{}, err
	}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, binPath, "run", "-c", configPath)
	cmd.Dir = workDir
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return deploymentManagedEntrypoint{}, err
	}

	exited := make(chan error, 1)
	go func() {
		exited <- cmd.Wait()
	}()

	endpoint := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	if err := waitForDeploymentEntrypoint(ctx, endpoint, exited, &stderr); err != nil {
		select {
		case <-exited:
		default:
			_ = cmd.Process.Kill()
			<-exited
		}
		return deploymentManagedEntrypoint{}, err
	}

	cleanupWorkDir = false
	return deploymentManagedEntrypoint{
		LocalEndpoint: endpoint,
		Cleanup: func() {
			if cmd.Process != nil {
				_ = cmd.Process.Signal(os.Interrupt)
			}
			select {
			case <-exited:
			case <-time.After(2 * time.Second):
				if cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
				<-exited
			}
			_ = os.RemoveAll(workDir)
		},
	}, nil
}

func temporaryDeploymentEntrypointConfig(input deploymentManagedEntrypointInput, dataDir string, port int) (string, error) {
	settings := *input.Settings
	settings.TunEnabled = false
	settings.ClashAPIPort = 0
	settings.ClashUIEnabled = false
	settings.FakeIPEnabled = false

	inbound := storage.InboundPort{
		ID:      "deployment-managed",
		Name:    "Deployment managed route",
		Type:    "socks",
		Listen:  "127.0.0.1",
		Port:    port,
		Enabled: true,
	}
	if input.Candidate.Kind == deploy.CandidateKindProxyChain && storage.ChainContainsTor(chainNodesByID(input.ProxyChains, input.Candidate.ChainID)) {
		inbound.UseTorExit = true
		inbound.TorChainID = input.Candidate.ChainID
	} else {
		inbound.Outbound = input.Candidate.Outbound
	}

	configBuilder := builder.NewConfigBuilder(&settings, input.Nodes, nil, []storage.InboundPort{inbound}, input.ProxyChains)
	configBuilder.SetDataDir(dataDir)
	config, err := configBuilder.Build()
	if err != nil {
		return "", err
	}
	config.Route.Final = "REJECT"
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func chainNodesByID(chains []storage.ProxyChain, id string) []string {
	for _, chain := range chains {
		if chain.ID == id {
			return chain.Nodes
		}
	}
	return nil
}

func waitForDeploymentEntrypoint(ctx context.Context, endpoint string, exited <-chan error, stderr *bytes.Buffer) error {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-exited:
			if err != nil {
				return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
			}
			return fmt.Errorf("sing-box 临时入口已退出")
		default:
		}
		conn, err := net.DialTimeout("tcp", endpoint, 150*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("等待本地临时入口超时: %s", strings.TrimSpace(stderr.String()))
}
