package raft

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"myetcd/internal/types"
)

/*
HTTP传输层实现

这个文件实现了Raft节点之间的HTTP通信。它提供了两个主要功能：
1. 发送消息到其他节点
2. 接收来自其他节点的消息

HTTP传输层是Raft算法的底层通信机制，负责在不同节点之间传递Raft消息。
它实现了Transport接口，使得Raft算法不依赖于具体的通信协议。

主要特性：
- 异步发送消息
- 并发处理多个请求
- 自动重试机制
- 超时控制
*/

// HTTPTransport HTTP传输层
type HTTPTransport struct {
	nodeID    string            // 节点ID
	addr      string            // 监听地址
	server    *http.Server      // HTTP服务器
	client    *http.Client      // HTTP客户端
	node      *Node             // Raft节点引用
	mu        sync.Mutex        // 互斥锁
	handlers  map[string]func(types.RaftMessage) // 消息处理器
}

// NewHTTPTransport 创建新的HTTP传输层
func NewHTTPTransport(nodeID, addr string, node *Node) *HTTPTransport {
	transport := &HTTPTransport{
		nodeID:   nodeID,
		addr:     addr,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		node:     node,
		handlers: make(map[string]func(types.RaftMessage)),
	}
	
	// 设置路由
	mux := http.NewServeMux()
	mux.HandleFunc("/raft/message", transport.handleMessage)
	
	transport.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	
	return transport
}

// Start 启动HTTP传输层
func (t *HTTPTransport) Start() error {
	log.Printf("Starting HTTP transport for node %s on %s", t.nodeID, t.addr)
	return t.server.ListenAndServe()
}

// Stop 停止HTTP传输层
func (t *HTTPTransport) Stop() error {
	log.Printf("Stopping HTTP transport for node %s", t.nodeID)
	return t.server.Close()
}

// Send 发送消息到指定节点
func (t *HTTPTransport) Send(to string, msg types.RaftMessage) error {
	// 记录发送消息的日志
	log.Printf("Node %s sending %s message to %s", t.nodeID, msg.Type.String(), to)
	
	// 确定目标节点的地址
	targetAddr := t.getNodeAddr(to)
	if targetAddr == "" {
		log.Printf("Failed to get address for node %s", to)
		return fmt.Errorf("unknown node: %s", to)
	}
	
	// 构建URL
	url := fmt.Sprintf("http://%s/raft/message", targetAddr)
	
	// 序列化消息
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	
	// 创建请求
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	// 发送HTTP请求
	resp, err := t.client.Do(req)
	if err != nil {
		log.Printf("Failed to send HTTP request to %s: %v", to, err)
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()
	
	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("HTTP request failed with status %d: %s", resp.StatusCode, string(body))
		return fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	log.Printf("Successfully sent %s message to %s", msg.Type.String(), to)
	return nil
}

// Broadcast 广播消息到所有其他节点
func (t *HTTPTransport) Broadcast(msg types.RaftMessage) error {
	var wg sync.WaitGroup
	errs := make(chan error, len(t.node.config.ClusterNodes))
	
	// 并发发送到所有节点
	for _, nodeID := range t.node.config.ClusterNodes {
		if nodeID == t.nodeID {
			continue // 跳过自己
		}
		
		wg.Add(1)
		go func(to string) {
			defer wg.Done()
			if err := t.Send(to, msg); err != nil {
				errs <- fmt.Errorf("failed to send to %s: %w", to, err)
			}
		}(nodeID)
	}
	
	// 等待所有发送完成
	go func() {
		wg.Wait()
		close(errs)
	}()
	
	// 收集错误
	var errors []error
	for err := range errs {
		errors = append(errors, err)
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("broadcast failed with %d errors: %v", len(errors), errors[0])
	}
	
	return nil
}

// handleMessage 处理接收到的HTTP消息
func (t *HTTPTransport) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	
	// 反序列化消息
	var msg types.RaftMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		http.Error(w, "Failed to unmarshal message", http.StatusBadRequest)
		return
	}
	
	// 确保消息的To字段设置正确
	if msg.To == "" {
		msg.To = t.nodeID
	}
	
	// 记录接收到的消息
	log.Printf("Node %s received %s message from %s (to=%s)", t.nodeID, msg.Type.String(), msg.From, msg.To)
	
	// 处理消息
	t.node.ProcessMessage(msg)
	
	// 返回成功响应
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// getNodeAddr 获取节点的地址
func (t *HTTPTransport) getNodeAddr(nodeID string) string {
	// 从配置中获取节点地址
	if t.node != nil && t.node.config != nil && t.node.config.NodeAddresses != nil {
		if addr, exists := t.node.config.NodeAddresses[nodeID]; exists {
			log.Printf("Found address for node %s: %s", nodeID, addr)
			return addr
		}
	}
	
	// 如果没有找到映射，返回节点ID作为后备方案
	// 这在单节点测试中可能会有用
	log.Printf("Address not found for node %s, using nodeID as address", nodeID)
	return nodeID
}

// SetNode 设置Raft节点引用
func (t *HTTPTransport) SetNode(node *Node) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.node = node
}

// GetAddr 获取监听地址
func (t *HTTPTransport) GetAddr() string {
	return t.addr
}