package tests

import (
	"fmt"
	"testing"
	"time"

	"myetcd/internal/raft"
	"myetcd/internal/types"
)

/*
Raft算法测试

这个测试文件验证Raft算法的正确性，包括：
1. 领导选举：确保集群能够选举出唯一的领导者
2. 日志复制：确保领导者能够将日志复制到跟随者
3. 故障恢复：确保节点故障后能够正确恢复
4. 一致性：确保所有节点最终达到一致状态

测试策略：
- 使用模拟的网络环境
- 创建多个节点组成的集群
- 模拟各种故障场景
- 验证系统的一致性和可用性
*/

// TestRaftLeaderElection 测试领导选举
func TestRaftLeaderElection(t *testing.T) {
	// 创建3节点集群
	cluster, err := raft.CreateTestCluster(3)
	if err != nil {
		t.Fatalf("Failed to create test cluster: %v", err)
	}
	defer cluster.Stop()
	
	// 启动集群
	if err := cluster.Start(); err != nil {
		t.Fatalf("Failed to start cluster: %v", err)
	}
	
	// 等待领导者选举
	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}
	
	if leader == "" {
		t.Fatal("No leader elected")
	}
	
	t.Logf("Leader elected: %s", leader)
	
	// 验证只有一个领导者
	leaderCount := 0
	nodes := cluster.GetNodes()
	for nodeID, node := range nodes {
		state, _ := node.GetState()
		if state == types.Leader {
			leaderCount++
			if nodeID != leader {
				t.Errorf("Multiple leaders detected: %s and %s", leader, nodeID)
			}
		}
	}
	
	if leaderCount != 1 {
		t.Errorf("Expected 1 leader, got %d", leaderCount)
	}
}

// TestRaftLogReplication 测试日志复制
func TestRaftLogReplication(t *testing.T) {
	// 创建3节点集群
	cluster, err := raft.CreateTestCluster(3)
	if err != nil {
		t.Fatalf("Failed to create test cluster: %v", err)
	}
	defer cluster.Stop()
	
	// 启动集群
	if err := cluster.Start(); err != nil {
		t.Fatalf("Failed to start cluster: %v", err)
	}
	
	// 等待领导者选举
	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}
	
	t.Logf("Leader elected: %s", leader)
	
	// 确保领导者已经准备好
	time.Sleep(1 * time.Second)
	
	// 再次检查领导者
	leader = cluster.GetLeader()
	t.Logf("Current leader after sleep: %s", leader)
	
	// 提议几个命令
	for i := 1; i <= 5; i++ {
		key := fmt.Sprintf("key%d", i)
		value := []byte(fmt.Sprintf("value%d", i))
		
		cmd := types.Command{
			Type:  types.CommandPut,
			Key:   key,
			Value: value,
		}
		
		t.Logf("About to propose command %d: %s = %s", i, key, string(value))
		
		// 等待一小段时间，确保领导者已经准备好
		time.Sleep(100 * time.Millisecond)
		
		if err := cluster.Propose(cmd); err != nil {
			t.Logf("Failed to propose command %d: %v", i, err)
			// 尝试获取当前领导者信息
			leader := cluster.GetLeader()
			t.Logf("Current leader: %s", leader)
			// 打印集群状态
			status := cluster.GetClusterStatus()
			t.Logf("Cluster status: %+v", status)
			continue
		}
		
		t.Logf("Successfully proposed command %d: %s = %s", i, key, string(value))
		
		// 等待一小段时间，确保命令被处理
		time.Sleep(100 * time.Millisecond)
	}
	
	// 等待日志复制
	time.Sleep(2 * time.Second)
	
	// 验证所有节点都有相同的数据
	nodes := cluster.GetNodes()
	expectedData := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
		"key4": "value4",
		"key5": "value5",
	}
	
	for nodeID := range nodes {
		sm := cluster.GetStateMachine(nodeID)
		if sm == nil {
			t.Fatalf("State machine not found for node %s", nodeID)
		}
		
		for key, expectedValue := range expectedData {
			value, err := sm.Get(key)
			if err != nil {
				t.Errorf("Node %s: key %s not found: %v", nodeID, key, err)
				continue
			}
			
			if string(value) != expectedValue {
				t.Errorf("Node %s: key %s has wrong value. Expected %s, got %s", 
					nodeID, key, expectedValue, string(value))
			}
		}
	}
	
	t.Log("Log replication test passed")
}

// TestRaftNodeFailure 测试节点故障
func TestRaftNodeFailure(t *testing.T) {
	// 创建3节点集群
	cluster, err := raft.CreateTestCluster(3)
	if err != nil {
		t.Fatalf("Failed to create test cluster: %v", err)
	}
	defer cluster.Stop()
	
	// 启动集群
	if err := cluster.Start(); err != nil {
		t.Fatalf("Failed to start cluster: %v", err)
	}
	
	// 等待领导者选举
	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}
	
	t.Logf("Initial leader: %s", leader)
	
	// 提议一些命令
	for i := 1; i <= 3; i++ {
		key := fmt.Sprintf("key%d", i)
		value := []byte(fmt.Sprintf("value%d", i))
		
		cmd := types.Command{
			Type:  types.CommandPut,
			Key:   key,
			Value: value,
		}
		
		if err := cluster.Propose(cmd); err != nil {
			t.Fatalf("Failed to propose command %d: %v", i, err)
		}
	}
	
	// 等待日志复制
	time.Sleep(1 * time.Second)
	
	// 选择一个非领导者节点停止
	var nodeToStop string
	for nodeID := range cluster.GetNodes() {
		if nodeID != leader {
			nodeToStop = nodeID
			break
		}
	}
	
	if nodeToStop == "" {
		t.Fatal("No non-leader node found to stop")
	}
	
	t.Logf("Stopping node %s", nodeToStop)
	if err := cluster.RemoveNode(nodeToStop); err != nil {
		t.Fatalf("Failed to remove node %s: %v", nodeToStop, err)
	}
	
	// 等待集群稳定
	time.Sleep(2 * time.Second)
	
	// 验证集群仍然可用
	newLeader := cluster.GetLeader()
	if newLeader == "" {
		t.Fatal("No leader after node failure")
	}
	
	t.Logf("New leader after node failure: %s", newLeader)
	
	// 提议更多命令
	for i := 4; i <= 6; i++ {
		key := fmt.Sprintf("key%d", i)
		value := []byte(fmt.Sprintf("value%d", i))
		
		cmd := types.Command{
			Type:  types.CommandPut,
			Key:   key,
			Value: value,
		}
		
		if err := cluster.Propose(cmd); err != nil {
			t.Fatalf("Failed to propose command %d after node failure: %v", i, err)
		}
	}
	
	// 等待日志复制
	time.Sleep(2 * time.Second)
	
	// 验证剩余节点有一致的数据
	expectedData := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
		"key4": "value4",
		"key5": "value5",
		"key6": "value6",
	}
	
	for nodeID := range cluster.GetNodes() {
		sm := cluster.GetStateMachine(nodeID)
		if sm == nil {
			continue
		}
		
		for key, expectedValue := range expectedData {
			value, err := sm.Get(key)
			if err != nil {
				t.Errorf("Node %s: key %s not found: %v", nodeID, key, err)
				continue
			}
			
			if string(value) != expectedValue {
				t.Errorf("Node %s: key %s has wrong value. Expected %s, got %s", 
					nodeID, key, expectedValue, string(value))
			}
		}
	}
	
	t.Log("Node failure test passed")
}

// TestRaftLeaderFailure 测试领导者故障
func TestRaftLeaderFailure(t *testing.T) {
	// 创建3节点集群
	cluster, err := raft.CreateTestCluster(3)
	if err != nil {
		t.Fatalf("Failed to create test cluster: %v", err)
	}
	defer cluster.Stop()
	
	// 启动集群
	if err := cluster.Start(); err != nil {
		t.Fatalf("Failed to start cluster: %v", err)
	}
	
	// 等待领导者选举
	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}
	
	t.Logf("Initial leader: %s", leader)
	
	// 提议一些命令
	for i := 1; i <= 3; i++ {
		key := fmt.Sprintf("key%d", i)
		value := []byte(fmt.Sprintf("value%d", i))
		
		cmd := types.Command{
			Type:  types.CommandPut,
			Key:   key,
			Value: value,
		}
		
		if err := cluster.Propose(cmd); err != nil {
			t.Fatalf("Failed to propose command %d: %v", i, err)
		}
	}
	
	// 等待日志复制
	time.Sleep(1 * time.Second)
	
	// 停止领导者节点
	t.Logf("Stopping leader node %s", leader)
	if err := cluster.RemoveNode(leader); err != nil {
		t.Fatalf("Failed to remove leader node %s: %v", leader, err)
	}
	
	// 等待新的领导者选举
	time.Sleep(3 * time.Second)
	
	// 验证新领导者被选出
	newLeader := cluster.GetLeader()
	if newLeader == "" {
		t.Fatal("No new leader elected after leader failure")
	}
	
	if newLeader == leader {
		t.Fatal("Old leader still detected after failure")
	}
	
	t.Logf("New leader elected: %s", newLeader)
	
	// 验证集群仍然可用
	for i := 4; i <= 6; i++ {
		key := fmt.Sprintf("key%d", i)
		value := []byte(fmt.Sprintf("value%d", i))
		
		cmd := types.Command{
			Type:  types.CommandPut,
			Key:   key,
			Value: value,
		}
		
		if err := cluster.Propose(cmd); err != nil {
			t.Fatalf("Failed to propose command %d after leader failure: %v", i, err)
		}
	}
	
	// 等待日志复制
	time.Sleep(2 * time.Second)
	
	// 验证剩余节点有一致的数据
	expectedData := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
		"key4": "value4",
		"key5": "value5",
		"key6": "value6",
	}
	
	for nodeID := range cluster.GetNodes() {
		sm := cluster.GetStateMachine(nodeID)
		if sm == nil {
			continue
		}
		
		for key, expectedValue := range expectedData {
			value, err := sm.Get(key)
			if err != nil {
				t.Errorf("Node %s: key %s not found: %v", nodeID, key, err)
				continue
			}
			
			if string(value) != expectedValue {
				t.Errorf("Node %s: key %s has wrong value. Expected %s, got %s", 
					nodeID, key, expectedValue, string(value))
			}
		}
	}
	
	t.Log("Leader failure test passed")
}

// TestRaftConsistency 测试一致性
func TestRaftConsistency(t *testing.T) {
	// 创建5节点集群
	cluster, err := raft.CreateTestCluster(5)
	if err != nil {
		t.Fatalf("Failed to create test cluster: %v", err)
	}
	defer cluster.Stop()
	
	// 启动集群
	if err := cluster.Start(); err != nil {
		t.Fatalf("Failed to start cluster: %v", err)
	}
	
	// 等待领导者选举
	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}
	
	t.Logf("Leader elected: %s", leader)
	
	// 并发提议多个命令
	cmdCount := 20
	done := make(chan bool, cmdCount)
	
	for i := 1; i <= cmdCount; i++ {
		go func(idx int) {
			defer func() { done <- true }()
			
			key := fmt.Sprintf("key%d", idx)
			value := []byte(fmt.Sprintf("value%d", idx))
			
			cmd := types.Command{
				Type:  types.CommandPut,
				Key:   key,
				Value: value,
			}
			
			// 重试机制
			for retry := 0; retry < 5; retry++ {
				if err := cluster.Propose(cmd); err != nil {
					t.Logf("Retry %d for command %d: %v", retry, idx, err)
					time.Sleep(100 * time.Millisecond)
					continue
				}
				break
			}
		}(i)
	}
	
	// 等待所有命令完成
	for i := 0; i < cmdCount; i++ {
		<-done
	}
	
	// 等待日志复制
	time.Sleep(3 * time.Second)
	
	// 验证所有节点有一致的数据
	expectedData := make(map[string]string)
	for i := 1; i <= cmdCount; i++ {
		expectedData[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", i)
	}
	
	for nodeID := range cluster.GetNodes() {
		sm := cluster.GetStateMachine(nodeID)
		if sm == nil {
			t.Fatalf("State machine not found for node %s", nodeID)
		}
		
		// 检查键的数量
		if sm.Size() != cmdCount {
			t.Errorf("Node %s: expected %d keys, got %d", nodeID, cmdCount, sm.Size())
		}
		
		// 检查每个键的值
		for key, expectedValue := range expectedData {
			value, err := sm.Get(key)
			if err != nil {
				t.Errorf("Node %s: key %s not found: %v", nodeID, key, err)
				continue
			}
			
			if string(value) != expectedValue {
				t.Errorf("Node %s: key %s has wrong value. Expected %s, got %s", 
					nodeID, key, expectedValue, string(value))
			}
		}
	}
	
	t.Log("Consistency test passed")
}

// BenchmarkRaftThroughput Raft吞吐量基准测试
func BenchmarkRaftThroughput(b *testing.B) {
	// 创建3节点集群
	cluster, err := raft.CreateTestCluster(3)
	if err != nil {
		b.Fatalf("Failed to create test cluster: %v", err)
	}
	defer cluster.Stop()
	
	// 启动集群
	if err := cluster.Start(); err != nil {
		b.Fatalf("Failed to start cluster: %v", err)
	}
	
	// 等待领导者选举
	leader, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		b.Fatalf("Failed to elect leader: %v", err)
	}
	
	b.Logf("Leader elected: %s", leader)
	
	// 重置计时器
	b.ResetTimer()
	
	// 并发提议命令
	for i := 0; i < b.N; i++ {
		go func(idx int) {
			key := fmt.Sprintf("key%d", idx)
			value := []byte(fmt.Sprintf("value%d", idx))
			
			cmd := types.Command{
				Type:  types.CommandPut,
				Key:   key,
				Value: value,
			}
			
			cluster.Propose(cmd) // 忽略错误，专注于吞吐量
		}(i)
	}
	
	// 等待所有命令完成
	time.Sleep(5 * time.Second)
}