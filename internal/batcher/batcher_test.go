package batcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"my-log-collector/internal/config"
	"my-log-collector/internal/model"
	"my-log-collector/internal/offset"
	"my-log-collector/internal/sls"
)

// roundTripper 用于 batcher 测试中将请求重定向到本地测试服务。
type roundTripper struct {
	target *httptest.Server
}

func (r *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = r.target.Listener.Addr().String()
	return http.DefaultTransport.RoundTrip(req)
}

// TestBatcher_FlushUpdatesOffset 验证 batcher 能消费日志并在成功上报后更新 offset。
func TestBatcher_FlushUpdatesOffset(t *testing.T) {
	// 构造一个本地 HTTP 服务，模拟 SLS。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	env := &config.EnvConfig{
		Project:  "proj",
		Host:     "example.com",
		Logstore: "ls",
		LogGroup: "group",
	}

	appCfg := &config.AppConfig{
		Batch: config.BatchConfig{
			Interval: 100 * time.Millisecond,
			MaxSize:  10,
		},
	}

	store := offset.NewStore(t.TempDir() + "/offsets.json")
	logCh := make(chan model.LogLine, 10)

	b := New(appCfg, env, "host1", store, logCh)

	// 使用单独构造的 SLS 客户端，并把 Transport 指向本地测试服务。
	customClient := sls.NewClient(env)
	customClient.SetHTTPClient(&http.Client{
		Transport: &roundTripper{target: srv},
		Timeout:   5 * time.Second,
	})
	b.SetClientForTest(customClient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	b.Start(ctx, &wg)

	path := "/tmp/test.log"
	logCh <- model.LogLine{FilePath: path, Line: 1, Time: time.Now(), Content: "a"}
	logCh <- model.LogLine{FilePath: path, Line: 2, Time: time.Now(), Content: "b"}

	// 等待一个批次周期。
	time.Sleep(300 * time.Millisecond)
	cancel()
	wg.Wait()

	off, ok := store.Get(path)
	if !ok {
		t.Fatalf("expected offset to be stored for %s", path)
	}
	if off != 2 {
		t.Fatalf("expected offset 2, got %d", off)
	}
}

