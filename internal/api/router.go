package api

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/xiaobei/singbox-manager/internal/auth"
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
	"github.com/xiaobei/singbox-manager/internal/zashboard"
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
	chainSyncSvc   *service.ChainSyncService
	healthCheckSvc *service.HealthCheckService
	authManager    *auth.Manager
	router         *gin.Engine
	sbmPath        string // sbm 可执行文件路径
	port           int    // Web 服务端口
	version        string // sbm 版本号
	swaggerEnabled bool   // 是否启用 Swagger
	baseDir        string // 基础数据目录
	// SQLite 存储和测速模块
	dbStore           *database.Store
	speedTestExecutor *speedtest.Executor
	speedTestHandler  *SpeedTestHandler
	// 标签模块
	tagEngine  *service.TagEngine
	tagHandler *TagHandler
	// 任务管理模块
	taskManager  *service.TaskManager
	taskHandler  *TaskHandler
	eventTrigger *service.EventTrigger
	// 统一调度器
	unifiedScheduler *service.UnifiedScheduler
	schedulerHandler *SchedulerHandler
	// SSE 事件
	eventsHandler *EventsHandler
}

// NewServer 创建 API 服务器
func NewServer(profileMgr *profile.Manager, processManager *daemon.ProcessManager, launchdManager *daemon.LaunchdManager, systemdManager *daemon.SystemdManager, sbmPath string, port int, version string, swaggerEnabled bool) (*Server, error) {
	gin.SetMode(gin.ReleaseMode)

	// 获取当前 Profile 目录，创建 JSONStore（用于兼容旧代码）
	profileDir := profileMgr.GetProfileDir()
	store, err := storage.NewJSONStore(profileDir)
	if err != nil {
		return nil, fmt.Errorf("初始化 JSONStore 失败: %w", err)
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
	authManager, err := auth.NewManager(baseDir)
	if err != nil {
		return nil, fmt.Errorf("初始化认证管理器失败: %w", err)
	}

	// 创建健康检测服务
	healthCheckSvc := service.NewHealthCheckService(store)

	// 从 Profile 管理器获取数据库连接
	dbStore := profileMgr.GetStore()
	var speedTestExecutor *speedtest.Executor
	var speedTestHandler *SpeedTestHandler
	var tagEngine *service.TagEngine
	var tagHandler *TagHandler
	var taskManager *service.TaskManager
	var taskHandler *TaskHandler
	var eventTrigger *service.EventTrigger
	var unifiedScheduler *service.UnifiedScheduler
	var schedulerHandler *SchedulerHandler
	var eventsHandler *EventsHandler

	if dbStore != nil {
		speedTestExecutor = speedtest.NewExecutor(dbStore)
		speedTestHandler = NewSpeedTestHandler(dbStore, speedTestExecutor)
		tagEngine = service.NewTagEngine(dbStore)
		tagHandler = NewTagHandler(dbStore, tagEngine)
		taskManager = service.NewTaskManager(dbStore)
		taskHandler = NewTaskHandler(dbStore, taskManager)
		eventTrigger = service.NewEventTrigger(dbStore, taskManager)
		eventTrigger.SetTagEngine(tagEngine)
		unifiedScheduler = service.NewUnifiedScheduler(dbStore, taskManager)
		schedulerHandler = NewSchedulerHandler(unifiedScheduler)
		// 设置 SpeedTestHandler 的统一调度器引用
		speedTestHandler.SetUnifiedScheduler(unifiedScheduler)
		// 设置 Executor 的 TaskManager 引用
		speedTestExecutor.SetTaskManager(taskManager)
		// 设置 TagEngine 的 TaskManager 引用
		tagEngine.SetTaskManager(taskManager)
		// 创建 SSE 事件处理器
		eventsHandler = NewEventsHandler(taskManager)
		logger.Info("测速、标签、任务和调度模块初始化完成")
	}

	s := &Server{
		profileMgr:        profileMgr,
		store:             store,
		subService:        subService,
		processManager:    processManager,
		launchdManager:    launchdManager,
		systemdManager:    systemdManager,
		kernelManager:     kernelManager,
		chainSyncSvc:      chainSyncSvc,
		healthCheckSvc:    healthCheckSvc,
		authManager:       authManager,
		router:            gin.Default(),
		sbmPath:           sbmPath,
		port:              port,
		version:           version,
		swaggerEnabled:    swaggerEnabled,
		baseDir:           baseDir,
		dbStore:           dbStore,
		speedTestExecutor: speedTestExecutor,
		speedTestHandler:  speedTestHandler,
		tagEngine:         tagEngine,
		tagHandler:        tagHandler,
		taskManager:       taskManager,
		taskHandler:       taskHandler,
		eventTrigger:      eventTrigger,
		unifiedScheduler:  unifiedScheduler,
		schedulerHandler:  schedulerHandler,
		eventsHandler:     eventsHandler,
	}

	// 初始同步节点到 SQLite（仅在 SQLite 为空时）
	if s.dbStore != nil {
		if nodes, _ := s.dbStore.GetNodes(); len(nodes) == 0 {
			if err := s.syncNodesToSQLite(); err != nil {
				logger.Warn("启动时同步节点到 SQLite 失败: %v", err)
			} else {
				logger.Info("节点已同步到 SQLite 数据库")
			}
		}
	}

	s.setupRoutes()
	return s, nil
}

// StartUnifiedScheduler 启动统一调度器
func (s *Server) StartUnifiedScheduler() {
	if s.unifiedScheduler != nil {
		s.unifiedScheduler.Start()
		s.initScheduleEntries()
	}
}

// StopUnifiedScheduler 停止统一调度器
func (s *Server) StopUnifiedScheduler() {
	if s.unifiedScheduler != nil {
		s.unifiedScheduler.Stop()
	}
}

// initScheduleEntries 初始化调度条目
func (s *Server) initScheduleEntries() {
	if s.unifiedScheduler == nil {
		return
	}

	// 1. 订阅定时更新
	subs := s.store.GetSubscriptions()
	for _, sub := range subs {
		s.updateSubscriptionSchedule(sub)
	}

	// 2. 测速策略定时执行（已通过 Executor 自动创建 Task）
	if s.dbStore != nil {
		profiles, _ := s.dbStore.GetSpeedTestProfiles()
		for _, profile := range profiles {
			if profile.AutoTest {
				profileID := profile.ID
				profileName := profile.Name
				var cronExpr string
				if profile.ScheduleType == "cron" && profile.ScheduleCron != "" {
					cronExpr = profile.ScheduleCron
				} else {
					cronExpr = service.IntervalToCron(profile.ScheduleInterval)
				}
				s.unifiedScheduler.AddSchedule(
					service.ScheduleTypeSpeedTest,
					fmt.Sprintf("%d", profileID),
					"定时测速: "+profileName,
					cronExpr,
					func() {
						if s.speedTestExecutor != nil {
							s.speedTestExecutor.RunWithProfile(profileID, nil, speedtest.TriggerTypeScheduled)
						}
					},
				)
			}
		}
	}

	// 3. 链路健康检测
	settings := s.store.GetSettings()
	if settings.ChainHealthConfig != nil && settings.ChainHealthConfig.Enabled {
		interval := settings.ChainHealthConfig.Interval
		if interval < 30 {
			interval = 30
		}
		cronExpr := service.IntervalToCron(interval / 60) // 转换为分钟
		s.unifiedScheduler.AddSchedule(
			service.ScheduleTypeChainCheck,
			"global",
			"链路健康检测",
			cronExpr,
			func() {
				chains := s.store.GetProxyChains()
				var enabledChains []string
				for _, chain := range chains {
					if chain.Enabled {
						enabledChains = append(enabledChains, chain.ID)
					}
				}

				if len(enabledChains) == 0 {
					return
				}

				// 创建任务记录
				var task *models.Task
				if s.taskManager != nil {
					task, _, _ = s.taskManager.CreateTask(
						models.TaskTypeChainCheck,
						"链路健康检测",
						models.TaskTriggerScheduled,
						len(enabledChains),
					)
					s.taskManager.StartTask(task.ID)
				}

				results := make(map[string]interface{})
				for i, chainID := range enabledChains {
					chain := s.store.GetProxyChain(chainID)
					if chain != nil && task != nil {
						s.taskManager.UpdateProgress(task.ID, i+1, chain.Name, "")
					}
					status, _ := s.healthCheckSvc.CheckChain(chainID)
					results[chainID] = status
				}

				if task != nil {
					s.taskManager.CompleteTask(task.ID, "检测完成", results)
				}
			},
		)
	}

	logger.Info("调度条目初始化完成")
}

// updateSubscriptionSchedule 更新订阅调度
func (s *Server) updateSubscriptionSchedule(sub storage.Subscription) {
	if s.unifiedScheduler == nil {
		return
	}

	subID := sub.ID

	// 先移除旧的调度
	s.unifiedScheduler.RemoveSchedule(service.ScheduleTypeSubUpdate, subID)

	// 如果启用自动更新，添加新调度
	if sub.Enabled && sub.AutoUpdate != nil && *sub.AutoUpdate && sub.UpdateInterval > 0 {
		subName := sub.Name
		cronExpr := service.IntervalToCron(sub.UpdateInterval)
		s.unifiedScheduler.AddSchedule(
			service.ScheduleTypeSubUpdate,
			subID,
			"订阅更新: "+subName,
			cronExpr,
			func() {
				// 创建任务记录
				var task *models.Task
				if s.taskManager != nil {
					task, _, _ = s.taskManager.CreateTask(
						models.TaskTypeSubUpdate,
						"定时更新: "+subName,
						models.TaskTriggerScheduled,
						0,
					)
					s.taskManager.StartTask(task.ID)
				}

				if err := s.subService.Refresh(subID); err != nil {
					if task != nil {
						s.taskManager.FailTask(task.ID, err.Error())
					}
					return
				}
				nodeIDs := s.syncNodesToSQLiteAndGetIDs()
				if s.eventTrigger != nil {
					s.eventTrigger.OnSubscriptionUpdate(subID, nodeIDs)
				}
				s.autoApplyConfig()

				if task != nil {
					s.taskManager.CompleteTask(task.ID, "订阅更新成功", map[string]interface{}{
						"subscription_id": subID,
						"node_count":      len(nodeIDs),
					})
				}
			},
		)
	}
}

// ensureDefaultSpeedTestProfile 确保存在启用自动测速的策略
// 当添加订阅时，检查是否有启用自动测速的策略，如果没有则创建两个默认策略
func (s *Server) ensureDefaultSpeedTestProfile() {
	if s.dbStore == nil || s.unifiedScheduler == nil {
		return
	}

	// 检查是否已有启用自动测速的策略
	profiles, err := s.dbStore.GetSpeedTestProfiles()
	if err != nil {
		return
	}

	// 检查是否有启用 AutoTest 的策略
	hasAutoTest := false
	hasDelayProfile := false
	hasSpeedProfile := false
	for _, p := range profiles {
		if p.AutoTest && p.Enabled {
			s.addSpeedTestSchedule(&p)
			hasAutoTest = true
		}
		// Mode 为空或 "delay" 都视为延迟检测类型
		if p.Mode == "delay" || p.Mode == "" {
			hasDelayProfile = true
		} else if p.Mode == "speed" {
			hasSpeedProfile = true
		}
	}

	// 如果已有策略但没启用自动测速，自动启用所有策略的自动测速
	if len(profiles) > 0 && !hasAutoTest {
		for i := range profiles {
			profile := &profiles[i]
			profile.AutoTest = true
			profile.Enabled = true
			if profile.ScheduleCron == "" {
				profile.ScheduleType = "cron"
				if profile.Mode == "delay" || profile.Mode == "" {
					profile.ScheduleCron = "0 0 * * * *" // 延迟检测每小时
					profile.Mode = "delay"
					hasDelayProfile = true
				} else {
					profile.ScheduleCron = "0 30 */6 * * *" // 速度测试每6小时
					hasSpeedProfile = true
				}
			}
			if err := s.dbStore.UpdateSpeedTestProfile(profile); err != nil {
				logger.Warn("更新测速策略失败: %v", err)
				continue
			}
			s.addSpeedTestSchedule(profile)
			logger.Info("已启用测速策略 [%s] 的自动测速", profile.Name)
		}
	}

	// 补充缺失的策略类型（无论是否已有策略，都检查并补充）
	if !hasDelayProfile {
		delayProfile := &models.SpeedTestProfile{
			Name:               "延迟检测",
			Enabled:            true,
			IsDefault:          len(profiles) == 0,
			AutoTest:           true,
			ScheduleType:       "cron",
			ScheduleCron:       "0 0 * * * *",
			Mode:               "delay",
			LatencyURL:         "https://cp.cloudflare.com/generate_204",
			SpeedURL:           "https://speed.cloudflare.com/__down?bytes=5000000",
			Timeout:            5,
			LatencyConcurrency: 100,
			SpeedConcurrency:   5,
			SpeedRecordMode:    "average",
			PeakSampleInterval: 100,
			LandingIPURL:       "https://api.ipify.org",
		}
		if err := s.dbStore.CreateSpeedTestProfile(delayProfile); err != nil {
			logger.Warn("创建延迟检测策略失败: %v", err)
		} else {
			s.addSpeedTestSchedule(delayProfile)
			logger.Info("已创建延迟检测策略")
		}
	}

	if !hasSpeedProfile {
		speedProfile := &models.SpeedTestProfile{
			Name:               "速度测试",
			Enabled:            true,
			IsDefault:          false,
			AutoTest:           true,
			ScheduleType:       "cron",
			ScheduleCron:       "0 30 */6 * * *",
			Mode:               "speed",
			LatencyURL:         "https://cp.cloudflare.com/generate_204",
			SpeedURL:           "https://speed.cloudflare.com/__down?bytes=10000000",
			Timeout:            10,
			LatencyConcurrency: 50,
			SpeedConcurrency:   3,
			SpeedRecordMode:    "peak",
			PeakSampleInterval: 50,
			LandingIPURL:       "https://api.ipify.org",
			DetectCountry:      true,
		}
		if err := s.dbStore.CreateSpeedTestProfile(speedProfile); err != nil {
			logger.Warn("创建速度测试策略失败: %v", err)
		} else {
			s.addSpeedTestSchedule(speedProfile)
			logger.Info("已创建速度测试策略")
		}
	}
}

// addSpeedTestSchedule 添加测速调度
func (s *Server) addSpeedTestSchedule(profile *models.SpeedTestProfile) {
	if s.unifiedScheduler == nil || profile == nil {
		return
	}

	profileID := profile.ID
	profileName := profile.Name
	cronExpr := profile.ScheduleCron
	if cronExpr == "" {
		cronExpr = "0 0 */6 * * *"
	}

	s.unifiedScheduler.AddSchedule(
		service.ScheduleTypeSpeedTest,
		fmt.Sprintf("%d", profileID),
		"定时测速: "+profileName,
		cronExpr,
		func() {
			if s.speedTestExecutor != nil {
				s.speedTestExecutor.RunWithProfile(profileID, nil, speedtest.TriggerTypeScheduled)
			}
		},
	)
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

	// 公共认证路由
	publicAPI := s.router.Group("/api")
	s.registerPublicAuthRoutes(publicAPI)

	// API 路由组
	api := s.router.Group("/api")
	api.Use(s.requireAuthentication())
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
		api.POST("/nodes/test-unsaved", s.testUnsavedNodeDelay)
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
		api.GET("/proxy-chains/speed", s.getAllChainSpeed)
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

		// 任务管理（需要 taskHandler 已初始化）
		if s.taskHandler != nil {
			api.GET("/tasks", s.taskHandler.GetTasks)
			api.GET("/tasks/:id", s.taskHandler.GetTask)
			api.POST("/tasks/:id/cancel", s.taskHandler.CancelTask)
			api.GET("/tasks/running", s.taskHandler.GetRunningTasks)
			api.GET("/tasks/stats", s.taskHandler.GetTaskStats)
			api.DELETE("/tasks/history", s.taskHandler.CleanupTasks)
		}

		// 调度管理（需要 schedulerHandler 已初始化）
		if s.schedulerHandler != nil {
			api.GET("/scheduler/status", s.schedulerHandler.GetStatus)
			api.GET("/scheduler/entries", s.schedulerHandler.GetEntries)
			api.POST("/scheduler/entries/:key/enable", s.schedulerHandler.EnableEntry)
			api.POST("/scheduler/entries/:key/disable", s.schedulerHandler.DisableEntry)
			api.POST("/scheduler/entries/:key/trigger", s.schedulerHandler.TriggerEntry)
			api.POST("/scheduler/pause", s.schedulerHandler.PauseScheduler)
			api.POST("/scheduler/resume", s.schedulerHandler.ResumeScheduler)
		}

		// SSE 事件流（需要 eventsHandler 已初始化）
		if s.eventsHandler != nil {
			api.GET("/events/stream", s.eventsHandler.StreamTasks)
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

	// 创建任务记录
	var task *models.Task
	if s.taskManager != nil {
		task, _, _ = s.taskManager.CreateTask(models.TaskTypeSubUpdate, "添加订阅: "+req.Name, models.TaskTriggerManual, 0)
		s.taskManager.StartTask(task.ID)
	}

	sub, err := s.subService.Add(req.Name, req.URL, req.AutoUpdate, req.UpdateInterval)
	if err != nil {
		if task != nil {
			s.taskManager.FailTask(task.ID, err.Error())
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 更新订阅调度
	s.updateSubscriptionSchedule(*sub)

	// 创建默认测速策略（如果是第一个订阅）
	s.ensureDefaultSpeedTestProfile()

	// 同步节点到 SQLite（用于测速模块）
	nodeIDs := s.syncNodesToSQLiteAndGetIDs()

	// 触发订阅更新事件
	if s.eventTrigger != nil {
		s.eventTrigger.OnSubscriptionUpdate(sub.ID, nodeIDs)
	}

	// 完成任务
	if task != nil {
		s.taskManager.CompleteTask(task.ID, "订阅添加成功", map[string]interface{}{
			"subscription_id": sub.ID,
			"node_count":      sub.NodeCount,
		})
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

	// 先获取原有订阅
	existing := s.subService.Get(id)
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "订阅不存在"})
		return
	}

	// 只绑定前端发送的字段
	var req struct {
		Name           string `json:"name"`
		URL            string `json:"url"`
		AutoUpdate     *bool  `json:"auto_update"`
		UpdateInterval *int   `json:"update_interval"`
		Enabled        *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 更新指定字段
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.URL != "" {
		existing.URL = req.URL
	}
	if req.AutoUpdate != nil {
		existing.AutoUpdate = req.AutoUpdate
	}
	if req.UpdateInterval != nil {
		existing.UpdateInterval = *req.UpdateInterval
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}

	if err := s.subService.Update(*existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 更新订阅调度
	s.updateSubscriptionSchedule(*existing)

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

	// 移除调度
	if s.unifiedScheduler != nil {
		s.unifiedScheduler.RemoveSchedule(service.ScheduleTypeSubUpdate, id)
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

	// 获取订阅信息用于任务名称
	sub := s.subService.Get(id)
	subName := "订阅"
	if sub != nil {
		subName = sub.Name
	}

	// 创建任务记录
	var task *models.Task
	if s.taskManager != nil {
		task, _, _ = s.taskManager.CreateTask(models.TaskTypeSubUpdate, "更新订阅: "+subName, models.TaskTriggerManual, 0)
		s.taskManager.StartTask(task.ID)
	}

	if err := s.subService.Refresh(id); err != nil {
		if task != nil {
			s.taskManager.FailTask(task.ID, err.Error())
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 同步节点到 SQLite（用于测速模块）
	nodeIDs := s.syncNodesToSQLiteAndGetIDs()

	// 触发订阅更新事件（应用标签规则等）
	if s.eventTrigger != nil {
		s.eventTrigger.OnSubscriptionUpdate(id, nodeIDs)
	}

	// 完成任务
	if task != nil {
		s.taskManager.CompleteTask(task.ID, "订阅更新成功", map[string]interface{}{
			"subscription_id": id,
			"node_count":      len(nodeIDs),
		})
	}

	// 自动应用配置
	if err := s.autoApplyConfig(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "刷新成功，但自动应用配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "刷新成功"})
}

func (s *Server) refreshAllSubscriptions(c *gin.Context) {
	// 创建任务记录
	var task *models.Task
	if s.taskManager != nil {
		task, _, _ = s.taskManager.CreateTask(models.TaskTypeSubUpdate, "更新全部订阅", models.TaskTriggerManual, 0)
		s.taskManager.StartTask(task.ID)
	}

	if err := s.subService.RefreshAll(); err != nil {
		if task != nil {
			s.taskManager.FailTask(task.ID, err.Error())
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 同步节点到 SQLite（用于测速模块）
	nodeIDs := s.syncNodesToSQLiteAndGetIDs()

	// 触发订阅更新事件
	if s.eventTrigger != nil {
		s.eventTrigger.OnSubscriptionUpdate("all", nodeIDs)
	}

	// 完成任务
	if task != nil {
		s.taskManager.CompleteTask(task.ID, "全部订阅更新成功", map[string]interface{}{
			"node_count": len(nodeIDs),
		})
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

// ==================== 设置 API ====================

func (s *Server) getSettings(c *gin.Context) {
	settings := s.cloneSettings()
	settings.WebPort = s.port
	settings.AdminPasswordHash = ""
	c.JSON(http.StatusOK, gin.H{"data": settings})
}

func (s *Server) updateSettings(c *gin.Context) {
	var settings storage.Settings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	currentSettings := s.store.GetSettings()
	if currentSettings != nil {
		settings.AdminPasswordHash = currentSettings.AdminPasswordHash
		settings.AuthBootstrappedAt = currentSettings.AuthBootstrappedAt
		if settings.SessionTTLMinutes <= 0 {
			settings.SessionTTLMinutes = currentSettings.SessionTTLMinutes
		}
	}

	if err := s.store.UpdateSettings(&settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 更新进程管理器的配置路径（sing-box 路径是固定的，无需更新）
	s.processManager.SetConfigPath(s.resolvePath(settings.ConfigPath))

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

func (s *Server) buildAndSaveCurrentConfig() error {
	settings := s.store.GetSettings()

	configJSON, err := s.buildConfig()
	if err != nil {
		return err
	}

	return s.saveConfigFile(s.resolvePath(settings.ConfigPath), configJSON)
}

func rebuildConfigAndRestart(build func() error, restart func() error) error {
	if err := build(); err != nil {
		return err
	}

	return restart()
}

func (s *Server) buildConfig() (string, error) {
	settings := s.store.GetSettings()
	if settings.ClashUIEnabled {
		if err := zashboard.EnsureEmbeddedUI(s.store.GetDataDir(), settings.ClashUIPath); err != nil {
			return "", fmt.Errorf("准备内置 zashboard 资源失败: %w", err)
		}
	}

	nodes := s.store.GetAllNodes()
	filters := s.store.GetFilters()
	inboundPorts := s.store.GetInboundPorts()
	proxyChains := s.store.GetProxyChains()

	b := builder.NewConfigBuilder(settings, nodes, filters, inboundPorts, proxyChains)
	b.SetDataDir(s.store.GetDataDir()) // 设置数据目录用于生成绝对路径
	return b.BuildJSON()
}

func (s *Server) saveConfigFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
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

	// 重新创建 JSON 依赖的服务
	s.chainSyncSvc = service.NewChainSyncService(s.store)
	s.subService = service.NewSubscriptionService(s.store, s.chainSyncSvc)
	s.healthCheckSvc = service.NewHealthCheckService(s.store)

	// Rebind 所有 DB 依赖的 service 和 handler（不替换对象，保持 Gin 路由引用有效）
	if s.dbStore != nil {
		// 停止调度器
		if s.unifiedScheduler != nil {
			s.unifiedScheduler.Stop()
		}

		// Rebind service 层
		s.taskManager.Rebind(s.dbStore)
		s.tagEngine.Rebind(s.dbStore)
		s.speedTestExecutor.Rebind(s.dbStore)
		s.eventTrigger.Rebind(s.dbStore)
		s.unifiedScheduler.Rebind(s.dbStore)

		// Rebind handler 层
		s.tagHandler.Rebind(s.dbStore, s.tagEngine)
		s.taskHandler.Rebind(s.dbStore)
		s.speedTestHandler.Rebind(s.dbStore)

		// 重启调度器并重新注册调度条目
		s.unifiedScheduler.Start()
		s.initScheduleEntries()
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

	if err := s.buildAndSaveCurrentConfig(); err != nil {
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
	if err := s.buildAndSaveCurrentConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

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
	if err := rebuildConfigAndRestart(s.buildAndSaveCurrentConfig, s.processManager.Restart); err != nil {
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

	if err := rebuildConfigAndRestart(s.buildAndSaveCurrentConfig, s.launchdManager.Restart); err != nil {
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
			"mode":        s.systemdManager.GetMode(),
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

	logsDir := s.baseDir + "/logs"

	config := daemon.SystemdConfig{
		SbmPath:    s.sbmPath,
		DataDir:    s.baseDir,
		Port:       strconv.Itoa(s.port),
		LogPath:    logsDir,
		WorkingDir: s.baseDir,
		HomeDir:    homeDir,
		RunAtLoad:  true,
		KeepAlive:  true,
	}

	if err := s.systemdManager.Install(config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := s.systemdManager.Start(); err != nil {
		mode := s.systemdManager.GetMode()
		var startCmd string
		if mode == "user" {
			startCmd = "systemctl --user start singbox-manager"
		} else {
			startCmd = "systemctl start singbox-manager"
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "服务已安装，但启动失败: " + err.Error() + "。请执行 " + startCmd,
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

	if err := rebuildConfigAndRestart(s.buildAndSaveCurrentConfig, s.systemdManager.Restart); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "服务已重启"})
}

// ==================== 统一守护进程 API ====================

// getDaemonMode 获取守护进程运行模式
func (s *Server) getDaemonMode() string {
	switch runtime.GOOS {
	case "darwin":
		return "user" // launchd 总是用户级
	case "linux":
		if s.systemdManager != nil {
			return s.systemdManager.GetMode()
		}
	}
	return "unknown"
}

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
			"mode":       s.getDaemonMode(),
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

	logsDir := s.baseDir + "/logs"

	switch runtime.GOOS {
	case "darwin":
		if s.launchdManager == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "当前系统不支持守护进程服务"})
			return
		}
		config := daemon.LaunchdConfig{
			SbmPath:    s.sbmPath,
			DataDir:    s.baseDir,
			Port:       strconv.Itoa(s.port),
			LogPath:    logsDir,
			WorkingDir: s.baseDir,
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
			DataDir:    s.baseDir,
			Port:       strconv.Itoa(s.port),
			LogPath:    logsDir,
			WorkingDir: s.baseDir,
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
	var restart func() error

	switch runtime.GOOS {
	case "darwin":
		if s.launchdManager == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "当前系统不支持守护进程服务"})
			return
		}
		restart = s.launchdManager.Restart
	case "linux":
		if s.systemdManager == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "当前系统不支持守护进程服务"})
			return
		}
		restart = s.systemdManager.Restart
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前系统不支持守护进程服务"})
		return
	}

	if err := rebuildConfigAndRestart(s.buildAndSaveCurrentConfig, restart); err != nil {
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
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "0"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	nodes, total := s.store.GetNodesByCountry(code, limit, offset)
	c.JSON(http.StatusOK, gin.H{"data": nodes, "total": total})
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

// testUnsavedNodeDelay 测试未保存节点的延迟 (使用 mihomo URL 测试)
func (s *Server) testUnsavedNodeDelay(c *gin.Context) {
	var node models.Node
	if err := c.ShouldBindJSON(&node); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	if node.Server == "" || node.ServerPort == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "server 和 server_port 为必填"})
		return
	}

	if node.Tag == "" {
		node.Tag = "test-unsaved"
	}

	defaultProfile := &models.SpeedTestProfile{
		LatencyURL:       "https://cp.cloudflare.com/generate_204",
		Timeout:          7,
		IncludeHandshake: false,
	}
	if s.dbStore != nil {
		if p, err := s.dbStore.GetDefaultSpeedTestProfile(); err == nil {
			defaultProfile = p
		}
	}

	tester := speedtest.NewTester(defaultProfile)
	result := tester.TestDelay(&node)

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"delay":  result.Delay,
		"status": result.Status,
		"error":  result.Error,
	}})
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

	// 获取默认测速策略，保持与批量刷新一致
	defaultProfile, err := s.dbStore.GetDefaultSpeedTestProfile()
	if err != nil {
		// 回退到硬编码配置
		defaultProfile = &models.SpeedTestProfile{
			LatencyURL:       "https://cp.cloudflare.com/generate_204",
			Timeout:          7,
			IncludeHandshake: false,
		}
	}
	tester := speedtest.NewTester(defaultProfile)
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

	// 检测端口冲突
	for _, existing := range s.store.GetInboundPorts() {
		if existing.Listen == port.Listen && existing.Port == port.Port {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("端口 %s:%d 已被「%s」占用", port.Listen, port.Port, existing.Name)})
			return
		}
	}

	// 检测系统端口是否可用
	if err := checkPortAvailable(port.Listen, port.Port); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

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

	// 检测端口冲突（排除自身）
	oldPort := s.store.GetInboundPort(id)
	for _, existing := range s.store.GetInboundPorts() {
		if existing.ID != id && existing.Listen == port.Listen && existing.Port == port.Port {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("端口 %s:%d 已被「%s」占用", port.Listen, port.Port, existing.Name)})
			return
		}
	}

	// 端口变更时检测系统端口是否可用
	portChanged := oldPort == nil || oldPort.Listen != port.Listen || oldPort.Port != port.Port
	if portChanged {
		if err := checkPortAvailable(port.Listen, port.Port); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

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

	// 获取旧链路数据，用于检测改名和停用
	oldChain := s.store.GetProxyChain(id)
	if oldChain == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "代理链路不存在"})
		return
	}

	var chain storage.ProxyChain
	if err := c.ShouldBindJSON(&chain); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	chain.ID = id

	// 自动生成 ChainNodes（从 Nodes 生成副本信息）
	chain.ChainNodes = s.generateChainNodes(chain.Name, chain.Nodes)

	// 检测级联操作
	var cascadeMessages []string

	// 改名：更新关联入站端口的 Outbound 引用
	if oldChain.Name != chain.Name {
		renamed, err := s.updateInboundPortsOutbound(oldChain.Name, chain.Name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if len(renamed) > 0 {
			cascadeMessages = append(cascadeMessages, fmt.Sprintf("已更新 %d 个关联入站端口", len(renamed)))
		}
	}

	// 停用：停用关联入站端口
	if oldChain.Enabled && !chain.Enabled {
		outboundName := chain.Name
		if oldChain.Name != chain.Name {
			outboundName = chain.Name // 如果同时改名，用新名字（已更新过了）
		}
		disabled, err := s.disableInboundPortsForOutbound(outboundName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if len(disabled) > 0 {
			cascadeMessages = append(cascadeMessages, fmt.Sprintf("已停用 %d 个关联入站端口", len(disabled)))
		}
	}

	if err := s.store.UpdateProxyChain(chain); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 自动应用配置
	if err := s.autoApplyConfig(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "更新成功，但自动应用配置失败: " + err.Error()})
		return
	}

	message := "更新成功"
	if len(cascadeMessages) > 0 {
		message = "更新成功，" + strings.Join(cascadeMessages, "，")
	}
	c.JSON(http.StatusOK, gin.H{"message": message})
}

func (s *Server) deleteProxyChain(c *gin.Context) {
	id := c.Param("id")
	chain := s.store.GetProxyChain(id)
	if chain == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "代理链路不存在"})
		return
	}

	disabledPorts, err := s.disableInboundPortsForOutbound(chain.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := s.store.DeleteProxyChain(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 自动应用配置
	if err := s.autoApplyConfig(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "删除成功，但自动应用配置失败: " + err.Error()})
		return
	}

	message := "删除成功"
	if len(disabledPorts) > 0 {
		message = fmt.Sprintf("删除成功，已停用 %d 个关联入站", len(disabledPorts))
	}

	c.JSON(http.StatusOK, gin.H{"message": message})
}

func (s *Server) updateInboundPortsOutbound(oldOutbound, newOutbound string) ([]storage.InboundPort, error) {
	ports := s.store.GetInboundPorts()
	updated := make([]storage.InboundPort, 0)

	for _, port := range ports {
		if port.Outbound != oldOutbound {
			continue
		}

		port.Outbound = newOutbound
		if err := s.store.UpdateInboundPort(port); err != nil {
			return nil, fmt.Errorf("更新入站端口 %s 的出站引用失败: %w", port.Name, err)
		}
		updated = append(updated, port)
	}

	return updated, nil
}

func (s *Server) disableInboundPortsForOutbound(outbound string) ([]storage.InboundPort, error) {
	ports := s.store.GetInboundPorts()
	disabled := make([]storage.InboundPort, 0)

	for _, port := range ports {
		if !port.Enabled || port.Outbound != outbound {
			continue
		}

		port.Enabled = false
		if err := s.store.UpdateInboundPort(port); err != nil {
			return nil, fmt.Errorf("停用入站端口 %s 失败: %w", port.Name, err)
		}
		disabled = append(disabled, port)
	}

	return disabled, nil
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
		if storage.IsChainCountryNodeTag(tag) {
			source = storage.GetChainCountryNodeSource(storage.ParseChainCountryNodeCode(tag))
		} else if exists {
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

func (s *Server) getAllChainSpeed(c *gin.Context) {
	results := s.healthCheckSvc.GetAllCachedSpeedResults()
	c.JSON(http.StatusOK, gin.H{"data": results})
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

// syncNodesToSQLiteAndGetIDs 同步节点并返回所有节点 ID
func (s *Server) syncNodesToSQLiteAndGetIDs() []uint {
	if err := s.syncNodesToSQLite(); err != nil {
		logger.Warn("同步节点到 SQLite 失败: %v", err)
		return nil
	}

	// 获取所有启用的节点 ID
	if s.dbStore == nil {
		return nil
	}

	nodes, err := s.dbStore.GetNodes()
	if err != nil {
		return nil
	}

	var ids []uint
	for _, node := range nodes {
		if node.Enabled {
			ids = append(ids, node.ID)
		}
	}
	return ids
}

func checkPortAvailable(listen string, port int) error {
	addr := ":" + strconv.Itoa(port)
	if listen != "" && listen != "0.0.0.0" && listen != "::" {
		addr = net.JoinHostPort(listen, strconv.Itoa(port))
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("端口 %s:%d 已被系统其他进程占用", listen, port)
	}
	_ = ln.Close()
	return nil
}
