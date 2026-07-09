package deploy

import "testing"

func TestParseScriptOutputKeepsLogsAndStructuredMarkers(t *testing.T) {
	output := `Preparing host
SBM_PROGRESS {"step":"install_singbox","status":"running","message":"Installing sing-box"}
ordinary log line
SBM_NODE_BEGIN
{"type":"vless","tag":"edge-a","server":"vpn.example.com","server_port":443,"extra":{"tls":true}}
SBM_NODE_END
SBM_RESULT {"status":"success","message":"ready"}
`

	parsed, err := ParseScriptOutput(output)
	if err != nil {
		t.Fatalf("ParseScriptOutput() error = %v", err)
	}
	if len(parsed.Progress) != 1 {
		t.Fatalf("progress markers = %#v", parsed.Progress)
	}
	if parsed.Progress[0]["step"] != "install_singbox" || parsed.Progress[0]["status"] != "running" {
		t.Fatalf("unexpected progress marker: %#v", parsed.Progress[0])
	}
	if parsed.Result["status"] != "success" {
		t.Fatalf("unexpected result: %#v", parsed.Result)
	}
	if parsed.GeneratedNode["server"] != "vpn.example.com" || parsed.GeneratedNode["server_port"].(float64) != 443 {
		t.Fatalf("unexpected generated node: %#v", parsed.GeneratedNode)
	}
	if parsed.Log != "Preparing host\nordinary log line\n" {
		t.Fatalf("ordinary log = %q", parsed.Log)
	}
}

func TestParseScriptOutputRejectsMalformedStructuredJSON(t *testing.T) {
	_, err := ParseScriptOutput(`SBM_PROGRESS {"step":`)
	if err == nil {
		t.Fatal("expected malformed progress JSON to fail")
	}
}
