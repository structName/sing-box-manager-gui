package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/structName/sing-box-manager-gui/internal/builder"
)

func TestApplySkipFieldsKeepsReasonAndDoesNotOverwriteFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	result := &builder.ValidatedConfig{
		SkippedNodes: []builder.SkippedNode{{
			Tag:    "bad-plugin",
			Type:   "shadowsocks",
			Reason: "unsupported plugin",
		}},
		SkippedChains: []builder.SkippedChain{{
			ID:     "chain-1",
			Name:   "missing-hop",
			Reason: "missing hops: exit",
		}},
	}
	resp := gin.H{"warning": "更新成功，但自动应用配置失败: restart"}
	applySkipFields(resp, result)

	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(encoded)
	for _, want := range []string{"bad-plugin", "unsupported plugin", "missing-hop", "missing hops: exit", "restart"} {
		if !strings.Contains(body, want) {
			t.Fatalf("response missing %q: %s", want, body)
		}
	}
	if strings.Contains(body, "node-secret") {
		t.Fatalf("unexpected secret in skip payload: %s", body)
	}
}

func TestConfigSkipHeadersDoNotIncludeSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	setConfigSkipHeaders(ctx, &builder.ValidatedConfig{
		SkippedNodes: []builder.SkippedNode{{
			Tag:    "bad-plugin",
			Type:   "shadowsocks",
			Reason: "unsupported plugin",
		}},
	})
	if got := recorder.Header().Get("X-SBM-Skipped-Nodes"); got != "1" {
		t.Fatalf("skipped node count header = %q", got)
	}
	warning := recorder.Header().Get("X-SBM-Config-Warning")
	if !strings.Contains(warning, "bad-plugin") || strings.Contains(warning, "node-secret") {
		t.Fatalf("warning header = %q", warning)
	}
}

func TestServiceStatusIncludesLastSkips(t *testing.T) {
	server := &Server{}
	server.rememberConfigSkips(&builder.ValidatedConfig{
		SkippedChains: []builder.SkippedChain{{
			ID:     "chain-1",
			Name:   "missing-hop",
			Reason: "missing hops: exit",
		}},
	})
	nodes, chains := server.configSkips()
	if len(nodes) != 0 || len(chains) != 1 || chains[0].Reason != "missing hops: exit" {
		t.Fatalf("status skips = %#v %#v", nodes, chains)
	}
}
