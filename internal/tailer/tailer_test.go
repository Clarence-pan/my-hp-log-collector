package tailer

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"my-log-collector/internal/config"
	"my-log-collector/internal/model"
	"my-log-collector/internal/offset"
)

// TestManager_TailAndDiscoverNewFiles 验证基本 tail 行为和动态发现新文件。
func TestManager_TailAndDiscoverNewFiles(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "a.log")
	file2 := filepath.Join(dir, "b.log")

	// 先创建第一个文件并写入一行。
	if err := os.WriteFile(file1, []byte("line1\n"), 0o644); err != nil {
		t.Fatalf("write file1 failed: %v", err)
	}

	enabled := true
	appCfg := &config.AppConfig{
		Sources: []config.SourceConfig{
			{
				Enabled:  &enabled,
				Name:     "test",
				Patterns: []string{filepath.Join(dir, "*.log")},
			},
		},
	}
	// 覆盖扫描间隔，使测试更快。
	appCfg.SourcesScanInterval = 100 * time.Millisecond

	store := offset.NewStore(filepath.Join(dir, "offsets.json"))
	logCh := make(chan model.LogLine, 10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	m := NewManager(appCfg, store, logCh)
	m.Start(ctx, &wg)

	// 等待 manager 扫描并开始 tail 第一个文件。
	select {
	case line := <-logCh:
		if line.FilePath != file1 {
			t.Fatalf("expected file1 path, got %s", line.FilePath)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for first log line")
	}

	// 创建第二个文件，模拟运行中新增文件。
	if err := os.WriteFile(file2, []byte("x1\n"), 0o644); err != nil {
		t.Fatalf("write file2 failed: %v", err)
	}

	// 再追加一行到第二个文件，确认动态发现。
	f2, err := os.OpenFile(file2, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open file2 failed: %v", err)
	}
	if _, err := f2.WriteString("x2\n"); err != nil {
		t.Fatalf("append file2 failed: %v", err)
	}
	_ = f2.Close()

	foundFile2 := false
	timeout := time.After(3 * time.Second)
	for !foundFile2 {
		select {
		case line := <-logCh:
			if line.FilePath == file2 {
				foundFile2 = true
			}
		case <-timeout:
			t.Fatalf("timeout waiting for logs from file2")
		}
	}

	cancel()
	wg.Wait()
}

