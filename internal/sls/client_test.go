package sls

import (
	"bytes"
	"compress/zlib"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"my-hp-log-collector/internal/config"
	"my-hp-log-collector/internal/model"
)

// captureOutput 用于在测试中捕获 stdout 或 stderr 输出。
func captureOutput(f func()) string {
	// 使用管道重定向输出
	reader, writer, _ := os.Pipe()
	origStdout := os.Stdout
	origStderr := os.Stderr

	// 同时重定向 stdout/stderr，方便复用。
	os.Stdout = writer
	os.Stderr = writer

	// 执行待测逻辑
	f()

	// 还原
	_ = writer.Close()
	os.Stdout = origStdout
	os.Stderr = origStderr

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, reader)
	_ = reader.Close()
	return buf.String()
}

func TestHandleResponse_SuccessLogsCount(t *testing.T) {
	c := &Client{}
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("ok")),
	}

	output := captureOutput(func() {
		shouldRetry, err := c.handleResponse("2025-01-01T00:00:00Z", resp, nil, 3)
		if shouldRetry {
			t.Fatalf("expected shouldRetry to be false")
		}
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	if !strings.Contains(output, "logs_count=3") {
		t.Fatalf("expected output to contain logs_count=3, got: %s", output)
	}
}

func TestHandleResponse_ServerErrorLogsCount(t *testing.T) {
	c := &Client{}
	resp := &http.Response{
		StatusCode: 500,
		Body:       io.NopCloser(strings.NewReader("server error")),
	}

	output := captureOutput(func() {
		shouldRetry, err := c.handleResponse("2025-01-01T00:00:00Z", resp, nil, 5)
		if !shouldRetry {
			t.Fatalf("expected shouldRetry to be true")
		}
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	if !strings.Contains(output, "logs_count=5") {
		t.Fatalf("expected output to contain logs_count=5, got: %s", output)
	}
}

// roundTripper 用于将请求重定向到 httptest.Server。
type roundTripper struct {
	target *httptest.Server
}

func (r *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// 重写请求的目标为本地测试服务。
	req.URL.Scheme = "http"
	req.URL.Host = r.target.Listener.Addr().String()
	return http.DefaultTransport.RoundTrip(req)
}

func TestSendLogs_Success(t *testing.T) {
	var receivedBody []byte
	var receivedHeaders http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		compressed, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body failed: %v", err)
		}
		// 解压以确认格式正确。
		zr, err := zlib.NewReader(bytes.NewReader(compressed))
		if err != nil {
			t.Fatalf("new zlib reader failed: %v", err)
		}
		defer zr.Close()
		raw, err := io.ReadAll(zr)
		if err != nil {
			t.Fatalf("uncompress failed: %v", err)
		}
		receivedBody = raw
		w.WriteHeader(200)
	}))
	defer srv.Close()

	env := &config.EnvConfig{
		Project:  "proj",
		Host:     "example.com",
		Logstore: "ls",
		LogGroup: "group",
	}
	c := NewClient(env)
	// 使用自定义 Transport，将请求转发到本地测试服务。
	c.SetHTTPClient(&http.Client{
		Transport: &roundTripper{target: srv},
		Timeout:   5 * time.Second,
	})

	now := time.Now()
	logs := []model.LogLine{
		{FilePath: "/tmp/a.log", Line: 1, Time: now, Content: "hello"},
		{FilePath: "/tmp/a.log", Line: 2, Time: now.Add(time.Millisecond), Content: "world"},
	}

	if err := c.SendLogs("/tmp/a.log", "host1", "group1", logs); err != nil {
		t.Fatalf("SendLogs returned error: %v", err)
	}

	if len(receivedBody) == 0 {
		t.Fatalf("expected body to be received")
	}
	if receivedHeaders.Get("x-log-compresstype") != "deflate" {
		t.Fatalf("unexpected compresstype header: %s", receivedHeaders.Get("x-log-compresstype"))
	}
	if receivedHeaders.Get("x-log-apiversion") != "0.6.0" {
		t.Fatalf("unexpected apiversion header: %s", receivedHeaders.Get("x-log-apiversion"))
	}
}

