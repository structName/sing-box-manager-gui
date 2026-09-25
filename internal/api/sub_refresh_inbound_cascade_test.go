package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/structName/sing-box-manager-gui/internal/builder"
	"github.com/structName/sing-box-manager-gui/internal/daemon"
	"github.com/structName/sing-box-manager-gui/internal/service"
	"github.com/structName/sing-box-manager-gui/internal/storage"
)

func TestRefreshSubscriptionDisablesInboundOnVanishedTag(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dataDir := t.TempDir()
	store, err := storage.NewJSONStore(dataDir)
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}
	settings := store.GetSettings()
	settings.AutoApply = false
	if err := store.UpdateSettings(settings); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	subID := "sub-1"
	initialContent := "socks://127.0.0.1:1080#hk-1\nsocks://127.0.0.1:1081#jp-1\n"
	if err := store.AddSubscription(storage.Subscription{
		ID: subID, Name: "airport", Enabled: true, Type: "local",
		Content: initialContent, UpdatedAt: time.Now(), NodeCount: 2,
		Nodes: []storage.Node{
			{Tag: "hk-1", Type: "socks", Server: "127.0.0.1", ServerPort: 1080},
			{Tag: "jp-1", Type: "socks", Server: "127.0.0.1", ServerPort: 1081},
		},
	}); err != nil {
		t.Fatalf("AddSubscription: %v", err)
	}
	if err := store.AddInboundPort(storage.InboundPort{
		ID: "port-hk", Name: "hk", Type: "mixed",
		Listen: "127.0.0.1", Port: 19001, Outbound: "hk-1", Enabled: true,
	}); err != nil {
		t.Fatalf("AddInboundPort hk: %v", err)
	}
	if err := store.AddInboundPort(storage.InboundPort{
		ID: "port-jp", Name: "jp", Type: "mixed",
		Listen: "127.0.0.1", Port: 19002, Outbound: "jp-1", Enabled: true,
	}); err != nil {
		t.Fatalf("AddInboundPort jp: %v", err)
	}
	if err := store.AddInboundPort(storage.InboundPort{
		ID: "port-direct", Name: "direct", Type: "mixed",
		Listen: "127.0.0.1", Port: 19003, Outbound: "DIRECT", Enabled: true,
	}); err != nil {
		t.Fatalf("AddInboundPort direct: %v", err)
	}

	chainSync := service.NewChainSyncService(store)
	subSvc := service.NewSubscriptionService(store, chainSync)
	server := &Server{
		store:          store,
		processManager: daemon.NewProcessManager("sing-box", "config.json", dataDir),
		baseDir:        dataDir,
		chainSyncSvc:   chainSync,
		subService:     subSvc,
	}

	// Drop hk-1 from local content before Refresh (Refresh re-parses Content).
	sub := store.GetSubscription(subID)
	sub.Content = "socks://127.0.0.1:1081#jp-1\nsocks://127.0.0.1:1082#tw-1\n"
	if err := store.UpdateSubscription(*sub); err != nil {
		t.Fatalf("UpdateSubscription content: %v", err)
	}

	router := gin.New()
	router.POST("/api/subscriptions/:id/refresh", server.refreshSubscription)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/subscriptions/"+subID+"/refresh", nil)
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	byID := map[string]storage.InboundPort{}
	for _, p := range store.GetInboundPorts() {
		byID[p.ID] = p
	}
	if p := byID["port-hk"]; p.Enabled || p.Outbound != "hk-1" {
		t.Fatalf("port-hk=%#v, want Enabled=false Outbound=hk-1", p)
	}
	if p := byID["port-jp"]; !p.Enabled || p.Outbound != "jp-1" {
		t.Fatalf("port-jp=%#v, want Enabled jp-1", p)
	}
	if p := byID["port-direct"]; !p.Enabled || p.Outbound != "DIRECT" {
		t.Fatalf("port-direct=%#v, want untouched DIRECT", p)
	}

	// Builder must not emit a route to missing hk-1.
	b := builder.NewConfigBuilder(store.GetSettings(), store.GetAllNodes(), nil, store.GetInboundPorts(), store.GetProxyChains())
	cfg, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	outTags := map[string]bool{}
	for _, o := range cfg.Outbounds {
		if tag, _ := o["tag"].(string); tag != "" {
			outTags[tag] = true
		}
	}
	if outTags["hk-1"] {
		t.Fatal("hk-1 should be absent from outbounds after refresh")
	}
	for _, r := range cfg.Route.Rules {
		if out, _ := r["outbound"].(string); out == "hk-1" {
			t.Fatalf("route still targets vanished hk-1: %#v", r)
		}
	}
}

func TestRefreshSubscriptionKeepsInboundWhenTagSurvivesElsewhere(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dataDir := t.TempDir()
	store, err := storage.NewJSONStore(dataDir)
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}
	settings := store.GetSettings()
	settings.AutoApply = false
	if err := store.UpdateSettings(settings); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	subID := "sub-1"
	if err := store.AddSubscription(storage.Subscription{
		ID: subID, Name: "airport", Enabled: true, Type: "local",
		Content: "socks://127.0.0.1:1080#shared\n", UpdatedAt: time.Now(), NodeCount: 1,
		Nodes: []storage.Node{
			{Tag: "shared", Type: "socks", Server: "127.0.0.1", ServerPort: 1080},
		},
	}); err != nil {
		t.Fatalf("AddSubscription: %v", err)
	}
	// Same Tag also exists as a manual node — refresh removing it from the
	// subscription must NOT disable the inbound (tag still alive).
	if err := store.AddManualNode(storage.ManualNode{
		ID: "manual-shared", Enabled: true,
		Node: storage.Node{Tag: "shared", Type: "socks", Server: "10.0.0.1", ServerPort: 1080},
	}); err != nil {
		t.Fatalf("AddManualNode: %v", err)
	}
	if err := store.AddInboundPort(storage.InboundPort{
		ID: "port-shared", Name: "shared", Type: "mixed",
		Listen: "127.0.0.1", Port: 19010, Outbound: "shared", Enabled: true,
	}); err != nil {
		t.Fatalf("AddInboundPort: %v", err)
	}

	chainSync := service.NewChainSyncService(store)
	subSvc := service.NewSubscriptionService(store, chainSync)
	server := &Server{
		store:          store,
		processManager: daemon.NewProcessManager("sing-box", "config.json", dataDir),
		baseDir:        dataDir,
		chainSyncSvc:   chainSync,
		subService:     subSvc,
	}

	sub := store.GetSubscription(subID)
	sub.Content = "socks://127.0.0.1:1082#other\n"
	if err := store.UpdateSubscription(*sub); err != nil {
		t.Fatalf("UpdateSubscription: %v", err)
	}

	router := gin.New()
	router.POST("/api/subscriptions/:id/refresh", server.refreshSubscription)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/subscriptions/"+subID+"/refresh", nil)
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	ports := store.GetInboundPorts()
	if len(ports) != 1 || !ports[0].Enabled || ports[0].Outbound != "shared" {
		t.Fatalf("inbound=%#v, want still enabled shared (manual keeps tag alive)", ports)
	}
}

func TestDisableInboundPortsForRemovedNodeTagsHelper(t *testing.T) {
	dataDir := t.TempDir()
	store, err := storage.NewJSONStore(dataDir)
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}
	if err := store.AddInboundPort(storage.InboundPort{
		ID: "p1", Name: "a", Type: "mixed", Listen: "127.0.0.1", Port: 1,
		Outbound: "gone", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.AddInboundPort(storage.InboundPort{
		ID: "p2", Name: "b", Type: "mixed", Listen: "127.0.0.1", Port: 2,
		Outbound: "alive", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.AddManualNode(storage.ManualNode{
		ID: "n1", Enabled: true,
		Node: storage.Node{Tag: "alive", Type: "socks", Server: "127.0.0.1", ServerPort: 1},
	}); err != nil {
		t.Fatal(err)
	}

	server := &Server{store: store}
	disabled, err := server.disableInboundPortsForRemovedNodeTags([]string{"gone", "alive", "gone", ""})
	if err != nil {
		t.Fatal(err)
	}
	if len(disabled) != 1 || disabled[0].ID != "p1" {
		t.Fatalf("disabled=%#v, want only p1", disabled)
	}
	byID := map[string]storage.InboundPort{}
	for _, p := range store.GetInboundPorts() {
		byID[p.ID] = p
	}
	if byID["p1"].Enabled {
		t.Fatal("p1 should be disabled")
	}
	if !byID["p2"].Enabled {
		t.Fatal("p2 should stay enabled (tag still alive)")
	}
}
