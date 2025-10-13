package main

import (
	"fmt"
	"log"
	"time"

	"myetcd/internal/api"
	"myetcd/internal/types"
)

func main() {
	// 创建客户端
	client := api.NewClient("localhost:2379")

	// 测试连接
	fmt.Println("Testing connection to MyEtcd server...")
	if err := client.Health(); err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	fmt.Println("✓ Connection successful")

	// Watch演示
	fmt.Println("\n=== Watch Demo ===")

	// 1. 单个键的Watch
	fmt.Println("Creating watch for key 'watch-key'...")
	watchID, eventChan, err := client.Watch("watch-key", false, true)
	if err != nil {
		log.Fatalf("Failed to create watch: %v", err)
	}
	fmt.Printf("✓ Watch created with ID: %d\n", watchID)

	// 启动事件处理goroutine
	go func() {
		for event := range eventChan {
			fmt.Printf("Watch Event: Type=%s, Key=%s, Value=%s, Version=%d\n",
				event.Type, event.Key, string(event.Value), event.Version)
			if len(event.PrevValue) > 0 {
				fmt.Printf("  Previous Value: %s\n", string(event.PrevValue))
			}
		}
		fmt.Println("Watch channel closed")
	}()

	// 等待Watch设置完成
	time.Sleep(100 * time.Millisecond)

	// 执行一些操作来触发Watch事件
	fmt.Println("\nPerforming operations to trigger watch events...")

	// PUT操作
	fmt.Println("Putting watch-key=value1...")
	if err := client.Put("watch-key", "value1", 0); err != nil {
		log.Printf("Failed to put key: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// 更新操作
	fmt.Println("Updating watch-key=value2...")
	if err := client.Put("watch-key", "value2", 0); err != nil {
		log.Printf("Failed to put key: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// DELETE操作
	fmt.Println("Deleting watch-key...")
	if err := client.Delete("watch-key"); err != nil {
		log.Printf("Failed to delete key: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// 取消Watch
	fmt.Printf("Cancelling watch %d...\n", watchID)
	if err := client.CancelWatch(watchID); err != nil {
		log.Printf("Failed to cancel watch: %v", err)
	}

	// 2. 前缀Watch演示
	fmt.Println("\n=== Prefix Watch Demo ===")

	fmt.Println("Creating prefix watch for 'demo-'...")
	prefixWatchID, prefixEventChan, err := client.Watch("demo-", true, false)
	if err != nil {
		log.Fatalf("Failed to create prefix watch: %v", err)
	}
	fmt.Printf("✓ Prefix watch created with ID: %d\n", prefixWatchID)

	// 启动前缀Watch事件处理goroutine
	go func() {
		for event := range prefixEventChan {
			fmt.Printf("Prefix Watch Event: Type=%s, Key=%s, Value=%s\n",
				event.Type, event.Key, string(event.Value))
		}
		fmt.Println("Prefix watch channel closed")
	}()

	// 等待Watch设置完成
	time.Sleep(100 * time.Millisecond)

	// 创建多个键来触发前缀Watch
	fmt.Println("\nCreating multiple keys with 'demo-' prefix...")
	demoKeys := []string{"demo-key1", "demo-key2", "demo-key3"}
	for i, key := range demoKeys {
		value := fmt.Sprintf("demo-value%d", i+1)
		fmt.Printf("Putting %s=%s...\n", key, value)
		if err := client.Put(key, value, 0); err != nil {
			log.Printf("Failed to put key %s: %v", key, err)
		}
		time.Sleep(200 * time.Millisecond)
	}

	// 删除一些键
	fmt.Println("\nDeleting some demo keys...")
	for _, key := range demoKeys[:2] {
		fmt.Printf("Deleting %s...\n", key)
		if err := client.Delete(key); err != nil {
			log.Printf("Failed to delete key %s: %v", key, err)
		}
		time.Sleep(200 * time.Millisecond)
	}

	// 取消前缀Watch
	fmt.Printf("Cancelling prefix watch %d...\n", prefixWatchID)
	if err := client.CancelWatch(prefixWatchID); err != nil {
		log.Printf("Failed to cancel prefix watch: %v", err)
	}

	// 3. 范围查询演示
	fmt.Println("\n=== Range Query Demo ===")

	// 准备一些测试数据
	fmt.Println("Preparing test data for range query...")
	rangeKeys := []string{"a", "b", "c", "d", "e", "f"}
	for _, key := range rangeKeys {
		value := fmt.Sprintf("range-value-%s", key)
		if err := client.Put(key, value, 0); err != nil {
			log.Printf("Failed to put key %s: %v", key, err)
		}
	}

	// 单个键查询
	fmt.Println("\nQuerying single key 'c'...")
	kvs, count, err := client.Range("c", "", 0)
	if err != nil {
		log.Printf("Failed to range query: %v", err)
	} else {
		fmt.Printf("Found %d keys:\n", count)
		for _, kv := range kvs {
			fmt.Printf("  %s: %s (version: %d)\n", kv.Key, kv.Value, kv.Version)
		}
	}

	// 范围查询
	fmt.Println("\nQuerying range from 'b' to 'e'...")
	kvs, count, err = client.Range("b", "e", 0)
	if err != nil {
		log.Printf("Failed to range query: %v", err)
	} else {
		fmt.Printf("Found %d keys in range:\n", count)
		for _, kv := range kvs {
			fmt.Printf("  %s: %s (version: %d)\n", kv.Key, kv.Value, kv.Version)
		}
	}

	// 限制查询
	fmt.Println("\nQuerying with limit 3...")
	kvs, count, err = client.Range("a", "z", 3)
	if err != nil {
		log.Printf("Failed to range query: %v", err)
	} else {
		fmt.Printf("Found %d keys (showing first 3):\n", count)
		for _, kv := range kvs {
			fmt.Printf("  %s: %s (version: %d)\n", kv.Key, kv.Value, kv.Version)
		}
	}

	// 4. 批量操作演示
	fmt.Println("\n=== Batch Operations Demo ===")

	// 批量PUT
	fmt.Println("Performing batch PUT...")
	batchOps := []types.Request{
		{Op: "PUT", Key: "batch1", Value: "batch-value1"},
		{Op: "PUT", Key: "batch2", Value: "batch-value2"},
		{Op: "PUT", Key: "batch3", Value: "batch-value3"},
	}

	batchResp, err := client.Batch(batchOps)
	if err != nil {
		log.Printf("Failed to execute batch: %v", err)
	} else {
		fmt.Printf("Batch operation completed: Success=%v\n", batchResp.Success)
		if len(batchResp.Responses) > 0 {
			fmt.Println("Batch responses:")
			for _, resp := range batchResp.Responses {
				fmt.Printf("  %v\n", resp)
			}
		}
	}

	// 批量DELETE
	fmt.Println("\nPerforming batch DELETE...")
	deleteOps := []types.Request{
		{Op: "DELETE", Key: "batch1"},
		{Op: "DELETE", Key: "batch3"},
	}

	deleteResp, err := client.Batch(deleteOps)
	if err != nil {
		log.Printf("Failed to execute batch delete: %v", err)
	} else {
		fmt.Printf("Batch delete completed: Success=%v\n", deleteResp.Success)
	}

	// 验证批量操作结果
	fmt.Println("\nVerifying batch operations...")
	verifyKeys := []string{"batch1", "batch2", "batch3"}
	for _, key := range verifyKeys {
		resp, err := client.Get(key)
		if err != nil {
			fmt.Printf("  %s: <not found>\n", key)
		} else {
			fmt.Printf("  %s: %s\n", key, resp.Value)
		}
	}

	// 清理演示数据
	fmt.Println("\n=== Cleanup ===")
	cleanupKeys := append([]string{"c", "d", "e", "f"}, rangeKeys...)
	for _, key := range cleanupKeys {
		if err := client.Delete(key); err != nil {
			log.Printf("Failed to delete key %s: %v", key, err)
		}
	}

	// 清理剩余的batch键
	client.Delete("batch2")

	// 清理剩余的demo键
	for _, key := range demoKeys {
		client.Delete(key)
	}

	fmt.Println("\n=== Watch Demo Complete ===")
	time.Sleep(1 * time.Second) // 等待所有事件处理完成
}
