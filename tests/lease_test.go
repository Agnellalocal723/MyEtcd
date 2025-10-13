package tests

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"myetcd/internal/storage"
	"myetcd/internal/types"
	"myetcd/internal/wal"
)

// TestLease 测试租约功能
func TestLease(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "myetcd-test-lease")
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

	// 测试创建租约
	t.Run("GrantLease", func(t *testing.T) {
		leaseID, err := engine.GrantLease(10) // 10秒TTL
		if err != nil {
			t.Fatalf("Failed to grant lease: %v", err)
		}

		if leaseID <= 0 {
			t.Errorf("Expected positive lease ID, got %d", leaseID)
		}

		// 获取租约信息
		lease, err := engine.GetLease(leaseID)
		if err != nil {
			t.Fatalf("Failed to get lease: %v", err)
		}

		if lease.ID != leaseID {
			t.Errorf("Expected lease ID %d, got %d", leaseID, lease.ID)
		}

		if lease.TTL != 10 {
			t.Errorf("Expected TTL 10, got %d", lease.TTL)
		}

		if lease.Remaining <= 0 || lease.Remaining > 10 {
			t.Errorf("Expected remaining time between 0 and 10, got %d", lease.Remaining)
		}
	})

	// 测试续约
	t.Run("KeepAliveLease", func(t *testing.T) {
		leaseID, err := engine.GrantLease(5) // 5秒TTL
		if err != nil {
			t.Fatalf("Failed to grant lease: %v", err)
		}

		// 等待1秒
		time.Sleep(1 * time.Second)

		// 续约
		ttl, err := engine.KeepAliveLease(leaseID)
		if err != nil {
			t.Fatalf("Failed to keep alive lease: %v", err)
		}

		if ttl != 5 {
			t.Errorf("Expected TTL 5, got %d", ttl)
		}

		// 获取租约信息验证续约
		lease, err := engine.GetLease(leaseID)
		if err != nil {
			t.Fatalf("Failed to get lease: %v", err)
		}

		if lease.Remaining <= 0 || lease.Remaining > 5 {
			t.Errorf("Expected remaining time between 0 and 5, got %d", lease.Remaining)
		}
	})

	// 测试撤销租约
	t.Run("RevokeLease", func(t *testing.T) {
		leaseID, err := engine.GrantLease(10)
		if err != nil {
			t.Fatalf("Failed to grant lease: %v", err)
		}

		// 撤销租约
		err = engine.RevokeLease(leaseID)
		if err != nil {
			t.Fatalf("Failed to revoke lease: %v", err)
		}

		// 尝试获取已撤销的租约
		_, err = engine.GetLease(leaseID)
		if err == nil {
			t.Error("Expected error when getting revoked lease")
		}
	})

	// 测试租约过期
	t.Run("LeaseExpiry", func(t *testing.T) {
		leaseID, err := engine.GrantLease(2) // 2秒TTL
		if err != nil {
			t.Fatalf("Failed to grant lease: %v", err)
		}

		// 设置键值
		err = engine.Put("expiry-key", []byte("expiry-value"), 0)
		if err != nil {
			t.Fatalf("Failed to put key: %v", err)
		}

		// 将键附加到租约
		err = engine.AttachToLease(leaseID, "expiry-key")
		if err != nil {
			t.Fatalf("Failed to attach key to lease: %v", err)
		}

		// 验证键存在
		kv, err := engine.Get("expiry-key")
		if err != nil {
			t.Fatalf("Failed to get key: %v", err)
		}

		if string(kv.Value) != "expiry-value" {
			t.Errorf("Expected 'expiry-value', got '%s'", string(kv.Value))
		}

		// 等待租约过期
		time.Sleep(3 * time.Second)

		// 验证键已过期
		_, err = engine.Get("expiry-key")
		if err != storage.ErrKeyNotFound && err != storage.ErrKeyExpired {
			t.Errorf("Expected key not found or expired error, got %v", err)
		}
	})

	// 测试列出租约
	t.Run("ListLeases", func(t *testing.T) {
		// 创建几个租约
		leaseIDs := make([]int64, 0)
		for i := 0; i < 3; i++ {
			leaseID, err := engine.GrantLease(30) // 30秒TTL
			if err != nil {
				t.Fatalf("Failed to grant lease %d: %v", i, err)
			}
			leaseIDs = append(leaseIDs, leaseID)
		}

		// 列出租约
		leases, err := engine.ListLeases()
		if err != nil {
			t.Fatalf("Failed to list leases: %v", err)
		}

		if len(leases) < 3 {
			t.Errorf("Expected at least 3 leases, got %d", len(leases))
		}

		// 验证租约ID
		leaseMap := make(map[int64]bool)
		for _, lease := range leases {
			leaseMap[lease.ID] = true
		}

		for _, leaseID := range leaseIDs {
			if !leaseMap[leaseID] {
				t.Errorf("Lease %d not found in list", leaseID)
			}
		}
	})

	// 测试租约计数
	t.Run("GetLeaseCount", func(t *testing.T) {
		initialCount := engine.GetLeaseCount()

		// 创建租约
		leaseID, err := engine.GrantLease(10)
		if err != nil {
			t.Fatalf("Failed to grant lease: %v", err)
		}

		// 验证计数增加
		if engine.GetLeaseCount() != initialCount+1 {
			t.Errorf("Expected lease count %d, got %d", initialCount+1, engine.GetLeaseCount())
		}

		// 撤销租约
		err = engine.RevokeLease(leaseID)
		if err != nil {
			t.Fatalf("Failed to revoke lease: %v", err)
		}

		// 验证计数减少
		if engine.GetLeaseCount() != initialCount {
			t.Errorf("Expected lease count %d, got %d", initialCount, engine.GetLeaseCount())
		}
	})
}

// TestLeaseWithTTL 测试带TTL的PUT操作与租约集成
func TestLeaseWithTTL(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "myetcd-test-lease-ttl")
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
	t.Run("PutWithTTL", func(t *testing.T) {
		// PUT带TTL的键
		err = engine.Put("ttl-key", []byte("ttl-value"), 3) // 3秒TTL
		if err != nil {
			t.Fatalf("Failed to put key with TTL: %v", err)
		}

		// 验证键存在
		kv, err := engine.Get("ttl-key")
		if err != nil {
			t.Fatalf("Failed to get key: %v", err)
		}

		if string(kv.Value) != "ttl-value" {
			t.Errorf("Expected 'ttl-value', got '%s'", string(kv.Value))
		}

		if kv.LeaseID <= 0 {
			t.Error("Expected positive lease ID")
		}

		// 验证租约存在
		lease, err := engine.GetLease(kv.LeaseID)
		if err != nil {
			t.Fatalf("Failed to get lease: %v", err)
		}

		if lease.TTL != 3 {
			t.Errorf("Expected TTL 3, got %d", lease.TTL)
		}

		// 验证键在租约中
		found := false
		for _, key := range lease.Keys {
			if key == "ttl-key" {
				found = true
				break
			}
		}

		if !found {
			t.Error("Key not found in lease")
		}

		// 等待键过期
		time.Sleep(4 * time.Second)

		// 验证键已过期
		_, err = engine.Get("ttl-key")
		if err != storage.ErrKeyNotFound && err != storage.ErrKeyExpired {
			t.Errorf("Expected key not found or expired error, got %v", err)
		}
	})
}
