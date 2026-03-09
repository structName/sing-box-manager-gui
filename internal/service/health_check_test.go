package service

import (
	"testing"

	"github.com/xiaobei/singbox-manager/internal/storage"
)

func TestGetMixedProxyAddress(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		settings *storage.Settings
		want     string
	}{
		{
			name: "default localhost",
			settings: &storage.Settings{
				MixedPort: 2080,
			},
			want: "127.0.0.1:2080",
		},
		{
			name: "lan enabled with wildcard ip",
			settings: &storage.Settings{
				MixedPort:       2080,
				LanProxyEnabled: true,
				LanListenIP:     "0.0.0.0",
			},
			want: "127.0.0.1:2080",
		},
		{
			name: "lan enabled with specific ip",
			settings: &storage.Settings{
				MixedPort:       2080,
				LanProxyEnabled: true,
				LanListenIP:     "192.168.31.10",
			},
			want: "192.168.31.10:2080",
		},
		{
			name: "lan enabled with ipv6",
			settings: &storage.Settings{
				MixedPort:       2080,
				LanProxyEnabled: true,
				LanListenIP:     "fe80::1",
			},
			want: "[fe80::1]:2080",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := getMixedProxyAddress(testCase.settings)
			if got != testCase.want {
				t.Fatalf("getMixedProxyAddress() = %q, want %q", got, testCase.want)
			}
		})
	}
}
