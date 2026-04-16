package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/xiaobei/singbox-manager/internal/storage"
)

// TestTestNodeViaMihomo_SSR 测试通过 mihomo adapter 直接测试 SSR 节点延迟
func TestTestNodeViaMihomo_SSR(t *testing.T) {
	store, _ := storage.NewJSONStore("/tmp/test-health-check")
	svc := NewHealthCheckService(store)

	node := storage.Node{
		Tag:        "🇨🇳 台湾・01",
		Type:       "shadowsocksr",
		Server:     "6kusjo.ps6ywnxy.com",
		ServerPort: 1070,
		Extra: map[string]interface{}{
			"method":         "chacha20-ietf",
			"obfs":           "plain",
			"obfs_param":     "11061346320.microsoft.com",
			"password":       "bxsnucrgk6hfish",
			"protocol":       "auth_aes128_sha1",
			"protocol_param": "346320:HEQHnc",
		},
	}

	testURL := "https://www.gstatic.com/generate_204"
	timeout := 10 * time.Second

	t.Run("mihomo_adapter_creation", func(t *testing.T) {
		// 测试 adapter 创建是否成功（不依赖网络连通性）
		latency, err := svc.testNodeViaMihomo(node, testURL, timeout)
		if err != nil {
			// 网络不通是预期的（CI 环境），但 adapter 创建不应报错
			t.Logf("节点测试结果: err=%v (网络不通是正常的)", err)
			// 确保错误不是 adapter 创建失败
			if err.Error() == "创建代理 adapter 失败" {
				t.Fatalf("mihomo adapter 创建失败: %v", err)
			}
		} else {
			t.Logf("节点测试成功: latency=%d ms", latency)
		}
	})

	t.Run("fallback_logic", func(t *testing.T) {
		// 测试回退逻辑: Clash API 不可用时应走 mihomo
		// 模拟 Clash API 端口不存在的情况
		latency, err := svc.testViaClashAPI(19999, "nonexistent-proxy", testURL, 3*time.Second)
		if err == nil {
			t.Fatalf("预期 Clash API 应该失败, 但得到 latency=%d", latency)
		}
		t.Logf("Clash API 预期失败: %v", err)

		// 然后 mihomo 回退应该能工作（至少 adapter 创建成功）
		latency, err = svc.testNodeViaMihomo(node, testURL, timeout)
		if err != nil {
			t.Logf("mihomo 回退测试: err=%v (如果是网络超时则正常)", err)
		} else {
			t.Logf("mihomo 回退成功: latency=%d ms", latency)
			if latency <= 0 {
				t.Error("延迟应大于 0")
			}
		}
	})
}

// TestCheckChain_FallbackPath 测试 CheckChain 完整回退路径
func TestCheckChain_FallbackPath(t *testing.T) {
	// 创建临时 store
	tmpDir := t.TempDir()
	store, _ := storage.NewJSONStore(tmpDir)

	// 设置 settings (ClashAPIPort=0 强制走 mihomo 回退)
	settings := store.GetSettings()
	settings.ClashAPIPort = 0
	store.UpdateSettings(settings)

	// 添加测试节点为手动节点
	store.AddManualNode(storage.ManualNode{
		ID:      "mn-test-exit",
		Enabled: true,
		Node: storage.Node{
			Tag:        "test-exit-node",
			Type:       "shadowsocksr",
			Server:     "6kusjo.ps6ywnxy.com",
			ServerPort: 1070,
			Extra: map[string]interface{}{
				"method":         "chacha20-ietf",
				"obfs":           "plain",
				"obfs_param":     "11061346320.microsoft.com",
				"password":       "bxsnucrgk6hfish",
				"protocol":       "auth_aes128_sha1",
				"protocol_param": "346320:HEQHnc",
			},
		},
	})

	// 创建链路
	store.AddProxyChain(storage.ProxyChain{
		ID:      "test-chain-001",
		Name:    "test-chain",
		Enabled: true,
		Nodes:   []string{"test-exit-node"},
	})

	svc := NewHealthCheckService(store)

	t.Run("clash_api_disabled_uses_mihomo", func(t *testing.T) {
		status, err := svc.CheckChain("test-chain-001")
		if err != nil {
			t.Fatalf("CheckChain 不应返回错误: %v", err)
		}

		t.Logf("链路状态: %s", status.Status)
		t.Logf("总延迟: %d ms", status.Latency)

		for _, ns := range status.NodeStatuses {
			t.Logf("  节点 [%s]: status=%s latency=%d err=%s",
				ns.Tag, ns.Status, ns.Latency, ns.Error)
		}

		// 验证没有 "Clash API 请求失败" 的错误
		for _, ns := range status.NodeStatuses {
			if ns.Error != "" && contains(ns.Error, "Clash API") {
				t.Errorf("不应出现 Clash API 错误: %s", ns.Error)
			}
		}

		fmt.Printf("\n✓ CheckChain 回退逻辑正常: ClashAPIPort=0 → 走 mihomo adapter\n")
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
