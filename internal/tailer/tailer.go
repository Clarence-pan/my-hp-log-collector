package tailer

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/nxadm/tail"

	"my-hp-log-collector/internal/config"
	"my-hp-log-collector/internal/model"
	"my-hp-log-collector/internal/offset"
)

// Manager 负责基于配置启动/管理多个 tail worker，并周期性发现新文件。
type Manager struct {
	cfg     *config.AppConfig
	store   *offset.Store
	logCh   chan<- model.LogLine
	mu      sync.Mutex
	tails   map[string]*tail.Tail // filePath -> tail instance
	lines   map[string]int64      // filePath -> current line number (仅供参考)
	startWg *sync.WaitGroup
}

// NewManager 创建一个新的 Manager。
func NewManager(cfg *config.AppConfig, store *offset.Store, logCh chan<- model.LogLine) *Manager {
	return &Manager{
		cfg:   cfg,
		store: store,
		logCh: logCh,
		tails: make(map[string]*tail.Tail),
		lines: make(map[string]int64),
	}
}

// TrackedFilesForTest 仅用于测试，返回当前已跟踪的文件路径列表。
func (m *Manager) TrackedFilesForTest() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	paths := make([]string, 0, len(m.tails))
	for p := range m.tails {
		paths = append(paths, p)
	}
	return paths
}

// Start 启动文件扫描与 tail 逻辑。
func (m *Manager) Start(ctx context.Context, wg *sync.WaitGroup) {
	m.startWg = wg
	wg.Add(1)
	go func() {
		defer wg.Done()
		m.run(ctx)
	}()
}

// run 主循环：定期扫描文件并维护 tail 集合。
func (m *Manager) run(ctx context.Context) {
	// 初次扫描
	m.scanOnce(ctx)

	ticker := time.NewTicker(m.cfg.SourcesScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.stopAll()
			return
		case <-ticker.C:
			m.scanOnce(ctx)
		}
	}
}

// scanOnce 针对所有启用的 sources 进行一次 glob 扫描，并对需要的文件启动/恢复 tail。
func (m *Manager) scanOnce(ctx context.Context) {
	for _, src := range m.cfg.Sources {
		if src.Enabled == nil || !*src.Enabled {
			continue
		}
		for _, pattern := range src.Patterns {
			paths, err := filepath.Glob(pattern)
			if err != nil {
				fmt.Printf("%s [WARN] invalid glob pattern '%s': %v\n", time.Now().Format(time.RFC3339), pattern, err)
				continue
			}
			for _, p := range paths {
				m.ensureTail(ctx, p)
			}
		}
	}
	// 清理已不存在的文件对应的 tail
	m.cleanupMissing()
}

// ensureTail 确保指定路径存在一个 tail，如无则基于 offset store 启动新的 tail。
func (m *Manager) ensureTail(ctx context.Context, path string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tails[path]; ok {
		return
	}

	seek := tail.SeekInfo{Whence: 2} // 默认从文件末尾
	if off, ok := m.store.Get(path); ok && off > 0 {
		seek = tail.SeekInfo{Offset: off, Whence: 0}
	}

	t, err := tail.TailFile(path, tail.Config{
		Follow:    true,
		ReOpen:    true,
		MustExist: false,
		Location:  &seek,
		Poll:      true,
	})
	if err != nil {
		fmt.Printf("%s [ERROR] failed to start tail for '%s': %v\n", time.Now().Format(time.RFC3339), path, err)
		return
	}

	m.tails[path] = t
	if _, ok := m.lines[path]; !ok {
		m.lines[path] = 0
	}

	if m.startWg != nil {
		m.startWg.Add(1)
	}
	go func(filePath string, tt *tail.Tail) {
		defer func() {
			if m.startWg != nil {
				m.startWg.Done()
			}
		}()
		m.consumeTail(ctx, filePath, tt)
	}(path, t)
}

// consumeTail 从 tail 实例中读取行并发送到全局 channel。
func (m *Manager) consumeTail(ctx context.Context, path string, t *tail.Tail) {
	for {
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case line, ok := <-t.Lines:
			if !ok {
				// Tail channel 关闭，结束该 worker。
				return
			}
			if line.Err != nil {
				fmt.Printf("%s [ERROR] tail error for '%s': %v\n", time.Now().Format(time.RFC3339), path, line.Err)
				continue
			}
			m.mu.Lock()
			m.lines[path]++
			lineNo := m.lines[path]
			m.mu.Unlock()

			m.logCh <- model.LogLine{
				FilePath: path,
				Line:     lineNo,
				Time:     time.Now(),
				Content:  line.Text,
			}
		}
	}
}

// cleanupMissing 停止并移除那些已经不在磁盘上的文件的 tail。
func (m *Manager) cleanupMissing() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for path, t := range m.tails {
		if _, err := filepath.Glob(path); err != nil {
			continue
		}
		// 这里简单地通过文件是否存在判断，若未来需要更精细可以改用 os.Stat。
		maybe, _ := filepath.Glob(path)
		if len(maybe) == 0 {
			t.Stop()
			delete(m.tails, path)
			delete(m.lines, path)
		}
	}
}

// stopAll 停止所有 tail 实例。
func (m *Manager) stopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for path, t := range m.tails {
		_ = t.Stop()
		delete(m.tails, path)
		delete(m.lines, path)
	}
}

