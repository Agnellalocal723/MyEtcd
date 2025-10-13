package raft

import (
	"fmt"
	"log"
	"sync"
	"time"

	"myetcd/internal/types"
)

/*
Raft集群管理器

集群管理器负责管理整个Raft集群，包括：
1. 创建和初始化Raft节点
2. 管理节点之间的通信
3. 处理集群成员变更
4. 监控集群健康状态

这个实现提供了一个简单的集群管理器，可以用于测试和演示。
在生产环境中，集群管理通常需要更复杂的机制，如服务发现、动态配置等。

主要特性：
- 自动节点发现
- 集群状态监控
- 故障检测和恢复
- 动态成员管理
*/

// Cluster Raft集群
type Cluster struct {
	mu      sync.RWMutex          // 读写锁
	nodes   map[string]*Node      // 节点ID到节点的映射
	configs map[string]*types.Config // 节点ID到配置的映射
	
	// 集群配置
	clusterConfig *types.Config   // 集群配置模板
	
	// 状态
	leader string                // 当前领导者
	
	// 通信
	transports map[string]*HTTPTransport // 节点ID到传输层的映射
	
	// 状态机
	stateMachines map[string]*SimpleStateMachine // 节点ID到状态机的映射
}

// NewCluster 创建新的Raft集群
func NewCluster(config *types.Config) *Cluster {
	return &Cluster{
		nodes:        make(map[string]*Node),
		configs:      make(map[string]*types.Config),
		clusterConfig: config,
		transports:   make(map[string]*HTTPTransport),
		stateMachines: make(map[string]*SimpleStateMachine),
	}
}

// AddNode 添加节点到集群
func (c *Cluster) AddNode(nodeID, addr string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// 检查节点是否已存在
	if _, exists := c.nodes[nodeID]; exists {
		return fmt.Errorf("node %s already exists", nodeID)
	}
	
	// 创建节点配置
	nodeConfig := *c.clusterConfig // 复制集群配置
	nodeConfig.NodeID = nodeID
	
	// 确保NodeAddresses映射存在
	if nodeConfig.NodeAddresses == nil {
		nodeConfig.NodeAddresses = make(map[string]string)
	}
	
	// 添加节点地址映射
	nodeConfig.NodeAddresses[nodeID] = addr
	
	// 添加节点到集群节点列表
	found := false
	for _, n := range nodeConfig.ClusterNodes {
		if n == nodeID {
			found = true
			break
		}
	}
	if !found {
		nodeConfig.ClusterNodes = append(nodeConfig.ClusterNodes, nodeID)
	}
	
	// 创建状态机
	stateMachine := NewSimpleStateMachine()
	c.stateMachines[nodeID] = stateMachine
	
	// 创建传输层
	transport := NewHTTPTransport(nodeID, addr, nil)
	c.transports[nodeID] = transport
	
	// 创建Raft节点
	node := NewNode(&nodeConfig, transport, stateMachine)
	transport.SetNode(node)
	
	// 保存节点和配置
	c.nodes[nodeID] = node
	c.configs[nodeID] = &nodeConfig
	
	log.Printf("Added node %s at %s to cluster", nodeID, addr)
	return nil
}

// RemoveNode 从集群中移除节点
func (c *Cluster) RemoveNode(nodeID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// 检查节点是否存在
	node, exists := c.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}
	
	// 停止节点
	node.Stop()
	
	// 停止传输层
	if transport, exists := c.transports[nodeID]; exists {
		transport.Stop()
		delete(c.transports, nodeID)
	}
	
	// 从集群节点列表中移除
	for i, n := range c.clusterConfig.ClusterNodes {
		if n == nodeID {
			c.clusterConfig.ClusterNodes = append(
				c.clusterConfig.ClusterNodes[:i],
				c.clusterConfig.ClusterNodes[i+1:]...)
			break
		}
	}
	
	// 删除节点和配置
	delete(c.nodes, nodeID)
	delete(c.configs, nodeID)
	delete(c.stateMachines, nodeID)
	
	// 更新其他节点的集群配置
	for _, otherNode := range c.nodes {
		otherNode.config.ClusterNodes = c.clusterConfig.ClusterNodes
	}
	
	log.Printf("Removed node %s from cluster", nodeID)
	return nil
}

// Start 启动集群
func (c *Cluster) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// 更新所有节点的节点地址映射
	for nodeID, node := range c.nodes {
		// 创建新的节点地址映射，避免共享引用
		nodeAddresses := make(map[string]string)
		for id, addr := range c.clusterConfig.NodeAddresses {
			nodeAddresses[id] = addr
		}
		node.config.NodeAddresses = nodeAddresses
		
		log.Printf("Node %s has %d nodes in cluster", nodeID, len(node.config.ClusterNodes))
		for _, id := range node.config.ClusterNodes {
			log.Printf("Node %s knows %s at %s", nodeID, id, node.config.NodeAddresses[id])
		}
	}
	
	// 启动所有节点的传输层
	for nodeID, transport := range c.transports {
		go func(id string, t *HTTPTransport) {
			if err := t.Start(); err != nil {
				log.Printf("Failed to start transport for node %s: %v", id, err)
			} else {
				log.Printf("Successfully started transport for node %s", id)
			}
		}(nodeID, transport)
		
		// 等待传输层启动
		time.Sleep(200 * time.Millisecond)
	}
	
	// 启动所有Raft节点
	for nodeID, node := range c.nodes {
		go func(id string, n *Node) {
			n.Start()
			log.Printf("Started Raft node %s", id)
		}(nodeID, node)
	}
	
	// 不在这里调用updateLeader，避免死锁
	// 领导者选举将由测试代码通过WaitForLeader方法检查
	
	log.Printf("Cluster started with %d nodes", len(c.nodes))
	return nil
}

// Stop 停止集群
func (c *Cluster) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// 停止所有Raft节点
	for nodeID, node := range c.nodes {
		node.Stop()
		log.Printf("Stopped Raft node %s", nodeID)
	}
	
	// 停止所有传输层
	for nodeID, transport := range c.transports {
		transport.Stop()
		log.Printf("Stopped transport for node %s", nodeID)
	}
	
	log.Printf("Cluster stopped")
	return nil
}

// GetLeader 获取当前领导者
func (c *Cluster) GetLeader() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.leader
}

// updateLeader 更新领导者信息
func (c *Cluster) updateLeader() {
	// 创建节点副本以避免在持有锁时调用节点方法
	nodes := make(map[string]*Node)
	c.mu.RLock()
	for id, node := range c.nodes {
		nodes[id] = node
	}
	c.mu.RUnlock()
	
	// 检查每个节点的状态
	for nodeID, node := range nodes {
		state, term := node.GetState()
		if state == types.Leader {
			c.mu.Lock()
			c.leader = nodeID
			c.mu.Unlock()
			log.Printf("Node %s is leader for term %d", nodeID, term)
			return
		}
	}
	
	c.mu.Lock()
	c.leader = ""
	c.mu.Unlock()
	log.Printf("No leader found")
}

// GetNode 获取指定节点
func (c *Cluster) GetNode(nodeID string) *Node {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nodes[nodeID]
}

// GetNodes 获取所有节点
func (c *Cluster) GetNodes() map[string]*Node {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	result := make(map[string]*Node)
	for id, node := range c.nodes {
		result[id] = node
	}
	return result
}

// GetStateMachine 获取指定节点的状态机
func (c *Cluster) GetStateMachine(nodeID string) *SimpleStateMachine {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stateMachines[nodeID]
}

// Propose 在领导者上提议新命令
func (c *Cluster) Propose(command types.Command) error {
	// 创建节点副本以避免在持有锁时调用节点方法
	nodes := make(map[string]*Node)
	c.mu.RLock()
	for id, node := range c.nodes {
		nodes[id] = node
	}
	c.mu.RUnlock()
	
	// 找到领导者节点
	var leader string
	var leaderNode *Node
	
	for nodeID, node := range nodes {
		state, _ := node.GetState()
		if state == types.Leader {
			leader = nodeID
			leaderNode = node
			break
		}
	}
	
	if leader == "" {
		return fmt.Errorf("no leader available")
	}
	
	if leaderNode == nil {
		return fmt.Errorf("leader node %s not found", leader)
	}
	
	log.Printf("Cluster proposing command to leader %s: type=%s, key=%s",
		leader, command.Type.String(), command.Key)
	
	err := leaderNode.Propose(command)
	if err != nil {
		log.Printf("Failed to propose command to leader %s: %v", leader, err)
		return err
	}
	
	log.Printf("Successfully proposed command to leader %s: type=%s, key=%s",
		leader, command.Type.String(), command.Key)
	
	return nil
}

// GetClusterStatus 获取集群状态
func (c *Cluster) GetClusterStatus() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	status := map[string]interface{}{
		"leader":      c.leader,
		"node_count":  len(c.nodes),
		"nodes":       make(map[string]interface{}),
	}
	
	for nodeID, node := range c.nodes {
		state, term := node.GetState()
		nodeStatus := map[string]interface{}{
			"state": state.String(),
			"term":  term,
		}
		
		// 添加状态机信息
		if sm, exists := c.stateMachines[nodeID]; exists {
			nodeStatus["key_count"] = sm.Size()
		}
		
		status["nodes"].(map[string]interface{})[nodeID] = nodeStatus
	}
	
	return status
}

// WaitForLeader 等待领导者选举完成
func (c *Cluster) WaitForLeader(timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		// 创建节点副本以避免在持有锁时调用节点方法
		nodes := make(map[string]*Node)
		c.mu.RLock()
		for id, node := range c.nodes {
			nodes[id] = node
		}
		c.mu.RUnlock()
		
		// 检查每个节点的状态
		for nodeID, node := range nodes {
			state, _ := node.GetState()
			if state == types.Leader {
				// 更新集群的领导者信息
				c.mu.Lock()
				c.leader = nodeID
				c.mu.Unlock()
				return nodeID, nil
			}
		}
		
		time.Sleep(200 * time.Millisecond)
	}
	
	return "", fmt.Errorf("no leader elected within timeout")
}

// CreateTestCluster 创建测试集群
func CreateTestCluster(nodeCount int) (*Cluster, error) {
	// 预先创建节点ID和地址映射
	nodeAddresses := make(map[string]string)
	clusterNodes := make([]string, 0, nodeCount)
	
	for i := 1; i <= nodeCount; i++ {
		nodeID := fmt.Sprintf("node%d", i)
		addr := fmt.Sprintf("127.0.0.1:%d", 8000+i)
		nodeAddresses[nodeID] = addr
		clusterNodes = append(clusterNodes, nodeID)
	}
	
	// 创建集群配置
	config := &types.Config{
		DataDir:           "./data",
		WALDir:            "./data/wal",
		SnapshotDir:       "./data/snapshot",
		SnapshotInterval:  time.Hour,
		MaxWALSize:        64 * 1024 * 1024, // 64MB
		MaxSnapshots:      3,
		HeartbeatInterval: 50 * time.Millisecond,
		ElectionTimeout:   1500 * time.Millisecond, // 减少选举超时时间
		NodeID:            "node1",
		ClusterNodes:      clusterNodes,
		NodeAddresses:     nodeAddresses,
	}
	
	// 创建集群
	cluster := NewCluster(config)
	
	// 添加节点
	for i := 1; i <= nodeCount; i++ {
		nodeID := fmt.Sprintf("node%d", i)
		addr := nodeAddresses[nodeID]
		
		if err := cluster.AddNode(nodeID, addr); err != nil {
			return nil, fmt.Errorf("failed to add node %s: %w", nodeID, err)
		}
	}
	
	return cluster, nil
}