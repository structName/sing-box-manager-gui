//go:build live

package api

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/structName/sing-box-manager-gui/internal/deploy"
	"github.com/structName/sing-box-manager-gui/internal/storage"
)

func TestLiveManagedEntrypointProxiesHTTPS(t *testing.T) {
	profileDir := strings.TrimSpace(os.Getenv("SBM_LIVE_PROFILE_DIR"))
	if profileDir == "" {
		t.Skip("set SBM_LIVE_PROFILE_DIR to a copied profile directory")
	}
	dataDir := strings.TrimSpace(os.Getenv("SBM_LIVE_DATA_DIR"))
	if dataDir == "" {
		t.Skip("set SBM_LIVE_DATA_DIR to a copied sing-box-manager data directory")
	}
	probeURL := strings.TrimSpace(os.Getenv("SBM_LIVE_PROBE_URL"))
	if probeURL == "" {
		probeURL = "https://www.gstatic.com/generate_204"
	}

	store, err := storage.NewJSONStore(profileDir)
	if err != nil {
		t.Fatalf("NewJSONStore(%q): %v", profileDir, err)
	}
	candidates := deploy.DiscoverConnectionCandidates(deploy.CandidateInventoryInput{
		Settings:       store.GetSettings(),
		Nodes:          store.GetAllNodes(),
		ProxyChains:    store.GetProxyChains(),
		InboundPorts:   store.GetInboundPorts(),
		ServiceRunning: false,
	})

	var candidate deploy.ConnectionCandidate
	for _, item := range candidates {
		if item.Available && item.RequiresTemporaryEntrypoint && strings.TrimSpace(item.Outbound) != "" {
			candidate = item
			break
		}
	}
	if candidate.ID == "" {
		t.Fatalf("no usable temporary-entrypoint candidate found; candidate_count=%d", len(candidates))
	}
	candidateLabel := candidate.Kind
	if candidate.Source != "" {
		candidateLabel += "/" + candidate.Source
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	entrypoint, err := (singBoxDeploymentManagedEntrypointStarter{}).Start(ctx, deploymentManagedEntrypointInput{
		Candidate:   candidate,
		Settings:    store.GetSettings(),
		Nodes:       store.GetAllNodes(),
		ProxyChains: store.GetProxyChains(),
		DataDir:     dataDir,
	})
	if err != nil {
		t.Fatalf("start managed entrypoint for %s: %v", candidate.ID, err)
	}
	defer entrypoint.Cleanup()

	curlCtx, curlCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer curlCancel()
	cmd := exec.CommandContext(
		curlCtx,
		"curl",
		"-fsS",
		"-o", "/dev/null",
		"-w", "http_code=%{http_code} remote_ip=%{remote_ip} time_total=%{time_total}\n",
		"--connect-timeout", "10",
		"--max-time", "25",
		"--socks5-hostname", entrypoint.LocalEndpoint,
		probeURL,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("curl through managed entrypoint %s (%s) failed: %v\n%s", entrypoint.LocalEndpoint, candidateLabel, err, output)
	}
	t.Logf("managed entrypoint %s via %s: %s", entrypoint.LocalEndpoint, candidateLabel, strings.TrimSpace(string(output)))
}

func TestLiveManagedEntrypointRejectsLegacySettingsMixedOnly(t *testing.T) {
	profileDir := strings.TrimSpace(os.Getenv("SBM_LIVE_PROFILE_DIR"))
	if profileDir == "" {
		t.Skip("set SBM_LIVE_PROFILE_DIR to a copied profile directory")
	}
	store, err := storage.NewJSONStore(profileDir)
	if err != nil {
		t.Fatalf("NewJSONStore(%q): %v", profileDir, err)
	}
	candidates := deploy.DiscoverConnectionCandidates(deploy.CandidateInventoryInput{
		Settings:       store.GetSettings(),
		Nodes:          store.GetAllNodes(),
		ProxyChains:    store.GetProxyChains(),
		InboundPorts:   store.GetInboundPorts(),
		ServiceRunning: true,
	})
	for _, candidate := range candidates {
		if candidate.ID == "global:mixed" || candidate.Kind == "global_mixed" {
			t.Fatalf("legacy settings mixed port was advertised as deployment route: %#v", candidate)
		}
	}
}
