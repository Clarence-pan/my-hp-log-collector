package batcher

import (
	"context"
	"sync"
	"time"

	"my-hp-log-collector/internal/config"
	"my-hp-log-collector/internal/model"
	"my-hp-log-collector/internal/offset"
	"my-hp-log-collector/internal/sls"
)

// Batcher 负责从全局日志通道中消费日志，并按时间窗口和最大条数进行批量上报。
type Batcher struct {
	cfg       *config.AppConfig
	env       *config.EnvConfig
	hostname  string
	logGroup  string
	store     *offset.Store
	client    *sls.Client
	inputCh   <-chan model.LogLine
	mu        sync.Mutex
	perFile   map[string][]model.LogLine
	startOnce sync.Once
}

// New 创建一个新的 Batcher。
func New(cfg *config.AppConfig, envCfg *config.EnvConfig, hostname string, store *offset.Store, input <-chan model.LogLine) *Batcher {
	return &Batcher{
		cfg:      cfg,
		env:      envCfg,
		hostname: hostname,
		logGroup: envCfg.LogGroup,
		store:    store,
		client:   sls.NewClient(envCfg),
		inputCh:  input,
		perFile:  make(map[string][]model.LogLine),
	}
}

// SetClientForTest 仅用于测试时替换内部 SLS 客户端。
func (b *Batcher) SetClientForTest(c *sls.Client) {
	b.client = c
}

// Start 启动 batcher 主循环。
func (b *Batcher) Start(ctx context.Context, wg *sync.WaitGroup) {
	b.startOnce.Do(func() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.run(ctx)
		}()
	})
}

func (b *Batcher) run(ctx context.Context) {
	ticker := time.NewTicker(b.cfg.Batch.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			b.flushAll()
			return
		case line := <-b.inputCh:
			b.addLine(line)
		case <-ticker.C:
			b.flushAll()
		}
	}
}

// addLine 将新日志加入对应文件的批次缓冲。
func (b *Batcher) addLine(line model.LogLine) {
	b.mu.Lock()
	defer b.mu.Unlock()
	buf := b.perFile[line.FilePath]
	buf = append(buf, line)
	if len(buf) >= b.cfg.Batch.MaxSize {
		// 达到单批次最大条数时立即 flush。
		go b.flushFile(line.FilePath, buf)
		buf = nil
	}
	b.perFile[line.FilePath] = buf
}

// flushAll 将所有文件的缓冲批次发送出去。
func (b *Batcher) flushAll() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for path, buf := range b.perFile {
		if len(buf) == 0 {
			continue
		}
		go b.flushFile(path, buf)
		b.perFile[path] = nil
	}
}

// flushFile 将单个文件的一批日志上报到 SLS，并在成功后更新 offset。
func (b *Batcher) flushFile(path string, buf []model.LogLine) {
	if len(buf) == 0 {
		return
	}
	// 按计划，只在成功上报后推进 committedOffset。
	last := buf[len(buf)-1]
	if err := b.client.SendLogs(path, b.hostname, b.logGroup, buf); err != nil {
		// 失败时由 client 内部负责 stdout/stderr 打印和重试，这里只返回。
		return
	}
	// 上报成功，提交 offset。这里假设 offset 使用文件内字节偏移，简化为使用最后一行的行号做近似。
	// 若未来需要更精确，可以结合 tail.Tell() 的结果更新。
	b.store.Set(path, last.Line)
	_ = b.store.Save()
}

