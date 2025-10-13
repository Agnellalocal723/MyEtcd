package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"myetcd/internal/api"
	"myetcd/internal/storage"
	"myetcd/internal/types"
	"myetcd/internal/wal"
)

func main() {
	// 解析命令行参数
	var (
		dataDir        = flag.String("data-dir", "./data", "Data directory")
		walDir         = flag.String("wal-dir", "./data/wal", "WAL directory")
		snapshotDir    = flag.String("snapshot-dir", "./data/snapshot", "Snapshot directory")
		port           = flag.String("port", ":2379", "Server port")
		maxWALSize     = flag.Int64("max-wal-size", 64*1024*1024, "Maximum WAL size in bytes")
		snapshotInterval = flag.Duration("snapshot-interval", time.Hour, "Snapshot interval")
		maxSnapshots   = flag.Int("max-snapshots", 3, "Maximum number of snapshots to keep")
	)
	flag.Parse()

	// 创建配置
	config := &types.Config{
		DataDir:          *dataDir,
		WALDir:           *walDir,
		SnapshotDir:      *snapshotDir,
		SnapshotInterval: *snapshotInterval,
		MaxWALSize:       *maxWALSize,
		MaxSnapshots:     *maxSnapshots,
	}

	log.Printf("Starting MyEtcd with config:")
	log.Printf("  Data Dir: %s", config.DataDir)
	log.Printf("  WAL Dir: %s", config.WALDir)
	log.Printf("  Snapshot Dir: %s", config.SnapshotDir)
	log.Printf("  Max WAL Size: %d bytes", config.MaxWALSize)
	log.Printf("  Snapshot Interval: %v", config.SnapshotInterval)

	// 创建WAL
	walInstance, err := wal.NewWAL(config.WALDir, config.MaxWALSize)
	if err != nil {
		log.Fatalf("Failed to create WAL: %v", err)
	}
	defer func() {
		if err := walInstance.Close(); err != nil {
			log.Printf("Error closing WAL: %v", err)
		}
	}()

	log.Printf("WAL initialized, current index: %d, term: %d", walInstance.Index(), walInstance.Term())

	// 创建存储引擎
	engine, err := storage.NewEngine(config, walInstance)
	if err != nil {
		log.Fatalf("Failed to create storage engine: %v", err)
	}
	defer func() {
		if err := engine.Close(); err != nil {
			log.Printf("Error closing storage engine: %v", err)
		}
	}()

	log.Printf("Storage engine initialized, keys count: %d", engine.Size())

	// 创建API服务器
	server := api.NewServer(engine, config)
	
	// 设置服务器端口
	server.SetAddr(*port)

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动服务器
	go func() {
		log.Printf("Starting HTTP server on %s", *port)
		if err := server.Start(); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 等待信号
	sig := <-sigChan
	log.Printf("Received signal %v, shutting down...", sig)

	// 优雅关闭
	shutdownStart := time.Now()
	
	// 停止HTTP服务器
	if err := server.Stop(); err != nil {
		log.Printf("Error stopping server: %v", err)
	}

	// 关闭存储引擎（会自动创建最终快照）
	if err := engine.Close(); err != nil {
		log.Printf("Error closing storage engine: %v", err)
	}

	// 关闭WAL
	if err := walInstance.Close(); err != nil {
		log.Printf("Error closing WAL: %v", err)
	}

	shutdownDuration := time.Since(shutdownStart)
	log.Printf("Shutdown completed in %v", shutdownDuration)
	log.Printf("Goodbye!")
}
