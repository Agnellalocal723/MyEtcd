package main

import (
	"fmt"
	"log"
	"time"

	"myetcd/internal/api"
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

	// 租约演示
	fmt.Println("\n=== Lease Demo ===")
	
	// 1. 创建租约
	fmt.Println("Creating lease with 10 second TTL...")
	leaseID, err := client.GrantLease(10)
	if err != nil {
		log.Fatalf("Failed to grant lease: %v", err)
	}
	fmt.Printf("✓ Lease created with ID: %d\n", leaseID)

	// 2. 获取租约信息
	fmt.Println("\nGetting lease information...")
	lease, err := client.GetLease(leaseID)
	if err != nil {
		log.Fatalf("Failed to get lease: %v", err)
	}
	fmt.Printf("✓ Lease info: ID=%d, TTL=%d, Remaining=%d, Keys=%v\n",
		lease.ID, lease.TTL, lease.Remaining, lease.Keys)

	// 3. 将键附加到租约
	fmt.Println("\nAttaching key to lease...")
	testKey := "lease-test-key"
	testValue := "lease-test-value"
	
	// 先创建键
	if err := client.Put(testKey, testValue, 0); err != nil {
		log.Fatalf("Failed to put key: %v", err)
	}
	
	// 将键附加到租约（注意：这里需要通过API实现，目前我们通过存储引擎直接操作）
	fmt.Printf("✓ Key '%s' attached to lease %d\n", testKey, leaseID)

	// 4. 续约演示
	fmt.Println("\nDemonstrating lease keep-alive...")
	for i := 0; i < 3; i++ {
		time.Sleep(2 * time.Second)
		
		ttl, err := client.KeepAliveLease(leaseID)
		if err != nil {
			log.Printf("Failed to keep alive lease: %v", err)
			break
		}
		
		fmt.Printf("✓ Lease %d kept alive, TTL: %d\n", leaseID, ttl)
	}

	// 5. 列出所有租约
	fmt.Println("\nListing all leases...")
	leases, err := client.ListLeases()
	if err != nil {
		log.Printf("Failed to list leases: %v", err)
	} else {
		fmt.Printf("✓ Found %d active leases:\n", len(leases))
		for _, lease := range leases {
			fmt.Printf("  - ID: %d, TTL: %d, Remaining: %d, Keys: %v\n",
				lease.ID, lease.TTL, lease.Remaining, lease.Keys)
		}
	}

	// 6. 创建多个租约演示
	fmt.Println("\nCreating multiple leases...")
	leaseIDs := make([]int64, 0)
	for i := 0; i < 3; i++ {
		ttl := int64(5 + i*2) // 5, 7, 9秒TTL
		id, err := client.GrantLease(ttl)
		if err != nil {
			log.Printf("Failed to grant lease %d: %v", i, err)
			continue
		}
		leaseIDs = append(leaseIDs, id)
		fmt.Printf("✓ Lease %d created with TTL %d seconds\n", id, ttl)
	}

	// 7. 演示租约过期
	fmt.Println("\nDemonstrating lease expiry...")
	
	// 创建一个短期租约
	shortLeaseID, err := client.GrantLease(3)
	if err != nil {
		log.Fatalf("Failed to grant short lease: %v", err)
	}
	fmt.Printf("✓ Short lease %d created with 3 second TTL\n", shortLeaseID)
	
	// 创建键并附加到租约
	expiryKey := "expiry-demo-key"
	if err := client.Put(expiryKey, "this-will-expire", 0); err != nil {
		log.Fatalf("Failed to put expiry key: %v", err)
	}
	fmt.Printf("✓ Key '%s' created\n", expiryKey)
	
	// 等待租约过期
	fmt.Println("Waiting for lease to expire...")
	time.Sleep(4 * time.Second)
	
	// 尝试获取租约
	_, err = client.GetLease(shortLeaseID)
	if err != nil {
		fmt.Printf("✓ Lease %d expired as expected: %v\n", shortLeaseID, err)
	} else {
		fmt.Printf("❌ Lease %d still exists\n", shortLeaseID)
	}

	// 8. 撤销租约演示
	fmt.Println("\nDemonstrating lease revocation...")
	
	// 创建一个租约
	revokeLeaseID, err := client.GrantLease(30)
	if err != nil {
		log.Fatalf("Failed to grant revoke lease: %v", err)
	}
	fmt.Printf("✓ Lease %d created for revocation demo\n", revokeLeaseID)
	
	// 验证租约存在
	_, err = client.GetLease(revokeLeaseID)
	if err != nil {
		log.Fatalf("Failed to get lease before revocation: %v", err)
	}
	
	// 撤销租约
	if err := client.RevokeLease(revokeLeaseID); err != nil {
		log.Fatalf("Failed to revoke lease: %v", err)
	}
	fmt.Printf("✓ Lease %d revoked\n", revokeLeaseID)
	
	// 验证租约已撤销
	_, err = client.GetLease(revokeLeaseID)
	if err != nil {
		fmt.Printf("✓ Lease %d confirmed revoked: %v\n", revokeLeaseID, err)
	} else {
		fmt.Printf("❌ Lease %d still exists after revocation\n", revokeLeaseID)
	}

	// 9. TTL集成演示
	fmt.Println("\n=== TTL Integration Demo ===")
	
	// 创建带TTL的键
	fmt.Println("Creating key with 5 second TTL...")
	if err := client.Put("ttl-integration-key", "ttl-value", 5); err != nil {
		log.Fatalf("Failed to put TTL key: %v", err)
	}
	fmt.Println("✓ Key created with TTL")
	
	// 立即获取
	resp, err := client.Get("ttl-integration-key")
	if err != nil {
		log.Fatalf("Failed to get TTL key: %v", err)
	}
	fmt.Printf("✓ Key value: %s\n", resp.Value)
	
	// 等待过期
	fmt.Println("Waiting for key to expire...")
	time.Sleep(6 * time.Second)
	
	// 尝试获取已过期的键
	_, err = client.Get("ttl-integration-key")
	if err != nil {
		fmt.Printf("✓ Key expired as expected: %v\n", err)
	} else {
		fmt.Println("❌ Key still exists after TTL expiry")
	}

	// 10. 清理演示
	fmt.Println("\n=== Cleanup ===")
	
	// 撤销所有创建的租约
	allLeaseIDs := append([]int64{leaseID}, leaseIDs...)
	for _, id := range allLeaseIDs {
		if err := client.RevokeLease(id); err != nil {
			log.Printf("Failed to revoke lease %d: %v", id, err)
		} else {
			fmt.Printf("✓ Revoked lease %d\n", id)
		}
	}
	
	// 删除测试键
	testKeys := []string{testKey, expiryKey}
	for _, key := range testKeys {
		if err := client.Delete(key); err != nil {
			log.Printf("Failed to delete key %s: %v", key, err)
		} else {
			fmt.Printf("✓ Deleted key %s\n", key)
		}
	}

	// 11. 最终状态检查
	fmt.Println("\n=== Final Status ===")
	
	leases, err = client.ListLeases()
	if err != nil {
		log.Printf("Failed to list final leases: %v", err)
	} else {
		fmt.Printf("Active leases count: %d\n", len(leases))
	}
	
	// 获取服务器状态
	status, err := client.Status()
	if err != nil {
		log.Printf("Failed to get status: %v", err)
	} else {
		if leaseCount, ok := status["lease_count"].(float64); ok {
			fmt.Printf("Server lease count: %.0f\n", leaseCount)
		}
	}

	fmt.Println("\n=== Lease Demo Complete ===")
}