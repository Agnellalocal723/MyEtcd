package types

import (
	"time"
)

// CommandType 命令类型
type CommandType int

const (
	CommandPut CommandType = iota
	CommandDelete
	CommandGet
)

// String 返回命令类型的字符串表示
func (c CommandType) String() string {
	switch c {
	case CommandPut:
		return "PUT"
	case CommandDelete:
		return "DELETE"
	case CommandGet:
		return "GET"
	default:
		return "UNKNOWN"
	}
}

// Command 表示一个操作命令
type Command struct {
	Type    CommandType `json:"type"`
	Key     string      `json:"key"`
	Value   []byte      `json:"value,omitempty"`
	Term    uint64      `json:"term"`
	Index   uint64      `json:"index"`
	Time    time.Time   `json:"time"`
}

// KeyValue 键值对
type KeyValue struct {
	Key         string    `json:"key"`
	Value       []byte    `json:"value"`
	Created     time.Time `json:"created"`
	Modified    time.Time `json:"modified"`
	Version     int64     `json:"version"`
	TTL         int64     `json:"ttl"`         // 生存时间(秒)，0表示永不过期
	Expires     time.Time `json:"expires"`     // 过期时间
	LeaseID     int64     `json:"lease_id"`    // 租约ID
}

// IsExpired 检查键值是否过期
func (kv *KeyValue) IsExpired() bool {
	if kv.TTL == 0 {
		return false
	}
	return time.Now().After(kv.Expires)
}

// Transaction 事务
type Transaction struct {
	ID        string      `json:"id"`
	Operations []Operation `json:"operations"`
	Timestamp time.Time   `json:"timestamp"`
	Status    TxnStatus   `json:"status"`
}

// Operation 事务操作
type Operation struct {
	Type  CommandType `json:"type"`
	Key   string      `json:"key"`
	Value []byte      `json:"value,omitempty"`
	Prev  []byte      `json:"prev,omitempty"` // 用于CAS操作
}

// TxnStatus 事务状态
type TxnStatus int

const (
	TxnPending TxnStatus = iota
	TxnCommitted
	TxnAborted
)

// String 返回事务状态的字符串表示
func (s TxnStatus) String() string {
	switch s {
	case TxnPending:
		return "PENDING"
	case TxnCommitted:
		return "COMMITTED"
	case TxnAborted:
		return "ABORTED"
	default:
		return "UNKNOWN"
	}
}

// WALRecord WAL记录
type WALRecord struct {
	Index     uint64    `json:"index"`
	Term      uint64    `json:"term"`
	Command   Command   `json:"command"`
	Checksum  uint32    `json:"checksum"`
	Timestamp time.Time `json:"timestamp"`
}

// Snapshot 快照
type Snapshot struct {
	Index     uint64                 `json:"index"`
	Term      uint64                 `json:"term"`
	Data      map[string]*KeyValue   `json:"data"`
	Checksum  uint32                 `json:"checksum"`
	Timestamp time.Time              `json:"timestamp"`
}

// Config 配置
type Config struct {
	DataDir        string        `json:"data_dir"`
	WALDir         string        `json:"wal_dir"`
	SnapshotDir    string        `json:"snapshot_dir"`
	SnapshotInterval time.Duration `json:"snapshot_interval"`
	MaxWALSize     int64         `json:"max_wal_size"`
	MaxSnapshots   int           `json:"max_snapshots"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
	ElectionTimeout time.Duration  `json:"election_timeout"`
	
	// Raft配置
	NodeID         string            `json:"node_id"`         // 节点唯一标识
	ClusterNodes   []string          `json:"cluster_nodes"`   // 集群中所有节点的ID
	NodeAddresses  map[string]string `json:"node_addresses"`  // 节点ID到地址的映射
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		DataDir:           "./data",
		WALDir:            "./data/wal",
		SnapshotDir:       "./data/snapshot",
		SnapshotInterval:  time.Hour,
		MaxWALSize:        64 * 1024 * 1024, // 64MB
		MaxSnapshots:      3,
		HeartbeatInterval: 100 * time.Millisecond,
		ElectionTimeout:   1000 * time.Millisecond,
		NodeID:            "node1",
		ClusterNodes:      []string{"node1"},
		NodeAddresses:     map[string]string{"node1": "localhost:8080"},
	}
}

// Response API响应
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// PutRequest PUT请求
type PutRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	TTL   int64  `json:"ttl,omitempty"`
}

// GetResponse GET响应
type GetResponse struct {
	Key     string    `json:"key"`
	Value   string    `json:"value"`
	Created time.Time `json:"created"`
	Modified time.Time `json:"modified"`
	Version int64     `json:"version"`
}

// TxnRequest 事务请求
type TxnRequest struct {
	Compare []Compare `json:"compare"`
	Success []Request `json:"success"`
	Failure []Request `json:"failure"`
}

// Compare 比较条件
type Compare struct {
	Key    string      `json:"key"`
	Op     string      `json:"op"`     // "=", "!=", ">", "<"
	Value  interface{} `json:"value"`
}

// Request 请求操作
type Request struct {
	Op    string `json:"op"`    // "PUT", "GET", "DELETE"
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

// WatchEvent Watch事件
type WatchEvent struct {
	Type      WatchEventType `json:"type"`
	Key       string         `json:"key"`
	Value     []byte         `json:"value,omitempty"`
	PrevValue []byte         `json:"prev_value,omitempty"`
	Version   int64          `json:"version"`
	Timestamp time.Time      `json:"timestamp"`
}

// WatchEventType Watch事件类型
type WatchEventType int

const (
	WatchPut WatchEventType = iota
	WatchDelete
)

// String 返回事件类型的字符串表示
func (e WatchEventType) String() string {
	switch e {
	case WatchPut:
		return "PUT"
	case WatchDelete:
		return "DELETE"
	default:
		return "UNKNOWN"
	}
}

// WatchRequest Watch请求
type WatchRequest struct {
	Key      string `json:"key"`
	Prefix   bool   `json:"prefix,omitempty"`
	PrevKV   bool   `json:"prev_kv,omitempty"`
	WatchID  int64  `json:"watch_id"`
}

// WatchResponse Watch响应
type WatchResponse struct {
	WatchID int64        `json:"watch_id"`
	Events  []WatchEvent `json:"events"`
}

// RangeRequest 范围查询请求
type RangeRequest struct {
	Key      string `json:"key"`
	RangeEnd string `json:"range_end,omitempty"`
	Limit    int64  `json:"limit,omitempty"`
	Revision int64  `json:"revision,omitempty"`
}

// RangeResponse 范围查询响应
type RangeResponse struct {
	Kvs    []*KeyValue `json:"kvs"`
	More   bool        `json:"more"`
	Count  int64       `json:"count"`
	Revision int64     `json:"revision"`
}

// BatchRequest 批量操作请求
type BatchRequest struct {
	Operations []Request `json:"operations"`
}

// BatchResponse 批量操作响应
type BatchResponse struct {
	Responses []interface{} `json:"responses"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
}

// Lease 租约
type Lease struct {
	ID        int64     `json:"id"`
	TTL       int64     `json:"ttl"`        // 租约TTL（秒）
	Remaining int64     `json:"remaining"`  // 剩余时间（秒）
	Granted   time.Time `json:"granted"`    // 授予时间
	Keys      []string  `json:"keys"`       // 关联的键
}

// LeaseRequest 租约请求
type LeaseRequest struct {
	TTL int64 `json:"ttl"` // TTL（秒）
}

// LeaseResponse 租约响应
type LeaseResponse struct {
	ID        int64 `json:"id"`
	TTL       int64 `json:"ttl"`
	Remaining int64 `json:"remaining"`
}

// LeaseGrantRequest 授予租约请求
type LeaseGrantRequest struct {
	TTL int64 `json:"ttl"` // TTL（秒）
	ID  int64 `json:"id"`  // 租约ID（可选，0表示自动分配）
}

// LeaseGrantResponse 授予租约响应
type LeaseGrantResponse struct {
	ID    int64 `json:"id"`
	TTL   int64 `json:"ttl"`
	Error string `json:"error,omitempty"`
}

// LeaseRevokeRequest 撤销租约请求
type LeaseRevokeRequest struct {
	ID int64 `json:"id"`
}

// LeaseRevokeResponse 撤销租约响应
type LeaseRevokeResponse struct {
	Header ResponseHeader `json:"header"`
}

// LeaseKeepAliveRequest 保持租约活跃请求
type LeaseKeepAliveRequest struct {
	ID int64 `json:"id"`
}

// LeaseKeepAliveResponse 保持租约活跃响应
type LeaseKeepAliveResponse struct {
	ID        int64 `json:"id"`
	TTL       int64 `json:"ttl"`
	Remaining int64 `json:"remaining"`
}

// ResponseHeader 响应头
type ResponseHeader struct {
	ClusterID uint64 `json:"cluster_id"`
	MemberID  uint64 `json:"member_id"`
	Revision  int64 `json:"revision"`
	RaftTerm  uint64 `json:"raft_term"`
}

// Metrics 指标
type Metrics struct {
	PutCount    int64 `json:"put_count"`
	GetCount    int64 `json:"get_count"`
	DeleteCount int64 `json:"delete_count"`
	TxnCount    int64 `json:"txn_count"`
	WatchCount  int64 `json:"watch_count"`
	KeyCount    int64 `json:"key_count"`
	StorageSize int64 `json:"storage_size"`
}

// Raft相关数据结构

// NodeState 节点状态
type NodeState int

const (
	Follower NodeState = iota // 跟随者
	Candidate                 // 候选者
	Leader                    // 领导者
)

// String 返回节点状态的字符串表示
func (s NodeState) String() string {
	switch s {
	case Follower:
		return "Follower"
	case Candidate:
		return "Candidate"
	case Leader:
		return "Leader"
	default:
		return "Unknown"
	}
}

// LogEntry 日志条目
type LogEntry struct {
	Term    uint64  `json:"term"`    // 任期号
	Index   uint64  `json:"index"`   // 日志索引
	Command Command `json:"command"` // 命令
}

// RaftMessage Raft消息类型
type RaftMessageType int

const (
	MsgAppendEntries RaftMessageType = iota // 追加条目RPC
	MsgAppendEntriesResponse               // 追加条目响应
	MsgRequestVote                        // 请求投票RPC
	MsgRequestVoteResponse                 // 请求投票响应
	MsgInstallSnapshot                    // 安装快照RPC
	MsgInstallSnapshotResponse            // 安装快照响应
)

// String 返回消息类型的字符串表示
func (m RaftMessageType) String() string {
	switch m {
	case MsgAppendEntries:
		return "AppendEntries"
	case MsgAppendEntriesResponse:
		return "AppendEntriesResponse"
	case MsgRequestVote:
		return "RequestVote"
	case MsgRequestVoteResponse:
		return "RequestVoteResponse"
	case MsgInstallSnapshot:
		return "InstallSnapshot"
	case MsgInstallSnapshotResponse:
		return "InstallSnapshotResponse"
	default:
		return "Unknown"
	}
}

// RaftMessage Raft消息
type RaftMessage struct {
	Type      RaftMessageType `json:"type"`       // 消息类型
	Term      uint64          `json:"term"`       // 发送者的任期号
	From      string          `json:"from"`       // 发送者ID
	To        string          `json:"to"`         // 接收者ID

	// AppendEntries字段
	PrevLogIndex uint64      `json:"prev_log_index,omitempty"` // 前一个日志条目的索引
	PrevLogTerm  uint64      `json:"prev_log_term,omitempty"`  // 前一个日志条目的任期号
	Entries      []LogEntry  `json:"entries,omitempty"`        // 日志条目
	LeaderCommit uint64      `json:"leader_commit,omitempty"`  // 领导者的提交索引

	// AppendEntriesResponse字段
	Success bool `json:"success,omitempty"` // 是否成功
	MatchIndex uint64 `json:"match_index,omitempty"` // 匹配的日志索引

	// RequestVote字段
	LastLogIndex uint64 `json:"last_log_index,omitempty"` // 候选人最后日志条目的索引
	LastLogTerm  uint64 `json:"last_log_term,omitempty"`  // 候选人最后日志条目的任期号

	// RequestVoteResponse字段
	VoteGranted bool `json:"vote_granted,omitempty"` // 是否给予投票

	// InstallSnapshot字段
	SnapshotTerm  uint64 `json:"snapshot_term,omitempty"`  // 快照的任期号
	SnapshotIndex uint64 `json:"snapshot_index,omitempty"` // 快照的索引
	SnapshotData  []byte `json:"snapshot_data,omitempty"`  // 快照数据

	// InstallSnapshotResponse字段
	InstallSuccess bool `json:"install_success,omitempty"` // 是否成功
}

// PersistentState 持久化状态
type PersistentState struct {
	CurrentTerm uint64 `json:"current_term"` // 当前任期号
	VotedFor    string `json:"voted_for"`    // 在当前任期投票给哪个候选人
}

// VolatileState 易失状态
type VolatileState struct {
	CommitIndex uint64 `json:"commit_index"` // 已知的最高提交索引
	LastApplied uint64 `json:"last_applied"` // 已应用到状态机的最高索引
}

// LeaderState 领导者状态（只有领导者拥有）
type LeaderState struct {
	NextIndex  map[string]uint64 `json:"next_index"`  // 对于每个服务器，要发送给它的下一个日志条目索引
	MatchIndex map[string]uint64 `json:"match_index"` // 对于每个服务器，已知已复制的最高索引
}
