package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"myetcd/internal/raft"
	"myetcd/internal/types"
)

/*
Raft算法示例程序

这个示例程序演示了如何使用Raft算法构建一个分布式键值存储系统。
它展示了以下功能：
1. 创建和启动Raft集群
2. 处理客户端请求
3. 领导选举和故障恢复
4. 日志复制和一致性保证

使用方法：
  go run examples/raft/main.go [nodeID] [port] [clusterSize]

示例：
  # 启动3节点集群
  go run examples/raft/main.go node1 8001 3 &
  go run examples/raft/main.go node2 8002 3 &
  go run examples/raft/main.go node3 8003 3 &

  # 启动5节点集群
  go run examples/raft/main.go node1 8001 5 &
  go run examples/raft/main.go node2 8002 5 &
  go run examples/raft/main.go node3 8003 5 &
  go run examples/raft/main.go node4 8004 5 &
  go run examples/raft/main.go node5 8005 5 &
*/

func main() {
	// 解析命令行参数
	if len(os.Args) < 4 {
		fmt.Println("Usage: go run examples/raft/main.go [nodeID] [port] [clusterSize]")
		fmt.Println("Example: go run examples/raft/main.go node1 8001 3")
		os.Exit(1)
	}

	nodeID := os.Args[1]
	port := os.Args[2]
	clusterSize, err := strconv.Atoi(os.Args[3])
	if err != nil {
		log.Fatalf("Invalid cluster size: %v", err)
	}

	// 创建集群配置
	config := &types.Config{
		DataDir:           fmt.Sprintf("./data-%s", nodeID),
		WALDir:            fmt.Sprintf("./data-%s/wal", nodeID),
		SnapshotDir:       fmt.Sprintf("./data-%s/snapshot", nodeID),
		SnapshotInterval:  30 * time.Second,
		MaxWALSize:        64 * 1024 * 1024, // 64MB
		MaxSnapshots:      3,
		HeartbeatInterval: 100 * time.Millisecond,
		ElectionTimeout:   1000 * time.Millisecond,
		NodeID:            nodeID,
		ClusterNodes:      make([]string, 0, clusterSize),
		NodeAddresses:     make(map[string]string),
	}

	// 构建集群节点列表和地址映射
	for i := 1; i <= clusterSize; i++ {
		clusterNodeID := fmt.Sprintf("node%d", i)
		clusterNodeAddr := fmt.Sprintf("127.0.0.1:%d", 8000+i)
		config.ClusterNodes = append(config.ClusterNodes, clusterNodeID)
		config.NodeAddresses[clusterNodeID] = clusterNodeAddr
	}

	// 创建状态机
	stateMachine := raft.NewSimpleStateMachine()

	// 创建传输层
	addr := fmt.Sprintf("127.0.0.1:%s", port)
	transport := raft.NewHTTPTransport(nodeID, addr, nil)

	// 创建Raft节点
	node := raft.NewNode(config, transport, stateMachine)
	transport.SetNode(node)

	// 创建集群管理器
	cluster := raft.NewCluster(config)

	// 添加节点到集群
	if err := cluster.AddNode(nodeID, addr); err != nil {
		log.Fatalf("Failed to add node to cluster: %v", err)
	}

	// 设置信号处理
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 启动节点
	go func() {
		log.Printf("Starting Raft node %s at %s", nodeID, addr)
		if err := transport.Start(); err != nil {
			log.Printf("Failed to start transport: %v", err)
			sigCh <- syscall.SIGTERM
		}
	}()

	// 启动Raft算法
	node.Start()

	// 等待集群稳定
	time.Sleep(2 * time.Second)

	// 启动演示goroutine
	go runDemo(node, nodeID, stateMachine)

	// 等待信号
	<-sigCh
	log.Printf("Shutting down node %s", nodeID)

	// 清理资源
	node.Stop()
	transport.Stop()
	cluster.Stop()

	log.Printf("Node %s stopped", nodeID)
}

// runDemo 运行演示
func runDemo(node *raft.Node, nodeID string, stateMachine *raft.SimpleStateMachine) {
	// 等待节点稳定
	time.Sleep(3 * time.Second)

	// 定期检查节点状态并演示功能
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	cmdCounter := 0

	for range ticker.C {
		// 获取节点状态
		state, term := node.GetState()
		log.Printf("Node %s state: %s, term: %d", nodeID, state.String(), term)

		// 如果是领导者，执行一些操作
		if state == types.Leader {
			cmdCounter++

			// 提议一个PUT命令
			key := fmt.Sprintf("demo-key-%d", cmdCounter)
			value := fmt.Sprintf("demo-value-%d-from-%s", cmdCounter, nodeID)

			cmd := types.Command{
				Type:  types.CommandPut,
				Key:   key,
				Value: []byte(value),
			}

			log.Printf("Leader %s proposing command: %s = %s", nodeID, key, value)
			if err := node.Propose(cmd); err != nil {
				log.Printf("Failed to propose command: %v", err)
			} else {
				log.Printf("Command proposed successfully")
			}

			// 每隔一段时间删除一个键
			if cmdCounter%5 == 0 {
				deleteKey := fmt.Sprintf("demo-key-%d", cmdCounter-4)
				deleteCmd := types.Command{
					Type: types.CommandDelete,
					Key:  deleteKey,
				}

				log.Printf("Leader %s proposing delete command: %s", nodeID, deleteKey)
				if err := node.Propose(deleteCmd); err != nil {
					log.Printf("Failed to propose delete command: %v", err)
				} else {
					log.Printf("Delete command proposed successfully")
				}
			}
		}

		// 显示状态机状态
		log.Printf("State machine size: %d keys", stateMachine.Size())

		// 列出一些键
		keys := stateMachine.ListKeys()
		if len(keys) > 0 {
			log.Printf("Sample keys: %v", keys[:min(5, len(keys))])

			// 显示第一个键的值
			if len(keys) > 0 {
				value, err := stateMachine.Get(keys[0])
				if err != nil {
					log.Printf("Failed to get key %s: %v", keys[0], err)
				} else {
					log.Printf("Key %s = %s", keys[0], string(value))
				}
			}
		}

		log.Printf("---")
	}
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
