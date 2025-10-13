package main

import (
	"fmt"
	"log"
	"os"

	"myetcd/internal/config"
	"myetcd/internal/types"
)

/*
配置文件示例程序

这个示例演示了如何使用MyEtcd的配置文件功能。
它展示了如何加载、验证和使用配置文件。
*/

func main() {
	// 配置文件路径
	configPath := "config/config.json"

	// 检查配置文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// 配置文件不存在，生成默认配置文件
		fmt.Printf("Config file not found, generating default config at %s\n", configPath)
		if err := config.GenerateDefaultConfig(configPath); err != nil {
			log.Fatalf("Failed to generate default config: %v", err)
		}
		fmt.Printf("Default config generated successfully\n")
	}

	// 加载配置
	fmt.Printf("Loading config from %s\n", configPath)
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 验证配置
	if err := config.ValidateConfig(cfg); err != nil {
		log.Fatalf("Config validation failed: %v", err)
	}

	// 打印配置信息
	printConfig(cfg)

	// 创建配置管理器
	fmt.Println("\n=== Using Config Manager ===")
	configManager := config.NewConfigManager(configPath)

	// 重新加载配置
	_, err = configManager.Load()
	if err != nil {
		log.Fatalf("Failed to load config with manager: %v", err)
	}

	// 获取配置
	managerConfig := configManager.GetConfig()
	fmt.Printf("Config loaded by manager: NodeID = %s\n", managerConfig.NodeID)

	// 修改配置
	fmt.Println("\n=== Modifying Config ===")
	modifiedConfig := *managerConfig
	modifiedConfig.NodeID = "modified-node"
	modifiedConfig.ClusterNodes = []string{"node1", "node2", "node3"}
	modifiedConfig.NodeAddresses = map[string]string{
		"node1": "localhost:8080",
		"node2": "localhost:8081",
		"node3": "localhost:8082",
	}

	// 更新配置管理器
	configManager.UpdateConfig(&modifiedConfig)

	// 保存修改后的配置
	if err := configManager.Save(); err != nil {
		log.Fatalf("Failed to save config: %v", err)
	}

	fmt.Println("Config modified and saved successfully")

	// 重新加载并验证
	_, err = configManager.Reload()
	if err != nil {
		log.Fatalf("Failed to reload config: %v", err)
	}

	// 打印修改后的配置
	printConfig(configManager.GetConfig())

	// 演示配置合并
	fmt.Println("\n=== Demonstrating Config Merge ===")
	baseConfig := &types.Config{
		DataDir:      "./base-data",
		NodeID:       "base-node",
		ClusterNodes: []string{"node1"},
	}

	overrideConfig := &types.Config{
		NodeID:       "override-node",
		ClusterNodes: []string{"node1", "node2"},
		WALDir:       "./override-wal",
	}

	mergedConfig := config.MergeConfigs(baseConfig, overrideConfig)
	fmt.Printf("Base config: NodeID = %s, DataDir = %s\n", baseConfig.NodeID, baseConfig.DataDir)
	fmt.Printf("Override config: NodeID = %s, WALDir = %s\n", overrideConfig.NodeID, overrideConfig.WALDir)
	fmt.Printf("Merged config: NodeID = %s, DataDir = %s, WALDir = %s\n",
		mergedConfig.NodeID, mergedConfig.DataDir, mergedConfig.WALDir)

	// 恢复原始配置
	fmt.Println("\n=== Restoring Original Config ===")
	if err := config.SaveConfig(cfg, configPath); err != nil {
		log.Fatalf("Failed to restore original config: %v", err)
	}

	fmt.Println("Original config restored")
}

// printConfig 打印配置信息
func printConfig(cfg *types.Config) {
	fmt.Println("\n=== Configuration ===")
	fmt.Printf("Data Directory: %s\n", cfg.DataDir)
	fmt.Printf("WAL Directory: %s\n", cfg.WALDir)
	fmt.Printf("Snapshot Directory: %s\n", cfg.SnapshotDir)
	fmt.Printf("Snapshot Interval: %v\n", cfg.SnapshotInterval)
	fmt.Printf("Max WAL Size: %d bytes\n", cfg.MaxWALSize)
	fmt.Printf("Max Snapshots: %d\n", cfg.MaxSnapshots)
	fmt.Printf("Heartbeat Interval: %v\n", cfg.HeartbeatInterval)
	fmt.Printf("Election Timeout: %v\n", cfg.ElectionTimeout)
	fmt.Printf("Node ID: %s\n", cfg.NodeID)
	fmt.Printf("Cluster Nodes: %v\n", cfg.ClusterNodes)
	fmt.Printf("Node Addresses:\n")
	for nodeID, addr := range cfg.NodeAddresses {
		fmt.Printf("  %s: %s\n", nodeID, addr)
	}
}
