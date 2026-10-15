# sing-box-manager-gui developer targets
#
# Socks mihomo mapping + rename-cascade regression gate (#63 / closed #23):
#   make test-socks-rename-gate
# Minimum packages (keep green on main when touching tester / chain_sync / router):
#   ./internal/speedtest  — Socks4/5/default/UoT mihomo mapping
#   ./internal/service    — Retarget / Chain prune
#   ./internal/api        — UpdateManualNode / DeleteManualNode cascade (+ embed stub)

.PHONY: ensure-web-dist-stub test-socks-rename-gate

ensure-web-dist-stub:
	@scripts/ensure-web-dist-stub.sh

# Focused -run set documents the #23 regression surface; packages are the gate.
test-socks-rename-gate: ensure-web-dist-stub
	go test ./internal/speedtest -count=1 -run 'TestNodeToMihomoProxySocks'
	go test ./internal/service -count=1 -run 'TestRetargetNodeTag|TestRemoveNodeTagPrunesFromChain|TestSyncChainNodesPrunesDeletedManualNode|TestSyncChainNodesKeepsDisabledManualNodeRefs'
	go test ./internal/api -count=1 -run 'TestUpdateManualNode|TestDeleteManualNode'
