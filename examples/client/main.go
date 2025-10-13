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

	// 获取服务器状态
	status, err := client.Status()
	if err != nil {
		log.Printf("Failed to get status: %v", err)
	} else {
		fmt.Printf("✓ Server status: %+v\n", status)
	}

	// 基本操作演示
	fmt.Println("\n=== Basic Operations ===")
	
	// PUT操作
	fmt.Println("Putting key1=value1...")
	if err := client.Put("key1", "value1", 0); err != nil {
		log.Printf("Failed to put key1: %v", err)
	} else {
		fmt.Println("✓ Successfully put key1")
	}

	// GET操作
	fmt.Println("Getting key1...")
	resp, err := client.Get("key1")
	if err != nil {
		log.Printf("Failed to get key1: %v", err)
	} else {
		fmt.Printf("✓ Got key1: %s (version: %d, created: %v)\n", resp.Value, resp.Version, resp.Created)
	}

	// PUT with TTL
	fmt.Println("Putting key2=value2 with TTL 5 seconds...")
	if err := client.Put("key2", "value2", 5); err != nil {
		log.Printf("Failed to put key2 with TTL: %v", err)
	} else {
		fmt.Println("✓ Successfully put key2 with TTL")
	}

	// 列出所有键
	fmt.Println("Listing all keys...")
	keys, err := client.ListKeys()
	if err != nil {
		log.Printf("Failed to list keys: %v", err)
	} else {
		fmt.Printf("✓ Keys: %v\n", keys)
	}

	// 事务演示
	fmt.Println("\n=== Transaction Demo ===")
	
	// 设置初始值
	fmt.Println("Setting txn-key=initial...")
	if err := client.Put("txn-key", "initial", 0); err != nil {
		log.Printf("Failed to set initial value: %v", err)
	}

	// 创建事务：如果值是"initial"，则更新为"updated"
	txnReq := types.TxnRequest{
		Compare: []types.Compare{
			{
				Key:   "txn-key",
				Op:    "=",
				Value: "initial",
			},
		},
		Success: []types.Request{
			{
				Op:    "PUT",
				Key:   "txn-key",
				Value: "updated",
			},
		},
		Failure: []types.Request{
			{
				Op:    "PUT",
				Key:   "txn-key",
				Value: "fallback",
			},
		},
	}

	fmt.Println("Executing transaction...")
	txnResp, err := client.Transaction(txnReq)
	if err != nil {
		log.Printf("Failed to execute transaction: %v", err)
	} else {
		fmt.Printf("✓ Transaction succeeded: %v\n", txnResp.Success)
		
		// 验证结果
		resp, err := client.Get("txn-key")
		if err != nil {
			log.Printf("Failed to get txn-key after transaction: %v", err)
		} else {
			fmt.Printf("✓ txn-key value after transaction: %s\n", resp.Value)
		}
	}

	// 复杂事务演示
	fmt.Println("\n=== Complex Transaction Demo ===")
	
	// 准备多个键
	client.Put("counter", "0", 0)
	client.Put("name", "alice", 0)

	// 创建复杂事务
	complexTxn := types.TxnRequest{
		Compare: []types.Compare{
			{
				Key:   "counter",
				Op:    "<",
				Value: "10",
			},
		},
		Success: []types.Request{
			{
				Op:    "PUT",
				Key:   "counter",
				Value: "1",
			},
			{
				Op:    "PUT",
				Key:   "name",
				Value: "bob",
			},
			{
				Op:    "PUT",
				Key:   "timestamp",
				Value: fmt.Sprintf("%d", time.Now().Unix()),
			},
		},
		Failure: []types.Request{
			{
				Op:    "PUT",
				Key:   "status",
				Value: "counter_too_high",
			},
		},
	}

	fmt.Println("Executing complex transaction...")
	complexResp, err := client.Transaction(complexTxn)
	if err != nil {
		log.Printf("Failed to execute complex transaction: %v", err)
	} else {
		fmt.Printf("✓ Complex transaction succeeded: %v\n", complexResp.Success)
		
		// 显示所有键的当前状态
		fmt.Println("Current key values:")
		allKeys := []string{"counter", "name", "timestamp", "status"}
		for _, key := range allKeys {
			resp, err := client.Get(key)
			if err != nil {
				fmt.Printf("  %s: <not found>\n", key)
			} else {
				fmt.Printf("  %s: %s\n", key, resp.Value)
			}
		}
	}

	// TTL演示
	fmt.Println("\n=== TTL Demo ===")
	
	// 检查key2是否还存在
	fmt.Println("Checking key2 (should still exist)...")
	resp2, err := client.Get("key2")
	if err != nil {
		fmt.Printf("key2: %v\n", err)
	} else {
		fmt.Printf("✓ key2 still exists: %s\n", resp2.Value)
	}

	// 等待TTL过期
	fmt.Println("Waiting for key2 to expire...")
	time.Sleep(6 * time.Second)

	// 再次检查key2
	fmt.Println("Checking key2 again (should be expired)...")
	resp2, err = client.Get("key2")
	if err != nil {
		fmt.Printf("✓ key2 expired as expected: %v\n", err)
	} else {
		fmt.Printf("❌ key2 still exists: %s\n", resp2.Value)
	}

	// 清理演示
	fmt.Println("\n=== Cleanup Demo ===")
	
	// 列出所有键
	keys, _ = client.ListKeys()
	fmt.Printf("Keys before cleanup: %v\n", keys)

	// 删除一些键
	for _, key := range []string{"key1", "txn-key", "counter", "name", "timestamp"} {
		fmt.Printf("Deleting %s...\n", key)
		if err := client.Delete(key); err != nil {
			log.Printf("Failed to delete %s: %v", key, err)
		} else {
			fmt.Printf("✓ Deleted %s\n", key)
		}
	}

	// 最终状态
	keys, _ = client.ListKeys()
	fmt.Printf("Keys after cleanup: %v\n", keys)

	fmt.Println("\n=== Demo Complete ===")
}
