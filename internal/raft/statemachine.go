package raft

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"myetcd/internal/types"
)

/*
状态机实现

状态机是Raft算法中应用日志条目的组件。当Raft节点提交日志条目后，
这些条目会被应用到状态机上，从而改变系统的状态。

这个实现提供了一个简单的键值存储状态机，它支持以下操作：
- PUT: 存储键值对
- DELETE: 删除键
- GET: 获取键值（GET操作不会记录到日志中，因为它是只读的）

状态机的关键特性：
- 线程安全：使用互斥锁保护并发访问
- 持久化：支持快照和恢复
- 幂等性：重复应用相同的日志条目不会产生副作用
- 错误处理：对无效操作返回错误

在实际应用中，状态机可以是任何需要保证一致性的数据结构，
如数据库、配置系统、分布式锁服务等。
*/

// SimpleStateMachine 简单的状态机实现
type SimpleStateMachine struct {
	mu    sync.RWMutex          // 读写锁
	data  map[string][]byte     // 键值存储
	terms map[string]uint64     // 键的版本号（用于冲突检测）
}

// NewSimpleStateMachine 创建新的简单状态机
func NewSimpleStateMachine() *SimpleStateMachine {
	return &SimpleStateMachine{
		data:  make(map[string][]byte),
		terms: make(map[string]uint64),
	}
}

// Apply 应用日志条目到状态机
func (sm *SimpleStateMachine) Apply(entry types.LogEntry) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	// 跳过索引为0的空日志条目
	if entry.Index == 0 {
		log.Printf("Skipping empty log entry: index=0, term=%d", entry.Term)
		return nil
	}
	
	log.Printf("Applying log entry: index=%d, term=%d, type=%s, key=%s",
		entry.Index, entry.Term, entry.Command.Type.String(), entry.Command.Key)
	
	switch entry.Command.Type {
	case types.CommandPut:
		return sm.applyPut(entry)
	case types.CommandDelete:
		return sm.applyDelete(entry)
	case types.CommandGet:
		// GET操作不应该应用到状态机，因为它是只读的
		log.Printf("Warning: GET command in log entry, ignoring")
		return nil
	default:
		return fmt.Errorf("unknown command type: %d", entry.Command.Type)
	}
}

// applyPut 应用PUT操作
func (sm *SimpleStateMachine) applyPut(entry types.LogEntry) error {
	key := entry.Command.Key
	value := entry.Command.Value
	
	// 存储键值对
	sm.data[key] = value
	
	// 更新版本号
	sm.terms[key] = entry.Term
	
	log.Printf("PUT: key=%s, value=%s, term=%d", key, string(value), entry.Term)
	return nil
}

// applyDelete 应用DELETE操作
func (sm *SimpleStateMachine) applyDelete(entry types.LogEntry) error {
	key := entry.Command.Key
	
	// 删除键值对
	delete(sm.data, key)
	delete(sm.terms, key)
	
	log.Printf("DELETE: key=%s, term=%d", key, entry.Term)
	return nil
}

// Get 获取键值
func (sm *SimpleStateMachine) Get(key string) ([]byte, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	value, exists := sm.data[key]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	
	// 返回值的副本
	result := make([]byte, len(value))
	copy(result, value)
	return result, nil
}

// ListKeys 列出所有键
func (sm *SimpleStateMachine) ListKeys() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	keys := make([]string, 0, len(sm.data))
	for key := range sm.data {
		keys = append(keys, key)
	}
	
	return keys
}

// Size 获取键值对数量
func (sm *SimpleStateMachine) Size() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	return len(sm.data)
}

// Snapshot 创建状态机快照
func (sm *SimpleStateMachine) Snapshot() ([]byte, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	// 创建快照数据
	snapshot := map[string]interface{}{
		"data":  sm.data,
		"terms": sm.terms,
	}
	
	// 序列化快照
	data, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal snapshot: %w", err)
	}
	
	log.Printf("Created snapshot with %d key-value pairs", len(sm.data))
	return data, nil
}

// Restore 从快照恢复状态机
func (sm *SimpleStateMachine) Restore(data []byte) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	// 反序列化快照
	var snapshot map[string]interface{}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}
	
	// 恢复数据
	if dataField, ok := snapshot["data"].(map[string]interface{}); ok {
		newData := make(map[string][]byte)
		for key, value := range dataField {
			if valueBytes, ok := value.([]byte); ok {
				newData[key] = valueBytes
			} else if valueStr, ok := value.(string); ok {
				newData[key] = []byte(valueStr)
			}
		}
		sm.data = newData
	}
	
	// 恢复版本号
	if termsField, ok := snapshot["terms"].(map[string]interface{}); ok {
		newTerms := make(map[string]uint64)
		for key, value := range termsField {
			if valueFloat, ok := value.(float64); ok {
				newTerms[key] = uint64(valueFloat)
			}
		}
		sm.terms = newTerms
	}
	
	log.Printf("Restored snapshot with %d key-value pairs", len(sm.data))
	return nil
}

// GetTerm 获取键的版本号
func (sm *SimpleStateMachine) GetTerm(key string) (uint64, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	term, exists := sm.terms[key]
	if !exists {
		return 0, fmt.Errorf("key not found: %s", key)
	}
	
	return term, nil
}

// Clear 清空状态机
func (sm *SimpleStateMachine) Clear() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	sm.data = make(map[string][]byte)
	sm.terms = make(map[string]uint64)
	
	log.Printf("State machine cleared")
}

// DebugInfo 获取调试信息
func (sm *SimpleStateMachine) DebugInfo() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	info := map[string]interface{}{
		"key_count": len(sm.data),
		"keys":      make([]string, 0, len(sm.data)),
	}
	
	for key := range sm.data {
		info["keys"] = append(info["keys"].([]string), key)
	}
	
	return info
}