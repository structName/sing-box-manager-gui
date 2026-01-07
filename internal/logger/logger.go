package logger

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// 默认最大日志文件大小 10MB
	DefaultMaxSize = 10 * 1024 * 1024
	// 默认保留的日志文件数量
	DefaultMaxBackups = 3
)

// Logger 日志管理器
type Logger struct {
	mu          sync.Mutex
	file        *os.File
	filePath    string // 基础路径，如 logs/singbox.log
	currentFile string // 当前实际文件路径，如 logs/singbox-2026-01-02.log
	maxSize     int64
	maxBackups  int
	currentSize int64
	currentDate string // 当前日期，用于检测日期变化
	logger      *log.Logger
	prefix      string
}

// LogManager 全局日志管理
type LogManager struct {
	dataDir       string
	appLogger     *Logger
	singboxLogger *Logger
}

var (
	// 全局日志管理器实例
	manager *LogManager
	once    sync.Once
)

// NewLogger 创建新的日志记录器
func NewLogger(filePath string, prefix string) (*Logger, error) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	l := &Logger{
		filePath:   filePath,
		maxSize:    DefaultMaxSize,
		maxBackups: DefaultMaxBackups,
		prefix:     prefix,
	}

	if err := l.openFile(); err != nil {
		return nil, err
	}

	return l, nil
}

// getDailyFilePath 获取按日期的文件路径
func (l *Logger) getDailyFilePath(date string) string {
	ext := filepath.Ext(l.filePath)
	base := strings.TrimSuffix(l.filePath, ext)
	return fmt.Sprintf("%s-%s%s", base, date, ext)
}

// openFile 打开或创建日志文件（按日期）
func (l *Logger) openFile() error {
	today := time.Now().Format("2006-01-02")
	dailyPath := l.getDailyFilePath(today)

	file, err := os.OpenFile(dailyPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	l.file = file
	l.currentFile = dailyPath
	l.currentDate = today
	l.currentSize = info.Size()
	l.logger = log.New(file, l.prefix, log.LstdFlags)

	// 清理过期的日志文件
	go l.cleanOldLogs()

	return nil
}

// checkDateChange 检查日期是否变化，如果变化则切换到新文件（调用前必须持有锁）
func (l *Logger) checkDateChange() error {
	today := time.Now().Format("2006-01-02")
	if today == l.currentDate {
		return nil
	}

	// 日期变化，切换到新文件
	if l.file != nil {
		l.file.Close()
	}

	return l.openFileUnlocked()
}

// openFileUnlocked 打开或创建日志文件（不加锁版本，供内部使用）
func (l *Logger) openFileUnlocked() error {
	today := time.Now().Format("2006-01-02")
	dailyPath := l.getDailyFilePath(today)

	file, err := os.OpenFile(dailyPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	l.file = file
	l.currentFile = dailyPath
	l.currentDate = today
	l.currentSize = info.Size()
	l.logger = log.New(file, l.prefix, log.LstdFlags)

	// 清理过期的日志文件
	go l.cleanOldLogs()

	return nil
}

// cleanOldLogs 清理过期的日志文件
func (l *Logger) cleanOldLogs() {
	dir := filepath.Dir(l.filePath)
	ext := filepath.Ext(l.filePath)
	base := filepath.Base(strings.TrimSuffix(l.filePath, ext))

	files, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	// 收集匹配的日志文件
	var logFiles []string
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		// 匹配格式: base-YYYY-MM-DD.ext
		if strings.HasPrefix(name, base+"-") && strings.HasSuffix(name, ext) {
			logFiles = append(logFiles, filepath.Join(dir, name))
		}
	}

	// 按修改时间排序，保留最近的 maxBackups 个文件
	if len(logFiles) <= l.maxBackups {
		return
	}

	// 获取文件信息并按时间排序
	type fileInfo struct {
		path    string
		modTime time.Time
	}
	var infos []fileInfo
	for _, path := range logFiles {
		if stat, err := os.Stat(path); err == nil {
			infos = append(infos, fileInfo{path: path, modTime: stat.ModTime()})
		}
	}

	// 按时间降序排序
	for i := 0; i < len(infos)-1; i++ {
		for j := i + 1; j < len(infos); j++ {
			if infos[j].modTime.After(infos[i].modTime) {
				infos[i], infos[j] = infos[j], infos[i]
			}
		}
	}

	// 删除超出保留数量的旧文件
	for i := l.maxBackups; i < len(infos); i++ {
		os.Remove(infos[i].path)
	}
}

// rotate 轮转日志文件（当单日文件过大时）
func (l *Logger) rotate() error {
	if l.file != nil {
		l.file.Close()
	}

	// 当天文件重命名添加序号
	base := strings.TrimSuffix(l.currentFile, filepath.Ext(l.currentFile))
	ext := filepath.Ext(l.currentFile)

	// 删除最旧的备份
	oldestBackup := fmt.Sprintf("%s.%d%s", base, l.maxBackups, ext)
	os.Remove(oldestBackup)

	// 移动现有备份
	for i := l.maxBackups - 1; i >= 1; i-- {
		oldPath := fmt.Sprintf("%s.%d%s", base, i, ext)
		newPath := fmt.Sprintf("%s.%d%s", base, i+1, ext)
		os.Rename(oldPath, newPath)
	}

	// 移动当前日志到 .1
	os.Rename(l.currentFile, fmt.Sprintf("%s.1%s", base, ext))

	// 创建新文件
	file, err := os.OpenFile(l.currentFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("创建新日志文件失败: %w", err)
	}

	l.file = file
	l.currentSize = 0
	l.logger = log.New(file, l.prefix, log.LstdFlags)

	return nil
}

// Write 实现 io.Writer 接口
func (l *Logger) Write(p []byte) (n int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 检查日期变化
	if err := l.checkDateChange(); err != nil {
		return 0, err
	}

	// 检查是否需要轮转
	if l.currentSize+int64(len(p)) > l.maxSize {
		if err := l.rotate(); err != nil {
			return 0, err
		}
	}

	n, err = l.file.Write(p)
	l.currentSize += int64(n)
	return
}

// Printf 格式化日志输出
func (l *Logger) Printf(format string, v ...interface{}) {
	timestamp := time.Now().Format("2006/01/02 15:04:05")
	msg := fmt.Sprintf(format, v...)
	line := fmt.Sprintf("%s %s%s\n", timestamp, l.prefix, msg)

	// 写入文件
	l.Write([]byte(line))

	// 同时输出到控制台
	fmt.Print(line)
}

// Println 输出一行日志
func (l *Logger) Println(v ...interface{}) {
	timestamp := time.Now().Format("2006/01/02 15:04:05")
	msg := fmt.Sprint(v...)
	line := fmt.Sprintf("%s %s%s\n", timestamp, l.prefix, msg)

	// 写入文件
	l.Write([]byte(line))

	// 同时输出到控制台
	fmt.Print(line)
}

// WriteRaw 写入原始日志行（不添加时间戳，用于 sing-box 输出）
// 只写入文件，不输出到控制台，避免和程序日志混在一起
func (l *Logger) WriteRaw(line string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 检查日期变化
	if err := l.checkDateChange(); err != nil {
		fmt.Fprintf(os.Stderr, "日期切换失败: %v\n", err)
		return
	}

	data := line + "\n"

	// 检查是否需要轮转
	if l.currentSize+int64(len(data)) > l.maxSize {
		if err := l.rotate(); err != nil {
			fmt.Fprintf(os.Stderr, "日志轮转失败: %v\n", err)
			return
		}
	}

	n, _ := l.file.Write([]byte(data))
	l.currentSize += int64(n)
}

// Close 关闭日志文件
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// ReadLastLines 读取最后 n 行日志（从当天的日志文件）
func (l *Logger) ReadLastLines(n int) ([]string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 同步文件
	if l.file != nil {
		l.file.Sync()
	}

	// 读取当天的日志文件
	file, err := os.Open(l.currentFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("打开日志文件失败: %w", err)
	}
	defer file.Close()

	// 使用 ring buffer 来存储最后 n 行
	lines := make([]string, 0, n)
	scanner := bufio.NewScanner(file)

	// 增加 scanner 缓冲区大小以处理长行
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > n {
			lines = lines[1:]
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取日志失败: %w", err)
	}

	return lines, nil
}

// GetFilePath 获取日志文件路径
func (l *Logger) GetFilePath() string {
	return l.filePath
}

// InitLogManager 初始化全局日志管理器
func InitLogManager(dataDir string) error {
	var initErr error
	once.Do(func() {
		logsDir := filepath.Join(dataDir, "logs")

		appLogger, err := NewLogger(filepath.Join(logsDir, "sbm.log"), "[SBM] ")
		if err != nil {
			initErr = fmt.Errorf("初始化应用日志失败: %w", err)
			return
		}

		singboxLogger, err := NewLogger(filepath.Join(logsDir, "singbox.log"), "")
		if err != nil {
			initErr = fmt.Errorf("初始化 sing-box 日志失败: %w", err)
			return
		}

		manager = &LogManager{
			dataDir:       dataDir,
			appLogger:     appLogger,
			singboxLogger: singboxLogger,
		}
	})

	return initErr
}

// GetLogManager 获取全局日志管理器
func GetLogManager() *LogManager {
	return manager
}

// AppLogger 获取应用日志记录器
func (m *LogManager) AppLogger() *Logger {
	return m.appLogger
}

// SingboxLogger 获取 sing-box 日志记录器
func (m *LogManager) SingboxLogger() *Logger {
	return m.singboxLogger
}

// Printf 应用日志快捷方法
func Printf(format string, v ...interface{}) {
	if manager != nil && manager.appLogger != nil {
		manager.appLogger.Printf(format, v...)
	} else {
		log.Printf(format, v...)
	}
}

// Println 应用日志快捷方法
func Println(v ...interface{}) {
	if manager != nil && manager.appLogger != nil {
		manager.appLogger.Println(v...)
	} else {
		log.Println(v...)
	}
}

// Info 信息日志
func Info(format string, v ...interface{}) {
	Printf("[INFO] "+format, v...)
}

// Warn 警告日志
func Warn(format string, v ...interface{}) {
	Printf("[WARN] "+format, v...)
}

// Error 错误日志
func Error(format string, v ...interface{}) {
	Printf("[ERROR] "+format, v...)
}

// Debug 调试日志
func Debug(format string, v ...interface{}) {
	Printf("[DEBUG] "+format, v...)
}

// SingboxWriter 返回一个可以用于 sing-box 输出的 Writer
type SingboxWriter struct {
	logger   *Logger
	memLogs  *[]string
	memMu    *sync.RWMutex
	maxLogs  int
	callback func(string) // 可选的回调函数
}

// NewSingboxWriter 创建 sing-box 输出写入器
func NewSingboxWriter(logger *Logger, memLogs *[]string, memMu *sync.RWMutex, maxLogs int) *SingboxWriter {
	return &SingboxWriter{
		logger:  logger,
		memLogs: memLogs,
		memMu:   memMu,
		maxLogs: maxLogs,
	}
}

// Write 实现 io.Writer 接口
func (w *SingboxWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

// WriteLine 写入一行日志
func (w *SingboxWriter) WriteLine(line string) {
	// 写入文件
	if w.logger != nil {
		w.logger.WriteRaw(line)
	}

	// 写入内存
	if w.memLogs != nil && w.memMu != nil {
		w.memMu.Lock()
		*w.memLogs = append(*w.memLogs, line)
		if len(*w.memLogs) > w.maxLogs {
			*w.memLogs = (*w.memLogs)[len(*w.memLogs)-w.maxLogs:]
		}
		w.memMu.Unlock()
	}
}

// ReadAppLogs 读取应用日志
func ReadAppLogs(lines int) ([]string, error) {
	if manager == nil || manager.appLogger == nil {
		return []string{}, nil
	}
	return manager.appLogger.ReadLastLines(lines)
}

// ReadSingboxLogs 读取 sing-box 日志
func ReadSingboxLogs(lines int) ([]string, error) {
	if manager == nil || manager.singboxLogger == nil {
		return []string{}, nil
	}
	return manager.singboxLogger.ReadLastLines(lines)
}

// ClearSingboxLogs 清空 sing-box 日志文件
func ClearSingboxLogs() error {
	if manager == nil || manager.singboxLogger == nil {
		return nil
	}
	return manager.singboxLogger.Clear()
}

// Clear 清空日志文件（当天的日志文件）
func (l *Logger) Clear() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 关闭当前文件
	if l.file != nil {
		l.file.Close()
	}

	// 截断当天的日志文件
	file, err := os.OpenFile(l.currentFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("清空日志文件失败: %w", err)
	}

	l.file = file
	l.currentSize = 0
	l.logger = log.New(file, l.prefix, log.LstdFlags)

	return nil
}

// MultiWriter 同时写入多个目标
type MultiWriter struct {
	writers []io.Writer
}

// NewMultiWriter 创建多目标写入器
func NewMultiWriter(writers ...io.Writer) *MultiWriter {
	return &MultiWriter{writers: writers}
}

// Write 写入所有目标
func (mw *MultiWriter) Write(p []byte) (n int, err error) {
	for _, w := range mw.writers {
		n, err = w.Write(p)
		if err != nil {
			return
		}
	}
	return len(p), nil
}
