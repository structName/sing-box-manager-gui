package deploy

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestTemplateRegistryIncludesProbeAndVLESSRealityWithChecksums(t *testing.T) {
	registry := BuiltinTemplates()

	probe, ok := registry.Get("probe-system")
	if !ok {
		t.Fatal("probe-system template is not registered")
	}
	if probe.UserSelectable {
		t.Fatal("probe-system should be an automatic prerequisite, not user-selectable")
	}
	if probe.Checksum == "" || probe.Checksum != sha256Hex(probe.Content) {
		t.Fatalf("probe checksum not stable: %#v", probe)
	}

	vless, ok := registry.Get("singbox-vless-reality")
	if !ok {
		t.Fatal("singbox-vless-reality template is not registered")
	}
	if !vless.UserSelectable {
		t.Fatal("singbox-vless-reality should be user-selectable")
	}
	if vless.Runtime.Name != "sing-box" || vless.Runtime.Version != PinnedSingBoxVersion {
		t.Fatalf("unexpected runtime metadata: %#v", vless.Runtime)
	}
	if vless.Runtime.Checksums["linux-amd64"] == "" || vless.Runtime.Checksums["linux-arm64"] == "" {
		t.Fatalf("runtime checksum metadata missing: %#v", vless.Runtime.Checksums)
	}
	params := make(map[string]TemplateParameter, len(vless.Parameters))
	for _, param := range vless.Parameters {
		params[param.Name] = param
	}
	if params["node_server"].Type != "string" || !params["node_server"].Required {
		t.Fatalf("node_server parameter metadata missing: %#v", vless.Parameters)
	}
	if params["proxy_port"].Type != "integer" || params["proxy_port"].Default != "443" {
		t.Fatalf("proxy_port parameter metadata missing: %#v", vless.Parameters)
	}
	if source := params["runtime_source"]; source.Type != "select" || source.Default != "remote" || !slices.Equal(source.Options, []string{"remote", "cache"}) {
		t.Fatalf("runtime_source parameter metadata missing: %#v", vless.Parameters)
	}
	if !params["sbm_uuid"].Secret || !params["reality_short_id"].Secret || !params["reality_private_key"].Secret {
		t.Fatalf("secret parameter metadata missing: %#v", vless.Parameters)
	}
	if checksum, ok := RuntimeChecksum("sing-box", PinnedSingBoxVersion, "linux", "amd64"); !ok || checksum != vless.Runtime.Checksums["linux-amd64"] {
		t.Fatalf("RuntimeChecksum(linux-amd64) = %q %v", checksum, ok)
	}
	if vless.Checksum == "" || vless.Checksum != sha256Hex(vless.Content) {
		t.Fatalf("vless checksum not stable: %#v", vless)
	}

	security, ok := registry.Get("security-basic")
	if !ok {
		t.Fatal("security-basic template is not registered")
	}
	if !security.UserSelectable {
		t.Fatal("security-basic should be user-selectable")
	}
	if security.Runtime.Name != "" || security.Runtime.Version != "" {
		t.Fatalf("security-basic should not carry proxy runtime metadata: %#v", security.Runtime)
	}
	securityParams := make(map[string]TemplateParameter, len(security.Parameters))
	for _, param := range security.Parameters {
		securityParams[param.Name] = param
	}
	if securityParams["security_install_base_packages"].Type != "boolean" || securityParams["security_install_base_packages"].Default != "false" {
		t.Fatalf("security install parameter metadata missing: %#v", security.Parameters)
	}
	if mode := securityParams["security_firewall_mode"]; mode.Type != "select" || mode.Default != "inspect_only" || !slices.Equal(mode.Options, []string{"inspect_only"}) {
		t.Fatalf("security firewall parameter metadata missing: %#v", security.Parameters)
	}
	if security.Checksum == "" || security.Checksum != sha256Hex(security.Content) {
		t.Fatalf("security-basic checksum not stable: %#v", security)
	}
}

func TestRuntimeCacheFindsSupportedArchiveAndRejectsUnsupportedArch(t *testing.T) {
	cache := NewRuntimeCache(t.TempDir())
	content := []byte("archive")
	checksum := sha256Hex(content)
	path, err := cache.Put("sing-box", PinnedSingBoxVersion, "linux", "amd64", content)
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if filepath.Base(path) != "linux-amd64.tar.gz" {
		t.Fatalf("archive path = %s", path)
	}

	archive, err := cache.Lookup("sing-box", PinnedSingBoxVersion, "linux", "amd64", checksum)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if archive.Path != path || archive.Checksum != checksum {
		t.Fatalf("unexpected archive: %#v", archive)
	}

	if _, err := cache.Lookup("sing-box", PinnedSingBoxVersion, "linux", "386", checksum); err == nil {
		t.Fatal("expected unsupported architecture to fail")
	}
}

func TestRuntimeCacheListsPinnedArchiveStatus(t *testing.T) {
	cache := NewRuntimeCache(t.TempDir())
	if _, err := cache.Put("sing-box", PinnedSingBoxVersion, "linux", "amd64", []byte("not the official archive")); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	archives, err := cache.List("sing-box", PinnedSingBoxVersion)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	byArch := make(map[string]RuntimeArchive, len(archives))
	for _, archive := range archives {
		byArch[archive.Arch] = archive
	}
	amd64 := byArch["amd64"]
	if amd64.Status != "invalid" || amd64.Path == "" || amd64.Checksum == "" || amd64.ExpectedChecksum == "" || !amd64.Downloadable || amd64.DownloadURL == "" {
		t.Fatalf("amd64 archive status = %#v", amd64)
	}
	arm64 := byArch["arm64"]
	if arm64.Status != "missing" || arm64.Path == "" || arm64.Checksum != "" || arm64.ExpectedChecksum == "" || !arm64.Downloadable || arm64.DownloadURL == "" {
		t.Fatalf("arm64 archive status = %#v", arm64)
	}
}

func TestRuntimeCacheRejectsNonPinnedRuntime(t *testing.T) {
	cache := NewRuntimeCache(t.TempDir())

	for _, tc := range []struct {
		name    string
		version string
	}{
		{name: "xray", version: PinnedSingBoxVersion},
		{name: "sing-box", version: "1.14.0"},
	} {
		if _, err := cache.Put(tc.name, tc.version, "linux", "amd64", []byte("archive")); err == nil {
			t.Fatalf("Put(%s, %s) expected error", tc.name, tc.version)
		}
		if _, err := cache.Lookup(tc.name, tc.version, "linux", "amd64", ""); err == nil {
			t.Fatalf("Lookup(%s, %s) expected error", tc.name, tc.version)
		}
		if _, err := cache.List(tc.name, tc.version); err == nil {
			t.Fatalf("List(%s, %s) expected error", tc.name, tc.version)
		}
		if url := RuntimeDownloadURL(tc.name, tc.version, "linux", "amd64"); url != "" {
			t.Fatalf("RuntimeDownloadURL(%s, %s) = %q", tc.name, tc.version, url)
		}
	}
}

func TestRuntimeCacheHelperScriptMatchesPinnedMetadata(t *testing.T) {
	scriptPath := filepath.Join("..", "..", "scripts", "runtime-cache", "download-singbox.sh")
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read runtime cache helper: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, `version="`+PinnedSingBoxVersion+`"`) {
		t.Fatalf("helper script does not default to pinned version %s", PinnedSingBoxVersion)
	}
	for _, target := range []struct {
		arch string
		varn string
	}{
		{arch: "amd64", varn: "checksum_amd64"},
		{arch: "arm64", varn: "checksum_arm64"},
	} {
		checksum, ok := RuntimeChecksum("sing-box", PinnedSingBoxVersion, "linux", target.arch)
		if !ok {
			t.Fatalf("missing checksum metadata for %s", target.arch)
		}
		if !strings.Contains(text, target.varn+`="`+checksum+`"`) {
			t.Fatalf("helper script %s does not match pinned checksum %s", target.varn, checksum)
		}
	}
}

func TestProbeScriptEmitsResultMarkerWhenRunLocally(t *testing.T) {
	probe, ok := BuiltinTemplates().Get("probe-system")
	if !ok {
		t.Fatal("probe-system template is not registered")
	}

	path := filepath.Join(t.TempDir(), "probe-system.sh")
	if err := os.WriteFile(path, []byte(probe.Content), 0755); err != nil {
		t.Fatalf("write probe script: %v", err)
	}

	output, err := RunLocalShell(path, map[string]string{"SBM_PROXY_PORT": "443"})
	if err != nil {
		t.Fatalf("RunLocalShell() error = %v, output = %s", err, output)
	}
	parsed, err := ParseScriptOutput(output)
	if err != nil {
		t.Fatalf("ParseScriptOutput() error = %v, output = %s", err, output)
	}
	if parsed.Result["status"] == "" || parsed.Result["os"] == "" || parsed.Result["arch"] == "" || parsed.Result["kernel"] == "" {
		t.Fatalf("probe result missing required keys: %#v", parsed.Result)
	}
}

func TestVLESSRealityTemplateInstallsSingBoxAndEnablesSystemd(t *testing.T) {
	template, ok := BuiltinTemplates().Get("singbox-vless-reality")
	if !ok {
		t.Fatal("singbox-vless-reality template is not registered")
	}
	content := string(template.Content)
	for _, required := range []string{
		`SBM_PROGRESS {"step":"preflight","status":"running"`,
		"target proxy port is already listening",
		`SBM_PROGRESS {"step":"install_singbox","status":"running"`,
		"expected_sha256",
		"sing-box archive checksum mismatch",
		"sing-box check -c",
		"systemctl enable sing-box",
		"systemctl restart sing-box",
		"systemctl start sing-box",
		`SBM_PROGRESS {"step":"verify_service","status":"running"`,
		`systemctl is-active --quiet sing-box`,
		"SBM_NODE_BEGIN",
		`"tls":{"enabled":true`,
		`"utls":{"enabled":true,"fingerprint":"chrome"}`,
		"SBM_RESULT {\"status\":\"success\"",
	} {
		if !strings.Contains(content, required) {
			t.Fatalf("template does not contain %q", required)
		}
	}
}

func TestSecurityBasicTemplateIsReadOnlyByDefault(t *testing.T) {
	template, ok := BuiltinTemplates().Get("security-basic")
	if !ok {
		t.Fatal("security-basic template is not registered")
	}
	content := string(template.Content)
	for _, forbidden := range []string{
		"sshd_config",
		"PermitRootLogin",
		"PasswordAuthentication",
		"systemctl restart ssh",
		"systemctl restart sshd",
		"ufw allow",
		"firewall-cmd --add",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("security-basic contains lockout-prone operation %q", forbidden)
		}
	}
	for _, required := range []string{
		`SBM_SECURITY_INSTALL_BASE_PACKAGES`,
		`SBM_SECURITY_FIREWALL_MODE only supports inspect_only`,
		`SBM_PROGRESS {"step":"security_inspect","status":"success"`,
		`SBM_PROGRESS {"step":"base_packages","status":"skipped"`,
		`SBM_PROGRESS {"step":"firewall_inspect","status":"success"`,
		`"ssh_policy_changed":false`,
		`"service_restarted":false`,
	} {
		if !strings.Contains(content, required) {
			t.Fatalf("security-basic missing %q", required)
		}
	}
}

func TestSecurityBasicTemplateEmitsResultMarkerWhenRunLocally(t *testing.T) {
	template, ok := BuiltinTemplates().Get("security-basic")
	if !ok {
		t.Fatal("security-basic template is not registered")
	}
	path := filepath.Join(t.TempDir(), "security-basic.sh")
	if err := os.WriteFile(path, []byte(template.Content), 0o755); err != nil {
		t.Fatalf("write security script: %v", err)
	}

	output, err := RunLocalShell(path, map[string]string{
		"SBM_SECURITY_INSTALL_BASE_PACKAGES": "false",
		"SBM_SECURITY_FIREWALL_MODE":         "inspect_only",
	})
	if err != nil {
		t.Fatalf("RunLocalShell() error = %v, output = %s", err, output)
	}
	parsed, err := ParseScriptOutput(output)
	if err != nil {
		t.Fatalf("ParseScriptOutput() error = %v, output = %s", err, output)
	}
	if parsed.Result["status"] != "success" || parsed.Result["base_packages_installed"] != false || parsed.Result["ssh_policy_changed"] != false {
		t.Fatalf("unexpected security-basic result: %#v", parsed.Result)
	}
	if parsed.Progress[0]["step"] != "security_inspect" {
		t.Fatalf("security-basic progress markers missing: %#v", parsed.Progress)
	}
}

func TestVLESSRealityTemplateRejectsInvalidProxyPortBeforeInstall(t *testing.T) {
	template, ok := BuiltinTemplates().Get("singbox-vless-reality")
	if !ok {
		t.Fatal("singbox-vless-reality template is not registered")
	}
	path := filepath.Join(t.TempDir(), "singbox-vless-reality.sh")
	if err := os.WriteFile(path, []byte(template.Content), 0o755); err != nil {
		t.Fatalf("write vless script: %v", err)
	}

	output, err := RunLocalShell(path, map[string]string{
		"SBM_NODE_SERVER":         "vpn.example.com",
		"SBM_PROXY_PORT":          "not-a-port",
		"SBM_UUID":                "11111111-1111-4111-8111-111111111111",
		"SBM_REALITY_PRIVATE_KEY": "private",
		"SBM_REALITY_PUBLIC_KEY":  "public",
	})
	if err == nil {
		t.Fatalf("expected invalid proxy port to fail, output = %s", output)
	}
	parsed, parseErr := ParseScriptOutput(output)
	if parseErr != nil {
		t.Fatalf("ParseScriptOutput() error = %v, output = %s", parseErr, output)
	}
	if parsed.Result["status"] != "failed" || parsed.Result["message"] != "SBM_PROXY_PORT must be in 1-65535" {
		t.Fatalf("unexpected invalid-port result: %#v", parsed.Result)
	}
	if strings.Contains(output, "install_singbox") {
		t.Fatalf("invalid port should fail before install progress, output = %s", output)
	}
}

func TestVLESSRealityTemplateSupportsPasswordlessSudo(t *testing.T) {
	template, ok := BuiltinTemplates().Get("singbox-vless-reality")
	if !ok {
		t.Fatal("singbox-vless-reality template is not registered")
	}
	tempDir := t.TempDir()
	fakeBin := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatalf("mkdir fake bin: %v", err)
	}
	stateDir := filepath.Join(tempDir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	logPath := filepath.Join(tempDir, "sudo.log")
	testRoot := filepath.Join(tempDir, "root")
	if err := os.MkdirAll(testRoot, 0o755); err != nil {
		t.Fatalf("mkdir test root: %v", err)
	}

	writeFakeCommand(t, fakeBin, "id", `#!/usr/bin/env bash
if [ "${1:-}" = "-u" ]; then
  printf '1000\n'
  exit 0
fi
/usr/bin/id "$@"
`)
	writeFakeCommand(t, fakeBin, "uname", `#!/usr/bin/env bash
case "${1:-}" in
  -s) printf 'Linux\n' ;;
  -m) printf 'x86_64\n' ;;
  -r) printf '6.8.0\n' ;;
  *) /usr/bin/uname "$@" ;;
esac
`)
	writeFakeCommand(t, fakeBin, "sudo", `#!/usr/bin/env bash
printf 'sudo %s\n' "$*" >>"$SBM_TEST_SUDO_LOG"
if [ "${1:-}" = "-n" ]; then
  shift
fi
if [ "${1:-}" = "true" ]; then
  exit 0
fi
if [ "${1:-}" = "/usr/local/bin/sing-box" ]; then
  shift
  exec sing-box "$@"
fi
exec "$@"
`)
	writeFakeCommand(t, fakeBin, "systemctl", `#!/usr/bin/env bash
printf 'systemctl %s\n' "$*" >>"$SBM_TEST_SUDO_LOG"
case "$*" in
  "daemon-reload") exit 0 ;;
  "enable sing-box") exit 0 ;;
  "start sing-box") touch "$SBM_TEST_STATE_DIR/service-active"; exit 0 ;;
  "restart sing-box") touch "$SBM_TEST_STATE_DIR/service-active"; touch "$SBM_TEST_STATE_DIR/service-restarted"; exit 0 ;;
  "is-active --quiet sing-box") test -f "$SBM_TEST_STATE_DIR/service-active"; exit $? ;;
esac
exit 0
`)
	writeFakeCommand(t, fakeBin, "ss", `#!/usr/bin/env bash
if [ -f "$SBM_TEST_STATE_DIR/service-active" ]; then
  printf 'LISTEN 0 4096 *:443 *:*\n'
fi
`)
	writeFakeCommand(t, fakeBin, "tar", `#!/usr/bin/env bash
dest=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-C" ]; then
    shift
    dest="$1"
  fi
  shift || true
done
mkdir -p "$dest/sing-box-test"
cat >"$dest/sing-box-test/sing-box" <<'SH'
#!/usr/bin/env bash
exit 0
SH
chmod 0755 "$dest/sing-box-test/sing-box"
`)
	writeFakeCommand(t, fakeBin, "install", `#!/usr/bin/env bash
args=("$@")
dest="${args[${#args[@]}-1]}"
if [[ " $* " == *" -d "* ]]; then
  if [[ "$dest" = /* ]]; then
    dest="$SBM_TEST_ROOT$dest"
  fi
  mkdir -p "$dest"
  exit 0
fi
src="${args[${#args[@]}-2]}"
if [[ "$dest" = /* ]]; then
  dest="$SBM_TEST_ROOT$dest"
fi
mkdir -p "$(dirname "$dest")"
cp "$src" "$dest"
`)
	writeFakeCommand(t, fakeBin, "sing-box", `#!/usr/bin/env bash
if [ "${1:-}" = "check" ]; then
  exit 0
fi
exit 0
`)

	archivePath := filepath.Join(tempDir, "sing-box.tar.gz")
	archiveContent := []byte("fake archive")
	if err := os.WriteFile(archivePath, archiveContent, 0o644); err != nil {
		t.Fatalf("write fake archive: %v", err)
	}
	checksum := fmt.Sprintf("%x", sha256.Sum256(archiveContent))
	templatePath := filepath.Join(tempDir, "singbox-vless-reality.sh")
	if err := os.WriteFile(templatePath, []byte(template.Content), 0o755); err != nil {
		t.Fatalf("write template: %v", err)
	}
	wrapperPath := filepath.Join(tempDir, "run-template.sh")
	if err := os.WriteFile(wrapperPath, []byte(`#!/usr/bin/env bash
set -euo pipefail
export PATH="$SBM_TEST_FAKE_BIN:$PATH"
exec bash "$SBM_TEST_TEMPLATE"
`), 0o755); err != nil {
		t.Fatalf("write wrapper: %v", err)
	}

	output, err := RunLocalShell(wrapperPath, map[string]string{
		"SBM_TEST_FAKE_BIN":        fakeBin,
		"SBM_TEST_TEMPLATE":        templatePath,
		"SBM_TEST_SUDO_LOG":        logPath,
		"SBM_TEST_STATE_DIR":       stateDir,
		"SBM_TEST_ROOT":            testRoot,
		"SBM_NODE_SERVER":          "vpn.example.com",
		"SBM_PROXY_PORT":           "443",
		"SBM_UUID":                 "11111111-1111-4111-8111-111111111111",
		"SBM_REALITY_PRIVATE_KEY":  "private",
		"SBM_REALITY_PUBLIC_KEY":   "public",
		"SBM_SINGBOX_ARCHIVE":      archivePath,
		"SBM_SINGBOX_SHA256_AMD64": checksum,
		"SBM_SINGBOX_SHA256_ARM64": "unused",
	})
	if err != nil {
		t.Fatalf("RunLocalShell() error = %v, output = %s", err, output)
	}
	parsed, err := ParseScriptOutput(output)
	if err != nil {
		t.Fatalf("ParseScriptOutput() error = %v, output = %s", err, output)
	}
	if parsed.Result["status"] != "success" || parsed.Result["service_active"] != true || parsed.GeneratedNode["server"] != "vpn.example.com" {
		t.Fatalf("unexpected sudo template result: %#v node=%#v output=%s", parsed.Result, parsed.GeneratedNode, output)
	}
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read sudo log: %v", err)
	}
	log := string(logBytes)
	for _, required := range []string{
		"sudo -n install -m 0755",
		"sudo -n install -d -m 0755 /etc/sing-box",
		"sudo -n systemctl enable sing-box",
		"sudo -n systemctl start sing-box",
		"sudo -n systemctl is-active --quiet sing-box",
	} {
		if !strings.Contains(log, required) {
			t.Fatalf("sudo log missing %q:\n%s", required, log)
		}
	}
}

func TestVLESSRealityServerConfigPassesSingBoxCheckWhenAvailable(t *testing.T) {
	singBoxPath := os.Getenv("SBM_SING_BOX_CHECK_BIN")
	if singBoxPath == "" {
		t.Skip("set SBM_SING_BOX_CHECK_BIN to run sing-box config validation")
	}
	config := `{
  "log": {"level": "info", "timestamp": true},
  "inbounds": [
    {
      "type": "vless",
      "tag": "vless-reality-in",
      "listen": "::",
      "listen_port": 443,
      "users": [
        {"uuid": "11111111-1111-4111-8111-111111111111", "flow": "xtls-rprx-vision"}
      ],
      "tls": {
        "enabled": true,
        "server_name": "www.microsoft.com",
        "reality": {
          "enabled": true,
          "handshake": {"server": "www.microsoft.com", "server_port": 443},
          "private_key": "WOmLYwv0QThoLxR7OSXyxwSZ9Uos4I7wL_YaJeDl_lU",
          "short_id": ["0123456789abcdef"]
        }
      }
    }
  ],
  "outbounds": [
    {"type": "direct", "tag": "direct"}
  ]
}`
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("write vless config: %v", err)
	}
	output, err := exec.Command(singBoxPath, "check", "-c", configPath).CombinedOutput()
	if err != nil {
		t.Fatalf("sing-box check failed: %v\n%s\nconfig:\n%s", err, output, config)
	}
}

func TestReadableTemplateCopiesMatchEmbeddedTemplates(t *testing.T) {
	registry := BuiltinTemplates()
	for _, name := range []string{"probe-system", "singbox-vless-reality"} {
		template, ok := registry.Get(name)
		if !ok {
			t.Fatalf("%s template is not registered", name)
		}
		copyPath := filepath.Join("..", "..", "scripts", "templates", name+".sh")
		copyContent, err := os.ReadFile(copyPath)
		if err != nil {
			t.Fatalf("read %s: %v", copyPath, err)
		}
		if string(copyContent) != string(template.Content) {
			t.Fatalf("%s does not match embedded template", copyPath)
		}
	}
}

func writeFakeCommand(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake command %s: %v", name, err)
	}
}
