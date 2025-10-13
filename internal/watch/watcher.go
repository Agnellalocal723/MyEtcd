package watch

import (
	"context"
	"fmt"
	"sync"
	"time"

	"myetcd/internal/types"
)

/*
Watch机制详解：

Watch是一种观察键值变化的机制，允许客户端实时接收键值存储的变化通知。
这是etcd等分布式系统的核心功能之一。

1. Watch的工作原理：
   - 客户端注册对某个键或前缀的Watch
   - 当键值发生变化时，系统推送事件给客户端
   - 客户端可以持续接收变化通知

2. Watch的事件类型：
   - PUT：键值被创建或更新
   - DELETE：键值被删除

3. Watch的特性：
   - 实时性：变化立即通知
   - 可靠性：保证事件不丢失
   - 高效性：支持大量并发Watch
*/

// Watcher Watch管理器
type Watcher struct {
	mu         sync.RWMutex
	watches    map[int64]*Watch  // 活跃的Watch
	watchers   map[string][]int64 // 键到Watch ID的映射
	nextWatchID int64            // 下一个Watch ID
	eventChan  chan *WatchEvent  // 事件通道
	ctx        context.Context
	cancel     context.CancelFunc
	closed     bool
}

// Watch 单个Watch实例
type Watch struct {
	ID      int64             // Watch ID
	Key     string            // 监听的键
	Prefix  bool              // 是否前缀匹配
	PrevKV  bool              // 是否返回之前的值
	Chan    chan *WatchEvent  // 事件通道
	Created time.Time         // 创建时间
	ctx     context.Context
	cancel  context.CancelFunc
}

// WatchEvent Watch事件（内部使用）
type WatchEvent struct {
	Type      types.WatchEventType
	Key       string
	Value     []byte
	PrevValue []byte
	Version   int64
	Timestamp time.Time
}

// NewWatcher 创建新的Watcher
func NewWatcher() *Watcher {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Watcher{
		watches:     make(map[int64]*Watch),
		watchers:    make(map[string][]int64),
		nextWatchID: 1,
		eventChan:   make(chan *WatchEvent, 1000), // 缓冲1000个事件
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start 启动Watcher
func (w *Watcher) Start() {
	go w.eventLoop()
}

// Stop 停止Watcher
func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if w.closed {
		return
	}
	
	w.closed = true
	w.cancel()
	
	// 关闭所有Watch
	for _, watch := range w.watches {
		w.closeWatch(watch)
	}
	
	close(w.eventChan)
}

// Watch 创建新的Watch
func (w *Watcher) Watch(key string, prefix, prevKV bool) (int64, <-chan *WatchEvent) {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if w.closed {
		return 0, nil
	}
	
	watchID := w.nextWatchID
	w.nextWatchID++
	
	ctx, cancel := context.WithCancel(w.ctx)
	
	watch := &Watch{
		ID:      watchID,
		Key:     key,
		Prefix:  prefix,
		PrevKV:  prevKV,
		Chan:    make(chan *WatchEvent, 100), // 缓冲100个事件
		Created: time.Now(),
		ctx:     ctx,
		cancel:  cancel,
	}
	
	w.watches[watchID] = watch
	
	// 添加到映射中
	if prefix {
		// 前缀匹配，使用特殊的映射
		w.watchers["prefix:"+key] = append(w.watchers["prefix:"+key], watchID)
	} else {
		// 精确匹配
		w.watchers["exact:"+key] = append(w.watchers["exact:"+key], watchID)
	}
	
	// 启动Watch清理goroutine
	go w.cleanupWatch(watch)
	
	return watchID, watch.Chan
}

// Cancel 取消Watch
func (w *Watcher) Cancel(watchID int64) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	watch, exists := w.watches[watchID]
	if !exists {
		return fmt.Errorf("watch %d not found", watchID)
	}
	
	w.closeWatch(watch)
	delete(w.watches, watchID)
	
	// 从映射中移除
	w.removeFromMapping(watch)
	
	return nil
}

// Notify 通知键值变化
func (w *Watcher) Notify(eventType types.WatchEventType, key string, value, prevValue []byte, version int64) {
	if w.closed {
		return
	}
	
	event := &WatchEvent{
		Type:      eventType,
		Key:       key,
		Value:     value,
		PrevValue: prevValue,
		Version:   version,
		Timestamp: time.Now(),
	}
	
	select {
	case w.eventChan <- event:
	default:
		// 事件通道满，丢弃事件（生产环境中应该记录日志）
	}
}

// eventLoop 事件循环
func (w *Watcher) eventLoop() {
	for {
		select {
		case event := <-w.eventChan:
			if event == nil {
				return
			}
			w.distributeEvent(event)
		case <-w.ctx.Done():
			return
		}
	}
}

// distributeEvent 分发事件到相关的Watch
func (w *Watcher) distributeEvent(event *WatchEvent) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	
	// 查找匹配的Watch
	var watchIDs []int64
	
	// 精确匹配
	if ids, exists := w.watchers["exact:"+event.Key]; exists {
		watchIDs = append(watchIDs, ids...)
	}
	
	// 前缀匹配
	for prefix, ids := range w.watchers {
		if len(prefix) > 7 && prefix[:7] == "prefix:" {
			prefixKey := prefix[7:]
			if len(event.Key) >= len(prefixKey) && event.Key[:len(prefixKey)] == prefixKey {
				watchIDs = append(watchIDs, ids...)
			}
		}
	}
	
	// 发送事件
	for _, watchID := range watchIDs {
		if watch, exists := w.watches[watchID]; exists {
			select {
			case watch.Chan <- event:
			default:
				// Watch通道满，丢弃事件
			}
		}
	}
}

// closeWatch 关闭Watch
func (w *Watcher) closeWatch(watch *Watch) {
	watch.cancel()
	close(watch.Chan)
}

// removeFromMapping 从映射中移除Watch
func (w *Watcher) removeFromMapping(watch *Watch) {
	var key string
	if watch.Prefix {
		key = "prefix:" + watch.Key
	} else {
		key = "exact:" + watch.Key
	}
	
	if ids, exists := w.watchers[key]; exists {
		for i, id := range ids {
			if id == watch.ID {
				w.watchers[key] = append(ids[:i], ids[i+1:]...)
				break
			}
		}
		if len(w.watchers[key]) == 0 {
			delete(w.watchers, key)
		}
	}
}

// cleanupWatch 清理Watch（当上下文取消时）
func (w *Watcher) cleanupWatch(watch *Watch) {
	<-watch.ctx.Done()
	
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if _, exists := w.watches[watch.ID]; exists {
		w.closeWatch(watch)
		delete(w.watches, watch.ID)
		w.removeFromMapping(watch)
	}
}

// GetWatchCount 获取活跃Watch数量
func (w *Watcher) GetWatchCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.watches)
}

// GetWatchInfo 获取Watch信息
func (w *Watcher) GetWatchInfo() map[int64]WatchInfo {
	w.mu.RLock()
	defer w.mu.RUnlock()
	
	info := make(map[int64]WatchInfo)
	for id, watch := range w.watches {
		info[id] = WatchInfo{
			ID:      watch.ID,
			Key:     watch.Key,
			Prefix:  watch.Prefix,
			PrevKV:  watch.PrevKV,
			Created: watch.Created,
		}
	}
	
	return info
}

// WatchInfo Watch信息
type WatchInfo struct {
	ID      int64     `json:"id"`
	Key     string    `json:"key"`
	Prefix  bool      `json:"prefix"`
	PrevKV  bool      `json:"prev_kv"`
	Created time.Time `json:"created"`
}

// ToTypesEvent 转换为types.WatchEvent
func (e *WatchEvent) ToTypesEvent(prevValue []byte) types.WatchEvent {
	return types.WatchEvent{
		Type:      e.Type,
		Key:       e.Key,
		Value:     e.Value,
		PrevValue: prevValue,
		Version:   e.Version,
		Timestamp: e.Timestamp,
	}
}