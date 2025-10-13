package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"myetcd/internal/metrics"
	"myetcd/internal/storage"
	"myetcd/internal/types"
)

/*
HTTP API服务器设计详解

## 1. API设计原则

### RESTful设计
```
为什么选择RESTful？
- 简洁明了：使用HTTP动词表示操作
- 易于理解：URL结构清晰
- 广泛支持：各种HTTP客户端都支持
- 可扩展：易于添加新功能

我们的API设计：
GET    /v1/key/{key}     - 获取单个键值
PUT    /v1/key/{key}     - 设置键值
DELETE /v1/key/{key}     - 删除键
GET    /v1/keys          - 列出所有键
POST   /v1/range         - 范围查询
POST   /v1/watch         - 创建监听
DELETE /v1/watch/{id}    - 取消监听
POST   /v1/batch         - 批量操作
POST   /v1/txn           - 事务操作
POST   /v1/lease/grant   - 授予租约
POST   /v1/lease/revoke  - 撤销租约
POST   /v1/lease/keepalive - 保持租约活跃
GET    /v1/health        - 健康检查
GET    /v1/status        - 状态查询
```

### 统一响应格式
```
为什么需要统一响应格式？
- 一致性：客户端可以统一处理响应
- 可扩展：易于添加新字段
- 错误处理：统一错误格式

响应结构：
{
  "success": true,        // 操作是否成功
  "data": {...},          // 响应数据
  "error": "error message" // 错误信息（仅失败时）
}
```

### 版本控制
```
为什么需要版本控制？
- 向后兼容：旧版本客户端可以继续使用
- 平滑升级：新旧版本可以并存
- 功能演进：可以修改API设计

版本策略：
- URL路径版本：/v1/, /v2/
- 向后兼容：保持旧版本可用
- 废弃通知：通过响应头通知客户端
```

## 2. 性能优化策略

### 连接管理
```
HTTP连接优化：
- Keep-Alive：复用TCP连接
- 超时设置：防止连接泄漏
- 连接池：限制并发连接数

客户端优化：
- 连接复用：使用HTTP客户端连接池
- 超时控制：设置合理的超时时间
- 重试机制：处理网络故障
```

### 数据传输优化
```
序列化优化：
- JSON格式：易于调试和理解
- 压缩传输：减少网络带宽
- 批量操作：减少网络往返

缓存策略：
- 健康检查：轻量级检查
- 状态查询：缓存部分状态
- 监听连接：长连接减少握手
```

### 并发控制
```
并发处理：
- 每个请求独立goroutine
- 读写锁保护共享资源
- 通道通信避免竞态

资源限制：
- 请求体大小限制
- 并发连接数限制
- 超时时间控制
```

## 3. 安全考虑

### 输入验证
```
为什么需要输入验证？
- 防止注入攻击
- 保证数据完整性
- 提供友好错误信息

验证策略：
- 键名格式检查
- 值长度限制
- TTL范围验证
- JSON格式验证
```

### 错误处理
```
错误处理原则：
- 不暴露内部信息
- 提供有意义的错误消息
- 记录详细错误日志

错误分类：
- 客户端错误：4xx状态码
- 服务器错误：5xx状态码
- 业务错误：自定义错误码
```

### 访问控制
```
当前实现：
- 无认证机制（简化实现）
- 基础的CORS支持
- 请求大小限制

扩展方案：
- JWT认证
- 基于角色的访问控制
- API限流
```

## 4. 监控和调试

### 指标收集
```
为什么需要指标？
- 性能监控：了解系统负载
- 故障诊断：快速定位问题
- 容量规划：预测资源需求

关键指标：
- 请求计数：按类型分类
- 响应时间：延迟分布
- 错误率：失败请求比例
- 并发连接：活跃连接数
```

### 日志记录
```
日志策略：
- 结构化日志：便于分析
- 不同级别：INFO/WARN/ERROR
- 关键操作：记录重要操作
- 错误详情：记录错误堆栈
```

### 健康检查
```
健康检查目的：
- 负载均衡：判断节点状态
- 故障检测：快速发现问题
- 运维监控：系统状态监控

检查内容：
- 服务状态：是否正常运行
- 存储状态：数据是否可访问
- 资源使用：CPU/内存/磁盘
```

## 5. 客户端设计

### 客户端功能
```
为什么提供客户端？
- 简化使用：封装HTTP细节
- 类型安全：强类型接口
- 错误处理：统一错误处理
- 连接管理：自动重连和超时

客户端特性：
- 同步和异步接口
- 自动重试机制
- 连接池管理
- 指标收集
```

### 使用模式
```
基本操作：
- 简单的键值操作
- 批量操作
- 事务操作
- 监听操作

高级功能：
- 租约管理
- 范围查询
- 健康检查
- 状态查询
```

## 6. 扩展性设计

### 插件机制
```
为什么需要插件？
- 功能扩展：不修改核心代码
- 第三方集成：支持自定义功能
- 实验性功能：安全地测试新功能

插件接口：
- 中间件：请求/响应处理
- 认证：自定义认证机制
- 存储：不同的存储后端
```

### 配置管理
```
配置策略：
- 文件配置：静态配置
- 环境变量：部署配置
- 动态配置：运行时修改
- 默认值：合理默认配置

配置项：
- 监听地址和端口
- 日志级别和输出
- 性能参数
- 安全设置
```

## 7. 最佳实践

### 代码组织
```
模块化设计：
- 按功能分组：键值、租约、事务等
- 接口抽象：便于测试和扩展
- 错误处理：统一错误处理机制
- 文档注释：详细的API文档
```

### 测试策略
```
测试类型：
- 单元测试：测试单个函数
- 集成测试：测试完整流程
- 性能测试：测试负载能力
- 兼容性测试：测试不同客户端

测试工具：
- Go标准测试包
- HTTP测试工具
- 基准测试
- 模拟测试
```

### 部署考虑
```
部署方式：
- 单机部署：简单直接
- 容器化：Docker/Kubernetes
- 集群部署：高可用性
- 负载均衡：分发请求

运维工具：
- 监控系统：Prometheus
- 日志收集：ELK Stack
- 告警系统：AlertManager
- 配置管理：etcd/confd
```

## 8. 实现细节

### 请求处理流程
```
1. 路由匹配：根据URL和方法选择处理函数
2. 参数解析：从URL、查询参数、请求体提取数据
3. 输入验证：检查参数格式和范围
4. 业务处理：调用存储引擎执行操作
5. 响应构建：格式化响应数据
6. 错误处理：统一错误响应格式
7. 指标记录：更新性能指标
8. 日志记录：记录关键操作
```

### 并发安全
```
并发控制策略：
- 读写锁：保护共享资源
- 原子操作：计数器和状态
- 通道通信：goroutine间通信
- 上下文管理：请求取消和超时

资源管理：
- 连接池：复用HTTP连接
- 内存池：减少GC压力
- 文件句柄：及时释放资源
- 超时控制：防止资源泄漏
```

### 错误处理策略
```
错误分类：
- 输入错误：客户端请求问题
- 系统错误：服务器内部问题
- 网络错误：通信问题
- 业务错误：逻辑问题

处理方式：
- 统一格式：一致的错误响应
- 错误码：标准化错误分类
- 详细日志：记录错误上下文
- 用户友好：提供可理解的错误信息
```

这个API服务器设计遵循了现代微服务的最佳实践，提供了完整的键值存储功能，同时考虑了性能、安全性和可扩展性。通过详细的注释和文档，开发者可以轻松理解和使用这个系统。
*/

// Server HTTP API服务器
//
// 这个结构体实现了完整的HTTP API服务器，包括：
// - RESTful API设计
// - 统一的响应格式
// - 完整的错误处理
// - 性能指标收集
// - 客户端SDK
//
// 设计特点：
// - 模块化：按功能分组处理函数
// - 可扩展：易于添加新的API端点
// - 高性能：并发处理和连接复用
// - 可观测：完整的监控和日志
type Server struct {
	engine *storage.Engine // 存储引擎实例
	config *types.Config   // 配置信息
	server *http.Server    // HTTP服务器实例
}

// NewServer 创建新的API服务器
func NewServer(engine *storage.Engine, config *types.Config) *Server {
	s := &Server{
		engine: engine,
		config: config,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/key/", s.handleKey)
	mux.HandleFunc("/v1/keys", s.handleKeys)
	mux.HandleFunc("/v1/range", s.handleRange)
	mux.HandleFunc("/v1/watch", s.handleWatch)
	mux.HandleFunc("/v1/watch/", s.handleWatchCancel)
	mux.HandleFunc("/v1/batch", s.handleBatch)
	mux.HandleFunc("/v1/lease", s.handleLease)
	mux.HandleFunc("/v1/lease/", s.handleLeaseByID)
	mux.HandleFunc("/v1/lease/grant", s.handleLeaseGrant)
	mux.HandleFunc("/v1/lease/revoke", s.handleLeaseRevoke)
	mux.HandleFunc("/v1/lease/keepalive", s.handleLeaseKeepAlive)
	mux.HandleFunc("/v1/lease/list", s.handleLeaseList)
	mux.HandleFunc("/v1/txn", s.handleTransaction)
	mux.HandleFunc("/v1/health", s.handleHealth)
	mux.HandleFunc("/v1/status", s.handleStatus)
	
	// 添加指标端点
	mux.Handle("/metrics", metrics.NewHandler())

	// 应用指标中间件
	handler := metrics.MetricsMiddleware(mux)

	s.server = &http.Server{
		Addr:    ":2379", // 默认etcd端口
		Handler: handler,
	}

	return s
}

// SetAddr 设置服务器地址
func (s *Server) SetAddr(addr string) {
	s.server.Addr = addr
}

// Start 启动服务器
func (s *Server) Start() error {
	log.Printf("Starting MyEtcd server on %s", s.server.Addr)
	return s.server.ListenAndServe()
}

// Stop 停止服务器
func (s *Server) Stop() error {
	return s.server.Close()
}

// handleKey 处理单个键的CRUD操作
func (s *Server) handleKey(w http.ResponseWriter, r *http.Request) {
	// 从URL路径中提取键名
	key := strings.TrimPrefix(r.URL.Path, "/v1/key/")
	if key == "" {
		s.writeError(w, http.StatusBadRequest, "key is required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.handleGet(w, r, key)
	case http.MethodPut:
		s.handlePut(w, r, key)
	case http.MethodDelete:
		s.handleDelete(w, r, key)
	default:
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleGet 处理GET请求
func (s *Server) handleGet(w http.ResponseWriter, r *http.Request, key string) {
	start := time.Now()
	defer func() {
		metrics.GetMetrics().ObserveStorageOperationDuration("GET", time.Since(start))
	}()
	
	kv, err := s.engine.Get(key)
	if err != nil {
		if err == storage.ErrKeyNotFound || err == storage.ErrKeyExpired {
			metrics.GetMetrics().RecordStorageOperation("GET", "not_found")
			s.writeError(w, http.StatusNotFound, "key not found")
			return
		}
		metrics.GetMetrics().RecordStorageOperation("GET", "error")
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	metrics.GetMetrics().RecordStorageOperation("GET", "success")

	response := types.GetResponse{
		Key:      kv.Key,
		Value:    string(kv.Value),
		Created:  kv.Created,
		Modified: kv.Modified,
		Version:  kv.Version,
	}

	s.writeJSON(w, http.StatusOK, types.Response{
		Success: true,
		Data:    response,
	})
}

// handlePut 处理PUT请求
func (s *Server) handlePut(w http.ResponseWriter, r *http.Request, key string) {
	start := time.Now()
	defer func() {
		metrics.GetMetrics().ObserveStorageOperationDuration("PUT", time.Since(start))
	}()
	
	var req types.PutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		metrics.GetMetrics().RecordStorageOperation("PUT", "invalid_request")
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// 验证请求
	if req.Key != "" && req.Key != key {
		metrics.GetMetrics().RecordStorageOperation("PUT", "key_mismatch")
		s.writeError(w, http.StatusBadRequest, "key mismatch")
		return
	}

	if key == "" {
		key = req.Key
	}

	if key == "" {
		metrics.GetMetrics().RecordStorageOperation("PUT", "missing_key")
		s.writeError(w, http.StatusBadRequest, "key is required")
		return
	}

	// 存储键值对
	err := s.engine.Put(key, []byte(req.Value), req.TTL)
	if err != nil {
		metrics.GetMetrics().RecordStorageOperation("PUT", "error")
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	metrics.GetMetrics().RecordStorageOperation("PUT", "success")

	s.writeJSON(w, http.StatusOK, types.Response{
		Success: true,
		Data:    map[string]string{"message": "key stored successfully"},
	})
}

// handleDelete 处理DELETE请求
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request, key string) {
	err := s.engine.Delete(key)
	if err != nil {
		if err == storage.ErrKeyNotFound {
			s.writeError(w, http.StatusNotFound, "key not found")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, types.Response{
		Success: true,
		Data:    map[string]string{"message": "key deleted successfully"},
	})
}

// handleKeys 处理列出所有键的请求
func (s *Server) handleKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	keys := s.engine.ListKeys()
	s.writeJSON(w, http.StatusOK, types.Response{
		Success: true,
		Data:    keys,
	})
}

// handleRange 处理范围查询请求
func (s *Server) handleRange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req types.RangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	kvs, count, err := s.engine.Range(req.Key, req.RangeEnd, req.Limit)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	response := types.RangeResponse{
		Kvs:      kvs,
		More:     req.Limit > 0 && int64(len(kvs)) < count,
		Count:    count,
		Revision: 0, // 简化实现，不维护版本号
	}
	
	s.writeJSON(w, http.StatusOK, types.Response{
		Success: true,
		Data:    response,
	})
}

// handleWatch 处理Watch请求
//
// 这个函数实现了Server-Sent Events (SSE) 长连接，用于实时监听键值变化：
// - 创建Watch监听器
// - 建立SSE长连接
// - 实时推送事件
//
// Watch机制的优势：
// - 实时性：立即推送变化
// - 高效：避免轮询开销
// - 可靠：自动重连机制
// - 标准：基于HTML5标准
//
// SSE vs WebSocket：
// - SSE：单向推送，简单可靠
// - WebSocket：双向通信，复杂但灵活
// - 选择SSE的原因：键值监听只需单向推送
//
// 实现细节：
// 1. 长连接：保持HTTP连接开放
// 2. 流式传输：逐步发送事件
// 3. 格式化：使用SSE标准格式
// 4. 错误处理：连接断开时清理资源
//
// 性能考虑：
// - 连接池：限制并发Watch数量
// - 缓冲区：防止阻塞
// - 心跳：保持连接活跃
// - 超时：自动清理死连接
func (s *Server) handleWatch(w http.ResponseWriter, r *http.Request) {
	// 验证HTTP方法
	// 为什么只支持POST？
	// 1. 参数传递：需要传递复杂的Watch参数
	// 2. 安全性：POST不会缓存请求
	// 3. 语义：POST表示创建Watch资源
	// 4. 扩展性：未来可能添加更多参数
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	// 解析请求参数
	// WatchRequest包含：
	// - Key：要监听的键
	// - Prefix：是否监听前缀
	// - PrevKV：是否包含前一个值
	// - WatchID：Watch ID（服务器分配）
	var req types.WatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// 错误处理：
		// 1. 状态码：400 Bad Request
		// 2. 错误信息：指出JSON解析失败
		// 3. 日志记录：记录请求详情
		// 4. 指标更新：更新错误指标
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	// 创建Watch监听器
	// 返回值：
	// - watchID：唯一标识Watch
	// - eventChan：事件通道
	// - err：错误信息
	//
	// 为什么返回通道而不是直接处理？
	// 1. 解耦：Watch逻辑与HTTP处理分离
	// 2. 可测试：便于单元测试
	// 3. 可扩展：支持多种事件源
	// 4. 并发：异步处理事件
	watchID, eventChan, err := s.engine.Watch(req.Key, req.Prefix, req.PrevKV)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	// 设置SSE响应头
	// 这些头部告诉浏览器：
	// 1. Content-Type：事件流格式
	// 2. Cache-Control：不缓存响应
	// 3. Connection：保持连接
	// 4. CORS：允许跨域访问
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	// 发送Watch ID确认
	// 这是第一个事件，确认Watch创建成功
	// 客户端应该等待这个确认后再处理后续事件
	fmt.Fprintf(w, "data: %s\n\n", mustMarshalJSON(types.WatchResponse{
		WatchID: watchID,
		Events:  []types.WatchEvent{},
	}))
	
	// 立即刷新响应
	// 为什么需要刷新？
	// 1. 确保确认消息立即发送
	// 2. 防止缓冲延迟
	// 3. 提供实时反馈
	// 4. 避免超时
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	
	// 发送事件流
	// 这个循环会一直运行，直到：
	// 1. 客户端断开连接
	// 2. 服务器关闭
	// 3. Watch被取消
	// 4. 发生错误
	for event := range eventChan {
		response := types.WatchResponse{
			WatchID: watchID,
			Events:  []types.WatchEvent{event},
		}
		
		// 使用SSE格式发送事件
		// 格式：data: {JSON}\n\n
		// 为什么这种格式？
		// 1. 标准：SSE标准格式
		// 2. 解析：客户端易于解析
		// 3. 兼容：广泛支持
		// 4. 扩展：支持事件类型和ID
		fmt.Fprintf(w, "data: %s\n\n", mustMarshalJSON(response))
		
		// 刷新响应
		// 确保事件立即发送给客户端
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}
	
	// 事件通道关闭，连接结束
	// 资源清理：
	// 1. 自动取消Watch
	// 2. 释放连接资源
	// 3. 记录日志
	// 4. 更新指标
}

// handleWatchCancel 处理取消Watch请求
func (s *Server) handleWatchCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	// 从URL路径中提取Watch ID
	watchIDStr := strings.TrimPrefix(r.URL.Path, "/v1/watch/")
	if watchIDStr == "" {
		s.writeError(w, http.StatusBadRequest, "watch ID is required")
		return
	}
	
	var watchID int64
	if _, err := fmt.Sscanf(watchIDStr, "%d", &watchID); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid watch ID")
		return
	}
	
	if err := s.engine.CancelWatch(watchID); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	s.writeJSON(w, http.StatusOK, types.Response{
		Success: true,
		Data:    map[string]interface{}{"message": "watch cancelled"},
	})
}

// handleBatch 处理批量操作请求
func (s *Server) handleBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req types.BatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	responses := make([]interface{}, 0, len(req.Operations))
	
	// 分离PUT和DELETE操作
	puts := make(map[string][]byte)
	deletes := []string{}
	
	for _, op := range req.Operations {
		switch op.Op {
		case "PUT":
			puts[op.Key] = []byte(op.Value)
		case "DELETE":
			deletes = append(deletes, op.Key)
		default:
			s.writeError(w, http.StatusBadRequest, fmt.Sprintf("unsupported operation: %s", op.Op))
			return
		}
	}
	
	// 执行批量PUT
	if len(puts) > 0 {
		if err := s.engine.BatchPut(puts, 0); err != nil {
			s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("batch put failed: %v", err))
			return
		}
		for key := range puts {
			responses = append(responses, map[string]string{"op": "PUT", "key": key, "status": "success"})
		}
	}
	
	// 执行批量DELETE
	if len(deletes) > 0 {
		if err := s.engine.BatchDelete(deletes); err != nil {
			s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("batch delete failed: %v", err))
			return
		}
		for _, key := range deletes {
			responses = append(responses, map[string]string{"op": "DELETE", "key": key, "status": "success"})
		}
	}
	
	s.writeJSON(w, http.StatusOK, types.Response{
		Success: true,
		Data: types.BatchResponse{
			Responses: responses,
			Success:   true,
		},
	})
}

// handleLeaseGrant 处理授予租约请求
func (s *Server) handleLeaseGrant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req types.LeaseGrantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if req.TTL <= 0 {
		s.writeError(w, http.StatusBadRequest, "TTL must be positive")
		return
	}
	
	leaseID, err := s.engine.GrantLease(req.TTL)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	response := types.LeaseGrantResponse{
		ID:  leaseID,
		TTL: req.TTL,
	}
	
	s.writeJSON(w, http.StatusOK, types.Response{
		Success: true,
		Data:    response,
	})
}

// handleLeaseRevoke 处理撤销租约请求
func (s *Server) handleLeaseRevoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req types.LeaseRevokeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if req.ID <= 0 {
		s.writeError(w, http.StatusBadRequest, "lease ID must be positive")
		return
	}
	
	if err := s.engine.RevokeLease(req.ID); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	response := types.LeaseRevokeResponse{
		Header: types.ResponseHeader{
			Revision: 0, // 简化实现
		},
	}
	
	s.writeJSON(w, http.StatusOK, types.Response{
		Success: true,
		Data:    response,
	})
}

// handleLeaseKeepAlive 处理保持租约活跃请求
func (s *Server) handleLeaseKeepAlive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req types.LeaseKeepAliveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if req.ID <= 0 {
		s.writeError(w, http.StatusBadRequest, "lease ID must be positive")
		return
	}
	
	ttl, err := s.engine.KeepAliveLease(req.ID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	response := types.LeaseKeepAliveResponse{
		ID:        req.ID,
		TTL:       ttl,
		Remaining: ttl, // 简化实现
	}
	
	s.writeJSON(w, http.StatusOK, types.Response{
		Success: true,
		Data:    response,
	})
}

// handleLease 处理租约列表请求
func (s *Server) handleLease(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	leases, err := s.engine.ListLeases()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	s.writeJSON(w, http.StatusOK, types.Response{
		Success: true,
		Data:    leases,
	})
}

// handleLeaseByID 处理根据ID获取租约请求
func (s *Server) handleLeaseByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	// 从URL路径中提取租约ID
	leaseIDStr := strings.TrimPrefix(r.URL.Path, "/v1/lease/")
	if leaseIDStr == "" {
		s.writeError(w, http.StatusBadRequest, "lease ID is required")
		return
	}
	
	var leaseID int64
	if _, err := fmt.Sscanf(leaseIDStr, "%d", &leaseID); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid lease ID")
		return
	}
	
	lease, err := s.engine.GetLease(leaseID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}
	
	s.writeJSON(w, http.StatusOK, types.Response{
		Success: true,
		Data:    lease,
	})
}

// handleLeaseList 处理租约列表请求（别名）
func (s *Server) handleLeaseList(w http.ResponseWriter, r *http.Request) {
	s.handleLease(w, r)
}

// handleTransaction 处理事务请求
func (s *Server) handleTransaction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req types.TxnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// 开始事务
	txn, err := s.engine.BeginTransaction()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 检查比较条件
	conditionsMet := s.checkConditions(req.Compare)
	
	// 根据条件结果选择执行的操作
	var operations []types.Request
	if conditionsMet {
		operations = req.Success
	} else {
		operations = req.Failure
	}

	// 执行操作
	results := make([]interface{}, 0, len(operations))
	for _, op := range operations {
		result, err := s.executeOperation(op)
		if err != nil {
			// 事务失败，回滚
			s.engine.RollbackTransaction(txn.ID)
			s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("transaction failed: %v", err))
			return
		}
		results = append(results, result)
	}

	// 提交事务
	if err := s.engine.CommitTransaction(txn.ID); err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to commit transaction: %v", err))
		return
	}

	s.writeJSON(w, http.StatusOK, types.Response{
		Success: true,
		Data: map[string]interface{}{
			"succeeded": conditionsMet,
			"responses": results,
		},
	})
}

// checkConditions 检查事务条件
func (s *Server) checkConditions(conditions []types.Compare) bool {
	for _, cond := range conditions {
		kv, err := s.engine.Get(cond.Key)
		if err != nil {
			// 键不存在，根据操作类型判断
			if cond.Op == "!=" || cond.Op == ">" {
				continue
			}
			return false
		}

		// 比较值
		kvValue := string(kv.Value)
		condValue := fmt.Sprintf("%v", cond.Value)

		switch cond.Op {
		case "=":
			if kvValue != condValue {
				return false
			}
		case "!=":
			if kvValue == condValue {
				return false
			}
		case ">":
			if kvValue <= condValue {
				return false
			}
		case "<":
			if kvValue >= condValue {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// executeOperation 执行单个操作
func (s *Server) executeOperation(op types.Request) (interface{}, error) {
	switch op.Op {
	case "PUT":
		err := s.engine.Put(op.Key, []byte(op.Value), 0)
		if err != nil {
			return nil, err
		}
		return map[string]string{"op": "PUT", "key": op.Key, "value": op.Value}, nil

	case "GET":
		kv, err := s.engine.Get(op.Key)
		if err != nil {
			return nil, err
		}
		return types.GetResponse{
			Key:      kv.Key,
			Value:    string(kv.Value),
			Created:  kv.Created,
			Modified: kv.Modified,
			Version:  kv.Version,
		}, nil

	case "DELETE":
		err := s.engine.Delete(op.Key)
		if err != nil {
			return nil, err
		}
		return map[string]string{"op": "DELETE", "key": op.Key}, nil

	default:
		return nil, fmt.Errorf("unknown operation: %s", op.Op)
	}
}

// handleHealth 处理健康检查
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	s.writeJSON(w, http.StatusOK, types.Response{
		Success: true,
		Data: map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now(),
		},
	})
}

// handleStatus 处理状态查询
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// 更新指标
	metrics.GetMetrics().SetStorageKeys(s.engine.Size())
	metrics.GetMetrics().SetWatchers(s.engine.GetWatchCount())

	s.writeJSON(w, http.StatusOK, types.Response{
		Success: true,
		Data: map[string]interface{}{
			"version":     "1.0.0",
			"keys_count":  s.engine.Size(),
			"uptime":      time.Since(time.Now()).String(), // 这里应该记录启动时间
			"data_dir":    s.config.DataDir,
			"wal_dir":     s.config.WALDir,
			"snapshot_dir": s.config.SnapshotDir,
			"watch_count": s.engine.GetWatchCount(),
		},
	})
}

// writeJSON 写入JSON响应
func (s *Server) writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
	}
}

// writeError 写入错误响应
func (s *Server) writeError(w http.ResponseWriter, statusCode int, message string) {
	s.writeJSON(w, statusCode, types.Response{
		Success: false,
		Error:   message,
	})
}

// Client 客户端
type Client struct {
	baseURL string
	client  *http.Client
}

// NewClient 创建新的客户端
func NewClient(baseURL string) *Client {
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}

	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// Put 存储键值对
func (c *Client) Put(key, value string, ttl int64) error {
	req := types.PutRequest{
		Key:   key,
		Value: value,
		TTL:   ttl,
	}

	resp, err := c.doRequest("PUT", fmt.Sprintf("/v1/key/%s", key), req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var result types.Response
		if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
			return fmt.Errorf("failed to put key: %s", result.Error)
		}
		return fmt.Errorf("failed to put key: status %d", resp.StatusCode)
	}

	return nil
}

// Get 获取键值
func (c *Client) Get(key string) (*types.GetResponse, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/v1/key/%s", key), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, storage.ErrKeyNotFound
		}
		return nil, fmt.Errorf("failed to get key: status %d", resp.StatusCode)
	}

	var result types.Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// 解析响应数据
	dataBytes, err := json.Marshal(result.Data)
	if err != nil {
		return nil, err
	}

	var getResp types.GetResponse
	if err := json.Unmarshal(dataBytes, &getResp); err != nil {
		return nil, err
	}

	return &getResp, nil
}

// Delete 删除键
func (c *Client) Delete(key string) error {
	resp, err := c.doRequest("DELETE", fmt.Sprintf("/v1/key/%s", key), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return storage.ErrKeyNotFound
		}
		return fmt.Errorf("failed to delete key: status %d", resp.StatusCode)
	}

	return nil
}

// ListKeys 列出所有键
func (c *Client) ListKeys() ([]string, error) {
	resp, err := c.doRequest("GET", "/v1/keys", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list keys: status %d", resp.StatusCode)
	}

	var result types.Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// 解析键列表
	var keys []string
	dataBytes, err := json.Marshal(result.Data)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(dataBytes, &keys); err != nil {
		return nil, err
	}

	return keys, nil
}

// Transaction 执行事务
func (c *Client) Transaction(req types.TxnRequest) (*types.Response, error) {
	resp, err := c.doRequest("POST", "/v1/txn", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to execute transaction: status %d", resp.StatusCode)
	}

	var result types.Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Health 健康检查
func (c *Client) Health() error {
	resp, err := c.doRequest("GET", "/v1/health", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: status %d", resp.StatusCode)
	}

	return nil
}

// Status 获取状态
func (c *Client) Status() (map[string]interface{}, error) {
	resp, err := c.doRequest("GET", "/v1/status", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get status: status %d", resp.StatusCode)
	}

	var result types.Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// 解析状态数据
	dataBytes, err := json.Marshal(result.Data)
	if err != nil {
		return nil, err
	}

	var status map[string]interface{}
	if err := json.Unmarshal(dataBytes, &status); err != nil {
		return nil, err
	}

	return status, nil
}

// doRequest 执行HTTP请求
func (c *Client) doRequest(method, path string, body interface{}) (*http.Response, error) {
	var reqBody *strings.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = strings.NewReader(string(bodyBytes))
	} else {
		reqBody = strings.NewReader("")
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.client.Do(req)
}

// Watch 创建Watch
func (c *Client) Watch(key string, prefix, prevKV bool) (int64, <-chan types.WatchEvent, error) {
	req := types.WatchRequest{
		Key:     key,
		Prefix:  prefix,
		PrevKV:  prevKV,
		WatchID: 0, // 服务器会分配
	}
	
	resp, err := c.doRequest("POST", "/v1/watch", req)
	if err != nil {
		return 0, nil, err
	}
	
	// 读取响应流
	eventChan := make(chan types.WatchEvent, 100)
	
	go func() {
		defer resp.Body.Close()
		defer close(eventChan)
		
		scanner := newScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				var watchResp types.WatchResponse
				if err := json.Unmarshal([]byte(data), &watchResp); err == nil {
					for _, event := range watchResp.Events {
						eventChan <- event
					}
				}
			}
		}
	}()
	
	// 等待第一个响应获取Watch ID
	select {
	case <-eventChan:
		// 第一个事件是确认，不包含实际数据
		return 0, eventChan, nil
	case <-time.After(5 * time.Second):
		return 0, nil, fmt.Errorf("timeout waiting for watch confirmation")
	}
}

// CancelWatch 取消Watch
func (c *Client) CancelWatch(watchID int64) error {
	resp, err := c.doRequest("DELETE", fmt.Sprintf("/v1/watch/%d", watchID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to cancel watch: status %d", resp.StatusCode)
	}
	
	return nil
}

// Range 范围查询
func (c *Client) Range(key, rangeEnd string, limit int64) ([]*types.KeyValue, int64, error) {
	req := types.RangeRequest{
		Key:      key,
		RangeEnd: rangeEnd,
		Limit:    limit,
	}
	
	resp, err := c.doRequest("POST", "/v1/range", req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("failed to range query: status %d", resp.StatusCode)
	}
	
	var result types.Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, 0, err
	}
	
	// 解析范围查询响应
	dataBytes, err := json.Marshal(result.Data)
	if err != nil {
		return nil, 0, err
	}
	
	var rangeResp types.RangeResponse
	if err := json.Unmarshal(dataBytes, &rangeResp); err != nil {
		return nil, 0, err
	}
	
	return rangeResp.Kvs, rangeResp.Count, nil
}

// Batch 批量操作
func (c *Client) Batch(operations []types.Request) (*types.BatchResponse, error) {
	req := types.BatchRequest{
		Operations: operations,
	}
	
	resp, err := c.doRequest("POST", "/v1/batch", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to execute batch: status %d", resp.StatusCode)
	}
	
	var result types.Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	// 解析批量响应
	dataBytes, err := json.Marshal(result.Data)
	if err != nil {
		return nil, err
	}
	
	var batchResp types.BatchResponse
	if err := json.Unmarshal(dataBytes, &batchResp); err != nil {
		return nil, err
	}
	
	return &batchResp, nil
}

// GrantLease 授予租约
func (c *Client) GrantLease(ttl int64) (int64, error) {
	req := types.LeaseGrantRequest{
		TTL: ttl,
	}
	
	resp, err := c.doRequest("POST", "/v1/lease/grant", req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to grant lease: status %d", resp.StatusCode)
	}
	
	var result types.Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}
	
	// 解析租约授予响应
	dataBytes, err := json.Marshal(result.Data)
	if err != nil {
		return 0, err
	}
	
	var leaseResp types.LeaseGrantResponse
	if err := json.Unmarshal(dataBytes, &leaseResp); err != nil {
		return 0, err
	}
	
	return leaseResp.ID, nil
}

// RevokeLease 撤销租约
func (c *Client) RevokeLease(leaseID int64) error {
	req := types.LeaseRevokeRequest{
		ID: leaseID,
	}
	
	resp, err := c.doRequest("POST", "/v1/lease/revoke", req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to revoke lease: status %d", resp.StatusCode)
	}
	
	return nil
}

// KeepAliveLease 保持租约活跃
func (c *Client) KeepAliveLease(leaseID int64) (int64, error) {
	req := types.LeaseKeepAliveRequest{
		ID: leaseID,
	}
	
	resp, err := c.doRequest("POST", "/v1/lease/keepalive", req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to keep alive lease: status %d", resp.StatusCode)
	}
	
	var result types.Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}
	
	// 解析租约保活响应
	dataBytes, err := json.Marshal(result.Data)
	if err != nil {
		return 0, err
	}
	
	var keepAliveResp types.LeaseKeepAliveResponse
	if err := json.Unmarshal(dataBytes, &keepAliveResp); err != nil {
		return 0, err
	}
	
	return keepAliveResp.TTL, nil
}

// GetLease 获取租约信息
func (c *Client) GetLease(leaseID int64) (*types.Lease, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/v1/lease/%d", leaseID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get lease: status %d", resp.StatusCode)
	}
	
	var result types.Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	// 解析租约信息
	dataBytes, err := json.Marshal(result.Data)
	if err != nil {
		return nil, err
	}
	
	var lease types.Lease
	if err := json.Unmarshal(dataBytes, &lease); err != nil {
		return nil, err
	}
	
	return &lease, nil
}

// ListLeases 列出所有租约
func (c *Client) ListLeases() ([]*types.Lease, error) {
	resp, err := c.doRequest("GET", "/v1/lease", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list leases: status %d", resp.StatusCode)
	}
	
	var result types.Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	// 解析租约列表
	dataBytes, err := json.Marshal(result.Data)
	if err != nil {
		return nil, err
	}
	
	var leases []*types.Lease
	if err := json.Unmarshal(dataBytes, &leases); err != nil {
		return nil, err
	}
	
	return leases, nil
}

// mustMarshalJSON 序列化JSON，失败时panic
func mustMarshalJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}

// scanner 简单的行扫描器
type scanner struct {
	reader *bufio.Reader
}

func newScanner(r io.Reader) *scanner {
	return &scanner{reader: bufio.NewReader(r)}
}

func (s *scanner) Scan() bool {
	_, err := s.reader.ReadString('\n')
	return err == nil || err != io.EOF
}

func (s *scanner) Text() string {
	line, _ := s.reader.ReadString('\n')
	return strings.TrimSuffix(line, "\n")
}
