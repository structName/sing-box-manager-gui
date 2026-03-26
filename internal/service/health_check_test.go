package service

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/xiaobei/singbox-manager/internal/storage"
)

func TestFindProxyPort(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		ports          []storage.InboundPort
		preferOutbound string
		want           string
		wantErr        bool
	}{
		{
			name:           "no ports",
			ports:          nil,
			preferOutbound: "us",
			wantErr:        true,
		},
		{
			name: "matching chain outbound",
			ports: []storage.InboundPort{
				{ID: "1", Type: "mixed", Listen: "127.0.0.1", Port: 7890, Outbound: "us", Enabled: true},
			},
			preferOutbound: "us",
			want:           "127.0.0.1:7890",
		},
		{
			name: "fallback to any socks port",
			ports: []storage.InboundPort{
				{ID: "1", Type: "socks", Listen: "127.0.0.1", Port: 1080, Outbound: "jp", Enabled: true},
			},
			preferOutbound: "us",
			want:           "127.0.0.1:1080",
		},
		{
			name: "skip http-only ports",
			ports: []storage.InboundPort{
				{ID: "1", Type: "http", Listen: "127.0.0.1", Port: 8080, Outbound: "us", Enabled: true},
			},
			preferOutbound: "us",
			wantErr:        true,
		},
		{
			name: "skip disabled ports",
			ports: []storage.InboundPort{
				{ID: "1", Type: "mixed", Listen: "127.0.0.1", Port: 7890, Outbound: "us", Enabled: false},
			},
			preferOutbound: "us",
			wantErr:        true,
		},
		{
			name: "wildcard listen defaults to localhost",
			ports: []storage.InboundPort{
				{ID: "1", Type: "mixed", Listen: "0.0.0.0", Port: 7890, Outbound: "us", Enabled: true},
			},
			preferOutbound: "us",
			want:           "127.0.0.1:7890",
		},
		{
			name: "empty listen defaults to localhost",
			ports: []storage.InboundPort{
				{ID: "1", Type: "mixed", Listen: "", Port: 7890, Outbound: "us", Enabled: true},
			},
			preferOutbound: "us",
			want:           "127.0.0.1:7890",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store, err := storage.NewJSONStore(t.TempDir())
			if err != nil {
				t.Fatalf("NewJSONStore() error = %v", err)
			}
			for _, p := range tc.ports {
				if err := store.AddInboundPort(p); err != nil {
					t.Fatalf("AddInboundPort() error = %v", err)
				}
			}

			svc := NewHealthCheckService(store)
			got, err := svc.findProxyPort(tc.preferOutbound)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("findProxyPort() expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("findProxyPort() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("findProxyPort() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTestViaClashAPIIncludesAuthorizationHeader(t *testing.T) {
	t.Parallel()

	settingsStore, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	settings := storage.DefaultSettings()
	settings.ClashAPISecret = "test-secret"
	if err := settingsStore.UpdateSettings(settings); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}

	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"delay":123}`))
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}

	port, err := strconv.Atoi(serverURL.Port())
	if err != nil {
		t.Fatalf("Atoi() error = %v", err)
	}

	service := NewHealthCheckService(settingsStore)
	delay, err := service.testViaClashAPI(port, "Proxy", "https://example.com", time.Second)
	if err != nil {
		t.Fatalf("testViaClashAPI() error = %v", err)
	}

	if delay != 123 {
		t.Fatalf("delay = %d, want 123", delay)
	}

	if authHeader != "Bearer test-secret" {
		t.Fatalf("Authorization header = %q, want %q", authHeader, "Bearer test-secret")
	}
}
