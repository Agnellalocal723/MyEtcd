package lease

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"myetcd/internal/types"
)

/*
租约机制详解：

租约（Lease）是一种自动过期机制，允许将键的生存时间与租约关联。
当租约过期时，所有关联的键都会被自动删除。

1. 租约的工作原理：
   - 客户端创建一个具有TTL的租约
   - 将键与租约关联，键的生存时间由租约控制
   - 客户端可以通过KeepAlive续约
   - 租约过期时，自动删除所有关联的键

2. 租约的特性：
   - 自动过期：租约到期自动清理关联键
   - 续约机制：客户端可以延长租约时间
   - 批量管理：一个租约可以关联多个键
   - 高效清理：避免大量键的单独TTL管理

3. 应用场景：
   - 临时节点：服务注册和发现
   - 分布式锁：锁的自动释放
   - 会话管理：用户会话的超时处理
*/

// LeaseManager 租约管理器
type LeaseManager struct {
	mu            sync.RWMutex
	leases        map[int64]*Lease   // 活跃租约
	keyToLease    map[string]int64   // 键到租约的映射
	nextLeaseID   int64              // 下一个租约ID
	revokedLeases map[int64]struct{} // 已撤销的租约
	closeChan     chan struct{}      // 关闭通道
	closed        int32              // 是否已关闭
}

// Lease 租约
type Lease struct {
	ID        int64     `json:"id"`
	TTL       int64     `json:"ttl"`       // 租约TTL（秒）
	Remaining int64     `json:"remaining"` // 剩余时间（秒）
	Granted   time.Time `json:"granted"`   // 授予时间
	Expires   time.Time `json:"expires"`   // 过期时间
	Keys      []string  `json:"keys"`      // 关联的键
	mu        sync.RWMutex
}

// NewLeaseManager 创建新的租约管理器
func NewLeaseManager() *LeaseManager {
	lm := &LeaseManager{
		leases:        make(map[int64]*Lease),
		keyToLease:    make(map[string]int64),
		nextLeaseID:   1,
		revokedLeases: make(map[int64]struct{}),
		closeChan:     make(chan struct{}),
	}

	// 启动清理goroutine
	go lm.cleanupRoutine()

	return lm
}

// Grant 创建新租约
func (lm *LeaseManager) Grant(ttl int64) (int64, error) {
	if atomic.LoadInt32(&lm.closed) == 1 {
		return 0, fmt.Errorf("lease manager is closed")
	}

	if ttl <= 0 {
		return 0, fmt.Errorf("TTL must be positive")
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()

	leaseID := atomic.AddInt64(&lm.nextLeaseID, 1)

	now := time.Now()
	lease := &Lease{
		ID:      leaseID,
		TTL:     ttl,
		Granted: now,
		Expires: now.Add(time.Duration(ttl) * time.Second),
		Keys:    make([]string, 0),
	}

	lm.leases[leaseID] = lease

	return leaseID, nil
}

// Revoke 撤销租约
func (lm *LeaseManager) Revoke(leaseID int64) error {
	if atomic.LoadInt32(&lm.closed) == 1 {
		return fmt.Errorf("lease manager is closed")
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()

	lease, exists := lm.leases[leaseID]
	if !exists {
		return fmt.Errorf("lease %d not found", leaseID)
	}

	// 标记为已撤销
	lm.revokedLeases[leaseID] = struct{}{}

	// 删除租约
	delete(lm.leases, leaseID)

	// 清理键映射
	for _, key := range lease.Keys {
		delete(lm.keyToLease, key)
	}

	return nil
}

// KeepAlive 续约
func (lm *LeaseManager) KeepAlive(leaseID int64) (int64, error) {
	if atomic.LoadInt32(&lm.closed) == 1 {
		return 0, fmt.Errorf("lease manager is closed")
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()

	lease, exists := lm.leases[leaseID]
	if !exists {
		return 0, fmt.Errorf("lease %d not found", leaseID)
	}

	// 检查是否已撤销
	if _, revoked := lm.revokedLeases[leaseID]; revoked {
		return 0, fmt.Errorf("lease %d is revoked", leaseID)
	}

	// 更新过期时间
	lease.Expires = time.Now().Add(time.Duration(lease.TTL) * time.Second)

	// 返回新的TTL
	return lease.TTL, nil
}

// Attach 将键附加到租约
func (lm *LeaseManager) Attach(leaseID int64, key string) error {
	if atomic.LoadInt32(&lm.closed) == 1 {
		return fmt.Errorf("lease manager is closed")
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()

	lease, exists := lm.leases[leaseID]
	if !exists {
		return fmt.Errorf("lease %d not found", leaseID)
	}

	// 检查是否已撤销
	if _, revoked := lm.revokedLeases[leaseID]; revoked {
		return fmt.Errorf("lease %d is revoked", leaseID)
	}

	// 如果键已经关联了其他租约，先解除关联
	if oldLeaseID, exists := lm.keyToLease[key]; exists {
		if oldLeaseID != leaseID {
			lm.detachFromLease(oldLeaseID, key)
		}
	}

	// 附加到新租约
	lease.mu.Lock()
	lease.Keys = append(lease.Keys, key)
	lease.mu.Unlock()

	lm.keyToLease[key] = leaseID

	return nil
}

// Detach 从租约分离键
func (lm *LeaseManager) Detach(key string) error {
	if atomic.LoadInt32(&lm.closed) == 1 {
		return fmt.Errorf("lease manager is closed")
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()

	leaseID, exists := lm.keyToLease[key]
	if !exists {
		return fmt.Errorf("key %s is not attached to any lease", key)
	}

	return lm.detachFromLease(leaseID, key)
}

// detachFromLease 从租约分离键（内部方法，需要持有锁）
func (lm *LeaseManager) detachFromLease(leaseID int64, key string) error {
	lease, exists := lm.leases[leaseID]
	if !exists {
		return fmt.Errorf("lease %d not found", leaseID)
	}

	// 从租约的键列表中移除
	lease.mu.Lock()
	for i, k := range lease.Keys {
		if k == key {
			lease.Keys = append(lease.Keys[:i], lease.Keys[i+1:]...)
			break
		}
	}
	lease.mu.Unlock()

	// 从键映射中移除
	delete(lm.keyToLease, key)

	return nil
}

// GetLease 获取租约信息
func (lm *LeaseManager) GetLease(leaseID int64) (*types.Lease, error) {
	if atomic.LoadInt32(&lm.closed) == 1 {
		return nil, fmt.Errorf("lease manager is closed")
	}

	lm.mu.RLock()
	defer lm.mu.RUnlock()

	lease, exists := lm.leases[leaseID]
	if !exists {
		return nil, fmt.Errorf("lease %d not found", leaseID)
	}

	// 检查是否已撤销
	if _, revoked := lm.revokedLeases[leaseID]; revoked {
		return nil, fmt.Errorf("lease %d is revoked", leaseID)
	}

	// 计算剩余时间
	now := time.Now()
	remaining := int64(lease.Expires.Sub(now).Seconds())
	if remaining < 0 {
		remaining = 0
	}

	// 复制键列表
	lease.mu.RLock()
	keys := make([]string, len(lease.Keys))
	copy(keys, lease.Keys)
	lease.mu.RUnlock()

	return &types.Lease{
		ID:        lease.ID,
		TTL:       lease.TTL,
		Remaining: remaining,
		Granted:   lease.Granted,
		Keys:      keys,
	}, nil
}

// GetLeaseByKey 根据键获取租约信息
func (lm *LeaseManager) GetLeaseByKey(key string) (*types.Lease, error) {
	if atomic.LoadInt32(&lm.closed) == 1 {
		return nil, fmt.Errorf("lease manager is closed")
	}

	lm.mu.RLock()
	defer lm.mu.RUnlock()

	leaseID, exists := lm.keyToLease[key]
	if !exists {
		return nil, fmt.Errorf("key %s is not attached to any lease", key)
	}

	return lm.getLeaseUnsafe(leaseID)
}

// getLeaseUnsafe 获取租约信息（内部方法，需要持有读锁）
func (lm *LeaseManager) getLeaseUnsafe(leaseID int64) (*types.Lease, error) {
	lease, exists := lm.leases[leaseID]
	if !exists {
		return nil, fmt.Errorf("lease %d not found", leaseID)
	}

	// 检查是否已撤销
	if _, revoked := lm.revokedLeases[leaseID]; revoked {
		return nil, fmt.Errorf("lease %d is revoked", leaseID)
	}

	// 计算剩余时间
	now := time.Now()
	remaining := int64(lease.Expires.Sub(now).Seconds())
	if remaining < 0 {
		remaining = 0
	}

	// 复制键列表
	lease.mu.RLock()
	keys := make([]string, len(lease.Keys))
	copy(keys, lease.Keys)
	lease.mu.RUnlock()

	return &types.Lease{
		ID:        lease.ID,
		TTL:       lease.TTL,
		Remaining: remaining,
		Granted:   lease.Granted,
		Keys:      keys,
	}, nil
}

// ListLeases 列出所有活跃租约
func (lm *LeaseManager) ListLeases() ([]*types.Lease, error) {
	if atomic.LoadInt32(&lm.closed) == 1 {
		return nil, fmt.Errorf("lease manager is closed")
	}

	lm.mu.RLock()
	defer lm.mu.RUnlock()

	leases := make([]*types.Lease, 0, len(lm.leases))

	for leaseID := range lm.leases {
		// 跳过已撤销的租约
		if _, revoked := lm.revokedLeases[leaseID]; revoked {
			continue
		}

		lease, err := lm.getLeaseUnsafe(leaseID)
		if err != nil {
			continue
		}

		leases = append(leases, lease)
	}

	return leases, nil
}

// GetExpiredKeys 获取过期的键
func (lm *LeaseManager) GetExpiredKeys() []string {
	if atomic.LoadInt32(&lm.closed) == 1 {
		return nil
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()

	now := time.Now()
	expiredKeys := make([]string, 0)

	// 检查所有租约
	for leaseID, lease := range lm.leases {
		// 跳过已撤销的租约
		if _, revoked := lm.revokedLeases[leaseID]; revoked {
			continue
		}

		// 检查是否过期
		if now.After(lease.Expires) {
			lease.mu.RLock()
			expiredKeys = append(expiredKeys, lease.Keys...)
			lease.mu.RUnlock()

			// 标记为已撤销
			lm.revokedLeases[leaseID] = struct{}{}

			// 删除租约
			delete(lm.leases, leaseID)

			// 清理键映射
			for _, key := range lease.Keys {
				delete(lm.keyToLease, key)
			}
		}
	}

	return expiredKeys
}

// cleanupRoutine 清理过期租约的goroutine
func (lm *LeaseManager) cleanupRoutine() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			lm.GetExpiredKeys() // 触发清理
		case <-lm.closeChan:
			return
		}
	}
}

// Close 关闭租约管理器
func (lm *LeaseManager) Close() {
	if !atomic.CompareAndSwapInt32(&lm.closed, 0, 1) {
		return // 已经关闭
	}

	close(lm.closeChan)

	lm.mu.Lock()
	defer lm.mu.Unlock()

	// 清理所有租约
	lm.leases = make(map[int64]*Lease)
	lm.keyToLease = make(map[string]int64)
	lm.revokedLeases = make(map[int64]struct{})
}

// GetLeaseCount 获取活跃租约数量
func (lm *LeaseManager) GetLeaseCount() int {
	if atomic.LoadInt32(&lm.closed) == 1 {
		return 0
	}

	lm.mu.RLock()
	defer lm.mu.RUnlock()

	count := 0
	for leaseID := range lm.leases {
		// 跳过已撤销的租约
		if _, revoked := lm.revokedLeases[leaseID]; revoked {
			continue
		}
		count++
	}

	return count
}
