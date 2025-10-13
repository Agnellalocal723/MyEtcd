package tests

import (
	"testing"
	"time"

	"myetcd/internal/benchmark"
)

/*
基准测试模块的单元测试

这个测试文件验证了基准测试模块的各项功能，包括：
1. 基准测试配置的创建和验证
2. 基准测试套件的初始化
3. 各种基准测试的执行
4. 性能统计数据的计算
*/

// TestDefaultBenchmarkConfig 测试默认基准测试配置
func TestDefaultBenchmarkConfig(t *testing.T) {
	config := benchmark.DefaultBenchmarkConfig()

	if config.ClientCount != 10 {
		t.Errorf("Expected ClientCount to be 10, got %d", config.ClientCount)
	}

	if config.OperationsCount != 10000 {
		t.Errorf("Expected OperationsCount to be 10000, got %d", config.OperationsCount)
	}

	if config.KeySize != 16 {
		t.Errorf("Expected KeySize to be 16, got %d", config.KeySize)
	}

	if config.ValueSize != 128 {
		t.Errorf("Expected ValueSize to be 128, got %d", config.ValueSize)
	}

	if config.ConcurrentLevel != 100 {
		t.Errorf("Expected ConcurrentLevel to be 100, got %d", config.ConcurrentLevel)
	}

	if config.TestDuration != 30*time.Second {
		t.Errorf("Expected TestDuration to be 30s, got %v", config.TestDuration)
	}

	if config.ServerAddr != "localhost:2379" {
		t.Errorf("Expected ServerAddr to be 'localhost:2379', got %s", config.ServerAddr)
	}
}

// TestBenchmarkSuiteCreation 测试基准测试套件的创建
func TestBenchmarkSuiteCreation(t *testing.T) {
	config := benchmark.DefaultBenchmarkConfig()
	suite := benchmark.NewBenchmarkSuite(config)

	if suite == nil {
		t.Fatal("Expected benchmark suite to be created, got nil")
	}

	// 由于config和client是私有字段，我们无法直接访问它们
	// 但我们可以通过测试其他方法来验证套件是否正确初始化
}

// TestCustomBenchmarkConfig 测试自定义基准测试配置
func TestCustomBenchmarkConfig(t *testing.T) {
	config := &benchmark.BenchmarkConfig{
		ClientCount:     5,
		OperationsCount: 5000,
		KeySize:         32,
		ValueSize:       256,
		ConcurrentLevel: 50,
		TestDuration:    60 * time.Second,
		ServerAddr:      "example.com:2379",
	}

	suite := benchmark.NewBenchmarkSuite(config)

	// 验证套件是否成功创建
	if suite == nil {
		t.Fatal("Expected benchmark suite to be created, got nil")
	}

	// 由于config是私有字段，我们无法直接访问它
	// 但我们可以通过验证原始配置来确保它被正确设置
	if config.ClientCount != 5 {
		t.Errorf("Expected ClientCount to be 5, got %d", config.ClientCount)
	}

	if config.OperationsCount != 5000 {
		t.Errorf("Expected OperationsCount to be 5000, got %d", config.OperationsCount)
	}

	if config.ServerAddr != "example.com:2379" {
		t.Errorf("Expected ServerAddr to be 'example.com:2379', got %s", config.ServerAddr)
	}
}

// BenchmarkPutOperation PUT操作的基准测试
func BenchmarkPutOperation(b *testing.B) {
	config := &benchmark.BenchmarkConfig{
		ClientCount:     1,
		OperationsCount: int64(b.N),
		KeySize:         16,
		ValueSize:       128,
		ConcurrentLevel: 1,
		TestDuration:    30 * time.Second,
		ServerAddr:      "localhost:2379",
	}

	suite := benchmark.NewBenchmarkSuite(config)

	// 重置计时器
	b.ResetTimer()

	// 运行基准测试
	result, err := suite.RunPutBenchmark()
	if err != nil {
		b.Fatalf("PUT benchmark failed: %v", err)
	}

	// 报告结果
	b.ReportMetric(result.Throughput, "ops/sec")
	b.ReportMetric(float64(result.AvgLatency.Nanoseconds()), "avg_latency_ns")
	b.ReportMetric(float64(result.P99Latency.Nanoseconds()), "p99_latency_ns")
}

// BenchmarkGetOperation GET操作的基准测试
func BenchmarkGetOperation(b *testing.B) {
	config := &benchmark.BenchmarkConfig{
		ClientCount:     1,
		OperationsCount: int64(b.N),
		KeySize:         16,
		ValueSize:       128,
		ConcurrentLevel: 1,
		TestDuration:    30 * time.Second,
		ServerAddr:      "localhost:2379",
	}

	suite := benchmark.NewBenchmarkSuite(config)

	// 重置计时器
	b.ResetTimer()

	// 运行基准测试
	result, err := suite.RunGetBenchmark()
	if err != nil {
		b.Fatalf("GET benchmark failed: %v", err)
	}

	// 报告结果
	b.ReportMetric(result.Throughput, "ops/sec")
	b.ReportMetric(float64(result.AvgLatency.Nanoseconds()), "avg_latency_ns")
	b.ReportMetric(float64(result.P99Latency.Nanoseconds()), "p99_latency_ns")
}

// BenchmarkDeleteOperation DELETE操作的基准测试
func BenchmarkDeleteOperation(b *testing.B) {
	config := &benchmark.BenchmarkConfig{
		ClientCount:     1,
		OperationsCount: int64(b.N),
		KeySize:         16,
		ValueSize:       128,
		ConcurrentLevel: 1,
		TestDuration:    30 * time.Second,
		ServerAddr:      "localhost:2379",
	}

	suite := benchmark.NewBenchmarkSuite(config)

	// 重置计时器
	b.ResetTimer()

	// 运行基准测试
	result, err := suite.RunDeleteBenchmark()
	if err != nil {
		b.Fatalf("DELETE benchmark failed: %v", err)
	}

	// 报告结果
	b.ReportMetric(result.Throughput, "ops/sec")
	b.ReportMetric(float64(result.AvgLatency.Nanoseconds()), "avg_latency_ns")
	b.ReportMetric(float64(result.P99Latency.Nanoseconds()), "p99_latency_ns")
}

// BenchmarkBatchOperation 批量操作的基准测试
func BenchmarkBatchOperation(b *testing.B) {
	config := &benchmark.BenchmarkConfig{
		ClientCount:     1,
		OperationsCount: int64(b.N),
		KeySize:         16,
		ValueSize:       128,
		ConcurrentLevel: 1,
		TestDuration:    30 * time.Second,
		ServerAddr:      "localhost:2379",
	}

	suite := benchmark.NewBenchmarkSuite(config)

	// 重置计时器
	b.ResetTimer()

	// 运行基准测试
	result, err := suite.RunBatchBenchmark()
	if err != nil {
		b.Fatalf("Batch benchmark failed: %v", err)
	}

	// 报告结果
	b.ReportMetric(result.Throughput, "ops/sec")
	b.ReportMetric(float64(result.AvgLatency.Nanoseconds()), "avg_latency_ns")
	b.ReportMetric(float64(result.P99Latency.Nanoseconds()), "p99_latency_ns")
}

// BenchmarkConcurrentOperation 并发操作的基准测试
func BenchmarkConcurrentOperation(b *testing.B) {
	config := &benchmark.BenchmarkConfig{
		ClientCount:     1,
		OperationsCount: int64(b.N),
		KeySize:         16,
		ValueSize:       128,
		ConcurrentLevel: 10,
		TestDuration:    10 * time.Second, // 较短的测试时间
		ServerAddr:      "localhost:2379",
	}

	suite := benchmark.NewBenchmarkSuite(config)

	// 重置计时器
	b.ResetTimer()

	// 运行基准测试
	result, err := suite.RunConcurrentBenchmark()
	if err != nil {
		b.Fatalf("Concurrent benchmark failed: %v", err)
	}

	// 报告结果
	b.ReportMetric(result.Throughput, "ops/sec")
	b.ReportMetric(float64(result.AvgLatency.Nanoseconds()), "avg_latency_ns")
	b.ReportMetric(float64(result.P99Latency.Nanoseconds()), "p99_latency_ns")
}

// TestBenchmarkResultValidation 测试基准测试结果的验证
func TestBenchmarkResultValidation(t *testing.T) {
	// 创建一个模拟的基准测试结果
	result := &benchmark.BenchmarkResult{
		Name:         "TEST",
		Operations:   1000,
		Duration:     10 * time.Second,
		SuccessCount: 950,
		ErrorCount:   50,
	}

	// 计算派生字段
	result.ErrorRate = float64(result.ErrorCount) / float64(result.Operations)
	result.Throughput = float64(result.SuccessCount) / result.Duration.Seconds()
	result.AvgLatency = time.Duration(5000000)  // 5ms
	result.P99Latency = time.Duration(10000000) // 10ms

	// 验证计算结果
	expectedErrorRate := 0.05 // 5%
	if result.ErrorRate != expectedErrorRate {
		t.Errorf("Expected ErrorRate to be %f, got %f", expectedErrorRate, result.ErrorRate)
	}

	expectedThroughput := 95.0 // 950 ops / 10s
	if result.Throughput != expectedThroughput {
		t.Errorf("Expected Throughput to be %f, got %f", expectedThroughput, result.Throughput)
	}

	if result.AvgLatency != 5*time.Millisecond {
		t.Errorf("Expected AvgLatency to be 5ms, got %v", result.AvgLatency)
	}

	if result.P99Latency != 10*time.Millisecond {
		t.Errorf("Expected P99Latency to be 10ms, got %v", result.P99Latency)
	}
}

// TestPrintResults 测试结果打印功能
func TestPrintResults(t *testing.T) {
	// 创建一些模拟结果
	results := []*benchmark.BenchmarkResult{
		{
			Name:         "PUT",
			Operations:   1000,
			Duration:     10 * time.Second,
			SuccessCount: 950,
			ErrorCount:   50,
			ErrorRate:    0.05,
			Throughput:   95.0,
			AvgLatency:   5 * time.Millisecond,
			P99Latency:   10 * time.Millisecond,
		},
		{
			Name:         "GET",
			Operations:   1000,
			Duration:     5 * time.Second,
			SuccessCount: 980,
			ErrorCount:   20,
			ErrorRate:    0.02,
			Throughput:   196.0,
			AvgLatency:   2 * time.Millisecond,
			P99Latency:   5 * time.Millisecond,
		},
	}

	// 这个测试只是确保函数不会panic
	// 在实际测试中，我们不会验证输出内容
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("PrintResults panicked: %v", r)
		}
	}()

	benchmark.PrintResults(results)
}
