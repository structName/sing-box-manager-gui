package api

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/xiaobei/singbox-manager/internal/builder"
	"github.com/xiaobei/singbox-manager/internal/daemon"
	"github.com/xiaobei/singbox-manager/internal/database"
	"github.com/xiaobei/singbox-manager/internal/database/models"
	"github.com/xiaobei/singbox-manager/internal/kernel"
	"github.com/xiaobei/singbox-manager/internal/logger"
	"github.com/xiaobei/singbox-manager/internal/parser"
	"github.com/xiaobei/singbox-manager/internal/profile"
	"github.com/xiaobei/singbox-manager/internal/service"
	"github.com/xiaobei/singbox-manager/internal/speedtest"
	"github.com/xiaobei/singbox-manager/internal/storage"
	"github.com/xiaobei/singbox-manager/web"
)

// Server API 服务器
type Server struct {
	profileMgr     *profile.Manager
	store          *storage.JSONStore
	subService     *service.SubscriptionService
	processManager *daemon.ProcessManager
	launchdManager *daemon.LaunchdManager
	systemdManager *daemon.SystemdManager
	kernelManager  *kernel.Manager
	scheduler      *service.Scheduler
	chainSyncSvc   *service.ChainSyncService
	healthCheckSvc *service.HealthCheckService
	router         *gin.Engine
	sbmPath        string // sbm 可执行文件路径
	port           int    // Web 服务端口
	version        string // sbm 版本号
	swaggerEnabled bool   // 是否启用 Swagger
	baseDir        string // 基础数据目录
	// SQLite 存储和测速模块
	dbStore            *database.Store
	speedTestExecutor  *speedtest.Executor
	speedTestScheduler *speedtest.Scheduler
	speedTestHandler   *SpeedTestHandler
	// 标签模块
	tagEngine  *service.TagEngine
	tagHandler *TagHandler
}

// NewServer 创建 API 服务器
func NewServer(profileMgr *profile.Manager, processManager *daemon.ProcessManager, launchdManager *daemon.LaunchdManager, systemdManager *daemon.SystemdManager, sbmPath string, port int, version string, swaggerEnabled bool) *Server {
	gin.SetMode(gin.ReleaseMode)

	// 获取当前 Profile 目录，创建 JSONStore（用于兼容旧代码）
	profileDir := profileMgr.GetProfileDir()
	store, err := storage.NewJSONStore(profileDir)
	if err != nil {
		logger.Error("初始化 JSONStore 失败: %v", err)
	}

	// 创建链路同步服务
	chainSyncSvc := service.NewChainSyncService(store)

	// 创建订阅服务，并注入链路同步服务
	subService := service.NewSubscriptionService(store, chainSyncSvc)

	// 创建内核管理器（内核放在 baseDir，所有 Profile 共享）
	baseDir := filepath.Dir(profileDir)
	if filepath.Base(baseDir) == "profiles" {
		baseDir = filepath.Dir(baseDir)
	}
	kernelManager := kernel.NewManager(baseDir, store.GetSettings)

	// 创建健康检测服务
	healthCheckSvc := service.NewHealthCheckService(store)

	// 从 Profile 管理器获取数据库连接
	dbStore := profileMgr.GetStore()
	var speedTestExecutor *speedtest.Executor
	var speedTestScheduler *speedtest.Scheduler
	var speedTestHandler *SpeedTestHandler
	var tagEngine *service.TagEngine
	var tagHandler *TagHandler

	if dbStore != nil {
		speedTestExecutor = speedtest.NewExecutor(dbStore)
		speedTestScheduler = speedtest.NewScheduler(dbStore, speedTestExecutor)
		speedTestHandler = NewSpeedTestHandler(dbStore, speedTestExecutor, speedTestScheduler)
		tagEngine = service.NewTagEngine(dbStore)
		tagHandler = NewTagHandler(dbStore, tagEngine)
		logger.Info("测速和标签模块初始化完成")
	}

	s := &Server{
		profileMgr:         profileMgr,
		store:              store,
		subService:         subService,
		processManager:     processManager,
		launchdManager:     launchdManager,
		systemdManager:     systemdManager,
		kernelManager:      kernelManager,
		scheduler:          service.NewScheduler(store, subService),
		chainSyncSvc:       chainSyncSvc,
		healthCheckSvc:     healthCheckSvc,
		router:             gin.Default(),
		sbmPath:            sbmPath,
		port:               port,
		version:            version,
		swaggerEnabled:     swaggerEnabled,
		baseDir:            baseDir,
		dbStore:            dbStore,
		speedTestExecutor:  speedTestExecutor,
		speedTestScheduler: speedTestScheduler,
		speedTestHandler:   speedTestHandler,
		tagEngine:          tagEngine,
		tagHandler:         tagHandler,
	}

	// 设置调度器的更新回调
	s.scheduler.SetUpdateCallback(s.autoApplyConfig)

	// 初始同步节点到 SQLite（用于测速模块）
	if s.dbStore != nil {
		if err := s.syncNodesToSQLite(); err != nil {
			logger.Warn("启动时同步节点到 SQLite 失败: %v", err)
		} else {
			logger.Info("节点已同步到 SQLite 数据库")
		}
	}

	s.setupRoutes()
	return s
}

// StartScheduler 启动定时任务调度器
func (s *Server) StartScheduler() {
	s.scheduler.Start()
}

// StopScheduler 停止定时任务调度器
func (s *Server) StopScheduler() {
	s.scheduler.Stop()
}

// StartSpeedTestScheduler 启动测速定时调度器
func (s *Server) StartSpeedTestScheduler() {
	if s.speedTestScheduler != nil {
		if err := s.speedTestScheduler.Start(); err != nil {
			logger.Error("启动测速调度器失败: %v", err)
		}
	}
}

// StopSpeedTestScheduler 停止测速定时调度器
func (s *Server) StopSpeedTestScheduler() {
	if s.speedTestScheduler != nil {
		s.speedTestScheduler.Stop()
	}
}

// setupRoutes 设置路由
func (s *Server) setupRoutes() {
	// CORS 配置
	s.router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// API 路由组
	api := s.router.Group("/api")
	{
		// 订阅管理
		api.GET("/subscriptions", s.getSubscriptions)
		api.POST("/subscriptions", s.addSubscription)
		api.PUT("/subscriptions/:id", s.updateSubscription)
		api.DELETE("/subscriptions/:id", s.deleteSubscription)
		api.POST("/subscriptions/:id/refresh", s.refreshSubscription)
		api.POST("/subscriptions/refresh-all", s.refreshAllSubscriptions)

		// 过滤器管理
		api.GET("/filters", s.getFilters)
		api.POST("/filters", s.addFilter)
		api.PUT("/filters/:id", s.updateFilter)
		api.DELETE("/filters/:id", s.deleteFilter)

		// 规则管理
		api.GET("/rules", s.getRules)
		api.POST("/rules", s.addRule)
		api.PUT("/rules/:id", s.updateRule)
		api.DELETE("/rules/:id", s.deleteRule)

		// 规则组管理
		api.GET("/rule-groups", s.getRuleGroups)
		api.PUT("/rule-groups/:id", s.updateRuleGroup)

		// 规则集验证
		api.GET("/ruleset/validate", s.validateRuleSet)

		// 设置
		api.GET("/settings", s.getSettings)
		api.PUT("/settings", s.updateSettings)

		// 系统 hosts
		api.GET("/system-hosts", s.getSystemHosts)

		// 配置生成
		api.POST("/config/generate", s.generateConfig)
		api.POST("/config/apply", s.applyConfig)
		api.GET("/config/preview", s.previewConfig)
		api.GET("/config/export", s.exportConfig)

		// 备份恢复
		api.GET("/backup", s.exportBackup)
		api.POST("/backup/restore", s.importBackup)

		// Profile 管理
		api.GET("/profiles", s.getProfiles)
		api.GET("/profiles/:id", s.getProfileData)
		api.POST("/profiles", s.createProfile)
		api.PUT("/profiles/:id", s.updateProfile)
		api.DELETE("/profiles/:id", s.deleteProfile)
		api.POST("/profiles/:id/activate", s.activateProfile)
		api.POST("/profiles/:id/snapshot", s.snapshotProfile)
		api.GET("/profiles/:id/export", s.exportProfile)
		api.POST("/profiles/import", s.importProfile)

		// 服务管理
		api.GET("/service/status", s.getServiceStatus)
		api.POST("/service/start", s.startService)
		api.POST("/service/stop", s.stopService)
		api.POST("/service/restart", s.restartService)
		api.POST("/service/reload", s.reloadService)

		// launchd 管理
		api.GET("/launchd/status", s.getLaunchdStatus)
		api.POST("/launchd/install", s.installLaunchd)
		api.POST("/launchd/uninstall", s.uninstallLaunchd)
		api.POST("/launchd/restart", s.restartLaunchd)

		// systemd 管理
		api.GET("/systemd/status", s.getSystemdStatus)
		api.POST("/systemd/install", s.installSystemd)
		api.POST("/systemd/uninstall", s.uninstallSystemd)
		api.POST("/systemd/restart", s.restartSystemd)

		// 统一守护进程管理（自动判断系统）
		api.GET("/daemon/status", s.getDaemonStatus)
		api.POST("/daemon/install", s.installDaemon)
		api.POST("/daemon/uninstall", s.uninstallDaemon)
		api.POST("/daemon/restart", s.restartDaemon)

		// 系统监控
		api.GET("/monitor/system", s.getSystemInfo)
		api.GET("/monitor/logs", s.getLogs)
		api.GET("/monitor/logs/sbm", s.getAppLogs)
		api.GET("/monitor/logs/singbox", s.getSingboxLogs)

		// 节点
		api.GET("/nodes", s.getAllNodes)
		api.GET("/nodes/grouped", s.getNodesGrouped)
		api.GET("/nodes/countries", s.getCountryGroups)
		api.GET("/nodes/country/:code", s.getNodesByCountry)
		api.POST("/nodes/parse", s.parseNodeURL)
		api.GET("/nodes/delays", s.getNodeDelays)
		api.POST("/nodes/:nodeId/delay", s.testNodeDelay)
		api.POST("/nodes/delays/refresh", s.refreshAllNodeDelays) // 批量刷新延迟

		// 手动节点
		api.GET("/manual-nodes", s.getManualNodes)
		api.POST("/manual-nodes", s.addManualNode)
		api.PUT("/manual-nodes/:id", s.updateManualNode)
		api.DELETE("/manual-nodes/:id", s.deleteManualNode)

		// 入站端口管理
		api.GET("/inbound-ports", s.getInboundPorts)
		api.POST("/inbound-ports", s.addInboundPort)
		api.PUT("/inbound-ports/:id", s.updateInboundPort)
		api.DELETE("/inbound-ports/:id", s.deleteInboundPort)

		// 代理链路管理
		api.GET("/proxy-chains", s.getProxyChains)
		api.POST("/proxy-chains", s.addProxyChain)
		api.PUT("/proxy-chains/:id", s.updateProxyChain)
		api.DELETE("/proxy-chains/:id", s.deleteProxyChain)

		// 代理链路健康检测
		api.GET("/proxy-chains/health", s.getAllChainHealth)
		api.GET("/proxy-chains/:id/health", s.getChainHealth)
		api.POST("/proxy-chains/:id/health/check", s.checkChainHealth)
		api.POST("/proxy-chains/:id/speed", s.checkChainSpeed)

		// 内核管理
		api.GET("/kernel/info", s.getKernelInfo)
		api.GET("/kernel/releases", s.getKernelReleases)
		api.POST("/kernel/download", s.startKernelDownload)
		api.GET("/kernel/progress", s.getKernelProgress)

		// 测速管理（需要 speedTestHandler 已初始化）
		if s.speedTestHandler != nil {
			// 测速策略
			api.GET("/speedtest/profiles", s.speedTestHandler.GetProfiles)
			api.GET("/speedtest/profiles/:id", s.speedTestHandler.GetProfile)
			api.POST("/speedtest/profiles", s.speedTestHandler.CreateProfile)
			api.PUT("/speedtest/profiles/:id", s.speedTestHandler.UpdateProfile)
			api.DELETE("/speedtest/profiles/:id", s.speedTestHandler.DeleteProfile)

			// 测速执行
			api.POST("/speedtest/run", s.speedTestHandler.RunTest)
			api.GET("/speedtest/tasks", s.speedTestHandler.GetTasks)
			api.GET("/speedtest/tasks/:id", s.speedTestHandler.GetTask)
			api.POST("/speedtest/tasks/:id/cancel", s.speedTestHandler.CancelTask)

			// 测速历史
			api.GET("/speedtest/nodes/:nodeId/history", s.speedTestHandler.GetNodeHistory)
		}

		// 标签管理（需要 tagHandler 已初始化）
		if s.tagHandler != nil {
			// 标签 CRUD
			api.GET("/tags", s.tagHandler.GetTags)
			api.GET("/tags/:id", s.tagHandler.GetTag)
			api.POST("/tags", s.tagHandler.CreateTag)
			api.PUT("/tags/:id", s.tagHandler.UpdateTag)
			api.DELETE("/tags/:id", s.tagHandler.DeleteTag)
			api.GET("/tags/groups", s.tagHandler.GetTagGroups)

			// 标签规则
			api.GET("/tag-rules", s.tagHandler.GetTagRules)
			api.GET("/tag-rules/:id", s.tagHandler.GetTagRule)
			api.POST("/tag-rules", s.tagHandler.CreateTagRule)
			api.PUT("/tag-rules/:id", s.tagHandler.UpdateTagRule)
			api.DELETE("/tag-rules/:id", s.tagHandler.DeleteTagRule)

			// 节点标签
			api.GET("/nodes/:nodeId/tags", s.tagHandler.GetNodeTags)
			api.PUT("/nodes/:nodeId/tags", s.tagHandler.SetNodeTags)
			api.POST("/nodes/:nodeId/tags", s.tagHandler.AddNodeTag)
			api.DELETE("/nodes/:nodeId/tags/:tagId", s.tagHandler.RemoveNodeTag)

			// 规则执行
			api.POST("/tags/apply-rules", s.tagHandler.ApplyTagRules)
		}
	}

	// Swagger 文档（默认关闭）
	if s.swaggerEnabled {
		s.setupSwagger()
	}

	// 静态文件服务（前端，使用嵌入的文件系统）
	distFS, err := web.GetDistFS()
	if err != nil {
		logger.Printf("加载前端资源失败: %v", err)
	} else {
		// 获取 assets 子目录
		assetsFS, _ := fs.Sub(distFS, "assets")
		s.router.StaticFS("/assets", http.FS(assetsFS))

		// 处理根路径和所有未匹配的路由（SPA 支持）
		indexHTML, _ := fs.ReadFile(distFS, "index.html")
		s.router.GET("/", func(c *gin.Context) {
			c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
		})
		s.router.NoRoute(func(c *gin.Context) {
			c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
		})
	}
}

// Run 运行服务器
func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}

// ==================== 订阅 API ====================

func (s *Server) getSubscriptions(c *gin.Context) {
	subs := s.subService.GetAll()
	c.JSON(http.StatusOK, gin.H{"data": subs})
}

func (s *Server) addSubscription(c *gin.Context) {
	var req struct {
		Name           string `json:"name" binding:"required"`
		URL            string `json:"url" binding:"required"`
		AutoUpdate     *bool  `json:"auto_update"`
		UpdateInterval int    `json:"update_interval"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sub, err := s.subService.Add(req.Name, req.URL, req.AutoUpdate, req.UpdateInterval)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 同步节点到 SQLite（用于测速模块）
	if err := s.syncNodesToSQLite(); err != nil {
		logger.Warn("同步节点到 SQLite 失败: %v", err)
	}

	// 自动应用配置
	if err := s.autoApplyConfig(); err != nil {
		c.JSON(http.StatusOK, gin.H{"data": sub, "warning": "添加成功，但自动应用配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": sub})
}

func (s *Server) updateSubscription(c *gin.Context) {
	id := c.Param("id")

	var sub storage.Subscription
	if err := c.ShouldBindJSON(&sub); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sub.ID = id
	if err := s.subService.Update(sub); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 自动应用配置
	if err := s.autoApplyConfig(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "更新成功，但自动应用配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

func (s *Server) deleteSubscription(c *gin.Context) {
	id := c.Param("id")

	if err := s.subService.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 同步节点到 SQLite（用于测速模块）
	if err := s.syncNodesToSQLite(); err != nil {
		logger.Warn("同步节点到 SQLite 失败: %v", err)
	}

	// 自动应用配置
	if err := s.autoApplyConfig(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "删除成功，但自动应用配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

func (s *Server) refreshSubscription(c *gin.Context) {
	id := c.Param("id")

	if err := s.subService.Refresh(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 同步节点到 SQLite（用于测速模块）
	if err := s.syncNodesToSQLite(); err != nil {
		logger.Warn("同步节点到 SQLite 失败: %v", err)
	}

	// 自动应用配置
	if err := s.autoApplyConfig(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "刷新成功，但自动应用配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "刷新成功"})
}

func (s *Server) refreshAllSubscriptions(c *gin.Context) {
	if err := s.subService.RefreshAll(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 同步节点到 SQLite（用于测速模块）
	if err := s.syncNodesToSQLite(); err != nil {
		logger.Warn("同步节点到 SQLite 失败: %v", err)
	}

	// 自动应用配置
	if err := s.autoApplyConfig(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "刷新成功，但自动应用配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "刷新成功"})
}

// ==================== 过滤器 API ====================

func (s *Server) getFilters(c *gin.Context) {
	filters := s.store.GetFilters()
	c.JSON(http.StatusOK, gin.H{"data": filters})
}

func (s *Server) addFilter(c *gin.Context) {
	var filter storage.Filter
	if err := c.ShouldBindJSON(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 生成 ID
	filter.ID = uuid.New().String()

	if err := s.store.AddFilter(filter); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 自动应用配置
	if err := s.autoApplyConfig(); err != nil {
		c.JSON(http.StatusOK, gin.H{"data": filter, "warning": "添加成功，但自动应用配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": filter})
}

func (s *Server) updateFilter(c *gin.Context) {
	id := c.Param("id")

	var filter storage.Filter
	if err := c.ShouldBindJSON(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	filter.ID = id
	if err := s.store.UpdateFilter(filter); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 自动应用配置
	if err := s.autoApplyConfig(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "更新成功，但自动应用配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

func (s *Server) deleteFilter(c *gin.Context) {
	id := c.Param("id")

	if err := s.store.DeleteFilter(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 自动应用配置
	if err := s.autoApplyConfig(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "删除成功，但自动应用配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ==================== 规则 API ====================

func (s *Server) getRules(c *gin.Context) {
	rules := s.store.GetRules()
	c.JSON(http.StatusOK, gin.H{"data": rules})
}

func (s *Server) addRule(c *gin.Context) {
	var rule storage.Rule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 生成 ID
	rule.ID = uuid.New().String()

	if err := s.store.AddRule(rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 自动应用配置
	if err := s.autoApplyConfig(); err != nil {
		c.JSON(http.StatusOK, gin.H{"data": rule, "warning": "添加成功，但自动应用配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": rule})
}

func (s *Server) updateRule(c *gin.Context) {
	id := c.Param("id")

	var rule storage.Rule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rule.ID = id
	if err := s.store.UpdateRule(rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 自动应用配置
	if err := s.autoApplyConfig(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "更新成功，但自动应用配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

func (s *Server) deleteRule(c *gin.Context) {
	id := c.Param("id")

	if err := s.store.DeleteRule(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 自动应用配置
	if err := s.autoApplyConfig(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "删除成功，但自动应用配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ==================== 规则组 API ====================

func (s *Server) getRuleGroups(c *gin.Context) {
	ruleGroups := s.store.GetRuleGroups()
	c.JSON(http.StatusOK, gin.H{"data": ruleGroups})
}

func (s *Server) updateRuleGroup(c *gin.Context) {
	id := c.Param("id")

	var ruleGroup storage.RuleGroup
	if err := c.ShouldBindJSON(&ruleGroup); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ruleGroup.ID = id
	if err := s.store.UpdateRuleGroup(ruleGroup); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 自动应用配置
	if err := s.autoApplyConfig(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "更新成功，但自动应用配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// ==================== 规则集验证 API ====================

func (s *Server) validateRuleSet(c *gin.Context) {
	ruleType := c.Query("type") // geosite 或 geoip
	name := c.Query("name")     // 规则集名称

	if ruleType == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数 type 和 name 是必需的"})
		return
	}

	if ruleType != "geosite" && ruleType != "geoip" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type 必须是 geosite 或 geoip"})
		return
	}

	settings := s.store.GetSettings()
	var url string
	var tag string

	if ruleType == "geosite" {
		tag = "geosite-" + name
		url = settings.RuleSetBaseURL + "/geosite-" + name + ".srs"
	} else {
		tag = "geoip-" + name
		// geoip 使用相对路径
		url = settings.RuleSetBaseURL + "/../rule-set-geoip/geoip-" + name + ".srs"
	}

	// 如果配置了 GitHub 代理，添加代理前缀
	if settings.GithubProxy != "" {
		url = settings.GithubProxy + url
	}

	// 发送 HEAD 请求检查文件是否存在
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Head(url)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"valid":   false,
			"url":     url,
			"tag":     tag,
			"message": "无法访问规则集: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		c.JSON(http.StatusOK, gin.H{
			"valid":   true,
			"url":     url,
			"tag":     tag,
			"message": "规则集存在",
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"valid":   false,
			"url":     url,
			"tag":     tag,
			"message": "规则集不存在 (HTTP " + strconv.Itoa(resp.StatusCode) + ")",
		})
	}
}

// ==================== 设置 API ====================

func (s *Server) getSettings(c *gin.Context) {
	settings := s.store.GetSettings()
	settings.WebPort = s.port
	c.JSON(http.StatusOK, gin.H{"data": settings})
}

func (s *Server) updateSettings(c *gin.Context) {
	var settings storage.Settings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.store.UpdateSettings(&settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 更新进程管理器的配置路径（sing-box 路径是固定的，无需更新）
	s.processManager.SetConfigPath(s.resolvePath(settings.ConfigPath))

	// 重启调度器（可能更新了定时间隔）
	s.scheduler.Restart()

	// 自动应用配置
	if err := s.autoApplyConfig(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "更新成功，但自动应用配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// ==================== 系统 hosts API ====================

func (s *Server) getSystemHosts(c *gin.Context) {
	hosts := builder.ParseSystemHosts()

	var entries []storage.HostEntry
	for domain, ips := range hosts {
		entries = append(entries, storage.HostEntry{
			ID:      "system-" + domain,
			Domain:  domain,
			IPs:     ips,
			Enabled: true,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": entries})
}

// ==================== 配置 API ====================

func (s *Server) generateConfig(c *gin.Context) {
	configJSON, err := s.buildConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": configJSON})
}

func (s *Server) previewConfig(c *gin.Context) {
	configJSON, err := s.buildConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.String(http.StatusOK, configJSON)
}

func (s *Server) applyConfig(c *gin.Context) {
	configJSON, err := s.buildConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 保存配置文件
	settings := s.store.GetSettings()
	if err := s.saveConfigFile(s.resolvePath(settings.ConfigPath), configJSON); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 检查配置
	if err := s.processManager.Check(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 重启服务
	if s.processManager.IsRunning() {
		if err := s.processManager.Restart(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "配置已应用"})
}

func (s *Server) buildConfig() (string, error) {
	settings := s.store.GetSettings()
	nodes := s.store.GetAllNodes()
	filters := s.store.GetFilters()
	rules := s.store.GetRules()
	ruleGroups := s.store.GetRuleGroups()
	inboundPorts := s.store.GetInboundPorts()
	proxyChains := s.store.GetProxyChains()

	b := builder.NewConfigBuilder(settings, nodes, filters, rules, ruleGroups, inboundPorts, proxyChains)
	b.SetDataDir(s.store.GetDataDir()) // 设置数据目录用于生成绝对路径
	return b.BuildJSON()
}

func (s *Server) saveConfigFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

// exportConfig 导出 sing-box 配置文件（下载）
func (s *Server) exportConfig(c *gin.Context) {
	configJSON, err := s.buildConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 设置响应头，触发浏览器下载
	c.Header("Content-Disposition", "attachment; filename=singbox-config.json")
	c.Header("Content-Type", "application/json")
	c.String(http.StatusOK, configJSON)
}

// exportBackup 导出应用完整数据（备份）
func (s *Server) exportBackup(c *gin.Context) {
	dataFile := filepath.Join(s.store.GetDataDir(), "data.json")
	data, err := os.ReadFile(dataFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取数据文件失败: " + err.Error()})
		return
	}

	filename := fmt.Sprintf("sbm-backup-%s.json", time.Now().Format("20060102-150405"))
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "application/json")
	c.Data(http.StatusOK, "application/json", data)
}

// importBackup 导入应用数据（恢复）
func (s *Server) importBackup(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请上传文件"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取文件失败: " + err.Error()})
		return
	}

	// 验证 JSON 格式
	var appData storage.AppData
	if err := json.Unmarshal(data, &appData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的备份文件格式: " + err.Error()})
		return
	}

	// 备份当前数据
	dataDir := s.store.GetDataDir()
	currentDataFile := filepath.Join(dataDir, "data.json")
	backupFile := filepath.Join(dataDir, fmt.Sprintf("data-backup-%s.json", time.Now().Format("20060102-150405")))
	if currentData, err := os.ReadFile(currentDataFile); err == nil {
		os.WriteFile(backupFile, currentData, 0644)
	}

	// 写入新数据
	if err := os.WriteFile(currentDataFile, data, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写入数据失败: " + err.Error()})
		return
	}

	// 重新加载存储
	if err := s.store.Reload(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "重新加载数据失败: " + err.Error()})
		return
	}

	// 自动应用配置
	s.autoApplyConfig()

	c.JSON(http.StatusOK, gin.H{"message": "数据已恢复", "backup": backupFile})
}

// ==================== Profile API ====================

// getProfiles 获取所有 Profile
func (s *Server) getProfiles(c *gin.Context) {
	profiles, err := s.profileMgr.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	activeProfile := s.profileMgr.GetActiveProfile()
	c.JSON(http.StatusOK, gin.H{
		"data":           profiles,
		"active_profile": activeProfile,
	})
}

// getProfileData 获取单个 Profile 信息
func (s *Server) getProfileData(c *gin.Context) {
	name := c.Param("id") // 这里 id 实际上是 profile name

	profiles, err := s.profileMgr.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for _, p := range profiles {
		if p.Name == name {
			c.JSON(http.StatusOK, gin.H{"profile": p})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "Profile 不存在"})
}

// createProfile 创建新 Profile
func (s *Server) createProfile(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.profileMgr.Create(req.Name, req.Description); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Profile 创建成功", "name": req.Name})
}

// updateProfile 更新 Profile 元数据
func (s *Server) updateProfile(c *gin.Context) {
	name := c.Param("id")

	var req struct {
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.profileMgr.UpdateInfo(name, req.Description); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// deleteProfile 删除 Profile
func (s *Server) deleteProfile(c *gin.Context) {
	name := c.Param("id")

	if err := s.profileMgr.Delete(name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// activateProfile 切换到指定 Profile
func (s *Server) activateProfile(c *gin.Context) {
	name := c.Param("id")

	// 停止当前服务
	if s.processManager != nil {
		s.processManager.Stop()
	}

	// 切换 Profile
	if err := s.profileMgr.Switch(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 重新初始化 store 和相关服务
	profileDir := s.profileMgr.GetProfileDir()
	newStore, err := storage.NewJSONStore(profileDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "初始化存储失败: " + err.Error()})
		return
	}
	s.store = newStore

	// 更新数据库 Store
	s.dbStore = s.profileMgr.GetStore()

	// 重新创建相关服务
	s.chainSyncSvc = service.NewChainSyncService(s.store)
	s.subService = service.NewSubscriptionService(s.store, s.chainSyncSvc)
	s.healthCheckSvc = service.NewHealthCheckService(s.store)

	// 更新测速模块
	if s.dbStore != nil {
		s.speedTestExecutor = speedtest.NewExecutor(s.dbStore)
		s.speedTestScheduler = speedtest.NewScheduler(s.dbStore, s.speedTestExecutor)
		s.speedTestHandler = NewSpeedTestHandler(s.dbStore, s.speedTestExecutor, s.speedTestScheduler)
		s.tagEngine = service.NewTagEngine(s.dbStore)
		s.tagHandler = NewTagHandler(s.dbStore, s.tagEngine)
	}

	// 更新进程管理器的配置路径
	configPath := filepath.Join(profileDir, "generated", "config.json")
	os.MkdirAll(filepath.Join(profileDir, "generated"), 0755)
	s.processManager.SetConfigPath(configPath)

	c.JSON(http.StatusOK, gin.H{"message": "已切换到 Profile: " + name})
}

// snapshotProfile 克隆当前 Profile
func (s *Server) snapshotProfile(c *gin.Context) {
	name := c.Param("id")

	var req struct {
		DestName string `json:"dest_name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.profileMgr.Clone(name, req.DestName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "克隆成功", "name": req.DestName})
}

// exportProfile 导出 Profile（下载 zip）
func (s *Server) exportProfile(c *gin.Context) {
	name := c.Param("id")

	filename := fmt.Sprintf("profile-%s-%s.zip", name, time.Now().Format("20060102"))
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "application/zip")

	if err := s.profileMgr.Export(name, c.Writer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
}

// importProfile 导入 Profile（上传 zip）
func (s *Server) importProfile(c *gin.Context) {
	name := c.PostForm("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 Profile 名称"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请上传文件"})
		return
	}
	defer file.Close()

	// 读取文件内容到临时文件
	tmpFile, err := os.CreateTemp("", "profile-import-*.zip")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建临时文件失败"})
		return
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}

	// 重新打开文件用于读取
	tmpFile.Seek(0, 0)
	stat, _ := tmpFile.Stat()

	if err := s.profileMgr.Import(name, tmpFile, stat.Size()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "导入成功", "name": name, "original_file": header.Filename})
}

// resolvePath 将相对路径解析为基于数据目录的绝对路径
func (s *Server) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(s.store.GetDataDir(), path)
}

// autoApplyConfig 自动应用配置（如果 sing-box 正在运行）
func (s *Server) autoApplyConfig() error {
	settings := s.store.GetSettings()
	if !settings.AutoApply {
		return nil
	}

	// 生成配置
	configJSON, err := s.buildConfig()
	if err != nil {
		return err
	}

	// 保存配置文件
	if err := s.saveConfigFile(s.resolvePath(settings.ConfigPath), configJSON); err != nil {
		return err
	}

	// 如果 sing-box 正在运行，则重启
	if s.processManager.IsRunning() {
		return s.processManager.Restart()
	}

	return nil
}

// ==================== 服务 API ====================

func (s *Server) getServiceStatus(c *gin.Context) {
	running := s.processManager.IsRunning()
	pid := s.processManager.GetPID()

	version := ""
	if v, err := s.processManager.Version(); err == nil {
		version = v
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"running":     running,
			"pid":         pid,
			"version":     version,
			"sbm_version": s.version,
		},
	})
}

func (s *Server) startService(c *gin.Context) {
	if err := s.processManager.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "服务已启动"})
}

func (s *Server) stopService(c *gin.Context) {
	if err := s.processManager.Stop(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "服务已停止"})
}

func (s *Server) restartService(c *gin.Context) {
	if err := s.processManager.Restart(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "服务已重启"})
}

func (s *Server) reloadService(c *gin.Context) {
	if err := s.processManager.Reload(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "配置已重载"})
}

// ==================== launchd API ====================

func (s *Server) getLaunchdStatus(c *gin.Context) {
	if s.launchdManager == nil {
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"installed": false,
				"running":   false,
				"plistPath": "",
				"supported": false,
			},
		})
		return
	}

	installed := s.launchdManager.IsInstalled()
	running := s.launchdManager.IsRunning()

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"installed": installed,
			"running":   running,
			"plistPath": s.launchdManager.GetPlistPath(),
			"supported": true,
		},
	})
}

func (s *Server) installLaunchd(c *gin.Context) {
	if s.launchdManager == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前系统不支持 launchd 服务"})
		return
	}

	// 获取用户主目录（支持多种方式）
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		// 备用方案：使用 os/user 包
		if u, err := user.Current(); err == nil && u.HomeDir != "" {
			homeDir = u.HomeDir
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户目录失败"})
			return
		}
	}

	// 确保日志目录存在
	logsDir := s.store.GetDataDir() + "/logs"

	config := daemon.LaunchdConfig{
		SbmPath:    s.sbmPath,
		DataDir:    s.store.GetDataDir(),
		Port:       strconv.Itoa(s.port),
		LogPath:    logsDir,
		WorkingDir: s.store.GetDataDir(),
		HomeDir:    homeDir,
		RunAtLoad:  true,
		KeepAlive:  true,
	}

	if err := s.launchdManager.Install(config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 安装成功后启动服务
	if err := s.launchdManager.Start(); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "服务已安装，但启动失败: " + err.Error() + "。请重启电脑或手动执行 launchctl load 命令",
			"action":  "manual",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "服务已安装并启动，您可以关闭此终端窗口。sbm 将在后台运行并开机自启。",
		"action":  "exit",
	})
}

func (s *Server) uninstallLaunchd(c *gin.Context) {
	if s.launchdManager == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前系统不支持 launchd 服务"})
		return
	}

	if err := s.launchdManager.Uninstall(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "服务已卸载"})
}

func (s *Server) restartLaunchd(c *gin.Context) {
	if s.launchdManager == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前系统不支持 launchd 服务"})
		return
	}

	if err := s.launchdManager.Restart(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "服务已重启"})
}

// ==================== systemd API ====================

func (s *Server) getSystemdStatus(c *gin.Context) {
	if s.systemdManager == nil {
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"installed":   false,
				"running":     false,
				"servicePath": "",
				"supported":   false,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"installed":   s.systemdManager.IsInstalled(),
			"running":     s.systemdManager.IsRunning(),
			"servicePath": s.systemdManager.GetServicePath(),
			"supported":   true,
		},
	})
}

func (s *Server) installSystemd(c *gin.Context) {
	if s.systemdManager == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前系统不支持 systemd 服务"})
		return
	}

	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		if u, err := user.Current(); err == nil && u.HomeDir != "" {
			homeDir = u.HomeDir
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户目录失败"})
			return
		}
	}

	logsDir := s.store.GetDataDir() + "/logs"

	config := daemon.SystemdConfig{
		SbmPath:    s.sbmPath,
		DataDir:    s.store.GetDataDir(),
		Port:       strconv.Itoa(s.port),
		LogPath:    logsDir,
		WorkingDir: s.store.GetDataDir(),
		HomeDir:    homeDir,
		RunAtLoad:  true,
		KeepAlive:  true,
	}

	if err := s.systemdManager.Install(config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := s.systemdManager.Start(); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "服务已安装，但启动失败: " + err.Error() + "。请执行 systemctl --user start singbox-manager",
			"action":  "manual",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "服务已安装并启动，您可以关闭此终端窗口。sbm 将在后台运行并开机自启。",
		"action":  "exit",
	})
}

func (s *Server) uninstallSystemd(c *gin.Context) {
	if s.systemdManager == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前系统不支持 systemd 服务"})
		return
	}

	if err := s.systemdManager.Uninstall(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "服务已卸载"})
}

func (s *Server) restartSystemd(c *gin.Context) {
	if s.systemdManager == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前系统不支持 systemd 服务"})
		return
	}

	if err := s.systemdManager.Restart(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "服务已重启"})
}

// ==================== 统一守护进程 API ====================

func (s *Server) getDaemonStatus(c *gin.Context) {
	platform := runtime.GOOS
	var installed, running, supported bool
	var configPath string

	switch platform {
	case "darwin":
		if s.launchdManager != nil {
			supported = true
			installed = s.launchdManager.IsInstalled()
			running = s.launchdManager.IsRunning()
			configPath = s.launchdManager.GetPlistPath()
		}
	case "linux":
		if s.systemdManager != nil {
			supported = true
			installed = s.systemdManager.IsInstalled()
			running = s.systemdManager.IsRunning()
			configPath = s.systemdManager.GetServicePath()
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"installed":  installed,
			"running":    running,
			"configPath": configPath,
			"supported":  supported,
			"platform":   platform,
		},
	})
}

func (s *Server) installDaemon(c *gin.Context) {
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		if u, err := user.Current(); err == nil && u.HomeDir != "" {
			homeDir = u.HomeDir
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户目录失败"})
			return
		}
	}

	logsDir := s.store.GetDataDir() + "/logs"

	switch runtime.GOOS {
	case "darwin":
		if s.launchdManager == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "当前系统不支持守护进程服务"})
			return
		}
		config := daemon.LaunchdConfig{
			SbmPath:    s.sbmPath,
			DataDir:    s.store.GetDataDir(),
			Port:       strconv.Itoa(s.port),
			LogPath:    logsDir,
			WorkingDir: s.store.GetDataDir(),
			HomeDir:    homeDir,
			RunAtLoad:  true,
			KeepAlive:  true,
		}
		if err := s.launchdManager.Install(config); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := s.launchdManager.Start(); err != nil {
			c.JSON(http.StatusOK, gin.H{"message": "服务已安装，但启动失败: " + err.Error(), "action": "manual"})
			return
		}
	case "linux":
		if s.systemdManager == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "当前系统不支持守护进程服务"})
			return
		}
		config := daemon.SystemdConfig{
			SbmPath:    s.sbmPath,
			DataDir:    s.store.GetDataDir(),
			Port:       strconv.Itoa(s.port),
			LogPath:    logsDir,
			WorkingDir: s.store.GetDataDir(),
			HomeDir:    homeDir,
			RunAtLoad:  true,
			KeepAlive:  true,
		}
		if err := s.systemdManager.Install(config); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := s.systemdManager.Start(); err != nil {
			c.JSON(http.StatusOK, gin.H{"message": "服务已安装，但启动失败: " + err.Error(), "action": "manual"})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前系统不支持守护进程服务"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "服务已安装并启动", "action": "exit"})
}

func (s *Server) uninstallDaemon(c *gin.Context) {
	var err error
	switch runtime.GOOS {
	case "darwin":
		if s.launchdManager == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "当前系统不支持守护进程服务"})
			return
		}
		err = s.launchdManager.Uninstall()
	case "linux":
		if s.systemdManager == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "当前系统不支持守护进程服务"})
			return
		}
		err = s.systemdManager.Uninstall()
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前系统不支持守护进程服务"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "服务已卸载"})
}

func (s *Server) restartDaemon(c *gin.Context) {
	var err error
	switch runtime.GOOS {
	case "darwin":
		if s.launchdManager == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "当前系统不支持守护进程服务"})
			return
		}
		err = s.launchdManager.Restart()
	case "linux":
		if s.systemdManager == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "当前系统不支持守护进程服务"})
			return
		}
		err = s.systemdManager.Restart()
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前系统不支持守护进程服务"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "服务已重启"})
}

// ==================== 监控 API ====================

// ProcessStats 进程资源统计
type ProcessStats struct {
	PID        int     `json:"pid"`
	CPUPercent float64 `json:"cpu_percent"`
	MemoryMB   float64 `json:"memory_mb"`
}

func (s *Server) getSystemInfo(c *gin.Context) {
	result := gin.H{}

	// 获取 sbm 进程信息
	sbmPid := int32(os.Getpid())
	if sbmProc, err := process.NewProcess(sbmPid); err == nil {
		cpuPercent, _ := sbmProc.CPUPercent()
		var memoryMB float64
		if memInfo, err := sbmProc.MemoryInfo(); err == nil && memInfo != nil {
			memoryMB = float64(memInfo.RSS) / 1024 / 1024
		}

		result["sbm"] = ProcessStats{
			PID:        int(sbmPid),
			CPUPercent: cpuPercent,
			MemoryMB:   memoryMB,
		}
	}

	// 获取 sing-box 进程信息
	if s.processManager.IsRunning() {
		singboxPid := int32(s.processManager.GetPID())
		if singboxProc, err := process.NewProcess(singboxPid); err == nil {
			cpuPercent, _ := singboxProc.CPUPercent()
			var memoryMB float64
			if memInfo, err := singboxProc.MemoryInfo(); err == nil && memInfo != nil {
				memoryMB = float64(memInfo.RSS) / 1024 / 1024
			}

			result["singbox"] = ProcessStats{
				PID:        int(singboxPid),
				CPUPercent: cpuPercent,
				MemoryMB:   memoryMB,
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (s *Server) getLogs(c *gin.Context) {
	lines := 200 // 默认返回 200 行
	if linesParam := c.Query("lines"); linesParam != "" {
		if n, err := strconv.Atoi(linesParam); err == nil && n > 0 {
			lines = n
		}
	}

	// 返回程序日志，不混合 sing-box 输出
	logs, err := logger.ReadAppLogs(lines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": logs})
}

// getAppLogs 获取应用日志
func (s *Server) getAppLogs(c *gin.Context) {
	lines := 200 // 默认返回 200 行
	if linesParam := c.Query("lines"); linesParam != "" {
		if n, err := strconv.Atoi(linesParam); err == nil && n > 0 {
			lines = n
		}
	}

	logs, err := logger.ReadAppLogs(lines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": logs})
}

// getSingboxLogs 获取 sing-box 日志
func (s *Server) getSingboxLogs(c *gin.Context) {
	lines := 200 // 默认返回 200 行
	if linesParam := c.Query("lines"); linesParam != "" {
		if n, err := strconv.Atoi(linesParam); err == nil && n > 0 {
			lines = n
		}
	}

	logs, err := logger.ReadSingboxLogs(lines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": logs})
}

// ==================== 节点 API ====================

func (s *Server) getAllNodes(c *gin.Context) {
	nodes := s.store.GetAllNodes()
	c.JSON(http.StatusOK, gin.H{"data": nodes})
}

func (s *Server) getNodesGrouped(c *gin.Context) {
	groups := s.store.GetNodesGrouped()
	c.JSON(http.StatusOK, gin.H{"data": groups})
}

func (s *Server) getCountryGroups(c *gin.Context) {
	groups := s.store.GetCountryGroups()
	c.JSON(http.StatusOK, gin.H{"data": groups})
}

func (s *Server) getNodesByCountry(c *gin.Context) {
	code := c.Param("code")
	nodes := s.store.GetNodesByCountry(code)
	c.JSON(http.StatusOK, gin.H{"data": nodes})
}

func (s *Server) parseNodeURL(c *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	node, err := parser.ParseURL(req.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "解析失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": node})
}

// NodeSpeedInfo 节点测速信息
type NodeSpeedInfo struct {
	Delay int     `json:"delay"` // 延迟 (ms), -1 表示超时, 0 表示未测试
	Speed float64 `json:"speed"` // 速度 (MB/s)
}

// getNodeDelays 获取所有节点的延迟和速度 (从 SQLite 数据库)
func (s *Server) getNodeDelays(c *gin.Context) {
	// 从 SQLite 获取所有节点的延迟信息
	if s.dbStore == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未初始化"})
		return
	}

	nodes, err := s.dbStore.GetNodes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取节点失败: " + err.Error()})
		return
	}

	// 构建测速信息映射
	speedInfos := make(map[string]NodeSpeedInfo)
	for _, node := range nodes {
		info := NodeSpeedInfo{}
		// 设置延迟
		if node.DelayStatus == "success" && node.Delay > 0 {
			info.Delay = node.Delay
		} else if node.DelayStatus == "timeout" || node.DelayStatus == "error" {
			info.Delay = -1
		}
		// 设置速度
		if node.SpeedStatus == "success" && node.Speed > 0 {
			info.Speed = node.Speed
		}
		// 只返回有测试数据的节点
		if info.Delay != 0 || info.Speed > 0 {
			speedInfos[node.Tag] = info
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": speedInfos})
}

// testNodeDelay 测试单个节点的延迟 (使用 mihomo)
func (s *Server) testNodeDelay(c *gin.Context) {
	tag := c.Param("nodeId")

	if s.dbStore == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未初始化"})
		return
	}

	// 从 SQLite 查找节点
	node, err := s.dbStore.GetNodeByTag(tag)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在: " + tag})
		return
	}

	// 使用 mihomo 测速
	profile := &models.SpeedTestProfile{
		LatencyURL:       "https://www.gstatic.com/generate_204",
		Timeout:          7,
		IncludeHandshake: true,
	}
	tester := speedtest.NewTester(profile)
	result := tester.TestDelay(node)

	// 更新节点延迟信息
	now := time.Now()
	node.Delay = result.Delay
	node.DelayStatus = result.Status
	node.TestedAt = &now
	if err := s.dbStore.UpdateNode(node); err != nil {
		logger.Warn("更新节点延迟失败: %v", err)
	}

	if result.Status == "success" {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"tag": tag, "delay": result.Delay}})
	} else {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"tag": tag, "delay": -1, "error": result.Error}})
	}
}

// refreshAllNodeDelays 批量刷新所有节点的延迟 (使用测速模块)
func (s *Server) refreshAllNodeDelays(c *gin.Context) {
	if s.speedTestExecutor == nil || s.dbStore == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "测速模块未初始化"})
		return
	}

	// 使用默认测速策略执行延迟测试
	defaultProfile, err := s.dbStore.GetDefaultSpeedTestProfile()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取默认测速策略失败"})
		return
	}

	// 临时覆盖为仅延迟模式（避免触发速度测试）
	profileCopy := *defaultProfile
	profileCopy.Mode = speedtest.TestModeDelay

	task, err := s.speedTestExecutor.RunWithProfileConfig(&profileCopy, nil, speedtest.TriggerTypeManual)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "延迟测试任务已启动",
		"task_id": task.ID,
	})
}

// ==================== 手动节点 API ====================

func (s *Server) getManualNodes(c *gin.Context) {
	nodes := s.store.GetManualNodes()
	c.JSON(http.StatusOK, gin.H{"data": nodes})
}

func (s *Server) addManualNode(c *gin.Context) {
	var node storage.ManualNode
	if err := c.ShouldBindJSON(&node); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 生成 ID
	node.ID = uuid.New().String()

	if err := s.store.AddManualNode(node); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 自动应用配置
	if err := s.autoApplyConfig(); err != nil {
		c.JSON(http.StatusOK, gin.H{"data": node, "warning": "添加成功，但自动应用配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": node})
}

func (s *Server) updateManualNode(c *gin.Context) {
	id := c.Param("id")

	var node storage.ManualNode
	if err := c.ShouldBindJSON(&node); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	node.ID = id
	if err := s.store.UpdateManualNode(node); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 自动应用配置
	if err := s.autoApplyConfig(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "更新成功，但自动应用配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

func (s *Server) deleteManualNode(c *gin.Context) {
	id := c.Param("id")

	if err := s.store.DeleteManualNode(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 自动应用配置
	if err := s.autoApplyConfig(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "删除成功，但自动应用配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ==================== 内核管理 API ====================

func (s *Server) getKernelInfo(c *gin.Context) {
	info := s.kernelManager.GetInfo()
	c.JSON(http.StatusOK, gin.H{"data": info})
}

func (s *Server) getKernelReleases(c *gin.Context) {
	releases, err := s.kernelManager.FetchReleases()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 只返回版本号和名称，不返回完整的 assets
	type ReleaseInfo struct {
		TagName string `json:"tag_name"`
		Name    string `json:"name"`
	}

	result := make([]ReleaseInfo, len(releases))
	for i, r := range releases {
		result[i] = ReleaseInfo{
			TagName: r.TagName,
			Name:    r.Name,
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (s *Server) startKernelDownload(c *gin.Context) {
	var req struct {
		Version string `json:"version" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.kernelManager.StartDownload(req.Version); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "下载已开始"})
}

func (s *Server) getKernelProgress(c *gin.Context) {
	progress := s.kernelManager.GetProgress()
	c.JSON(http.StatusOK, gin.H{"data": progress})
}

// ==================== 入站端口 API ====================

func (s *Server) getInboundPorts(c *gin.Context) {
	ports := s.store.GetInboundPorts()
	c.JSON(http.StatusOK, gin.H{"data": ports})
}

func (s *Server) addInboundPort(c *gin.Context) {
	var port storage.InboundPort
	if err := c.ShouldBindJSON(&port); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 生成 ID
	port.ID = uuid.New().String()

	if err := s.store.AddInboundPort(port); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 自动应用配置
	if err := s.autoApplyConfig(); err != nil {
		c.JSON(http.StatusOK, gin.H{"data": port, "warning": "添加成功，但自动应用配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": port})
}

func (s *Server) updateInboundPort(c *gin.Context) {
	id := c.Param("id")

	var port storage.InboundPort
	if err := c.ShouldBindJSON(&port); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	port.ID = id
	if err := s.store.UpdateInboundPort(port); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 自动应用配置
	if err := s.autoApplyConfig(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "更新成功，但自动应用配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

func (s *Server) deleteInboundPort(c *gin.Context) {
	id := c.Param("id")

	if err := s.store.DeleteInboundPort(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 自动应用配置
	if err := s.autoApplyConfig(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "删除成功，但自动应用配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ==================== 代理链路 API ====================

func (s *Server) getProxyChains(c *gin.Context) {
	chains := s.store.GetProxyChains()
	c.JSON(http.StatusOK, gin.H{"data": chains})
}

func (s *Server) addProxyChain(c *gin.Context) {
	var chain storage.ProxyChain
	if err := c.ShouldBindJSON(&chain); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 生成 ID
	chain.ID = uuid.New().String()

	// 自动生成 ChainNodes（从 Nodes 生成副本信息）
	chain.ChainNodes = s.generateChainNodes(chain.Name, chain.Nodes)

	if err := s.store.AddProxyChain(chain); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 自动应用配置
	if err := s.autoApplyConfig(); err != nil {
		c.JSON(http.StatusOK, gin.H{"data": chain, "warning": "添加成功，但自动应用配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": chain})
}

func (s *Server) updateProxyChain(c *gin.Context) {
	id := c.Param("id")

	var chain storage.ProxyChain
	if err := c.ShouldBindJSON(&chain); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	chain.ID = id

	// 自动生成 ChainNodes（从 Nodes 生成副本信息）
	chain.ChainNodes = s.generateChainNodes(chain.Name, chain.Nodes)

	if err := s.store.UpdateProxyChain(chain); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 自动应用配置
	if err := s.autoApplyConfig(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "更新成功，但自动应用配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

func (s *Server) deleteProxyChain(c *gin.Context) {
	id := c.Param("id")

	if err := s.store.DeleteProxyChain(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 自动应用配置
	if err := s.autoApplyConfig(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "删除成功，但自动应用配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// generateChainNodes 根据节点 Tag 列表生成 ChainNode 列表
func (s *Server) generateChainNodes(chainName string, nodeTags []string) []storage.ChainNode {
	allNodes := s.store.GetAllNodes()
	nodeMap := make(map[string]storage.Node)
	for _, n := range allNodes {
		nodeMap[n.Tag] = n
	}

	result := make([]storage.ChainNode, 0, len(nodeTags))
	for _, tag := range nodeTags {
		node, exists := nodeMap[tag]
		source := ""
		if exists {
			source = node.Source
		}
		result = append(result, storage.ChainNode{
			OriginalTag: tag,
			CopyTag:     storage.GenerateChainNodeCopyTag(chainName, tag),
			Source:      source,
		})
	}
	return result
}

// ==================== 链路健康检测 API ====================

func (s *Server) getAllChainHealth(c *gin.Context) {
	statuses := s.healthCheckSvc.GetAllCachedStatuses()
	c.JSON(http.StatusOK, gin.H{"data": statuses})
}

func (s *Server) getChainHealth(c *gin.Context) {
	id := c.Param("id")
	status := s.healthCheckSvc.GetCachedStatus(id)
	c.JSON(http.StatusOK, gin.H{"data": status})
}

func (s *Server) checkChainHealth(c *gin.Context) {
	id := c.Param("id")
	status, err := s.healthCheckSvc.CheckChain(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": status})
}

func (s *Server) checkChainSpeed(c *gin.Context) {
	id := c.Param("id")
	result, err := s.healthCheckSvc.CheckChainSpeed(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// ==================== 节点同步 ====================

// syncNodesToSQLite 同步 JSONStore 节点到 SQLite 数据库 (用于测速模块)
func (s *Server) syncNodesToSQLite() error {
	if s.dbStore == nil {
		return nil // SQLite 未初始化，跳过同步
	}

	logger.Debug("开始同步节点到 SQLite 数据库...")

	// 获取 JSONStore 中的所有订阅和节点
	subs := s.store.GetSubscriptions()
	manualNodes := s.store.GetManualNodes()

	// 构建 source -> 节点列表 的映射
	sourceNodes := make(map[string][]storage.Node)

	// 订阅节点
	for _, sub := range subs {
		if sub.Enabled {
			sourceNodes[sub.ID] = sub.Nodes
		}
	}

	// 手动节点
	var enabledManualNodes []storage.Node
	for _, mn := range manualNodes {
		if mn.Enabled {
			mn.Node.Source = "manual"
			mn.Node.SourceName = "手动添加"
			enabledManualNodes = append(enabledManualNodes, mn.Node)
		}
	}
	if len(enabledManualNodes) > 0 {
		sourceNodes["manual"] = enabledManualNodes
	}

	// 获取数据库中现有节点
	existingNodes, err := s.dbStore.GetNodes()
	if err != nil {
		return fmt.Errorf("获取现有节点失败: %w", err)
	}

	// 构建现有节点的 tag -> node 映射
	existingMap := make(map[string]*models.Node)
	for i := range existingNodes {
		existingMap[existingNodes[i].Tag] = &existingNodes[i]
	}

	// 构建需要的节点 tag 集合
	neededTags := make(map[string]bool)
	var nodesToCreate []models.Node
	var nodesToUpdate []models.Node

	for source, nodes := range sourceNodes {
		// 获取订阅名称
		sourceName := "手动添加"
		if source != "manual" {
			for _, sub := range subs {
				if sub.ID == source {
					sourceName = sub.Name
					break
				}
			}
		}

		for _, node := range nodes {
			neededTags[node.Tag] = true

			if existing, ok := existingMap[node.Tag]; ok {
				// 节点已存在，检查是否需要更新基本信息
				needUpdate := false
				if existing.Server != node.Server || existing.ServerPort != node.ServerPort ||
					existing.Type != node.Type || existing.Source != source {
					existing.Server = node.Server
					existing.ServerPort = node.ServerPort
					existing.Type = node.Type
					existing.Source = source
					existing.SourceName = sourceName
					existing.Country = node.Country
					existing.CountryEmoji = node.CountryEmoji
					existing.Extra = models.JSONMap(node.Extra)
					needUpdate = true
				}
				// 确保节点启用
				if !existing.Enabled {
					existing.Enabled = true
					needUpdate = true
				}
				if needUpdate {
					nodesToUpdate = append(nodesToUpdate, *existing)
				}
			} else {
				// 新节点，需要创建
				dbNode := models.Node{
					Tag:          node.Tag,
					Type:         node.Type,
					Server:       node.Server,
					ServerPort:   node.ServerPort,
					Source:       source,
					SourceName:   sourceName,
					Country:      node.Country,
					CountryEmoji: node.CountryEmoji,
					Extra:        models.JSONMap(node.Extra),
					Enabled:      true,
					DelayStatus:  "untested",
					SpeedStatus:  "untested",
				}
				nodesToCreate = append(nodesToCreate, dbNode)
			}
		}
	}

	// 标记不再需要的节点为禁用
	var nodesToDisable []uint
	for tag, existing := range existingMap {
		if !neededTags[tag] && existing.Enabled {
			nodesToDisable = append(nodesToDisable, existing.ID)
		}
	}

	// 执行数据库操作
	if len(nodesToCreate) > 0 {
		if err := s.dbStore.BatchCreateNodes(nodesToCreate); err != nil {
			return fmt.Errorf("批量创建节点失败: %w", err)
		}
		logger.Debug("创建了 %d 个新节点", len(nodesToCreate))
	}

	for _, node := range nodesToUpdate {
		if err := s.dbStore.UpdateNode(&node); err != nil {
			logger.Warn("更新节点 %s 失败: %v", node.Tag, err)
		}
	}
	if len(nodesToUpdate) > 0 {
		logger.Debug("更新了 %d 个节点", len(nodesToUpdate))
	}

	// 禁用不再需要的节点
	if len(nodesToDisable) > 0 {
		for _, id := range nodesToDisable {
			node, err := s.dbStore.GetNode(id)
			if err == nil && node != nil {
				node.Enabled = false
				s.dbStore.UpdateNode(node)
			}
		}
		logger.Debug("禁用了 %d 个节点", len(nodesToDisable))
	}

	logger.Debug("节点同步完成")
	return nil
}
