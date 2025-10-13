package wal

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"myetcd/internal/types"
)

const (
	walFileNamePrefix = "wal-"
	walFilePerm       = 0644
)

var (
	ErrWALClosed    = errors.New("wal is closed")
	ErrWALCorrupted = errors.New("wal is corrupted")
	ErrNotFound     = errors.New("record not found")
)

/*
WAL (Write-Ahead Log) 预写日志机制详解：

1. 什么是WAL？
WAL是一种用于保证数据持久性的核心技术，广泛应用于数据库和分布式系统中。
它的核心思想是：在任何数据修改被应用到实际存储之前，必须先记录到日志中。

2. WAL的作用：
   - 持久性保证：确保即使系统崩溃，已提交的操作也不会丢失
   - 崩溃恢复：系统重启后可以通过重放WAL来恢复状态
   - 事务支持：为实现ACID特性提供基础

3. WAL的工作流程：
   a) 客户端发起写操作
   b) 将操作记录写入WAL并同步到磁盘
   c) 将操作应用到内存中的数据结构
   d) 返回成功响应给客户端

4. 为什么WAL很重要？
   - 防止数据丢失：即使内存数据丢失，也可以通过WAL恢复
   - 保证一致性：只有成功写入WAL的操作才会被应用
   - 支持回滚：可以通过WAL回滚未完成的操作

在这个实现中，我们使用分段WAL来管理日志文件，避免单个文件过大。
*/

// WAL Write-Ahead Log实现
// 
// 这个结构体实现了完整的预写日志功能，包括：
// - 日志记录的追加和读取
// - 文件分段管理
// - 数据完整性校验
// - 并发安全控制
type WAL struct {
	mu       sync.RWMutex  // 读写锁，保证并发安全
	dir      string        // WAL文件存储目录
	file     *os.File      // 当前活跃的WAL文件
	writer   *bufio.Writer // 带缓冲的写入器，提高写入性能
	index    uint64        // 当前日志索引（单调递增）
	term     uint64        // 当前任期（用于分布式一致性）
	closed   bool          // WAL是否已关闭
	fileSize int64         // 当前文件大小
	maxSize  int64         // 单个WAL文件的最大大小，超过时会创建新文件
}

// WALSegment WAL段信息
type WALSegment struct {
	Index uint64
	Term  uint64
	Path  string
	Size  int64
}

// NewWAL 创建新的WAL实例
func NewWAL(dir string, maxSize int64) (*WAL, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create wal directory: %w", err)
	}

	wal := &WAL{
		dir:     dir,
		maxSize: maxSize,
	}

	// 加载最新的WAL段
	if err := wal.loadLatestSegment(); err != nil {
		return nil, fmt.Errorf("failed to load latest segment: %w", err)
	}

	return wal, nil
}

// loadLatestSegment 加载最新的WAL段
func (w *WAL) loadLatestSegment() error {
	segments, err := w.listSegments()
	if err != nil {
		return err
	}

	if len(segments) == 0 {
		// 创建新的WAL段
		return w.createNewSegment(0, 0)
	}

	// 加载最新的段
	latest := segments[len(segments)-1]
	file, err := os.OpenFile(latest.Path, os.O_RDWR|os.O_APPEND, walFilePerm)
	if err != nil {
		return fmt.Errorf("failed to open wal segment: %w", err)
	}

	w.file = file
	w.writer = bufio.NewWriter(file)
	w.index = latest.Index
	w.term = latest.Term
	w.fileSize = latest.Size

	return nil
}

// listSegments 列出所有WAL段
func (w *WAL) listSegments() ([]WALSegment, error) {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read wal directory: %w", err)
	}

	var segments []WALSegment
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if len(name) < len(walFileNamePrefix) || name[:len(walFileNamePrefix)] != walFileNamePrefix {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		var index, term uint64
		if _, err := fmt.Sscanf(name[len(walFileNamePrefix):], "%d-%d", &index, &term); err != nil {
			continue
		}

		segments = append(segments, WALSegment{
			Index: index,
			Term:  term,
			Path:  filepath.Join(w.dir, name),
			Size:  info.Size(),
		})
	}

	return segments, nil
}

// createNewSegment 创建新的WAL段
func (w *WAL) createNewSegment(index, term uint64) error {
	if w.file != nil {
		if err := w.writer.Flush(); err != nil {
			return fmt.Errorf("failed to flush writer: %w", err)
		}
		if err := w.file.Close(); err != nil {
			return fmt.Errorf("failed to close file: %w", err)
		}
	}

	fileName := fmt.Sprintf("%s%016d-%016d", walFileNamePrefix, index, term)
	filePath := filepath.Join(w.dir, fileName)

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, walFilePerm)
	if err != nil {
		return fmt.Errorf("failed to create wal segment: %w", err)
	}

	w.file = file
	w.writer = bufio.NewWriter(file)
	w.index = index
	w.term = term
	w.fileSize = 0

	return nil
}

// Append 追加记录到WAL
// 
// 这是WAL的核心方法，实现了预写日志的关键步骤：
// 1. 获取写锁，确保并发安全
// 2. 为记录分配唯一的索引和任期
// 3. 序列化记录并计算校验和
// 4. 写入到磁盘文件
// 5. 更新索引
//
// 注意：这个方法必须保证原子性，要么完全成功，要么完全失败
func (w *WAL) Append(record *types.WALRecord) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return ErrWALClosed
	}

	// 设置记录的元数据
	// 索引是单调递增的，用于标识记录的顺序
	record.Index = w.index + 1
	// 任期用于分布式系统中的领导者选举和一致性保证
	record.Term = w.term
	// 时间戳用于调试和监控
	record.Timestamp = time.Now()

	// 序列化记录为JSON格式
	// JSON格式便于调试和跨语言支持
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	// 计算CRC32校验和
	// 这是WAL数据完整性的关键保证：
	// - 防止数据损坏
	// - 检测磁盘错误
	// - 确保数据一致性
	checksum := crc32.ChecksumIEEE(data)
	record.Checksum = checksum

	// 重新序列化包含校验和的记录
	data, err = json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal record with checksum: %w", err)
	}

	// 写入记录到文件
	// 这是最关键的步骤，必须确保数据真正写入磁盘
	if err := w.writeRecord(data); err != nil {
		return err
	}

	// 更新索引
	// 只有在成功写入后才更新索引，保证一致性
	w.index++
	return nil
}

// writeRecord 写入记录到文件
func (w *WAL) writeRecord(data []byte) error {
	// 检查是否需要创建新段
	if w.fileSize+int64(len(data)) > w.maxSize {
		if err := w.createNewSegment(w.index, w.term); err != nil {
			return fmt.Errorf("failed to create new segment: %w", err)
		}
	}

	// 写入长度前缀和数据
	length := uint32(len(data))
	if err := binaryWrite(w.writer, length); err != nil {
		return fmt.Errorf("failed to write length: %w", err)
	}

	n, err := w.writer.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}

	w.fileSize += int64(4 + n) // 4字节长度 + 数据
	return nil
}

// binaryWrite 二进制写入
func binaryWrite(w io.Writer, v uint32) error {
	b := []byte{
		byte(v >> 24),
		byte(v >> 16),
		byte(v >> 8),
		byte(v),
	}
	_, err := w.Write(b)
	return err
}

// binaryRead 二进制读取
func binaryRead(r io.Reader) (uint32, error) {
	var b [4]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), nil
}

// ReadAll 读取所有WAL记录
func (w *WAL) ReadAll() ([]*types.WALRecord, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.closed {
		return nil, ErrWALClosed
	}

	var records []*types.WALRecord

	// 读取所有段
	segments, err := w.listSegments()
	if err != nil {
		return nil, err
	}

	for _, segment := range segments {
		segmentRecords, err := w.readSegment(segment.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to read segment %s: %w", segment.Path, err)
		}
		records = append(records, segmentRecords...)
	}

	return records, nil
}

// readSegment 读取单个WAL段
func (w *WAL) readSegment(path string) ([]*types.WALRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open segment: %w", err)
	}
	defer file.Close()

	var records []*types.WALRecord
	reader := bufio.NewReader(file)

	for {
		// 读取长度
		length, err := binaryRead(reader)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to read length: %w", err)
		}

		// 读取数据
		data := make([]byte, length)
		if _, err := io.ReadFull(reader, data); err != nil {
			return nil, fmt.Errorf("failed to read data: %w", err)
		}

		// 解析记录
		var record types.WALRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, fmt.Errorf("failed to unmarshal record: %w", err)
		}

		// 验证校验和
		checksum := record.Checksum
		record.Checksum = 0
		verifyData, err := json.Marshal(record)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal record for verification: %w", err)
		}

		if crc32.ChecksumIEEE(verifyData) != checksum {
			return nil, ErrWALCorrupted
		}

		record.Checksum = checksum
		records = append(records, &record)
	}

	return records, nil
}

// Truncate 截断WAL到指定索引
func (w *WAL) Truncate(index uint64) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return ErrWALClosed
	}

	// 删除所有大于指定索引的段
	segments, err := w.listSegments()
	if err != nil {
		return err
	}

	for _, segment := range segments {
		if segment.Index > index {
			if err := os.Remove(segment.Path); err != nil {
				return fmt.Errorf("failed to remove segment %s: %w", segment.Path, err)
			}
		}
	}

	// 重新加载最新的段
	return w.loadLatestSegment()
}

// Sync 同步WAL到磁盘
func (w *WAL) Sync() error {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.closed {
		return ErrWALClosed
	}

	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}

	return w.file.Sync()
}

// Close 关闭WAL
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	var err error
	if w.writer != nil {
		if flushErr := w.writer.Flush(); flushErr != nil {
			err = fmt.Errorf("failed to flush writer: %w", flushErr)
		}
	}

	if w.file != nil {
		if closeErr := w.file.Close(); closeErr != nil {
			if err != nil {
				err = fmt.Errorf("%v; failed to close file: %w", err, closeErr)
			} else {
				err = fmt.Errorf("failed to close file: %w", closeErr)
			}
		}
	}

	w.closed = true
	return err
}

// Index 返回当前索引
func (w *WAL) Index() uint64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.index
}

// Term 返回当前任期
func (w *WAL) Term() uint64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.term
}

// SetTerm 设置当前任期
func (w *WAL) SetTerm(term uint64) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return ErrWALClosed
	}

	if w.term != term {
		return w.createNewSegment(w.index, term)
	}

	return nil
}
