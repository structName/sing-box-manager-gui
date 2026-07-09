package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/structName/sing-box-manager-gui/internal/database/models"
	"github.com/structName/sing-box-manager-gui/internal/deploy"
	"github.com/structName/sing-box-manager-gui/internal/storage"
)

type deploymentRunRequest struct {
	TemplateName string                  `json:"template_name"`
	DryRun       bool                    `json:"dry_run"`
	SSH          deploymentSSHRequest    `json:"ssh"`
	Connection   deploymentConnectionReq `json:"connection"`
	Parameters   map[string]interface{}  `json:"parameters"`
}

type deploymentSSHRequest struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	User       string `json:"user"`
	AuthMethod string `json:"auth_method"`
	Password   string `json:"password"`
	PrivateKey string `json:"private_key"`
	Passphrase string `json:"private_key_passphrase"`
}

type deploymentConnectionTestRequest struct {
	SSH        deploymentSSHRequest    `json:"ssh"`
	Connection deploymentConnectionReq `json:"connection"`
}

type deploymentConnectionTestResult struct {
	OK         bool              `json:"ok"`
	Message    string            `json:"message"`
	Host       string            `json:"host"`
	Port       int               `json:"port"`
	User       string            `json:"user"`
	AuthMethod string            `json:"auth_method"`
	DurationMS int64             `json:"duration_ms"`
	Remote     map[string]string `json:"remote,omitempty"`
}

type deploymentConnectionReq struct {
	Mode               string `json:"mode"`
	ManagedCandidateID string `json:"managed_candidate_id"`
	ProxyType          string `json:"proxy_type"`
	ProxyHost          string `json:"proxy_host"`
	ProxyPort          int    `json:"proxy_port"`
	ProxyUsername      string `json:"proxy_username"`
	ProxyPassword      string `json:"proxy_password"`
}

type deploymentRunner interface {
	Run(ctx context.Context, run *models.DeploymentRun, req deploymentRunRequest) error
}

type deploymentSSHTester interface {
	Test(ctx context.Context, req deploymentConnectionTestRequest) (deploymentConnectionTestResult, error)
}

type importDeploymentNodeRequest struct {
	Tag                    string         `json:"tag"`
	SourceName             string         `json:"source_name"`
	Enabled                *bool          `json:"enabled"`
	AdvancedProtocolEdit   bool           `json:"advanced_protocol_edit"`
	GeneratedNodeOverrides models.JSONMap `json:"generated_node_overrides"`
}

type deploymentGeneratedNodeReview struct {
	RunID          string         `json:"run_id"`
	GeneratedNode  models.JSONMap `json:"generated_node"`
	ShareLink      string         `json:"share_link,omitempty"`
	DefaultTag     string         `json:"default_tag"`
	CanImport      bool           `json:"can_import"`
	ImportedNodeID *uint          `json:"imported_node_id,omitempty"`
}

type deploymentRuntimeCacheRequest struct {
	RuntimeName string
	Version     string
	OS          string
	Arch        string
}

type dryRunDeploymentRunner struct {
	checkpoint func(*models.DeploymentRun, int, string, string)
}

func (r dryRunDeploymentRunner) Run(ctx context.Context, run *models.DeploymentRun, req deploymentRunRequest) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	r.checkpointRun(run, 15, "dry_run", "部署 dry-run 已开始")

	switch req.TemplateName {
	case "security-basic":
		run.Stdout = "SBM_PROGRESS {\"step\":\"dry_run\",\"status\":\"success\",\"message\":\"Security-basic request validated\"}\nSBM_RESULT {\"status\":\"success\",\"template\":\"security-basic\",\"base_packages_installed\":false,\"service_restarted\":false,\"ssh_policy_changed\":false}\n"
	default:
		run.Stdout = fmt.Sprintf("SBM_PROGRESS {\"step\":\"dry_run\",\"status\":\"success\",\"message\":\"Deployment request validated\"}\nSBM_NODE_BEGIN\n{\"type\":\"vless\",\"tag\":%q,\"server\":%q,\"server_port\":%d,\"extra\":{\"flow\":\"xtls-rprx-vision\",\"tls\":{\"enabled\":true,\"server_name\":\"www.microsoft.com\",\"reality\":{\"enabled\":true}}}}\nSBM_NODE_END\nSBM_RESULT {\"status\":\"success\",\"privilege_mode\":\"dry-run\",\"systemd\":true}\n",
			stringParam(req.Parameters, "node_name", "deployed-"+run.SSHHost),
			run.NodeServer,
			run.NodeProxyPort,
		)
	}
	parsed, err := deploy.ParseScriptOutput(run.Stdout)
	if err != nil {
		return err
	}
	progress := models.JSONMap{}
	for _, marker := range parsed.Progress {
		if step, ok := marker["step"].(string); ok {
			progress[step] = marker
		}
	}
	run.ProgressMarkers = progress
	run.ProbeResult = models.JSONMap(parsed.Result)
	run.GeneratedNode = models.JSONMap(parsed.GeneratedNode)
	exitCode := 0
	run.ExitCode = &exitCode
	r.checkpointRun(run, 80, "dry_run", "部署 dry-run 输出已解析")
	return nil
}

func (r dryRunDeploymentRunner) checkpointRun(run *models.DeploymentRun, progress int, currentItem, message string) {
	if r.checkpoint != nil {
		r.checkpoint(run, progress, currentItem, message)
	}
}

func (s *Server) deploymentRunnerForRequest(req deploymentRunRequest) deploymentRunner {
	if s.deployRunner == nil {
		if req.DryRun {
			return dryRunDeploymentRunner{checkpoint: s.checkpointDeploymentRun}
		}
		return remoteDeploymentRunner{
			executor:            s.deployScriptExecutor,
			reachabilityChecker: s.deployReachabilityChecker,
			dataDir:             s.deploymentDataDir(),
			checkpoint:          s.checkpointDeploymentRun,
		}
	}
	return s.deployRunner
}

func (s *Server) checkpointDeploymentRun(run *models.DeploymentRun, progress int, currentItem, message string) {
	if s.dbStore != nil {
		_ = s.dbStore.UpdateDeploymentRun(run)
	}
	if s.taskManager != nil && run.TaskID != "" {
		_ = s.taskManager.UpdateProgress(run.TaskID, progress, currentItem, message)
	}
}

func (s *Server) deploymentDataDir() string {
	if strings.TrimSpace(s.baseDir) != "" {
		return s.baseDir
	}
	return "."
}

func (s *Server) ensureDeploymentSSHTester() deploymentSSHTester {
	if s.deploySSHTester == nil {
		s.deploySSHTester = realDeploymentSSHTester{}
	}
	return s.deploySSHTester
}

func (s *Server) registerDeploymentRoutes(api gin.IRoutes) {
	api.GET("/deployments/templates", s.listDeploymentTemplates)
	api.GET("/deployments/connection-candidates", s.listDeploymentConnectionCandidates)
	api.GET("/deployments/runtime-cache", s.listDeploymentRuntimeCache)
	api.POST("/deployments/runtime-cache", s.uploadDeploymentRuntimeCache)
	api.POST("/deployments/connection-test", s.testDeploymentConnection)
	api.POST("/deployments/runs", s.createDeploymentRun)
	api.GET("/deployments/runs", s.listDeploymentRuns)
	api.GET("/deployments/runs/:id", s.getDeploymentRun)
	api.GET("/deployments/runs/:id/generated-node", s.getDeploymentGeneratedNode)
	api.POST("/deployments/runs/:id/cancel", s.cancelDeploymentRun)
	api.POST("/deployments/runs/:id/import-node", s.importDeploymentGeneratedNode)
}

func (s *Server) listDeploymentRuntimeCache(c *gin.Context) {
	req := deploymentRuntimeCacheRequest{
		RuntimeName: strings.TrimSpace(c.DefaultQuery("runtime_name", "sing-box")),
		Version:     strings.TrimSpace(c.DefaultQuery("version", deploy.PinnedSingBoxVersion)),
	}
	if req.RuntimeName == "" || req.Version == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "runtime_name 和 version 为必填"})
		return
	}
	if err := deploy.ValidatePinnedRuntime(req.RuntimeName, req.Version); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	archives, err := deploy.NewRuntimeCache(s.deploymentDataDir()).List(req.RuntimeName, req.Version)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": archives})
}

func (s *Server) uploadDeploymentRuntimeCache(c *gin.Context) {
	req := deploymentRuntimeCacheRequest{
		RuntimeName: strings.TrimSpace(c.DefaultPostForm("runtime_name", "sing-box")),
		Version:     strings.TrimSpace(c.DefaultPostForm("version", deploy.PinnedSingBoxVersion)),
		OS:          strings.TrimSpace(c.DefaultPostForm("os", "linux")),
		Arch:        strings.TrimSpace(c.PostForm("arch")),
	}
	if req.RuntimeName == "" || req.Version == "" || req.OS == "" || req.Arch == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "runtime_name/version/os/arch 为必填"})
		return
	}
	if err := deploy.ValidatePinnedRuntime(req.RuntimeName, req.Version); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file 为必填"})
		return
	}
	if file.Size > 128*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "runtime archive 不能超过 128MB"})
		return
	}
	opened, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer opened.Close()
	content, err := io.ReadAll(io.LimitReader(opened, 128*1024*1024+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(content) > 128*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "runtime archive 不能超过 128MB"})
		return
	}
	cache := deploy.NewRuntimeCache(s.deploymentDataDir())
	path, err := cache.Put(req.RuntimeName, req.Version, req.OS, req.Arch, content)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	expectedChecksum, _ := deploy.RuntimeChecksum(req.RuntimeName, req.Version, req.OS, req.Arch)
	archive, err := cache.Lookup(req.RuntimeName, req.Version, req.OS, req.Arch, expectedChecksum)
	if err != nil {
		_ = os.Remove(path)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	archive.Path = path
	c.JSON(http.StatusOK, gin.H{"data": archive})
}

func (s *Server) listDeploymentTemplates(c *gin.Context) {
	templates := deploy.BuiltinTemplates().List()
	c.JSON(http.StatusOK, gin.H{"data": templates})
}

func (s *Server) listDeploymentConnectionCandidates(c *gin.Context) {
	candidates := s.deploymentConnectionCandidates()
	c.JSON(http.StatusOK, gin.H{"data": candidates})
}

func (s *Server) testDeploymentConnection(c *gin.Context) {
	var req deploymentConnectionTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := normalizeDeploymentSSHRequest(&req.SSH); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Connection.Mode) == "" {
		req.Connection.Mode = models.DeploymentConnectionDirect
	}
	connectionCandidates := s.deploymentConnectionCandidates()
	if err := normalizeDeploymentConnectionRequest(&req.Connection, connectionCandidates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	cleanup, err := s.prepareDeploymentManagedConnection(ctx, &req.Connection)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer cleanup()
	result, err := s.ensureDeploymentSSHTester().Test(ctx, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (s *Server) createDeploymentRun(c *gin.Context) {
	if s.dbStore == nil || s.taskManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "部署模块未初始化"})
		return
	}

	var req deploymentRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.TemplateName) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "template_name 为必填"})
		return
	}
	if err := normalizeDeploymentSSHRequest(&req.SSH); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Connection.Mode) == "" {
		req.Connection.Mode = models.DeploymentConnectionDirect
	}
	connectionCandidates := s.deploymentConnectionCandidates()
	if err := normalizeDeploymentConnectionRequest(&req.Connection, connectionCandidates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateDeploymentParameters(req.Parameters); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	template, ok := deploy.BuiltinTemplates().Get(req.TemplateName)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的部署模板"})
		return
	}

	task, ctx, err := s.taskManager.CreateTask(models.TaskTypeDeploymentRun, "Proxy Node Deployment", models.TaskTriggerManual, 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	run := &models.DeploymentRun{
		ID:                  uuid.New().String(),
		TaskID:              task.ID,
		TemplateName:        req.TemplateName,
		TemplateVersion:     template.Version,
		TemplateChecksum:    template.Checksum,
		Status:              models.DeploymentRunStatusRunning,
		DryRun:              req.DryRun,
		SSHHost:             req.SSH.Host,
		SSHPort:             req.SSH.Port,
		SSHUser:             req.SSH.User,
		AuthMethod:          req.SSH.AuthMethod,
		NodeServer:          stringParam(req.Parameters, "node_server", req.SSH.Host),
		NodeProxyPort:       intParam(req.Parameters, "proxy_port", 443),
		ConnectionMode:      req.Connection.Mode,
		ProxyType:           req.Connection.ProxyType,
		ProxyHost:           req.Connection.ProxyHost,
		ConnectionProxyPort: req.Connection.ProxyPort,
		RedactedParams:      redactedDeploymentParams(req, connectionCandidates),
		StartedAt:           &now,
	}
	if err := s.dbStore.CreateDeploymentRun(run); err != nil {
		_ = s.taskManager.FailTask(task.ID, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := s.taskManager.StartTask(task.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !req.DryRun {
		go s.executeDeploymentRun(ctx, run, req)
		c.JSON(http.StatusAccepted, gin.H{"data": deploymentRunHistoryView(run)})
		return
	}

	if err := s.executeDeploymentRun(ctx, run, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "data": deploymentRunHistoryView(run)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": deploymentRunHistoryView(run)})
}

func (s *Server) executeDeploymentRun(ctx context.Context, run *models.DeploymentRun, req deploymentRunRequest) error {
	if !req.DryRun {
		cleanup, err := s.prepareDeploymentManagedConnection(ctx, &req.Connection)
		if err != nil {
			completedAt := time.Now()
			run.CompletedAt = &completedAt
			run.Stderr = err.Error()
			if ctx.Err() != nil {
				run.Status = models.DeploymentRunStatusCancelled
				_ = s.dbStore.UpdateDeploymentRun(run)
				return err
			}
			run.Status = models.DeploymentRunStatusFailed
			_ = s.dbStore.UpdateDeploymentRun(run)
			_ = s.taskManager.FailTask(run.TaskID, err.Error())
			return err
		}
		defer cleanup()
	}
	if err := s.deploymentRunnerForRequest(req).Run(ctx, run, req); err != nil {
		completedAt := time.Now()
		run.CompletedAt = &completedAt
		run.Stderr = err.Error()
		if ctx.Err() != nil {
			run.Status = models.DeploymentRunStatusCancelled
			_ = s.dbStore.UpdateDeploymentRun(run)
			return err
		}
		run.Status = models.DeploymentRunStatusFailed
		_ = s.dbStore.UpdateDeploymentRun(run)
		_ = s.taskManager.FailTask(run.TaskID, err.Error())
		return err
	}

	completedDeploymentRun(run)
	if err := s.dbStore.UpdateDeploymentRun(run); err != nil {
		_ = s.taskManager.FailTask(run.TaskID, err.Error())
		return err
	}
	message := "部署已完成"
	if req.DryRun {
		message = "部署 dry-run 已完成"
	}
	_ = s.taskManager.CompleteTask(run.TaskID, message, map[string]interface{}{"deployment_run_id": run.ID})
	return nil
}

func (s *Server) listDeploymentRuns(c *gin.Context) {
	if s.dbStore == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未初始化"})
		return
	}

	limit := 50
	offset := 0
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	runs, err := s.dbStore.GetDeploymentRuns(limit, offset, c.Query("status"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": deploymentRunHistoryViews(runs)})
}

func (s *Server) getDeploymentRun(c *gin.Context) {
	if s.dbStore == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未初始化"})
		return
	}

	run, err := s.dbStore.GetDeploymentRun(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "部署记录不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": deploymentRunHistoryView(run)})
}

func (s *Server) getDeploymentGeneratedNode(c *gin.Context) {
	if s.dbStore == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未初始化"})
		return
	}

	run, err := s.dbStore.GetDeploymentRun(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "部署记录不存在"})
		return
	}
	if run.Status != models.DeploymentRunStatusSuccess {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只能查看成功部署生成的节点"})
		return
	}
	if run.DryRun {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dry-run 生成节点仅用于预览"})
		return
	}
	if len(run.GeneratedNode) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "部署记录没有生成节点"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": deploymentGeneratedNodeReview{
		RunID:          run.ID,
		GeneratedNode:  copyDeploymentJSONMap(run.GeneratedNode),
		ShareLink:      deploymentGeneratedNodeShareLink(run.GeneratedNode),
		DefaultTag:     stringFromMap(run.GeneratedNode, "tag"),
		CanImport:      run.ImportedNodeID == nil,
		ImportedNodeID: run.ImportedNodeID,
	}})
}

func deploymentRunHistoryViews(runs []models.DeploymentRun) []models.DeploymentRun {
	views := make([]models.DeploymentRun, len(runs))
	for i := range runs {
		views[i] = deploymentRunHistoryView(&runs[i])
	}
	return views
}

func deploymentRunHistoryView(run *models.DeploymentRun) models.DeploymentRun {
	if run == nil {
		return models.DeploymentRun{}
	}
	view := *run
	view.RedactedParams = redactDeploymentHistoryParams(run.RedactedParams)
	view.GeneratedNode = redactDeploymentJSONMap(run.GeneratedNode)
	view.Stdout = redactDeploymentLog(run.Stdout)
	view.Stderr = redactDeploymentLog(run.Stderr)
	return view
}

func redactDeploymentHistoryParams(values models.JSONMap) models.JSONMap {
	if values == nil {
		return nil
	}
	redacted := redactDeploymentJSONMap(values)
	if candidateID, ok := redacted["managed_candidate_id"]; ok && strings.TrimSpace(fmt.Sprint(candidateID)) != "" {
		redacted["managed_candidate_id"] = "[REDACTED]"
	}
	return redacted
}

func redactDeploymentLog(text string) string {
	if text == "" {
		return ""
	}
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	var output strings.Builder
	var node strings.Builder
	inNode := false

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "SBM_NODE_BEGIN":
			inNode = true
			node.Reset()
			output.WriteString(line)
			output.WriteByte('\n')
		case line == "SBM_NODE_END":
			if inNode {
				output.WriteString(redactedDeploymentNodeLogBlock(node.String()))
				output.WriteByte('\n')
			}
			inNode = false
			output.WriteString(line)
			output.WriteByte('\n')
		case inNode:
			node.WriteString(line)
			node.WriteByte('\n')
		case strings.HasPrefix(line, "SBM_PROGRESS "):
			output.WriteString(redactedDeploymentMarkerLine(line, "SBM_PROGRESS "))
			output.WriteByte('\n')
		case strings.HasPrefix(line, "SBM_RESULT "):
			output.WriteString(redactedDeploymentMarkerLine(line, "SBM_RESULT "))
			output.WriteByte('\n')
		default:
			output.WriteString(line)
			output.WriteByte('\n')
		}
	}
	if inNode {
		output.WriteString(redactedDeploymentNodeLogBlock(node.String()))
		output.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		output.WriteString("[deployment log redacted: scanner error]\n")
	}
	return output.String()
}

func redactedDeploymentNodeLogBlock(raw string) string {
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return `{"redacted":"[REDACTED]"}`
	}
	data, err := json.Marshal(redactDeploymentMap(decoded))
	if err != nil {
		return `{"redacted":"[REDACTED]"}`
	}
	return string(data)
}

func redactedDeploymentMarkerLine(line, prefix string) string {
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, prefix))), &decoded); err != nil {
		return prefix + `{"redacted":"[REDACTED]"}`
	}
	data, err := json.Marshal(redactDeploymentMap(decoded))
	if err != nil {
		return prefix + `{"redacted":"[REDACTED]"}`
	}
	return prefix + string(data)
}

func redactDeploymentJSONMap(values models.JSONMap) models.JSONMap {
	if values == nil {
		return nil
	}
	redacted := models.JSONMap{}
	for key, value := range values {
		if isSensitiveDeploymentKey(key) {
			redacted[key] = "[REDACTED]"
			continue
		}
		redacted[key] = redactDeploymentValue(value)
	}
	return redacted
}

func redactDeploymentMap(values map[string]interface{}) map[string]interface{} {
	if values == nil {
		return nil
	}
	redacted := map[string]interface{}{}
	for key, value := range values {
		if isSensitiveDeploymentKey(key) {
			redacted[key] = "[REDACTED]"
			continue
		}
		redacted[key] = redactDeploymentValue(value)
	}
	return redacted
}

func redactDeploymentValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return redactDeploymentMap(typed)
	case models.JSONMap:
		return redactDeploymentJSONMap(typed)
	case []interface{}:
		redacted := make([]interface{}, len(typed))
		for i, item := range typed {
			redacted[i] = redactDeploymentValue(item)
		}
		return redacted
	default:
		return typed
	}
}

func (s *Server) cancelDeploymentRun(c *gin.Context) {
	if s.dbStore == nil || s.taskManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "部署模块未初始化"})
		return
	}

	run, err := s.dbStore.GetDeploymentRun(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "部署记录不存在"})
		return
	}
	if run.Status != models.DeploymentRunStatusRunning && run.Status != models.DeploymentRunStatusPending {
		c.JSON(http.StatusBadRequest, gin.H{"error": "部署记录已结束"})
		return
	}
	if err := s.taskManager.CancelTask(run.TaskID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	completedAt := time.Now()
	run.Status = models.DeploymentRunStatusCancelled
	run.CompletedAt = &completedAt
	if err := s.dbStore.UpdateDeploymentRun(run); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": deploymentRunHistoryView(run)})
}

func (s *Server) importDeploymentGeneratedNode(c *gin.Context) {
	if s.dbStore == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "部署模块未初始化"})
		return
	}

	run, err := s.dbStore.GetDeploymentRun(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "部署记录不存在"})
		return
	}
	if run.Status != models.DeploymentRunStatusSuccess {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只能导入成功部署生成的节点"})
		return
	}
	if run.DryRun {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dry-run 生成节点仅用于预览，不能导入"})
		return
	}
	if run.ImportedNodeID != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "部署生成节点已导入"})
		return
	}
	if len(run.GeneratedNode) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "部署记录没有生成节点"})
		return
	}
	if s.store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "部署模块未初始化"})
		return
	}

	var req importDeploymentNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	generatedNode, err := deploymentGeneratedNodeForImport(run.GeneratedNode, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	manualNode, err := deploymentGeneratedNodeToManualNode(run, req, generatedNode)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if deploymentNodeTagExists(s.store, manualNode.Node.Tag) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "节点 tag 已存在"})
		return
	}
	if err := s.store.AddManualNode(manualNode); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := s.syncNodesToSQLite(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	node, err := s.dbStore.GetNodeByTag(manualNode.Node.Tag)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	run.ImportedNodeID = &node.ID
	if err := s.dbStore.UpdateDeploymentRun(run); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": node})
}

func deploymentNodeTagExists(store *storage.JSONStore, tag string) bool {
	if store == nil {
		return false
	}
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return false
	}
	for _, manualNode := range store.GetManualNodes() {
		if strings.TrimSpace(manualNode.Node.Tag) == tag {
			return true
		}
	}
	for _, sub := range store.GetSubscriptions() {
		for _, node := range sub.Nodes {
			if strings.TrimSpace(node.Tag) == tag {
				return true
			}
		}
	}
	for _, node := range store.GetAllNodes() {
		if strings.TrimSpace(node.Tag) == tag {
			return true
		}
	}
	return false
}

func deploymentGeneratedNodeForImport(generated models.JSONMap, req importDeploymentNodeRequest) (models.JSONMap, error) {
	if len(req.GeneratedNodeOverrides) > 0 && !req.AdvancedProtocolEdit {
		return nil, fmt.Errorf("协议字段修改需要显式开启 advanced_protocol_edit")
	}
	next := copyDeploymentJSONMap(generated)
	if req.AdvancedProtocolEdit {
		for key, value := range req.GeneratedNodeOverrides {
			next[key] = value
		}
	}
	return next, nil
}

func deploymentGeneratedNodeShareLink(generated models.JSONMap) string {
	if !strings.EqualFold(stringFromMap(generated, "type"), "vless") {
		return ""
	}
	server := stringFromMap(generated, "server")
	port := intFromMap(generated, "server_port")
	extra := deploymentMapValue(generated["extra"])
	uuid := stringFromMap(extra, "uuid")
	if server == "" || port <= 0 || uuid == "" {
		return ""
	}

	params := url.Values{}
	params.Set("encryption", "none")
	params.Set("type", "tcp")
	if flow := stringFromMap(extra, "flow"); flow != "" {
		params.Set("flow", flow)
	}
	tls := deploymentMapValue(extra["tls"])
	if len(tls) > 0 {
		if enabled, ok := tls["enabled"].(bool); ok && enabled {
			params.Set("security", "tls")
		}
		if serverName := stringFromMap(tls, "server_name"); serverName != "" {
			params.Set("sni", serverName)
		}
		utls := deploymentMapValue(tls["utls"])
		if fingerprint := stringFromMap(utls, "fingerprint"); fingerprint != "" {
			params.Set("fp", fingerprint)
		}
		reality := deploymentMapValue(tls["reality"])
		if len(reality) > 0 {
			publicKey := stringFromMap(reality, "public_key")
			shortID := stringFromMap(reality, "short_id")
			if enabled, ok := reality["enabled"].(bool); ok && enabled || publicKey != "" || shortID != "" {
				params.Set("security", "reality")
			}
			if publicKey != "" {
				params.Set("pbk", publicKey)
			}
			if shortID != "" {
				params.Set("sid", shortID)
			}
		}
	}
	if params.Get("security") == "" {
		params.Set("security", "none")
	}

	link := url.URL{
		Scheme:   "vless",
		User:     url.User(uuid),
		Host:     net.JoinHostPort(server, strconv.Itoa(port)),
		RawQuery: params.Encode(),
		Fragment: stringFromMap(generated, "tag"),
	}
	return link.String()
}

func deploymentMapValue(value interface{}) models.JSONMap {
	switch typed := value.(type) {
	case models.JSONMap:
		return typed
	case map[string]interface{}:
		next := models.JSONMap{}
		for key, child := range typed {
			next[key] = child
		}
		return next
	default:
		return models.JSONMap{}
	}
}

func copyDeploymentJSONMap(values models.JSONMap) models.JSONMap {
	copied := models.JSONMap{}
	for key, value := range values {
		copied[key] = value
	}
	return copied
}

func deploymentGeneratedNodeToManualNode(run *models.DeploymentRun, req importDeploymentNodeRequest, generatedNode models.JSONMap) (storage.ManualNode, error) {
	tag := strings.TrimSpace(req.Tag)
	if tag == "" {
		tag = stringFromMap(generatedNode, "tag")
	}
	if tag == "" {
		return storage.ManualNode{}, fmt.Errorf("节点 tag 为必填")
	}
	sourceName := strings.TrimSpace(req.SourceName)
	if sourceName == "" {
		sourceName = "手动添加"
	}
	nodeType := stringFromMap(generatedNode, "type")
	server := stringFromMap(generatedNode, "server")
	port := intFromMap(generatedNode, "server_port")
	if nodeType == "" || server == "" || port == 0 {
		return storage.ManualNode{}, fmt.Errorf("生成节点缺少 type/server/server_port")
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	extra := models.JSONMap{}
	if rawExtra, ok := generatedNode["extra"].(map[string]interface{}); ok {
		for key, value := range rawExtra {
			extra[key] = value
		}
	}
	extra["node_origin"] = "deployed_self_hosted"
	extra["entry_method"] = "deployment_import"
	extra["deployment_run_id"] = run.ID

	return storage.ManualNode{
		ID: uuid.New().String(),
		Node: storage.Node{
			Tag:        tag,
			Type:       nodeType,
			Server:     server,
			ServerPort: port,
			Source:     "manual",
			SourceName: sourceName,
			Extra:      extra,
		},
		Enabled: enabled,
	}, nil
}

func redactedDeploymentParams(req deploymentRunRequest, candidates []deploy.ConnectionCandidate) models.JSONMap {
	params := models.JSONMap{}
	params["ssh_host"] = req.SSH.Host
	params["ssh_port"] = req.SSH.Port
	params["ssh_user"] = req.SSH.User
	params["auth_method"] = req.SSH.AuthMethod
	params["connection_mode"] = req.Connection.Mode
	addRedactedDeploymentManagedCandidateParams(params, req.Connection.ManagedCandidateID, candidates)
	params["proxy_type"] = req.Connection.ProxyType
	params["proxy_host"] = req.Connection.ProxyHost
	params["connection_proxy_port"] = req.Connection.ProxyPort
	params["proxy_username"] = req.Connection.ProxyUsername
	params["node_proxy_port"] = intParam(req.Parameters, "proxy_port", 443)

	if req.SSH.Password != "" {
		params["ssh_password"] = "[REDACTED]"
	}
	if req.SSH.PrivateKey != "" {
		params["ssh_private_key"] = "[REDACTED]"
	}
	if req.SSH.Passphrase != "" {
		params["ssh_private_key_passphrase"] = "[REDACTED]"
	}
	if req.Connection.ProxyPassword != "" {
		params["proxy_password"] = "[REDACTED]"
	}
	for key, value := range req.Parameters {
		if isSensitiveDeploymentKey(key) {
			params[key] = "[REDACTED]"
			continue
		}
		params[key] = value
	}
	return params
}

func addRedactedDeploymentManagedCandidateParams(params models.JSONMap, candidateID string, candidates []deploy.ConnectionCandidate) {
	candidateID = strings.TrimSpace(candidateID)
	if candidateID == "" {
		params["managed_candidate_id"] = ""
		return
	}
	params["managed_candidate_id"] = "[REDACTED]"
	params["managed_candidate_route"] = "unknown"
	if candidate, ok := deploymentCandidateByID(candidates, candidateID); ok {
		params["managed_candidate_kind"] = candidate.Kind
		if strings.TrimSpace(candidate.LocalEndpoint) != "" {
			params["managed_candidate_route"] = "local_endpoint"
		} else if candidate.RequiresTemporaryEntrypoint {
			params["managed_candidate_route"] = "temporary_entrypoint"
		}
	}
}

func stringFromMap(values map[string]interface{}, key string) string {
	value, ok := values[key]
	if !ok {
		return ""
	}
	text, _ := value.(string)
	return text
}

func intFromMap(values map[string]interface{}, key string) int {
	value, ok := values[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	case string:
		parsed, _ := strconv.Atoi(typed)
		return parsed
	default:
		return 0
	}
}

func isSensitiveDeploymentKey(key string) bool {
	upper := strings.ToUpper(strings.TrimSpace(key))
	for _, suffix := range []string{"_PASSWORD", "_SECRET", "_TOKEN", "_PRIVATE_KEY", "_UUID", "_KEY", "_SHORT_ID"} {
		if strings.HasSuffix(upper, suffix) {
			return true
		}
	}
	return upper == "PASSWORD" || upper == "SECRET" || upper == "TOKEN" || upper == "UUID" || upper == "KEY" || upper == "SHORT_ID"
}

func normalizeDeploymentSSHRequest(req *deploymentSSHRequest) error {
	req.Host = strings.TrimSpace(req.Host)
	if req.Host == "" {
		return fmt.Errorf("ssh.host 为必填")
	}
	if req.Port == 0 {
		req.Port = 22
	}
	if req.Port < 1 || req.Port > 65535 {
		return fmt.Errorf("ssh.port 必须在 1-65535 之间")
	}
	req.User = strings.TrimSpace(req.User)
	if req.User == "" {
		req.User = "root"
	}
	req.AuthMethod = strings.TrimSpace(req.AuthMethod)
	if req.AuthMethod == "" {
		req.AuthMethod = "password"
	}
	switch req.AuthMethod {
	case "password":
		if strings.TrimSpace(req.Password) == "" {
			return fmt.Errorf("ssh.password 为必填")
		}
		if strings.TrimSpace(req.PrivateKey) != "" || strings.TrimSpace(req.Passphrase) != "" {
			return fmt.Errorf("ssh.password 与 ssh.private_key 不能同时提供")
		}
	case "private_key":
		if strings.TrimSpace(req.PrivateKey) == "" {
			return fmt.Errorf("ssh.private_key 为必填")
		}
		if strings.TrimSpace(req.Password) != "" {
			return fmt.Errorf("ssh.password 与 ssh.private_key 不能同时提供")
		}
	default:
		return fmt.Errorf("不支持的 SSH 认证方式")
	}
	return nil
}

func (s *Server) deploymentConnectionCandidates() []deploy.ConnectionCandidate {
	if s.store == nil {
		return []deploy.ConnectionCandidate{}
	}
	return deploy.DiscoverConnectionCandidates(deploy.CandidateInventoryInput{
		Settings:       s.store.GetSettings(),
		Nodes:          s.store.GetAllNodes(),
		ProxyChains:    s.store.GetProxyChains(),
		InboundPorts:   s.store.GetInboundPorts(),
		ServiceRunning: s.deploymentServiceRunning(),
	})
}

func (s *Server) deploymentServiceRunning() bool {
	if s.deployServiceRunning != nil {
		return *s.deployServiceRunning
	}
	return s.processManager != nil && s.processManager.IsRunning()
}

func normalizeDeploymentConnectionRequest(req *deploymentConnectionReq, candidates []deploy.ConnectionCandidate) error {
	req.Mode = strings.TrimSpace(req.Mode)
	if req.Mode == "" {
		req.Mode = models.DeploymentConnectionDirect
	}
	switch req.Mode {
	case models.DeploymentConnectionDirect:
		req.ManagedCandidateID = ""
		req.ProxyType = ""
		req.ProxyHost = ""
		req.ProxyPort = 0
		req.ProxyUsername = ""
		req.ProxyPassword = ""
		return nil
	case models.DeploymentConnectionCustomProxy:
		return normalizeDeploymentCustomProxy(req)
	case models.DeploymentConnectionManagedProxy:
		return normalizeDeploymentManagedProxy(req, candidates)
	default:
		return fmt.Errorf("不支持的连接模式")
	}
}

func normalizeDeploymentCustomProxy(req *deploymentConnectionReq) error {
	req.ProxyType = strings.ToLower(strings.TrimSpace(req.ProxyType))
	switch req.ProxyType {
	case "socks5", "http_connect":
	default:
		return fmt.Errorf("proxy_type 仅支持 socks5 或 http_connect")
	}
	req.ProxyHost = strings.TrimSpace(req.ProxyHost)
	if req.ProxyHost == "" {
		return fmt.Errorf("proxy_host 为必填")
	}
	if req.ProxyPort < 1 || req.ProxyPort > 65535 {
		return fmt.Errorf("proxy_port 必须在 1-65535 之间")
	}
	req.ProxyUsername = strings.TrimSpace(req.ProxyUsername)
	return nil
}

func normalizeDeploymentManagedProxy(req *deploymentConnectionReq, candidates []deploy.ConnectionCandidate) error {
	req.ManagedCandidateID = strings.TrimSpace(req.ManagedCandidateID)
	if req.ManagedCandidateID == "" {
		return fmt.Errorf("managed_candidate_id 为必填")
	}
	for _, candidate := range candidates {
		if candidate.ID != req.ManagedCandidateID {
			continue
		}
		if !candidate.Available {
			if candidate.UnavailableReason != "" {
				return fmt.Errorf("托管路线不可用: %s", candidate.UnavailableReason)
			}
			return fmt.Errorf("托管路线不可用")
		}
		if strings.TrimSpace(candidate.LocalEndpoint) == "" {
			if candidate.RequiresTemporaryEntrypoint {
				req.ProxyType = ""
				req.ProxyHost = ""
				req.ProxyPort = 0
				req.ProxyUsername = ""
				req.ProxyPassword = ""
				return nil
			}
			return fmt.Errorf("托管路线缺少本地入口")
		}
		host, portText, err := net.SplitHostPort(candidate.LocalEndpoint)
		if err != nil {
			return fmt.Errorf("托管路线入口无效")
		}
		port, err := strconv.Atoi(portText)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("托管路线入口端口无效")
		}
		req.ProxyType = "socks5"
		req.ProxyHost = strings.TrimSpace(host)
		req.ProxyPort = port
		req.ProxyUsername = ""
		req.ProxyPassword = ""
		return nil
	}
	return fmt.Errorf("托管路线不存在")
}

func validateDeploymentParameters(params map[string]interface{}) error {
	proxyPort, err := portParam(params, "proxy_port", 443)
	if err != nil {
		return fmt.Errorf("parameters.proxy_port 必须在 1-65535 之间")
	}
	if proxyPort < 1 || proxyPort > 65535 {
		return fmt.Errorf("parameters.proxy_port 必须在 1-65535 之间")
	}
	if params != nil {
		if value, ok := params["runtime_version"]; ok && value != nil {
			runtimeVersion, ok := value.(string)
			if !ok {
				return fmt.Errorf("parameters.runtime_version 仅支持固定版本 %s", deploy.PinnedSingBoxVersion)
			}
			runtimeVersion = strings.TrimSpace(runtimeVersion)
			if runtimeVersion != "" && runtimeVersion != deploy.PinnedSingBoxVersion {
				return fmt.Errorf("parameters.runtime_version 仅支持固定版本 %s", deploy.PinnedSingBoxVersion)
			}
		}
		if value, ok := params["runtime_source"]; ok && value != nil {
			runtimeSource, ok := value.(string)
			if !ok {
				return fmt.Errorf("parameters.runtime_source 仅支持 remote 或 cache")
			}
			runtimeSource = strings.TrimSpace(runtimeSource)
			if runtimeSource != "" && runtimeSource != "remote" && runtimeSource != "cache" {
				return fmt.Errorf("parameters.runtime_source 仅支持 remote 或 cache")
			}
		}
	}
	return nil
}

func portParam(params map[string]interface{}, key string, fallback int) (int, error) {
	if params == nil {
		return fallback, nil
	}
	value, ok := params[key]
	if !ok || value == nil {
		return fallback, nil
	}
	switch typed := value.(type) {
	case int:
		return typed, nil
	case float64:
		if typed != float64(int(typed)) {
			return 0, fmt.Errorf("端口必须是整数")
		}
		return int(typed), nil
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return fallback, nil
		}
		parsed, err := strconv.Atoi(text)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("端口类型无效")
	}
}

func stringParam(params map[string]interface{}, key, fallback string) string {
	if params == nil {
		return fallback
	}
	value, ok := params[key]
	if !ok {
		return fallback
	}
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return fallback
	}
	return text
}

func intParam(params map[string]interface{}, key string, fallback int) int {
	if params == nil {
		return fallback
	}
	value, ok := params[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case int:
		if typed > 0 {
			return typed
		}
	case float64:
		if typed > 0 {
			return int(typed)
		}
	case string:
		parsed, err := strconv.Atoi(typed)
		if err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}

func boolParam(params map[string]interface{}, key string, fallback bool) bool {
	if params == nil {
		return fallback
	}
	value, ok := params[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return fallback
		}
		return strings.EqualFold(text, "true")
	default:
		return fallback
	}
}

func copyDeploymentParameters(params map[string]interface{}) map[string]interface{} {
	copied := make(map[string]interface{}, len(params))
	for key, value := range params {
		copied[key] = value
	}
	return copied
}
