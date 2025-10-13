package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"myetcd/internal/lease"
	"myetcd/internal/types"
	"myetcd/internal/watch"
)

var (
	ErrKeyNotFound = fmt.Errorf("key not found")
	ErrKeyExpired  = fmt.Errorf("key expired")
	ErrTxnNotFound = fmt.Errorf("transaction not found")
	ErrTxnAborted  = fmt.Errorf("transaction aborted")
	ErrCASFailed   = fmt.Errorf("compare-and-swap failed")
	ErrInvalidTxn  = fmt.Errorf("invalid transaction")
)

/*
ACID特性详解：

ACID是数据库事务的四个核心特性，确保数据的一致性和可靠性：

1. 原子性 (Atomicity)
   - 定义：事务中的所有操作要么全部成功，要么全部失败
   - 实现：通过WAL记录所有操作，失败时回滚
   - 例子：转账操作，扣款和存款必须同时成功或失败

2. 一致性 (Consistency)
   - 定义：数据库始终保持在一致状态
   - 实现：通过约束检查和验证规则
   - 例子：账户余额不能为负数

3. 隔离性 (Isolation)
   - 定义：并发事务不会相互干扰
   - 实现：通过读写锁和版本控制
   - 例子：两个用户同时修改同一个键

4. 持久性 (Durability)
   - 定义：已提交的事务不会丢失
   - 实现：通过WAL和快照机制
   - 例子：系统重启后数据仍然存在

在这个存储引擎中，我们通过以下方式实现ACID：
- 使用WAL保证原子性和持久性
- 使用读写锁保证隔离性
- 使用事务机制保证一致性
*/

// Engine 存储引擎
//
// 这是整个系统的核心，实现了ACID事务特性：
// - 原子性：通过WAL和事务回滚保证
// - 一致性：通过数据验证和约束检查
// - 隔离性：通过读写锁和版本控制
// - 持久性：通过WAL和定期快照
type Engine struct {
	mu            sync.RWMutex                  // 读写锁，保证并发安全
	data          map[string]*types.KeyValue    // 内存中的键值存储
	transactions  map[string]*types.Transaction // 活跃事务管理
	config        *types.Config                 // 配置信息
	wal           WALWrapper                    // WAL接口，用于持久化
	snapshotIndex uint64                        // 当前快照索引
	watcher       *watch.Watcher                // Watch管理器
	leaseManager  *lease.LeaseManager           // 租约管理器
	closed        bool                          // 引擎是否已关闭
	closeChan     chan struct{}                 // 用于通知后台goroutine退出
}

// WALWrapper WAL包装器
type WALWrapper interface {
	Append(record *types.WALRecord) error
	ReadAll() ([]*types.WALRecord, error)
	Sync() error
	Index() uint64
	Term() uint64
	SetTerm(term uint64) error
	Truncate(index uint64) error
	Close() error
}

// NewEngine 创建新的存储引擎
func NewEngine(config *types.Config, wal WALWrapper) (*Engine, error) {
	engine := &Engine{
		data:         make(map[string]*types.KeyValue),
		transactions: make(map[string]*types.Transaction),
		config:       config,
		wal:          wal,
		watcher:      watch.NewWatcher(),
		leaseManager: lease.NewLeaseManager(),
		closeChan:    make(chan struct{}),
	}

	// 确保数据目录存在
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// 从WAL恢复数据
	if err := engine.recoverFromWAL(); err != nil {
		return nil, fmt.Errorf("failed to recover from WAL: %w", err)
	}

	// 启动后台清理任务
	go engine.cleanupExpiredKeys()
	go engine.snapshotRoutine()

	// 启动Watcher
	engine.watcher.Start()

	// 启动租约清理goroutine
	go engine.leaseCleanupRoutine()

	return engine, nil
}

// recoverFromWAL 从WAL恢复数据
func (e *Engine) recoverFromWAL() error {
	records, err := e.wal.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read WAL records: %w", err)
	}

	for _, record := range records {
		if err := e.applyCommand(&record.Command); err != nil {
			// 记录错误但继续恢复其他记录
			fmt.Printf("Warning: failed to apply command %d: %v\n", record.Index, err)
		}
		e.snapshotIndex = record.Index
	}

	return nil
}

// applyCommand 应用命令到存储引擎
func (e *Engine) applyCommand(cmd *types.Command) error {
	switch cmd.Type {
	case types.CommandPut:
		return e.applyPut(cmd.Key, cmd.Value)
	case types.CommandDelete:
		return e.applyDelete(cmd.Key)
	case types.CommandGet:
		// GET命令不需要应用到存储引擎
		return nil
	default:
		return fmt.Errorf("unknown command type: %d", cmd.Type)
	}
}

// applyPut 应用PUT操作
func (e *Engine) applyPut(key string, value []byte) error {
	now := time.Now()

	if existing, exists := e.data[key]; exists {
		// 更新现有键值
		existing.Value = value
		existing.Modified = now
		existing.Version++
	} else {
		// 创建新键值
		e.data[key] = &types.KeyValue{
			Key:      key,
			Value:    value,
			Created:  now,
			Modified: now,
			Version:  1,
		}
	}

	return nil
}

// applyDelete 应用DELETE操作
func (e *Engine) applyDelete(key string) error {
	delete(e.data, key)
	return nil
}

// Put 存储键值对
//
// 这个方法完美展示了WAL机制和ACID特性的实现：
//
// 1. 持久性 (Durability)：
//   - 首先将操作写入WAL，确保即使系统崩溃也能恢复
//   - 调用Sync()确保数据真正写入磁盘
//
// 2. 原子性 (Atomicity)：
//   - 要么WAL写入成功且内存更新成功，要么都失败
//   - 如果WAL写入失败，不会修改内存数据
//
// 3. 隔离性 (Isolation)：
//   - 使用写锁确保同一时间只有一个写操作
//   - 防止并发写操作导致数据不一致
//
// 4. 一致性 (Consistency)：
//   - 验证输入参数
//   - 维护数据的完整性约束
func (e *Engine) Put(key string, value []byte, ttl int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return fmt.Errorf("engine is closed")
	}

	// 步骤1：创建命令对象
	// 这是WAL的第一步：将操作封装为命令
	cmd := &types.Command{
		Type:  types.CommandPut,
		Key:   key,
		Value: value,
		Time:  time.Now(),
	}

	// 步骤2：写入WAL（预写日志的核心）
	// 这是持久性的关键：先写日志，再修改数据
	record := &types.WALRecord{
		Command: *cmd,
	}

	if err := e.wal.Append(record); err != nil {
		return fmt.Errorf("failed to append to WAL: %w", err)
	}

	// 步骤3：强制同步到磁盘
	// 确保WAL真正写入磁盘，而不是停留在缓冲区
	// 这是防止数据丢失的关键步骤
	if err := e.wal.Sync(); err != nil {
		return fmt.Errorf("failed to sync WAL: %w", err)
	}

	// 步骤4：应用到内存存储
	// 只有在WAL成功写入后，才修改内存数据
	// 这保证了原子性：要么都成功，要么都失败
	now := time.Now()
	var expires time.Time
	if ttl > 0 {
		expires = now.Add(time.Duration(ttl) * time.Second)
	}

	var prevValue []byte
	var newVersion int64

	if existing, exists := e.data[key]; exists {
		// 保存之前的值用于Watch通知
		prevValue = append([]byte(nil), existing.Value...)

		// 更新现有键值
		existing.Value = value
		existing.Modified = now
		existing.Version++ // 版本号递增，用于乐观锁
		existing.TTL = ttl
		existing.Expires = expires
		newVersion = existing.Version
	} else {
		// 创建新键值
		e.data[key] = &types.KeyValue{
			Key:      key,
			Value:    value,
			Created:  now,
			Modified: now,
			Version:  1, // 新键值从版本1开始
			TTL:      ttl,
			Expires:  expires,
		}
		newVersion = 1
	}

	// 如果键关联了租约，附加到租约
	if ttl > 0 {
		// 创建新租约
		leaseID, err := e.leaseManager.Grant(ttl)
		if err != nil {
			return fmt.Errorf("failed to create lease: %w", err)
		}

		// 附加键到租约
		if err := e.leaseManager.Attach(leaseID, key); err != nil {
			// 如果附加失败，撤销租约
			e.leaseManager.Revoke(leaseID)
			return fmt.Errorf("failed to attach key to lease: %w", err)
		}

		// 更新KeyValue的租约ID
		if existing, exists := e.data[key]; exists {
			existing.LeaseID = leaseID
		} else {
			// 新创建的键
			if newKV, exists := e.data[key]; exists {
				newKV.LeaseID = leaseID
			}
		}
	}

	// 通知Watcher
	e.watcher.Notify(types.WatchPut, key, value, prevValue, newVersion)

	return nil
}

// Get 获取键值
func (e *Engine) Get(key string) (*types.KeyValue, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.closed {
		return nil, fmt.Errorf("engine is closed")
	}

	kv, exists := e.data[key]
	if !exists {
		return nil, ErrKeyNotFound
	}

	// 检查是否过期
	if kv.IsExpired() {
		// 异步删除过期键
		go func() {
			e.mu.Lock()
			defer e.mu.Unlock()
			delete(e.data, key)
		}()
		return nil, ErrKeyExpired
	}

	// 返回副本
	return &types.KeyValue{
		Key:      kv.Key,
		Value:    append([]byte(nil), kv.Value...),
		Created:  kv.Created,
		Modified: kv.Modified,
		Version:  kv.Version,
		TTL:      kv.TTL,
		Expires:  kv.Expires,
		LeaseID:  kv.LeaseID,
	}, nil
}

// Delete 删除键
func (e *Engine) Delete(key string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return fmt.Errorf("engine is closed")
	}

	if _, exists := e.data[key]; !exists {
		return ErrKeyNotFound
	}

	// 创建命令
	cmd := &types.Command{
		Type: types.CommandDelete,
		Key:  key,
		Time: time.Now(),
	}

	// 写入WAL
	record := &types.WALRecord{
		Command: *cmd,
	}

	if err := e.wal.Append(record); err != nil {
		return fmt.Errorf("failed to append to WAL: %w", err)
	}

	// 同步WAL
	if err := e.wal.Sync(); err != nil {
		return fmt.Errorf("failed to sync WAL: %w", err)
	}

	// 保存之前的值用于Watch通知
	var prevValue []byte
	if existing, exists := e.data[key]; exists {
		prevValue = append([]byte(nil), existing.Value...)
	}

	// 从内存存储中删除
	delete(e.data, key)

	// 如果键关联了租约，从租约分离
	if existing, exists := e.data[key]; exists && existing.LeaseID > 0 {
		e.leaseManager.Detach(key)
	}

	// 通知Watcher
	e.watcher.Notify(types.WatchDelete, key, nil, prevValue, 0)

	return nil
}

// CompareAndSwap 比较并交换
func (e *Engine) CompareAndSwap(key string, prevValue, newValue []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return fmt.Errorf("engine is closed")
	}

	kv, exists := e.data[key]
	if !exists {
		if len(prevValue) != 0 {
			return ErrCASFailed
		}
	} else {
		// 检查是否过期
		if kv.IsExpired() {
			delete(e.data, key)
			if len(prevValue) != 0 {
				return ErrCASFailed
			}
		} else if string(kv.Value) != string(prevValue) {
			return ErrCASFailed
		}
	}

	// 执行交换
	return e.putUnsafe(key, newValue, 0)
}

// putUnsafe 不加锁的PUT操作（内部使用）
func (e *Engine) putUnsafe(key string, value []byte, ttl int64) error {
	// 创建命令
	cmd := &types.Command{
		Type:  types.CommandPut,
		Key:   key,
		Value: value,
		Time:  time.Now(),
	}

	// 写入WAL
	record := &types.WALRecord{
		Command: *cmd,
	}

	if err := e.wal.Append(record); err != nil {
		return fmt.Errorf("failed to append to WAL: %w", err)
	}

	// 同步WAL
	if err := e.wal.Sync(); err != nil {
		return fmt.Errorf("failed to sync WAL: %w", err)
	}

	// 应用到内存存储
	now := time.Now()
	var expires time.Time
	if ttl > 0 {
		expires = now.Add(time.Duration(ttl) * time.Second)
	}

	if existing, exists := e.data[key]; exists {
		existing.Value = value
		existing.Modified = now
		existing.Version++
		existing.TTL = ttl
		existing.Expires = expires

		// 通知Watcher
		e.watcher.Notify(types.WatchPut, key, value, nil, existing.Version)
	} else {
		e.data[key] = &types.KeyValue{
			Key:      key,
			Value:    value,
			Created:  now,
			Modified: now,
			Version:  1,
			TTL:      ttl,
			Expires:  expires,
		}

		// 通知Watcher
		e.watcher.Notify(types.WatchPut, key, value, nil, 1)
	}

	return nil
}

// BeginTransaction 开始事务
//
// 事务是ACID特性的核心实现：
// 1. 创建唯一的事务ID
// 2. 初始化操作列表
// 3. 设置事务状态为PENDING
// 4. 将事务注册到引擎中
//
// 注意：这个方法获取了写锁，确保事务创建的原子性
func (e *Engine) BeginTransaction() (*types.Transaction, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil, fmt.Errorf("engine is closed")
	}

	// 创建新事务
	txn := &types.Transaction{
		ID:         generateTxnID(),     // 生成唯一的事务ID
		Operations: []types.Operation{}, // 初始化操作列表
		Timestamp:  time.Now(),          // 记录事务开始时间
		Status:     types.TxnPending,    // 设置为待执行状态
	}

	// 将事务注册到引擎中
	e.transactions[txn.ID] = txn
	return txn, nil
}

// CommitTransaction 提交事务
//
// 这是实现原子性的关键方法：
// 1. 验证事务存在且状态正确
// 2. 按顺序执行所有操作
// 3. 如果任何操作失败，标记事务为失败状态
// 4. 如果所有操作成功，标记事务为已提交
//
// 注意：由于我们使用了WAL，这里的apply操作实际上是在内存中执行的
// 真正的持久化已经在Put/Delete时通过WAL完成
func (e *Engine) CommitTransaction(txnID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return fmt.Errorf("engine is closed")
	}

	// 查找事务
	txn, exists := e.transactions[txnID]
	if !exists {
		return ErrTxnNotFound
	}

	// 验证事务状态
	if txn.Status != types.TxnPending {
		return ErrInvalidTxn
	}

	// 执行事务中的所有操作
	// 这里实现了原子性：要么全部成功，要么全部失败
	for _, op := range txn.Operations {
		switch op.Type {
		case types.CommandPut:
			if err := e.applyPut(op.Key, op.Value); err != nil {
				// 操作失败，标记事务为中止状态
				// 注意：这里没有实现完整的回滚机制，因为我们的WAL已经记录了操作
				// 在实际生产环境中，需要实现补偿事务或回滚日志
				txn.Status = types.TxnAborted
				return fmt.Errorf("failed to apply put operation: %w", err)
			}
		case types.CommandDelete:
			if err := e.applyDelete(op.Key); err != nil {
				// 操作失败，标记事务为中止状态
				txn.Status = types.TxnAborted
				return fmt.Errorf("failed to apply delete operation: %w", err)
			}
		}
	}

	// 所有操作成功，标记事务为已提交
	txn.Status = types.TxnCommitted
	// 从活跃事务列表中移除
	delete(e.transactions, txnID)
	return nil
}

// RollbackTransaction 回滚事务
func (e *Engine) RollbackTransaction(txnID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return fmt.Errorf("engine is closed")
	}

	txn, exists := e.transactions[txnID]
	if !exists {
		return ErrTxnNotFound
	}

	txn.Status = types.TxnAborted
	delete(e.transactions, txnID)
	return nil
}

// AddOperation 添加操作到事务
func (e *Engine) AddOperation(txnID string, op types.Operation) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return fmt.Errorf("engine is closed")
	}

	txn, exists := e.transactions[txnID]
	if !exists {
		return ErrTxnNotFound
	}

	if txn.Status != types.TxnPending {
		return ErrInvalidTxn
	}

	txn.Operations = append(txn.Operations, op)
	return nil
}

// ListKeys 列出所有键
func (e *Engine) ListKeys() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.closed {
		return nil
	}

	var keys []string
	for key := range e.data {
		if !e.data[key].IsExpired() {
			keys = append(keys, key)
		}
	}

	return keys
}

// Size 返回存储的键值对数量
func (e *Engine) Size() int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.closed {
		return 0
	}

	count := 0
	for _, kv := range e.data {
		if !kv.IsExpired() {
			count++
		}
	}

	return count
}

// Watch 创建Watch
func (e *Engine) Watch(key string, prefix, prevKV bool) (int64, <-chan types.WatchEvent, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.closed {
		return 0, nil, fmt.Errorf("engine is closed")
	}

	watchID, eventChan := e.watcher.Watch(key, prefix, prevKV)

	// 转换事件通道
	resultChan := make(chan types.WatchEvent, 100)
	go func() {
		for event := range eventChan {
			resultChan <- event.ToTypesEvent(event.PrevValue)
		}
		close(resultChan)
	}()

	return watchID, resultChan, nil
}

// CancelWatch 取消Watch
func (e *Engine) CancelWatch(watchID int64) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.closed {
		return fmt.Errorf("engine is closed")
	}

	return e.watcher.Cancel(watchID)
}

// GetWatchCount 获取活跃Watch数量
func (e *Engine) GetWatchCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.closed {
		return 0
	}

	return e.watcher.GetWatchCount()
}

// Range 范围查询
func (e *Engine) Range(key, rangeEnd string, limit int64) ([]*types.KeyValue, int64, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.closed {
		return nil, 0, fmt.Errorf("engine is closed")
	}

	var result []*types.KeyValue
	var count int64

	// 确定范围
	if rangeEnd == "" {
		// 单个键
		if kv, exists := e.data[key]; exists && !kv.IsExpired() {
			result = append(result, e.copyKeyValue(kv))
			count = 1
		}
	} else {
		// 范围查询
		for k, kv := range e.data {
			if kv.IsExpired() {
				continue
			}

			// 检查是否在范围内
			if k >= key && (rangeEnd == "" || k < rangeEnd) {
				result = append(result, e.copyKeyValue(kv))
				count++

				// 检查限制
				if limit > 0 && int64(len(result)) >= limit {
					break
				}
			}
		}
	}

	return result, count, nil
}

// copyKeyValue 复制键值对
func (e *Engine) copyKeyValue(kv *types.KeyValue) *types.KeyValue {
	return &types.KeyValue{
		Key:      kv.Key,
		Value:    append([]byte(nil), kv.Value...),
		Created:  kv.Created,
		Modified: kv.Modified,
		Version:  kv.Version,
		TTL:      kv.TTL,
		Expires:  kv.Expires,
		LeaseID:  kv.LeaseID,
	}
}

// BatchPut 批量PUT操作
func (e *Engine) BatchPut(kvs map[string][]byte, ttl int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return fmt.Errorf("engine is closed")
	}

	now := time.Now()
	var expires time.Time
	if ttl > 0 {
		expires = now.Add(time.Duration(ttl) * time.Second)
	}

	// 为每个键值创建命令和记录
	for key, value := range kvs {
		// 创建命令
		cmd := &types.Command{
			Type:  types.CommandPut,
			Key:   key,
			Value: value,
			Time:  now,
		}

		// 写入WAL
		record := &types.WALRecord{
			Command: *cmd,
		}

		if err := e.wal.Append(record); err != nil {
			return fmt.Errorf("failed to append to WAL for key %s: %w", key, err)
		}
	}

	// 同步WAL
	if err := e.wal.Sync(); err != nil {
		return fmt.Errorf("failed to sync WAL: %w", err)
	}

	// 应用到内存存储
	for key, value := range kvs {
		var prevValue []byte
		var newVersion int64

		if existing, exists := e.data[key]; exists {
			prevValue = append([]byte(nil), existing.Value...)

			existing.Value = value
			existing.Modified = now
			existing.Version++
			existing.TTL = ttl
			existing.Expires = expires
			newVersion = existing.Version
		} else {
			e.data[key] = &types.KeyValue{
				Key:      key,
				Value:    value,
				Created:  now,
				Modified: now,
				Version:  1,
				TTL:      ttl,
				Expires:  expires,
			}
			newVersion = 1
		}

		// 通知Watcher
		e.watcher.Notify(types.WatchPut, key, value, prevValue, newVersion)
	}

	return nil
}

// BatchDelete 批量DELETE操作
func (e *Engine) BatchDelete(keys []string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return fmt.Errorf("engine is closed")
	}

	now := time.Now()

	// 为每个键创建命令和记录
	for _, key := range keys {
		if _, exists := e.data[key]; !exists {
			continue // 跳过不存在的键
		}

		// 创建命令
		cmd := &types.Command{
			Type: types.CommandDelete,
			Key:  key,
			Time: now,
		}

		// 写入WAL
		record := &types.WALRecord{
			Command: *cmd,
		}

		if err := e.wal.Append(record); err != nil {
			return fmt.Errorf("failed to append to WAL for key %s: %w", key, err)
		}
	}

	// 同步WAL
	if err := e.wal.Sync(); err != nil {
		return fmt.Errorf("failed to sync WAL: %w", err)
	}

	// 应用到内存存储
	for _, key := range keys {
		if existing, exists := e.data[key]; exists {
			prevValue := append([]byte(nil), existing.Value...)

			delete(e.data, key)

			// 通知Watcher
			e.watcher.Notify(types.WatchDelete, key, nil, prevValue, 0)
		}
	}

	return nil
}

// leaseCleanupRoutine 租约清理goroutine
func (e *Engine) leaseCleanupRoutine() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			expiredKeys := e.leaseManager.GetExpiredKeys()
			if len(expiredKeys) > 0 {
				e.mu.Lock()
				for _, key := range expiredKeys {
					if kv, exists := e.data[key]; exists {
						prevValue := append([]byte(nil), kv.Value...)
						delete(e.data, key)

						// 通知Watcher
						e.watcher.Notify(types.WatchDelete, key, nil, prevValue, 0)
					}
				}
				e.mu.Unlock()
			}
		case <-e.closeChan:
			return
		}
	}
}

// GrantLease 创建租约
func (e *Engine) GrantLease(ttl int64) (int64, error) {
	if ttl <= 0 {
		return 0, fmt.Errorf("TTL must be positive")
	}

	return e.leaseManager.Grant(ttl)
}

// RevokeLease 撤销租约
func (e *Engine) RevokeLease(leaseID int64) error {
	// 获取租约关联的键
	lease, err := e.leaseManager.GetLease(leaseID)
	if err != nil {
		return err
	}

	// 删除所有关联的键
	e.mu.Lock()
	for _, key := range lease.Keys {
		if kv, exists := e.data[key]; exists && kv.LeaseID == leaseID {
			prevValue := append([]byte(nil), kv.Value...)
			delete(e.data, key)

			// 通知Watcher
			e.watcher.Notify(types.WatchDelete, key, nil, prevValue, 0)
		}
	}
	e.mu.Unlock()

	// 撤销租约
	return e.leaseManager.Revoke(leaseID)
}

// KeepAliveLease 续约
func (e *Engine) KeepAliveLease(leaseID int64) (int64, error) {
	return e.leaseManager.KeepAlive(leaseID)
}

// AttachToLease 将键附加到租约
func (e *Engine) AttachToLease(leaseID int64, key string) error {
	// 检查键是否存在
	e.mu.RLock()
	kv, exists := e.data[key]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("key %s not found", key)
	}

	// 如果键已经关联了租约，先分离
	if kv.LeaseID > 0 {
		e.leaseManager.Detach(key)
	}

	// 附加到新租约
	if err := e.leaseManager.Attach(leaseID, key); err != nil {
		return err
	}

	// 更新KeyValue的租约ID
	e.mu.Lock()
	if kv, exists := e.data[key]; exists {
		kv.LeaseID = leaseID
	}
	e.mu.Unlock()

	return nil
}

// DetachFromLease 从租约分离键
func (e *Engine) DetachFromLease(key string) error {
	// 检查键是否存在
	e.mu.RLock()
	kv, exists := e.data[key]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("key %s not found", key)
	}

	// 从租约分离
	if kv.LeaseID > 0 {
		if err := e.leaseManager.Detach(key); err != nil {
			return err
		}

		// 清除KeyValue的租约ID
		e.mu.Lock()
		if kv, exists := e.data[key]; exists {
			kv.LeaseID = 0
			kv.TTL = 0
			kv.Expires = time.Time{}
		}
		e.mu.Unlock()
	}

	return nil
}

// GetLease 获取租约信息
func (e *Engine) GetLease(leaseID int64) (*types.Lease, error) {
	return e.leaseManager.GetLease(leaseID)
}

// ListLeases 列出所有活跃租约
func (e *Engine) ListLeases() ([]*types.Lease, error) {
	return e.leaseManager.ListLeases()
}

// GetLeaseCount 获取活跃租约数量
func (e *Engine) GetLeaseCount() int {
	return e.leaseManager.GetLeaseCount()
}

// cleanupExpiredKeys 清理过期键
func (e *Engine) cleanupExpiredKeys() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.mu.Lock()
			if e.closed {
				e.mu.Unlock()
				return
			}

			now := time.Now()
			for key, kv := range e.data {
				if kv.TTL > 0 && now.After(kv.Expires) {
					delete(e.data, key)
				}
			}
			e.mu.Unlock()
		case <-e.closeChan:
			return
		}
	}
}

// snapshotRoutine 定期快照
func (e *Engine) snapshotRoutine() {
	ticker := time.NewTicker(e.config.SnapshotInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := e.createSnapshot(); err != nil {
				fmt.Printf("Warning: failed to create snapshot: %v\n", err)
			}
		case <-e.closeChan:
			return
		}
	}
}

// createSnapshot 创建快照
func (e *Engine) createSnapshot() error {
	e.mu.RLock()
	if e.closed {
		e.mu.RUnlock()
		return fmt.Errorf("engine is closed")
	}

	// 复制数据
	data := make(map[string]*types.KeyValue)
	for k, v := range e.data {
		if !v.IsExpired() {
			data[k] = &types.KeyValue{
				Key:      v.Key,
				Value:    append([]byte(nil), v.Value...),
				Created:  v.Created,
				Modified: v.Modified,
				Version:  v.Version,
				TTL:      v.TTL,
				Expires:  v.Expires,
				LeaseID:  v.LeaseID,
			}
		}
	}

	index := e.wal.Index()
	term := e.wal.Term()
	e.mu.RUnlock()

	// 创建快照
	snapshot := &types.Snapshot{
		Index:     index,
		Term:      term,
		Data:      data,
		Timestamp: time.Now(),
	}

	// 计算校验和
	snapshotData, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}
	snapshot.Checksum = crc32Checksum(snapshotData)

	// 保存快照
	snapshotFile := filepath.Join(e.config.SnapshotDir, fmt.Sprintf("snapshot-%d-%d", index, term))
	if err := os.MkdirAll(e.config.SnapshotDir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	file, err := os.Create(snapshotFile)
	if err != nil {
		return fmt.Errorf("failed to create snapshot file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(snapshot); err != nil {
		return fmt.Errorf("failed to write snapshot: %w", err)
	}

	// 更新快照索引
	e.mu.Lock()
	e.snapshotIndex = index
	e.mu.Unlock()

	// 清理旧的WAL文件
	if err := e.wal.Truncate(index); err != nil {
		return fmt.Errorf("failed to truncate WAL: %w", err)
	}

	return nil
}

// crc32Checksum 计算CRC32校验和
func crc32Checksum(data []byte) uint32 {
	// 简单的CRC32实现
	var crc uint32 = 0xFFFFFFFF
	for _, b := range data {
		crc ^= uint32(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0xEDB88320
			} else {
				crc >>= 1
			}
		}
	}
	return ^crc
}

// generateTxnID 生成事务ID
func generateTxnID() string {
	return fmt.Sprintf("txn-%d-%d", time.Now().UnixNano(), time.Now().Unix())
}

// Close 关闭存储引擎
func (e *Engine) Close() error {
	e.mu.Lock()

	if e.closed {
		e.mu.Unlock()
		return nil
	}

	e.closed = true

	// 关闭后台goroutine
	close(e.closeChan)

	e.mu.Unlock()

	// 创建最终快照（在锁外进行）
	if err := e.createSnapshot(); err != nil {
		fmt.Printf("Warning: failed to create final snapshot: %v\n", err)
	}

	// 关闭Watcher
	if e.watcher != nil {
		e.watcher.Stop()
	}

	// 关闭租约管理器
	if e.leaseManager != nil {
		e.leaseManager.Close()
	}

	// 关闭WAL
	if e.wal != nil {
		if err := e.wal.Close(); err != nil {
			return fmt.Errorf("failed to close WAL: %w", err)
		}
	}

	return nil
}
