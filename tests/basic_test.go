package tests

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"myetcd/internal/api"
	"myetcd/internal/storage"
	"myetcd/internal/types"
	"myetcd/internal/wal"
)

// TestBasicOperations 测试基本操作
func TestBasicOperations(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "myetcd-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建配置
	config := &types.Config{
		DataDir:          tempDir,
		WALDir:           filepath.Join(tempDir, "wal"),
		SnapshotDir:      filepath.Join(tempDir, "snapshot"),
		MaxWALSize:       1024 * 1024, // 1MB
		SnapshotInterval: time.Hour,
		MaxSnapshots:     3,
	}

	// 创建WAL
	walInstance, err := wal.NewWAL(config.WALDir, config.MaxWALSize)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}
	defer walInstance.Close()

	// 创建存储引擎
	engine, err := storage.NewEngine(config, walInstance)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	// 测试PUT操作
	err = engine.Put("test-key", []byte("test-value"), 0)
	if err != nil {
		t.Fatalf("Failed to put key: %v", err)
	}

	// 测试GET操作
	kv, err := engine.Get("test-key")
	if err != nil {
		t.Fatalf("Failed to get key: %v", err)
	}

	if string(kv.Value) != "test-value" {
		t.Errorf("Expected value 'test-value', got '%s'", string(kv.Value))
	}

	// 测试LIST操作
	keys := engine.ListKeys()
	if len(keys) != 1 || keys[0] != "test-key" {
		t.Errorf("Expected 1 key 'test-key', got %v", keys)
	}

	// 测试DELETE操作
	err = engine.Delete("test-key")
	if err != nil {
		t.Fatalf("Failed to delete key: %v", err)
	}

	// 验证删除后的状态
	_, err = engine.Get("test-key")
	if err != storage.ErrKeyNotFound {
		t.Errorf("Expected ErrKeyNotFound, got %v", err)
	}
}

// TestTTL 测试TTL功能
func TestTTL(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "myetcd-test-ttl")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建配置
	config := &types.Config{
		DataDir:          tempDir,
		WALDir:           filepath.Join(tempDir, "wal"),
		SnapshotDir:      filepath.Join(tempDir, "snapshot"),
		MaxWALSize:       1024 * 1024,
		SnapshotInterval: time.Hour,
		MaxSnapshots:     3,
	}

	// 创建WAL
	walInstance, err := wal.NewWAL(config.WALDir, config.MaxWALSize)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}
	defer walInstance.Close()

	// 创建存储引擎
	engine, err := storage.NewEngine(config, walInstance)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	// 测试带TTL的PUT操作
	err = engine.Put("ttl-key", []byte("ttl-value"), 2) // 2秒TTL
	if err != nil {
		t.Fatalf("Failed to put key with TTL: %v", err)
	}

	// 立即获取应该成功
	kv, err := engine.Get("ttl-key")
	if err != nil {
		t.Fatalf("Failed to get key before expiry: %v", err)
	}
	if string(kv.Value) != "ttl-value" {
		t.Errorf("Expected value 'ttl-value', got '%s'", string(kv.Value))
	}

	// 等待过期
	time.Sleep(3 * time.Second)

	// 再次获取应该失败
	_, err = engine.Get("ttl-key")
	if err != storage.ErrKeyNotFound {
		t.Errorf("Expected ErrKeyNotFound (expired keys are treated as not found), got %v", err)
	}
}

// TestTransaction 测试事务功能
func TestTransaction(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "myetcd-test-txn")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建配置
	config := &types.Config{
		DataDir:          tempDir,
		WALDir:           filepath.Join(tempDir, "wal"),
		SnapshotDir:      filepath.Join(tempDir, "snapshot"),
		MaxWALSize:       1024 * 1024,
		SnapshotInterval: time.Hour,
		MaxSnapshots:     3,
	}

	// 创建WAL
	walInstance, err := wal.NewWAL(config.WALDir, config.MaxWALSize)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}
	defer walInstance.Close()

	// 创建存储引擎
	engine, err := storage.NewEngine(config, walInstance)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	// 开始事务
	txn, err := engine.BeginTransaction()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// 添加操作
	err = engine.AddOperation(txn.ID, types.Operation{
		Type:  types.CommandPut,
		Key:   "txn-key1",
		Value: []byte("txn-value1"),
	})
	if err != nil {
		t.Fatalf("Failed to add operation: %v", err)
	}

	err = engine.AddOperation(txn.ID, types.Operation{
		Type:  types.CommandPut,
		Key:   "txn-key2",
		Value: []byte("txn-value2"),
	})
	if err != nil {
		t.Fatalf("Failed to add operation: %v", err)
	}

	// 提交事务
	err = engine.CommitTransaction(txn.ID)
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	// 验证事务结果
	kv1, err := engine.Get("txn-key1")
	if err != nil {
		t.Fatalf("Failed to get txn-key1: %v", err)
	}
	if string(kv1.Value) != "txn-value1" {
		t.Errorf("Expected 'txn-value1', got '%s'", string(kv1.Value))
	}

	kv2, err := engine.Get("txn-key2")
	if err != nil {
		t.Fatalf("Failed to get txn-key2: %v", err)
	}
	if string(kv2.Value) != "txn-value2" {
		t.Errorf("Expected 'txn-value2', got '%s'", string(kv2.Value))
	}
}

// TestWALRecovery 测试WAL恢复
func TestWALRecovery(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "myetcd-test-wal")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建配置
	config := &types.Config{
		DataDir:          tempDir,
		WALDir:           filepath.Join(tempDir, "wal"),
		SnapshotDir:      filepath.Join(tempDir, "snapshot"),
		MaxWALSize:       1024 * 1024,
		SnapshotInterval: time.Hour,
		MaxSnapshots:     3,
	}

	// 第一阶段：写入数据
	{
		// 创建WAL
		walInstance, err := wal.NewWAL(config.WALDir, config.MaxWALSize)
		if err != nil {
			t.Fatalf("Failed to create WAL: %v", err)
		}

		// 创建存储引擎
		engine, err := storage.NewEngine(config, walInstance)
		if err != nil {
			t.Fatalf("Failed to create engine: %v", err)
		}

		// 写入一些数据
		engine.Put("recovery-key1", []byte("recovery-value1"), 0)
		engine.Put("recovery-key2", []byte("recovery-value2"), 0)

		// 关闭引擎和WAL
		engine.Close()
		walInstance.Close()
	}

	// 第二阶段：恢复数据
	{
		// 重新创建WAL
		walInstance, err := wal.NewWAL(config.WALDir, config.MaxWALSize)
		if err != nil {
			t.Fatalf("Failed to recreate WAL: %v", err)
		}
		defer walInstance.Close()

		// 重新创建存储引擎（应该从WAL恢复）
		engine, err := storage.NewEngine(config, walInstance)
		if err != nil {
			t.Fatalf("Failed to recreate engine: %v", err)
		}
		defer engine.Close()

		// 验证数据恢复
		kv1, err := engine.Get("recovery-key1")
		if err != nil {
			t.Fatalf("Failed to get recovered key1: %v", err)
		}
		if string(kv1.Value) != "recovery-value1" {
			t.Errorf("Expected 'recovery-value1', got '%s'", string(kv1.Value))
		}

		kv2, err := engine.Get("recovery-key2")
		if err != nil {
			t.Fatalf("Failed to get recovered key2: %v", err)
		}
		if string(kv2.Value) != "recovery-value2" {
			t.Errorf("Expected 'recovery-value2', got '%s'", string(kv2.Value))
		}
	}
}

// TestAPIServer 测试API服务器
func TestAPIServer(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "myetcd-test-api")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建配置
	config := &types.Config{
		DataDir:          tempDir,
		WALDir:           filepath.Join(tempDir, "wal"),
		SnapshotDir:      filepath.Join(tempDir, "snapshot"),
		MaxWALSize:       1024 * 1024,
		SnapshotInterval: time.Hour,
		MaxSnapshots:     3,
	}

	// 创建WAL
	walInstance, err := wal.NewWAL(config.WALDir, config.MaxWALSize)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}
	defer walInstance.Close()

	// 创建存储引擎
	engine, err := storage.NewEngine(config, walInstance)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	// 创建API服务器
	server := api.NewServer(engine, config)
	server.SetAddr(":12345") // 使用不同的端口避免冲突

	// 启动服务器
	go func() {
		if err := server.Start(); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 创建客户端
	client := api.NewClient("localhost:12345")

	// 测试PUT
	err = client.Put("api-key", "api-value", 0)
	if err != nil {
		t.Fatalf("Failed to put via API: %v", err)
	}

	// 测试GET
	resp, err := client.Get("api-key")
	if err != nil {
		t.Fatalf("Failed to get via API: %v", err)
	}
	if resp.Value != "api-value" {
		t.Errorf("Expected 'api-value', got '%s'", resp.Value)
	}

	// 测试LIST
	keys, err := client.ListKeys()
	if err != nil {
		t.Fatalf("Failed to list keys via API: %v", err)
	}
	if len(keys) != 1 || keys[0] != "api-key" {
		t.Errorf("Expected 1 key 'api-key', got %v", keys)
	}

	// 测试DELETE
	err = client.Delete("api-key")
	if err != nil {
		t.Fatalf("Failed to delete via API: %v", err)
	}

	// 停止服务器
	server.Stop()
}

// TestTransactionAPI 测试事务API
func TestTransactionAPI(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "myetcd-test-txn-api")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建配置
	config := &types.Config{
		DataDir:          tempDir,
		WALDir:           filepath.Join(tempDir, "wal"),
		SnapshotDir:      filepath.Join(tempDir, "snapshot"),
		MaxWALSize:       1024 * 1024,
		SnapshotInterval: time.Hour,
		MaxSnapshots:     3,
	}

	// 创建WAL
	walInstance, err := wal.NewWAL(config.WALDir, config.MaxWALSize)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}
	defer walInstance.Close()

	// 创建存储引擎
	engine, err := storage.NewEngine(config, walInstance)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	// 创建API服务器
	server := api.NewServer(engine, config)
	server.SetAddr(":12346")

	// 启动服务器
	go func() {
		if err := server.Start(); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 创建客户端
	client := api.NewClient("localhost:12346")

	// 首先设置一个键
	err = client.Put("txn-test-key", "old-value", 0)
	if err != nil {
		t.Fatalf("Failed to put initial key: %v", err)
	}

	// 创建事务请求：如果键的值是"old-value"，则更新为"new-value"
	txnReq := types.TxnRequest{
		Compare: []types.Compare{
			{
				Key:   "txn-test-key",
				Op:    "=",
				Value: "old-value",
			},
		},
		Success: []types.Request{
			{
				Op:    "PUT",
				Key:   "txn-test-key",
				Value: "new-value",
			},
		},
	}

	// 执行事务
	txnResp, err := client.Transaction(txnReq)
	if err != nil {
		t.Fatalf("Failed to execute transaction: %v", err)
	}

	// 检查事务结果
	if !txnResp.Success {
		t.Errorf("Expected transaction to succeed")
	}

	// 验证键的值已更新
	resp, err := client.Get("txn-test-key")
	if err != nil {
		t.Fatalf("Failed to get key after transaction: %v", err)
	}
	if resp.Value != "new-value" {
		t.Errorf("Expected 'new-value', got '%s'", resp.Value)
	}

	// 停止服务器
	server.Stop()
}
