package tailer

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"my-hp-log-collector/internal/config"
	"my-hp-log-collector/internal/model"
	"my-hp-log-collector/internal/offset"
)

// TestManager_DiscoverAndTrackFiles 验证基于 glob 的文件发现与动态新增文件跟踪。
func TestManager_DiscoverAndTrackFiles(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "a.log")
	file2 := filepath.Join(dir, "b.log")

	// 先创建第一个文件，第二个文件稍后再创建。
	if err := os.WriteFile(file1, []byte{}, 0o644); err != nil {
		t.Fatalf("create file1 failed: %v", err)
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
	appCfg.SourcesScanInterval = 100 * time.Millisecond

	store := offset.NewStore(filepath.Join(dir, "offsets.json"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 使用一个丢弃型的日志 channel，测试中不消费具体内容，仅关注 tracked 文件集合。
	dummyLogCh := make(chan model.LogLine, 1)

	var wg sync.WaitGroup
	m := NewManager(appCfg, store, dummyLogCh)

	m.Start(ctx, &wg)

	// 等待一次扫描周期，确认第一个文件被跟踪。
	time.Sleep(300 * time.Millisecond)
	tracked := m.TrackedFilesForTest()
	if len(tracked) == 0 {
		t.Fatalf("expected at least one tracked file")
	}

	// 创建第二个文件，等待下一次扫描后应被加入跟踪集合。
	if err := os.WriteFile(file2, []byte{}, 0o644); err != nil {
		t.Fatalf("create file2 failed: %v", err)
	}

	time.Sleep(400 * time.Millisecond)
	tracked = m.TrackedFilesForTest()

	found1 := false
	found2 := false
	for _, p := range tracked {
		if p == file1 {
			found1 = true
		}
		if p == file2 {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Fatalf("expected both file1 and file2 to be tracked, got %v", tracked)
	}

	cancel()
	wg.Wait()
}

