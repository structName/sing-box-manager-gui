package api

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/structName/sing-box-manager-gui/internal/database"
	"github.com/structName/sing-box-manager-gui/internal/database/models"
	"github.com/structName/sing-box-manager-gui/internal/deploy"
	"github.com/structName/sing-box-manager-gui/internal/service"
	"github.com/structName/sing-box-manager-gui/internal/storage"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

func TestCreateDryRunDeploymentStoresRedactedRunHistory(t *testing.T) {
	store := newDeploymentTestStore(t)
	taskManager := service.NewTaskManager(store)
	server := &Server{
		dbStore:     store,
		taskManager: taskManager,
		taskHandler: NewTaskHandler(store, taskManager),
		router:      gin.New(),
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"template_name":"singbox-vless-reality",
		"dry_run":true,
		"ssh":{"host":"203.0.113.10","port":22,"user":"root","auth_method":"password","password":"ssh-secret"},
		"connection":{"mode":"custom_proxy","proxy_type":"socks5","proxy_host":"127.0.0.1","proxy_port":1080,"proxy_username":"ops","proxy_password":"proxy-secret"},
		"parameters":{"node_name":"edge-a","node_server":"vpn.example.com","proxy_port":443,"sbm_uuid":"secret-uuid","reality_short_id":"short-secret","api_token":"token-secret"}
	}`)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("POST /deployments/runs status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var createResp struct {
		Data models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createResp.Data.ID == "" {
		t.Fatal("expected deployment run id")
	}
	if createResp.Data.TaskID == "" {
		t.Fatal("expected linked task id")
	}
	task, err := store.GetTask(createResp.Data.TaskID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if task.Status != models.TaskStatusCompleted || task.Progress != task.Total || task.CurrentItem != "dry_run" {
		t.Fatalf("dry-run task progress was not recorded: %#v", task)
	}
	if task.Message == "" || !strings.Contains(task.Message, "dry-run") {
		t.Fatalf("dry-run task message missing: %#v", task)
	}
	if createResp.Data.Status != models.DeploymentRunStatusSuccess {
		t.Fatalf("status = %q, want %q", createResp.Data.Status, models.DeploymentRunStatusSuccess)
	}
	if createResp.Data.TemplateVersion == "" || createResp.Data.TemplateChecksum == "" {
		t.Fatalf("template identity was not recorded: %#v", createResp.Data)
	}
	if createResp.Data.SSHHost != "203.0.113.10" || createResp.Data.NodeServer != "vpn.example.com" {
		t.Fatalf("target metadata was not preserved: %#v", createResp.Data)
	}
	assertNoSecretValue(t, createResp.Data.RedactedParams, "ssh-secret", "proxy-secret", "secret-uuid", "short-secret", "token-secret")
	if createResp.Data.RedactedParams["ssh_password"] != "[REDACTED]" {
		t.Fatalf("ssh password not redacted in params: %#v", createResp.Data.RedactedParams)
	}
	if createResp.Data.RedactedParams["proxy_password"] != "[REDACTED]" {
		t.Fatalf("proxy password not redacted in params: %#v", createResp.Data.RedactedParams)
	}
	if createResp.Data.RedactedParams["sbm_uuid"] != "[REDACTED]" {
		t.Fatalf("uuid-like parameter not redacted: %#v", createResp.Data.RedactedParams)
	}
	if createResp.Data.RedactedParams["reality_short_id"] != "[REDACTED]" {
		t.Fatalf("short-id parameter not redacted: %#v", createResp.Data.RedactedParams)
	}
	if createResp.Data.RedactedParams["node_proxy_port"] != float64(443) || createResp.Data.RedactedParams["connection_proxy_port"] != float64(1080) {
		t.Fatalf("node/connection proxy ports were not disambiguated: %#v", createResp.Data.RedactedParams)
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/deployments/runs", nil)
	server.router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("GET /deployments/runs status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}

	var listResp struct {
		Data []models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResp.Data) != 1 {
		t.Fatalf("expected one deployment run, got %d", len(listResp.Data))
	}
	assertNoSecretValue(t, listResp.Data[0].RedactedParams, "ssh-secret", "proxy-secret", "secret-uuid", "short-secret", "token-secret")
}

func TestCreateDryRunDeploymentRedactsPrivateKeyPassphrase(t *testing.T) {
	store := newDeploymentTestStore(t)
	taskManager := service.NewTaskManager(store)
	server := &Server{
		dbStore:     store,
		taskManager: taskManager,
		taskHandler: NewTaskHandler(store, taskManager),
		router:      gin.New(),
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"template_name":"singbox-vless-reality",
		"dry_run":true,
		"ssh":{"host":"203.0.113.10","port":22,"user":"root","auth_method":"private_key","private_key":"-----BEGIN OPENSSH PRIVATE KEY-----\nprivate-key-secret\n-----END OPENSSH PRIVATE KEY-----","private_key_passphrase":"passphrase-secret"},
		"connection":{"mode":"direct"},
		"parameters":{"node_name":"edge-a","node_server":"vpn.example.com","proxy_port":443}
	}`)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("POST /deployments/runs status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var createResp struct {
		Data models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	assertNoSecretValue(t, createResp.Data.RedactedParams, "private-key-secret", "passphrase-secret")
	if createResp.Data.RedactedParams["ssh_private_key"] != "[REDACTED]" {
		t.Fatalf("ssh private key not redacted in params: %#v", createResp.Data.RedactedParams)
	}
	if createResp.Data.RedactedParams["ssh_private_key_passphrase"] != "[REDACTED]" {
		t.Fatalf("ssh private key passphrase not redacted in params: %#v", createResp.Data.RedactedParams)
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/deployments/runs", nil)
	server.router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("GET /deployments/runs status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	var listResp struct {
		Data []models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResp.Data) != 1 {
		t.Fatalf("expected one deployment run, got %d", len(listResp.Data))
	}
	assertNoSecretValue(t, listResp.Data[0].RedactedParams, "private-key-secret", "passphrase-secret")
}

func TestCreateDryRunDeploymentRedactsManagedCandidateIdentity(t *testing.T) {
	store := newDeploymentTestStore(t)
	jsonStore, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}
	const sensitiveRoute = "日本BGP-secret-route"
	if err := jsonStore.AddManualNode(storage.ManualNode{
		ID: "manual-sensitive-route",
		Node: storage.Node{
			Tag:        sensitiveRoute,
			Type:       "socks",
			Server:     "127.0.0.1",
			ServerPort: 1080,
		},
		Enabled: true,
	}); err != nil {
		t.Fatalf("AddManualNode() error = %v", err)
	}
	taskManager := service.NewTaskManager(store)
	server := &Server{
		dbStore:     store,
		store:       jsonStore,
		taskManager: taskManager,
		taskHandler: NewTaskHandler(store, taskManager),
		router:      gin.New(),
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"template_name":"singbox-vless-reality",
		"dry_run":true,
		"ssh":{"host":"203.0.113.10","port":22,"user":"root","auth_method":"password","password":"ssh-secret"},
		"connection":{"mode":"managed_proxy","managed_candidate_id":"node:` + sensitiveRoute + `"},
		"parameters":{"node_name":"edge-a","node_server":"vpn.example.com","proxy_port":443}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("POST /deployments/runs status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if bytes.Contains(recorder.Body.Bytes(), []byte(sensitiveRoute)) {
		t.Fatalf("create response leaked managed candidate identity: %s", recorder.Body.String())
	}
	var createResp struct {
		Data models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createResp.Data.RedactedParams["managed_candidate_id"] != "[REDACTED]" ||
		createResp.Data.RedactedParams["managed_candidate_kind"] != deploy.CandidateKindNode ||
		createResp.Data.RedactedParams["managed_candidate_route"] != "temporary_entrypoint" {
		t.Fatalf("managed candidate history was not redacted with route metadata: %#v", createResp.Data.RedactedParams)
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/deployments/runs", nil)
	server.router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("GET /deployments/runs status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	if bytes.Contains(listRecorder.Body.Bytes(), []byte(sensitiveRoute)) {
		t.Fatalf("list response leaked managed candidate identity: %s", listRecorder.Body.String())
	}

	detailRecorder := httptest.NewRecorder()
	detailRequest := httptest.NewRequest(http.MethodGet, "/api/deployments/runs/"+createResp.Data.ID, nil)
	server.router.ServeHTTP(detailRecorder, detailRequest)
	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("GET /deployments/runs/:id status = %d, body = %s", detailRecorder.Code, detailRecorder.Body.String())
	}
	if bytes.Contains(detailRecorder.Body.Bytes(), []byte(sensitiveRoute)) {
		t.Fatalf("detail response leaked managed candidate identity: %s", detailRecorder.Body.String())
	}
}

func TestDeploymentRunHistoryViewRedactsLegacyManagedCandidateIdentity(t *testing.T) {
	store := newDeploymentTestStore(t)
	server := &Server{
		dbStore: store,
		router:  gin.New(),
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	const sensitiveRoute = "node:日本BGP-legacy-route"
	if err := store.CreateDeploymentRun(&models.DeploymentRun{
		ID:             "run-legacy-managed-candidate",
		TemplateName:   "singbox-vless-reality",
		Status:         models.DeploymentRunStatusSuccess,
		ConnectionMode: models.DeploymentConnectionManagedProxy,
		RedactedParams: models.JSONMap{
			"managed_candidate_id": sensitiveRoute,
			"connection_mode":      models.DeploymentConnectionManagedProxy,
		},
	}); err != nil {
		t.Fatalf("CreateDeploymentRun() error = %v", err)
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/deployments/runs", nil)
	server.router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("GET /deployments/runs status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	if bytes.Contains(listRecorder.Body.Bytes(), []byte(sensitiveRoute)) {
		t.Fatalf("list response leaked legacy managed candidate identity: %s", listRecorder.Body.String())
	}

	var listResp struct {
		Data []models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResp.Data) != 1 || listResp.Data[0].RedactedParams["managed_candidate_id"] != "[REDACTED]" {
		t.Fatalf("legacy managed candidate id was not redacted in list: %#v", listResp.Data)
	}

	detailRecorder := httptest.NewRecorder()
	detailRequest := httptest.NewRequest(http.MethodGet, "/api/deployments/runs/run-legacy-managed-candidate", nil)
	server.router.ServeHTTP(detailRecorder, detailRequest)
	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("GET /deployments/runs/:id status = %d, body = %s", detailRecorder.Code, detailRecorder.Body.String())
	}
	if bytes.Contains(detailRecorder.Body.Bytes(), []byte(sensitiveRoute)) {
		t.Fatalf("detail response leaked legacy managed candidate identity: %s", detailRecorder.Body.String())
	}
}

func TestListDeploymentRunsFiltersStatusNewestFirst(t *testing.T) {
	store := newDeploymentTestStore(t)
	server := &Server{
		dbStore: store,
		router:  gin.New(),
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	now := time.Now()
	runs := []models.DeploymentRun{
		{ID: "old-success", TemplateName: "singbox-vless-reality", Status: models.DeploymentRunStatusSuccess, CreatedAt: now.Add(-3 * time.Hour)},
		{ID: "mid-failed", TemplateName: "singbox-vless-reality", Status: models.DeploymentRunStatusFailed, CreatedAt: now.Add(-2 * time.Hour)},
		{ID: "new-failed", TemplateName: "singbox-vless-reality", Status: models.DeploymentRunStatusFailed, CreatedAt: now.Add(-1 * time.Hour)},
	}
	for i := range runs {
		if err := store.CreateDeploymentRun(&runs[i]); err != nil {
			t.Fatalf("CreateDeploymentRun(%s) error = %v", runs[i].ID, err)
		}
	}

	allRecorder := httptest.NewRecorder()
	allRequest := httptest.NewRequest(http.MethodGet, "/api/deployments/runs", nil)
	server.router.ServeHTTP(allRecorder, allRequest)
	if allRecorder.Code != http.StatusOK {
		t.Fatalf("GET deployment runs status = %d, body = %s", allRecorder.Code, allRecorder.Body.String())
	}
	var allResp struct {
		Data []models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(allRecorder.Body.Bytes(), &allResp); err != nil {
		t.Fatalf("decode all runs response: %v", err)
	}
	if got := deploymentRunIDs(allResp.Data); strings.Join(got, ",") != "new-failed,mid-failed,old-success" {
		t.Fatalf("all runs order = %#v", got)
	}

	failedRecorder := httptest.NewRecorder()
	failedRequest := httptest.NewRequest(http.MethodGet, "/api/deployments/runs?status=failed", nil)
	server.router.ServeHTTP(failedRecorder, failedRequest)
	if failedRecorder.Code != http.StatusOK {
		t.Fatalf("GET failed deployment runs status = %d, body = %s", failedRecorder.Code, failedRecorder.Body.String())
	}
	var failedResp struct {
		Data []models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(failedRecorder.Body.Bytes(), &failedResp); err != nil {
		t.Fatalf("decode failed runs response: %v", err)
	}
	if got := deploymentRunIDs(failedResp.Data); strings.Join(got, ",") != "new-failed,mid-failed" {
		t.Fatalf("failed runs order/filter = %#v", got)
	}
}

func TestCreateDeploymentRunExecutesTemplateWhenNotDryRun(t *testing.T) {
	store := newDeploymentTestStore(t)
	taskManager := service.NewTaskManager(store)
	executor := &fakeDeploymentScriptExecutor{
		outputs: []string{
			`SBM_PROGRESS {"step":"probe_system","status":"success","message":"System probe completed"}
SBM_RESULT {"status":"success","os":"linux","arch":"amd64","privilege_mode":"root","systemd":true,"proxy_port":443,"port_listening":false}
`,
			`remote preparing
SBM_PROGRESS {"step":"render_config","status":"success"}
SBM_PROGRESS {"step":"verify_service","status":"success"}
SBM_NODE_BEGIN
{"type":"vless","tag":"edge-a","server":"vpn.example.com","server_port":443,"extra":{"uuid":"generated-uuid","flow":"xtls-rprx-vision","tls":{"enabled":true,"server_name":"www.microsoft.com","reality":{"enabled":true,"public_key":"generated-public","short_id":"0123456789abcdef"}}}}
SBM_NODE_END
SBM_RESULT {"status":"success","systemd":true,"service_active":true,"port_listening":true}
			`,
		},
	}
	reachability := &fakeDeploymentReachabilityChecker{}
	server := &Server{
		dbStore:                   store,
		taskManager:               taskManager,
		taskHandler:               NewTaskHandler(store, taskManager),
		router:                    gin.New(),
		deployScriptExecutor:      executor,
		deployReachabilityChecker: reachability,
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"template_name":"singbox-vless-reality",
		"dry_run":false,
		"ssh":{"host":"203.0.113.10","port":22,"user":"root","auth_method":"password","password":"ssh-secret"},
		"connection":{"mode":"direct"},
		"parameters":{"node_name":"edge-a","node_server":"vpn.example.com","proxy_port":443}
	}`)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("POST /deployments/runs status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var acceptedResp struct {
		Data models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &acceptedResp); err != nil {
		t.Fatalf("decode accepted response: %v", err)
	}
	if acceptedResp.Data.Status != models.DeploymentRunStatusRunning {
		t.Fatalf("accepted status = %q, want running", acceptedResp.Data.Status)
	}
	completed := waitForDeploymentRunStatus(t, store, acceptedResp.Data.ID, models.DeploymentRunStatusSuccess)
	_ = waitForTaskStatus(t, store, acceptedResp.Data.TaskID, models.TaskStatusCompleted)
	if executor.calls != 2 {
		t.Fatalf("script executor calls = %d, want 2", executor.calls)
	}
	if len(executor.executionModes) != 2 || executor.executionModes[0] != "stdin" || executor.executionModes[1] != "uploaded_script" {
		t.Fatalf("execution modes = %#v, want probe stdin then uploaded template", executor.executionModes)
	}
	if !bytes.Contains(executor.scripts[0], []byte("probe_system")) || !bytes.Contains(executor.scripts[1], []byte("SBM_NODE_BEGIN")) {
		t.Fatalf("executor did not receive probe then deployment scripts")
	}
	for _, key := range []string{"SBM_NODE_NAME", "SBM_NODE_SERVER", "SBM_PROXY_PORT", "SBM_UUID", "SBM_REALITY_PRIVATE_KEY", "SBM_REALITY_PUBLIC_KEY", "SBM_REALITY_SHORT_ID", "SBM_REALITY_SERVER_NAME"} {
		if executor.env[key] == "" {
			t.Fatalf("executor env missing %s in %#v", key, executor.env)
		}
	}
	if executor.env["SBM_NODE_NAME"] != "edge-a" || executor.env["SBM_NODE_SERVER"] != "vpn.example.com" || executor.env["SBM_PROXY_PORT"] != "443" {
		t.Fatalf("deployment env did not preserve request parameters: %#v", executor.env)
	}

	if completed.DryRun {
		t.Fatal("run should be recorded as non-dry-run")
	}
	if completed.Status != models.DeploymentRunStatusSuccess {
		t.Fatalf("status = %q, want success", completed.Status)
	}
	if completed.GeneratedNode["tag"] != "edge-a" || completed.ProbeResult["systemd"] != true || completed.ProbeResult["privilege_mode"] != "root" {
		t.Fatalf("script markers were not parsed into run: %#v", completed)
	}
	extra, ok := completed.GeneratedNode["extra"].(map[string]interface{})
	if !ok {
		t.Fatalf("generated node extra type = %T", completed.GeneratedNode["extra"])
	}
	tls, ok := extra["tls"].(map[string]interface{})
	if !ok || tls["enabled"] != true {
		t.Fatalf("generated node tls shape = %#v", extra["tls"])
	}
	if _, ok := tls["reality"].(map[string]interface{}); !ok {
		t.Fatalf("generated node missing nested reality: %#v", tls)
	}
	if completed.ProgressMarkers["verify_service"] == nil || completed.ProgressMarkers["deployment_result"] == nil {
		t.Fatalf("deployment verification/result markers missing: %#v", completed.ProgressMarkers)
	}
	if reachability.calls != 1 || reachability.host != "vpn.example.com" || reachability.port != 443 {
		t.Fatalf("external reachability check = %d %s:%d, want vpn.example.com:443", reachability.calls, reachability.host, reachability.port)
	}
	if marker, ok := completed.ProgressMarkers["external_reachability"].(map[string]interface{}); !ok || marker["status"] != "success" {
		t.Fatalf("external reachability marker missing: %#v", completed.ProgressMarkers["external_reachability"])
	}
	if completed.Stdout == "" || completed.ExitCode == nil || *completed.ExitCode != 0 {
		t.Fatalf("run output/exit code not recorded: %#v", completed)
	}
	if completed.RedactedParams["runtime_source"] != "remote" ||
		completed.RedactedParams["runtime_arch"] != "amd64" ||
		completed.RedactedParams["runtime_download_url"] != deploy.RuntimeDownloadURL("sing-box", deploy.PinnedSingBoxVersion, "linux", "amd64") {
		t.Fatalf("runtime remote metadata not recorded: %#v", completed.RedactedParams)
	}
	assertNoSecretValue(t, completed.RedactedParams, "ssh-secret", executor.env["SBM_REALITY_PRIVATE_KEY"])
}

func TestCreateDeploymentRunDoesNotSendConnectionSecretsToRemoteScript(t *testing.T) {
	store := newDeploymentTestStore(t)
	taskManager := service.NewTaskManager(store)
	executor := &fakeDeploymentScriptExecutor{
		outputs: []string{
			`SBM_PROGRESS {"step":"probe_system","status":"success","message":"System probe completed"}
SBM_RESULT {"status":"success","os":"linux","arch":"amd64","privilege_mode":"root","systemd":true,"proxy_port":443,"port_listening":false}
`,
			`SBM_PROGRESS {"step":"verify_service","status":"success"}
SBM_NODE_BEGIN
{"type":"vless","tag":"edge-a","server":"vpn.example.com","server_port":443}
SBM_NODE_END
SBM_RESULT {"status":"success","systemd":true,"service_active":true,"port_listening":true}
`,
		},
	}
	server := &Server{
		dbStore:                   store,
		taskManager:               taskManager,
		taskHandler:               NewTaskHandler(store, taskManager),
		router:                    gin.New(),
		deployScriptExecutor:      executor,
		deployReachabilityChecker: &fakeDeploymentReachabilityChecker{},
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"template_name":"singbox-vless-reality",
		"dry_run":false,
		"ssh":{"host":"203.0.113.10","port":22,"user":"root","auth_method":"private_key","private_key":"-----BEGIN OPENSSH PRIVATE KEY-----\nssh-private-key-secret\n-----END OPENSSH PRIVATE KEY-----","private_key_passphrase":"ssh-key-passphrase-secret"},
		"connection":{"mode":"custom_proxy","proxy_type":"socks5","proxy_host":"127.0.0.1","proxy_port":1080,"proxy_username":"ops","proxy_password":"proxy-password-secret"},
		"parameters":{"node_name":"edge-a","node_server":"vpn.example.com","proxy_port":443}
	}`)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("POST /deployments/runs status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var acceptedResp struct {
		Data models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &acceptedResp); err != nil {
		t.Fatalf("decode accepted response: %v", err)
	}
	completed := waitForDeploymentRunStatus(t, store, acceptedResp.Data.ID, models.DeploymentRunStatusSuccess)
	if executor.calls != 2 {
		t.Fatalf("script executor calls = %d, want 2", executor.calls)
	}

	secrets := []string{"ssh-private-key-secret", "ssh-key-passphrase-secret", "proxy-password-secret"}
	for call, env := range executor.envs {
		encoded, err := json.Marshal(env)
		if err != nil {
			t.Fatalf("marshal executor env %d: %v", call, err)
		}
		assertNoSecretBytes(t, fmt.Sprintf("executor env %d", call), encoded, secrets...)
	}
	for call, script := range executor.scripts {
		assertNoSecretBytes(t, fmt.Sprintf("executor script %d", call), script, secrets...)
	}
	assertNoSecretValue(t, completed.RedactedParams, secrets...)
}

func TestCreateDeploymentRunUsesTemporaryManagedEntrypoint(t *testing.T) {
	store := newDeploymentTestStore(t)
	jsonStore, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}
	if err := jsonStore.AddManualNode(storage.ManualNode{
		ID: "manual-1",
		Node: storage.Node{
			Tag:        "hk-1",
			Type:       "socks",
			Server:     "127.0.0.1",
			ServerPort: 1080,
		},
		Enabled: true,
	}); err != nil {
		t.Fatalf("AddManualNode() error = %v", err)
	}
	taskManager := service.NewTaskManager(store)
	executor := &fakeDeploymentScriptExecutor{
		outputs: []string{
			`SBM_PROGRESS {"step":"probe_system","status":"success"}
SBM_RESULT {"status":"success","os":"linux","arch":"amd64","privilege_mode":"root","systemd":true,"proxy_port":443,"port_listening":false}
`,
			`SBM_PROGRESS {"step":"verify_service","status":"success"}
SBM_NODE_BEGIN
{"type":"vless","tag":"edge-a","server":"vpn.example.com","server_port":443,"extra":{"uuid":"generated-uuid","flow":"xtls-rprx-vision","tls":{"enabled":true,"server_name":"www.microsoft.com","reality":{"enabled":true,"public_key":"generated-public","short_id":"0123456789abcdef"}}}}
SBM_NODE_END
SBM_RESULT {"status":"success","systemd":true,"service_active":true,"port_listening":true}
`,
		},
	}
	reachability := &fakeDeploymentReachabilityChecker{}
	var cleaned atomic.Bool
	server := &Server{
		dbStore:                   store,
		store:                     jsonStore,
		taskManager:               taskManager,
		taskHandler:               NewTaskHandler(store, taskManager),
		router:                    gin.New(),
		deployScriptExecutor:      executor,
		deployReachabilityChecker: reachability,
		deployEntrypointStarter: &fakeDeploymentEntrypointStarter{
			endpoint: "127.0.0.1:34568",
			cleaned:  &cleaned,
		},
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"template_name":"singbox-vless-reality",
		"dry_run":false,
		"ssh":{"host":"203.0.113.10","port":22,"user":"root","auth_method":"password","password":"ssh-secret"},
		"connection":{"mode":"managed_proxy","managed_candidate_id":"node:hk-1"},
		"parameters":{"node_name":"edge-a","node_server":"vpn.example.com","proxy_port":443}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("POST /deployments/runs status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var resp struct {
		Data models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	_ = waitForDeploymentRunCondition(t, store, resp.Data.ID, func(run *models.DeploymentRun) bool {
		return run.Status == models.DeploymentRunStatusSuccess && cleaned.Load()
	})
	_ = waitForTaskStatus(t, store, resp.Data.TaskID, models.TaskStatusCompleted)
	if executor.req.Connection.Mode != models.DeploymentConnectionManagedProxy ||
		executor.req.Connection.ProxyType != "socks5" ||
		executor.req.Connection.ProxyHost != "127.0.0.1" ||
		executor.req.Connection.ProxyPort != 34568 {
		t.Fatalf("script executor did not receive managed SOCKS5 endpoint: %#v", executor.req.Connection)
	}
	if len(executor.reqs) != 2 {
		t.Fatalf("script executor request count = %d, want probe and deployment requests", len(executor.reqs))
	}
	for i, req := range executor.reqs {
		if req.Connection.Mode != models.DeploymentConnectionManagedProxy ||
			req.Connection.ProxyType != "socks5" ||
			req.Connection.ProxyHost != "127.0.0.1" ||
			req.Connection.ProxyPort != 34568 {
			t.Fatalf("script executor request %d did not use managed SOCKS5 endpoint: %#v", i, req.Connection)
		}
	}
	if reachability.req.Connection.ProxyPort != 34568 {
		t.Fatalf("reachability checker did not reuse managed SOCKS5 endpoint: %#v", reachability.req.Connection)
	}
}

func TestCreateDeploymentRunReusesExistingManagedTunnel(t *testing.T) {
	store := newDeploymentTestStore(t)
	jsonStore, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}
	if err := jsonStore.AddManualNode(storage.ManualNode{
		ID: "manual-1",
		Node: storage.Node{
			Tag:        "hk-1",
			Type:       "socks",
			Server:     "127.0.0.1",
			ServerPort: 1080,
		},
		Enabled: true,
	}); err != nil {
		t.Fatalf("AddManualNode() error = %v", err)
	}
	if err := jsonStore.AddInboundPort(storage.InboundPort{
		ID:       "hk-tunnel",
		Name:     "香港隧道",
		Type:     "mixed",
		Listen:   "127.0.0.1",
		Port:     2081,
		Outbound: "hk-1",
		Enabled:  true,
	}); err != nil {
		t.Fatalf("AddInboundPort() error = %v", err)
	}
	taskManager := service.NewTaskManager(store)
	executor := &fakeDeploymentScriptExecutor{
		outputs: []string{
			successfulProbeOutput("amd64"),
			`SBM_PROGRESS {"step":"verify_service","status":"success"}
SBM_NODE_BEGIN
{"type":"vless","tag":"edge-a","server":"vpn.example.com","server_port":443}
SBM_NODE_END
SBM_RESULT {"status":"success","systemd":true,"service_active":true,"port_listening":true}
`,
		},
	}
	reachability := &fakeDeploymentReachabilityChecker{}
	starter := &fakeDeploymentEntrypointStarter{endpoint: "127.0.0.1:34568"}
	serviceRunning := true
	server := &Server{
		dbStore:                   store,
		store:                     jsonStore,
		taskManager:               taskManager,
		taskHandler:               NewTaskHandler(store, taskManager),
		router:                    gin.New(),
		deployScriptExecutor:      executor,
		deployReachabilityChecker: reachability,
		deployEntrypointStarter:   starter,
		deployServiceRunning:      &serviceRunning,
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"template_name":"singbox-vless-reality",
		"dry_run":false,
		"ssh":{"host":"203.0.113.10","port":22,"user":"root","auth_method":"password","password":"ssh-secret"},
		"connection":{"mode":"managed_proxy","managed_candidate_id":"node:hk-1"},
		"parameters":{"node_name":"edge-a","node_server":"vpn.example.com","proxy_port":443}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("POST /deployments/runs status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var resp struct {
		Data models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	_ = waitForDeploymentRunStatus(t, store, resp.Data.ID, models.DeploymentRunStatusSuccess)
	if starter.started {
		t.Fatal("existing managed tunnel should be reused without starting a temporary entrypoint")
	}
	for i, req := range executor.reqs {
		if req.Connection.Mode != models.DeploymentConnectionManagedProxy ||
			req.Connection.ProxyType != "socks5" ||
			req.Connection.ProxyHost != "127.0.0.1" ||
			req.Connection.ProxyPort != 2081 {
			t.Fatalf("script executor request %d did not reuse existing tunnel: %#v", i, req.Connection)
		}
	}
	if reachability.req.Connection.ProxyPort != 2081 {
		t.Fatalf("reachability checker did not reuse existing tunnel: %#v", reachability.req.Connection)
	}
}

func TestCreateDeploymentRunCleansTemporaryManagedEntrypointOnFailure(t *testing.T) {
	store := newDeploymentTestStore(t)
	jsonStore, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}
	if err := jsonStore.AddManualNode(storage.ManualNode{
		ID: "manual-1",
		Node: storage.Node{
			Tag:        "hk-1",
			Type:       "socks",
			Server:     "127.0.0.1",
			ServerPort: 1080,
		},
		Enabled: true,
	}); err != nil {
		t.Fatalf("AddManualNode() error = %v", err)
	}
	taskManager := service.NewTaskManager(store)
	executor := &fakeDeploymentScriptExecutor{
		outputs: []string{
			successfulProbeOutput("amd64"),
			`SBM_PROGRESS {"step":"install_singbox","status":"failed"}
SBM_RESULT {"status":"failed","message":"curl or wget is required"}
`,
		},
	}
	var cleaned atomic.Bool
	server := &Server{
		dbStore:              store,
		store:                jsonStore,
		taskManager:          taskManager,
		taskHandler:          NewTaskHandler(store, taskManager),
		router:               gin.New(),
		deployScriptExecutor: executor,
		deployEntrypointStarter: &fakeDeploymentEntrypointStarter{
			endpoint: "127.0.0.1:34568",
			cleaned:  &cleaned,
		},
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"template_name":"singbox-vless-reality",
		"dry_run":false,
		"ssh":{"host":"203.0.113.10","port":22,"user":"root","auth_method":"password","password":"ssh-secret"},
		"connection":{"mode":"managed_proxy","managed_candidate_id":"node:hk-1"},
		"parameters":{"node_name":"edge-a","node_server":"vpn.example.com","proxy_port":443}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("POST /deployments/runs status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var resp struct {
		Data models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	failed := waitForDeploymentRunCondition(t, store, resp.Data.ID, func(run *models.DeploymentRun) bool {
		return run.Status == models.DeploymentRunStatusFailed && cleaned.Load()
	})
	if !strings.Contains(failed.Stderr, "curl or wget is required") {
		t.Fatalf("stderr = %q, want script failure", failed.Stderr)
	}
}

func TestCancelManagedDeploymentRunCleansTemporaryEntrypoint(t *testing.T) {
	store := newDeploymentTestStore(t)
	jsonStore, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}
	if err := jsonStore.AddManualNode(storage.ManualNode{
		ID: "manual-1",
		Node: storage.Node{
			Tag:        "hk-1",
			Type:       "socks",
			Server:     "127.0.0.1",
			ServerPort: 1080,
		},
		Enabled: true,
	}); err != nil {
		t.Fatalf("AddManualNode() error = %v", err)
	}
	taskManager := service.NewTaskManager(store)
	executor := &fakeDeploymentScriptExecutor{waitForCancel: true, waitForCancelCall: 1}
	var cleaned atomic.Bool
	server := &Server{
		dbStore:              store,
		store:                jsonStore,
		taskManager:          taskManager,
		taskHandler:          NewTaskHandler(store, taskManager),
		router:               gin.New(),
		deployScriptExecutor: executor,
		deployEntrypointStarter: &fakeDeploymentEntrypointStarter{
			endpoint: "127.0.0.1:34568",
			cleaned:  &cleaned,
		},
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"template_name":"singbox-vless-reality",
		"dry_run":false,
		"ssh":{"host":"203.0.113.10","port":22,"user":"root","auth_method":"password","password":"ssh-secret"},
		"connection":{"mode":"managed_proxy","managed_candidate_id":"node:hk-1"},
		"parameters":{"node_name":"edge-a","node_server":"vpn.example.com","proxy_port":443}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("POST /deployments/runs status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var resp struct {
		Data models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	cancelRecorder := httptest.NewRecorder()
	cancelRequest := httptest.NewRequest(http.MethodPost, "/api/deployments/runs/"+resp.Data.ID+"/cancel", nil)
	server.router.ServeHTTP(cancelRecorder, cancelRequest)
	if cancelRecorder.Code != http.StatusOK {
		t.Fatalf("POST cancel status = %d, body = %s", cancelRecorder.Code, cancelRecorder.Body.String())
	}
	_ = waitForDeploymentRunCondition(t, store, resp.Data.ID, func(run *models.DeploymentRun) bool {
		return run.Status == models.DeploymentRunStatusCancelled && cleaned.Load()
	})
}

func TestCreateDeploymentRunFailsWhenExternalReachabilityFails(t *testing.T) {
	store := newDeploymentTestStore(t)
	taskManager := service.NewTaskManager(store)
	executor := &fakeDeploymentScriptExecutor{
		outputs: []string{
			`SBM_PROGRESS {"step":"probe_system","status":"success","message":"System probe completed"}
SBM_RESULT {"status":"success","os":"linux","arch":"amd64","privilege_mode":"root","systemd":true,"proxy_port":443,"port_listening":false}
`,
			`remote preparing
SBM_PROGRESS {"step":"render_config","status":"success"}
SBM_PROGRESS {"step":"verify_service","status":"success"}
SBM_NODE_BEGIN
{"type":"vless","tag":"edge-a","server":"vpn.example.com","server_port":443}
SBM_NODE_END
SBM_RESULT {"status":"success","systemd":true,"service_active":true,"port_listening":true}
`,
		},
	}
	reachability := &fakeDeploymentReachabilityChecker{err: errors.New("dial tcp: i/o timeout")}
	server := &Server{
		dbStore:                   store,
		taskManager:               taskManager,
		taskHandler:               NewTaskHandler(store, taskManager),
		router:                    gin.New(),
		deployScriptExecutor:      executor,
		deployReachabilityChecker: reachability,
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"template_name":"singbox-vless-reality",
		"dry_run":false,
		"ssh":{"host":"203.0.113.10","port":22,"user":"root","auth_method":"password","password":"ssh-secret"},
		"connection":{"mode":"direct"},
		"parameters":{"node_name":"edge-a","node_server":"vpn.example.com","proxy_port":443}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("POST /deployments/runs status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var acceptedResp struct {
		Data models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &acceptedResp); err != nil {
		t.Fatalf("decode accepted response: %v", err)
	}
	failed := waitForDeploymentRunStatus(t, store, acceptedResp.Data.ID, models.DeploymentRunStatusFailed)
	if reachability.calls != 1 || reachability.host != "vpn.example.com" || reachability.port != 443 {
		t.Fatalf("external reachability check = %d %s:%d, want vpn.example.com:443", reachability.calls, reachability.host, reachability.port)
	}
	if !strings.Contains(failed.Stderr, "部署节点端口不可达") {
		t.Fatalf("stderr = %q, want reachability failure", failed.Stderr)
	}
	if failed.GeneratedNode["tag"] != "edge-a" || failed.ProbeResult["systemd"] != true || !strings.Contains(failed.Stdout, "remote preparing") {
		t.Fatalf("reachability failure did not preserve deployment evidence: %#v", failed)
	}
	marker, ok := failed.ProgressMarkers["external_reachability"].(map[string]interface{})
	if !ok || marker["status"] != "failed" {
		t.Fatalf("failed reachability marker missing: %#v", failed.ProgressMarkers["external_reachability"])
	}
	if failed.ExitCode == nil || *failed.ExitCode != 1 {
		t.Fatalf("exit code = %v, want 1", failed.ExitCode)
	}

	reviewRecorder := httptest.NewRecorder()
	reviewRequest := httptest.NewRequest(http.MethodGet, "/api/deployments/runs/"+acceptedResp.Data.ID+"/generated-node", nil)
	server.router.ServeHTTP(reviewRecorder, reviewRequest)
	if reviewRecorder.Code != http.StatusBadRequest {
		t.Fatalf("GET failed generated node review status = %d, body = %s", reviewRecorder.Code, reviewRecorder.Body.String())
	}
	importRecorder := httptest.NewRecorder()
	importRequest := httptest.NewRequest(http.MethodPost, "/api/deployments/runs/"+acceptedResp.Data.ID+"/import-node", bytes.NewReader([]byte(`{"tag":"edge-a","enabled":true}`)))
	importRequest.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(importRecorder, importRequest)
	if importRecorder.Code != http.StatusBadRequest {
		t.Fatalf("POST failed generated node import status = %d, body = %s", importRecorder.Code, importRecorder.Body.String())
	}
}

func TestCreateDeploymentRunRejectsCachedRuntimeArchiveChecksumMismatch(t *testing.T) {
	store := newDeploymentTestStore(t)
	taskManager := service.NewTaskManager(store)
	baseDir := t.TempDir()
	content := []byte("archive")
	cache := deploy.NewRuntimeCache(baseDir)
	if _, err := cache.Put("sing-box", deploy.PinnedSingBoxVersion, "linux", "amd64", content); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	executor := &fakeDeploymentScriptExecutor{
		outputs: []string{successfulProbeOutput("amd64")},
	}
	server := &Server{
		dbStore:                   store,
		taskManager:               taskManager,
		taskHandler:               NewTaskHandler(store, taskManager),
		router:                    gin.New(),
		baseDir:                   baseDir,
		deployScriptExecutor:      executor,
		deployReachabilityChecker: &fakeDeploymentReachabilityChecker{},
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"template_name":"singbox-vless-reality",
		"dry_run":false,
		"ssh":{"host":"203.0.113.10","port":22,"user":"root","auth_method":"password","password":"ssh-secret"},
		"connection":{"mode":"direct"},
		"parameters":{"node_name":"edge-a","node_server":"vpn.example.com","proxy_port":443,"runtime_source":"cache","runtime_os":"linux","runtime_arch":"amd64"}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("POST /deployments/runs status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var acceptedResp struct {
		Data models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &acceptedResp); err != nil {
		t.Fatalf("decode accepted response: %v", err)
	}
	failed := waitForDeploymentRunStatus(t, store, acceptedResp.Data.ID, models.DeploymentRunStatusFailed)
	if executor.calls != 1 {
		t.Fatalf("script executor calls = %d, want only the probe before invalid cache is used", executor.calls)
	}
	if !strings.Contains(failed.Stderr, "runtime archive checksum mismatch") {
		t.Fatalf("stderr = %q, want checksum mismatch", failed.Stderr)
	}
}

func successfulProbeOutput(arch string) string {
	return fmt.Sprintf(`SBM_PROGRESS {"step":"probe_system","status":"success","message":"System probe completed"}
SBM_RESULT {"status":"success","os":"linux","arch":%q,"kernel":"6.8.0","privilege_mode":"root","systemd":true,"proxy_port":443,"port_listening":false}
`, arch)
}

func TestCreateDeploymentRunSelectsRuntimeArchiveFromProbeArch(t *testing.T) {
	store := newDeploymentTestStore(t)
	taskManager := service.NewTaskManager(store)
	baseDir := t.TempDir()
	executor := &fakeDeploymentScriptExecutor{
		outputs: []string{`SBM_PROGRESS {"step":"probe_system","status":"success","message":"System probe completed"}
SBM_RESULT {"status":"success","os":"linux","arch":"arm64","kernel":"6.8.0","privilege_mode":"root","systemd":true,"proxy_port":443,"port_listening":false}
`},
	}
	server := &Server{
		dbStore:                   store,
		taskManager:               taskManager,
		taskHandler:               NewTaskHandler(store, taskManager),
		router:                    gin.New(),
		baseDir:                   baseDir,
		deployScriptExecutor:      executor,
		deployReachabilityChecker: &fakeDeploymentReachabilityChecker{},
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"template_name":"singbox-vless-reality",
		"dry_run":false,
		"ssh":{"host":"203.0.113.10","port":22,"user":"root","auth_method":"password","password":"ssh-secret"},
		"connection":{"mode":"direct"},
		"parameters":{"node_name":"edge-a","node_server":"vpn.example.com","proxy_port":443,"runtime_source":"cache","runtime_os":"linux","runtime_arch":"amd64"}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("POST /deployments/runs status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var acceptedResp struct {
		Data models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &acceptedResp); err != nil {
		t.Fatalf("decode accepted response: %v", err)
	}
	failed := waitForDeploymentRunStatus(t, store, acceptedResp.Data.ID, models.DeploymentRunStatusFailed)
	if executor.calls != 1 {
		t.Fatalf("script executor calls = %d, want only probe", executor.calls)
	}
	if !strings.Contains(failed.Stderr, "linux-arm64.tar.gz") {
		t.Fatalf("stderr = %q, want arm64 cache lookup from probe result", failed.Stderr)
	}
	if failed.RedactedParams["runtime_source"] != "cache" || failed.RedactedParams["runtime_arch"] != "arm64" {
		t.Fatalf("effective runtime cache metadata not recorded from probe result: %#v", failed.RedactedParams)
	}
	if _, ok := failed.RedactedParams["runtime_download_url"]; ok {
		t.Fatalf("cache mode should not record remote download URL: %#v", failed.RedactedParams)
	}
}

func TestDeploymentScriptEnvSetsPinnedRuntimeChecksums(t *testing.T) {
	template, ok := deploy.BuiltinTemplates().Get("singbox-vless-reality")
	if !ok {
		t.Fatal("singbox-vless-reality template is not registered")
	}
	env, err := deploymentScriptEnv(deploymentRunRequest{
		SSH: deploymentSSHRequest{Host: "203.0.113.10"},
		Parameters: map[string]interface{}{
			"node_name":   "edge-a",
			"node_server": "vpn.example.com",
			"proxy_port":  443,
		},
	}, t.TempDir(), template)
	if err != nil {
		t.Fatalf("deploymentScriptEnv() error = %v", err)
	}
	if env["SBM_SINGBOX_SHA256_AMD64"] == "" || env["SBM_SINGBOX_SHA256_ARM64"] == "" {
		t.Fatalf("runtime checksum env missing: %#v", env)
	}
}

func TestDeploymentScriptEnvSetsDebugPreserveRemoteRunDir(t *testing.T) {
	template, ok := deploy.BuiltinTemplates().Get("singbox-vless-reality")
	if !ok {
		t.Fatal("singbox-vless-reality template is not registered")
	}
	env, err := deploymentScriptEnv(deploymentRunRequest{
		SSH: deploymentSSHRequest{Host: "203.0.113.10"},
		Parameters: map[string]interface{}{
			"node_name":                     "edge-a",
			"node_server":                   "vpn.example.com",
			"proxy_port":                    443,
			"debug_preserve_remote_run_dir": true,
		},
	}, t.TempDir(), template)
	if err != nil {
		t.Fatalf("deploymentScriptEnv() error = %v", err)
	}
	if env["SBM_DEBUG_PRESERVE_REMOTE_RUN_DIR"] != "true" {
		t.Fatalf("debug preserve env missing: %#v", env)
	}
}

func TestSecurityBasicDeploymentScriptEnvDoesNotIncludeProxyTemplateSecrets(t *testing.T) {
	template, ok := deploy.BuiltinTemplates().Get("security-basic")
	if !ok {
		t.Fatal("security-basic template is not registered")
	}
	env, err := deploymentScriptEnv(deploymentRunRequest{
		SSH: deploymentSSHRequest{Host: "203.0.113.10"},
		Parameters: map[string]interface{}{
			"security_install_base_packages": true,
			"security_base_packages":         "curl tar",
			"security_firewall_mode":         "inspect_only",
		},
	}, t.TempDir(), template)
	if err != nil {
		t.Fatalf("deploymentScriptEnv() error = %v", err)
	}
	if env["SBM_SECURITY_INSTALL_BASE_PACKAGES"] != "true" || env["SBM_SECURITY_BASE_PACKAGES"] != "curl tar" || env["SBM_SECURITY_FIREWALL_MODE"] != "inspect_only" {
		t.Fatalf("security-basic env missing: %#v", env)
	}
	for _, forbidden := range []string{"SBM_UUID", "SBM_REALITY_PRIVATE_KEY", "SBM_REALITY_PUBLIC_KEY", "SBM_SINGBOX_VERSION", "SBM_SINGBOX_SHA256_AMD64"} {
		if _, ok := env[forbidden]; ok {
			t.Fatalf("security-basic env should not include %s: %#v", forbidden, env)
		}
	}
}

func TestCreateSecurityBasicDeploymentRunSkipsProxyReachability(t *testing.T) {
	store := newDeploymentTestStore(t)
	taskManager := service.NewTaskManager(store)
	executor := &fakeDeploymentScriptExecutor{
		outputs: []string{
			`SBM_PROGRESS {"step":"probe_system","status":"success","message":"System probe completed"}
SBM_RESULT {"status":"success","os":"linux","arch":"amd64","privilege_mode":"root","systemd":true,"proxy_port":443,"port_listening":false}
`,
			`SBM_PROGRESS {"step":"security_inspect","status":"success","message":"Security baseline inspected"}
SBM_PROGRESS {"step":"base_packages","status":"skipped","message":"Base package installation disabled"}
SBM_PROGRESS {"step":"firewall_inspect","status":"success","message":"Firewall state inspected without changes"}
SBM_RESULT {"status":"success","package_manager":"apt","firewall":"ufw","firewall_mode":"inspect_only","privilege_mode":"root","base_packages_installed":false,"service_restarted":false,"ssh_policy_changed":false}
`,
		},
	}
	reachability := &fakeDeploymentReachabilityChecker{err: fmt.Errorf("should not be called")}
	server := &Server{
		dbStore:                   store,
		taskManager:               taskManager,
		taskHandler:               NewTaskHandler(store, taskManager),
		router:                    gin.New(),
		deployScriptExecutor:      executor,
		deployReachabilityChecker: reachability,
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"template_name":"security-basic",
		"dry_run":false,
		"ssh":{"host":"203.0.113.10","port":22,"user":"root","auth_method":"password","password":"ssh-secret"},
		"connection":{"mode":"direct"},
		"parameters":{"security_install_base_packages":false,"security_firewall_mode":"inspect_only"}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("POST /deployments/runs status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var resp struct {
		Data models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	completed := waitForDeploymentRunStatus(t, store, resp.Data.ID, models.DeploymentRunStatusSuccess)
	if completed.TemplateName != "security-basic" || completed.TemplateVersion == "" || completed.TemplateChecksum == "" {
		t.Fatalf("security-basic template identity missing: %#v", completed)
	}
	if reachability.calls != 0 {
		t.Fatalf("security-basic should not run proxy reachability check, calls = %d", reachability.calls)
	}
	if len(completed.GeneratedNode) != 0 {
		t.Fatalf("security-basic should not generate proxy node: %#v", completed.GeneratedNode)
	}
	if completed.ProgressMarkers["security_inspect"] == nil || completed.ProgressMarkers["deployment_result"] == nil {
		t.Fatalf("security-basic progress/result missing: %#v", completed.ProgressMarkers)
	}
	if _, ok := completed.RedactedParams["runtime_name"]; ok {
		t.Fatalf("security-basic should not record proxy runtime metadata: %#v", completed.RedactedParams)
	}
	if completed.ExitCode == nil || *completed.ExitCode != 0 {
		t.Fatalf("exit code = %v, want 0", completed.ExitCode)
	}
}

func TestCreateSecurityBasicDryRunDoesNotGenerateProxyNode(t *testing.T) {
	store := newDeploymentTestStore(t)
	taskManager := service.NewTaskManager(store)
	server := &Server{
		dbStore:     store,
		taskManager: taskManager,
		taskHandler: NewTaskHandler(store, taskManager),
		router:      gin.New(),
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"template_name":"security-basic",
		"dry_run":true,
		"ssh":{"host":"203.0.113.10","port":22,"user":"root","auth_method":"password","password":"ssh-secret"},
		"connection":{"mode":"direct"},
		"parameters":{"security_install_base_packages":false,"security_firewall_mode":"inspect_only"}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("POST /deployments/runs status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var resp struct {
		Data models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data.GeneratedNode) != 0 {
		t.Fatalf("security-basic dry-run generated proxy node: %#v", resp.Data.GeneratedNode)
	}
	if !strings.Contains(resp.Data.Stdout, `"template":"security-basic"`) {
		t.Fatalf("security-basic dry-run stdout missing template marker: %s", resp.Data.Stdout)
	}
}

func TestCreateDeploymentRunRejectsInvalidNodeProxyPort(t *testing.T) {
	tests := []struct {
		name      string
		proxyPort string
	}{
		{name: "out of range", proxyPort: `70000`},
		{name: "non numeric string", proxyPort: `"abc"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newDeploymentTestStore(t)
			taskManager := service.NewTaskManager(store)
			server := &Server{
				dbStore:     store,
				taskManager: taskManager,
				taskHandler: NewTaskHandler(store, taskManager),
				router:      gin.New(),
			}
			server.registerDeploymentRoutes(server.router.Group("/api"))

			body := []byte(`{
				"template_name":"singbox-vless-reality",
				"dry_run":false,
				"ssh":{"host":"203.0.113.10","port":22,"user":"root","auth_method":"password","password":"ssh-secret"},
				"connection":{"mode":"direct"},
				"parameters":{"node_name":"edge-a","node_server":"vpn.example.com","proxy_port":` + tt.proxyPort + `}
			}`)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs", bytes.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			server.router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("POST /deployments/runs status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			if !bytes.Contains(recorder.Body.Bytes(), []byte("parameters.proxy_port")) {
				t.Fatalf("response %s does not identify proxy_port", recorder.Body.String())
			}
			runs, err := store.GetDeploymentRuns(10, 0, "")
			if err != nil {
				t.Fatalf("GetDeploymentRuns() error = %v", err)
			}
			if len(runs) != 0 {
				t.Fatalf("invalid request created deployment runs: %#v", runs)
			}
		})
	}
}

func TestCreateDeploymentRunRejectsRuntimeVersionOverride(t *testing.T) {
	store := newDeploymentTestStore(t)
	taskManager := service.NewTaskManager(store)
	server := &Server{
		dbStore:     store,
		taskManager: taskManager,
		taskHandler: NewTaskHandler(store, taskManager),
		router:      gin.New(),
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"template_name":"singbox-vless-reality",
		"dry_run":false,
		"ssh":{"host":"203.0.113.10","port":22,"user":"root","auth_method":"password","password":"ssh-secret"},
		"connection":{"mode":"direct"},
		"parameters":{"node_name":"edge-a","node_server":"vpn.example.com","proxy_port":443,"runtime_version":"1.14.0"}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("POST /deployments/runs status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("parameters.runtime_version")) {
		t.Fatalf("response %s does not identify runtime_version", recorder.Body.String())
	}
	runs, err := store.GetDeploymentRuns(10, 0, "")
	if err != nil {
		t.Fatalf("GetDeploymentRuns() error = %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("invalid request created deployment runs: %#v", runs)
	}
}

func TestCreateDeploymentRunRejectsInvalidRuntimeSourceBeforeCreatingTask(t *testing.T) {
	store := newDeploymentTestStore(t)
	taskManager := service.NewTaskManager(store)
	server := &Server{
		dbStore:     store,
		taskManager: taskManager,
		taskHandler: NewTaskHandler(store, taskManager),
		router:      gin.New(),
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"template_name":"singbox-vless-reality",
		"dry_run":false,
		"ssh":{"host":"203.0.113.10","port":22,"user":"root","auth_method":"password","password":"ssh-secret"},
		"connection":{"mode":"direct"},
		"parameters":{"node_name":"edge-a","node_server":"vpn.example.com","proxy_port":443,"runtime_source":"mirror"}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("POST /deployments/runs status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("parameters.runtime_source")) {
		t.Fatalf("response %s does not identify runtime_source", recorder.Body.String())
	}
	runs, err := store.GetDeploymentRuns(10, 0, "")
	if err != nil {
		t.Fatalf("GetDeploymentRuns() error = %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("invalid request created deployment runs: %#v", runs)
	}
	tasks, err := store.GetTasks(10, 0, models.TaskTypeDeploymentRun, "")
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("invalid request created tasks: %#v", tasks)
	}
}

func TestCreateDeploymentRunRejectsUnsupportedTemplateBeforeCreatingTask(t *testing.T) {
	store := newDeploymentTestStore(t)
	taskManager := service.NewTaskManager(store)
	server := &Server{
		dbStore:     store,
		taskManager: taskManager,
		taskHandler: NewTaskHandler(store, taskManager),
		router:      gin.New(),
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"template_name":"unknown-template",
		"dry_run":false,
		"ssh":{"host":"203.0.113.10","port":22,"user":"root","auth_method":"password","password":"ssh-secret"},
		"connection":{"mode":"direct"},
		"parameters":{"node_name":"edge-a","node_server":"vpn.example.com","proxy_port":443}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("POST /deployments/runs status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	tasks, err := store.GetTasks(10, 0, models.TaskTypeDeploymentRun, "")
	if err != nil {
		t.Fatalf("GetTasks() error = %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("unsupported template created tasks: %#v", tasks)
	}
	runs, err := store.GetDeploymentRuns(10, 0, "")
	if err != nil {
		t.Fatalf("GetDeploymentRuns() error = %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("unsupported template created deployment runs: %#v", runs)
	}
}

func TestCreateDeploymentRunFailsWhenProbeFindsPortInUse(t *testing.T) {
	store := newDeploymentTestStore(t)
	taskManager := service.NewTaskManager(store)
	executor := &fakeDeploymentScriptExecutor{
		outputs: []string{`SBM_PROGRESS {"step":"probe_system","status":"success","message":"System probe completed"}
SBM_RESULT {"status":"success","os":"linux","arch":"amd64","privilege_mode":"root","systemd":true,"proxy_port":443,"port_listening":true}
`},
	}
	server := &Server{
		dbStore:              store,
		taskManager:          taskManager,
		taskHandler:          NewTaskHandler(store, taskManager),
		router:               gin.New(),
		deployScriptExecutor: executor,
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"template_name":"singbox-vless-reality",
		"dry_run":false,
		"ssh":{"host":"203.0.113.10","port":22,"user":"root","auth_method":"password","password":"ssh-secret"},
		"connection":{"mode":"direct"},
		"parameters":{"node_name":"edge-a","node_server":"vpn.example.com","proxy_port":443}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("POST /deployments/runs status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var resp struct {
		Data models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	failed := waitForDeploymentRunStatus(t, store, resp.Data.ID, models.DeploymentRunStatusFailed)
	if executor.calls != 1 {
		t.Fatalf("script executor calls = %d, want only probe", executor.calls)
	}
	if failed.ProbeResult["port_listening"] != true || !bytes.Contains([]byte(failed.Stderr), []byte("目标端口已被占用")) {
		t.Fatalf("port-in-use failure was not recorded: %#v", failed)
	}
}

func TestCreateDeploymentRunFailsWhenProbeFindsUnsupportedTarget(t *testing.T) {
	store := newDeploymentTestStore(t)
	taskManager := service.NewTaskManager(store)
	executor := &fakeDeploymentScriptExecutor{
		outputs: []string{`SBM_PROGRESS {"step":"probe_system","status":"success","message":"System probe completed"}
SBM_RESULT {"status":"success","os":"linux","arch":"386","kernel":"6.8.0","privilege_mode":"root","systemd":true,"proxy_port":443,"port_listening":false}
`},
	}
	server := &Server{
		dbStore:              store,
		taskManager:          taskManager,
		taskHandler:          NewTaskHandler(store, taskManager),
		router:               gin.New(),
		deployScriptExecutor: executor,
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"template_name":"singbox-vless-reality",
		"dry_run":false,
		"ssh":{"host":"203.0.113.10","port":22,"user":"root","auth_method":"password","password":"ssh-secret"},
		"connection":{"mode":"direct"},
		"parameters":{"node_name":"edge-a","node_server":"vpn.example.com","proxy_port":443}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("POST /deployments/runs status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var resp struct {
		Data models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	failed := waitForDeploymentRunStatus(t, store, resp.Data.ID, models.DeploymentRunStatusFailed)
	if executor.calls != 1 {
		t.Fatalf("script executor calls = %d, want only probe", executor.calls)
	}
	if failed.ProbeResult["arch"] != "386" || !bytes.Contains([]byte(failed.Stderr), []byte("目标架构不受支持")) {
		t.Fatalf("unsupported target failure was not recorded: %#v", failed)
	}
}

func TestCreateDeploymentRunRecordsScriptFailure(t *testing.T) {
	store := newDeploymentTestStore(t)
	taskManager := service.NewTaskManager(store)
	executor := &fakeDeploymentScriptExecutor{
		outputs: []string{
			`SBM_PROGRESS {"step":"probe_system","status":"success","message":"System probe completed"}
SBM_RESULT {"status":"success","os":"linux","arch":"amd64","privilege_mode":"root","systemd":true,"proxy_port":443,"port_listening":false}
`,
			"remote preparing\nSBM_PROGRESS {\"step\":\"install\",\"status\":\"running\"}\n",
		},
		errors: []error{nil, context.DeadlineExceeded},
	}
	server := &Server{
		dbStore:              store,
		taskManager:          taskManager,
		taskHandler:          NewTaskHandler(store, taskManager),
		router:               gin.New(),
		deployScriptExecutor: executor,
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"template_name":"singbox-vless-reality",
		"dry_run":false,
		"ssh":{"host":"203.0.113.10","port":22,"user":"root","auth_method":"password","password":"ssh-secret"},
		"connection":{"mode":"direct"},
		"parameters":{"node_name":"edge-a","node_server":"vpn.example.com","proxy_port":443}
	}`)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("POST /deployments/runs status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var resp struct {
		Data models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	failed := waitForDeploymentRunStatus(t, store, resp.Data.ID, models.DeploymentRunStatusFailed)
	if failed.Status != models.DeploymentRunStatusFailed {
		t.Fatalf("status = %q, want failed", failed.Status)
	}
	if !strings.Contains(failed.Stdout, "remote preparing") || failed.Stderr == "" || failed.ExitCode == nil || *failed.ExitCode != 1 {
		t.Fatalf("failure output was not recorded: %#v", failed)
	}
}

func TestCreateDeploymentRunPreservesOutputWhenStructuredMarkerIsMalformed(t *testing.T) {
	store := newDeploymentTestStore(t)
	taskManager := service.NewTaskManager(store)
	executor := &fakeDeploymentScriptExecutor{
		outputs: []string{
			`SBM_PROGRESS {"step":"probe_system","status":"success","message":"System probe completed"}
SBM_RESULT {"status":"success","os":"linux","arch":"amd64","privilege_mode":"root","systemd":true,"proxy_port":443,"port_listening":false}
`,
			"remote preparing\nSBM_PROGRESS {\"step\":\nordinary log after malformed marker\n",
		},
	}
	server := &Server{
		dbStore:              store,
		taskManager:          taskManager,
		taskHandler:          NewTaskHandler(store, taskManager),
		router:               gin.New(),
		deployScriptExecutor: executor,
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"template_name":"singbox-vless-reality",
		"dry_run":false,
		"ssh":{"host":"203.0.113.10","port":22,"user":"root","auth_method":"password","password":"ssh-secret"},
		"connection":{"mode":"direct"},
		"parameters":{"node_name":"edge-a","node_server":"vpn.example.com","proxy_port":443}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("POST /deployments/runs status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var resp struct {
		Data models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	failed := waitForDeploymentRunStatus(t, store, resp.Data.ID, models.DeploymentRunStatusFailed)
	if !strings.Contains(failed.Stdout, "remote preparing") || !strings.Contains(failed.Stdout, "ordinary log after malformed marker") {
		t.Fatalf("malformed marker failure did not preserve stdout: %#v", failed)
	}
	if !strings.Contains(failed.Stderr, "parse SBM_PROGRESS JSON") {
		t.Fatalf("stderr = %q, want parse SBM_PROGRESS JSON", failed.Stderr)
	}
	if failed.ExitCode == nil || *failed.ExitCode != 1 {
		t.Fatalf("exit code = %v, want 1", failed.ExitCode)
	}
}

func TestCreateDeploymentRunPreservesStructuredScriptFailure(t *testing.T) {
	store := newDeploymentTestStore(t)
	taskManager := service.NewTaskManager(store)
	executor := &fakeDeploymentScriptExecutor{
		outputs: []string{
			`SBM_PROGRESS {"step":"probe_system","status":"success","message":"System probe completed"}
SBM_RESULT {"status":"success","os":"linux","arch":"amd64","privilege_mode":"root","systemd":true,"proxy_port":443,"port_listening":false}
`,
			`SBM_PROGRESS {"step":"install_singbox","status":"running"}
SBM_RESULT {"status":"failed","message":"curl or wget is required"}
`,
		},
		errors: []error{nil, errors.New("Process exited with status 1")},
	}
	server := &Server{
		dbStore:              store,
		taskManager:          taskManager,
		taskHandler:          NewTaskHandler(store, taskManager),
		router:               gin.New(),
		deployScriptExecutor: executor,
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"template_name":"singbox-vless-reality",
		"dry_run":false,
		"ssh":{"host":"203.0.113.10","port":22,"user":"root","auth_method":"password","password":"ssh-secret"},
		"connection":{"mode":"direct"},
		"parameters":{"node_name":"edge-a","node_server":"vpn.example.com","proxy_port":443}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("POST /deployments/runs status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var resp struct {
		Data models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	failed := waitForDeploymentRunStatus(t, store, resp.Data.ID, models.DeploymentRunStatusFailed)
	if failed.ProgressMarkers["install_singbox"] == nil {
		t.Fatalf("structured failure progress was not preserved: %#v", failed.ProgressMarkers)
	}
	result, ok := failed.ProgressMarkers["deployment_result"].(map[string]interface{})
	if !ok || result["status"] != "failed" || result["message"] != "curl or wget is required" {
		t.Fatalf("structured failure result was not preserved: %#v", failed.ProgressMarkers["deployment_result"])
	}
	if failed.ExitCode == nil || *failed.ExitCode != 1 {
		t.Fatalf("exit code = %v, want 1", failed.ExitCode)
	}
}

func TestCreateDeploymentRunFailsWhenScriptResultStatusFailed(t *testing.T) {
	store := newDeploymentTestStore(t)
	taskManager := service.NewTaskManager(store)
	executor := &fakeDeploymentScriptExecutor{
		outputs: []string{
			`SBM_PROGRESS {"step":"probe_system","status":"success","message":"System probe completed"}
SBM_RESULT {"status":"success","os":"linux","arch":"amd64","privilege_mode":"root","systemd":true,"proxy_port":443,"port_listening":false}
`,
			`SBM_PROGRESS {"step":"install_singbox","status":"failed"}
SBM_RESULT {"status":"failed","message":"curl or wget is required"}
`,
		},
	}
	server := &Server{
		dbStore:              store,
		taskManager:          taskManager,
		taskHandler:          NewTaskHandler(store, taskManager),
		router:               gin.New(),
		deployScriptExecutor: executor,
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"template_name":"singbox-vless-reality",
		"dry_run":false,
		"ssh":{"host":"203.0.113.10","port":22,"user":"root","auth_method":"password","password":"ssh-secret"},
		"connection":{"mode":"direct"},
		"parameters":{"node_name":"edge-a","node_server":"vpn.example.com","proxy_port":443}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("POST /deployments/runs status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var resp struct {
		Data models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	failed := waitForDeploymentRunStatus(t, store, resp.Data.ID, models.DeploymentRunStatusFailed)
	if !strings.Contains(failed.Stderr, "curl or wget is required") {
		t.Fatalf("stderr = %q, want script result message", failed.Stderr)
	}
	result, ok := failed.ProgressMarkers["deployment_result"].(map[string]interface{})
	if !ok || result["status"] != "failed" {
		t.Fatalf("deployment result was not preserved: %#v", failed.ProgressMarkers["deployment_result"])
	}
	if failed.ExitCode == nil || *failed.ExitCode != 1 {
		t.Fatalf("exit code = %v, want 1", failed.ExitCode)
	}
}

func TestCancelRunningDeploymentRunStopsBackgroundExecution(t *testing.T) {
	store := newDeploymentTestStore(t)
	taskManager := service.NewTaskManager(store)
	executorStarted := make(chan struct{})
	executor := &fakeDeploymentScriptExecutor{waitForCancel: true, started: executorStarted}
	server := &Server{
		dbStore:              store,
		taskManager:          taskManager,
		taskHandler:          NewTaskHandler(store, taskManager),
		router:               gin.New(),
		deployScriptExecutor: executor,
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"template_name":"singbox-vless-reality",
		"dry_run":false,
		"ssh":{"host":"203.0.113.10","port":22,"user":"root","auth_method":"password","password":"ssh-secret"},
		"connection":{"mode":"direct"},
		"parameters":{"node_name":"edge-a","node_server":"vpn.example.com","proxy_port":443}
	}`)
	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/deployments/runs", bytes.NewReader(body))
	createRequest.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusAccepted {
		t.Fatalf("POST /deployments/runs status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	var createResp struct {
		Data models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	select {
	case <-executorStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("deployment executor did not start")
	}

	cancelRecorder := httptest.NewRecorder()
	cancelRequest := httptest.NewRequest(http.MethodPost, "/api/deployments/runs/"+createResp.Data.ID+"/cancel", nil)
	server.router.ServeHTTP(cancelRecorder, cancelRequest)
	if cancelRecorder.Code != http.StatusOK {
		t.Fatalf("POST cancel status = %d, body = %s", cancelRecorder.Code, cancelRecorder.Body.String())
	}
	cancelled := waitForDeploymentRunCondition(t, store, createResp.Data.ID, func(run *models.DeploymentRun) bool {
		return run.Status == models.DeploymentRunStatusCancelled && run.Stderr != ""
	})
	if cancelled.Stderr == "" || cancelled.CompletedAt == nil {
		t.Fatalf("cancelled run did not retain cancellation details: %#v", cancelled)
	}
}

func TestRunningDeploymentRunPersistsProbeCheckpoint(t *testing.T) {
	store := newDeploymentTestStore(t)
	taskManager := service.NewTaskManager(store)
	executor := &fakeDeploymentScriptExecutor{
		outputs: []string{
			`SBM_PROGRESS {"step":"probe_system","status":"success","message":"System probe completed"}
SBM_RESULT {"status":"success","os":"linux","arch":"amd64","privilege_mode":"root","systemd":true,"proxy_port":443,"port_listening":false}
`,
		},
		waitForCancel:     true,
		waitForCancelCall: 2,
	}
	server := &Server{
		dbStore:              store,
		taskManager:          taskManager,
		taskHandler:          NewTaskHandler(store, taskManager),
		router:               gin.New(),
		deployScriptExecutor: executor,
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"template_name":"singbox-vless-reality",
		"dry_run":false,
		"ssh":{"host":"203.0.113.10","port":22,"user":"root","auth_method":"password","password":"ssh-secret"},
		"connection":{"mode":"direct"},
		"parameters":{"node_name":"edge-a","node_server":"vpn.example.com","proxy_port":443}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("POST /deployments/runs status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var createResp struct {
		Data models.DeploymentRun `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	running := waitForDeploymentRunCondition(t, store, createResp.Data.ID, func(run *models.DeploymentRun) bool {
		return run.Status == models.DeploymentRunStatusRunning && run.ProbeResult["systemd"] == true && run.ProgressMarkers["probe_system"] != nil
	})
	if !strings.Contains(running.Stdout, "probe_system") {
		t.Fatalf("running checkpoint did not persist probe stdout: %#v", running)
	}
	task, err := store.GetTask(createResp.Data.TaskID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if task.Progress < 25 || task.CurrentItem == "" {
		t.Fatalf("task progress was not checkpointed: %#v", task)
	}

	cancelRecorder := httptest.NewRecorder()
	cancelRequest := httptest.NewRequest(http.MethodPost, "/api/deployments/runs/"+createResp.Data.ID+"/cancel", nil)
	server.router.ServeHTTP(cancelRecorder, cancelRequest)
	if cancelRecorder.Code != http.StatusOK {
		t.Fatalf("POST cancel status = %d, body = %s", cancelRecorder.Code, cancelRecorder.Body.String())
	}
	_ = waitForDeploymentRunStatus(t, store, createResp.Data.ID, models.DeploymentRunStatusCancelled)
}

func TestUploadDeploymentRuntimeArchiveRejectsChecksumMismatch(t *testing.T) {
	server := &Server{router: gin.New(), baseDir: t.TempDir()}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("runtime_name", "sing-box"); err != nil {
		t.Fatalf("WriteField(runtime_name): %v", err)
	}
	if err := writer.WriteField("version", deploy.PinnedSingBoxVersion); err != nil {
		t.Fatalf("WriteField(version): %v", err)
	}
	if err := writer.WriteField("os", "linux"); err != nil {
		t.Fatalf("WriteField(os): %v", err)
	}
	if err := writer.WriteField("arch", "amd64"); err != nil {
		t.Fatalf("WriteField(arch): %v", err)
	}
	part, err := writer.CreateFormFile("file", "sing-box.tar.gz")
	if err != nil {
		t.Fatalf("CreateFormFile(): %v", err)
	}
	if _, err := part.Write([]byte("archive")); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	uploadRecorder := httptest.NewRecorder()
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/deployments/runtime-cache", &body)
	uploadRequest.Header.Set("Content-Type", writer.FormDataContentType())
	server.router.ServeHTTP(uploadRecorder, uploadRequest)
	if uploadRecorder.Code != http.StatusBadRequest {
		t.Fatalf("POST runtime-cache status = %d, body = %s", uploadRecorder.Code, uploadRecorder.Body.String())
	}
	if !bytes.Contains(uploadRecorder.Body.Bytes(), []byte("checksum mismatch")) {
		t.Fatalf("upload response does not explain checksum mismatch: %s", uploadRecorder.Body.String())
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/deployments/runtime-cache?runtime_name=sing-box&version="+deploy.PinnedSingBoxVersion, nil)
	server.router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("GET runtime-cache status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	var listResp struct {
		Data []deploy.RuntimeArchive `json:"data"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	byArch := make(map[string]deploy.RuntimeArchive, len(listResp.Data))
	for _, archive := range listResp.Data {
		byArch[archive.Arch] = archive
	}
	if archive := byArch["amd64"]; archive.Status != "missing" || archive.Path == "" || archive.Checksum != "" || archive.ExpectedChecksum == "" {
		t.Fatalf("unexpected amd64 runtime cache status after rejected upload: %#v", archive)
	}
	if archive := byArch["arm64"]; archive.Status != "missing" || archive.Path == "" || archive.Checksum != "" || archive.ExpectedChecksum == "" {
		t.Fatalf("unexpected arm64 runtime cache status: %#v", archive)
	}
}

func TestRuntimeCacheAPIRejectsNonPinnedRuntime(t *testing.T) {
	baseDir := t.TempDir()
	server := &Server{router: gin.New(), baseDir: baseDir}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/deployments/runtime-cache?runtime_name=sing-box&version=1.14.0", nil)
	server.router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusBadRequest {
		t.Fatalf("GET runtime-cache status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	if !bytes.Contains(listRecorder.Body.Bytes(), []byte(deploy.PinnedSingBoxVersion)) {
		t.Fatalf("GET runtime-cache response does not explain pinned version: %s", listRecorder.Body.String())
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("runtime_name", "sing-box"); err != nil {
		t.Fatalf("WriteField(runtime_name): %v", err)
	}
	if err := writer.WriteField("version", "1.14.0"); err != nil {
		t.Fatalf("WriteField(version): %v", err)
	}
	if err := writer.WriteField("os", "linux"); err != nil {
		t.Fatalf("WriteField(os): %v", err)
	}
	if err := writer.WriteField("arch", "amd64"); err != nil {
		t.Fatalf("WriteField(arch): %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	uploadRecorder := httptest.NewRecorder()
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/deployments/runtime-cache", &body)
	uploadRequest.Header.Set("Content-Type", writer.FormDataContentType())
	server.router.ServeHTTP(uploadRecorder, uploadRequest)
	if uploadRecorder.Code != http.StatusBadRequest {
		t.Fatalf("POST runtime-cache status = %d, body = %s", uploadRecorder.Code, uploadRecorder.Body.String())
	}
	if !bytes.Contains(uploadRecorder.Body.Bytes(), []byte(deploy.PinnedSingBoxVersion)) {
		t.Fatalf("POST runtime-cache response does not explain pinned version: %s", uploadRecorder.Body.String())
	}
	if _, err := os.Stat(filepath.Join(baseDir, "runtime-cache", "sing-box", "1.14.0")); !os.IsNotExist(err) {
		t.Fatalf("unsupported runtime cache directory was created, stat err = %v", err)
	}
}

func TestDeploymentRunHistoryRedactsGeneratedNodeSecrets(t *testing.T) {
	store := newDeploymentTestStore(t)
	server := &Server{
		dbStore: store,
		router:  gin.New(),
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))
	run := &models.DeploymentRun{
		ID:              "run-redacted",
		TemplateName:    "singbox-vless-reality",
		TemplateVersion: "1.0.0",
		Status:          models.DeploymentRunStatusSuccess,
		GeneratedNode: models.JSONMap{
			"type":        "vless",
			"tag":         "edge-a",
			"server":      "vpn.example.com",
			"server_port": float64(443),
			"extra": map[string]interface{}{
				"uuid": "generated-secret",
				"tls": map[string]interface{}{
					"reality": map[string]interface{}{
						"public_key": "generated-public",
						"short_id":   "short-secret",
					},
				},
			},
		},
		Stdout: `installing
SBM_NODE_BEGIN
{"type":"vless","tag":"edge-a","server":"vpn.example.com","server_port":443,"extra":{"uuid":"generated-secret","tls":{"reality":{"public_key":"generated-public","private_key":"generated-private","short_id":"short-secret"}}}}
SBM_NODE_END
SBM_RESULT {"status":"success","api_token":"result-token"}
`,
	}
	if err := store.CreateDeploymentRun(run); err != nil {
		t.Fatalf("CreateDeploymentRun() error = %v", err)
	}

	detailRecorder := httptest.NewRecorder()
	detailRequest := httptest.NewRequest(http.MethodGet, "/api/deployments/runs/run-redacted", nil)
	server.router.ServeHTTP(detailRecorder, detailRequest)
	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("GET deployment run status = %d, body = %s", detailRecorder.Code, detailRecorder.Body.String())
	}
	if bytes.Contains(detailRecorder.Body.Bytes(), []byte("generated-secret")) ||
		bytes.Contains(detailRecorder.Body.Bytes(), []byte("generated-public")) ||
		bytes.Contains(detailRecorder.Body.Bytes(), []byte("generated-private")) ||
		bytes.Contains(detailRecorder.Body.Bytes(), []byte("short-secret")) ||
		bytes.Contains(detailRecorder.Body.Bytes(), []byte("result-token")) {
		t.Fatalf("detail response leaked generated node secrets: %s", detailRecorder.Body.String())
	}
	if !bytes.Contains(detailRecorder.Body.Bytes(), []byte("[REDACTED]")) {
		t.Fatalf("detail response did not include redaction marker: %s", detailRecorder.Body.String())
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/deployments/runs", nil)
	server.router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("GET deployment runs status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	if bytes.Contains(listRecorder.Body.Bytes(), []byte("generated-secret")) ||
		bytes.Contains(listRecorder.Body.Bytes(), []byte("generated-public")) ||
		bytes.Contains(listRecorder.Body.Bytes(), []byte("generated-private")) ||
		bytes.Contains(listRecorder.Body.Bytes(), []byte("short-secret")) ||
		bytes.Contains(listRecorder.Body.Bytes(), []byte("result-token")) {
		t.Fatalf("list response leaked generated node secrets: %s", listRecorder.Body.String())
	}

	stored, err := store.GetDeploymentRun("run-redacted")
	if err != nil {
		t.Fatalf("GetDeploymentRun() error = %v", err)
	}
	extra := stored.GeneratedNode["extra"].(map[string]interface{})
	if extra["uuid"] != "generated-secret" {
		t.Fatalf("stored generated node was mutated: %#v", stored.GeneratedNode)
	}
	if !strings.Contains(stored.Stdout, "generated-private") {
		t.Fatalf("stored stdout was mutated: %q", stored.Stdout)
	}
}

func TestDeploymentGeneratedNodeReviewReturnsUnredactedPayloadExplicitly(t *testing.T) {
	store := newDeploymentTestStore(t)
	server := &Server{
		dbStore: store,
		router:  gin.New(),
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))
	run := &models.DeploymentRun{
		ID:              "run-review",
		TemplateName:    "singbox-vless-reality",
		TemplateVersion: "1.0.0",
		Status:          models.DeploymentRunStatusSuccess,
		DryRun:          false,
		GeneratedNode: models.JSONMap{
			"type":        "vless",
			"tag":         "edge-a",
			"server":      "vpn.example.com",
			"server_port": float64(443),
			"extra": map[string]interface{}{
				"uuid": "generated-secret",
				"tls": map[string]interface{}{
					"reality": map[string]interface{}{
						"public_key": "generated-public",
					},
				},
			},
		},
	}
	if err := store.CreateDeploymentRun(run); err != nil {
		t.Fatalf("CreateDeploymentRun() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/deployments/runs/run-review/generated-node", nil)
	server.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET generated node review status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("generated-secret")) ||
		!bytes.Contains(recorder.Body.Bytes(), []byte("generated-public")) {
		t.Fatalf("generated node review did not include import-capable payload: %s", recorder.Body.String())
	}
	var resp struct {
		Data deploymentGeneratedNodeReview `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Data.DefaultTag != "edge-a" || !resp.Data.CanImport {
		t.Fatalf("unexpected review metadata: %#v", resp.Data)
	}
	if resp.Data.ShareLink == "" ||
		!strings.HasPrefix(resp.Data.ShareLink, "vless://generated-secret@vpn.example.com:443?") ||
		!strings.Contains(resp.Data.ShareLink, "security=reality") ||
		!strings.Contains(resp.Data.ShareLink, "pbk=generated-public") ||
		!strings.HasSuffix(resp.Data.ShareLink, "#edge-a") {
		t.Fatalf("generated node review share link missing VLESS Reality fields: %q", resp.Data.ShareLink)
	}
}

func TestDeploymentGeneratedNodeReviewRejectsDryRun(t *testing.T) {
	store := newDeploymentTestStore(t)
	server := &Server{
		dbStore: store,
		router:  gin.New(),
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))
	run := &models.DeploymentRun{
		ID:              "dry-run-review",
		TemplateName:    "singbox-vless-reality",
		TemplateVersion: "1.0.0",
		Status:          models.DeploymentRunStatusSuccess,
		DryRun:          true,
		GeneratedNode: models.JSONMap{
			"type":        "vless",
			"tag":         "edge-preview",
			"server":      "vpn.example.com",
			"server_port": float64(443),
		},
	}
	if err := store.CreateDeploymentRun(run); err != nil {
		t.Fatalf("CreateDeploymentRun() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/deployments/runs/dry-run-review/generated-node", nil)
	server.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("GET dry-run generated node review status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("dry-run")) {
		t.Fatalf("dry-run response does not explain preview-only generated node: %s", recorder.Body.String())
	}
}

func TestImportDeploymentGeneratedNodeCreatesManualNodeWithProvenance(t *testing.T) {
	store := newDeploymentTestStore(t)
	jsonStore, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}
	taskManager := service.NewTaskManager(store)
	server := &Server{
		dbStore:     store,
		store:       jsonStore,
		taskManager: taskManager,
		taskHandler: NewTaskHandler(store, taskManager),
		router:      gin.New(),
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	run := &models.DeploymentRun{
		ID:              "run-1",
		TemplateName:    "singbox-vless-reality",
		TemplateVersion: "1.0.0",
		Status:          models.DeploymentRunStatusSuccess,
		SSHHost:         "203.0.113.10",
		SSHPort:         22,
		SSHUser:         "root",
		NodeServer:      "vpn.example.com",
		NodeProxyPort:   443,
		ConnectionMode:  models.DeploymentConnectionDirect,
		GeneratedNode: models.JSONMap{
			"type":        "vless",
			"tag":         "edge-a",
			"server":      "vpn.example.com",
			"server_port": float64(443),
			"extra": map[string]interface{}{
				"uuid": "generated-secret",
				"tls": map[string]interface{}{
					"enabled":     true,
					"server_name": "www.microsoft.com",
					"reality": map[string]interface{}{
						"enabled":    true,
						"public_key": "generated-public",
						"short_id":   "0123456789abcdef",
					},
				},
			},
		},
	}
	if err := store.CreateDeploymentRun(run); err != nil {
		t.Fatalf("CreateDeploymentRun() error = %v", err)
	}

	body := []byte(`{"tag":"edge-a-imported","source_name":"自建节点","enabled":true}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs/run-1/import-node", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("POST import status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var resp struct {
		Data models.Node `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode import response: %v", err)
	}
	if resp.Data.Source != "manual" {
		t.Fatalf("source = %q, want manual", resp.Data.Source)
	}
	if resp.Data.SourceName != "自建节点" {
		t.Fatalf("source_name = %q, want 自建节点", resp.Data.SourceName)
	}
	if resp.Data.Tag != "edge-a-imported" || resp.Data.Type != "vless" || resp.Data.Server != "vpn.example.com" || resp.Data.ServerPort != 443 {
		t.Fatalf("unexpected imported node: %#v", resp.Data)
	}
	if resp.Data.Extra["node_origin"] != "deployed_self_hosted" {
		t.Fatalf("node origin not recorded in Extra: %#v", resp.Data.Extra)
	}
	if resp.Data.Extra["entry_method"] != "deployment_import" {
		t.Fatalf("entry method not recorded in Extra: %#v", resp.Data.Extra)
	}
	if resp.Data.Extra["deployment_run_id"] != "run-1" {
		t.Fatalf("deployment run link not recorded in Extra: %#v", resp.Data.Extra)
	}
	manualNodes := jsonStore.GetManualNodes()
	if len(manualNodes) != 1 {
		t.Fatalf("manual node count = %d, want 1", len(manualNodes))
	}
	if manualNodes[0].Node.Tag != "edge-a-imported" {
		t.Fatalf("manual node tag = %q, want edge-a-imported", manualNodes[0].Node.Tag)
	}
	if manualNodes[0].Node.Source != "manual" || manualNodes[0].Node.SourceName != "自建节点" {
		t.Fatalf("manual node source metadata = %q/%q, want manual/自建节点", manualNodes[0].Node.Source, manualNodes[0].Node.SourceName)
	}
	importedTLS, ok := manualNodes[0].Node.Extra["tls"].(map[string]interface{})
	if !ok || importedTLS["enabled"] != true {
		t.Fatalf("imported node tls shape = %#v", manualNodes[0].Node.Extra["tls"])
	}
	if _, ok := importedTLS["reality"].(map[string]interface{}); !ok {
		t.Fatalf("imported node reality shape = %#v", importedTLS)
	}

	updatedRun, err := store.GetDeploymentRun("run-1")
	if err != nil {
		t.Fatalf("GetDeploymentRun() error = %v", err)
	}
	if updatedRun.ImportedNodeID == nil || *updatedRun.ImportedNodeID != resp.Data.ID {
		t.Fatalf("run imported node link = %v, want %d", updatedRun.ImportedNodeID, resp.Data.ID)
	}

	reviewRecorder := httptest.NewRecorder()
	reviewRequest := httptest.NewRequest(http.MethodGet, "/api/deployments/runs/run-1/generated-node", nil)
	server.router.ServeHTTP(reviewRecorder, reviewRequest)
	if reviewRecorder.Code != http.StatusOK {
		t.Fatalf("GET generated node review after import status = %d, body = %s", reviewRecorder.Code, reviewRecorder.Body.String())
	}
	var reviewResp struct {
		Data deploymentGeneratedNodeReview `json:"data"`
	}
	if err := json.Unmarshal(reviewRecorder.Body.Bytes(), &reviewResp); err != nil {
		t.Fatalf("decode generated node review: %v", err)
	}
	if reviewResp.Data.CanImport {
		t.Fatalf("review can_import = true after import: %#v", reviewResp.Data)
	}
	if reviewResp.Data.ImportedNodeID == nil || *reviewResp.Data.ImportedNodeID != resp.Data.ID {
		t.Fatalf("review imported_node_id = %v, want %d", reviewResp.Data.ImportedNodeID, resp.Data.ID)
	}
	if reviewResp.Data.GeneratedNode["extra"] == nil {
		t.Fatalf("review generated_node lost payload after import: %#v", reviewResp.Data.GeneratedNode)
	}
}

func TestImportDeploymentGeneratedNodeAppearsInExistingNodeLists(t *testing.T) {
	store := newDeploymentTestStore(t)
	jsonStore, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}
	server := &Server{
		dbStore: store,
		store:   jsonStore,
		router:  gin.New(),
	}
	api := server.router.Group("/api")
	server.registerDeploymentRoutes(api)
	api.GET("/manual-nodes", server.getManualNodes)
	api.GET("/nodes/grouped", server.getNodesGrouped)

	run := &models.DeploymentRun{
		ID:              "run-listing",
		TemplateName:    "singbox-vless-reality",
		TemplateVersion: "1.0.0",
		Status:          models.DeploymentRunStatusSuccess,
		GeneratedNode: models.JSONMap{
			"type":        "vless",
			"tag":         "edge-listing",
			"server":      "vpn.example.com",
			"server_port": float64(443),
			"extra": map[string]interface{}{
				"uuid": "generated-secret",
				"tls": map[string]interface{}{
					"enabled":     true,
					"server_name": "www.microsoft.com",
					"reality": map[string]interface{}{
						"enabled":    true,
						"public_key": "generated-public",
						"short_id":   "0123456789abcdef",
					},
				},
			},
		},
	}
	if err := store.CreateDeploymentRun(run); err != nil {
		t.Fatalf("CreateDeploymentRun() error = %v", err)
	}

	importRecorder := httptest.NewRecorder()
	importRequest := httptest.NewRequest(http.MethodPost, "/api/deployments/runs/run-listing/import-node", bytes.NewReader([]byte(`{"tag":"edge-listing-imported","source_name":"自建节点","enabled":true}`)))
	importRequest.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(importRecorder, importRequest)
	if importRecorder.Code != http.StatusOK {
		t.Fatalf("POST import status = %d, body = %s", importRecorder.Code, importRecorder.Body.String())
	}

	manualRecorder := httptest.NewRecorder()
	manualRequest := httptest.NewRequest(http.MethodGet, "/api/manual-nodes", nil)
	server.router.ServeHTTP(manualRecorder, manualRequest)
	if manualRecorder.Code != http.StatusOK {
		t.Fatalf("GET manual-nodes status = %d, body = %s", manualRecorder.Code, manualRecorder.Body.String())
	}
	var manualResp struct {
		Data []storage.ManualNode `json:"data"`
	}
	if err := json.Unmarshal(manualRecorder.Body.Bytes(), &manualResp); err != nil {
		t.Fatalf("decode manual nodes: %v", err)
	}
	if len(manualResp.Data) != 1 || manualResp.Data[0].Node.Tag != "edge-listing-imported" || manualResp.Data[0].Node.SourceName != "自建节点" {
		t.Fatalf("imported node missing from manual list: %#v", manualResp.Data)
	}

	groupedRecorder := httptest.NewRecorder()
	groupedRequest := httptest.NewRequest(http.MethodGet, "/api/nodes/grouped", nil)
	server.router.ServeHTTP(groupedRecorder, groupedRequest)
	if groupedRecorder.Code != http.StatusOK {
		t.Fatalf("GET nodes/grouped status = %d, body = %s", groupedRecorder.Code, groupedRecorder.Body.String())
	}
	var groupedResp struct {
		Data []storage.NodeGroup `json:"data"`
	}
	if err := json.Unmarshal(groupedRecorder.Body.Bytes(), &groupedResp); err != nil {
		t.Fatalf("decode grouped nodes: %v", err)
	}
	if !groupedNodesContain(groupedResp.Data, "manual:自建节点", "自建节点", "edge-listing-imported") {
		t.Fatalf("imported node missing from grouped node workflow: %#v", groupedResp.Data)
	}
}

func TestImportDeploymentGeneratedNodeCanCreateDisabledManualNode(t *testing.T) {
	store := newDeploymentTestStore(t)
	jsonStore, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}
	server := &Server{
		dbStore: store,
		store:   jsonStore,
		router:  gin.New(),
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	run := &models.DeploymentRun{
		ID:              "run-disabled-import",
		TemplateName:    "singbox-vless-reality",
		TemplateVersion: "1.0.0",
		Status:          models.DeploymentRunStatusSuccess,
		GeneratedNode: models.JSONMap{
			"type":        "vless",
			"tag":         "edge-disabled",
			"server":      "vpn.example.com",
			"server_port": float64(443),
			"extra": map[string]interface{}{
				"uuid": "generated-secret",
			},
		},
	}
	if err := store.CreateDeploymentRun(run); err != nil {
		t.Fatalf("CreateDeploymentRun() error = %v", err)
	}

	body := []byte(`{"tag":"edge-disabled","enabled":false}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs/run-disabled-import/import-node", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("POST disabled import status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var resp struct {
		Data models.Node `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode import response: %v", err)
	}
	if resp.Data.Enabled {
		t.Fatalf("imported SQLite node enabled = true, want false")
	}
	manualNodes := jsonStore.GetManualNodes()
	if len(manualNodes) != 1 || manualNodes[0].Enabled {
		t.Fatalf("disabled manual node was not persisted as disabled: %#v", manualNodes)
	}
	updatedRun, err := store.GetDeploymentRun("run-disabled-import")
	if err != nil {
		t.Fatalf("GetDeploymentRun() error = %v", err)
	}
	if updatedRun.ImportedNodeID == nil || *updatedRun.ImportedNodeID != resp.Data.ID {
		t.Fatalf("run imported node link = %v, want %d", updatedRun.ImportedNodeID, resp.Data.ID)
	}
}

func TestImportDeploymentGeneratedNodeRejectsDuplicateTag(t *testing.T) {
	store := newDeploymentTestStore(t)
	jsonStore, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}
	if err := jsonStore.AddManualNode(storage.ManualNode{
		ID:      "manual-existing",
		Enabled: false,
		Node: storage.Node{
			Tag:        "edge-a",
			Type:       "socks",
			Server:     "127.0.0.1",
			ServerPort: 1080,
		},
	}); err != nil {
		t.Fatalf("AddManualNode() error = %v", err)
	}
	taskManager := service.NewTaskManager(store)
	server := &Server{
		dbStore:     store,
		store:       jsonStore,
		taskManager: taskManager,
		taskHandler: NewTaskHandler(store, taskManager),
		router:      gin.New(),
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	run := &models.DeploymentRun{
		ID:              "run-duplicate-tag",
		TemplateName:    "singbox-vless-reality",
		TemplateVersion: "1.0.0",
		Status:          models.DeploymentRunStatusSuccess,
		GeneratedNode: models.JSONMap{
			"type":        "vless",
			"tag":         "edge-a",
			"server":      "vpn.example.com",
			"server_port": float64(443),
		},
	}
	if err := store.CreateDeploymentRun(run); err != nil {
		t.Fatalf("CreateDeploymentRun() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs/run-duplicate-tag/import-node", bytes.NewReader([]byte(`{"tag":"edge-a","enabled":true}`)))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("POST duplicate import status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("节点 tag 已存在")) {
		t.Fatalf("duplicate response did not identify tag conflict: %s", recorder.Body.String())
	}
	if len(jsonStore.GetManualNodes()) != 1 {
		t.Fatalf("duplicate import changed manual nodes: %#v", jsonStore.GetManualNodes())
	}
	updatedRun, err := store.GetDeploymentRun("run-duplicate-tag")
	if err != nil {
		t.Fatalf("GetDeploymentRun() error = %v", err)
	}
	if updatedRun.ImportedNodeID != nil {
		t.Fatalf("duplicate import marked run as imported: %#v", updatedRun.ImportedNodeID)
	}
}

func TestImportDeploymentGeneratedNodeRejectsDuplicateTagFromDisabledSubscription(t *testing.T) {
	store := newDeploymentTestStore(t)
	jsonStore, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}
	if err := jsonStore.AddSubscription(storage.Subscription{
		ID:      "sub-disabled",
		Name:    "Disabled Sub",
		Enabled: false,
		Nodes: []storage.Node{
			{
				Tag:        "edge-a",
				Type:       "socks",
				Server:     "127.0.0.1",
				ServerPort: 1080,
			},
		},
	}); err != nil {
		t.Fatalf("AddSubscription() error = %v", err)
	}
	server := &Server{
		dbStore: store,
		store:   jsonStore,
		router:  gin.New(),
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	run := &models.DeploymentRun{
		ID:              "run-duplicate-sub-tag",
		TemplateName:    "singbox-vless-reality",
		TemplateVersion: "1.0.0",
		Status:          models.DeploymentRunStatusSuccess,
		GeneratedNode: models.JSONMap{
			"type":        "vless",
			"tag":         "edge-a",
			"server":      "vpn.example.com",
			"server_port": float64(443),
		},
	}
	if err := store.CreateDeploymentRun(run); err != nil {
		t.Fatalf("CreateDeploymentRun() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs/run-duplicate-sub-tag/import-node", bytes.NewReader([]byte(`{"tag":"edge-a","enabled":true}`)))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("POST duplicate subscription import status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("节点 tag 已存在")) {
		t.Fatalf("duplicate subscription response did not identify tag conflict: %s", recorder.Body.String())
	}
	if len(jsonStore.GetManualNodes()) != 0 {
		t.Fatalf("duplicate subscription import created manual nodes: %#v", jsonStore.GetManualNodes())
	}
}

func TestImportDeploymentGeneratedNodeRejectsProtocolOverridesWithoutAdvancedOptIn(t *testing.T) {
	store := newDeploymentTestStore(t)
	jsonStore, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}
	server := &Server{
		dbStore: store,
		store:   jsonStore,
		router:  gin.New(),
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	run := &models.DeploymentRun{
		ID:              "run-advanced-required",
		TemplateName:    "singbox-vless-reality",
		TemplateVersion: "1.0.0",
		Status:          models.DeploymentRunStatusSuccess,
		GeneratedNode: models.JSONMap{
			"type":        "vless",
			"tag":         "edge-a",
			"server":      "vpn.example.com",
			"server_port": float64(443),
			"extra": map[string]interface{}{
				"uuid": "generated-secret",
			},
		},
	}
	if err := store.CreateDeploymentRun(run); err != nil {
		t.Fatalf("CreateDeploymentRun() error = %v", err)
	}

	body := []byte(`{"tag":"edge-a-imported","enabled":true,"generated_node_overrides":{"server":"override.example.com"}}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs/run-advanced-required/import-node", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("POST import status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("advanced_protocol_edit")) {
		t.Fatalf("response %s does not identify advanced opt-in", recorder.Body.String())
	}
	if len(jsonStore.GetManualNodes()) != 0 {
		t.Fatalf("rejected advanced import created manual nodes: %#v", jsonStore.GetManualNodes())
	}
}

func TestImportDeploymentGeneratedNodeAppliesAdvancedProtocolOverrides(t *testing.T) {
	store := newDeploymentTestStore(t)
	jsonStore, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}
	server := &Server{
		dbStore: store,
		store:   jsonStore,
		router:  gin.New(),
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	run := &models.DeploymentRun{
		ID:              "run-advanced-import",
		TemplateName:    "singbox-vless-reality",
		TemplateVersion: "1.0.0",
		Status:          models.DeploymentRunStatusSuccess,
		GeneratedNode: models.JSONMap{
			"type":        "vless",
			"tag":         "edge-a",
			"server":      "vpn.example.com",
			"server_port": float64(443),
			"extra": map[string]interface{}{
				"uuid": "generated-secret",
				"tls": map[string]interface{}{
					"enabled": true,
				},
			},
		},
	}
	if err := store.CreateDeploymentRun(run); err != nil {
		t.Fatalf("CreateDeploymentRun() error = %v", err)
	}

	body := []byte(`{
		"tag":"edge-a-advanced",
		"enabled":true,
		"advanced_protocol_edit":true,
		"generated_node_overrides":{
			"server":"override.example.com",
			"server_port":8443,
			"extra":{"uuid":"edited-secret","flow":"xtls-rprx-vision"}
		}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs/run-advanced-import/import-node", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("POST import status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	manualNodes := jsonStore.GetManualNodes()
	if len(manualNodes) != 1 {
		t.Fatalf("manual node count = %d, want 1", len(manualNodes))
	}
	node := manualNodes[0].Node
	if node.Tag != "edge-a-advanced" || node.Server != "override.example.com" || node.ServerPort != 8443 {
		t.Fatalf("advanced override was not applied: %#v", node)
	}
	if node.Extra["uuid"] != "edited-secret" || node.Extra["flow"] != "xtls-rprx-vision" {
		t.Fatalf("advanced extra override was not applied: %#v", node.Extra)
	}
	if node.Source != "manual" || node.Extra["node_origin"] != "deployed_self_hosted" || node.Extra["entry_method"] != "deployment_import" {
		t.Fatalf("advanced import lost provenance: %#v", node)
	}
}

func TestSyncNodesToSQLiteUpdatesExistingNodeExtra(t *testing.T) {
	dbStore := newDeploymentTestStore(t)
	jsonStore, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	if err := dbStore.CreateNode(&models.Node{
		Tag:         "edge-a",
		Type:        "vless",
		Server:      "vpn.example.com",
		ServerPort:  443,
		Source:      "manual",
		SourceName:  "手动添加",
		Extra:       models.JSONMap{"flow": "xtls-rprx-vision", "tls": true},
		Enabled:     true,
		DelayStatus: "untested",
		SpeedStatus: "untested",
	}); err != nil {
		t.Fatalf("CreateNode() error = %v", err)
	}
	if err := jsonStore.AddManualNode(storage.ManualNode{
		ID: "manual-1",
		Node: storage.Node{
			Tag:        "edge-a",
			Type:       "vless",
			Server:     "vpn.example.com",
			ServerPort: 443,
			SourceName: "自建节点",
			Extra: map[string]interface{}{
				"uuid": "generated-uuid",
				"flow": "xtls-rprx-vision",
				"tls": map[string]interface{}{
					"enabled":     true,
					"server_name": "www.microsoft.com",
					"reality": map[string]interface{}{
						"enabled":    true,
						"public_key": "generated-public",
						"short_id":   "0123456789abcdef",
					},
				},
			},
		},
		Enabled: true,
	}); err != nil {
		t.Fatalf("AddManualNode() error = %v", err)
	}

	server := &Server{dbStore: dbStore, store: jsonStore}
	if err := server.syncNodesToSQLite(); err != nil {
		t.Fatalf("syncNodesToSQLite() error = %v", err)
	}

	node, err := dbStore.GetNodeByTag("edge-a")
	if err != nil {
		t.Fatalf("GetNodeByTag() error = %v", err)
	}
	if node.Extra["uuid"] != "generated-uuid" {
		t.Fatalf("uuid was not synced into SQLite extra: %#v", node.Extra)
	}
	if node.Source != "manual" || node.SourceName != "自建节点" {
		t.Fatalf("manual source metadata was not synced into SQLite: %q/%q", node.Source, node.SourceName)
	}
	tls, ok := node.Extra["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls type = %T, want map[string]interface{}", node.Extra["tls"])
	}
	reality, ok := tls["reality"].(map[string]interface{})
	if !ok || reality["public_key"] != "generated-public" || reality["short_id"] != "0123456789abcdef" {
		t.Fatalf("reality was not synced into SQLite extra: %#v", tls)
	}
}

func TestImportDeploymentGeneratedNodeRejectsDryRun(t *testing.T) {
	store := newDeploymentTestStore(t)
	jsonStore, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}
	server := &Server{
		dbStore: store,
		store:   jsonStore,
		router:  gin.New(),
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	run := &models.DeploymentRun{
		ID:            "dry-run-1",
		TemplateName:  "singbox-vless-reality",
		Status:        models.DeploymentRunStatusSuccess,
		DryRun:        true,
		SSHHost:       "203.0.113.10",
		SSHPort:       22,
		SSHUser:       "root",
		NodeServer:    "vpn.example.com",
		NodeProxyPort: 443,
		GeneratedNode: models.JSONMap{
			"type":        "vless",
			"tag":         "edge-preview",
			"server":      "vpn.example.com",
			"server_port": float64(443),
		},
	}
	if err := store.CreateDeploymentRun(run); err != nil {
		t.Fatalf("CreateDeploymentRun() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/runs/dry-run-1/import-node", bytes.NewReader([]byte(`{"tag":"edge-preview","enabled":true}`)))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("POST import dry-run status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("dry-run")) {
		t.Fatalf("response %s does not explain dry-run rejection", recorder.Body.String())
	}
	if len(jsonStore.GetManualNodes()) != 0 {
		t.Fatalf("dry-run import created manual nodes: %#v", jsonStore.GetManualNodes())
	}
}

func TestListDeploymentTemplatesExposesSupportedTemplateMetadata(t *testing.T) {
	server := &Server{router: gin.New()}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/deployments/templates", nil)
	server.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET templates status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var resp struct {
		Data []struct {
			Name           string `json:"name"`
			Version        string `json:"version"`
			Checksum       string `json:"checksum"`
			UserSelectable bool   `json:"user_selectable"`
			Parameters     []struct {
				Name     string   `json:"name"`
				Type     string   `json:"type"`
				Required bool     `json:"required"`
				Default  string   `json:"default,omitempty"`
				Options  []string `json:"options,omitempty"`
			} `json:"parameters"`
			Runtime struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"runtime"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode templates response: %v", err)
	}
	var found bool
	for _, template := range resp.Data {
		if template.Name == "singbox-vless-reality" {
			found = true
			if !template.UserSelectable || template.Version == "" || template.Checksum == "" {
				t.Fatalf("bad template metadata: %#v", template)
			}
			if template.Runtime.Name != "sing-box" || template.Runtime.Version == "" {
				t.Fatalf("bad runtime metadata: %#v", template.Runtime)
			}
			params := make(map[string]struct {
				Type     string
				Required bool
				Default  string
				Options  []string
			}, len(template.Parameters))
			for _, param := range template.Parameters {
				params[param.Name] = struct {
					Type     string
					Required bool
					Default  string
					Options  []string
				}{
					Type:     param.Type,
					Required: param.Required,
					Default:  param.Default,
					Options:  param.Options,
				}
			}
			if params["node_server"].Type != "string" || !params["node_server"].Required {
				t.Fatalf("node_server parameter metadata missing: %#v", template.Parameters)
			}
			if params["proxy_port"].Type != "integer" || params["proxy_port"].Default != "443" {
				t.Fatalf("proxy_port parameter metadata missing: %#v", template.Parameters)
			}
			runtimeSource := params["runtime_source"]
			if runtimeSource.Type != "select" || runtimeSource.Default != "remote" || !slices.Equal(runtimeSource.Options, []string{"remote", "cache"}) {
				t.Fatalf("runtime_source parameter metadata missing: %#v", template.Parameters)
			}
		}
	}
	if !found {
		t.Fatalf("singbox-vless-reality not listed: %#v", resp.Data)
	}
}

func TestListDeploymentConnectionCandidatesDiscoversManagedRoutes(t *testing.T) {
	jsonStore, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}
	if err := jsonStore.AddManualNode(storage.ManualNode{
		ID: "manual-1",
		Node: storage.Node{
			Tag:        "hk-1",
			Type:       "socks",
			Server:     "127.0.0.1",
			ServerPort: 1080,
		},
		Enabled: true,
	}); err != nil {
		t.Fatalf("AddManualNode() error = %v", err)
	}
	if err := jsonStore.AddProxyChain(storage.ProxyChain{
		ID:      "chain-1",
		Name:    "香港链路",
		Nodes:   []string{"hk-1"},
		Enabled: true,
	}); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}
	if err := jsonStore.AddInboundPort(storage.InboundPort{
		ID:       "port-1",
		Name:     "香港隧道",
		Type:     "mixed",
		Listen:   "127.0.0.1",
		Port:     2081,
		Outbound: "香港链路",
		Enabled:  true,
	}); err != nil {
		t.Fatalf("AddInboundPort() error = %v", err)
	}

	server := &Server{router: gin.New(), store: jsonStore}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/deployments/connection-candidates", nil)
	server.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET connection-candidates status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var resp struct {
		Data []deploy.ConnectionCandidate `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode candidates response: %v", err)
	}
	byID := make(map[string]deploy.ConnectionCandidate, len(resp.Data))
	for _, candidate := range resp.Data {
		byID[candidate.ID] = candidate
	}

	if candidate := byID["node:hk-1"]; candidate.Kind != deploy.CandidateKindNode || !candidate.RequiresTemporaryEntrypoint || !candidate.Available || candidate.UnavailableReason != "" {
		t.Fatalf("node candidate = %#v", candidate)
	}
	if candidate := byID["chain:chain-1"]; candidate.Kind != deploy.CandidateKindProxyChain || !candidate.Available || candidate.Outbound != "香港链路" {
		t.Fatalf("chain candidate = %#v", candidate)
	}
	if candidate := byID["inbound:port-1"]; candidate.Kind != deploy.CandidateKindInboundTunnel || candidate.Available || candidate.UnavailableReason != "sing-box 服务未运行" {
		t.Fatalf("inbound tunnel should require running service: %#v", candidate)
	}
	if _, exists := byID["global:mixed"]; exists {
		t.Fatalf("legacy settings mixed port should not be listed as a deployment tunnel: %#v", byID["global:mixed"])
	}
}

func TestListDeploymentConnectionCandidatesWithoutStoreReturnsEmptyArray(t *testing.T) {
	server := &Server{router: gin.New()}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/deployments/connection-candidates", nil)
	server.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET connection-candidates status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var resp struct {
		Data []deploy.ConnectionCandidate `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode candidates response: %v", err)
	}
	if resp.Data == nil {
		t.Fatalf("connection candidates should be an empty array, got nil: %s", recorder.Body.String())
	}
	if len(resp.Data) != 0 {
		t.Fatalf("connection candidates = %#v, want empty", resp.Data)
	}
}

func TestDeploymentConnectionTestValidatesSSHAuth(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		message    string
		realTester bool
	}{
		{
			name:    "missing password",
			body:    `{"ssh":{"host":"203.0.113.10","user":"root","auth_method":"password"},"connection":{"mode":"direct"}}`,
			message: "ssh.password 为必填",
		},
		{
			name:    "missing private key",
			body:    `{"ssh":{"host":"203.0.113.10","user":"root","auth_method":"private_key"},"connection":{"mode":"direct"}}`,
			message: "ssh.private_key 为必填",
		},
		{
			name:    "unsupported auth",
			body:    `{"ssh":{"host":"203.0.113.10","user":"root","auth_method":"agent"},"connection":{"mode":"direct"}}`,
			message: "不支持的 SSH 认证方式",
		},
		{
			name:    "password mode rejects private key",
			body:    `{"ssh":{"host":"203.0.113.10","user":"root","auth_method":"password","password":"secret","private_key":"also-secret"},"connection":{"mode":"direct"}}`,
			message: "ssh.password 与 ssh.private_key 不能同时提供",
		},
		{
			name:    "private key mode rejects password",
			body:    `{"ssh":{"host":"203.0.113.10","user":"root","auth_method":"private_key","private_key":"not-a-private-key","password":"also-secret"},"connection":{"mode":"direct"}}`,
			message: "ssh.password 与 ssh.private_key 不能同时提供",
		},
		{
			name:       "malformed private key",
			body:       `{"ssh":{"host":"203.0.113.10","user":"root","auth_method":"private_key","private_key":"not-a-private-key"},"connection":{"mode":"direct"}}`,
			message:    "SSH 私钥无法解析",
			realTester: true,
		},
		{
			name:    "custom proxy missing type",
			body:    `{"ssh":{"host":"203.0.113.10","user":"root","auth_method":"password","password":"secret"},"connection":{"mode":"custom_proxy"}}`,
			message: "proxy_type 仅支持 socks5 或 http_connect",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &Server{router: gin.New(), deploySSHTester: fakeDeploymentSSHTester{}}
			if tt.realTester {
				server.deploySSHTester = realDeploymentSSHTester{}
			}
			server.registerDeploymentRoutes(server.router.Group("/api"))

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/deployments/connection-test", bytes.NewReader([]byte(tt.body)))
			request.Header.Set("Content-Type", "application/json")
			server.router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			if !bytes.Contains(recorder.Body.Bytes(), []byte(tt.message)) {
				t.Fatalf("response %s does not contain %q", recorder.Body.String(), tt.message)
			}
		})
	}
}

func TestDeploymentConnectionTestAcceptsCustomProxy(t *testing.T) {
	var captured deploymentConnectionTestRequest
	server := &Server{
		router: gin.New(),
		deploySSHTester: fakeDeploymentSSHTester{
			captured: &captured,
		},
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"ssh":{"host":"203.0.113.10","user":"root","auth_method":"password","password":"ssh-secret"},
		"connection":{"mode":"custom_proxy","proxy_type":"socks5","proxy_host":"127.0.0.1","proxy_port":1080,"proxy_username":"ops","proxy_password":"proxy-secret"}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/connection-test", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("POST connection-test status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if captured.Connection.Mode != models.DeploymentConnectionCustomProxy || captured.Connection.ProxyType != "socks5" {
		t.Fatalf("connection was not normalized as custom proxy: %#v", captured.Connection)
	}
	if captured.Connection.ProxyHost != "127.0.0.1" || captured.Connection.ProxyPort != 1080 || captured.Connection.ProxyUsername != "ops" {
		t.Fatalf("proxy fields were not preserved: %#v", captured.Connection)
	}
	if bytes.Contains(recorder.Body.Bytes(), []byte("ssh-secret")) || bytes.Contains(recorder.Body.Bytes(), []byte("proxy-secret")) {
		t.Fatalf("connection test response leaked secret: %s", recorder.Body.String())
	}
}

func TestNormalizeDeploymentManagedProxyResolvesReusableTunnel(t *testing.T) {
	req := deploymentConnectionReq{
		Mode:               models.DeploymentConnectionManagedProxy,
		ManagedCandidateID: "inbound:port-1",
	}
	err := normalizeDeploymentConnectionRequest(&req, []deploy.ConnectionCandidate{
		{
			ID:            "inbound:port-1",
			Kind:          deploy.CandidateKindInboundTunnel,
			LocalEndpoint: "127.0.0.1:2081",
			Available:     true,
		},
	})
	if err != nil {
		t.Fatalf("normalize managed proxy error = %v", err)
	}
	if req.ProxyType != "socks5" || req.ProxyHost != "127.0.0.1" || req.ProxyPort != 2081 {
		t.Fatalf("managed proxy did not resolve to SOCKS5 local endpoint: %#v", req)
	}
}

func TestNormalizeDeploymentManagedProxyReusesNodeLocalEndpoint(t *testing.T) {
	req := deploymentConnectionReq{
		Mode:               models.DeploymentConnectionManagedProxy,
		ManagedCandidateID: "node:hk-1",
	}
	err := normalizeDeploymentConnectionRequest(&req, []deploy.ConnectionCandidate{
		{
			ID:            "node:hk-1",
			Kind:          deploy.CandidateKindNode,
			NodeTag:       "hk-1",
			Outbound:      "hk-1",
			LocalEndpoint: "127.0.0.1:2081",
			Available:     true,
		},
	})
	if err != nil {
		t.Fatalf("normalize managed node proxy error = %v", err)
	}
	if req.ProxyType != "socks5" || req.ProxyHost != "127.0.0.1" || req.ProxyPort != 2081 {
		t.Fatalf("managed node proxy did not reuse local endpoint: %#v", req)
	}
}

func TestNormalizeDeploymentManagedProxyRejectsUnavailableRoute(t *testing.T) {
	req := deploymentConnectionReq{
		Mode:               models.DeploymentConnectionManagedProxy,
		ManagedCandidateID: "inbound:port-1",
	}
	err := normalizeDeploymentConnectionRequest(&req, []deploy.ConnectionCandidate{
		{
			ID:                "inbound:port-1",
			Kind:              deploy.CandidateKindInboundTunnel,
			LocalEndpoint:     "127.0.0.1:19090",
			Available:         false,
			UnavailableReason: "sing-box 服务未运行",
		},
	})
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("sing-box 服务未运行")) {
		t.Fatalf("expected unavailable route error, got %v", err)
	}
}

func TestSocks5ConnectDialer(t *testing.T) {
	proxyAddress := startFakeSocks5Proxy(t)
	conn, err := socks5Connect(context.Background(), "tcp", proxyAddress, "example.com:22", "", "")
	if err != nil {
		t.Fatalf("socks5Connect() error = %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatalf("write through socks5 conn: %v", err)
	}
	reply := make([]byte, 4)
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatalf("read through socks5 conn: %v", err)
	}
	if string(reply) != "pong" {
		t.Fatalf("reply = %q, want pong", reply)
	}
}

func TestRealDeploymentSSHTesterUsesCustomSocks5Proxy(t *testing.T) {
	sshAddress := startFakeDeploymentSSHServer(t)
	proxyAddress, targets := startForwardingSocks5Proxy(t)

	sshHost, sshPortText, err := net.SplitHostPort(sshAddress)
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", sshAddress, err)
	}
	sshPort, err := strconv.Atoi(sshPortText)
	if err != nil {
		t.Fatalf("Atoi(%q): %v", sshPortText, err)
	}
	proxyHost, proxyPortText, err := net.SplitHostPort(proxyAddress)
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", proxyAddress, err)
	}
	proxyPort, err := strconv.Atoi(proxyPortText)
	if err != nil {
		t.Fatalf("Atoi(%q): %v", proxyPortText, err)
	}

	result, err := realDeploymentSSHTester{}.Test(context.Background(), deploymentConnectionTestRequest{
		SSH: deploymentSSHRequest{
			Host:       sshHost,
			Port:       sshPort,
			User:       "root",
			AuthMethod: "password",
			Password:   "ssh-secret",
		},
		Connection: deploymentConnectionReq{
			Mode:      models.DeploymentConnectionCustomProxy,
			ProxyType: "socks5",
			ProxyHost: proxyHost,
			ProxyPort: proxyPort,
		},
	})
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	if !result.OK || result.Remote["uname_s"] != "Linux" || result.Remote["systemd"] != "present" {
		t.Fatalf("unexpected SSH test result: %#v", result)
	}
	select {
	case target := <-targets:
		if target != sshAddress {
			t.Fatalf("SOCKS5 target = %q, want %q", target, sshAddress)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SOCKS5 proxy did not receive SSH target")
	}
}

func TestRealDeploymentSSHTesterUsesPrivateKeyAuth(t *testing.T) {
	privateKeyPEM, publicKey := generateDeploymentTestPrivateKey(t)
	sshAddress := startFakeDeploymentSSHServerWithPublicKey(t, publicKey)

	sshHost, sshPortText, err := net.SplitHostPort(sshAddress)
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", sshAddress, err)
	}
	sshPort, err := strconv.Atoi(sshPortText)
	if err != nil {
		t.Fatalf("Atoi(%q): %v", sshPortText, err)
	}

	result, err := realDeploymentSSHTester{}.Test(context.Background(), deploymentConnectionTestRequest{
		SSH: deploymentSSHRequest{
			Host:       sshHost,
			Port:       sshPort,
			User:       "root",
			AuthMethod: "private_key",
			PrivateKey: string(privateKeyPEM),
		},
		Connection: deploymentConnectionReq{
			Mode: models.DeploymentConnectionDirect,
		},
	})
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	if !result.OK || result.AuthMethod != "private_key" || result.Remote["uname_s"] != "Linux" || result.Remote["systemd"] != "present" {
		t.Fatalf("unexpected SSH test result: %#v", result)
	}
}

func TestRealDeploymentSSHTesterUsesPassphrasePrivateKeyAuth(t *testing.T) {
	privateKeyPEM, publicKey := generateDeploymentTestEncryptedPrivateKey(t, []byte("key-passphrase"))
	sshAddress := startFakeDeploymentSSHServerWithPublicKey(t, publicKey)

	sshHost, sshPortText, err := net.SplitHostPort(sshAddress)
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", sshAddress, err)
	}
	sshPort, err := strconv.Atoi(sshPortText)
	if err != nil {
		t.Fatalf("Atoi(%q): %v", sshPortText, err)
	}

	result, err := realDeploymentSSHTester{}.Test(context.Background(), deploymentConnectionTestRequest{
		SSH: deploymentSSHRequest{
			Host:       sshHost,
			Port:       sshPort,
			User:       "root",
			AuthMethod: "private_key",
			PrivateKey: string(privateKeyPEM),
			Passphrase: "key-passphrase",
		},
		Connection: deploymentConnectionReq{
			Mode: models.DeploymentConnectionDirect,
		},
	})
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	if !result.OK || result.AuthMethod != "private_key" || result.Remote["uname_s"] != "Linux" || result.Remote["systemd"] != "present" {
		t.Fatalf("unexpected SSH test result: %#v", result)
	}
}

func TestRealDeploymentSSHTesterUsesCustomHTTPConnectProxy(t *testing.T) {
	sshAddress := startFakeDeploymentSSHServer(t)
	proxyAddress, targets := startForwardingHTTPConnectProxy(t, "Basic b3BzOnNlY3JldA==")

	sshHost, sshPortText, err := net.SplitHostPort(sshAddress)
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", sshAddress, err)
	}
	sshPort, err := strconv.Atoi(sshPortText)
	if err != nil {
		t.Fatalf("Atoi(%q): %v", sshPortText, err)
	}
	proxyHost, proxyPortText, err := net.SplitHostPort(proxyAddress)
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", proxyAddress, err)
	}
	proxyPort, err := strconv.Atoi(proxyPortText)
	if err != nil {
		t.Fatalf("Atoi(%q): %v", proxyPortText, err)
	}

	result, err := realDeploymentSSHTester{}.Test(context.Background(), deploymentConnectionTestRequest{
		SSH: deploymentSSHRequest{
			Host:       sshHost,
			Port:       sshPort,
			User:       "root",
			AuthMethod: "password",
			Password:   "ssh-secret",
		},
		Connection: deploymentConnectionReq{
			Mode:          models.DeploymentConnectionCustomProxy,
			ProxyType:     "http_connect",
			ProxyHost:     proxyHost,
			ProxyPort:     proxyPort,
			ProxyUsername: "ops",
			ProxyPassword: "secret",
		},
	})
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	if !result.OK || result.Remote["uname_s"] != "Linux" || result.Remote["systemd"] != "present" {
		t.Fatalf("unexpected SSH test result: %#v", result)
	}
	select {
	case target := <-targets:
		if target != sshAddress {
			t.Fatalf("HTTP CONNECT target = %q, want %q", target, sshAddress)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("HTTP CONNECT proxy did not receive SSH target")
	}
}

func TestTCPDeploymentReachabilityCheckerUsesCustomProxy(t *testing.T) {
	proxyAddress := startFakeSocks5Proxy(t)
	proxyHost, proxyPortText, err := net.SplitHostPort(proxyAddress)
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", proxyAddress, err)
	}
	proxyPort, err := strconv.Atoi(proxyPortText)
	if err != nil {
		t.Fatalf("Atoi(%q): %v", proxyPortText, err)
	}

	checker := tcpDeploymentReachabilityChecker{}
	err = checker.Check(context.Background(), deploymentConnectionTestRequest{
		Connection: deploymentConnectionReq{
			Mode:      models.DeploymentConnectionCustomProxy,
			ProxyType: "socks5",
			ProxyHost: proxyHost,
			ProxyPort: proxyPort,
		},
	}, "vpn.example.com", 443)
	if err != nil {
		t.Fatalf("Check() through custom SOCKS5 proxy error = %v", err)
	}
}

func TestTCPDeploymentReachabilityCheckerUsesHTTPConnectProxy(t *testing.T) {
	targetAddress := startAcceptingTCPServer(t)
	targetHost, targetPortText, err := net.SplitHostPort(targetAddress)
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", targetAddress, err)
	}
	targetPort, err := strconv.Atoi(targetPortText)
	if err != nil {
		t.Fatalf("Atoi(%q): %v", targetPortText, err)
	}
	proxyAddress, targets := startForwardingHTTPConnectProxy(t, "")
	proxyHost, proxyPortText, err := net.SplitHostPort(proxyAddress)
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", proxyAddress, err)
	}
	proxyPort, err := strconv.Atoi(proxyPortText)
	if err != nil {
		t.Fatalf("Atoi(%q): %v", proxyPortText, err)
	}

	checker := tcpDeploymentReachabilityChecker{}
	err = checker.Check(context.Background(), deploymentConnectionTestRequest{
		Connection: deploymentConnectionReq{
			Mode:      models.DeploymentConnectionCustomProxy,
			ProxyType: "http_connect",
			ProxyHost: proxyHost,
			ProxyPort: proxyPort,
		},
	}, targetHost, targetPort)
	if err != nil {
		t.Fatalf("Check() through custom HTTP CONNECT proxy error = %v", err)
	}
	select {
	case target := <-targets:
		if target != targetAddress {
			t.Fatalf("HTTP CONNECT reachability target = %q, want %q", target, targetAddress)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("HTTP CONNECT proxy did not receive reachability target")
	}
}

func TestHTTPConnectDialer(t *testing.T) {
	proxyAddress := startFakeHTTPConnectProxy(t, "Basic b3BzOnNlY3JldA==")
	conn, err := httpConnect(context.Background(), "tcp", proxyAddress, "example.com:22", "ops", "secret")
	if err != nil {
		t.Fatalf("httpConnect() error = %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatalf("write through http connect conn: %v", err)
	}
	reply := make([]byte, 4)
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatalf("read through http connect conn: %v", err)
	}
	if string(reply) != "pong" {
		t.Fatalf("reply = %q, want pong", reply)
	}
}

func TestHTTPConnectDialerPreservesBufferedTunnelBytes(t *testing.T) {
	proxyAddress := startFakeHTTPConnectProxyWithEarlyData(t, []byte("SSHB"))
	conn, err := httpConnect(context.Background(), "tcp", proxyAddress, "example.com:22", "", "")
	if err != nil {
		t.Fatalf("httpConnect() error = %v", err)
	}
	defer conn.Close()

	reply := make([]byte, 4)
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatalf("read early tunnel bytes: %v", err)
	}
	if string(reply) != "SSHB" {
		t.Fatalf("early tunnel bytes = %q, want SSHB", reply)
	}
}

func TestDeploymentConnectionTestReturnsSanitizedResult(t *testing.T) {
	server := &Server{
		router: gin.New(),
		deploySSHTester: fakeDeploymentSSHTester{
			result: deploymentConnectionTestResult{
				OK:         true,
				Message:    "SSH 连接成功",
				Host:       "203.0.113.10",
				Port:       22,
				User:       "root",
				AuthMethod: "password",
				DurationMS: 12,
				Remote: map[string]string{
					"uname_s": "Linux",
					"systemd": "present",
				},
			},
		},
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{"ssh":{"host":"203.0.113.10","user":"","auth_method":"password","password":"ssh-secret"},"connection":{"mode":"direct"}}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/connection-test", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("POST connection-test status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if bytes.Contains(recorder.Body.Bytes(), []byte("ssh-secret")) {
		t.Fatalf("connection test response leaked password: %s", recorder.Body.String())
	}
	var resp struct {
		Data deploymentConnectionTestResult `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Data.OK || resp.Data.User != "root" || resp.Data.Remote["uname_s"] != "Linux" {
		t.Fatalf("unexpected test result: %#v", resp.Data)
	}
}

func TestDeploymentConnectionTestStartsTemporaryManagedEntrypoint(t *testing.T) {
	jsonStore, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}
	if err := jsonStore.AddManualNode(storage.ManualNode{
		ID: "manual-1",
		Node: storage.Node{
			Tag:        "hk-1",
			Type:       "socks",
			Server:     "127.0.0.1",
			ServerPort: 1080,
		},
		Enabled: true,
	}); err != nil {
		t.Fatalf("AddManualNode() error = %v", err)
	}

	var captured deploymentConnectionTestRequest
	var cleaned atomic.Bool
	starter := &fakeDeploymentEntrypointStarter{
		endpoint: "127.0.0.1:34567",
		cleaned:  &cleaned,
	}
	server := &Server{
		router:                  gin.New(),
		store:                   jsonStore,
		deploySSHTester:         fakeDeploymentSSHTester{captured: &captured},
		deployEntrypointStarter: starter,
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"ssh":{"host":"203.0.113.10","user":"root","auth_method":"password","password":"ssh-secret"},
		"connection":{"mode":"managed_proxy","managed_candidate_id":"node:hk-1"}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/connection-test", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("POST connection-test status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !starter.started {
		t.Fatal("temporary managed entrypoint was not started")
	}
	if !cleaned.Load() {
		t.Fatal("temporary managed entrypoint was not cleaned up")
	}
	if starter.input.Candidate.ID != "node:hk-1" || starter.input.Candidate.Outbound != "hk-1" {
		t.Fatalf("unexpected entrypoint candidate: %#v", starter.input.Candidate)
	}
	if captured.Connection.Mode != models.DeploymentConnectionManagedProxy ||
		captured.Connection.ProxyType != "socks5" ||
		captured.Connection.ProxyHost != "127.0.0.1" ||
		captured.Connection.ProxyPort != 34567 {
		t.Fatalf("SSH tester did not receive managed SOCKS5 endpoint: %#v", captured.Connection)
	}
}

func TestDeploymentConnectionTestCleansTemporaryManagedEntrypointOnSSHFailure(t *testing.T) {
	jsonStore, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}
	if err := jsonStore.AddManualNode(storage.ManualNode{
		ID: "manual-1",
		Node: storage.Node{
			Tag:        "hk-1",
			Type:       "socks",
			Server:     "127.0.0.1",
			ServerPort: 1080,
		},
		Enabled: true,
	}); err != nil {
		t.Fatalf("AddManualNode() error = %v", err)
	}

	var cleaned atomic.Bool
	server := &Server{
		router:                  gin.New(),
		store:                   jsonStore,
		deploySSHTester:         fakeDeploymentSSHTester{err: fmt.Errorf("ssh handshake failed")},
		deployEntrypointStarter: &fakeDeploymentEntrypointStarter{endpoint: "127.0.0.1:34567", cleaned: &cleaned},
	}
	server.registerDeploymentRoutes(server.router.Group("/api"))

	body := []byte(`{
		"ssh":{"host":"203.0.113.10","user":"root","auth_method":"password","password":"ssh-secret"},
		"connection":{"mode":"managed_proxy","managed_candidate_id":"node:hk-1"}
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/deployments/connection-test", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("POST connection-test status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("ssh handshake failed")) {
		t.Fatalf("response does not include SSH failure: %s", recorder.Body.String())
	}
	if !cleaned.Load() {
		t.Fatal("temporary managed entrypoint was not cleaned up after SSH failure")
	}
}

func TestTemporaryDeploymentEntrypointConfigRoutesToCandidate(t *testing.T) {
	configJSON, err := temporaryDeploymentEntrypointConfig(deploymentManagedEntrypointInput{
		Candidate: deploy.ConnectionCandidate{
			ID:       "node:hk-1",
			Kind:     deploy.CandidateKindNode,
			Outbound: "hk-1",
		},
		Settings: &storage.Settings{
			ProxyDNS:             "https://1.1.1.1/dns-query",
			DirectDNS:            "https://dns.alidns.com/dns-query",
			FinalOutbound:        "Proxy",
			ClashAPIPort:         19091,
			ClashUIEnabled:       true,
			ClashAPILanEnabled:   true,
			ClashAPISecret:       "secret",
			RuleSetBaseURL:       "https://example.com/rules",
			ChainHealthConfig:    &storage.ChainHealthConfig{},
			TorExtraArgs:         []string{},
			TorrcValues:          map[string]string{},
			SubscriptionInterval: 60,
		},
		Nodes: []storage.Node{
			{Tag: "hk-1", Type: "socks", Server: "127.0.0.1", ServerPort: 1080},
		},
		DataDir: t.TempDir(),
	}, t.TempDir(), 34567)
	if err != nil {
		t.Fatalf("temporaryDeploymentEntrypointConfig() error = %v", err)
	}

	var config struct {
		Inbounds []map[string]interface{} `json:"inbounds"`
		Route    struct {
			Rules []map[string]interface{} `json:"rules"`
			Final string                   `json:"final"`
		} `json:"route"`
		Experimental struct {
			ClashAPI interface{} `json:"clash_api"`
		} `json:"experimental"`
	}
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		t.Fatalf("decode temporary config: %v\n%s", err, configJSON)
	}
	if len(config.Inbounds) != 1 || config.Inbounds[0]["type"] != "socks" || config.Inbounds[0]["listen_port"] != float64(34567) {
		t.Fatalf("temporary config inbound = %#v", config.Inbounds)
	}
	foundRoute := false
	for _, rule := range config.Route.Rules {
		if rule["outbound"] == "hk-1" {
			foundRoute = true
		}
	}
	if !foundRoute || config.Route.Final != "REJECT" {
		t.Fatalf("temporary config route did not isolate selected outbound: %#v", config.Route)
	}
	if config.Experimental.ClashAPI != nil {
		t.Fatalf("temporary config should disable Clash API: %#v", config.Experimental.ClashAPI)
	}
}

func TestTemporaryDeploymentEntrypointConfigPassesSingBoxCheckWhenAvailable(t *testing.T) {
	singBoxPath := os.Getenv("SBM_SING_BOX_CHECK_BIN")
	if singBoxPath == "" {
		t.Skip("set SBM_SING_BOX_CHECK_BIN to run sing-box config validation")
	}
	configJSON, err := temporaryDeploymentEntrypointConfig(deploymentManagedEntrypointInput{
		Candidate: deploy.ConnectionCandidate{
			ID:       "node:hk-1",
			Kind:     deploy.CandidateKindNode,
			Outbound: "hk-1",
		},
		Settings: storage.DefaultSettings(),
		Nodes: []storage.Node{
			{Tag: "hk-1", Type: "socks", Server: "127.0.0.1", ServerPort: 1080},
		},
		DataDir: t.TempDir(),
	}, t.TempDir(), 34567)
	if err != nil {
		t.Fatalf("temporaryDeploymentEntrypointConfig() error = %v", err)
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte(configJSON), 0o600); err != nil {
		t.Fatalf("write temporary config: %v", err)
	}
	output, err := exec.Command(singBoxPath, "check", "-c", configPath).CombinedOutput()
	if err != nil {
		t.Fatalf("sing-box check failed: %v\n%s\nconfig:\n%s", err, output, configJSON)
	}
}

func TestPrepareDeploymentScriptEnvUploadsCachedArchive(t *testing.T) {
	uploader := &fakeDeploymentArchiveUploader{remotePath: "/tmp/sbm-runtime.tar.gz"}
	env := map[string]string{
		"SBM_NODE_NAME":       "edge-a",
		"SBM_SINGBOX_ARCHIVE": "/data/runtime-cache/sing-box/v/linux-amd64.tar.gz",
	}

	prepared, cleanup, err := prepareDeploymentScriptEnv(context.Background(), env, uploader)
	if err != nil {
		t.Fatalf("prepareDeploymentScriptEnv() error = %v", err)
	}
	if uploader.localPath != "/data/runtime-cache/sing-box/v/linux-amd64.tar.gz" {
		t.Fatalf("uploaded local path = %q", uploader.localPath)
	}
	if prepared["SBM_SINGBOX_ARCHIVE"] != "/tmp/sbm-runtime.tar.gz" {
		t.Fatalf("archive env was not replaced with remote path: %#v", prepared)
	}
	if env["SBM_SINGBOX_ARCHIVE"] != "/data/runtime-cache/sing-box/v/linux-amd64.tar.gz" {
		t.Fatalf("original env was mutated: %#v", env)
	}

	cleanup(context.Background())
	if !uploader.cleaned {
		t.Fatal("remote archive cleanup was not called")
	}
}

func TestUploadedDeploymentScriptCommandRunsRemoteScriptFromRunDir(t *testing.T) {
	command := uploadedDeploymentScriptCommand("/tmp/sbm-deploy-abc123", "/tmp/sbm-deploy-abc123/template.sh", map[string]string{
		"SBM_NODE_NAME":      "edge-a",
		"SBM_REMOTE_RUN_DIR": "/tmp/sbm-deploy-abc123",
	})
	if !strings.HasPrefix(command, "cd '/tmp/sbm-deploy-abc123' && env ") {
		t.Fatalf("command does not enter remote run dir: %s", command)
	}
	if !strings.Contains(command, "SBM_REMOTE_RUN_DIR='/tmp/sbm-deploy-abc123'") {
		t.Fatalf("command does not expose remote run dir: %s", command)
	}
	if !strings.Contains(command, "bash '/tmp/sbm-deploy-abc123/template.sh'") {
		t.Fatalf("command does not run uploaded script file: %s", command)
	}
	if strings.Contains(command, "bash -s") {
		t.Fatalf("uploaded command should not use stdin script execution: %s", command)
	}
}

func TestRemoteRunDirProgressMarkerParsesIntoHistoryMarkers(t *testing.T) {
	marker := remoteRunDirProgressMarker("/tmp/sbm-deploy-abc123", true)
	parsed, err := deploy.ParseScriptOutput(marker + `SBM_RESULT {"status":"success"}` + "\n")
	if err != nil {
		t.Fatalf("ParseScriptOutput() error = %v", err)
	}
	if len(parsed.Progress) != 1 {
		t.Fatalf("progress markers = %#v", parsed.Progress)
	}
	got := parsed.Progress[0]
	if got["step"] != "remote_run_dir" || got["status"] != "preserved" || got["path"] != "/tmp/sbm-deploy-abc123" || got["preserved"] != true {
		t.Fatalf("remote run dir marker = %#v", got)
	}
	if marker := remoteRunDirProgressMarker("/tmp/sbm-deploy-abc123", false); marker != "" {
		t.Fatalf("default cleanup should not emit marker, got %q", marker)
	}
}

type fakeDeploymentSSHTester struct {
	result   deploymentConnectionTestResult
	err      error
	captured *deploymentConnectionTestRequest
}

func (f fakeDeploymentSSHTester) Test(ctx context.Context, req deploymentConnectionTestRequest) (deploymentConnectionTestResult, error) {
	_ = ctx
	if f.captured != nil {
		*f.captured = req
	}
	if f.err != nil {
		return deploymentConnectionTestResult{}, f.err
	}
	if f.result.Host == "" {
		return deploymentConnectionTestResult{
			OK:         true,
			Message:    "SSH 连接成功",
			Host:       req.SSH.Host,
			Port:       req.SSH.Port,
			User:       req.SSH.User,
			AuthMethod: req.SSH.AuthMethod,
			DurationMS: 1,
		}, nil
	}
	return f.result, nil
}

type fakeDeploymentEntrypointStarter struct {
	endpoint string
	input    deploymentManagedEntrypointInput
	started  bool
	cleaned  *atomic.Bool
	err      error
}

func (f *fakeDeploymentEntrypointStarter) Start(ctx context.Context, input deploymentManagedEntrypointInput) (deploymentManagedEntrypoint, error) {
	_ = ctx
	f.started = true
	f.input = input
	if f.err != nil {
		return deploymentManagedEntrypoint{}, f.err
	}
	return deploymentManagedEntrypoint{
		LocalEndpoint: f.endpoint,
		Cleanup: func() {
			if f.cleaned != nil {
				f.cleaned.Store(true)
			}
		},
	}, nil
}

type fakeDeploymentScriptExecutor struct {
	output            string
	outputs           []string
	err               error
	errors            []error
	calls             int
	req               deploymentConnectionTestRequest
	reqs              []deploymentConnectionTestRequest
	script            []byte
	scripts           [][]byte
	executionMode     string
	executionModes    []string
	env               map[string]string
	envs              []map[string]string
	waitForCancel     bool
	waitForCancelCall int
	started           chan struct{}
	startedOnce       sync.Once
}

type fakeDeploymentReachabilityChecker struct {
	err   error
	calls int
	host  string
	port  int
	req   deploymentConnectionTestRequest
}

type fakeDeploymentArchiveUploader struct {
	remotePath string
	localPath  string
	cleaned    bool
	err        error
}

func (f *fakeDeploymentArchiveUploader) Upload(ctx context.Context, localPath string) (string, func(context.Context), error) {
	_ = ctx
	f.localPath = localPath
	if f.err != nil {
		return "", nil, f.err
	}
	return f.remotePath, func(context.Context) {
		f.cleaned = true
	}, nil
}

func (f *fakeDeploymentReachabilityChecker) Check(ctx context.Context, req deploymentConnectionTestRequest, host string, port int) error {
	_ = ctx
	f.calls++
	f.req = req
	f.host = host
	f.port = port
	return f.err
}

func (f *fakeDeploymentScriptExecutor) Execute(ctx context.Context, req deploymentConnectionTestRequest, script []byte, env map[string]string, executionMode string) (string, error) {
	_ = ctx
	f.calls++
	f.req = req
	f.reqs = append(f.reqs, req)
	f.script = append([]byte(nil), script...)
	f.scripts = append(f.scripts, append([]byte(nil), script...))
	f.executionMode = executionMode
	f.executionModes = append(f.executionModes, executionMode)
	f.env = map[string]string{}
	for key, value := range env {
		f.env[key] = value
	}
	f.envs = append(f.envs, f.env)
	if f.started != nil {
		f.startedOnce.Do(func() { close(f.started) })
	}
	if f.waitForCancel && (f.waitForCancelCall == 0 || f.waitForCancelCall == f.calls) {
		<-ctx.Done()
		return f.output, ctx.Err()
	}
	output := f.output
	if len(f.outputs) >= f.calls {
		output = f.outputs[f.calls-1]
	}
	if len(f.errors) >= f.calls && f.errors[f.calls-1] != nil {
		return output, f.errors[f.calls-1]
	}
	if f.err != nil {
		return output, f.err
	}
	return output, nil
}

func startFakeSocks5Proxy(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake socks5 proxy: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		header := make([]byte, 2)
		if _, err := io.ReadFull(conn, header); err != nil {
			return
		}
		methods := make([]byte, int(header[1]))
		if _, err := io.ReadFull(conn, methods); err != nil {
			return
		}
		if _, err := conn.Write([]byte{0x05, 0x00}); err != nil {
			return
		}

		connectHeader := make([]byte, 4)
		if _, err := io.ReadFull(conn, connectHeader); err != nil {
			return
		}
		switch connectHeader[3] {
		case 0x01:
			_, _ = io.CopyN(io.Discard, conn, 4+2)
		case 0x03:
			length := []byte{0}
			if _, err := io.ReadFull(conn, length); err != nil {
				return
			}
			_, _ = io.CopyN(io.Discard, conn, int64(length[0])+2)
		case 0x04:
			_, _ = io.CopyN(io.Discard, conn, 16+2)
		default:
			return
		}
		if _, err := conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 127, 0, 0, 1, 0, 0}); err != nil {
			return
		}
		payload := make([]byte, 4)
		if _, err := io.ReadFull(conn, payload); err != nil {
			return
		}
		if string(payload) == "ping" {
			_, _ = conn.Write([]byte("pong"))
		}
	}()
	return listener.Addr().String()
}

func startForwardingSocks5Proxy(t *testing.T) (string, <-chan string) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen forwarding socks5 proxy: %v", err)
	}
	targets := make(chan string, 1)
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		header := make([]byte, 2)
		if _, err := io.ReadFull(conn, header); err != nil {
			return
		}
		methods := make([]byte, int(header[1]))
		if _, err := io.ReadFull(conn, methods); err != nil {
			return
		}
		if _, err := conn.Write([]byte{0x05, 0x00}); err != nil {
			return
		}

		target, err := readSocks5Target(conn)
		if err != nil {
			return
		}
		targets <- target
		upstream, err := net.Dial("tcp", target)
		if err != nil {
			_, _ = conn.Write([]byte{0x05, 0x05, 0x00, 0x01, 127, 0, 0, 1, 0, 0})
			return
		}
		defer upstream.Close()
		if _, err := conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 127, 0, 0, 1, 0, 0}); err != nil {
			return
		}
		done := make(chan struct{}, 2)
		go func() {
			_, _ = io.Copy(upstream, conn)
			done <- struct{}{}
		}()
		go func() {
			_, _ = io.Copy(conn, upstream)
			done <- struct{}{}
		}()
		<-done
	}()
	return listener.Addr().String(), targets
}

func readSocks5Target(conn net.Conn) (string, error) {
	connectHeader := make([]byte, 4)
	if _, err := io.ReadFull(conn, connectHeader); err != nil {
		return "", err
	}
	var host string
	switch connectHeader[3] {
	case 0x01:
		ip := make([]byte, 4)
		if _, err := io.ReadFull(conn, ip); err != nil {
			return "", err
		}
		host = net.IP(ip).String()
	case 0x03:
		length := []byte{0}
		if _, err := io.ReadFull(conn, length); err != nil {
			return "", err
		}
		name := make([]byte, int(length[0]))
		if _, err := io.ReadFull(conn, name); err != nil {
			return "", err
		}
		host = string(name)
	case 0x04:
		ip := make([]byte, 16)
		if _, err := io.ReadFull(conn, ip); err != nil {
			return "", err
		}
		host = net.IP(ip).String()
	default:
		return "", fmt.Errorf("unsupported socks5 address type")
	}
	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBytes); err != nil {
		return "", err
	}
	port := int(binary.BigEndian.Uint16(portBytes))
	return net.JoinHostPort(host, strconv.Itoa(port)), nil
}

func startFakeDeploymentSSHServer(t *testing.T) string {
	return startFakeDeploymentSSHServerWithPublicKey(t, nil)
}

func startFakeDeploymentSSHServerWithPublicKey(t *testing.T, allowedPublicKey ssh.PublicKey) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate SSH host key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatalf("create SSH signer: %v", err)
	}
	config := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if conn.User() == "root" && string(password) == "ssh-secret" {
				return nil, nil
			}
			return nil, fmt.Errorf("permission denied")
		},
	}
	if allowedPublicKey != nil {
		config.PublicKeyCallback = func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if conn.User() == "root" && bytes.Equal(key.Marshal(), allowedPublicKey.Marshal()) {
				return nil, nil
			}
			return nil, fmt.Errorf("permission denied")
		}
	}
	config.AddHostKey(signer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake SSH server: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, channels, requests, err := ssh.NewServerConn(conn, config)
		if err != nil {
			return
		}
		go ssh.DiscardRequests(requests)
		for channel := range channels {
			if channel.ChannelType() != "session" {
				_ = channel.Reject(ssh.UnknownChannelType, "unsupported channel type")
				continue
			}
			session, requests, err := channel.Accept()
			if err != nil {
				return
			}
			go handleFakeDeploymentSSHSession(session, requests)
		}
	}()
	return listener.Addr().String()
}

func generateDeploymentTestPrivateKey(t *testing.T) ([]byte, ssh.PublicKey) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate client private key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatalf("create client signer: %v", err)
	}
	privateKey := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	return privateKey, signer.PublicKey()
}

func generateDeploymentTestEncryptedPrivateKey(t *testing.T, passphrase []byte) ([]byte, ssh.PublicKey) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate encrypted client private key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatalf("create encrypted client signer: %v", err)
	}
	block, err := ssh.MarshalPrivateKeyWithPassphrase(key, "deployment-test", passphrase)
	if err != nil {
		t.Fatalf("marshal encrypted private key: %v", err)
	}
	return pem.EncodeToMemory(block), signer.PublicKey()
}

func handleFakeDeploymentSSHSession(channel ssh.Channel, requests <-chan *ssh.Request) {
	defer channel.Close()
	for request := range requests {
		switch request.Type {
		case "exec":
			_ = request.Reply(true, nil)
			_, _ = channel.Write([]byte("uname_s=Linux\nuname_m=x86_64\nid_u=0\nsystemd=present\n"))
			_, _ = channel.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
			return
		default:
			_ = request.Reply(false, nil)
		}
	}
}

func startFakeHTTPConnectProxy(t *testing.T, expectedAuth string) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake http connect proxy: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		request, err := http.ReadRequest(bufio.NewReader(conn))
		if err != nil {
			return
		}
		if request.Method != http.MethodConnect || request.Host != "example.com:22" {
			_, _ = conn.Write([]byte("HTTP/1.1 400 Bad Request\r\nContent-Length: 0\r\n\r\n"))
			return
		}
		if expectedAuth != "" && request.Header.Get("Proxy-Authorization") != expectedAuth {
			_, _ = conn.Write([]byte("HTTP/1.1 407 Proxy Authentication Required\r\nContent-Length: 0\r\n\r\n"))
			return
		}
		if _, err := conn.Write([]byte("HTTP/1.1 200 Connection Established\r\nContent-Length: 0\r\n\r\n")); err != nil {
			return
		}
		payload := make([]byte, 4)
		if _, err := io.ReadFull(conn, payload); err != nil {
			return
		}
		if string(payload) == "ping" {
			_, _ = conn.Write([]byte("pong"))
		}
	}()
	return listener.Addr().String()
}

func startForwardingHTTPConnectProxy(t *testing.T, expectedAuth string) (string, <-chan string) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen forwarding http connect proxy: %v", err)
	}
	targets := make(chan string, 1)
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		request, err := http.ReadRequest(bufio.NewReader(conn))
		if err != nil {
			return
		}
		if request.Method != http.MethodConnect {
			_, _ = conn.Write([]byte("HTTP/1.1 405 Method Not Allowed\r\nContent-Length: 0\r\n\r\n"))
			return
		}
		if expectedAuth != "" && request.Header.Get("Proxy-Authorization") != expectedAuth {
			_, _ = conn.Write([]byte("HTTP/1.1 407 Proxy Authentication Required\r\nContent-Length: 0\r\n\r\n"))
			return
		}
		targets <- request.Host
		upstream, err := net.Dial("tcp", request.Host)
		if err != nil {
			_, _ = conn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n"))
			return
		}
		defer upstream.Close()
		if _, err := conn.Write([]byte("HTTP/1.1 200 Connection Established\r\nContent-Length: 0\r\n\r\n")); err != nil {
			return
		}
		done := make(chan struct{}, 2)
		go func() {
			_, _ = io.Copy(upstream, conn)
			done <- struct{}{}
		}()
		go func() {
			_, _ = io.Copy(conn, upstream)
			done <- struct{}{}
		}()
		<-done
	}()
	return listener.Addr().String(), targets
}

func startAcceptingTCPServer(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen accepting tcp server: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		_ = conn.Close()
	}()
	return listener.Addr().String()
}

func startFakeHTTPConnectProxyWithEarlyData(t *testing.T, data []byte) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake http connect proxy: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		request, err := http.ReadRequest(bufio.NewReader(conn))
		if err != nil || request.Method != http.MethodConnect {
			return
		}
		response := append([]byte("HTTP/1.1 200 Connection Established\r\nContent-Length: 0\r\n\r\n"), data...)
		_, _ = conn.Write(response)
	}()
	return listener.Addr().String()
}

func newDeploymentTestStore(t *testing.T) *database.Store {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "sbm-test.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Task{}, &models.DeploymentRun{}, &models.Node{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return database.NewStore(db)
}

func assertNoSecretValue(t *testing.T, params models.JSONMap, secrets ...string) {
	t.Helper()

	encoded, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	assertNoSecretBytes(t, "params", encoded, secrets...)
}

func assertNoSecretBytes(t *testing.T, label string, data []byte, secrets ...string) {
	t.Helper()

	for _, secret := range secrets {
		if bytes.Contains(data, []byte(secret)) {
			t.Fatalf("secret %q leaked in %s: %s", secret, label, data)
		}
	}
}

func deploymentRunIDs(runs []models.DeploymentRun) []string {
	ids := make([]string, 0, len(runs))
	for _, run := range runs {
		ids = append(ids, run.ID)
	}
	return ids
}

func waitForDeploymentRunStatus(t *testing.T, store *database.Store, id, status string) *models.DeploymentRun {
	t.Helper()
	return waitForDeploymentRunCondition(t, store, id, func(run *models.DeploymentRun) bool {
		return run.Status == status
	})
}

func waitForTaskStatus(t *testing.T, store *database.Store, id, status string) *models.Task {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	var last *models.Task
	for time.Now().Before(deadline) {
		task, err := store.GetTask(id)
		if err == nil {
			last = task
			if task.Status == status {
				return task
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if last == nil {
		t.Fatalf("task %s was not found", id)
	}
	t.Fatalf("task %s did not reach status %s, last=%#v", id, status, last)
	return nil
}

func waitForDeploymentRunCondition(t *testing.T, store *database.Store, id string, done func(*models.DeploymentRun) bool) *models.DeploymentRun {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	var last *models.DeploymentRun
	for time.Now().Before(deadline) {
		run, err := store.GetDeploymentRun(id)
		if err == nil {
			last = run
			if done(run) {
				return run
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if last == nil {
		t.Fatalf("deployment run %s was not found", id)
	}
	t.Fatalf("deployment run %s did not reach expected condition, last=%#v", id, last)
	return nil
}

func groupedNodesContain(groups []storage.NodeGroup, source, sourceName, tag string) bool {
	for _, group := range groups {
		if group.Source != source || group.SourceName != sourceName {
			continue
		}
		for _, node := range group.Nodes {
			if node.Tag == tag {
				return true
			}
		}
	}
	return false
}
