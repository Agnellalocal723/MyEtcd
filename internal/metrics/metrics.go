package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

/*
性能监控和指标导出模块

这个模块实现了MyEtcd的性能监控功能，使用Prometheus格式导出指标。
包括以下指标：
1. HTTP请求计数器和延迟
2. 存储操作计数器和延迟
3. Raft操作计数器和延迟
4. 系统资源使用情况
5. 业务指标（如键值对数量、Watch数量等）
*/

var (
	// 全局指标实例
	instance *Metrics
	once     sync.Once
)

// Metrics 指标结构体
type Metrics struct {
	// HTTP指标
	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec

	// 存储指标
	storageOperationsTotal   *prometheus.CounterVec
	storageOperationDuration *prometheus.HistogramVec
	storageKeysTotal         prometheus.Gauge
	storageSizeBytes         prometheus.Gauge

	// Raft指标
	raftMessagesTotal   *prometheus.CounterVec
	raftMessageDuration *prometheus.HistogramVec
	raftLogEntriesTotal prometheus.Gauge
	raftLogIndex        prometheus.Gauge
	raftTerm            prometheus.Gauge
	raftState           prometheus.Gauge

	// Watch指标
	watchersTotal    prometheus.Gauge
	watchEventsTotal *prometheus.CounterVec

	// 租约指标
	leasesTotal           prometheus.Gauge
	leaseExpirationsTotal *prometheus.CounterVec

	// 系统指标
	goRoutines      prometheus.Gauge
	gcPauseDuration *prometheus.HistogramVec
}

// GetMetrics 获取全局指标实例
func GetMetrics() *Metrics {
	once.Do(func() {
		instance = NewMetrics()
	})
	return instance
}

// NewMetrics 创建新的指标实例
func NewMetrics() *Metrics {
	return &Metrics{
		// HTTP指标
		httpRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "myetcd_http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "endpoint", "status"},
		),
		httpRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "myetcd_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "endpoint"},
		),

		// 存储指标
		storageOperationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "myetcd_storage_operations_total",
				Help: "Total number of storage operations",
			},
			[]string{"operation", "status"},
		),
		storageOperationDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "myetcd_storage_operation_duration_seconds",
				Help:    "Storage operation duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation"},
		),
		storageKeysTotal: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "myetcd_storage_keys_total",
				Help: "Total number of keys in storage",
			},
		),
		storageSizeBytes: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "myetcd_storage_size_bytes",
				Help: "Total size of storage in bytes",
			},
		),

		// Raft指标
		raftMessagesTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "myetcd_raft_messages_total",
				Help: "Total number of Raft messages",
			},
			[]string{"type", "from", "to", "status"},
		),
		raftMessageDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "myetcd_raft_message_duration_seconds",
				Help:    "Raft message processing duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"type"},
		),
		raftLogEntriesTotal: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "myetcd_raft_log_entries_total",
				Help: "Total number of Raft log entries",
			},
		),
		raftLogIndex: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "myetcd_raft_log_index",
				Help: "Current Raft log index",
			},
		),
		raftTerm: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "myetcd_raft_term",
				Help: "Current Raft term",
			},
		),
		raftState: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "myetcd_raft_state",
				Help: "Current Raft state (0=Follower, 1=Candidate, 2=Leader)",
			},
		),

		// Watch指标
		watchersTotal: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "myetcd_watchers_total",
				Help: "Total number of active watchers",
			},
		),
		watchEventsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "myetcd_watch_events_total",
				Help: "Total number of watch events",
			},
			[]string{"type"},
		),

		// 租约指标
		leasesTotal: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "myetcd_leases_total",
				Help: "Total number of active leases",
			},
		),
		leaseExpirationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "myetcd_lease_expirations_total",
				Help: "Total number of lease expirations",
			},
			[]string{"reason"},
		),

		// 系统指标
		goRoutines: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "myetcd_go_routines",
				Help: "Number of goroutines",
			},
		),
		gcPauseDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "myetcd_gc_pause_duration_seconds",
				Help:    "GC pause duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{},
		),
	}
}

// HTTP指标记录方法

// RecordHTTPRequest 记录HTTP请求
func (m *Metrics) RecordHTTPRequest(method, endpoint, status string) {
	m.httpRequestsTotal.WithLabelValues(method, endpoint, status).Inc()
}

// ObserveHTTPRequestDuration 观察HTTP请求持续时间
func (m *Metrics) ObserveHTTPRequestDuration(method, endpoint string, duration time.Duration) {
	m.httpRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

// 存储指标记录方法

// RecordStorageOperation 记录存储操作
func (m *Metrics) RecordStorageOperation(operation, status string) {
	m.storageOperationsTotal.WithLabelValues(operation, status).Inc()
}

// ObserveStorageOperationDuration 观察存储操作持续时间
func (m *Metrics) ObserveStorageOperationDuration(operation string, duration time.Duration) {
	m.storageOperationDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// SetStorageKeys 设置存储键数量
func (m *Metrics) SetStorageKeys(count int) {
	m.storageKeysTotal.Set(float64(count))
}

// SetStorageSize 设置存储大小
func (m *Metrics) SetStorageSize(size int64) {
	m.storageSizeBytes.Set(float64(size))
}

// Raft指标记录方法

// RecordRaftMessage 记录Raft消息
func (m *Metrics) RecordRaftMessage(msgType, from, to, status string) {
	m.raftMessagesTotal.WithLabelValues(msgType, from, to, status).Inc()
}

// ObserveRaftMessageDuration 观察Raft消息处理持续时间
func (m *Metrics) ObserveRaftMessageDuration(msgType string, duration time.Duration) {
	m.raftMessageDuration.WithLabelValues(msgType).Observe(duration.Seconds())
}

// SetRaftLogEntries 设置Raft日志条目数量
func (m *Metrics) SetRaftLogEntries(count int) {
	m.raftLogEntriesTotal.Set(float64(count))
}

// SetRaftLogIndex 设置Raft日志索引
func (m *Metrics) SetRaftLogIndex(index uint64) {
	m.raftLogIndex.Set(float64(index))
}

// SetRaftTerm 设置Raft任期
func (m *Metrics) SetRaftTerm(term uint64) {
	m.raftTerm.Set(float64(term))
}

// SetRaftState 设置Raft状态
func (m *Metrics) SetRaftState(state int) {
	m.raftState.Set(float64(state))
}

// Watch指标记录方法

// SetWatchers 设置Watcher数量
func (m *Metrics) SetWatchers(count int) {
	m.watchersTotal.Set(float64(count))
}

// RecordWatchEvent 记录Watch事件
func (m *Metrics) RecordWatchEvent(eventType string) {
	m.watchEventsTotal.WithLabelValues(eventType).Inc()
}

// 租约指标记录方法

// SetLeases 设置租约数量
func (m *Metrics) SetLeases(count int) {
	m.leasesTotal.Set(float64(count))
}

// RecordLeaseExpiration 记录租约过期
func (m *Metrics) RecordLeaseExpiration(reason string) {
	m.leaseExpirationsTotal.WithLabelValues(reason).Inc()
}

// 系统指标记录方法

// SetGoRoutines 设置goroutine数量
func (m *Metrics) SetGoRoutines(count int) {
	m.goRoutines.Set(float64(count))
}

// ObserveGCPause 观察GC暂停时间
func (m *Metrics) ObserveGCPause(duration time.Duration) {
	m.gcPauseDuration.WithLabelValues().Observe(duration.Seconds())
}
