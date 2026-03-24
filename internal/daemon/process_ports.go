package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	processExitTimeout  = 5 * time.Second
	processPollInterval = 100 * time.Millisecond
)

type singboxConfigSnapshot struct {
	Inbounds []singboxInboundSnapshot `json:"inbounds"`
}

type singboxInboundSnapshot struct {
	Type       string `json:"type"`
	Listen     string `json:"listen"`
	ListenPort int    `json:"listen_port"`
}

func (pm *ProcessManager) waitForProcessExit(pid int, exitCh chan struct{}) error {
	if exitCh != nil {
		select {
		case <-exitCh:
			return nil
		case <-time.After(processExitTimeout):
			return fmt.Errorf("等待 sing-box 退出超时, PID: %d", pid)
		}
	}

	deadline := time.Now().Add(processExitTimeout)
	for time.Now().Before(deadline) {
		if pid <= 0 || !pm.isProcessAlive(pid) {
			return nil
		}
		time.Sleep(processPollInterval)
	}

	return fmt.Errorf("等待 sing-box 退出超时, PID: %d", pid)
}

func (pm *ProcessManager) ensureInboundPortsAvailable() error {
	inbounds, err := pm.loadPortCheckInbounds()
	if err != nil {
		return err
	}

	var occupied []string
	for _, inbound := range inbounds {
		if !isInboundPortAvailable(inbound.Listen, inbound.ListenPort) {
			occupied = append(occupied, formatInboundEndpoint(inbound.Listen, inbound.ListenPort))
		}
	}

	if len(occupied) > 0 {
		return fmt.Errorf("入站端口被占用: %s", strings.Join(occupied, ", "))
	}

	return nil
}

func (pm *ProcessManager) loadPortCheckInbounds() ([]singboxInboundSnapshot, error) {
	content, err := os.ReadFile(pm.configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config singboxConfigSnapshot
	if err := json.Unmarshal(content, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	inbounds := make([]singboxInboundSnapshot, 0, len(config.Inbounds))
	for _, inbound := range config.Inbounds {
		if inbound.ListenPort <= 0 || inbound.Type == "tun" {
			continue
		}
		inbounds = append(inbounds, inbound)
	}

	return inbounds, nil
}

func isInboundPortAvailable(listen string, port int) bool {
	address := formatListenAddress(listen, port)
	ln, err := net.Listen("tcp", address)
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func formatListenAddress(listen string, port int) string {
	if listen == "" || listen == "0.0.0.0" || listen == "::" {
		return ":" + strconv.Itoa(port)
	}
	return net.JoinHostPort(listen, strconv.Itoa(port))
}

func formatInboundEndpoint(listen string, port int) string {
	if listen == "" {
		listen = "0.0.0.0"
	}
	return net.JoinHostPort(listen, strconv.Itoa(port))
}
