package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/structName/sing-box-manager-gui/internal/database/models"
	"github.com/structName/sing-box-manager-gui/internal/deploy"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/ssh"
)

type deploymentScriptExecutor interface {
	Execute(ctx context.Context, req deploymentConnectionTestRequest, script []byte, env map[string]string, executionMode string) (string, error)
}

type deploymentReachabilityChecker interface {
	Check(ctx context.Context, req deploymentConnectionTestRequest, host string, port int) error
}

type deploymentArchiveUploader interface {
	Upload(ctx context.Context, localPath string) (remotePath string, cleanup func(context.Context), err error)
}

type remoteDeploymentRunner struct {
	executor            deploymentScriptExecutor
	reachabilityChecker deploymentReachabilityChecker
	dataDir             string
	checkpoint          func(*models.DeploymentRun, int, string, string)
}

func (r remoteDeploymentRunner) Run(ctx context.Context, run *models.DeploymentRun, req deploymentRunRequest) error {
	template, ok := deploy.BuiltinTemplates().Get(req.TemplateName)
	if !ok {
		return fmt.Errorf("不支持的部署模板")
	}
	probe, ok := deploy.BuiltinTemplates().Get("probe-system")
	if !ok {
		return fmt.Errorf("系统探测模板不存在")
	}
	executor := r.executor
	if executor == nil {
		executor = sshDeploymentScriptExecutor{}
	}
	reachabilityChecker := r.reachabilityChecker
	if reachabilityChecker == nil {
		reachabilityChecker = tcpDeploymentReachabilityChecker{}
	}
	connectionReq := deploymentConnectionTestRequest{
		SSH:        req.SSH,
		Connection: req.Connection,
	}
	progress := models.JSONMap{}
	probeOutput, execErr := executor.Execute(ctx, connectionReq, probe.Content, deploymentProbeEnv(req), probe.ExecutionMode)
	run.Stdout = probeOutput
	parsedProbe, err := deploy.ParseScriptOutput(probeOutput)
	if err != nil {
		run.Stderr = err.Error()
		exitCode := 1
		run.ExitCode = &exitCode
		return err
	}
	mergeDeploymentProgress(progress, parsedProbe.Progress)
	run.ProbeResult = models.JSONMap(parsedProbe.Result)
	run.ProgressMarkers = progress
	r.checkpointRun(run, 25, "probe_system", "系统探测完成")
	if err := deploymentResultStatusError(parsedProbe.Result, "系统探测失败"); err != nil {
		run.Stderr = err.Error()
		exitCode := 1
		run.ExitCode = &exitCode
		run.ProgressMarkers = progress
		return err
	}
	if execErr != nil {
		run.Stderr = execErr.Error()
		exitCode := 1
		run.ExitCode = &exitCode
		run.ProgressMarkers = progress
		return execErr
	}
	if err := validateDeploymentProbeResult(parsedProbe.Result); err != nil {
		run.Stderr = err.Error()
		exitCode := 1
		run.ExitCode = &exitCode
		run.ProgressMarkers = progress
		return err
	}

	envReq := req
	envReq.Parameters = copyDeploymentParameters(req.Parameters)
	if osName, _ := parsedProbe.Result["os"].(string); strings.TrimSpace(osName) != "" {
		envReq.Parameters["runtime_os"] = strings.TrimSpace(osName)
	}
	if arch, _ := parsedProbe.Result["arch"].(string); strings.TrimSpace(arch) != "" {
		envReq.Parameters["runtime_arch"] = strings.TrimSpace(arch)
	}
	annotateDeploymentRuntimeHistory(run, envReq, template)
	r.checkpointRun(run, 35, "runtime_source", "运行时来源已确定")
	env, err := deploymentScriptEnv(envReq, r.dataDir, template)
	if err != nil {
		run.Stderr = err.Error()
		exitCode := 1
		run.ExitCode = &exitCode
		run.ProgressMarkers = progress
		return err
	}

	output, execErr := executor.Execute(ctx, connectionReq, template.Content, env, template.ExecutionMode)
	run.Stdout = probeOutput + output
	parsed, err := deploy.ParseScriptOutput(output)
	if err != nil {
		run.Stderr = err.Error()
		exitCode := 1
		run.ExitCode = &exitCode
		return err
	}
	mergeDeploymentProgress(progress, parsed.Progress)
	if parsed.Result != nil {
		progress["deployment_result"] = parsed.Result
	}
	run.ProgressMarkers = progress
	run.GeneratedNode = models.JSONMap(parsed.GeneratedNode)
	r.checkpointRun(run, 80, "deployment_script", "部署脚本执行完成")
	if err := deploymentResultStatusError(parsed.Result, "部署脚本失败"); err != nil {
		run.Stderr = err.Error()
		exitCode := 1
		run.ExitCode = &exitCode
		run.ProgressMarkers = progress
		return err
	}
	if execErr != nil {
		run.Stderr = execErr.Error()
		exitCode := 1
		run.ExitCode = &exitCode
		run.ProgressMarkers = progress
		return execErr
	}
	if shouldVerifyDeploymentExternalReachability(template, parsed.GeneratedNode) {
		if err := verifyDeploymentExternalReachability(ctx, reachabilityChecker, connectionReq, req, progress); err != nil {
			run.Stderr = err.Error()
			exitCode := 1
			run.ExitCode = &exitCode
			run.ProgressMarkers = progress
			return err
		}
		run.ProgressMarkers = progress
		r.checkpointRun(run, 95, "external_reachability", "外部可达性检查完成")
	}
	exitCode := 0
	run.ExitCode = &exitCode
	return nil
}

func (r remoteDeploymentRunner) checkpointRun(run *models.DeploymentRun, progress int, currentItem, message string) {
	if r.checkpoint != nil {
		r.checkpoint(run, progress, currentItem, message)
	}
}

func shouldVerifyDeploymentExternalReachability(template deploy.Template, generatedNode map[string]interface{}) bool {
	return template.Name == "singbox-vless-reality" || len(generatedNode) > 0
}

func deploymentResultStatusError(result map[string]interface{}, fallback string) error {
	if result == nil {
		return nil
	}
	status, _ := result["status"].(string)
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" || status == "success" {
		return nil
	}
	message, _ := result["message"].(string)
	message = strings.TrimSpace(message)
	if message == "" {
		message = status
	}
	return fmt.Errorf("%s: %s", fallback, message)
}

func verifyDeploymentExternalReachability(ctx context.Context, checker deploymentReachabilityChecker, connectionReq deploymentConnectionTestRequest, req deploymentRunRequest, progress models.JSONMap) error {
	host := strings.TrimSpace(stringParam(req.Parameters, "node_server", req.SSH.Host))
	port := intParam(req.Parameters, "proxy_port", 443)
	marker := map[string]interface{}{
		"step":   "external_reachability",
		"host":   host,
		"port":   port,
		"status": "running",
	}
	if err := checker.Check(ctx, connectionReq, host, port); err != nil {
		reachabilityErr := fmt.Errorf("部署节点端口不可达: %s", stripSensitiveErrorText(err.Error()))
		marker["status"] = "failed"
		marker["message"] = reachabilityErr.Error()
		progress["external_reachability"] = marker
		return reachabilityErr
	}
	marker["status"] = "success"
	progress["external_reachability"] = marker
	return nil
}

type tcpDeploymentReachabilityChecker struct{}

func (tcpDeploymentReachabilityChecker) Check(ctx context.Context, req deploymentConnectionTestRequest, host string, port int) error {
	host = strings.TrimSpace(host)
	if host == "" {
		return fmt.Errorf("node_server 为必填")
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("proxy_port 无效")
	}
	dialContext, err := deploymentDialerForConnection(req.Connection)
	if err != nil {
		return err
	}
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := dialContext(checkCtx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}

func deploymentProbeEnv(req deploymentRunRequest) map[string]string {
	return map[string]string{
		"SBM_PROXY_PORT": strconv.Itoa(intParam(req.Parameters, "proxy_port", 443)),
	}
}

func mergeDeploymentProgress(progress models.JSONMap, markers []map[string]interface{}) {
	for _, marker := range markers {
		if step, ok := marker["step"].(string); ok {
			progress[step] = marker
		}
	}
}

func validateDeploymentProbeResult(result map[string]interface{}) error {
	if result == nil {
		return fmt.Errorf("系统探测未返回结果")
	}
	osName, _ := result["os"].(string)
	if !strings.EqualFold(strings.TrimSpace(osName), "linux") {
		return fmt.Errorf("目标系统仅支持 Linux")
	}
	arch, _ := result["arch"].(string)
	switch strings.TrimSpace(arch) {
	case "amd64", "arm64":
	default:
		return fmt.Errorf("目标架构不受支持: %s", strings.TrimSpace(arch))
	}
	if boolFromDeploymentMap(result, "port_listening") {
		return fmt.Errorf("目标端口已被占用")
	}
	if !boolFromDeploymentMap(result, "systemd") {
		return fmt.Errorf("目标系统未检测到 systemd")
	}
	privilege, _ := result["privilege_mode"].(string)
	if privilege != "root" && privilege != "passwordless_sudo" {
		return fmt.Errorf("目标主机需要 root 或免密 sudo")
	}
	return nil
}

func boolFromDeploymentMap(values map[string]interface{}, key string) bool {
	value, ok := values[key]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

type sshDeploymentScriptExecutor struct{}

func (sshDeploymentScriptExecutor) Execute(ctx context.Context, req deploymentConnectionTestRequest, script []byte, env map[string]string, executionMode string) (string, error) {
	client, err := newDeploymentSSHClient(ctx, req)
	if err != nil {
		return "", err
	}
	defer client.Close()

	remoteEnv, cleanup, err := prepareDeploymentScriptEnv(ctx, env, sshDeploymentArchiveUploader{client: client})
	if err != nil {
		return "", err
	}
	defer cleanup(context.Background())

	switch strings.TrimSpace(executionMode) {
	case "", "stdin":
		command := deploymentRemoteShellCommand(remoteEnv, "bash -s")
		return runDeploymentSSHCommand(ctx, client, command, bytes.NewReader(script))
	case "uploaded_script":
		return executeUploadedDeploymentScript(ctx, client, script, remoteEnv)
	default:
		return "", fmt.Errorf("不支持的模板执行模式: %s", executionMode)
	}
}

func executeUploadedDeploymentScript(ctx context.Context, client *ssh.Client, script []byte, env map[string]string) (string, error) {
	remoteDir, err := runDeploymentSSHCommand(ctx, client, "mktemp -d /tmp/sbm-deploy-XXXXXX", nil)
	if err != nil {
		return "", err
	}
	remoteDir = strings.TrimSpace(remoteDir)
	if remoteDir == "" {
		return "", fmt.Errorf("remote temporary run directory is empty")
	}
	cleanup := func(cleanupCtx context.Context) {
		_, _ = runDeploymentSSHCommand(cleanupCtx, client, "rm -rf "+shellQuote(remoteDir), nil)
	}
	preserveRunDir := boolEnv(env, "SBM_DEBUG_PRESERVE_REMOTE_RUN_DIR")
	if !preserveRunDir {
		defer cleanup(context.Background())
	}
	preservedMarker := remoteRunDirProgressMarker(remoteDir, preserveRunDir)

	scriptPath := strings.TrimRight(remoteDir, "/") + "/template.sh"
	if _, err := runDeploymentSSHCommand(ctx, client, "cat > "+shellQuote(scriptPath)+" && chmod 0700 "+shellQuote(scriptPath), bytes.NewReader(script)); err != nil {
		return preservedMarker, err
	}
	remoteEnv := make(map[string]string, len(env)+1)
	for key, value := range env {
		remoteEnv[key] = value
	}
	remoteEnv["SBM_REMOTE_RUN_DIR"] = remoteDir
	command := uploadedDeploymentScriptCommand(remoteDir, scriptPath, remoteEnv)
	output, err := runDeploymentSSHCommand(ctx, client, command, nil)
	return preservedMarker + output, err
}

func uploadedDeploymentScriptCommand(remoteDir, scriptPath string, env map[string]string) string {
	return "cd " + shellQuote(remoteDir) + " && " + deploymentRemoteShellCommand(env, "bash "+shellQuote(scriptPath))
}

func boolEnv(env map[string]string, key string) bool {
	return strings.EqualFold(strings.TrimSpace(env[key]), "true")
}

func remoteRunDirProgressMarker(remoteDir string, preserved bool) string {
	if !preserved {
		return ""
	}
	marker := map[string]interface{}{
		"step":      "remote_run_dir",
		"status":    "preserved",
		"path":      remoteDir,
		"preserved": true,
	}
	data, err := json.Marshal(marker)
	if err != nil {
		return ""
	}
	return "SBM_PROGRESS " + string(data) + "\n"
}

func prepareDeploymentScriptEnv(ctx context.Context, env map[string]string, uploader deploymentArchiveUploader) (map[string]string, func(context.Context), error) {
	prepared := make(map[string]string, len(env))
	for key, value := range env {
		prepared[key] = value
	}
	cleanup := func(context.Context) {}
	archivePath := strings.TrimSpace(prepared["SBM_SINGBOX_ARCHIVE"])
	if archivePath == "" {
		return prepared, cleanup, nil
	}
	if uploader == nil {
		return nil, cleanup, fmt.Errorf("runtime archive uploader is not configured")
	}
	remotePath, remoteCleanup, err := uploader.Upload(ctx, archivePath)
	if err != nil {
		return nil, cleanup, fmt.Errorf("上传 runtime archive 失败: %w", err)
	}
	prepared["SBM_SINGBOX_ARCHIVE"] = remotePath
	if remoteCleanup != nil {
		cleanup = remoteCleanup
	}
	return prepared, cleanup, nil
}

type sshDeploymentArchiveUploader struct {
	client *ssh.Client
}

func (u sshDeploymentArchiveUploader) Upload(ctx context.Context, localPath string) (string, func(context.Context), error) {
	if u.client == nil {
		return "", nil, fmt.Errorf("SSH client is not initialized")
	}
	file, err := os.Open(localPath)
	if err != nil {
		return "", nil, err
	}
	defer file.Close()

	remotePath, err := runDeploymentSSHCommand(ctx, u.client, "mktemp /tmp/sbm-sing-box-archive-XXXXXX.tar.gz", nil)
	if err != nil {
		return "", nil, err
	}
	remotePath = strings.TrimSpace(remotePath)
	if remotePath == "" {
		return "", nil, fmt.Errorf("remote temporary archive path is empty")
	}
	cleanup := func(cleanupCtx context.Context) {
		_, _ = runDeploymentSSHCommand(cleanupCtx, u.client, "rm -f "+shellQuote(remotePath), nil)
	}
	if _, err := runDeploymentSSHCommand(ctx, u.client, "cat > "+shellQuote(remotePath)+" && chmod 0600 "+shellQuote(remotePath), file); err != nil {
		cleanup(context.Background())
		return "", nil, err
	}
	return remotePath, cleanup, nil
}

func runDeploymentSSHCommand(ctx context.Context, client *ssh.Client, command string, stdin io.Reader) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	if stdin != nil {
		session.Stdin = stdin
	}
	done := make(chan struct {
		output []byte
		err    error
	}, 1)
	go func() {
		output, err := session.CombinedOutput(command)
		done <- struct {
			output []byte
			err    error
		}{output: output, err: err}
	}()

	select {
	case <-ctx.Done():
		_ = session.Close()
		return "", ctx.Err()
	case got := <-done:
		return string(got.output), got.err
	}
}

func deploymentRemoteShellCommand(env map[string]string, scriptCommand string) string {
	assignments := make([]string, 0, len(env)+1)
	assignments = append(assignments, "env")
	for key, value := range env {
		assignments = append(assignments, key+"="+shellQuote(value))
	}
	assignments = append(assignments, scriptCommand)
	return strings.Join(assignments, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func deploymentScriptEnv(req deploymentRunRequest, dataDir string, template deploy.Template) (map[string]string, error) {
	switch template.Name {
	case "singbox-vless-reality":
		return vlessRealityDeploymentScriptEnv(req, dataDir, template)
	case "security-basic":
		return securityBasicDeploymentScriptEnv(req), nil
	default:
		return nil, fmt.Errorf("不支持的部署模板")
	}
}

func vlessRealityDeploymentScriptEnv(req deploymentRunRequest, dataDir string, template deploy.Template) (map[string]string, error) {
	env := map[string]string{
		"SBM_NODE_NAME":           stringParam(req.Parameters, "node_name", "deployed-"+req.SSH.Host),
		"SBM_NODE_SERVER":         stringParam(req.Parameters, "node_server", req.SSH.Host),
		"SBM_PROXY_PORT":          strconv.Itoa(intParam(req.Parameters, "proxy_port", 443)),
		"SBM_UUID":                stringParam(req.Parameters, "sbm_uuid", uuid.New().String()),
		"SBM_REALITY_SHORT_ID":    stringParam(req.Parameters, "reality_short_id", randomHexString(8)),
		"SBM_REALITY_SERVER_NAME": stringParam(req.Parameters, "reality_server_name", "www.microsoft.com"),
		"SBM_SINGBOX_VERSION":     template.Runtime.Version,
	}
	if boolParam(req.Parameters, "debug_preserve_remote_run_dir", false) {
		env["SBM_DEBUG_PRESERVE_REMOTE_RUN_DIR"] = "true"
	}
	if strings.TrimSpace(env["SBM_SINGBOX_VERSION"]) == "" {
		env["SBM_SINGBOX_VERSION"] = deploy.PinnedSingBoxVersion
	}
	if checksum, ok := deploy.RuntimeChecksum("sing-box", env["SBM_SINGBOX_VERSION"], "linux", "amd64"); ok {
		env["SBM_SINGBOX_SHA256_AMD64"] = checksum
	}
	if checksum, ok := deploy.RuntimeChecksum("sing-box", env["SBM_SINGBOX_VERSION"], "linux", "arm64"); ok {
		env["SBM_SINGBOX_SHA256_ARM64"] = checksum
	}
	if archivePath, err := deploymentRuntimeArchivePath(req, dataDir, template); err != nil {
		return nil, err
	} else if archivePath != "" {
		env["SBM_SINGBOX_ARCHIVE"] = archivePath
	}
	privateKey := stringParam(req.Parameters, "reality_private_key", "")
	publicKey := stringParam(req.Parameters, "reality_public_key", "")
	if privateKey == "" && publicKey == "" {
		var err error
		privateKey, publicKey, err = generateRealityKeyPair()
		if err != nil {
			return nil, err
		}
	} else if privateKey == "" || publicKey == "" {
		return nil, fmt.Errorf("reality_private_key 和 reality_public_key 必须同时提供")
	}
	env["SBM_REALITY_PRIVATE_KEY"] = privateKey
	env["SBM_REALITY_PUBLIC_KEY"] = publicKey
	return env, nil
}

func securityBasicDeploymentScriptEnv(req deploymentRunRequest) map[string]string {
	return map[string]string{
		"SBM_SECURITY_INSTALL_BASE_PACKAGES": strconv.FormatBool(boolParam(req.Parameters, "security_install_base_packages", false)),
		"SBM_SECURITY_BASE_PACKAGES":         stringParam(req.Parameters, "security_base_packages", "curl ca-certificates tar gzip unzip"),
		"SBM_SECURITY_FIREWALL_MODE":         stringParam(req.Parameters, "security_firewall_mode", "inspect_only"),
	}
}

func deploymentRuntimeArchivePath(req deploymentRunRequest, dataDir string, template deploy.Template) (string, error) {
	source := strings.ToLower(strings.TrimSpace(stringParam(req.Parameters, "runtime_source", "remote")))
	switch source {
	case "", "remote":
		return "", nil
	case "cache":
	default:
		return "", fmt.Errorf("runtime_source 仅支持 remote 或 cache")
	}
	runtimeName := strings.TrimSpace(template.Runtime.Name)
	if runtimeName == "" {
		return "", fmt.Errorf("模板没有 runtime 元数据")
	}
	version := stringParam(req.Parameters, "runtime_version", template.Runtime.Version)
	if strings.TrimSpace(version) == "" {
		version = deploy.PinnedSingBoxVersion
	}
	osName := stringParam(req.Parameters, "runtime_os", "linux")
	arch := strings.TrimSpace(stringParam(req.Parameters, "runtime_arch", ""))
	if arch == "" {
		return "", fmt.Errorf("runtime_arch 为必填")
	}
	expectedChecksum, ok := deploy.RuntimeChecksum(runtimeName, version, osName, arch)
	if !ok {
		return "", fmt.Errorf("runtime checksum metadata missing for %s %s-%s", version, osName, arch)
	}
	archive, err := deploy.NewRuntimeCache(dataDir).Lookup(runtimeName, version, osName, arch, expectedChecksum)
	if err != nil {
		return "", err
	}
	return archive.Path, nil
}

func annotateDeploymentRuntimeHistory(run *models.DeploymentRun, req deploymentRunRequest, template deploy.Template) {
	if run.RedactedParams == nil {
		run.RedactedParams = models.JSONMap{}
	}
	if strings.TrimSpace(template.Runtime.Name) == "" {
		return
	}
	source := strings.ToLower(strings.TrimSpace(stringParam(req.Parameters, "runtime_source", "remote")))
	if source == "" {
		source = "remote"
	}
	version := stringParam(req.Parameters, "runtime_version", template.Runtime.Version)
	if strings.TrimSpace(version) == "" {
		version = deploy.PinnedSingBoxVersion
	}
	osName := stringParam(req.Parameters, "runtime_os", "linux")
	arch := strings.TrimSpace(stringParam(req.Parameters, "runtime_arch", ""))
	run.RedactedParams["runtime_source"] = source
	run.RedactedParams["runtime_name"] = template.Runtime.Name
	run.RedactedParams["runtime_version"] = version
	run.RedactedParams["runtime_os"] = osName
	run.RedactedParams["runtime_arch"] = arch
	if source == "remote" && template.Runtime.Name == "sing-box" && osName == "linux" && arch != "" {
		run.RedactedParams["runtime_download_url"] = deploy.RuntimeDownloadURL(template.Runtime.Name, version, osName, arch)
	}
}

func generateRealityKeyPair() (string, string, error) {
	privateKey := make([]byte, curve25519.ScalarSize)
	if _, err := io.ReadFull(rand.Reader, privateKey); err != nil {
		return "", "", fmt.Errorf("生成 Reality 私钥失败: %w", err)
	}
	privateKey[0] &= 248
	privateKey[31] &= 127
	privateKey[31] |= 64
	publicKey, err := curve25519.X25519(privateKey, curve25519.Basepoint)
	if err != nil {
		return "", "", fmt.Errorf("生成 Reality 公钥失败: %w", err)
	}
	encoding := base64.RawURLEncoding
	return encoding.EncodeToString(privateKey), encoding.EncodeToString(publicKey), nil
}

func randomHexString(bytesLen int) string {
	if bytesLen <= 0 {
		return ""
	}
	data := make([]byte, bytesLen)
	if _, err := io.ReadFull(rand.Reader, data); err != nil {
		return "0123456789abcdef"
	}
	return hex.EncodeToString(data)
}

func completedDeploymentRun(run *models.DeploymentRun) {
	completedAt := time.Now()
	run.Status = models.DeploymentRunStatusSuccess
	run.CompletedAt = &completedAt
}
