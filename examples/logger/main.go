package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"myetcd/internal/logger"
)

/*
日志系统示例程序

这个示例演示了如何使用MyEtcd的日志系统。
它展示了不同级别的日志、格式化输出、文件记录等功能。
*/

func main() {
	// 创建日志目录
	logDir := "./logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Printf("Failed to create log directory: %v\n", err)
		return
	}

	fmt.Println("=== MyEtcd 日志系统示例 ===\n")

	// 示例1: 基本日志记录
	fmt.Println("1. 基本日志记录")
	basicLoggingExample()

	// 示例2: 带字段的日志记录
	fmt.Println("\n2. 带字段的日志记录")
	fieldLoggingExample()

	// 示例3: 文件日志记录
	fmt.Println("\n3. 文件日志记录")
	fileLoggingExample(logDir)

	// 示例4: 多目标日志记录
	fmt.Println("\n4. 多目标日志记录")
	multiTargetLoggingExample(logDir)

	// 示例5: 日志级别控制
	fmt.Println("\n5. 日志级别控制")
	logLevelExample()

	// 示例6: 结构化日志记录
	fmt.Println("\n6. 结构化日志记录")
	structuredLoggingExample()

	// 示例7: 性能测试
	fmt.Println("\n7. 性能测试")
	performanceTest()

	// 等待用户输入退出
	fmt.Println("\n按 Ctrl+C 退出...")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	// 关闭日志系统
	if err := logger.Close(); err != nil {
		fmt.Printf("Error closing logger: %v\n", err)
	}

	fmt.Println("程序退出")
}

// basicLoggingExample 基本日志记录示例
func basicLoggingExample() {
	// 使用默认日志记录器
	logger.Debug("这是一条调试信息")
	logger.Info("这是一条信息")
	logger.Warn("这是一条警告")
	logger.Error("这是一条错误")
	
	// 注意：Fatal会终止程序，所以在这里不演示
	// logger.Fatal("这是一条致命错误")
}

// fieldLoggingExample 带字段的日志记录示例
func fieldLoggingExample() {
	// 使用WithFields添加上下文信息
	requestLogger := logger.WithFields(map[string]interface{}{
		"request_id": "12345",
		"user_id":    "user123",
		"ip":         "192.168.1.100",
	})

	requestLogger.Info("用户登录成功")

	// 添加更多字段
	logger.WithFields(map[string]interface{}{
		"request_id": "12345",
		"user_id":    "user123",
		"ip":         "192.168.1.100",
		"method":     "GET",
		"path":       "/api/v1/keys",
	}).Info("API请求")

	// 单次添加字段
	logger.Info("操作完成", map[string]interface{}{
		"operation": "PUT",
		"key":       "test-key",
		"duration":  "100ms",
	})
}

// fileLoggingExample 文件日志记录示例
func fileLoggingExample(logDir string) {
	// 创建文件日志记录器
	fileLogger, err := logger.NewStandardLogger(logDir+"/app.log", logger.INFO)
	if err != nil {
		fmt.Printf("Failed to create file logger: %v\n", err)
		return
	}
	defer fileLogger.Close()

	fileLogger.Info("这条日志会写入文件")
	fileLogger.Warn("文件日志警告", map[string]interface{}{
		"file_size": "1MB",
	})

	fmt.Printf("日志已写入文件: %s/app.log\n", logDir)
}

// multiTargetLoggingExample 多目标日志记录示例
func multiTargetLoggingExample(logDir string) {
	// 创建文件写入器
	fileWriter, err := logger.NewFileWriter(logDir+"/multi.log", 10*1024*1024) // 10MB
	if err != nil {
		fmt.Printf("Failed to create file writer: %v\n", err)
		return
	}

	// 创建控制台写入器
	consoleWriter := logger.NewConsoleWriter(false)

	// 创建多目标写入器
	multiWriter := logger.NewMultiWriter(consoleWriter, fileWriter)

	// 创建日志记录器
	multiLogger := logger.NewLogger(logger.DEBUG, multiWriter)
	defer multiLogger.Close()

	multiLogger.Debug("这条日志会同时输出到控制台和文件")
	multiLogger.Info("多目标日志测试", map[string]interface{}{
		"targets": []string{"console", "file"},
	})

	fmt.Printf("日志已同时输出到控制台和文件: %s/multi.log\n", logDir)
}

// logLevelExample 日志级别控制示例
func logLevelExample() {
	// 创建控制台日志记录器
	consoleLogger, err := logger.NewStandardLogger("", logger.DEBUG)
	if err != nil {
		fmt.Printf("Failed to create console logger: %v\n", err)
		return
	}

	fmt.Println("设置日志级别为 DEBUG，所有日志都会显示:")
	consoleLogger.Debug("DEBUG 级别日志")
	consoleLogger.Info("INFO 级别日志")
	consoleLogger.Warn("WARN 级别日志")
	consoleLogger.Error("ERROR 级别日志")

	fmt.Println("\n设置日志级别为 WARN，只有 WARN 和 ERROR 会显示:")
	consoleLogger.SetLevel(logger.WARN)
	consoleLogger.Debug("这条 DEBUG 日志不会显示")
	consoleLogger.Info("这条 INFO 日志不会显示")
	consoleLogger.Warn("这条 WARN 日志会显示")
	consoleLogger.Error("这条 ERROR 日志会显示")
}

// structuredLoggingExample 结构化日志记录示例
func structuredLoggingExample() {
	// 创建自定义日志记录器
	// 注意：这里我们需要修改FileWriter以支持自定义Formatter
	// 由于我们的FileWriter目前硬编码使用TextFormatter，这里仅作演示
	structuredLogger, err := logger.NewStandardLogger("./logs/structured.log", logger.INFO)
	if err != nil {
		fmt.Printf("Failed to create structured logger: %v\n", err)
		return
	}
	defer structuredLogger.Close()

	structuredLogger.Info("结构化日志", map[string]interface{}{
		"event":     "user_action",
		"user_id":   "user123",
		"action":    "create_key",
		"key":       "mykey",
		"timestamp": time.Now().Unix(),
		"metadata": map[string]interface{}{
			"version": "v1.0.0",
			"source":  "api",
		},
	})

	fmt.Println("结构化日志已写入文件: logs/structured.log")
}

// performanceTest 性能测试示例
func performanceTest() {
	consoleLogger, err := logger.NewStandardLogger("", logger.ERROR) // 只记录错误，减少输出
	if err != nil {
		fmt.Printf("Failed to create console logger: %v\n", err)
		return
	}

	count := 10000
	start := time.Now()

	for i := 0; i < count; i++ {
		consoleLogger.Info(fmt.Sprintf("性能测试日志 #%d", i))
	}

	duration := time.Since(start)
	throughput := float64(count) / duration.Seconds()

	fmt.Printf("性能测试结果:\n")
	fmt.Printf("  日志数量: %d\n", count)
	fmt.Printf("  总耗时: %v\n", duration)
	fmt.Printf("  吞吐量: %.2f 条/秒\n", throughput)
}