package deploy

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeploymentAssetsPackageContainsStandaloneScripts(t *testing.T) {
	root := filepath.Join("..", "..")
	archivePath := filepath.Join(t.TempDir(), "sbm-deployment-assets.tar.gz")

	cmd := exec.Command("bash", "scripts/package-deployment-assets.sh", "--output", archivePath)
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("package deployment assets error = %v, output = %s", err, output)
	}

	listCmd := exec.Command("tar", "-tzf", archivePath)
	output, err := listCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("list deployment assets archive error = %v, output = %s", err, output)
	}
	entries := string(output)
	for _, want := range []string{
		"scripts/verify-deployment-vps-matrix.sh",
		"scripts/verify-deployment-vps.sh",
		"scripts/templates/probe-system.sh",
		"scripts/templates/security-basic.sh",
		"scripts/templates/singbox-vless-reality.sh",
		"scripts/runtime-cache/download-singbox.sh",
		"docs/proxy-node-deployment-verification.md",
	} {
		if !strings.Contains(entries, want) {
			t.Fatalf("deployment assets archive missing %s; entries:\n%s", want, entries)
		}
	}
	assertDeploymentAssetScriptModes(t, archivePath, []string{
		"scripts/verify-deployment-vps-matrix.sh",
		"scripts/verify-deployment-vps.sh",
		"scripts/templates/probe-system.sh",
		"scripts/templates/security-basic.sh",
		"scripts/templates/singbox-vless-reality.sh",
		"scripts/runtime-cache/download-singbox.sh",
	})

	workflow, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}
	if !strings.Contains(string(workflow), "scripts/package-deployment-assets.sh") {
		t.Fatalf("release workflow does not package deployment assets")
	}
}

func TestVerifyDeploymentVPSMatrixPlanDocumentsCasesWithoutSecrets(t *testing.T) {
	root := filepath.Join("..", "..")
	cmd := exec.Command("bash", "scripts/verify-deployment-vps-matrix.sh", "--plan", "--skip-unavailable")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"SBM_PASSWORD=matrix-password-secret",
		"SBM_SSH_HOST=203.0.113.10",
		"SBM_SSH_PASSWORD=ssh-password-secret",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("verify deployment vps matrix plan error = %v, output = %s", err, output)
	}
	plan := string(output)
	for _, want := range []string{
		"PLAN direct",
		"PLAN managed",
		"SKIP custom_socks5",
		"SKIP custom_http_connect",
		"PLAN import_export",
		"PLAN runtime_cache",
		"PLAN cancel",
		"PLAN preserve_remote_run_dir",
		"SBM_CONNECTION_MODE=managed_proxy",
		"SBM_CHECK_EXPORTED_CONFIG=true",
		"SBM_CANCEL_AFTER_CREATE=true",
		"SBM_DEBUG_PRESERVE_REMOTE_RUN_DIR=true",
	} {
		if !strings.Contains(plan, want) {
			t.Fatalf("matrix plan missing %s; plan:\n%s", want, plan)
		}
	}
	for _, leaked := range []string{"matrix-password-secret", "ssh-password-secret"} {
		if strings.Contains(plan, leaked) {
			t.Fatalf("matrix plan leaked secret %q:\n%s", leaked, plan)
		}
	}
}

func TestVerifyDeploymentVPSMatrixPlanSupportsCaseSpecificProxyEnv(t *testing.T) {
	root := filepath.Join("..", "..")
	cmd := exec.Command("bash", "scripts/verify-deployment-vps-matrix.sh", "--plan", "--cases", "custom_socks5,custom_http_connect")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"SBM_SOCKS5_PROXY_HOST=127.0.0.1",
		"SBM_SOCKS5_PROXY_PORT=1080",
		"SBM_HTTP_CONNECT_PROXY_HOST=127.0.0.1",
		"SBM_HTTP_CONNECT_PROXY_PORT=18080",
		"SBM_HTTP_CONNECT_PROXY_USERNAME=ops",
		"SBM_HTTP_CONNECT_PROXY_PASSWORD=http-proxy-secret",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("verify deployment vps matrix custom proxy plan error = %v, output = %s", err, output)
	}
	plan := string(output)
	for _, want := range []string{
		"PLAN custom_socks5",
		"SBM_PROXY_TYPE=socks5",
		"SBM_PROXY_PORT_LOCAL=1080",
		"PLAN custom_http_connect",
		"SBM_PROXY_TYPE=http_connect",
		"SBM_PROXY_PORT_LOCAL=18080",
	} {
		if !strings.Contains(plan, want) {
			t.Fatalf("matrix proxy plan missing %s; plan:\n%s", want, plan)
		}
	}
	if strings.Contains(plan, "http-proxy-secret") {
		t.Fatalf("matrix proxy plan leaked proxy password:\n%s", plan)
	}
}

func TestVerifyDeploymentVPSMatrixRequiresCustomProxyEnvUnlessSkipping(t *testing.T) {
	root := filepath.Join("..", "..")
	cmd := exec.Command("bash", "scripts/verify-deployment-vps-matrix.sh", "--plan", "--cases", "custom_socks5")
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("matrix plan without custom proxy env unexpectedly succeeded: %s", output)
	}
	if !strings.Contains(string(output), "custom_socks5 requires proxy host and port") {
		t.Fatalf("matrix plan did not explain missing custom proxy env: %s", output)
	}
	if strings.Contains(string(output), "Proxy Node Deployment VPS verification matrix finished") {
		t.Fatalf("matrix plan reported finish after missing custom proxy env: %s", output)
	}
}

func TestVerifyDeploymentVPSMatrixRunPassesCaseEnvironment(t *testing.T) {
	root := filepath.Join("..", "..")
	tmpDir := t.TempDir()
	capturePath := filepath.Join(tmpDir, "matrix-env.log")
	fakeVerifierPath := filepath.Join(tmpDir, "fake-verifier.sh")
	fakeVerifier := `#!/usr/bin/env bash
set -euo pipefail
printf 'mode=%s proxy_type=%s proxy_host=%s proxy_port=%s import=%s export=%s runtime=%s cancel=%s preserve=%s\n' \
  "${SBM_CONNECTION_MODE:-}" \
  "${SBM_PROXY_TYPE:-}" \
  "${SBM_PROXY_HOST:-}" \
  "${SBM_PROXY_PORT_LOCAL:-}" \
  "${SBM_IMPORT_NODE:-}" \
  "${SBM_CHECK_EXPORTED_CONFIG:-}" \
  "${SBM_RUNTIME_SOURCE:-}" \
  "${SBM_CANCEL_AFTER_CREATE:-}" \
  "${SBM_DEBUG_PRESERVE_REMOTE_RUN_DIR:-}" >> "$SBM_MATRIX_CAPTURE_FILE"
`
	if err := os.WriteFile(fakeVerifierPath, []byte(fakeVerifier), 0o700); err != nil {
		t.Fatalf("write fake verifier: %v", err)
	}

	cmd := exec.Command("bash", "scripts/verify-deployment-vps-matrix.sh", "--run")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"SBM_VERIFY_SCRIPT="+fakeVerifierPath,
		"SBM_MATRIX_CAPTURE_FILE="+capturePath,
		"SBM_PASSWORD=matrix-password-secret",
		"SBM_SSH_HOST=203.0.113.10",
		"SBM_SSH_PASSWORD=ssh-password-secret",
		"SBM_SOCKS5_PROXY_HOST=127.0.0.1",
		"SBM_SOCKS5_PROXY_PORT=1080",
		"SBM_HTTP_CONNECT_PROXY_HOST=127.0.0.1",
		"SBM_HTTP_CONNECT_PROXY_PORT=18080",
		"SBM_HTTP_CONNECT_PROXY_USERNAME=ops",
		"SBM_HTTP_CONNECT_PROXY_PASSWORD=http-proxy-secret",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("verify deployment vps matrix run error = %v, output = %s", err, output)
	}
	logData, err := os.ReadFile(capturePath)
	if err != nil {
		t.Fatalf("read matrix capture: %v", err)
	}
	log := string(logData)
	for _, want := range []string{
		"mode=direct proxy_type= proxy_host= proxy_port= import=false export=false runtime=remote cancel=false preserve=false",
		"mode=managed_proxy proxy_type= proxy_host= proxy_port= import=false export=false runtime=remote cancel=false preserve=false",
		"mode=custom_proxy proxy_type=socks5 proxy_host=127.0.0.1 proxy_port=1080 import=false export=false runtime=remote cancel=false preserve=false",
		"mode=custom_proxy proxy_type=http_connect proxy_host=127.0.0.1 proxy_port=18080 import=false export=false runtime=remote cancel=false preserve=false",
		"mode=direct proxy_type= proxy_host= proxy_port= import=true export=true runtime=remote cancel=false preserve=false",
		"mode=direct proxy_type= proxy_host= proxy_port= import=false export=false runtime=cache cancel=false preserve=false",
		"mode=direct proxy_type= proxy_host= proxy_port= import=false export=false runtime=remote cancel=true preserve=false",
		"mode=direct proxy_type= proxy_host= proxy_port= import=false export=false runtime=remote cancel=false preserve=true",
	} {
		if !strings.Contains(log, want) {
			t.Fatalf("matrix run capture missing %s; capture:\n%s\noutput:\n%s", want, log, output)
		}
	}
	for _, leaked := range []string{"matrix-password-secret", "ssh-password-secret", "http-proxy-secret"} {
		if strings.Contains(string(output), leaked) {
			t.Fatalf("matrix run output leaked secret %q:\n%s", leaked, output)
		}
	}
}

func TestVerifyDeploymentVPSMatrixRunPreparesRuntimeCache(t *testing.T) {
	root := filepath.Join("..", "..")
	tmpDir := t.TempDir()
	cacheCapturePath := filepath.Join(tmpDir, "runtime-cache.log")
	verifyCapturePath := filepath.Join(tmpDir, "verify.log")
	fakeCachePath := filepath.Join(tmpDir, "fake-runtime-cache.sh")
	fakeVerifierPath := filepath.Join(tmpDir, "fake-verifier.sh")

	fakeCache := `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$SBM_MATRIX_CACHE_CAPTURE_FILE"
`
	if err := os.WriteFile(fakeCachePath, []byte(fakeCache), 0o700); err != nil {
		t.Fatalf("write fake runtime cache helper: %v", err)
	}
	fakeVerifier := `#!/usr/bin/env bash
set -euo pipefail
printf 'runtime=%s\n' "${SBM_RUNTIME_SOURCE:-}" >> "$SBM_MATRIX_VERIFY_CAPTURE_FILE"
`
	if err := os.WriteFile(fakeVerifierPath, []byte(fakeVerifier), 0o700); err != nil {
		t.Fatalf("write fake verifier: %v", err)
	}

	cmd := exec.Command("bash", "scripts/verify-deployment-vps-matrix.sh", "--run", "--cases", "runtime_cache", "--prepare-cache")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"SBM_VERIFY_SCRIPT="+fakeVerifierPath,
		"SBM_RUNTIME_CACHE_SCRIPT="+fakeCachePath,
		"SBM_MATRIX_CACHE_CAPTURE_FILE="+cacheCapturePath,
		"SBM_MATRIX_VERIFY_CAPTURE_FILE="+verifyCapturePath,
		"SBM_RUNTIME_VERSION=1.13.13",
		"SBM_DATA_DIR=/tmp/sbm-matrix-data",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("verify deployment vps matrix prepare-cache error = %v, output = %s", err, output)
	}
	cacheLog, err := os.ReadFile(cacheCapturePath)
	if err != nil {
		t.Fatalf("read runtime cache capture: %v", err)
	}
	if got, want := strings.TrimSpace(string(cacheLog)), "--version 1.13.13 --data-dir /tmp/sbm-matrix-data"; got != want {
		t.Fatalf("runtime cache helper args = %q, want %q; output:\n%s", got, want, output)
	}
	verifyLog, err := os.ReadFile(verifyCapturePath)
	if err != nil {
		t.Fatalf("read verifier capture: %v", err)
	}
	if strings.TrimSpace(string(verifyLog)) != "runtime=cache" {
		t.Fatalf("verifier did not receive runtime cache source: %s", verifyLog)
	}
	if !strings.Contains(string(output), "==> PREPARE runtime_cache") || !strings.Contains(string(output), "==> RUN runtime_cache") {
		t.Fatalf("matrix output did not show prepare and run phases:\n%s", output)
	}
}

func TestVerifyDeploymentVPSHelpDocumentsDebugTmpdirFlag(t *testing.T) {
	root := filepath.Join("..", "..")
	cmd := exec.Command("bash", "scripts/verify-deployment-vps.sh", "--help")
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("verify deployment vps help error = %v, output = %s", err, output)
	}
	help := string(output)
	for _, want := range []string{
		"SBM_DEBUG_PRESERVE_VERIFY_TMPDIR",
		"SBM_DEBUG_PRESERVE_REMOTE_RUN_DIR",
		"SBM_CONNECTION_MODE",
		"SBM_SSH_PRIVATE_KEY_FILE",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("verify deployment vps help missing %s; help:\n%s", want, help)
		}
	}
}

func TestVerifyDeploymentVPSMatrixHelpDocumentsRunMode(t *testing.T) {
	root := filepath.Join("..", "..")
	cmd := exec.Command("bash", "scripts/verify-deployment-vps-matrix.sh", "--help")
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("verify deployment vps matrix help error = %v, output = %s", err, output)
	}
	help := string(output)
	for _, want := range []string{
		"--run",
		"custom_http_connect",
		"SBM_MATRIX_SKIP_UNAVAILABLE",
		"SBM_MATRIX_PREPARE_CACHE",
		"SBM_HTTP_CONNECT_PROXY_HOST",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("verify deployment vps matrix help missing %s; help:\n%s", want, help)
		}
	}
}

func TestStandaloneDeploymentTemplatesMatchEmbeddedTemplates(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, name := range []string{"probe-system", "security-basic", "singbox-vless-reality"} {
		embedded, err := os.ReadFile(filepath.Join(root, "internal", "deploy", "templates", name+".sh"))
		if err != nil {
			t.Fatalf("read embedded template %s: %v", name, err)
		}
		standalone, err := os.ReadFile(filepath.Join(root, "scripts", "templates", name+".sh"))
		if err != nil {
			t.Fatalf("read standalone template %s: %v", name, err)
		}
		if string(embedded) != string(standalone) {
			t.Fatalf("standalone deployment template %s differs from embedded template", name)
		}
	}
}

func TestWorkflowsRunGoTestsWithSupportedToolchain(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, workflowName := range []string{"ci.yml", "release.yml"} {
		workflow, err := os.ReadFile(filepath.Join(root, ".github", "workflows", workflowName))
		if err != nil {
			t.Fatalf("read workflow %s: %v", workflowName, err)
		}
		text := string(workflow)
		if !strings.Contains(text, "go-version: '1.24'") {
			t.Fatalf("%s does not use the documented Go 1.24 toolchain", workflowName)
		}
	}

	ci, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("read ci workflow: %v", err)
	}
	if !strings.Contains(string(ci), "go test ./...") {
		t.Fatalf("CI workflow does not run Go tests")
	}
}

func TestVerifyDeploymentVPSScriptAutoSelectsManagedCandidate(t *testing.T) {
	const selectedCandidateID = "inbound:auto"

	result := runVerifyDeploymentVPSScriptWithManagedCandidates(t, "", `[
		{"id":"node:down","display_name":"Down","available":false,"requires_temporary_entrypoint":true},
		{"id":"`+selectedCandidateID+`","display_name":"Auto tunnel","available":true,"local_endpoint":"127.0.0.1:2081"}
	]`)
	if result.connectionTestCandidate != selectedCandidateID {
		t.Fatalf("connection-test managed candidate = %q, want %q; output = %s", result.connectionTestCandidate, selectedCandidateID, result.output)
	}
	if result.runCandidate != selectedCandidateID {
		t.Fatalf("run managed candidate = %q, want %q; output = %s", result.runCandidate, selectedCandidateID, result.output)
	}
	if !strings.Contains(result.output, "managed candidate selected: route/local-endpoint") {
		t.Fatalf("script output did not report selected managed candidate route: %s", result.output)
	}
	if strings.Contains(result.output, "Auto tunnel") || strings.Contains(result.output, selectedCandidateID) {
		t.Fatalf("script output leaked selected managed candidate identity: %s", result.output)
	}
}

func TestVerifyDeploymentVPSScriptUsesRequestedManagedCandidate(t *testing.T) {
	const requestedCandidateID = "node:requested"

	result := runVerifyDeploymentVPSScriptWithManagedCandidates(t, requestedCandidateID, `[
		{"id":"inbound:first","display_name":"First tunnel","available":true,"local_endpoint":"127.0.0.1:2081"},
		{"id":"`+requestedCandidateID+`","display_name":"Requested node","available":true,"requires_temporary_entrypoint":true}
	]`)
	if result.connectionTestCandidate != requestedCandidateID {
		t.Fatalf("connection-test managed candidate = %q, want %q; output = %s", result.connectionTestCandidate, requestedCandidateID, result.output)
	}
	if result.runCandidate != requestedCandidateID {
		t.Fatalf("run managed candidate = %q, want %q; output = %s", result.runCandidate, requestedCandidateID, result.output)
	}
	if !strings.Contains(result.output, "managed candidate selected: route/temporary-entrypoint") {
		t.Fatalf("script output did not report requested managed candidate route: %s", result.output)
	}
	if strings.Contains(result.output, "Requested node") || strings.Contains(result.output, requestedCandidateID) {
		t.Fatalf("script output leaked requested managed candidate identity: %s", result.output)
	}
}

func TestVerifyDeploymentVPSScriptRejectsUnknownRequestedManagedCandidate(t *testing.T) {
	result := runVerifyDeploymentVPSScriptWithManagedCandidatesAllowError(t, "node:missing", `[
		{"id":"inbound:first","display_name":"First tunnel","available":true,"local_endpoint":"127.0.0.1:2081"}
	]`)
	if result.err == nil {
		t.Fatalf("verify deployment vps script unexpectedly succeeded: %s", result.output)
	}
	if !strings.Contains(result.output, "managed candidate not found") {
		t.Fatalf("script output did not explain missing requested candidate: %s", result.output)
	}
	if strings.Contains(result.output, "node:missing") {
		t.Fatalf("script output leaked missing requested candidate id: %s", result.output)
	}
	if result.connectionTestCandidate != "" || result.runCandidate != "" {
		t.Fatalf("script should fail before connection-test/run, got connection-test=%q run=%q", result.connectionTestCandidate, result.runCandidate)
	}
}

func TestVerifyDeploymentVPSScriptRejectsUnavailableRequestedManagedCandidate(t *testing.T) {
	result := runVerifyDeploymentVPSScriptWithManagedCandidatesAllowError(t, "node:down", `[
		{"id":"node:down","display_name":"Down","available":false,"unavailable_reason":"sing-box 服务未运行","requires_temporary_entrypoint":true}
	]`)
	if result.err == nil {
		t.Fatalf("verify deployment vps script unexpectedly succeeded: %s", result.output)
	}
	if !strings.Contains(result.output, "managed candidate unavailable: sing-box 服务未运行") {
		t.Fatalf("script output did not explain unavailable requested candidate: %s", result.output)
	}
	if result.connectionTestCandidate != "" || result.runCandidate != "" {
		t.Fatalf("script should fail before connection-test/run, got connection-test=%q run=%q", result.connectionTestCandidate, result.runCandidate)
	}
}

func TestVerifyDeploymentVPSScriptDoesNotLeakUnavailableManagedCandidateIdentity(t *testing.T) {
	result := runVerifyDeploymentVPSScriptWithManagedCandidatesAllowError(t, "node:private-tag", `[
		{"id":"node:private-tag","display_name":"Private Node","available":false,"requires_temporary_entrypoint":true}
	]`)
	if result.err == nil {
		t.Fatalf("verify deployment vps script unexpectedly succeeded: %s", result.output)
	}
	if !strings.Contains(result.output, "managed candidate unavailable") {
		t.Fatalf("script output did not explain unavailable requested candidate: %s", result.output)
	}
	if strings.Contains(result.output, "node:private-tag") || strings.Contains(result.output, "Private Node") {
		t.Fatalf("script output leaked unavailable managed candidate identity: %s", result.output)
	}
	if result.connectionTestCandidate != "" || result.runCandidate != "" {
		t.Fatalf("script should fail before connection-test/run, got connection-test=%q run=%q", result.connectionTestCandidate, result.runCandidate)
	}
}

func TestVerifyDeploymentVPSScriptSendsCustomHTTPConnectProxy(t *testing.T) {
	result := runVerifyDeploymentVPSScriptWithExtraEnv(t, []string{
		"SBM_CONNECTION_MODE=custom_proxy",
		"SBM_PROXY_TYPE=http_connect",
		"SBM_PROXY_HOST=127.0.0.1",
		"SBM_PROXY_PORT_LOCAL=18080",
		"SBM_PROXY_USERNAME=ops",
		"SBM_PROXY_PASSWORD=proxy-secret",
	})

	for label, connection := range map[string]deploymentVerifierConnectionPayload{
		"connection-test": result.connectionTestConnection,
		"run":             result.runConnection,
	} {
		if connection.Mode != "custom_proxy" ||
			connection.ProxyType != "http_connect" ||
			connection.ProxyHost != "127.0.0.1" ||
			connection.ProxyPort != 18080 ||
			connection.ProxyUsername != "ops" ||
			connection.ProxyPassword != "proxy-secret" {
			t.Fatalf("%s custom proxy payload = %#v", label, connection)
		}
	}
}

func TestVerifyDeploymentVPSScriptSendsDefaultCustomSocks5Proxy(t *testing.T) {
	result := runVerifyDeploymentVPSScriptWithExtraEnv(t, []string{
		"SBM_CONNECTION_MODE=custom_proxy",
		"SBM_PROXY_HOST=127.0.0.1",
		"SBM_PROXY_PORT_LOCAL=1080",
		"SBM_PROXY_USERNAME=ops",
		"SBM_PROXY_PASSWORD=proxy-secret",
	})

	for label, connection := range map[string]deploymentVerifierConnectionPayload{
		"connection-test": result.connectionTestConnection,
		"run":             result.runConnection,
	} {
		if connection.Mode != "custom_proxy" ||
			connection.ProxyType != "socks5" ||
			connection.ProxyHost != "127.0.0.1" ||
			connection.ProxyPort != 1080 ||
			connection.ProxyUsername != "ops" ||
			connection.ProxyPassword != "proxy-secret" {
			t.Fatalf("%s custom proxy payload = %#v", label, connection)
		}
	}
}

func TestVerifyDeploymentVPSScriptSendsPrivateKeyFileAuth(t *testing.T) {
	const privateKey = "-----BEGIN OPENSSH PRIVATE KEY-----\nfile-private-key-secret\n-----END OPENSSH PRIVATE KEY-----"
	result := runVerifyDeploymentVPSScriptWithExtraEnv(t, []string{
		"SBM_SSH_AUTH_METHOD=private_key",
		"SBM_SSH_PASSWORD=",
		"SBM_TEST_PRIVATE_KEY_FILE_CONTENT=" + privateKey,
		"SBM_SSH_PRIVATE_KEY_PASSPHRASE=key-passphrase",
	})
	if result.err != nil {
		t.Fatalf("verify deployment vps script error = %v, output = %s", result.err, result.output)
	}

	for label, ssh := range map[string]deploymentVerifierSSHPayload{
		"connection-test": result.connectionTestSSH,
		"run":             result.runSSH,
	} {
		if ssh.AuthMethod != "private_key" ||
			ssh.PrivateKey != privateKey ||
			ssh.PrivateKeyPassphrase != "key-passphrase" ||
			ssh.Password != "" {
			t.Fatalf("%s SSH private key file payload = %#v", label, ssh)
		}
	}
}

func TestVerifyDeploymentVPSScriptSendsInlinePrivateKeyAuth(t *testing.T) {
	const privateKey = "-----BEGIN OPENSSH PRIVATE KEY-----\ninline-private-key-secret\n-----END OPENSSH PRIVATE KEY-----"
	result := runVerifyDeploymentVPSScriptWithExtraEnv(t, []string{
		"SBM_SSH_AUTH_METHOD=private_key",
		"SBM_SSH_PASSWORD=",
		"SBM_SSH_PRIVATE_KEY=" + privateKey,
		"SBM_SSH_PRIVATE_KEY_PASSPHRASE=inline-key-passphrase",
	})
	if result.err != nil {
		t.Fatalf("verify deployment vps script error = %v, output = %s", result.err, result.output)
	}

	for label, ssh := range map[string]deploymentVerifierSSHPayload{
		"connection-test": result.connectionTestSSH,
		"run":             result.runSSH,
	} {
		if ssh.AuthMethod != "private_key" ||
			ssh.PrivateKey != privateKey ||
			ssh.PrivateKeyPassphrase != "inline-key-passphrase" ||
			ssh.Password != "" {
			t.Fatalf("%s SSH inline private key payload = %#v", label, ssh)
		}
	}
}

func TestVerifyDeploymentVPSScriptImportsNodeAndChecksExportedConfig(t *testing.T) {
	root := filepath.Join("..", "..")
	tempDir := t.TempDir()
	argsPath := filepath.Join(tempDir, "sing-box-args.txt")
	fakeSingBoxPath := filepath.Join(tempDir, "sing-box")
	if err := os.WriteFile(fakeSingBoxPath, []byte("#!/usr/bin/env bash\nprintf '%s\\n' \"$*\" > \"$SBM_TEST_SING_BOX_ARGS\"\n"), 0o755); err != nil {
		t.Fatalf("write fake sing-box: %v", err)
	}

	imported := false
	exportedConfig := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/auth/me":
			_, _ = w.Write([]byte(`{"data":{"bootstrapped":true,"authenticated":true}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/templates":
			_, _ = w.Write([]byte(`{"data":[{"name":"singbox-vless-reality","runtime":{"name":"sing-box","version":"1.13.13"}}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/connection-candidates":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/deployments/connection-test":
			_, _ = w.Write([]byte(`{"data":{"ok":true,"remote":{"uname_s":"Linux","uname_m":"x86_64"}}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/deployments/runs":
			_, _ = w.Write([]byte(`{"data":{"id":"run-import"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/runs":
			_, _ = w.Write([]byte(`{"data":[{"id":"run-import","status":"success","generated_node":{"type":"vless","tag":"edge-a"},"progress_markers":{"external_reachability":{"status":"success"}},"exit_code":0}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/runs/run-import":
			_, _ = w.Write([]byte(`{"data":{"id":"run-import","status":"success","generated_node":{"type":"vless","tag":"edge-a"},"progress_markers":{"external_reachability":{"status":"success"}},"exit_code":0}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/runs/run-import/generated-node":
			if imported {
				_, _ = w.Write([]byte(`{"data":{"can_import":false,"imported_node_id":42,"default_tag":"edge-a","generated_node":{"type":"vless","tag":"edge-a"}}}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":{"can_import":true,"default_tag":"edge-a","generated_node":{"type":"vless","tag":"edge-a"}}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/deployments/runs/run-import/import-node":
			imported = true
			_, _ = w.Write([]byte(`{"data":{"id":42,"tag":"edge-a"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/manual-nodes":
			_, _ = w.Write([]byte(`{"data":[{"node":{"tag":"edge-a","source":"manual","extra":{"node_origin":"deployed_self_hosted","entry_method":"deployment_import","deployment_run_id":"run-import"}}}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/config/export":
			exportedConfig = true
			_, _ = w.Write([]byte(`{"log":{"level":"info"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cmd := exec.Command("bash", "scripts/verify-deployment-vps.sh")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"SBM_API_BASE="+server.URL,
		"SBM_PASSWORD=manager-password",
		"SBM_SSH_HOST=203.0.113.10",
		"SBM_SSH_PASSWORD=ssh-password",
		"SBM_VERIFY_POLL_SECONDS=0",
		"SBM_IMPORT_NODE=true",
		"SBM_CHECK_EXPORTED_CONFIG=true",
		"SBM_SING_BOX_CHECK_BIN="+fakeSingBoxPath,
		"SBM_TEST_SING_BOX_ARGS="+argsPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("verify deployment vps script error = %v, output = %s", err, output)
	}
	if !imported {
		t.Fatal("script did not import generated node")
	}
	if !exportedConfig {
		t.Fatal("script did not export generated config")
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read fake sing-box args: %v", err)
	}
	if !strings.Contains(string(args), "check -c ") {
		t.Fatalf("sing-box check was not invoked with config path, args = %q, output = %s", args, output)
	}
}

func TestVerifyDeploymentVPSScriptDoesNotEchoImportedManualNodeSecretsOnProvenanceFailure(t *testing.T) {
	result := runVerifyDeploymentVPSScriptWithManualNodesResponse(t, `{"data":[{"node":{"tag":"edge-a","source":"subscription","extra":{"uuid":"imported-node-uuid-secret","node_origin":"deployed_self_hosted","entry_method":"deployment_import","deployment_run_id":"run-import"}}}]}`)
	if result.err == nil {
		t.Fatalf("verify deployment vps script unexpectedly succeeded: %s", result.output)
	}
	if !strings.Contains(result.output, "imported node source is not manual") {
		t.Fatalf("script output did not explain imported node provenance failure: %s", result.output)
	}
	if strings.Contains(result.output, "imported-node-uuid-secret") {
		t.Fatalf("script output echoed imported manual node secret: %s", result.output)
	}
}

func TestVerifyDeploymentVPSScriptRejectsInvalidRuntimeCacheBeforeRun(t *testing.T) {
	result := runVerifyDeploymentVPSScriptWithRuntimeCache(t, `[
		{"runtime_name":"sing-box","version":"1.13.13","os":"linux","arch":"amd64","status":"invalid"}
	]`)
	if result.err == nil {
		t.Fatalf("verify deployment vps script unexpectedly succeeded: %s", result.output)
	}
	if result.runCreated {
		t.Fatalf("script created deployment run despite invalid runtime cache: %s", result.output)
	}
	if !strings.Contains(result.output, "runtime cache for linux/amd64 is invalid") {
		t.Fatalf("script output did not explain invalid runtime cache: %s", result.output)
	}
}

func TestVerifyDeploymentVPSScriptAcceptsValidRuntimeCache(t *testing.T) {
	result := runVerifyDeploymentVPSScriptWithRuntimeCache(t, `[
		{"runtime_name":"sing-box","version":"1.13.13","os":"linux","arch":"amd64","status":"valid"}
	]`)
	if result.err != nil {
		t.Fatalf("verify deployment vps script error = %v, output = %s", result.err, result.output)
	}
	if !result.runCreated {
		t.Fatalf("script did not create deployment run after valid runtime cache: %s", result.output)
	}
	if !strings.Contains(result.output, "runtime cache: linux/amd64 valid") {
		t.Fatalf("script output did not report valid runtime cache: %s", result.output)
	}
}

func TestVerifyDeploymentVPSScriptDoesNotEchoFailedRunBody(t *testing.T) {
	result := runVerifyDeploymentVPSScriptWithRunCurrent(t, `{"id":"run-managed","status":"failed","stdout":"remote-log-secret","progress_markers":{"external_reachability":{"status":"failed"}},"exit_code":1}`, nil)
	if result.err == nil {
		t.Fatalf("verify deployment vps script unexpectedly succeeded: %s", result.output)
	}
	if !strings.Contains(result.output, "deployment run did not succeed: failed") {
		t.Fatalf("script output did not report failed run status: %s", result.output)
	}
	if strings.Contains(result.output, "remote-log-secret") {
		t.Fatalf("script output echoed failed run body: %s", result.output)
	}
}

func TestVerifyDeploymentVPSScriptRejectsRuntimeArchMismatchBeforeRun(t *testing.T) {
	result := runVerifyDeploymentVPSScriptWithExtraEnv(t, []string{"SBM_RUNTIME_ARCH=arm64"})
	if result.err == nil {
		t.Fatalf("verify deployment vps script unexpectedly succeeded: %s", result.output)
	}
	if result.runCreated {
		t.Fatalf("script created deployment run despite runtime arch mismatch: %s", result.output)
	}
	if !strings.Contains(result.output, "SBM_RUNTIME_ARCH=arm64 does not match target architecture amd64") {
		t.Fatalf("script output did not explain arch mismatch: %s", result.output)
	}
}

func TestVerifyDeploymentVPSScriptRejectsUnsupportedTargetOSBeforeRun(t *testing.T) {
	result := runVerifyDeploymentVPSScriptWithRemote(t, `{"uname_s":"Darwin","uname_m":"x86_64"}`, nil)
	if result.err == nil {
		t.Fatalf("verify deployment vps script unexpectedly succeeded: %s", result.output)
	}
	if result.runCreated {
		t.Fatalf("script created deployment run despite unsupported OS: %s", result.output)
	}
	if !strings.Contains(result.output, "target OS is not supported: Darwin") {
		t.Fatalf("script output did not explain unsupported OS: %s", result.output)
	}
}

func TestVerifyDeploymentVPSScriptRequiresPreservedRemoteRunDirMarker(t *testing.T) {
	result := runVerifyDeploymentVPSScriptWithExtraEnv(t, []string{"SBM_DEBUG_PRESERVE_REMOTE_RUN_DIR=true"})
	if result.err == nil {
		t.Fatalf("verify deployment vps script unexpectedly succeeded: %s", result.output)
	}
	if !strings.Contains(result.output, "remote_run_dir marker is not preserved") {
		t.Fatalf("script output did not explain missing preserved remote dir marker: %s", result.output)
	}
}

func TestVerifyDeploymentVPSScriptAcceptsPreservedRemoteRunDirMarker(t *testing.T) {
	result := runVerifyDeploymentVPSScriptWithRunCurrent(t, `{"id":"run-managed","status":"success","generated_node":{"type":"vless","tag":"edge-a"},"progress_markers":{"external_reachability":{"status":"success"},"remote_run_dir":{"status":"preserved","preserved":true,"path":"/tmp/sbm-deploy-test"}},"exit_code":0}`, []string{"SBM_DEBUG_PRESERVE_REMOTE_RUN_DIR=true"})
	if result.err != nil {
		t.Fatalf("verify deployment vps script error = %v, output = %s", result.err, result.output)
	}
	if !strings.Contains(result.output, "remote run dir preserved: /tmp/sbm-deploy-test") {
		t.Fatalf("script output did not report preserved remote dir marker: %s", result.output)
	}
}

func TestVerifyDeploymentVPSScriptRejectsUnredactedGeneratedNodeHistory(t *testing.T) {
	result := runVerifyDeploymentVPSScriptWithRunCurrent(t, `{"id":"run-managed","status":"success","generated_node":{"type":"vless","tag":"edge-a","extra":{"uuid":"generated-secret","tls":{"reality":{"public_key":"generated-public","short_id":"short-secret"}}}},"progress_markers":{"external_reachability":{"status":"success"}},"exit_code":0}`, nil)
	if result.err == nil {
		t.Fatalf("verify deployment vps script unexpectedly succeeded: %s", result.output)
	}
	if !strings.Contains(result.output, "sensitive history fields were not redacted") {
		t.Fatalf("script output did not explain unredacted generated-node history: %s", result.output)
	}
	if result.generatedNodeReviewed {
		t.Fatalf("script should fail before generated-node review when run history leaks secrets: %s", result.output)
	}
}

func TestVerifyDeploymentVPSScriptRejectsUnredactedGeneratedNodeListHistory(t *testing.T) {
	result := runVerifyDeploymentVPSScriptWithCurrentAndList(t,
		`{"id":"run-managed","status":"success","generated_node":{"type":"vless","tag":"edge-a","extra":{"uuid":"[REDACTED]","tls":{"reality":{"public_key":"[REDACTED]","short_id":"[REDACTED]"}}}},"progress_markers":{"external_reachability":{"status":"success"}},"exit_code":0}`,
		`[{"id":"run-managed","status":"success","generated_node":{"type":"vless","tag":"edge-a","extra":{"uuid":"generated-secret","tls":{"reality":{"public_key":"generated-public","short_id":"short-secret"}}}},"progress_markers":{"external_reachability":{"status":"success"}},"exit_code":0}]`,
		nil,
	)
	if result.err == nil {
		t.Fatalf("verify deployment vps script unexpectedly succeeded: %s", result.output)
	}
	if !strings.Contains(result.output, "sensitive history fields were not redacted") {
		t.Fatalf("script output did not explain unredacted generated-node list history: %s", result.output)
	}
	if result.generatedNodeReviewed {
		t.Fatalf("script should fail before generated-node review when run list leaks secrets: %s", result.output)
	}
}

func TestVerifyDeploymentVPSScriptRejectsManagedCandidateIdentityLeak(t *testing.T) {
	const candidateID = "node:sensitive-route"
	result := runVerifyDeploymentVPSScriptWithRemoteCurrentAndList(
		t,
		`[{"id":"`+candidateID+`","display_name":"Sensitive route","available":true,"requires_temporary_entrypoint":true}]`,
		`{"uname_s":"Linux","uname_m":"x86_64"}`,
		`{"id":"run-managed","status":"success","redacted_params":{"managed_candidate_id":"`+candidateID+`"},"generated_node":{"type":"vless","tag":"edge-a"},"progress_markers":{"external_reachability":{"status":"success"}},"exit_code":0}`,
		`[{"id":"run-managed","status":"success","redacted_params":{"managed_candidate_id":"[REDACTED]"},"generated_node":{"type":"vless","tag":"edge-a"},"progress_markers":{"external_reachability":{"status":"success"}},"exit_code":0}]`,
		[]string{"SBM_CONNECTION_MODE=managed_proxy", "SBM_MANAGED_CANDIDATE_ID=" + candidateID},
	)
	if result.err == nil {
		t.Fatalf("verify deployment vps script unexpectedly succeeded: %s", result.output)
	}
	if !strings.Contains(result.output, "secret leaked in API response") {
		t.Fatalf("script output did not explain managed candidate identity leak: %s", result.output)
	}
	if strings.Contains(result.output, candidateID) {
		t.Fatalf("script output echoed leaked managed candidate identity: %s", result.output)
	}
	if result.generatedNodeReviewed {
		t.Fatalf("script should fail before generated-node review when managed candidate identity leaks: %s", result.output)
	}
}

func TestVerifyDeploymentVPSScriptRejectsPrivateKeyFileLeak(t *testing.T) {
	const privateKey = "-----BEGIN OPENSSH PRIVATE KEY-----\\nfile-private-key-secret\\n-----END OPENSSH PRIVATE KEY-----"
	result := runVerifyDeploymentVPSScriptWithConnectionTestResponseAndExtraEnv(
		t,
		`{"data":{"ok":true,"message":"`+privateKey+`","remote":{"uname_s":"Linux","uname_m":"x86_64"}}}`,
		[]string{
			"SBM_SSH_AUTH_METHOD=private_key",
			"SBM_SSH_PASSWORD=",
			"SBM_TEST_PRIVATE_KEY_FILE_CONTENT=" + privateKey,
		},
	)
	if result.err == nil {
		t.Fatalf("verify deployment vps script unexpectedly succeeded: %s", result.output)
	}
	if result.runCreated {
		t.Fatalf("script created deployment run despite private key file leak: %s", result.output)
	}
	if !strings.Contains(result.output, "secret leaked in API response") {
		t.Fatalf("script output did not explain private key file leak: %s", result.output)
	}
	if strings.Contains(result.output, privateKey) || strings.Contains(result.output, "file-private-key-secret") {
		t.Fatalf("script output echoed leaked private key: %s", result.output)
	}
}

func TestVerifyDeploymentVPSScriptRejectsInlinePrivateKeyLeak(t *testing.T) {
	const privateKey = "-----BEGIN OPENSSH PRIVATE KEY-----\\ninline-private-key-secret\\n-----END OPENSSH PRIVATE KEY-----"
	result := runVerifyDeploymentVPSScriptWithConnectionTestResponseAndExtraEnv(
		t,
		`{"data":{"ok":true,"message":"`+privateKey+`","remote":{"uname_s":"Linux","uname_m":"x86_64"}}}`,
		[]string{
			"SBM_SSH_AUTH_METHOD=private_key",
			"SBM_SSH_PASSWORD=",
			"SBM_SSH_PRIVATE_KEY=" + privateKey,
		},
	)
	if result.err == nil {
		t.Fatalf("verify deployment vps script unexpectedly succeeded: %s", result.output)
	}
	if result.runCreated {
		t.Fatalf("script created deployment run despite inline private key leak: %s", result.output)
	}
	if !strings.Contains(result.output, "secret leaked in API response") {
		t.Fatalf("script output did not explain inline private key leak: %s", result.output)
	}
	if strings.Contains(result.output, privateKey) || strings.Contains(result.output, "inline-private-key-secret") {
		t.Fatalf("script output echoed leaked inline private key: %s", result.output)
	}
}

func TestVerifyDeploymentVPSScriptRejectsPrivateKeyPassphraseLeak(t *testing.T) {
	const privateKey = "-----BEGIN OPENSSH PRIVATE KEY-----\\ninline-private-key-secret\\n-----END OPENSSH PRIVATE KEY-----"
	const passphrase = "inline-key-passphrase-secret"
	result := runVerifyDeploymentVPSScriptWithConnectionTestResponseAndExtraEnv(
		t,
		`{"data":{"ok":true,"message":"`+passphrase+`","remote":{"uname_s":"Linux","uname_m":"x86_64"}}}`,
		[]string{
			"SBM_SSH_AUTH_METHOD=private_key",
			"SBM_SSH_PASSWORD=",
			"SBM_SSH_PRIVATE_KEY=" + privateKey,
			"SBM_SSH_PRIVATE_KEY_PASSPHRASE=" + passphrase,
		},
	)
	if result.err == nil {
		t.Fatalf("verify deployment vps script unexpectedly succeeded: %s", result.output)
	}
	if result.runCreated {
		t.Fatalf("script created deployment run despite private key passphrase leak: %s", result.output)
	}
	if !strings.Contains(result.output, "secret leaked in API response") {
		t.Fatalf("script output did not explain private key passphrase leak: %s", result.output)
	}
	if strings.Contains(result.output, passphrase) {
		t.Fatalf("script output echoed leaked private key passphrase: %s", result.output)
	}
}

func TestVerifyDeploymentVPSScriptStopsWhenConnectionTestFails(t *testing.T) {
	result := runVerifyDeploymentVPSScriptWithConnectionTestResponseAndExtraEnv(
		t,
		`{"data":{"ok":false,"message":"SSH 连接失败","remote":{}}}`,
		[]string{"SBM_SSH_PASSWORD=ssh-password"},
	)
	if result.err == nil {
		t.Fatalf("verify deployment vps script unexpectedly succeeded: %s", result.output)
	}
	if result.runCreated {
		t.Fatalf("script created deployment run despite failed connection test: %s", result.output)
	}
	if !strings.Contains(result.output, "SSH 连接失败") {
		t.Fatalf("script output did not include connection-test failure: %s", result.output)
	}
}

func TestVerifyDeploymentVPSScriptRejectsFailedConnectionTestSecretLeak(t *testing.T) {
	result := runVerifyDeploymentVPSScriptWithConnectionTestResponseAndExtraEnv(
		t,
		`{"data":{"ok":false,"message":"ssh-password","remote":{}}}`,
		[]string{"SBM_SSH_PASSWORD=ssh-password"},
	)
	if result.err == nil {
		t.Fatalf("verify deployment vps script unexpectedly succeeded: %s", result.output)
	}
	if result.runCreated {
		t.Fatalf("script created deployment run despite failed connection-test leak: %s", result.output)
	}
	if !strings.Contains(result.output, "secret leaked in API response") {
		t.Fatalf("script output did not explain failed connection-test secret leak: %s", result.output)
	}
	if strings.Contains(result.output, "ssh-password") {
		t.Fatalf("script output echoed leaked SSH password: %s", result.output)
	}
}

func TestVerifyDeploymentVPSScriptDoesNotEchoHTTPErrorBodySecrets(t *testing.T) {
	result := runVerifyDeploymentVPSScriptWithConnectionTestHTTPError(t, http.StatusInternalServerError, `{"error":"ssh-password"}`)
	if result.err == nil {
		t.Fatalf("verify deployment vps script unexpectedly succeeded: %s", result.output)
	}
	if result.runCreated {
		t.Fatalf("script created deployment run despite connection-test HTTP error: %s", result.output)
	}
	if !strings.Contains(result.output, "HTTP 500 POST /api/deployments/connection-test") {
		t.Fatalf("script output did not report connection-test HTTP status: %s", result.output)
	}
	if !strings.Contains(result.output, "response body not printed") {
		t.Fatalf("script output did not explain omitted response body: %s", result.output)
	}
	if strings.Contains(result.output, "ssh-password") {
		t.Fatalf("script output echoed HTTP error body secret: %s", result.output)
	}
}

func TestVerifyDeploymentVPSScriptCanPreserveHTTPErrorBodyForDebugging(t *testing.T) {
	const responseBody = `{"error":"debug-body"}`
	result := runVerifyDeploymentVPSScriptWithConnectionTestHTTPError(
		t,
		http.StatusInternalServerError,
		responseBody,
		"SBM_DEBUG_PRESERVE_VERIFY_TMPDIR=true",
	)
	if result.err == nil {
		t.Fatalf("verify deployment vps script unexpectedly succeeded: %s", result.output)
	}
	tmpdir := verifierPreservedTmpdirFromOutput(result.output)
	if tmpdir == "" {
		t.Fatalf("script output did not report preserved tmpdir: %s", result.output)
	}
	defer os.RemoveAll(tmpdir)
	responsePath := filepath.Join(tmpdir, "connection-test-response.json")
	data, err := os.ReadFile(responsePath)
	if err != nil {
		t.Fatalf("read preserved response body: %v; output = %s", err, result.output)
	}
	if string(data) != responseBody {
		t.Fatalf("preserved response body = %q, want %q", data, responseBody)
	}
}

func TestVerifyDeploymentVPSScriptDoesNotEchoGeneratedNodeReviewSecrets(t *testing.T) {
	result := runVerifyDeploymentVPSScriptWithGeneratedNodeResponse(t, `{
		"data":{
			"can_import":false,
			"default_tag":"edge-a",
			"generated_node":{
				"type":"vless",
				"tag":"edge-a",
				"extra":{
					"uuid":"generated-uuid-secret",
					"tls":{"reality":{"short_id":"short-secret"}}
				}
			}
		}
	}`)
	if result.err == nil {
		t.Fatalf("verify deployment vps script unexpectedly succeeded: %s", result.output)
	}
	if !result.generatedNodeReviewed {
		t.Fatalf("script did not reach generated-node review: %s", result.output)
	}
	if !strings.Contains(result.output, "generated-node review cannot import") {
		t.Fatalf("script output did not report generated-node review failure: %s", result.output)
	}
	for _, secret := range []string{"generated-uuid-secret", "short-secret"} {
		if strings.Contains(result.output, secret) {
			t.Fatalf("script output echoed generated-node secret %q: %s", secret, result.output)
		}
	}
}

func TestVerifyDeploymentVPSScriptCancelsRunAfterCreate(t *testing.T) {
	result := runVerifyDeploymentVPSScriptWithExtraEnv(t, []string{"SBM_CANCEL_AFTER_CREATE=true"})
	if result.err != nil {
		t.Fatalf("verify deployment vps script error = %v, output = %s", result.err, result.output)
	}
	if !result.cancelCalled {
		t.Fatalf("script did not call cancel endpoint: %s", result.output)
	}
	if result.generatedNodeReviewed {
		t.Fatalf("script should stop after cancellation instead of reviewing generated node: %s", result.output)
	}
	if !strings.Contains(result.output, "cancellation validation passed: run-managed") {
		t.Fatalf("script output did not report cancellation validation: %s", result.output)
	}
}

func TestVerifyDeploymentVPSScriptRejectsCancelAndImportCombination(t *testing.T) {
	result := runVerifyDeploymentVPSScriptWithExtraEnv(t, []string{"SBM_CANCEL_AFTER_CREATE=true", "SBM_IMPORT_NODE=true"})
	if result.err == nil {
		t.Fatalf("verify deployment vps script unexpectedly succeeded: %s", result.output)
	}
	if result.runCreated || result.cancelCalled {
		t.Fatalf("script should fail before run creation when cancel/import conflict: %s", result.output)
	}
	if !strings.Contains(result.output, "SBM_CANCEL_AFTER_CREATE=true cannot be combined with SBM_IMPORT_NODE=true") {
		t.Fatalf("script output did not explain cancel/import conflict: %s", result.output)
	}
}

func TestVerifyDeploymentVPSScriptRejectsInvalidPreserveVerifyTmpdirFlag(t *testing.T) {
	result := runVerifyDeploymentVPSScriptWithExtraEnv(t, []string{"SBM_DEBUG_PRESERVE_VERIFY_TMPDIR=yes"})
	if result.err == nil {
		t.Fatalf("verify deployment vps script unexpectedly succeeded: %s", result.output)
	}
	if result.runCreated {
		t.Fatalf("script should fail before run creation when preserve tmpdir flag is invalid: %s", result.output)
	}
	if !strings.Contains(result.output, "SBM_DEBUG_PRESERVE_VERIFY_TMPDIR must be true or false") {
		t.Fatalf("script output did not explain invalid preserve tmpdir flag: %s", result.output)
	}
}

type runtimeCacheVerifierResult struct {
	output     string
	err        error
	runCreated bool
}

func runVerifyDeploymentVPSScriptWithRuntimeCache(t *testing.T, runtimeCacheJSON string) runtimeCacheVerifierResult {
	t.Helper()

	root := filepath.Join("..", "..")
	result := runtimeCacheVerifierResult{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/auth/me":
			_, _ = w.Write([]byte(`{"data":{"bootstrapped":true,"authenticated":true}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/templates":
			_, _ = w.Write([]byte(`{"data":[{"name":"singbox-vless-reality","runtime":{"name":"sing-box","version":"1.13.13"}}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/connection-candidates":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/deployments/connection-test":
			_, _ = w.Write([]byte(`{"data":{"ok":true,"remote":{"uname_s":"Linux","uname_m":"x86_64"}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/runtime-cache":
			_, _ = w.Write([]byte(`{"data":` + runtimeCacheJSON + `}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/deployments/runs":
			result.runCreated = true
			_, _ = w.Write([]byte(`{"data":{"id":"run-cache"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/runs":
			_, _ = w.Write([]byte(`{"data":[{"id":"run-cache","status":"success","generated_node":{"type":"vless","tag":"edge-a"},"progress_markers":{"external_reachability":{"status":"success"}},"exit_code":0}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/runs/run-cache":
			_, _ = w.Write([]byte(`{"data":{"id":"run-cache","status":"success","generated_node":{"type":"vless","tag":"edge-a"},"progress_markers":{"external_reachability":{"status":"success"}},"exit_code":0}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/runs/run-cache/generated-node":
			_, _ = w.Write([]byte(`{"data":{"can_import":true,"default_tag":"edge-a","generated_node":{"type":"vless","tag":"edge-a"}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cmd := exec.Command("bash", "scripts/verify-deployment-vps.sh")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"SBM_API_BASE="+server.URL,
		"SBM_PASSWORD=manager-password",
		"SBM_SSH_HOST=203.0.113.10",
		"SBM_SSH_PASSWORD=ssh-password",
		"SBM_VERIFY_POLL_SECONDS=0",
		"SBM_RUNTIME_SOURCE=cache",
	)
	output, err := cmd.CombinedOutput()
	result.output = string(output)
	result.err = err
	return result
}

type deploymentVerifierConnectionPayload struct {
	Mode               string `json:"mode"`
	ManagedCandidateID string `json:"managed_candidate_id"`
	ProxyType          string `json:"proxy_type"`
	ProxyHost          string `json:"proxy_host"`
	ProxyPort          int    `json:"proxy_port"`
	ProxyUsername      string `json:"proxy_username"`
	ProxyPassword      string `json:"proxy_password"`
}

type deploymentVerifierSSHPayload struct {
	Host                 string `json:"host"`
	Port                 int    `json:"port"`
	User                 string `json:"user"`
	AuthMethod           string `json:"auth_method"`
	Password             string `json:"password"`
	PrivateKey           string `json:"private_key"`
	PrivateKeyPassphrase string `json:"private_key_passphrase"`
}

type deploymentVerifierRequestPayload struct {
	SSH        deploymentVerifierSSHPayload        `json:"ssh"`
	Connection deploymentVerifierConnectionPayload `json:"connection"`
}

type verifyDeploymentScriptResult struct {
	output                   string
	connectionTestCandidate  string
	runCandidate             string
	connectionTestSSH        deploymentVerifierSSHPayload
	runSSH                   deploymentVerifierSSHPayload
	connectionTestConnection deploymentVerifierConnectionPayload
	runConnection            deploymentVerifierConnectionPayload
	runCreated               bool
	cancelCalled             bool
	generatedNodeReviewed    bool
	err                      error
}

func runVerifyDeploymentVPSScriptWithManagedCandidates(t *testing.T, requestedCandidateID string, candidatesJSON string) verifyDeploymentScriptResult {
	t.Helper()

	result := runVerifyDeploymentVPSScriptWithManagedCandidatesAllowError(t, requestedCandidateID, candidatesJSON)
	if result.err != nil {
		t.Fatalf("verify deployment vps script error = %v, output = %s", result.err, result.output)
	}
	return result
}

func runVerifyDeploymentVPSScriptWithManagedCandidatesAllowError(t *testing.T, requestedCandidateID string, candidatesJSON string) verifyDeploymentScriptResult {
	t.Helper()

	env := []string{"SBM_CONNECTION_MODE=managed_proxy"}
	if requestedCandidateID != "" {
		env = append(env, "SBM_MANAGED_CANDIDATE_ID="+requestedCandidateID)
	}
	return runVerifyDeploymentVPSScriptWithCandidatesAndExtraEnv(t, candidatesJSON, env)
}

func runVerifyDeploymentVPSScriptWithExtraEnv(t *testing.T, extraEnv []string) verifyDeploymentScriptResult {
	t.Helper()

	return runVerifyDeploymentVPSScriptWithCandidatesAndExtraEnv(t, `[]`, extraEnv)
}

func runVerifyDeploymentVPSScriptWithCandidatesAndExtraEnv(t *testing.T, candidatesJSON string, extraEnv []string) verifyDeploymentScriptResult {
	t.Helper()

	return runVerifyDeploymentVPSScriptWithRemoteAndRunCurrent(t, candidatesJSON, `{"uname_s":"Linux","uname_m":"x86_64"}`, `{"id":"run-managed","status":"success","generated_node":{"type":"vless","tag":"edge-a"},"progress_markers":{"external_reachability":{"status":"success"}},"exit_code":0}`, extraEnv)
}

func runVerifyDeploymentVPSScriptWithRemote(t *testing.T, remoteJSON string, extraEnv []string) verifyDeploymentScriptResult {
	t.Helper()

	return runVerifyDeploymentVPSScriptWithRemoteAndRunCurrent(t, `[]`, remoteJSON, `{"id":"run-managed","status":"success","generated_node":{"type":"vless","tag":"edge-a"},"progress_markers":{"external_reachability":{"status":"success"}},"exit_code":0}`, extraEnv)
}

func runVerifyDeploymentVPSScriptWithRunCurrent(t *testing.T, runCurrentJSON string, extraEnv []string) verifyDeploymentScriptResult {
	t.Helper()

	return runVerifyDeploymentVPSScriptWithRemoteAndRunCurrent(t, `[]`, `{"uname_s":"Linux","uname_m":"x86_64"}`, runCurrentJSON, extraEnv)
}

func runVerifyDeploymentVPSScriptWithRemoteAndRunCurrent(t *testing.T, candidatesJSON string, remoteJSON string, runCurrentJSON string, extraEnv []string) verifyDeploymentScriptResult {
	t.Helper()

	return runVerifyDeploymentVPSScriptWithRemoteCurrentAndList(t, candidatesJSON, remoteJSON, runCurrentJSON, `[`+runCurrentJSON+`]`, extraEnv)
}

func runVerifyDeploymentVPSScriptWithCurrentAndList(t *testing.T, runCurrentJSON string, runListJSON string, extraEnv []string) verifyDeploymentScriptResult {
	t.Helper()

	return runVerifyDeploymentVPSScriptWithRemoteCurrentAndList(t, `[]`, `{"uname_s":"Linux","uname_m":"x86_64"}`, runCurrentJSON, runListJSON, extraEnv)
}

func runVerifyDeploymentVPSScriptWithConnectionTestResponseAndExtraEnv(t *testing.T, connectionTestResponse string, extraEnv []string) verifyDeploymentScriptResult {
	t.Helper()

	root := filepath.Join("..", "..")
	result := verifyDeploymentScriptResult{}
	env := verifierExtraEnvWithPrivateKeyFile(t, extraEnv)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/auth/me":
			_, _ = w.Write([]byte(`{"data":{"bootstrapped":true,"authenticated":true}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/templates":
			_, _ = w.Write([]byte(`{"data":[{"name":"singbox-vless-reality","runtime":{"name":"sing-box","version":"1.13.13"}}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/connection-candidates":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/deployments/connection-test":
			payload := decodeVerifierPayload(t, r)
			result.connectionTestSSH = payload.SSH
			result.connectionTestConnection = payload.Connection
			_, _ = w.Write([]byte(connectionTestResponse))
		case r.Method == http.MethodPost && r.URL.Path == "/api/deployments/runs":
			result.runCreated = true
			_, _ = w.Write([]byte(`{"data":{"id":"run-should-not-start"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cmd := exec.Command("bash", "scripts/verify-deployment-vps.sh")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"SBM_API_BASE="+server.URL,
		"SBM_PASSWORD=manager-password",
		"SBM_SSH_HOST=203.0.113.10",
		"SBM_VERIFY_POLL_SECONDS=0",
	)
	cmd.Env = append(cmd.Env, env...)
	output, err := cmd.CombinedOutput()
	result.output = string(output)
	result.err = err
	return result
}

func runVerifyDeploymentVPSScriptWithConnectionTestHTTPError(t *testing.T, status int, responseBody string, extraEnv ...string) verifyDeploymentScriptResult {
	t.Helper()

	root := filepath.Join("..", "..")
	result := verifyDeploymentScriptResult{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/auth/me":
			_, _ = w.Write([]byte(`{"data":{"bootstrapped":true,"authenticated":true}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/templates":
			_, _ = w.Write([]byte(`{"data":[{"name":"singbox-vless-reality","runtime":{"name":"sing-box","version":"1.13.13"}}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/connection-candidates":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/deployments/connection-test":
			w.WriteHeader(status)
			_, _ = w.Write([]byte(responseBody))
		case r.Method == http.MethodPost && r.URL.Path == "/api/deployments/runs":
			result.runCreated = true
			_, _ = w.Write([]byte(`{"data":{"id":"run-should-not-start"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cmd := exec.Command("bash", "scripts/verify-deployment-vps.sh")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"SBM_API_BASE="+server.URL,
		"SBM_PASSWORD=manager-password",
		"SBM_SSH_HOST=203.0.113.10",
		"SBM_SSH_PASSWORD=ssh-password",
		"SBM_VERIFY_POLL_SECONDS=0",
	)
	cmd.Env = append(cmd.Env, extraEnv...)
	output, err := cmd.CombinedOutput()
	result.output = string(output)
	result.err = err
	return result
}

func verifierPreservedTmpdirFromOutput(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if path, ok := strings.CutPrefix(line, "verifier tmpdir preserved: "); ok {
			return strings.TrimSpace(path)
		}
	}
	return ""
}

func assertDeploymentAssetScriptModes(t *testing.T, archivePath string, scriptPaths []string) {
	t.Helper()

	file, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("open deployment assets archive: %v", err)
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("open gzip reader: %v", err)
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	modes := map[string]int64{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read tar header: %v", err)
		}
		modes[header.Name] = header.Mode
	}
	for _, path := range scriptPaths {
		mode, ok := modes[path]
		if !ok {
			t.Fatalf("deployment assets archive missing script %s", path)
		}
		if mode&0o100 == 0 {
			t.Fatalf("deployment asset script %s is not owner-executable: mode=%#o", path, mode)
		}
	}
}

func runVerifyDeploymentVPSScriptWithGeneratedNodeResponse(t *testing.T, generatedNodeResponse string) verifyDeploymentScriptResult {
	t.Helper()

	root := filepath.Join("..", "..")
	result := verifyDeploymentScriptResult{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/auth/me":
			_, _ = w.Write([]byte(`{"data":{"bootstrapped":true,"authenticated":true}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/templates":
			_, _ = w.Write([]byte(`{"data":[{"name":"singbox-vless-reality","runtime":{"name":"sing-box","version":"1.13.13"}}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/connection-candidates":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/deployments/connection-test":
			_, _ = w.Write([]byte(`{"data":{"ok":true,"remote":{"uname_s":"Linux","uname_m":"x86_64"}}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/deployments/runs":
			result.runCreated = true
			_, _ = w.Write([]byte(`{"data":{"id":"run-generated-node"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/runs":
			_, _ = w.Write([]byte(`{"data":[{"id":"run-generated-node","status":"success","generated_node":{"type":"vless","tag":"edge-a"},"progress_markers":{"external_reachability":{"status":"success"}},"exit_code":0}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/runs/run-generated-node":
			_, _ = w.Write([]byte(`{"data":{"id":"run-generated-node","status":"success","generated_node":{"type":"vless","tag":"edge-a"},"progress_markers":{"external_reachability":{"status":"success"}},"exit_code":0}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/runs/run-generated-node/generated-node":
			result.generatedNodeReviewed = true
			_, _ = w.Write([]byte(generatedNodeResponse))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cmd := exec.Command("bash", "scripts/verify-deployment-vps.sh")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"SBM_API_BASE="+server.URL,
		"SBM_PASSWORD=manager-password",
		"SBM_SSH_HOST=203.0.113.10",
		"SBM_SSH_PASSWORD=ssh-password",
		"SBM_VERIFY_POLL_SECONDS=0",
	)
	output, err := cmd.CombinedOutput()
	result.output = string(output)
	result.err = err
	return result
}

func runVerifyDeploymentVPSScriptWithManualNodesResponse(t *testing.T, manualNodesResponse string) verifyDeploymentScriptResult {
	t.Helper()

	root := filepath.Join("..", "..")
	result := verifyDeploymentScriptResult{}
	imported := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/auth/me":
			_, _ = w.Write([]byte(`{"data":{"bootstrapped":true,"authenticated":true}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/templates":
			_, _ = w.Write([]byte(`{"data":[{"name":"singbox-vless-reality","runtime":{"name":"sing-box","version":"1.13.13"}}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/connection-candidates":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/deployments/connection-test":
			_, _ = w.Write([]byte(`{"data":{"ok":true,"remote":{"uname_s":"Linux","uname_m":"x86_64"}}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/deployments/runs":
			result.runCreated = true
			_, _ = w.Write([]byte(`{"data":{"id":"run-import"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/runs":
			_, _ = w.Write([]byte(`{"data":[{"id":"run-import","status":"success","generated_node":{"type":"vless","tag":"edge-a"},"progress_markers":{"external_reachability":{"status":"success"}},"exit_code":0}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/runs/run-import":
			_, _ = w.Write([]byte(`{"data":{"id":"run-import","status":"success","generated_node":{"type":"vless","tag":"edge-a"},"progress_markers":{"external_reachability":{"status":"success"}},"exit_code":0}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/runs/run-import/generated-node":
			if imported {
				_, _ = w.Write([]byte(`{"data":{"can_import":false,"imported_node_id":42,"default_tag":"edge-a","generated_node":{"type":"vless","tag":"edge-a"}}}`))
				return
			}
			result.generatedNodeReviewed = true
			_, _ = w.Write([]byte(`{"data":{"can_import":true,"default_tag":"edge-a","generated_node":{"type":"vless","tag":"edge-a"}}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/deployments/runs/run-import/import-node":
			imported = true
			_, _ = w.Write([]byte(`{"data":{"id":42,"tag":"edge-a"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/manual-nodes":
			_, _ = w.Write([]byte(manualNodesResponse))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cmd := exec.Command("bash", "scripts/verify-deployment-vps.sh")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"SBM_API_BASE="+server.URL,
		"SBM_PASSWORD=manager-password",
		"SBM_SSH_HOST=203.0.113.10",
		"SBM_SSH_PASSWORD=ssh-password",
		"SBM_VERIFY_POLL_SECONDS=0",
		"SBM_IMPORT_NODE=true",
	)
	output, err := cmd.CombinedOutput()
	result.output = string(output)
	result.err = err
	return result
}

func verifierExtraEnvWithPrivateKeyFile(t *testing.T, extraEnv []string) []string {
	t.Helper()

	env := append([]string{}, extraEnv...)
	for _, item := range extraEnv {
		content, ok := strings.CutPrefix(item, "SBM_TEST_PRIVATE_KEY_FILE_CONTENT=")
		if !ok {
			continue
		}
		keyPath := filepath.Join(t.TempDir(), "id_ed25519")
		if err := os.WriteFile(keyPath, []byte(content), 0o600); err != nil {
			t.Fatalf("write private key file: %v", err)
		}
		env = append(env, "SBM_SSH_PRIVATE_KEY_FILE="+keyPath)
	}
	return env
}

func runVerifyDeploymentVPSScriptWithRemoteCurrentAndList(t *testing.T, candidatesJSON string, remoteJSON string, runCurrentJSON string, runListJSON string, extraEnv []string) verifyDeploymentScriptResult {
	t.Helper()

	root := filepath.Join("..", "..")
	env := verifierExtraEnvWithPrivateKeyFile(t, extraEnv)
	result := verifyDeploymentScriptResult{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/auth/me":
			_, _ = w.Write([]byte(`{"data":{"bootstrapped":true,"authenticated":true}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/templates":
			_, _ = w.Write([]byte(`{"data":[{"name":"singbox-vless-reality","runtime":{"name":"sing-box","version":"1.13.13"}}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/connection-candidates":
			_, _ = w.Write([]byte(`{"data":` + candidatesJSON + `}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/deployments/connection-test":
			payload := decodeVerifierPayload(t, r)
			result.connectionTestSSH = payload.SSH
			result.connectionTestConnection = payload.Connection
			result.connectionTestCandidate = result.connectionTestConnection.ManagedCandidateID
			_, _ = w.Write([]byte(`{"data":{"ok":true,"remote":` + remoteJSON + `}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/deployments/runs":
			result.runCreated = true
			payload := decodeVerifierPayload(t, r)
			result.runSSH = payload.SSH
			result.runConnection = payload.Connection
			result.runCandidate = result.runConnection.ManagedCandidateID
			_, _ = w.Write([]byte(`{"data":{"id":"run-managed"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/deployments/runs/run-managed/cancel":
			result.cancelCalled = true
			_, _ = w.Write([]byte(`{"data":{"id":"run-managed","status":"cancelled","redacted_params":{"ssh_password":"[REDACTED]"}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/runs":
			if result.cancelCalled {
				_, _ = w.Write([]byte(`{"data":[{"id":"run-managed","status":"cancelled","redacted_params":{"ssh_password":"[REDACTED]"}}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":` + runListJSON + `}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/runs/run-managed":
			if result.cancelCalled {
				_, _ = w.Write([]byte(`{"data":{"id":"run-managed","status":"cancelled","redacted_params":{"ssh_password":"[REDACTED]"}}}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":` + runCurrentJSON + `}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/runs/run-managed/generated-node":
			result.generatedNodeReviewed = true
			_, _ = w.Write([]byte(`{"data":{"can_import":true,"default_tag":"edge-a","generated_node":{"type":"vless","tag":"edge-a"}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cmd := exec.Command("bash", "scripts/verify-deployment-vps.sh")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"SBM_API_BASE="+server.URL,
		"SBM_PASSWORD=manager-password",
		"SBM_SSH_HOST=203.0.113.10",
		"SBM_SSH_PASSWORD=ssh-password",
		"SBM_VERIFY_POLL_SECONDS=0",
	)
	cmd.Env = append(cmd.Env, env...)
	output, err := cmd.CombinedOutput()
	result.output = string(output)
	result.err = err
	return result
}

func decodeVerifierPayload(t *testing.T, r *http.Request) deploymentVerifierRequestPayload {
	t.Helper()

	var payload deploymentVerifierRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	return payload
}
