package benchmark

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"myetcd/internal/api"
)

/*
性能基准测试模块

这个模块实现了对MyEtcd的各种性能基准测试，包括：
1. PUT操作性能测试
2. GET操作性能测试
3. DELETE操作性能测试
4. 批量操作性能测试
5. 并发操作性能测试
6. Watch性能测试
7. 租约性能测试
8. 范围查询性能测试

基准测试结果包括：
- 吞吐量（操作/秒）
- 平均延迟
- P99延迟
- 错误率
*/

// BenchmarkResult 基准测试结果
type BenchmarkResult struct {
	Name         string        `json:"name"`
	Operations   int64         `json:"operations"`
	Duration     time.Duration `json:"duration"`
	Throughput   float64       `json:"throughput"`
	AvgLatency   time.Duration `json:"avg_latency"`
	P99Latency   time.Duration `json:"p99_latency"`
	ErrorCount   int64         `json:"error_count"`
	ErrorRate    float64       `json:"error_rate"`
	SuccessCount int64         `json:"success_count"`
}

// BenchmarkConfig 基准测试配置
type BenchmarkConfig struct {
	ClientCount     int           `json:"client_count"`
	OperationsCount int64         `json:"operations_count"`
	KeySize         int           `json:"key_size"`
	ValueSize       int           `json:"value_size"`
	ConcurrentLevel int           `json:"concurrent_level"`
	TestDuration    time.Duration `json:"test_duration"`
	ServerAddr      string        `json:"server_addr"`
}

// DefaultBenchmarkConfig 返回默认基准测试配置
func DefaultBenchmarkConfig() *BenchmarkConfig {
	return &BenchmarkConfig{
		ClientCount:     10,
		OperationsCount: 10000,
		KeySize:         16,
		ValueSize:       128,
		ConcurrentLevel: 100,
		TestDuration:    30 * time.Second,
		ServerAddr:      "localhost:2379",
	}
}

// BenchmarkSuite 基准测试套件
type BenchmarkSuite struct {
	config *BenchmarkConfig
	client *api.Client
}

// NewBenchmarkSuite 创建基准测试套件
func NewBenchmarkSuite(config *BenchmarkConfig) *BenchmarkSuite {
	return &BenchmarkSuite{
		config: config,
		client: api.NewClient(config.ServerAddr),
	}
}

// RunAllBenchmarks 运行所有基准测试
func (bs *BenchmarkSuite) RunAllBenchmarks() ([]*BenchmarkResult, error) {
	var results []*BenchmarkResult

	// 1. PUT操作基准测试
	putResult, err := bs.RunPutBenchmark()
	if err != nil {
		return nil, fmt.Errorf("PUT benchmark failed: %w", err)
	}
	results = append(results, putResult)

	// 2. GET操作基准测试
	getResult, err := bs.RunGetBenchmark()
	if err != nil {
		return nil, fmt.Errorf("GET benchmark failed: %w", err)
	}
	results = append(results, getResult)

	// 3. DELETE操作基准测试
	deleteResult, err := bs.RunDeleteBenchmark()
	if err != nil {
		return nil, fmt.Errorf("DELETE benchmark failed: %w", err)
	}
	results = append(results, deleteResult)

	// 4. 批量操作基准测试
	batchResult, err := bs.RunBatchBenchmark()
	if err != nil {
		return nil, fmt.Errorf("batch benchmark failed: %w", err)
	}
	results = append(results, batchResult)

	// 5. 并发操作基准测试
	concurrentResult, err := bs.RunConcurrentBenchmark()
	if err != nil {
		return nil, fmt.Errorf("concurrent benchmark failed: %w", err)
	}
	results = append(results, concurrentResult)

	return results, nil
}

// RunPutBenchmark 运行PUT操作基准测试
func (bs *BenchmarkSuite) RunPutBenchmark() (*BenchmarkResult, error) {
	fmt.Printf("Running PUT benchmark with %d operations...\n", bs.config.OperationsCount)

	var (
		successCount int64
		errorCount   int64
		totalLatency int64
		latencies     []int64
	)

	startTime := time.Now()
	var wg sync.WaitGroup

	// 并发执行PUT操作
	for i := 0; i < bs.config.ClientCount; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			operationsPerClient := bs.config.OperationsCount / int64(bs.config.ClientCount)
			for j := int64(0); j < operationsPerClient; j++ {
				key := bs.generateKey(int64(clientID), j)
				value := string(bs.generateValue())

				opStart := time.Now()
				err := bs.client.Put(key, value, 0)
				latency := time.Since(opStart)

				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					continue
				}

				atomic.AddInt64(&successCount, 1)
				atomic.AddInt64(&totalLatency, latency.Nanoseconds())
				latencies = append(latencies, latency.Nanoseconds())
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	// 计算统计信息
	result := &BenchmarkResult{
		Name:         "PUT",
		Operations:   bs.config.OperationsCount,
		Duration:     duration,
		SuccessCount: successCount,
		ErrorCount:   errorCount,
		ErrorRate:    float64(errorCount) / float64(bs.config.OperationsCount),
		Throughput:   float64(successCount) / duration.Seconds(),
	}

	if successCount > 0 {
		result.AvgLatency = time.Duration(totalLatency / successCount)
		result.P99Latency = bs.calculateP99(latencies)
	}

	return result, nil
}

// RunGetBenchmark 运行GET操作基准测试
func (bs *BenchmarkSuite) RunGetBenchmark() (*BenchmarkResult, error) {
	fmt.Printf("Running GET benchmark with %d operations...\n", bs.config.OperationsCount)

	// 预先插入数据用于GET测试
	preInsertCount := bs.config.OperationsCount / 2
	for i := int64(0); i < preInsertCount; i++ {
		key := fmt.Sprintf("bench-key-%d", i)
		value := string(bs.generateValue())
		if err := bs.client.Put(key, value, 0); err != nil {
			return nil, fmt.Errorf("failed to pre-insert data: %w", err)
		}
	}

	var (
		successCount int64
		errorCount   int64
		totalLatency int64
		latencies     []int64
	)

	startTime := time.Now()
	var wg sync.WaitGroup

	// 并发执行GET操作
	for i := 0; i < bs.config.ClientCount; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			operationsPerClient := bs.config.OperationsCount / int64(bs.config.ClientCount)
			for j := int64(0); j < operationsPerClient; j++ {
				key := fmt.Sprintf("bench-key-%d", rand.Int63n(preInsertCount))

				opStart := time.Now()
				_, err := bs.client.Get(key)
				latency := time.Since(opStart)

				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					continue
				}

				atomic.AddInt64(&successCount, 1)
				atomic.AddInt64(&totalLatency, latency.Nanoseconds())
				latencies = append(latencies, latency.Nanoseconds())
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	// 计算统计信息
	result := &BenchmarkResult{
		Name:         "GET",
		Operations:   bs.config.OperationsCount,
		Duration:     duration,
		SuccessCount: successCount,
		ErrorCount:   errorCount,
		ErrorRate:    float64(errorCount) / float64(bs.config.OperationsCount),
		Throughput:   float64(successCount) / duration.Seconds(),
	}

	if successCount > 0 {
		result.AvgLatency = time.Duration(totalLatency / successCount)
		result.P99Latency = bs.calculateP99(latencies)
	}

	return result, nil
}

// RunDeleteBenchmark 运行DELETE操作基准测试
func (bs *BenchmarkSuite) RunDeleteBenchmark() (*BenchmarkResult, error) {
	fmt.Printf("Running DELETE benchmark with %d operations...\n", bs.config.OperationsCount)

	// 预先插入数据用于DELETE测试
	for i := int64(0); i < bs.config.OperationsCount; i++ {
		key := fmt.Sprintf("delete-key-%d", i)
		value := string(bs.generateValue())
		if err := bs.client.Put(key, value, 0); err != nil {
			return nil, fmt.Errorf("failed to pre-insert data: %w", err)
		}
	}

	var (
		successCount int64
		errorCount   int64
		totalLatency int64
		latencies     []int64
	)

	startTime := time.Now()
	var wg sync.WaitGroup

	// 并发执行DELETE操作
	for i := 0; i < bs.config.ClientCount; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			operationsPerClient := bs.config.OperationsCount / int64(bs.config.ClientCount)
			for j := int64(0); j < operationsPerClient; j++ {
				key := fmt.Sprintf("delete-key-%d", int64(clientID)*operationsPerClient+j)

				opStart := time.Now()
				err := bs.client.Delete(key)
				latency := time.Since(opStart)

				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					continue
				}

				atomic.AddInt64(&successCount, 1)
				atomic.AddInt64(&totalLatency, latency.Nanoseconds())
				latencies = append(latencies, latency.Nanoseconds())
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	// 计算统计信息
	result := &BenchmarkResult{
		Name:         "DELETE",
		Operations:   bs.config.OperationsCount,
		Duration:     duration,
		SuccessCount: successCount,
		ErrorCount:   errorCount,
		ErrorRate:    float64(errorCount) / float64(bs.config.OperationsCount),
		Throughput:   float64(successCount) / duration.Seconds(),
	}

	if successCount > 0 {
		result.AvgLatency = time.Duration(totalLatency / successCount)
		result.P99Latency = bs.calculateP99(latencies)
	}

	return result, nil
}

// RunBatchBenchmark 运行批量操作基准测试
func (bs *BenchmarkSuite) RunBatchBenchmark() (*BenchmarkResult, error) {
	fmt.Printf("Running batch benchmark with %d operations...\n", bs.config.OperationsCount)

	var (
		successCount int64
		errorCount   int64
		totalLatency int64
		latencies     []int64
	)

	startTime := time.Now()
	var wg sync.WaitGroup

	// 并发执行批量操作
	for i := 0; i < bs.config.ClientCount; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			operationsPerClient := bs.config.OperationsCount / int64(bs.config.ClientCount)
			batchSize := int64(10) // 每批10个操作

			for j := int64(0); j < operationsPerClient; j += batchSize {
				kvs := make(map[string]string)
				for k := int64(0); k < batchSize && j+k < operationsPerClient; k++ {
					key := bs.generateKey(int64(clientID), j+k)
					value := string(bs.generateValue())
					kvs[key] = value
				}

				opStart := time.Now()
				// 将批量PUT转换为多个单独的PUT操作
				var batchErr error
				for k, v := range kvs {
					if err := bs.client.Put(k, v, 0); err != nil {
						batchErr = err
						break
					}
				}
				latency := time.Since(opStart)

				if batchErr != nil {
					atomic.AddInt64(&errorCount, 1)
					continue
				}

				atomic.AddInt64(&successCount, 1)
				atomic.AddInt64(&totalLatency, latency.Nanoseconds())
				latencies = append(latencies, latency.Nanoseconds())
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	// 计算统计信息
	result := &BenchmarkResult{
		Name:         "BATCH_PUT",
		Operations:   bs.config.OperationsCount,
		Duration:     duration,
		SuccessCount: successCount,
		ErrorCount:   errorCount,
		ErrorRate:    float64(errorCount) / float64(bs.config.OperationsCount),
		Throughput:   float64(successCount) / duration.Seconds(),
	}

	if successCount > 0 {
		result.AvgLatency = time.Duration(totalLatency / successCount)
		result.P99Latency = bs.calculateP99(latencies)
	}

	return result, nil
}

// RunConcurrentBenchmark 运行并发操作基准测试
func (bs *BenchmarkSuite) RunConcurrentBenchmark() (*BenchmarkResult, error) {
	fmt.Printf("Running concurrent benchmark with %d operations...\n", bs.config.OperationsCount)

	ctx, cancel := context.WithTimeout(context.Background(), bs.config.TestDuration)
	defer cancel()

	var (
		successCount int64
		errorCount   int64
		totalLatency int64
		latencies     []int64
		opCount       int64
	)

	startTime := time.Now()
	var wg sync.WaitGroup

	// 启动多个goroutine执行混合操作
	for i := 0; i < bs.config.ConcurrentLevel; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					// 随机选择操作类型
					opType := rand.Intn(3)
					key := fmt.Sprintf("concurrent-key-%d-%d", workerID, rand.Intn(1000))
					value := string(bs.generateValue())

					opStart := time.Now()
					var err error

					switch opType {
					case 0: // PUT
						err = bs.client.Put(key, value, 0)
					case 1: // GET
						_, err = bs.client.Get(key)
					case 2: // DELETE
						err = bs.client.Delete(key)
					}

					latency := time.Since(opStart)
					atomic.AddInt64(&opCount, 1)

					if err != nil {
						atomic.AddInt64(&errorCount, 1)
						continue
					}

					atomic.AddInt64(&successCount, 1)
					atomic.AddInt64(&totalLatency, latency.Nanoseconds())
					latencies = append(latencies, latency.Nanoseconds())
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	// 计算统计信息
	result := &BenchmarkResult{
		Name:         "CONCURRENT_MIX",
		Operations:   opCount,
		Duration:     duration,
		SuccessCount: successCount,
		ErrorCount:   errorCount,
		ErrorRate:    float64(errorCount) / float64(opCount),
		Throughput:   float64(successCount) / duration.Seconds(),
	}

	if successCount > 0 {
		result.AvgLatency = time.Duration(totalLatency / successCount)
		result.P99Latency = bs.calculateP99(latencies)
	}

	return result, nil
}

// generateKey 生成测试键
func (bs *BenchmarkSuite) generateKey(clientID, opID int64) string {
	return fmt.Sprintf("bench-key-%d-%d", clientID, opID)
}

// generateValue 生成测试值
func (bs *BenchmarkSuite) generateValue() []byte {
	value := make([]byte, bs.config.ValueSize)
	for i := range value {
		value[i] = byte('a' + rand.Intn(26))
	}
	return value
}

// calculateP99 计算P99延迟
func (bs *BenchmarkSuite) calculateP99(latencies []int64) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	// 简单的P99计算（实际应该使用更精确的算法）
	sorted := make([]int64, len(latencies))
	copy(sorted, latencies)

	// 简单排序（实际应该使用更高效的排序算法）
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	p99Index := int(float64(len(sorted)) * 0.99)
	if p99Index >= len(sorted) {
		p99Index = len(sorted) - 1
	}

	return time.Duration(sorted[p99Index])
}

// PrintResults 打印基准测试结果
func PrintResults(results []*BenchmarkResult) {
	fmt.Println("\n=== 基准测试结果 ===")
	fmt.Printf("%-15s %10s %12s %12s %12s %10s %10s\n",
		"操作类型", "操作数", "吞吐量(ops/s)", "平均延迟", "P99延迟", "成功率", "错误率")
	fmt.Println(strings.Repeat("-", 90))

	for _, result := range results {
		successRate := float64(result.SuccessCount) / float64(result.Operations) * 100
		fmt.Printf("%-15s %10d %12.2f %12s %12s %9.2f%% %9.2f%%\n",
			result.Name,
			result.Operations,
			result.Throughput,
			result.AvgLatency.Round(time.Microsecond),
			result.P99Latency.Round(time.Microsecond),
			successRate,
			result.ErrorRate*100)
	}
}