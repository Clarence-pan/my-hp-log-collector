package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"my-log-collector/internal/batcher"
	"my-log-collector/internal/config"
	"my-log-collector/internal/model"
	"my-log-collector/internal/offset"
	"my-log-collector/internal/tailer"
)

func main() {
	configPath := flag.String("config", "log-collector.yaml", "path to config file")
	flag.Parse()

	// 加载环境变量与 SLS 配置。
	envCfg, err := config.LoadEnvIfPresent()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s [FATAL] %v\n", time.Now().Format(time.RFC3339), err)
		os.Exit(1)
	}

	// 加载应用配置。
	appCfg, err := config.LoadAppConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s [FATAL] load config failed: %v\n", time.Now().Format(time.RFC3339), err)
		os.Exit(1)
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-host"
	}

	fmt.Fprintf(os.Stdout, "%s [INFO] starting log-collector, config=%s, hostname=%s\n",
		time.Now().Format(time.RFC3339), *configPath, hostname)

	// 初始化 offset store。
	store := offset.NewStore(appCfg.OffsetFile)
	if err := store.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "%s [WARN] failed to load offset store: %v\n", time.Now().Format(time.RFC3339), err)
	}

	// 全局日志通道。
	logCh := make(chan model.LogLine, 4096)

	// 上下文与信号处理。
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	// 启动 tailer 管理。
	tm := tailer.NewManager(appCfg, store, logCh)
	tm.Start(ctx, &wg)

	// 启动 batcher。
	bt := batcher.New(appCfg, envCfg, hostname, store, logCh)
	bt.Start(ctx, &wg)

	// 等待信号。
	sig := <-sigCh
	fmt.Fprintf(os.Stdout, "%s [INFO] received signal: %s, shutting down...\n", time.Now().Format(time.RFC3339), sig.String())
	cancel()

	// 等待各组件结束。
	wg.Wait()

	// 最后保存一次 offset。
	if err := store.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "%s [WARN] failed to save offset store on shutdown: %v\n", time.Now().Format(time.RFC3339), err)
	}

	fmt.Fprintf(os.Stdout, "%s [INFO] log-collector exited gracefully\n", time.Now().Format(time.RFC3339))
}

