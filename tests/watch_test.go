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

// TestWatch 测试Watch功能
func TestWatch(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "myetcd-test-watch")
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

	// 测试单个键的Watch
	t.Run("SingleKeyWatch", func(t *testing.T) {
		// 创建Watch
		watchID, eventChan, err := engine.Watch("watch-key", false, true)
		if err != nil {
			t.Fatalf("Failed to create watch: %v", err)
		}

		// PUT操作
		err = engine.Put("watch-key", []byte("value1"), 0)
		if err != nil {
			t.Fatalf("Failed to put key: %v", err)
		}

		// 等待事件
		select {
		case event := <-eventChan:
			if event.Type != types.WatchPut {
				t.Errorf("Expected PUT event, got %v", event.Type)
			}
			if event.Key != "watch-key" {
				t.Errorf("Expected key 'watch-key', got '%s'", event.Key)
			}
			if string(event.Value) != "value1" {
				t.Errorf("Expected value 'value1', got '%s'", string(event.Value))
			}
		case <-time.After(1 * time.Second):
			t.Error("Timeout waiting for watch event")
		}

		// 取消Watch
		err = engine.CancelWatch(watchID)
		if err != nil {
			t.Errorf("Failed to cancel watch: %v", err)
		}
	})

	// 测试前缀Watch
	t.Run("PrefixWatch", func(t *testing.T) {
		// 创建前缀Watch
		watchID, eventChan, err := engine.Watch("prefix-", true, true)
		if err != nil {
			t.Fatalf("Failed to create prefix watch: %v", err)
		}

		// PUT操作
		err = engine.Put("prefix-key1", []byte("value1"), 0)
		if err != nil {
			t.Fatalf("Failed to put key: %v", err)
		}

		// 等待事件
		select {
		case event := <-eventChan:
			if event.Type != types.WatchPut {
				t.Errorf("Expected PUT event, got %v", event.Type)
			}
			if event.Key != "prefix-key1" {
				t.Errorf("Expected key 'prefix-key1', got '%s'", event.Key)
			}
		case <-time.After(1 * time.Second):
			t.Error("Timeout waiting for watch event")
		}

		// 另一个PUT操作
		err = engine.Put("prefix-key2", []byte("value2"), 0)
		if err != nil {
			t.Fatalf("Failed to put key: %v", err)
		}

		// 等待事件
		select {
		case event := <-eventChan:
			if event.Type != types.WatchPut {
				t.Errorf("Expected PUT event, got %v", event.Type)
			}
			if event.Key != "prefix-key2" {
				t.Errorf("Expected key 'prefix-key2', got '%s'", event.Key)
			}
		case <-time.After(1 * time.Second):
			t.Error("Timeout waiting for watch event")
		}

		// DELETE操作
		err = engine.Delete("prefix-key1")
		if err != nil {
			t.Fatalf("Failed to delete key: %v", err)
		}

		// 等待事件
		select {
		case event := <-eventChan:
			if event.Type != types.WatchDelete {
				t.Errorf("Expected DELETE event, got %v", event.Type)
			}
			if event.Key != "prefix-key1" {
				t.Errorf("Expected key 'prefix-key1', got '%s'", event.Key)
			}
		case <-time.After(1 * time.Second):
			t.Error("Timeout waiting for watch event")
		}

		// 取消Watch
		err = engine.CancelWatch(watchID)
		if err != nil {
			t.Errorf("Failed to cancel watch: %v", err)
		}
	})

	// 测试多个Watch
	t.Run("MultipleWatches", func(t *testing.T) {
		// 创建多个Watch
		watchID1, eventChan1, err := engine.Watch("multi-key", false, false)
		if err != nil {
			t.Fatalf("Failed to create watch1: %v", err)
		}

		watchID2, eventChan2, err := engine.Watch("multi-key", false, true)
		if err != nil {
			t.Fatalf("Failed to create watch2: %v", err)
		}

		// PUT操作
		err = engine.Put("multi-key", []byte("multi-value"), 0)
		if err != nil {
			t.Fatalf("Failed to put key: %v", err)
		}

		// 两个Watch都应该收到事件
		select {
		case event := <-eventChan1:
			if event.Type != types.WatchPut {
				t.Errorf("Watch1: Expected PUT event, got %v", event.Type)
			}
		case <-time.After(1 * time.Second):
			t.Error("Watch1: Timeout waiting for watch event")
		}

		select {
		case event := <-eventChan2:
			if event.Type != types.WatchPut {
				t.Errorf("Watch2: Expected PUT event, got %v", event.Type)
			}
			if len(event.PrevValue) != 0 {
				t.Errorf("Watch2: Expected empty prev value, got '%s'", string(event.PrevValue))
			}
		case <-time.After(1 * time.Second):
			t.Error("Watch2: Timeout waiting for watch event")
		}

		// 取消Watch
		err = engine.CancelWatch(watchID1)
		if err != nil {
			t.Errorf("Failed to cancel watch1: %v", err)
		}

		err = engine.CancelWatch(watchID2)
		if err != nil {
			t.Errorf("Failed to cancel watch2: %v", err)
		}
	})
}

// TestRange 测试范围查询功能
func TestRange(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "myetcd-test-range")
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

	// 准备测试数据
	testKeys := []string{"a", "b", "c", "d", "e"}
	testValues := []string{"value1", "value2", "value3", "value4", "value5"}

	for i, key := range testKeys {
		err = engine.Put(key, []byte(testValues[i]), 0)
		if err != nil {
			t.Fatalf("Failed to put key %s: %v", key, err)
		}
	}

	// 测试单个键查询
	t.Run("SingleKey", func(t *testing.T) {
		kvs, count, err := engine.Range("b", "", 0)
		if err != nil {
			t.Fatalf("Failed to range query: %v", err)
		}

		if count != 1 {
			t.Errorf("Expected count 1, got %d", count)
		}

		if len(kvs) != 1 {
			t.Errorf("Expected 1 key-value, got %d", len(kvs))
		}

		if kvs[0].Key != "b" {
			t.Errorf("Expected key 'b', got '%s'", kvs[0].Key)
		}

		if string(kvs[0].Value) != "value2" {
			t.Errorf("Expected value 'value2', got '%s'", string(kvs[0].Value))
		}
	})

	// 测试范围查询
	t.Run("RangeQuery", func(t *testing.T) {
		kvs, count, err := engine.Range("b", "e", 0)
		if err != nil {
			t.Fatalf("Failed to range query: %v", err)
		}

		if count != 3 {
			t.Errorf("Expected count 3, got %d", count)
		}

		if len(kvs) != 3 {
			t.Errorf("Expected 3 key-values, got %d", len(kvs))
		}

		expectedKeys := []string{"b", "c", "d"}
		for i, kv := range kvs {
			if kv.Key != expectedKeys[i] {
				t.Errorf("Expected key '%s', got '%s'", expectedKeys[i], kv.Key)
			}
		}
	})

	// 测试限制查询
	t.Run("LimitedQuery", func(t *testing.T) {
		kvs, count, err := engine.Range("a", "z", 2)
		if err != nil {
			t.Fatalf("Failed to range query: %v", err)
		}

		if count != 5 {
			t.Errorf("Expected total count 5, got %d", count)
		}

		if len(kvs) != 2 {
			t.Errorf("Expected 2 key-values, got %d", len(kvs))
		}
	})
}

// TestBatch 测试批量操作功能
func TestBatch(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "myetcd-test-batch")
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

	// 测试批量PUT
	t.Run("BatchPut", func(t *testing.T) {
		kvs := map[string][]byte{
			"batch1": []byte("value1"),
			"batch2": []byte("value2"),
			"batch3": []byte("value3"),
		}

		err = engine.BatchPut(kvs, 0)
		if err != nil {
			t.Fatalf("Failed to batch put: %v", err)
		}

		// 验证数据
		for key, expectedValue := range kvs {
			kv, err := engine.Get(key)
			if err != nil {
				t.Errorf("Failed to get key %s: %v", key, err)
				continue
			}

			if string(kv.Value) != string(expectedValue) {
				t.Errorf("Expected value '%s', got '%s'", string(expectedValue), string(kv.Value))
			}
		}
	})

	// 测试批量DELETE
	t.Run("BatchDelete", func(t *testing.T) {
		keys := []string{"batch1", "batch3"}

		err = engine.BatchDelete(keys)
		if err != nil {
			t.Fatalf("Failed to batch delete: %v", err)
		}

		// 验证删除
		for _, key := range keys {
			_, err := engine.Get(key)
			if err != storage.ErrKeyNotFound {
				t.Errorf("Expected key %s to be deleted", key)
			}
		}

		// 验证未删除的键
		kv, err := engine.Get("batch2")
		if err != nil {
			t.Errorf("Failed to get key batch2: %v", err)
		} else {
			if string(kv.Value) != "value2" {
				t.Errorf("Expected value 'value2', got '%s'", string(kv.Value))
			}
		}
	})
}
