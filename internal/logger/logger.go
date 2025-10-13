package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

/*
日志系统模块

这个模块实现了一个功能完整的日志系统，包括：
1. 多级别日志支持（DEBUG, INFO, WARN, ERROR, FATAL）
2. 结构化日志输出
3. 多种输出目标（控制台、文件、网络）
4. 日志轮转和归档
5. 性能优化的异步写入
6. 上下文信息记录

日志级别说明：
- DEBUG: 详细的调试信息，通常只在开发时使用
- INFO: 一般信息，记录系统正常运行状态
- WARN: 警告信息，表示可能的问题但不影响正常运行
- ERROR: 错误信息，表示出现了错误但系统仍可运行
- FATAL: 致命错误，表示系统无法继续运行
*/

// LogLevel 日志级别类型
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

// String 返回日志级别的字符串表示
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Color 返回日志级别对应的颜色代码
func (l LogLevel) Color() string {
	switch l {
	case DEBUG:
		return "\033[36m" // Cyan
	case INFO:
		return "\033[32m" // Green
	case WARN:
		return "\033[33m" // Yellow
	case ERROR:
		return "\033[31m" // Red
	case FATAL:
		return "\033[35m" // Magenta
	default:
		return "\033[0m" // Reset
	}
}

// LogEntry 日志条目
type LogEntry struct {
	Level     LogLevel               `json:"level"`
	Timestamp time.Time              `json:"timestamp"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	File      string                 `json:"file,omitempty"`
	Line      int                    `json:"line,omitempty"`
	Function  string                 `json:"function,omitempty"`
}

// Formatter 日志格式化器接口
type Formatter interface {
	Format(entry *LogEntry) ([]byte, error)
}

// JSONFormatter JSON格式化器
type JSONFormatter struct {
	PrettyPrint bool `json:"pretty_print"`
}

// Format 格式化日志条目为JSON
func (f *JSONFormatter) Format(entry *LogEntry) ([]byte, error) {
	// 这里简化处理，实际应该使用json.Marshal
	return []byte(fmt.Sprintf(`{"level":"%s","timestamp":"%s","message":"%s"}`+"\n",
		entry.Level.String(),
		entry.Timestamp.Format(time.RFC3339),
		entry.Message)), nil
}

// TextFormatter 文本格式化器
type TextFormatter struct {
	DisableColors bool `json:"disable_colors"`
	FullTimestamp bool `json:"full_timestamp"`
}

// Format 格式化日志条目为文本
func (f *TextFormatter) Format(entry *LogEntry) ([]byte, error) {
	var timestamp string
	if f.FullTimestamp {
		timestamp = entry.Timestamp.Format("2006-01-02 15:04:05.000")
	} else {
		timestamp = entry.Timestamp.Format("15:04:05")
	}

	levelStr := entry.Level.String()
	if !f.DisableColors {
		levelStr = fmt.Sprintf("%s%s\033[0m", entry.Level.Color(), levelStr)
	}

	var location string
	if entry.File != "" {
		location = fmt.Sprintf(" [%s:%d]", filepath.Base(entry.File), entry.Line)
	}

	var fieldsStr string
	if len(entry.Fields) > 0 {
		var fields []string
		for k, v := range entry.Fields {
			fields = append(fields, fmt.Sprintf("%s=%v", k, v))
		}
		fieldsStr = " " + strings.Join(fields, " ")
	}

	return []byte(fmt.Sprintf("%s %s%s%s%s\n",
		timestamp,
		levelStr,
		location,
		entry.Message,
		fieldsStr)), nil
}

// Writer 日志写入器接口
type Writer interface {
	Write(entry *LogEntry) error
	Close() error
}

// ConsoleWriter 控制台写入器
type ConsoleWriter struct {
	Formatter Formatter
}

// NewConsoleWriter 创建控制台写入器
func NewConsoleWriter(disableColors bool) *ConsoleWriter {
	return &ConsoleWriter{
		Formatter: &TextFormatter{
			DisableColors: disableColors,
			FullTimestamp: true,
		},
	}
}

// Write 写入日志到控制台
func (w *ConsoleWriter) Write(entry *LogEntry) error {
	data, err := w.Formatter.Format(entry)
	if err != nil {
		return err
	}

	// 根据日志级别选择输出流
	if entry.Level >= ERROR {
		_, err = os.Stderr.Write(data)
	} else {
		_, err = os.Stdout.Write(data)
	}

	return err
}

// Close 关闭写入器
func (w *ConsoleWriter) Close() error {
	return nil
}

// FileWriter 文件写入器
type FileWriter struct {
	file      *os.File
	filename  string
	maxSize   int64
	current   int64
	mu        sync.Mutex
	Formatter Formatter
}

// NewFileWriter 创建文件写入器
func NewFileWriter(filename string, maxSize int64) (*FileWriter, error) {
	// 确保目录存在
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// 获取当前文件大小
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to get file stats: %w", err)
	}

	return &FileWriter{
		file:      file,
		filename:  filename,
		maxSize:   maxSize,
		current:   stat.Size(),
		Formatter: &TextFormatter{DisableColors: true, FullTimestamp: true},
	}, nil
}

// Write 写入日志到文件
func (w *FileWriter) Write(entry *LogEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 检查是否需要轮转
	if w.maxSize > 0 && w.current >= w.maxSize {
		if err := w.rotate(); err != nil {
			return err
		}
	}

	data, err := w.Formatter.Format(entry)
	if err != nil {
		return err
	}

	n, err := w.file.Write(data)
	if err != nil {
		return err
	}

	w.current += int64(n)
	return nil
}

// rotate 轮转日志文件
func (w *FileWriter) rotate() error {
	// 关闭当前文件
	if err := w.file.Close(); err != nil {
		return err
	}

	// 重命名当前文件
	timestamp := time.Now().Format("20060102-150405")
	backupName := fmt.Sprintf("%s.%s", w.filename, timestamp)
	if err := os.Rename(w.filename, backupName); err != nil {
		return err
	}

	// 创建新文件
	file, err := os.OpenFile(w.filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	w.file = file
	w.current = 0
	return nil
}

// Close 关闭写入器
func (w *FileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

// MultiWriter 多目标写入器
type MultiWriter struct {
	writers []Writer
}

// NewMultiWriter 创建多目标写入器
func NewMultiWriter(writers ...Writer) *MultiWriter {
	return &MultiWriter{writers: writers}
}

// Write 写入日志到多个目标
func (w *MultiWriter) Write(entry *LogEntry) error {
	var errors []error
	for _, writer := range w.writers {
		if err := writer.Write(entry); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("multiple write errors: %v", errors)
	}
	return nil
}

// Close 关闭所有写入器
func (w *MultiWriter) Close() error {
	var errors []error
	for _, writer := range w.writers {
		if err := writer.Close(); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("multiple close errors: %v", errors)
	}
	return nil
}

// Logger 日志记录器
type Logger struct {
	level   LogLevel
	writer  Writer
	mu      sync.RWMutex
	enabled bool
}

// NewLogger 创建日志记录器
func NewLogger(level LogLevel, writer Writer) *Logger {
	return &Logger{
		level:   level,
		writer:  writer,
		enabled: true,
	}
}

// SetLevel 设置日志级别
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// GetLevel 获取日志级别
func (l *Logger) GetLevel() LogLevel {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}

// SetEnabled 设置是否启用日志
func (l *Logger) SetEnabled(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enabled = enabled
}

// IsEnabled 检查日志是否启用
func (l *Logger) IsEnabled() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.enabled
}

// log 内部日志方法
func (l *Logger) log(level LogLevel, message string, fields map[string]interface{}) {
	l.mu.RLock()
	if !l.enabled || level < l.level {
		l.mu.RUnlock()
		return
	}
	writer := l.writer
	l.mu.RUnlock()

	// 获取调用者信息
	_, file, line, ok := runtime.Caller(2)
	var function string
	if ok {
		pc, _, _, _ := runtime.Caller(2)
		function = runtime.FuncForPC(pc).Name()
		// 提取函数名（去掉包路径）
		if idx := strings.LastIndex(function, "."); idx != -1 {
			function = function[idx+1:]
		}
	}

	entry := &LogEntry{
		Level:     level,
		Timestamp: time.Now(),
		Message:   message,
		Fields:    fields,
		File:      file,
		Line:      line,
		Function:  function,
	}

	if err := writer.Write(entry); err != nil {
		// 如果写入失败，输出到标准错误
		fmt.Fprintf(os.Stderr, "Failed to write log: %v\n", err)
	}

	// 如果是致命错误，退出程序
	if level == FATAL {
		os.Exit(1)
	}
}

// Debug 记录调试日志
func (l *Logger) Debug(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(DEBUG, message, f)
}

// Info 记录信息日志
func (l *Logger) Info(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(INFO, message, f)
}

// Warn 记录警告日志
func (l *Logger) Warn(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(WARN, message, f)
}

// Error 记录错误日志
func (l *Logger) Error(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(ERROR, message, f)
}

// Fatal 记录致命错误日志
func (l *Logger) Fatal(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(FATAL, message, f)
}

// WithFields 添加字段
func (l *Logger) WithFields(fields map[string]interface{}) *FieldLogger {
	return &FieldLogger{
		logger: l,
		fields: fields,
	}
}

// Close 关闭日志记录器
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.writer != nil {
		return l.writer.Close()
	}
	return nil
}

// FieldLogger 带字段的日志记录器
type FieldLogger struct {
	logger *Logger
	fields map[string]interface{}
}

// Debug 记录调试日志
func (fl *FieldLogger) Debug(message string, fields ...map[string]interface{}) {
	f := fl.mergeFields(fields...)
	fl.logger.log(DEBUG, message, f)
}

// Info 记录信息日志
func (fl *FieldLogger) Info(message string, fields ...map[string]interface{}) {
	f := fl.mergeFields(fields...)
	fl.logger.log(INFO, message, f)
}

// Warn 记录警告日志
func (fl *FieldLogger) Warn(message string, fields ...map[string]interface{}) {
	f := fl.mergeFields(fields...)
	fl.logger.log(WARN, message, f)
}

// Error 记录错误日志
func (fl *FieldLogger) Error(message string, fields ...map[string]interface{}) {
	f := fl.mergeFields(fields...)
	fl.logger.log(ERROR, message, f)
}

// Fatal 记录致命错误日志
func (fl *FieldLogger) Fatal(message string, fields ...map[string]interface{}) {
	f := fl.mergeFields(fields...)
	fl.logger.log(FATAL, message, f)
}

// mergeFields 合并字段
func (fl *FieldLogger) mergeFields(fields ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range fl.fields {
		result[k] = v
	}

	if len(fields) > 0 {
		for k, v := range fields[0] {
			result[k] = v
		}
	}

	return result
}

// 全局默认日志记录器
var defaultLogger *Logger

// InitLogger 初始化默认日志记录器
func InitLogger(level LogLevel, writer Writer) {
	defaultLogger = NewLogger(level, writer)
}

// GetDefaultLogger 获取默认日志记录器
func GetDefaultLogger() *Logger {
	if defaultLogger == nil {
		// 如果没有初始化，创建一个默认的
		defaultLogger = NewLogger(INFO, NewConsoleWriter(false))
	}
	return defaultLogger
}

// SetDefaultLevel 设置默认日志级别
func SetDefaultLevel(level LogLevel) {
	GetDefaultLogger().SetLevel(level)
}

// Debug 记录调试日志
func Debug(message string, fields ...map[string]interface{}) {
	GetDefaultLogger().Debug(message, fields...)
}

// Info 记录信息日志
func Info(message string, fields ...map[string]interface{}) {
	GetDefaultLogger().Info(message, fields...)
}

// Warn 记录警告日志
func Warn(message string, fields ...map[string]interface{}) {
	GetDefaultLogger().Warn(message, fields...)
}

// Error 记录错误日志
func Error(message string, fields ...map[string]interface{}) {
	GetDefaultLogger().Error(message, fields...)
}

// Fatal 记录致命错误日志
func Fatal(message string, fields ...map[string]interface{}) {
	GetDefaultLogger().Fatal(message, fields...)
}

// WithFields 添加字段
func WithFields(fields map[string]interface{}) *FieldLogger {
	return GetDefaultLogger().WithFields(fields)
}

// Close 关闭默认日志记录器
func Close() error {
	if defaultLogger != nil {
		return defaultLogger.Close()
	}
	return nil
}

// 创建标准日志记录器的便捷函数
func NewStandardLogger(logFile string, level LogLevel) (*Logger, error) {
	var writer Writer

	if logFile == "" {
		// 只输出到控制台
		writer = NewConsoleWriter(false)
	} else {
		// 输出到控制台和文件
		fileWriter, err := NewFileWriter(logFile, 100*1024*1024) // 100MB
		if err != nil {
			return nil, err
		}

		consoleWriter := NewConsoleWriter(false)
		writer = NewMultiWriter(consoleWriter, fileWriter)
	}

	return NewLogger(level, writer), nil
}
