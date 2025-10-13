package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"myetcd/internal/benchmark"
)

/*
基准测试示例程序

这个程序演示了如何使用MyEtcd的基准测试模块来评估系统性能。
它可以运行各种基准测试，包括PUT、GET、DELETE、批量操作和并发操作测试。

使用方法:
  go run examples/benchmark/main.go [选项]

选项:
  -server string     服务器地址 (默认: "localhost:2379")
  -clients int       客户端数量 (默认: 10)
  -operations int    操作数量 (默认: 10000)
  -key-size int      键大小(字节) (默认: 16)
  -value-size int    值大小(字节) (默认: 128)
  -concurrent int    并发级别 (默认: 100)
  -duration int      测试持续时间(秒) (默认: 30)
  -test string       运行特定测试 (put|get|delete|batch|concurrent|all)
*/

func main() {
	// 解析命令行参数
	config := parseArgs()

	// 创建基准测试套件
	suite := benchmark.NewBenchmarkSuite(config)

	// 根据参数运行不同的测试
	var results []*benchmark.BenchmarkResult
	var err error

	switch config.TestDuration {
	case time.Second: // 使用持续时间作为测试类型标识符
		// 运行所有基准测试
		fmt.Println("开始运行所有基准测试...")
		results, err = suite.RunAllBenchmarks()
		if err != nil {
			log.Fatalf("运行基准测试失败: %v", err)
		}
	default:
		// 运行所有基准测试
		fmt.Println("开始运行所有基准测试...")
		results, err = suite.RunAllBenchmarks()
		if err != nil {
			log.Fatalf("运行基准测试失败: %v", err)
		}
	}

	// 打印结果
	benchmark.PrintResults(results)

	// 生成性能报告
	generateReport(results, config)
}

// parseArgs 解析命令行参数
func parseArgs() *benchmark.BenchmarkConfig {
	config := benchmark.DefaultBenchmarkConfig()

	// 简单的命令行参数解析
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-server":
			if i+1 < len(args) {
				config.ServerAddr = args[i+1]
				i++
			}
		case "-clients":
			if i+1 < len(args) {
				var clientCount int
				fmt.Sscanf(args[i+1], "%d", &clientCount)
				config.ClientCount = clientCount
				i++
			}
		case "-operations":
			if i+1 < len(args) {
				var operationsCount int64
				fmt.Sscanf(args[i+1], "%d", &operationsCount)
				config.OperationsCount = operationsCount
				i++
			}
		case "-key-size":
			if i+1 < len(args) {
				var keySize int
				fmt.Sscanf(args[i+1], "%d", &keySize)
				config.KeySize = keySize
				i++
			}
		case "-value-size":
			if i+1 < len(args) {
				var valueSize int
				fmt.Sscanf(args[i+1], "%d", &valueSize)
				config.ValueSize = valueSize
				i++
			}
		case "-concurrent":
			if i+1 < len(args) {
				var concurrentLevel int
				fmt.Sscanf(args[i+1], "%d", &concurrentLevel)
				config.ConcurrentLevel = concurrentLevel
				i++
			}
		case "-duration":
			if i+1 < len(args) {
				var duration int
				fmt.Sscanf(args[i+1], "%d", &duration)
				config.TestDuration = time.Duration(duration) * time.Second
				i++
			}
		case "-test":
			if i+1 < len(args) {
				testType := args[i+1]
				switch testType {
				case "put":
					// 这里可以添加运行单个测试的逻辑
				case "get":
					// 这里可以添加运行单个测试的逻辑
				case "delete":
					// 这里可以添加运行单个测试的逻辑
				case "batch":
					// 这里可以添加运行单个测试的逻辑
				case "concurrent":
					// 这里可以添加运行单个测试的逻辑
				case "all":
					// 默认运行所有测试
				}
				i++
			}
		case "-help", "-h":
			printUsage()
			os.Exit(0)
		}
	}

	return config
}

// printUsage 打印使用说明
func printUsage() {
	fmt.Println("MyEtcd 基准测试工具")
	fmt.Println()
	fmt.Println("使用方法:")
	fmt.Println("  go run examples/benchmark/main.go [选项]")
	fmt.Println()
	fmt.Println("选项:")
	fmt.Println("  -server string     服务器地址 (默认: \"localhost:2379\")")
	fmt.Println("  -clients int       客户端数量 (默认: 10)")
	fmt.Println("  -operations int    操作数量 (默认: 10000)")
	fmt.Println("  -key-size int      键大小(字节) (默认: 16)")
	fmt.Println("  -value-size int    值大小(字节) (默认: 128)")
	fmt.Println("  -concurrent int    并发级别 (默认: 100)")
	fmt.Println("  -duration int      测试持续时间(秒) (默认: 30)")
	fmt.Println("  -test string       运行特定测试 (put|get|delete|batch|concurrent|all)")
	fmt.Println("  -help, -h          显示帮助信息")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  # 使用默认配置运行所有测试")
	fmt.Println("  go run examples/benchmark/main.go")
	fmt.Println()
	fmt.Println("  # 使用更多客户端和操作运行测试")
	fmt.Println("  go run examples/benchmark/main.go -clients 20 -operations 50000")
	fmt.Println()
	fmt.Println("  # 测试更大的键值对")
	fmt.Println("  go run examples/benchmark/main.go -key-size 32 -value-size 1024")
	fmt.Println()
	fmt.Println("  # 测试更高的并发级别")
	fmt.Println("  go run examples/benchmark/main.go -concurrent 500")
	fmt.Println()
	fmt.Println("  # 连接到不同的服务器")
	fmt.Println("  go run examples/benchmark/main.go -server 192.168.1.100:2379")
}

// generateReport 生成性能报告
func generateReport(results []*benchmark.BenchmarkResult, config *benchmark.BenchmarkConfig) {
	fmt.Println("\n=== 性能分析报告 ===")
	fmt.Printf("测试配置:\n")
	fmt.Printf("  服务器地址: %s\n", config.ServerAddr)
	fmt.Printf("  客户端数量: %d\n", config.ClientCount)
	fmt.Printf("  操作数量: %d\n", config.OperationsCount)
	fmt.Printf("  键大小: %d 字节\n", config.KeySize)
	fmt.Printf("  值大小: %d 字节\n", config.ValueSize)
	fmt.Printf("  并发级别: %d\n", config.ConcurrentLevel)
	fmt.Printf("  测试持续时间: %v\n", config.TestDuration)
	fmt.Println()

	// 分析性能特征
	var totalThroughput float64
	var avgLatency, avgP99Latency time.Duration
	var totalOperations int64

	for _, result := range results {
		totalThroughput += result.Throughput
		avgLatency += result.AvgLatency
		avgP99Latency += result.P99Latency
		totalOperations += result.Operations
	}

	testCount := len(results)
	if testCount > 0 {
		avgLatency = avgLatency / time.Duration(testCount)
		avgP99Latency = avgP99Latency / time.Duration(testCount)

		fmt.Printf("性能摘要:\n")
		fmt.Printf("  总操作数: %d\n", totalOperations)
		fmt.Printf("  平均吞吐量: %.2f ops/s\n", totalThroughput/float64(testCount))
		fmt.Printf("  平均延迟: %v\n", avgLatency.Round(time.Microsecond))
		fmt.Printf("  平均P99延迟: %v\n", avgP99Latency.Round(time.Microsecond))
		fmt.Println()

		// 性能评级
		rating := evaluatePerformance(totalThroughput/float64(testCount), avgLatency)
		fmt.Printf("性能评级: %s\n", rating)
		fmt.Println()

		// 优化建议
		generateOptimizationSuggestions(results, config)
	}
}

// evaluatePerformance 评估性能等级
func evaluatePerformance(throughput float64, avgLatency time.Duration) string {
	// 简单的性能评级标准
	if throughput > 10000 && avgLatency < time.Microsecond*100 {
		return "优秀 (Excellent)"
	} else if throughput > 5000 && avgLatency < time.Microsecond*500 {
		return "良好 (Good)"
	} else if throughput > 1000 && avgLatency < time.Millisecond {
		return "一般 (Average)"
	} else {
		return "需要优化 (Needs Optimization)"
	}
}

// generateOptimizationSuggestions 生成优化建议
func generateOptimizationSuggestions(results []*benchmark.BenchmarkResult, config *benchmark.BenchmarkConfig) {
	fmt.Println("优化建议:")
	
	// 这里可以根据测试结果生成具体的优化建议
	fmt.Println("  1. 考虑增加服务器资源以提高吞吐量")
	fmt.Println("  2. 优化网络配置以减少延迟")
	fmt.Println("  3. 调整批处理大小以提高批量操作性能")
	fmt.Println("  4. 考虑使用连接池以减少连接开销")
	fmt.Println("  5. 根据工作负载调整缓存策略")
	fmt.Println()
}