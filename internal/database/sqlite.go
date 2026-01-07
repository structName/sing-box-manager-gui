package database

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/glebarez/sqlite"
	"github.com/xiaobei/singbox-manager/internal/database/models"
	"github.com/xiaobei/singbox-manager/internal/logger"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var (
	db   *gorm.DB
	once sync.Once
	mu   sync.Mutex
)

// InitDB 初始化数据库（兼容旧接口，使用 once 保证单例）
func InitDB(dataDir string) (*gorm.DB, error) {
	var err error
	once.Do(func() {
		db, err = initDBInternal(dataDir)
	})
	return db, err
}

// InitDBWithPath 初始化指定路径的数据库（支持多 Profile）
func InitDBWithPath(dataDir string) (*gorm.DB, error) {
	mu.Lock()
	defer mu.Unlock()
	return initDBInternal(dataDir)
}

// ResetDB 重置数据库单例（用于 Profile 切换）
func ResetDB() {
	mu.Lock()
	defer mu.Unlock()
	once = sync.Once{}
	db = nil
}

// initDBInternal 内部初始化逻辑
func initDBInternal(dataDir string) (*gorm.DB, error) {
	// 确保数据目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	dbPath := filepath.Join(dataDir, "sbm.db")
	logger.Info("初始化 SQLite 数据库: %s", dbPath)

	// 配置 GORM
	config := &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	}

	// 打开数据库
	newDB, err := gorm.Open(sqlite.Open(dbPath+"?_journal_mode=WAL&_busy_timeout=5000"), config)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// 自动迁移表结构
	if err := autoMigrateDB(newDB); err != nil {
		return nil, fmt.Errorf("迁移数据库失败: %w", err)
	}

	// 初始化默认数据
	if err := initDefaultDataDB(newDB); err != nil {
		return nil, fmt.Errorf("初始化默认数据失败: %w", err)
	}

	logger.Info("数据库初始化完成")
	db = newDB
	return newDB, nil
}

// GetDB 获取数据库实例
func GetDB() *gorm.DB {
	return db
}

// autoMigrateDB 自动迁移表结构
func autoMigrateDB(database *gorm.DB) error {
	return database.AutoMigrate(
		// 订阅和节点
		&models.Subscription{},
		&models.Node{},

		// 代理链路
		&models.ProxyChain{},
		&models.ProxyChainNode{},

		// 规则
		&models.Rule{},
		&models.RuleGroup{},
		&models.Filter{},
		&models.HostEntry{},
		&models.InboundPort{},

		// 标签
		&models.Tag{},
		&models.TagRule{},

		// 测速
		&models.SpeedTestProfile{},
		&models.SpeedTestTask{},
		&models.SpeedTestHistory{},

		// 设置
		&models.Setting{},
		&models.Profile{},
	)
}

// initDefaultDataDB 初始化默认数据
func initDefaultDataDB(database *gorm.DB) error {
	// 初始化默认设置
	for key, value := range models.DefaultSettings {
		var count int64
		database.Model(&models.Setting{}).Where("key = ?", key).Count(&count)
		if count == 0 {
			if err := database.Create(&models.Setting{Key: key, Value: value}).Error; err != nil {
				return err
			}
		}
	}

	// 初始化默认规则组
	for _, rg := range models.DefaultRuleGroups {
		var count int64
		database.Model(&models.RuleGroup{}).Where("id = ?", rg.ID).Count(&count)
		if count == 0 {
			if err := database.Create(&rg).Error; err != nil {
				return err
			}
		}
	}

	// 初始化默认测速策略
	var profileCount int64
	database.Model(&models.SpeedTestProfile{}).Count(&profileCount)
	if profileCount == 0 {
		defaultProfile := &models.SpeedTestProfile{
			Name:               "默认策略",
			Enabled:            true,
			IsDefault:          true,
			Mode:               "delay",
			LatencyURL:         "https://cp.cloudflare.com/generate_204",
			SpeedURL:           "https://speed.cloudflare.com/__down?bytes=5000000",
			Timeout:            7,
			LatencyConcurrency: 50,
			SpeedConcurrency:   5,
			IncludeHandshake:   false,
			DetectCountry:      false,
			LandingIPURL:       "https://api.ipify.org",
			SpeedRecordMode:    "average",
			PeakSampleInterval: 100,
		}
		if err := database.Create(defaultProfile).Error; err != nil {
			return err
		}
	}

	return nil
}

// CloseDB 关闭数据库连接
func CloseDB() error {
	if db != nil {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}

// CreateIndexes 创建索引（可选，AutoMigrate 已处理基本索引）
func CreateIndexes() error {
	// nodes 表索引
	db.Exec("CREATE INDEX IF NOT EXISTS idx_nodes_source ON nodes(source)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_nodes_country ON nodes(country)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_nodes_type ON nodes(type)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_nodes_delay ON nodes(delay)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_nodes_speed ON nodes(speed)")

	// proxy_chain_nodes 表索引
	db.Exec("CREATE INDEX IF NOT EXISTS idx_chain_nodes_chain ON proxy_chain_nodes(chain_id)")

	// node_tags 表索引
	db.Exec("CREATE INDEX IF NOT EXISTS idx_node_tags_node ON node_tags(node_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_node_tags_tag ON node_tags(tag_id)")

	// speed_test_tasks 表索引
	db.Exec("CREATE INDEX IF NOT EXISTS idx_tasks_status ON speed_test_tasks(status)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_tasks_started ON speed_test_tasks(started_at)")

	// speed_test_history 表索引
	db.Exec("CREATE INDEX IF NOT EXISTS idx_history_node ON speed_test_history(node_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_history_tested ON speed_test_history(tested_at)")

	return nil
}
