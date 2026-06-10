package api

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/xiaobei/singbox-manager/internal/storage"
)

func TestRebuildConfigAndRestartStopsOnBuildError(t *testing.T) {
	buildErr := errors.New("build failed")
	restartCalled := false

	err := rebuildConfigAndRestart(func() error {
		return buildErr
	}, func() error {
		restartCalled = true
		return nil
	})

	if !errors.Is(err, buildErr) {
		t.Fatalf("expected build error %v, got %v", buildErr, err)
	}
	if restartCalled {
		t.Fatal("restart should not be called when build fails")
	}
}

func TestRebuildConfigAndRestartReturnsRestartError(t *testing.T) {
	restartErr := errors.New("restart failed")
	buildCalled := false
	restartCalled := false

	err := rebuildConfigAndRestart(func() error {
		buildCalled = true
		return nil
	}, func() error {
		restartCalled = true
		return restartErr
	})

	if !buildCalled {
		t.Fatal("build should be called before restart")
	}
	if !restartCalled {
		t.Fatal("restart should be called after build succeeds")
	}
	if !errors.Is(err, restartErr) {
		t.Fatalf("expected restart error %v, got %v", restartErr, err)
	}
}

func TestSanitizeSubscriptionResponsesDoesNotMutateSource(t *testing.T) {
	subs := []storage.Subscription{
		{ID: "sub-1", Name: "local", Content: "proxies:\n  - name: test\n"},
	}

	result := sanitizeSubscriptionResponses(subs)

	if result[0].Content != "" {
		t.Fatalf("sanitized content = %q, want empty", result[0].Content)
	}
	if subs[0].Content == "" {
		t.Fatal("source subscription content was cleared")
	}
}

func TestCheckInboundPortAvailabilityChecksSystemPortForUnchangedStoppedPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer listener.Close()

	portNumber := listener.Addr().(*net.TCPAddr).Port
	store, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	port := storage.InboundPort{
		ID:       "port-1",
		Name:     "test",
		Type:     "mixed",
		Listen:   "127.0.0.1",
		Port:     portNumber,
		Outbound: "Proxy",
		Enabled:  true,
	}
	if err := store.AddInboundPort(port); err != nil {
		t.Fatalf("AddInboundPort() error = %v", err)
	}

	server := &Server{store: store}
	result := server.checkInboundPortAvailability(port)

	if available, _ := result["available"].(bool); available {
		t.Fatal("available = true, want false for occupied port")
	}
	if message, _ := result["message"].(string); !strings.Contains(message, strconv.Itoa(portNumber)) {
		t.Fatalf("message = %q, want occupied port number %d", message, portNumber)
	}
}

func TestInboundPortTestHostUsesIPv6LoopbackForAnyIPv6(t *testing.T) {
	if got := inboundPortTestHost("::"); got != "::1" {
		t.Fatalf("inboundPortTestHost(::) = %q, want ::1", got)
	}
}
