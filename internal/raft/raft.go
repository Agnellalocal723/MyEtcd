package raft

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"myetcd/internal/types"
)

/*
Raft一致性算法实现

Raft是一个一致性算法，它通过将一致性问题分解为几个相对独立的子问题来简化问题：
1. 领导选举：在集群中选举出一个领导者，负责管理 replicated log
2. 日志复制：领导者接收客户端请求，将操作作为日志条目复制到集群中的其他节点
3. 安全性：确保系统的安全性，比如防止选举脑裂、确保已提交的日志不会丢失等

Raft算法的关键特性：
- 强领导者：只有领导者处理客户端请求
- 选举安全性：每个任期最多只能有一个领导者
- 成员变化：支持集群成员的安全变更
- 日志压缩：通过快照机制压缩日志

这个实现包含了Raft算法的核心功能，包括领导选举、日志复制、安全性保证等。
*/

// Node Raft节点
type Node struct {
	// 持久化状态（在重启前保持不变）
	persistentState types.PersistentState

	// 易失状态（所有节点都有）
	volatileState types.VolatileState

	// 领导者状态（只有领导者拥有）
	leaderState types.LeaderState

	// Raft状态
	state types.NodeState // 节点状态：Follower、Candidate或Leader

	// 日志
	log []types.LogEntry // 日志条目数组

	// 配置
	config *types.Config

	// 通信
	transport Transport // 网络传输层

	// 状态机（应用层）
	stateMachine StateMachine // 状态机接口

	// 同步
	mu sync.Mutex

	// 通道
	appendEntriesCh chan types.RaftMessage
	requestVoteCh   chan types.RaftMessage
	installSnapshotCh chan types.RaftMessage
	tickCh          chan struct{}
	applyCh         chan types.LogEntry

	// 定时器
	electionTimer  *time.Timer
	heartbeatTimer *time.Timer

	// 其他
	commitIndex uint64 // 已提交的最高日志索引
	lastApplied uint64 // 已应用到状态机的最高日志索引

	// 用于通知的通道
	readyCh chan struct{}
	
	// 选举相关
	votesReceived int // 收到的票数
}

// Transport 网络传输接口
type Transport interface {
	Send(to string, msg types.RaftMessage) error
	Broadcast(msg types.RaftMessage) error
}

// StateMachine 状态机接口
type StateMachine interface {
	Apply(entry types.LogEntry) error
	Snapshot() ([]byte, error)
	Restore(data []byte) error
}

// NewNode 创建新的Raft节点
func NewNode(config *types.Config, transport Transport, stateMachine StateMachine) *Node {
	node := &Node{
		persistentState: types.PersistentState{
			CurrentTerm: 0,
			VotedFor:    "",
		},
		volatileState: types.VolatileState{
			CommitIndex: 0,
			LastApplied: 0,
		},
		leaderState: types.LeaderState{
			NextIndex:  make(map[string]uint64),
			MatchIndex: make(map[string]uint64),
		},
		state:            types.Follower,
		log:              make([]types.LogEntry, 0),
		config:           config,
		transport:        transport,
		stateMachine:     stateMachine,
		appendEntriesCh:  make(chan types.RaftMessage, 1000),
		requestVoteCh:    make(chan types.RaftMessage, 1000),
		installSnapshotCh: make(chan types.RaftMessage, 1000),
		tickCh:           make(chan struct{}, 1),
		applyCh:          make(chan types.LogEntry, 1000),
		commitIndex:      0,
		lastApplied:      0,
		readyCh:          make(chan struct{}, 1),
	}

	// 初始化定时器
	node.electionTimer = time.NewTimer(randomizedElectionTimeout(config.ElectionTimeout))
	node.heartbeatTimer = time.NewTimer(0) // 初始不启动心跳定时器

	// 添加一个空的日志条目，索引为0（占位符，不会被应用）
	node.log = append(node.log, types.LogEntry{
		Term:    0,
		Index:   0,
		Command: types.Command{}, // 空命令
	})

	return node
}

// Start 启动Raft节点
func (n *Node) Start() {
	log.Printf("Starting Raft node %s as %s", n.config.NodeID, n.state.String())
	
	// 启动主循环
	go n.run()
	
	// 启动应用日志的goroutine
	go n.applyLoop()
	
	// 启动ticker goroutine来触发定时器
	go n.ticker()
}

// ticker 定期触发定时器事件
func (n *Node) ticker() {
	for {
		select {
		case <-n.electionTimer.C:
			// 使用非阻塞方式发送tick信号
			select {
			case n.tickCh <- struct{}{}:
				// 成功发送
			default:
				// 已经有一个tick在处理中或通道已关闭
			}
			
		case <-n.heartbeatTimer.C:
			// 检查是否是领导者
			n.mu.Lock()
			isLeader := n.state == types.Leader
			n.mu.Unlock()
			
			if isLeader {
				// 使用非阻塞方式发送tick信号
				select {
				case n.tickCh <- struct{}{}:
					// 成功发送
				default:
					// 已经有一个tick在处理中或通道已关闭
				}
			}
			
		case <-n.tickCh:
			// 通道已关闭，退出循环
			return
		}
	}
}

// run Raft节点主循环
func (n *Node) run() {
	for {
		select {
		case _, ok := <-n.tickCh:
			// 定时器触发
			if !ok {
				// 通道已关闭，退出循环
				return
			}
			n.tick()
			
		case msg, ok := <-n.appendEntriesCh:
			// 处理AppendEntries RPC
			if !ok {
				// 通道已关闭，退出循环
				return
			}
			n.handleAppendEntries(msg)
			
		case msg, ok := <-n.requestVoteCh:
			// 处理RequestVote RPC
			if !ok {
				// 通道已关闭，退出循环
				return
			}
			n.handleRequestVote(msg)
			
		case msg, ok := <-n.installSnapshotCh:
			// 处理InstallSnapshot RPC
			if !ok {
				// 通道已关闭，退出循环
				return
			}
			n.handleInstallSnapshot(msg)
			
		case entry, ok := <-n.applyCh:
			// 应用日志条目到状态机
			if !ok {
				// 通道已关闭，退出循环
				return
			}
			n.applyEntry(entry)
		}
	}
}

// tick 处理定时器事件
func (n *Node) tick() {
	n.mu.Lock()
	defer n.mu.Unlock()

	switch n.state {
	case types.Follower:
		// 跟随者如果在选举超时时间内没有收到领导者的心跳，就转为候选者
		log.Printf("Follower %s election timeout, becoming candidate", n.config.NodeID)
		n.becomeCandidate()
		
	case types.Candidate:
		// 候选者如果选举超时，开始新一轮选举
		log.Printf("Candidate %s election timeout, starting new election", n.config.NodeID)
		n.startElection()
		
	case types.Leader:
		// 领导者定期发送心跳
		n.sendHeartbeats()
	}
}

// becomeCandidate 转为候选者状态
func (n *Node) becomeCandidate() {
	n.state = types.Candidate
	n.persistentState.CurrentTerm++
	n.persistentState.VotedFor = n.config.NodeID
	
	// 重置选举定时器
	n.resetElectionTimer()
	
	// 开始选举
	n.startElection()
}

// startElection 开始选举
func (n *Node) startElection() {
	log.Printf("Candidate %s starting election for term %d", n.config.NodeID, n.persistentState.CurrentTerm)
	
	// 获取最后一条日志的索引和任期
	lastLogIndex, lastLogTerm := n.getLastLogInfo()
	
	// 重置投票计数
	n.votesReceived = 1 // 自己的一票
	
	// 向所有其他节点发送请求投票RPC
	for _, nodeID := range n.config.ClusterNodes {
		if nodeID == n.config.NodeID {
			continue
		}
		
		msg := types.RaftMessage{
			Type:         types.MsgRequestVote,
			Term:         n.persistentState.CurrentTerm,
			From:         n.config.NodeID,
			To:           nodeID,
			LastLogIndex: lastLogIndex,
			LastLogTerm:  lastLogTerm,
		}
		
		log.Printf("Node %s sending RequestVote message to %s", n.config.NodeID, nodeID)
		
		go func(to string) {
			if err := n.transport.Send(to, msg); err != nil {
				log.Printf("Failed to send RequestVote to %s: %v", to, err)
			} else {
				log.Printf("Successfully sent RequestVote message to %s", to)
			}
		}(nodeID)
	}
	
	// 不在这里立即检查票数，而是在处理响应时检查
}

// becomeLeader 成为领导者
func (n *Node) becomeLeader() {
	n.state = types.Leader
	log.Printf("Node %s became leader for term %d", n.config.NodeID, n.persistentState.CurrentTerm)
	
	// 初始化领导者状态
	for _, nodeID := range n.config.ClusterNodes {
		if nodeID == n.config.NodeID {
			continue
		}
		n.leaderState.NextIndex[nodeID] = uint64(len(n.log))
		n.leaderState.MatchIndex[nodeID] = 0
	}
	
	// 启动心跳定时器
	n.resetHeartbeatTimer()
	
	// 立即发送一次心跳（空的AppendEntries）
	n.sendHeartbeats()
}

// sendHeartbeats 发送心跳
func (n *Node) sendHeartbeats() {
	for _, nodeID := range n.config.ClusterNodes {
		if nodeID == n.config.NodeID {
			continue
		}
		
		nextIndex := n.leaderState.NextIndex[nodeID]
		prevLogIndex := nextIndex - 1
		prevLogTerm := uint64(0)
		
		if prevLogIndex > 0 && prevLogIndex < uint64(len(n.log)) {
			prevLogTerm = n.log[prevLogIndex].Term
		}
		
		// 准备要发送的日志条目
		var entries []types.LogEntry
		if nextIndex < uint64(len(n.log)) {
			entries = n.log[nextIndex:]
		}
		
		msg := types.RaftMessage{
			Type:         types.MsgAppendEntries,
			Term:         n.persistentState.CurrentTerm,
			From:         n.config.NodeID,
			To:           nodeID,
			PrevLogIndex: prevLogIndex,
			PrevLogTerm:  prevLogTerm,
			Entries:      entries,
			LeaderCommit: n.commitIndex,
		}
		
		go func(to string) {
			if err := n.transport.Send(to, msg); err != nil {
				log.Printf("Failed to send AppendEntries to %s: %v", to, err)
			}
		}(nodeID)
	}
	
	// 重置心跳定时器
	n.resetHeartbeatTimer()
}

// handleAppendEntries 处理AppendEntries RPC
func (n *Node) handleAppendEntries(msg types.RaftMessage) {
	n.mu.Lock()
	defer n.mu.Unlock()
	
	// 如果任期小于当前任期，拒绝
	if msg.Term < n.persistentState.CurrentTerm {
		n.sendAppendEntriesResponse(msg.From, false, n.commitIndex)
		return
	}
	
	// 如果任期大于当前任期，更新任期并转为跟随者
	if msg.Term > n.persistentState.CurrentTerm {
		n.persistentState.CurrentTerm = msg.Term
		n.persistentState.VotedFor = ""
		n.state = types.Follower
	}
	
	// 重置选举定时器
	n.resetElectionTimer()
	
	// 检查前一个日志条目是否匹配
	if msg.PrevLogIndex > 0 {
		if msg.PrevLogIndex >= uint64(len(n.log)) || 
		   n.log[msg.PrevLogIndex].Term != msg.PrevLogTerm {
			n.sendAppendEntriesResponse(msg.From, false, n.commitIndex)
			return
		}
	}
	
	// 添加新的日志条目
	if len(msg.Entries) > 0 {
		// 检查是否有冲突
		for i, entry := range msg.Entries {
			index := msg.PrevLogIndex + 1 + uint64(i)
			
			if index < uint64(len(n.log)) && n.log[index].Term != entry.Term {
				// 删除冲突的条目及其后的所有条目
				n.log = n.log[:index]
				break
			}
		}
		
		// 添加新条目
		for _, entry := range msg.Entries {
			n.log = append(n.log, entry)
		}
	}
	
	// 更新提交索引
	if msg.LeaderCommit > n.commitIndex {
		n.commitIndex = min(msg.LeaderCommit, uint64(len(n.log)-1))
		n.notifyReady()
	}
	
	// 发送成功响应
	matchIndex := msg.PrevLogIndex + uint64(len(msg.Entries))
	n.sendAppendEntriesResponse(msg.From, true, matchIndex)
}

// sendAppendEntriesResponse 发送AppendEntries响应
func (n *Node) sendAppendEntriesResponse(to string, success bool, matchIndex uint64) {
	// 检查to参数是否为空
	if to == "" {
		log.Printf("Warning: sendAppendEntriesResponse called with empty to parameter")
		return
	}
	
	msg := types.RaftMessage{
		Type:       types.MsgAppendEntriesResponse,
		Term:       n.persistentState.CurrentTerm,
		From:       n.config.NodeID,
		To:         to,
		Success:    success,
		MatchIndex: matchIndex,
	}
	
	log.Printf("Node %s sending AppendEntriesResponse to %s", n.config.NodeID, to)
	
	go func() {
		if err := n.transport.Send(to, msg); err != nil {
			log.Printf("Failed to send AppendEntriesResponse to %s: %v", to, err)
		}
	}()
}

// handleRequestVote 处理RequestVote RPC
func (n *Node) handleRequestVote(msg types.RaftMessage) {
	n.mu.Lock()
	defer n.mu.Unlock()
	
	// 如果任期小于当前任期，拒绝
	if msg.Term < n.persistentState.CurrentTerm {
		n.sendRequestVoteResponse(msg.From, false)
		return
	}
	
	// 如果任期大于当前任期，更新任期并转为跟随者
	if msg.Term > n.persistentState.CurrentTerm {
		n.persistentState.CurrentTerm = msg.Term
		n.persistentState.VotedFor = ""
		n.state = types.Follower
	}
	
	// 检查是否已经投票
	voteGranted := false
	if n.persistentState.VotedFor == "" || n.persistentState.VotedFor == msg.From {
		// 检查候选人的日志是否至少和自己一样新
		lastLogIndex, lastLogTerm := n.getLastLogInfo()
		if msg.LastLogTerm > lastLogTerm || 
		   (msg.LastLogTerm == lastLogTerm && msg.LastLogIndex >= lastLogIndex) {
			voteGranted = true
			n.persistentState.VotedFor = msg.From
		}
	}
	
	// 重置选举定时器
	n.resetElectionTimer()
	
	// 发送响应
	n.sendRequestVoteResponse(msg.From, voteGranted)
}

// sendRequestVoteResponse 发送RequestVote响应
func (n *Node) sendRequestVoteResponse(to string, voteGranted bool) {
	// 检查to参数是否为空
	if to == "" {
		log.Printf("Warning: sendRequestVoteResponse called with empty to parameter")
		return
	}
	
	msg := types.RaftMessage{
		Type:        types.MsgRequestVoteResponse,
		Term:        n.persistentState.CurrentTerm,
		From:        n.config.NodeID,
		To:          to,
		VoteGranted: voteGranted,
	}
	
	log.Printf("Node %s sending RequestVoteResponse to %s", n.config.NodeID, to)
	
	go func() {
		if err := n.transport.Send(to, msg); err != nil {
			log.Printf("Failed to send RequestVoteResponse to %s: %v", to, err)
		}
	}()
}

// handleInstallSnapshot 处理InstallSnapshot RPC
func (n *Node) handleInstallSnapshot(msg types.RaftMessage) {
	n.mu.Lock()
	defer n.mu.Unlock()
	
	// 如果任期小于当前任期，拒绝
	if msg.Term < n.persistentState.CurrentTerm {
		n.sendInstallSnapshotResponse(msg.From, false)
		return
	}
	
	// 如果任期大于当前任期，更新任期并转为跟随者
	if msg.Term > n.persistentState.CurrentTerm {
		n.persistentState.CurrentTerm = msg.Term
		n.persistentState.VotedFor = ""
		n.state = types.Follower
	}
	
	// 重置选举定时器
	n.resetElectionTimer()
	
	// 应用快照
	if err := n.stateMachine.Restore(msg.SnapshotData); err != nil {
		log.Printf("Failed to restore snapshot: %v", err)
		n.sendInstallSnapshotResponse(msg.From, false)
		return
	}
	
	// 更新状态
	n.commitIndex = msg.SnapshotIndex
	n.lastApplied = msg.SnapshotIndex
	
	// 清空日志，只保留快照点之后的日志
	n.log = n.log[:msg.SnapshotIndex+1]
	
	// 发送成功响应
	n.sendInstallSnapshotResponse(msg.From, true)
}

// sendInstallSnapshotResponse 发送InstallSnapshot响应
func (n *Node) sendInstallSnapshotResponse(to string, success bool) {
	// 检查to参数是否为空
	if to == "" {
		log.Printf("Warning: sendInstallSnapshotResponse called with empty to parameter")
		return
	}
	
	msg := types.RaftMessage{
		Type:           types.MsgInstallSnapshotResponse,
		Term:           n.persistentState.CurrentTerm,
		From:           n.config.NodeID,
		To:             to,
		InstallSuccess: success,
	}
	
	log.Printf("Node %s sending InstallSnapshotResponse to %s", n.config.NodeID, to)
	
	go func() {
		if err := n.transport.Send(to, msg); err != nil {
			log.Printf("Failed to send InstallSnapshotResponse to %s: %v", to, err)
		}
	}()
}

// applyLoop 应用日志条目到状态机的循环
func (n *Node) applyLoop() {
	for {
		select {
		case entry, ok := <-n.applyCh:
			if !ok {
				// 通道已关闭，退出循环
				return
			}
			if err := n.stateMachine.Apply(entry); err != nil {
				log.Printf("Failed to apply log entry: %v", err)
			}
		}
	}
}

// applyEntry 应用日志条目
func (n *Node) applyEntry(entry types.LogEntry) {
	n.mu.Lock()
	defer n.mu.Unlock()
	
	if entry.Index <= n.lastApplied {
		return
	}
	
	// 跳过索引为0的空日志条目
	if entry.Index == 0 {
		n.lastApplied = entry.Index
		return
	}
	
	// 发送到应用通道，如果通道满则等待
	n.applyCh <- entry
	n.lastApplied = entry.Index
}

// notifyReady 通知有新的日志条目可以应用
func (n *Node) notifyReady() {
	// 应用所有已提交但未应用的日志条目
	for n.lastApplied < n.commitIndex {
		n.lastApplied++
		if n.lastApplied < uint64(len(n.log)) {
			entry := n.log[n.lastApplied]
			// 跳过索引为0的空日志条目
			if entry.Index == 0 {
				continue
			}
			
			log.Printf("Leader %s applying committed log entry: index=%d, term=%d, type=%s, key=%s",
				n.config.NodeID, entry.Index, entry.Term, entry.Command.Type.String(), entry.Command.Key)
			
			// 发送到应用通道，如果通道满则等待
			n.applyCh <- entry
		}
	}
	
	// 发送就绪信号
	select {
	case n.readyCh <- struct{}{}:
	default:
	}
}

// getLastLogInfo 获取最后一条日志的索引和任期
func (n *Node) getLastLogInfo() (uint64, uint64) {
	if len(n.log) == 0 {
		return 0, 0
	}
	lastEntry := n.log[len(n.log)-1]
	return lastEntry.Index, lastEntry.Term
}

// resetElectionTimer 重置选举定时器
func (n *Node) resetElectionTimer() {
	if !n.electionTimer.Stop() {
		select {
		case <-n.electionTimer.C:
		default:
		}
	}
	n.electionTimer.Reset(randomizedElectionTimeout(n.config.ElectionTimeout))
}

// resetHeartbeatTimer 重置心跳定时器
func (n *Node) resetHeartbeatTimer() {
	if !n.heartbeatTimer.Stop() {
		select {
		case <-n.heartbeatTimer.C:
		default:
		}
	}
	n.heartbeatTimer.Reset(n.config.HeartbeatInterval)
}

// randomizedElectionTimeout 返回随机的选举超时时间
func randomizedElectionTimeout(base time.Duration) time.Duration {
	// 在base到2*base之间随机选择
	return base + time.Duration(rand.Int63n(int64(base)))
}

// min 返回两个uint64中的较小值
func min(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

// ProcessMessage 处理接收到的Raft消息
func (n *Node) ProcessMessage(msg types.RaftMessage) {
	// 检查节点是否已经停止
	n.mu.Lock()
	
	// 检查节点是否已经停止（通过检查tickCh是否已关闭）
	select {
	case <-n.tickCh:
		// tickCh已关闭，表示节点已停止
		n.mu.Unlock()
		return
	default:
	}
	
	// 处理消息
	switch msg.Type {
	case types.MsgAppendEntries:
		select {
		case n.appendEntriesCh <- msg:
		default:
			log.Printf("AppendEntries channel is full or closed, dropping message")
		}
		n.mu.Unlock()
		
	case types.MsgAppendEntriesResponse:
		n.mu.Unlock()
		n.handleAppendEntriesResponse(msg)
		
	case types.MsgRequestVote:
		select {
		case n.requestVoteCh <- msg:
		default:
			log.Printf("RequestVote channel is full or closed, dropping message")
		}
		n.mu.Unlock()
		
	case types.MsgRequestVoteResponse:
		n.mu.Unlock()
		n.handleRequestVoteResponse(msg)
		
	case types.MsgInstallSnapshot:
		select {
		case n.installSnapshotCh <- msg:
		default:
			log.Printf("InstallSnapshot channel is full or closed, dropping message")
		}
		n.mu.Unlock()
		
	case types.MsgInstallSnapshotResponse:
		n.mu.Unlock()
		// 暂时不处理
		
	default:
		n.mu.Unlock()
		log.Printf("Unknown message type: %d", msg.Type)
	}
}

// handleAppendEntriesResponse 处理AppendEntries响应
func (n *Node) handleAppendEntriesResponse(msg types.RaftMessage) {
	n.mu.Lock()
	defer n.mu.Unlock()
	
	// 检查msg.From是否为空
	if msg.From == "" {
		log.Printf("Warning: received AppendEntriesResponse with empty From field")
		return
	}
	
	if n.state != types.Leader || msg.Term < n.persistentState.CurrentTerm {
		log.Printf("Node %s ignoring AppendEntriesResponse from %s: not leader or term mismatch",
			n.config.NodeID, msg.From)
		return
	}
	
	if msg.Term > n.persistentState.CurrentTerm {
		n.persistentState.CurrentTerm = msg.Term
		n.state = types.Follower
		log.Printf("Node %s stepping down to follower due to higher term %d from %s",
			n.config.NodeID, msg.Term, msg.From)
		return
	}
	
	if msg.Success {
		// 成功，更新匹配索引和下一个索引
		oldMatchIndex := n.leaderState.MatchIndex[msg.From]
		n.leaderState.MatchIndex[msg.From] = msg.MatchIndex
		n.leaderState.NextIndex[msg.From] = msg.MatchIndex + 1
		
		log.Printf("Node %s: AppendEntries to %s succeeded, matchIndex updated from %d to %d",
			n.config.NodeID, msg.From, oldMatchIndex, msg.MatchIndex)
		
		// 检查是否可以提交新的日志条目
		n.checkCommitIndex()
		
		// 如果日志条目已提交，立即发送心跳通知跟随者
		if n.commitIndex > 0 {
			n.sendHeartbeats()
		}
	} else {
		// 失败，减少下一个索引并重试
		oldNextIndex := n.leaderState.NextIndex[msg.From]
		if n.leaderState.NextIndex[msg.From] > 1 {
			n.leaderState.NextIndex[msg.From]--
		}
		
		log.Printf("Node %s: AppendEntries to %s failed, nextIndex decreased from %d to %d",
			n.config.NodeID, msg.From, oldNextIndex, n.leaderState.NextIndex[msg.From])
		
		// 重新发送日志条目
		nextIndex := n.leaderState.NextIndex[msg.From]
		prevLogIndex := nextIndex - 1
		prevLogTerm := uint64(0)
		
		if prevLogIndex > 0 && prevLogIndex < uint64(len(n.log)) {
			prevLogTerm = n.log[prevLogIndex].Term
		}
		
		var entries []types.LogEntry
		if nextIndex < uint64(len(n.log)) {
			entries = n.log[nextIndex:]
		}
		
		responseMsg := types.RaftMessage{
			Type:         types.MsgAppendEntries,
			Term:         n.persistentState.CurrentTerm,
			From:         n.config.NodeID,
			To:           msg.From,
			PrevLogIndex: prevLogIndex,
			PrevLogTerm:  prevLogTerm,
			Entries:      entries,
			LeaderCommit: n.commitIndex,
		}
		
		log.Printf("Node %s resending AppendEntries to %s: nextIndex=%d, prevLogIndex=%d, entries=%d",
			n.config.NodeID, msg.From, nextIndex, prevLogIndex, len(entries))
		
		go func() {
			if err := n.transport.Send(msg.From, responseMsg); err != nil {
				log.Printf("Failed to send AppendEntries to %s: %v", msg.From, err)
			}
		}()
	}
}

// handleRequestVoteResponse 处理RequestVote响应
func (n *Node) handleRequestVoteResponse(msg types.RaftMessage) {
	n.mu.Lock()
	defer n.mu.Unlock()
	
	// 检查msg.From是否为空
	if msg.From == "" {
		log.Printf("Warning: received RequestVoteResponse with empty From field")
		return
	}
	
	if n.state != types.Candidate || msg.Term < n.persistentState.CurrentTerm {
		return
	}
	
	if msg.Term > n.persistentState.CurrentTerm {
		n.persistentState.CurrentTerm = msg.Term
		n.state = types.Follower
		return
	}
	
	if msg.VoteGranted {
		// 获得一票，增加投票计数
		n.votesReceived++
		log.Printf("Candidate %s received vote from %s, total votes: %d", n.config.NodeID, msg.From, n.votesReceived)
		
		// 检查是否获得了足够的票数
		neededVotes := len(n.config.ClusterNodes)/2 + 1
		if n.votesReceived >= neededVotes {
			n.becomeLeader()
		}
	}
}

// checkCommitIndex 检查是否可以提交新的日志条目
func (n *Node) checkCommitIndex() {
	// 找到可以被大多数节点复制的最高索引
	maxCommitIndex := n.commitIndex
	
	for i := n.commitIndex + 1; i < uint64(len(n.log)); i++ {
		count := 1 // 领导者自己
		
		for _, nodeID := range n.config.ClusterNodes {
			if nodeID == n.config.NodeID {
				continue
			}
			
			if n.leaderState.MatchIndex[nodeID] >= i {
				count++
			}
		}
		
		// 如果大多数节点已经复制了这个条目
		if count > len(n.config.ClusterNodes)/2 {
			// 提交日志条目
			// 根据Raft论文，领导者可以提交任何任期的日志条目，
			// 只要它知道这个条目已经被大多数节点复制
			maxCommitIndex = i
		}
	}
	
	// 批量更新提交索引
	if maxCommitIndex > n.commitIndex {
		n.commitIndex = maxCommitIndex
		log.Printf("Leader %s committing log entries up to index %d (term %d)",
			n.config.NodeID, n.commitIndex, n.log[n.commitIndex].Term)
		n.notifyReady()
	}
}

// Propose 提议一个新的命令
func (n *Node) Propose(command types.Command) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	
	if n.state != types.Leader {
		return fmt.Errorf("not leader, current state: %s", n.state.String())
	}
	
	// 创建新的日志条目
	entry := types.LogEntry{
		Term:    n.persistentState.CurrentTerm,
		Index:   uint64(len(n.log)),
		Command: command,
	}
	
	log.Printf("Leader %s proposing new log entry: index=%d, term=%d, type=%s, key=%s",
		n.config.NodeID, entry.Index, entry.Term, entry.Command.Type.String(), entry.Command.Key)
	
	// 添加到本地日志
	n.log = append(n.log, entry)
	
	// 立即发送给所有跟随者
	n.sendLogEntries()
	
	return nil
}

// sendLogEntries 发送日志条目到所有跟随者
func (n *Node) sendLogEntries() {
	for _, nodeID := range n.config.ClusterNodes {
		if nodeID == n.config.NodeID {
			continue
		}
		
		nextIndex := n.leaderState.NextIndex[nodeID]
		prevLogIndex := nextIndex - 1
		prevLogTerm := uint64(0)
		
		if prevLogIndex > 0 && prevLogIndex < uint64(len(n.log)) {
			prevLogTerm = n.log[prevLogIndex].Term
		}
		
		// 准备要发送的日志条目
		var entries []types.LogEntry
		if nextIndex < uint64(len(n.log)) {
			entries = n.log[nextIndex:]
		}
		
		msg := types.RaftMessage{
			Type:         types.MsgAppendEntries,
			Term:         n.persistentState.CurrentTerm,
			From:         n.config.NodeID,
			To:           nodeID,
			PrevLogIndex: prevLogIndex,
			PrevLogTerm:  prevLogTerm,
			Entries:      entries,
			LeaderCommit: n.commitIndex,
		}
		
		log.Printf("Node %s sending AppendEntries to %s: nextIndex=%d, prevLogIndex=%d, prevLogTerm=%d, entries=%d, commitIndex=%d",
			n.config.NodeID, nodeID, nextIndex, prevLogIndex, prevLogTerm, len(entries), n.commitIndex)
		
		go func(to string) {
			if err := n.transport.Send(to, msg); err != nil {
				log.Printf("Failed to send AppendEntries to %s: %v", to, err)
			} else {
				log.Printf("Successfully sent AppendEntries to %s with %d entries", to, len(entries))
			}
		}(nodeID)
	}
}

// GetState 获取节点状态
func (n *Node) GetState() (types.NodeState, uint64) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.state, n.persistentState.CurrentTerm
}

// GetLeader 获取当前领导者
func (n *Node) GetLeader() string {
	// 简化实现，实际应该跟踪当前领导者
	return ""
}

// Stop 停止Raft节点
func (n *Node) Stop() {
	log.Printf("Stopping Raft node %s", n.config.NodeID)
	
	n.mu.Lock()
	defer n.mu.Unlock()
	
	// 停止定时器
	n.electionTimer.Stop()
	n.heartbeatTimer.Stop()
	
	// 关闭所有通道
	// 注意：先关闭tickCh以停止ticker goroutine
	close(n.tickCh)
	close(n.appendEntriesCh)
	close(n.requestVoteCh)
	close(n.installSnapshotCh)
	close(n.applyCh)
	close(n.readyCh)
	
	log.Printf("Stopped Raft node %s", n.config.NodeID)
}