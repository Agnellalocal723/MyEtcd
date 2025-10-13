package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"myetcd/internal/api"
	"myetcd/internal/metrics"
	"myetcd/internal/storage"
	"myetcd/internal/types"
	"myetcd/internal/wal"
)

/*
指标收集示例程序

这个示例演示了如何使用MyEtcd的指标收集功能。
它启动一个带有指标收集的MyEtcd服务器，并展示如何访问指标端点。
*/

func main() {
	// 创建配置
	config := &types.Config{
		DataDir:          "./data",
		WALDir:           "./data/wal",
		SnapshotDir:      "./data/snapshot",
		SnapshotInterval: 5 * time.Minute, // 5分钟快照间隔
		NodeID:           "node1",
	}

	// 创建WAL
	w, err := wal.NewWAL(config.WALDir, 64*1024*1024) // 64MB
	if err != nil {
		log.Fatalf("Failed to create WAL: %v", err)
	}

	// 创建存储引擎
	engine, err := storage.NewEngine(config, w)
	if err != nil {
		log.Fatalf("Failed to create storage engine: %v", err)
	}

	// 创建API服务器
	server := api.NewServer(engine, config)
	server.SetAddr(":2379")

	// 启动指标服务器（在单独的端口）
	go func() {
		log.Printf("Starting metrics server on :2380")
		if err := metrics.SetupMetricsServer(":2380"); err != nil {
			log.Printf("Failed to start metrics server: %v", err)
		}
	}()

	// 启动API服务器
	go func() {
		log.Printf("Starting MyEtcd server on :2379")
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 创建客户端进行一些操作以生成指标
	client := api.NewClient("localhost:2379")

	// 定期执行一些操作以生成指标
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		counter := 0
		for {
			select {
			case <-ticker.C:
				key := fmt.Sprintf("test-key-%d", counter%10)
				value := fmt.Sprintf("test-value-%d", counter)

				// 执行PUT操作
				if err := client.Put(key, value, 10); err != nil {
					log.Printf("Failed to put key: %v", err)
				} else {
					log.Printf("Put key: %s", key)
				}

				// 执行GET操作
				if _, err := client.Get(key); err != nil {
					log.Printf("Failed to get key: %v", err)
				}

				counter++
			}
		}
	}()

	// 等待中断信号
	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("Received shutdown signal, shutting down...")
		cancel()
	}()

	// 等待上下文取消
	<-ctx.Done()

	// 优雅关闭
	if err := server.Stop(); err != nil {
		log.Printf("Error stopping server: %v", err)
	}

	// 关闭存储引擎
	if err := engine.Close(); err != nil {
		log.Printf("Error closing engine: %v", err)
	}

	log.Println("Server stopped")
}
