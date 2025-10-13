package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"myetcd/internal/types"
)

/*
配置文件支持模块

这个模块实现了从文件加载配置的功能，支持JSON和YAML格式。
配置文件允许用户自定义MyEtcd的各种参数，包括：
- 数据存储路径
- 网络监听地址
- Raft集群配置
- 性能调优参数
- 日志级别等

使用配置文件的好处：
1. 无需重新编译即可修改配置
2. 支持不同环境的配置（开发、测试、生产）
3. 配置版本控制和审计
4. 配置验证和默认值处理
*/

// Loader 配置加载器接口
type Loader interface {
	Load(path string) (*types.Config, error)
	Save(config *types.Config, path string) error
}

// JSONConfig JSON配置结构体，用于处理时间间隔的字符串表示
type JSONConfig struct {
	DataDir           string            `json:"data_dir"`
	WALDir            string            `json:"wal_dir"`
	SnapshotDir       string            `json:"snapshot_dir"`
	SnapshotInterval  string            `json:"snapshot_interval"` // 字符串表示的时间间隔
	MaxWALSize        int64             `json:"max_wal_size"`
	MaxSnapshots      int               `json:"max_snapshots"`
	HeartbeatInterval string            `json:"heartbeat_interval"` // 字符串表示的时间间隔
	ElectionTimeout   string            `json:"election_timeout"`   // 字符串表示的时间间隔
	NodeID            string            `json:"node_id"`
	ClusterNodes      []string          `json:"cluster_nodes"`
	NodeAddresses     map[string]string `json:"node_addresses"`
}

// JSONLoader JSON配置加载器
type JSONLoader struct{}

// Load 从JSON文件加载配置
func (l *JSONLoader) Load(path string) (*types.Config, error) {
	// 检查文件是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// 文件不存在，返回默认配置
		return types.DefaultConfig(), nil
	}

	// 读取文件内容
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 解析JSON到临时结构体
	var jsonConfig JSONConfig
	if err := json.Unmarshal(data, &jsonConfig); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 转换为实际配置结构体
	config := &types.Config{
		DataDir:       jsonConfig.DataDir,
		WALDir:        jsonConfig.WALDir,
		SnapshotDir:   jsonConfig.SnapshotDir,
		MaxWALSize:    jsonConfig.MaxWALSize,
		MaxSnapshots:  jsonConfig.MaxSnapshots,
		NodeID:        jsonConfig.NodeID,
		ClusterNodes:  jsonConfig.ClusterNodes,
		NodeAddresses: jsonConfig.NodeAddresses,
	}

	// 解析时间间隔
	if jsonConfig.SnapshotInterval != "" {
		if duration, err := time.ParseDuration(jsonConfig.SnapshotInterval); err == nil {
			config.SnapshotInterval = duration
		}
	}

	if jsonConfig.HeartbeatInterval != "" {
		if duration, err := time.ParseDuration(jsonConfig.HeartbeatInterval); err == nil {
			config.HeartbeatInterval = duration
		}
	}

	if jsonConfig.ElectionTimeout != "" {
		if duration, err := time.ParseDuration(jsonConfig.ElectionTimeout); err == nil {
			config.ElectionTimeout = duration
		}
	}

	// 验证和填充默认值
	if err := l.validateAndFillDefaults(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

// Save 保存配置到JSON文件
func (l *JSONLoader) Save(config *types.Config, path string) error {
	// 验证配置
	if err := l.validateAndFillDefaults(config); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// 转换为JSON配置结构体
	jsonConfig := JSONConfig{
		DataDir:       config.DataDir,
		WALDir:        config.WALDir,
		SnapshotDir:   config.SnapshotDir,
		MaxWALSize:    config.MaxWALSize,
		MaxSnapshots:  config.MaxSnapshots,
		NodeID:        config.NodeID,
		ClusterNodes:  config.ClusterNodes,
		NodeAddresses: config.NodeAddresses,
	}

	// 格式化时间间隔为字符串
	jsonConfig.SnapshotInterval = config.SnapshotInterval.String()
	jsonConfig.HeartbeatInterval = config.HeartbeatInterval.String()
	jsonConfig.ElectionTimeout = config.ElectionTimeout.String()

	// 序列化为JSON
	data, err := json.MarshalIndent(jsonConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// validateAndFillDefaults 验证配置并填充默认值
func (l *JSONLoader) validateAndFillDefaults(config *types.Config) error {
	defaultConfig := types.DefaultConfig()

	// 验证并填充数据目录
	if config.DataDir == "" {
		config.DataDir = defaultConfig.DataDir
	}

	// 验证并填充WAL目录
	if config.WALDir == "" {
		config.WALDir = defaultConfig.WALDir
	}

	// 验证并填充快照目录
	if config.SnapshotDir == "" {
		config.SnapshotDir = defaultConfig.SnapshotDir
	}

	// 验证快照间隔
	if config.SnapshotInterval <= 0 {
		config.SnapshotInterval = defaultConfig.SnapshotInterval
	}

	// 验证WAL大小
	if config.MaxWALSize <= 0 {
		config.MaxWALSize = defaultConfig.MaxWALSize
	}

	// 验证最大快照数
	if config.MaxSnapshots <= 0 {
		config.MaxSnapshots = defaultConfig.MaxSnapshots
	}

	// 验证心跳间隔
	if config.HeartbeatInterval <= 0 {
		config.HeartbeatInterval = defaultConfig.HeartbeatInterval
	}

	// 验证选举超时
	if config.ElectionTimeout <= 0 {
		config.ElectionTimeout = defaultConfig.ElectionTimeout
	}

	// 验证节点ID
	if config.NodeID == "" {
		config.NodeID = defaultConfig.NodeID
	}

	// 验证集群节点
	if len(config.ClusterNodes) == 0 {
		config.ClusterNodes = defaultConfig.ClusterNodes
	}

	// 验证节点地址映射
	if config.NodeAddresses == nil {
		config.NodeAddresses = defaultConfig.NodeAddresses
	}

	return nil
}

// LoadConfig 加载配置文件
func LoadConfig(path string) (*types.Config, error) {
	// 根据文件扩展名选择加载器
	var loader Loader

	if len(path) > 5 && path[len(path)-5:] == ".json" {
		loader = &JSONLoader{}
	} else {
		// 默认使用JSON加载器
		loader = &JSONLoader{}
	}

	return loader.Load(path)
}

// SaveConfig 保存配置文件
func SaveConfig(config *types.Config, path string) error {
	// 根据文件扩展名选择加载器
	var loader Loader

	if len(path) > 5 && path[len(path)-5:] == ".json" {
		loader = &JSONLoader{}
	} else {
		// 默认使用JSON加载器
		loader = &JSONLoader{}
	}

	return loader.Save(config, path)
}

// GenerateDefaultConfig 生成默认配置文件
func GenerateDefaultConfig(path string) error {
	config := types.DefaultConfig()
	return SaveConfig(config, path)
}

// ConfigManager 配置管理器
type ConfigManager struct {
	configPath string
	config     *types.Config
	loader     Loader
}

// NewConfigManager 创建配置管理器
func NewConfigManager(configPath string) *ConfigManager {
	var loader Loader

	if len(configPath) > 5 && configPath[len(configPath)-5:] == ".json" {
		loader = &JSONLoader{}
	} else {
		loader = &JSONLoader{}
	}

	return &ConfigManager{
		configPath: configPath,
		loader:     loader,
	}
}

// Load 加载配置
func (cm *ConfigManager) Load() (*types.Config, error) {
	config, err := cm.loader.Load(cm.configPath)
	if err != nil {
		return nil, err
	}

	cm.config = config
	return config, nil
}

// Save 保存配置
func (cm *ConfigManager) Save() error {
	if cm.config == nil {
		return fmt.Errorf("no config to save")
	}

	return cm.loader.Save(cm.config, cm.configPath)
}

// GetConfig 获取当前配置
func (cm *ConfigManager) GetConfig() *types.Config {
	return cm.config
}

// UpdateConfig 更新配置
func (cm *ConfigManager) UpdateConfig(config *types.Config) {
	cm.config = config
}

// Reload 重新加载配置
func (cm *ConfigManager) Reload() (*types.Config, error) {
	return cm.Load()
}

// ValidateConfig 验证配置
func ValidateConfig(config *types.Config) error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}

	// 验证数据目录
	if config.DataDir == "" {
		return fmt.Errorf("data directory is required")
	}

	// 验证WAL目录
	if config.WALDir == "" {
		return fmt.Errorf("WAL directory is required")
	}

	// 验证快照目录
	if config.SnapshotDir == "" {
		return fmt.Errorf("snapshot directory is required")
	}

	// 验证节点ID
	if config.NodeID == "" {
		return fmt.Errorf("node ID is required")
	}

	// 验证时间间隔
	if config.SnapshotInterval <= 0 {
		return fmt.Errorf("snapshot interval must be positive")
	}

	if config.HeartbeatInterval <= 0 {
		return fmt.Errorf("heartbeat interval must be positive")
	}

	if config.ElectionTimeout <= 0 {
		return fmt.Errorf("election timeout must be positive")
	}

	// 验证选举超时必须大于心跳间隔
	if config.ElectionTimeout <= config.HeartbeatInterval {
		return fmt.Errorf("election timeout must be greater than heartbeat interval")
	}

	return nil
}

// MergeConfigs 合并配置（新配置覆盖旧配置）
func MergeConfigs(base, override *types.Config) *types.Config {
	if base == nil {
		return override
	}
	if override == nil {
		return base
	}

	result := *base

	// 覆盖非零值
	if override.DataDir != "" {
		result.DataDir = override.DataDir
	}
	if override.WALDir != "" {
		result.WALDir = override.WALDir
	}
	if override.SnapshotDir != "" {
		result.SnapshotDir = override.SnapshotDir
	}
	if override.SnapshotInterval > 0 {
		result.SnapshotInterval = override.SnapshotInterval
	}
	if override.MaxWALSize > 0 {
		result.MaxWALSize = override.MaxWALSize
	}
	if override.MaxSnapshots > 0 {
		result.MaxSnapshots = override.MaxSnapshots
	}
	if override.HeartbeatInterval > 0 {
		result.HeartbeatInterval = override.HeartbeatInterval
	}
	if override.ElectionTimeout > 0 {
		result.ElectionTimeout = override.ElectionTimeout
	}
	if override.NodeID != "" {
		result.NodeID = override.NodeID
	}
	if len(override.ClusterNodes) > 0 {
		result.ClusterNodes = override.ClusterNodes
	}
	if len(override.NodeAddresses) > 0 {
		result.NodeAddresses = override.NodeAddresses
	}

	return &result
}
