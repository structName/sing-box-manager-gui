package parser

import (
	"testing"
)

func TestSocksParser_Parse(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		wantServer   string
		wantPort     int
		wantUsername string
		wantPassword string
		wantVersion  string
		wantErr      bool
	}{
		{
			name:         "Base64 encoded auth",
			url:          "socks://dGVzdHVzZXI6dGVzdHBhc3M=@192.168.1.100:1080#测试节点",
			wantServer:   "192.168.1.100",
			wantPort:     1080,
			wantUsername: "testuser",
			wantPassword: "testpass",
			wantVersion:  "5",
			wantErr:      false,
		},
		{
			name:         "Plain text auth",
			url:          "socks://user:pass@example.com:1080#test",
			wantServer:   "example.com",
			wantPort:     1080,
			wantUsername: "user",
			wantPassword: "pass",
			wantVersion:  "5",
			wantErr:      false,
		},
		{
			name:         "No auth",
			url:          "socks://192.168.1.1:1080#no-auth",
			wantServer:   "192.168.1.1",
			wantPort:     1080,
			wantUsername: "",
			wantPassword: "",
			wantVersion:  "5",
			wantErr:      false,
		},
		{
			name:         "SOCKS5 explicit",
			url:          "socks5://user:pass@example.com:1080#socks5-test",
			wantServer:   "example.com",
			wantPort:     1080,
			wantUsername: "user",
			wantPassword: "pass",
			wantVersion:  "5",
			wantErr:      false,
		},
		{
			name:         "SOCKS4",
			url:          "socks4://user@example.com:1080#socks4-test",
			wantServer:   "example.com",
			wantPort:     1080,
			wantUsername: "user",
			wantPassword: "",
			wantVersion:  "4",
			wantErr:      false,
		},
		{
			name:         "SOCKS4A keeps remote DNS version",
			url:          "socks4a://user@example.com:1080#socks4a-test",
			wantServer:   "example.com",
			wantPort:     1080,
			wantUsername: "user",
			wantPassword: "",
			wantVersion:  "4a",
			wantErr:      false,
		},
		{
			name:         "IPv6 address",
			url:          "socks://user:pass@[::1]:1080#ipv6-test",
			wantServer:   "::1",
			wantPort:     1080,
			wantUsername: "user",
			wantPassword: "pass",
			wantVersion:  "5",
			wantErr:      false,
		},
	}

	parser := &SocksParser{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := parser.Parse(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("SocksParser.Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if node.Server != tt.wantServer {
				t.Errorf("Server = %v, want %v", node.Server, tt.wantServer)
			}
			if node.ServerPort != tt.wantPort {
				t.Errorf("ServerPort = %v, want %v", node.ServerPort, tt.wantPort)
			}
			if node.Type != "socks" {
				t.Errorf("Type = %v, want socks", node.Type)
			}

			if username, ok := node.Extra["username"].(string); ok {
				if username != tt.wantUsername {
					t.Errorf("username = %v, want %v", username, tt.wantUsername)
				}
			} else if tt.wantUsername != "" {
				t.Errorf("username not found, want %v", tt.wantUsername)
			}

			if password, ok := node.Extra["password"].(string); ok {
				if password != tt.wantPassword {
					t.Errorf("password = %v, want %v", password, tt.wantPassword)
				}
			} else if tt.wantPassword != "" {
				t.Errorf("password not found, want %v", tt.wantPassword)
			}

			if version, ok := node.Extra["version"].(string); ok {
				if version != tt.wantVersion {
					t.Errorf("version = %v, want %v", version, tt.wantVersion)
				}
			}
		})
	}
}

func TestParseURL_Socks(t *testing.T) {
	// 测试通过 ParseURL 函数解析 (Base64: testuser:testpass)
	url := "socks://dGVzdHVzZXI6dGVzdHBhc3M=@192.168.1.100:1080#美国-测试节点"
	node, err := ParseURL(url)
	if err != nil {
		t.Fatalf("ParseURL() error = %v", err)
	}

	if node.Type != "socks" {
		t.Errorf("Type = %v, want socks", node.Type)
	}
	if node.Server != "192.168.1.100" {
		t.Errorf("Server = %v, want 192.168.1.100", node.Server)
	}
	if node.ServerPort != 1080 {
		t.Errorf("ServerPort = %v, want 1080", node.ServerPort)
	}
	if node.Tag != "美国-测试节点" {
		t.Errorf("Tag = %v, want 美国-测试节点", node.Tag)
	}
	if node.Country != "US" {
		t.Errorf("Country = %v, want US", node.Country)
	}
}
