# MyEtcd 性能基准测试

## 概述

MyEtcd 提供了一个全面的性能基准测试模块，用于评估系统在各种工作负载下的性能表现。基准测试模块支持多种操作类型，包括 PUT、GET、DELETE、批量操作和并发操作测试。

## 功能特性

### 支持的基准测试类型

1. **PUT 操作测试**: 测试键值对写入性能
2. **GET 操作测试**: 测试键值对读取性能
3. **DELETE 操作测试**: 测试键值对删除性能
4. **批量操作测试**: 测试批量写入性能
5. **并发操作测试**: 测试混合操作下的并发性能

### 性能指标

- **吞吐量 (Throughput)**: 每秒操作数 (ops/s)
- **平均延迟 (Average Latency)**: 操作的平均响应时间
- **P99 延迟 (P99 Latency)**: 99% 的操作的响应时间
- **错误率 (Error Rate)**: 失败操作的百分比
- **成功率 (Success Rate)**: 成功操作的百分比

## 使用方法

### 1. 基本使用

```go
package main

import (
    "fmt"
    "log"
    "time"
    
    "myetcd/internal/benchmark"
)

func main() {
    // 使用默认配置
    config := benchmark.DefaultBenchmarkConfig()
    
    // 创建基准测试套件
    suite := benchmark.NewBenchmarkSuite(config)
    
    // 运行所有基准测试
    results, err := suite.RunAllBenchmarks()
    if err != nil {
        log.Fatalf("基准测试失败: %v", err)
    }
    
    // 打印结果
    benchmark.PrintResults(results)
}
```

### 2. 自定义配置

```go
config := &benchmark.BenchmarkConfig{
    ClientCount:     20,              // 客户端数量
    OperationsCount: 50000,           // 操作数量
    KeySize:         32,              // 键大小(字节)
    ValueSize:       1024,            // 值大小(字节)
    ConcurrentLevel: 200,             // 并发级别
    TestDuration:    60 * time.Second, // 测试持续时间
    ServerAddr:      "localhost:2379", // 服务器地址
}

suite := benchmark.NewBenchmarkSuite(config)
results, err := suite.RunAllBenchmarks()
```

### 3. 运行特定测试

```go
suite := benchmark.NewBenchmarkSuite(config)

// 只运行 PUT 测试
putResult, err := suite.RunPutBenchmark()
if err != nil {
    log.Printf("PUT 测试失败: %v", err)
} else {
    fmt.Printf("PUT 吞吐量: %.2f ops/s\n", putResult.Throughput)
}

// 只运行 GET 测试
getResult, err := suite.RunGetBenchmark()
if err != nil {
    log.Printf("GET 测试失败: %v", err)
} else {
    fmt.Printf("GET 平均延迟: %v\n", getResult.AvgLatency)
}
```

## 命令行工具

MyEtcd 提供了一个命令行基准测试工具，可以方便地进行性能测试。

### 基本用法

```bash
# 使用默认配置运行所有测试
go run examples/benchmark/main.go

# 自定义客户端数量和操作数量
go run examples/benchmark/main.go -clients 20 -operations 50000

# 测试更大的键值对
go run examples/benchmark/main.go -key-size 32 -value-size 1024

# 测试更高的并发级别
go run examples/benchmark/main.go -concurrent 500

# 连接到不同的服务器
go run examples/benchmark/main.go -server 192.168.1.100:2379
```

### 命令行选项

| 选项 | 描述 | 默认值 |
|------|------|--------|
| `-server` | 服务器地址 | `localhost:2379` |
| `-clients` | 客户端数量 | `10` |
| `-operations` | 操作数量 | `10000` |
| `-key-size` | 键大小(字节) | `16` |
| `-value-size` | 值大小(字节) | `128` |
| `-concurrent` | 并发级别 | `100` |
| `-duration` | 测试持续时间(秒) | `30` |
| `-test` | 运行特定测试 | `all` |
| `-help, -h` | 显示帮助信息 | - |

## 测试结果示例

```
=== 基准测试结果 ===
操作类型          操作数     吞吐量(ops/s)  平均延迟      P99延迟       成功率    错误率
------------------------------------------------------------------------------------------
PUT              10000      1250.50       800µs         1500µs        99.80%    0.20%
GET              10000      2100.75       475µs         900µs         99.90%    0.10%
DELETE           10000      1800.25       555µs         1100µs        99.85%    0.15%
BATCH_PUT        10000      950.30        1050µs        2000µs        99.70%    0.30%
CONCURRENT_MIX   10000      1650.80       606µs         1200µs        99.75%    0.25%

=== 性能分析报告 ===
测试配置:
  服务器地址: localhost:2379
  客户端数量: 10
  操作数量: 10000
  键大小: 16 字节
  值大小: 128 字节
  并发级别: 100
  测试持续时间: 30s

性能摘要:
  总操作数: 50000
  平均吞吐量: 1550.52 ops/s
  平均延迟: 688µs
  平均P99延迟: 1340µs

性能评级: 良好 (Good)

优化建议:
  1. 考虑增加服务器资源以提高吞吐量
  2. 优化网络配置以减少延迟
  3. 调整批处理大小以提高批量操作性能
  4. 考虑使用连接池以减少连接开销
  5. 根据工作负载调整缓存策略
```

## 性能调优建议

### 1. 提高吞吐量

- **增加客户端数量**: 适当增加并发客户端数量可以提高总体吞吐量
- **优化批处理大小**: 对于批量操作，找到最佳的批处理大小
- **调整并发级别**: 根据服务器能力调整并发级别

### 2. 降低延迟

- **减少网络延迟**: 确保客户端和服务器之间的网络连接良好
- **优化键值大小**: 较小的键值对通常有更低的延迟
- **使用连接池**: 减少连接建立和销毁的开销

### 3. 提高稳定性

- **监控错误率**: 如果错误率过高，可能需要调整配置
- **合理设置超时**: 确保操作超时设置合理
- **资源监控**: 监控服务器资源使用情况

## 最佳实践

1. **测试环境**: 在与生产环境相似的硬件和网络条件下进行测试
2. **多次测试**: 运行多次测试以获得更准确的结果
3. **渐进式测试**: 从小规模测试开始，逐步增加负载
4. **监控资源**: 在测试过程中监控 CPU、内存和网络使用情况
5. **记录结果**: 保存测试结果以便后续分析和比较

## 故障排除

### 常见问题

1. **连接被拒绝**: 确保 MyEtcd 服务器正在运行并且地址正确
2. **高错误率**: 检查服务器资源使用情况，可能需要增加资源
3. **低吞吐量**: 可能是网络延迟或服务器性能问题
4. **测试超时**: 增加测试超时时间或减少操作数量

### 调试技巧

1. **启用详细日志**: 增加日志级别以获取更多信息
2. **单客户端测试**: 先使用单个客户端进行测试
3. **减少操作数量**: 从少量操作开始，逐步增加
4. **检查服务器状态**: 使用健康检查端点验证服务器状态

## 扩展和定制

### 添加新的基准测试

可以通过实现 `BenchmarkSuite` 接口来添加新的基准测试类型：

```go
// 添加新的基准测试方法
func (bs *BenchmarkSuite) RunCustomBenchmark() (*BenchmarkResult, error) {
    // 实现自定义基准测试逻辑
    return result, nil
}
```

### 自定义性能指标

可以扩展 `BenchmarkResult` 结构体来添加更多性能指标：

```go
type BenchmarkResult struct {
    // 现有字段...
    MinLatency   time.Duration `json:"min_latency"`
    MaxLatency   time.Duration `json:"max_latency"`
    MemoryUsage  int64         `json:"memory_usage"`
    CPUUsage     float64       `json:"cpu_usage"`
}
```

## 总结

MyEtcd 的基准测试模块提供了一个强大而灵活的工具来评估系统性能。通过合理配置和使用这些测试，可以：

- 了解系统在不同工作负载下的性能表现
- 识别性能瓶颈和优化机会
- 验证系统改进的效果
- 为容量规划提供数据支持

定期进行基准测试是确保 MyEtcd 系统性能稳定和持续优化的重要实践。