package service

import (
	"os"
	"sync"
	"time"

	"github.com/xiaobei/singbox-manager/internal/logger"
)

// ConfigWatcher 配置文件监控器
type ConfigWatcher struct {
	configPath    string
	lastModTime   time.Time
	autoRestart   bool
	debounceDelay time.Duration

	restartFunc func() error
	stopCh      chan struct{}
	mu          sync.Mutex
	running     bool
	pending     bool // 是否有待处理的重启
}

// NewConfigWatcher 创建配置监控器
func NewConfigWatcher(configPath string, restartFunc func() error) *ConfigWatcher {
	return &ConfigWatcher{
		configPath:    configPath,
		autoRestart:   false,
		debounceDelay: 3 * time.Second,
		restartFunc:   restartFunc,
		stopCh:        make(chan struct{}),
	}
}

// SetAutoRestart 设置是否自动重启
func (w *ConfigWatcher) SetAutoRestart(enabled bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.autoRestart = enabled
}

// SetDebounceDelay 设置防抖延迟
func (w *ConfigWatcher) SetDebounceDelay(delay time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.debounceDelay = delay
}

// Start 启动监控
func (w *ConfigWatcher) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.stopCh = make(chan struct{})
	w.mu.Unlock()

	// 记录初始修改时间
	if info, err := os.Stat(w.configPath); err == nil {
		w.lastModTime = info.ModTime()
	}

	go w.watch()
	logger.Info("配置监控器已启动: %s", w.configPath)
}

// Stop 停止监控
func (w *ConfigWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	close(w.stopCh)
	w.running = false
	logger.Info("配置监控器已停止")
}

// NotifyConfigChanged 通知配置已变更（由外部调用）
func (w *ConfigWatcher) NotifyConfigChanged() {
	w.mu.Lock()
	if !w.autoRestart {
		w.mu.Unlock()
		return
	}
	w.pending = true
	w.mu.Unlock()
}

// TriggerRestart 手动触发重启
func (w *ConfigWatcher) TriggerRestart() error {
	if w.restartFunc == nil {
		return nil
	}
	return w.restartFunc()
}

// watch 监控循环
func (w *ConfigWatcher) watch() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var debounceTimer *time.Timer

	for {
		select {
		case <-w.stopCh:
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return

		case <-ticker.C:
			w.mu.Lock()
			pending := w.pending
			autoRestart := w.autoRestart
			delay := w.debounceDelay
			w.mu.Unlock()

			if !autoRestart {
				continue
			}

			// 检查文件变更
			changed := w.checkFileChanged()

			if (changed || pending) && debounceTimer == nil {
				w.mu.Lock()
				w.pending = false
				w.mu.Unlock()

				debounceTimer = time.AfterFunc(delay, func() {
					w.doRestart()
					debounceTimer = nil
				})
			}
		}
	}
}

// checkFileChanged 检查文件是否变更
func (w *ConfigWatcher) checkFileChanged() bool {
	info, err := os.Stat(w.configPath)
	if err != nil {
		return false
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if info.ModTime().After(w.lastModTime) {
		w.lastModTime = info.ModTime()
		return true
	}
	return false
}

// doRestart 执行重启
func (w *ConfigWatcher) doRestart() {
	if w.restartFunc == nil {
		return
	}

	logger.Info("配置变更，正在重启 sing-box...")
	if err := w.restartFunc(); err != nil {
		logger.Error("重启 sing-box 失败: %v", err)
	} else {
		logger.Info("sing-box 已重启")
	}
}

// IsRunning 检查是否运行中
func (w *ConfigWatcher) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}
