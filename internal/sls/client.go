package sls

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"my-hp-log-collector/internal/config"
	"my-hp-log-collector/internal/model"
)

// Client 封装阿里云 SLS Web Tracking HTTP 客户端。
type Client struct {
	env    *config.EnvConfig
	client *http.Client
}

// NewClient 创建新的 SLS 客户端。
func NewClient(env *config.EnvConfig) *Client {
	return &Client{
		env: env,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SetHTTPClient 主要用于测试场景下替换内部 http.Client。
func (c *Client) SetHTTPClient(hc *http.Client) {
	c.client = hc
}

// payload 表示单次上报的 JSON 结构。
type payload struct {
	Source string                 `json:"__source__"`
	Tags   map[string]string      `json:"__tags__"`
	Logs   []map[string]string    `json:"__logs__"`
	Extra  map[string]interface{} `json:"-"`
}

// SendLogs 按 filePath 维度上报一批日志。
func (c *Client) SendLogs(filePath, hostname, group string, logs []model.LogLine) error {
	now := time.Now().Format(time.RFC3339)
	if len(logs) == 0 {
		return nil
	}

	p := payload{
		Source: hostname,
		Tags: map[string]string{
			"filePath": filePath,
			"host":     hostname,
			"group":    group,
		},
		Logs: make([]map[string]string, 0, len(logs)),
	}

	for _, l := range logs {
		p.Logs = append(p.Logs, map[string]string{
			"time":    l.Time.Format(time.RFC3339Nano),
			"line":    fmt.Sprintf("%d", l.Line),
			"content": l.Content,
		})
	}

	raw, err := json.Marshal(p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s [ERROR] failed to marshal payload: %v\n", now, err)
		return err
	}

	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(raw); err != nil {
		_ = zw.Close()
		fmt.Fprintf(os.Stderr, "%s [ERROR] failed to compress payload: %v\n", now, err)
		return err
	}
	if err := zw.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "%s [ERROR] failed to finish compression: %v\n", now, err)
		return err
	}

	url := fmt.Sprintf("https://%s.%s/logstores/%s/track", c.env.Project, c.env.Host, c.env.Logstore)
	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s [ERROR] failed to create request: %v\n", now, err)
		return err
	}

	req.Header.Set("x-log-apiversion", "0.6.0")
	req.Header.Set("x-log-bodyrawsize", fmt.Sprintf("%d", len(raw)))
	req.Header.Set("x-log-compresstype", "deflate")
	req.Header.Set("Content-Type", "application/json")

	// 首次发送
	resp, err := c.client.Do(req)
	if shouldRetry, err2 := c.handleResponse(now, resp, err); shouldRetry {
		time.Sleep(1 * time.Second)
		// 重建请求体（buf 已被消费，重新压缩一次）
		var buf2 bytes.Buffer
		zw2 := zlib.NewWriter(&buf2)
		if _, err3 := zw2.Write(raw); err3 != nil {
			_ = zw2.Close()
			fmt.Fprintf(os.Stderr, "%s [ERROR] failed to recompress payload for retry: %v\n", now, err3)
			return err3
		}
		if err3 := zw2.Close(); err3 != nil {
			fmt.Fprintf(os.Stderr, "%s [ERROR] failed to finish recompression for retry: %v\n", now, err3)
			return err3
		}
		req2, err3 := http.NewRequest(http.MethodPost, url, &buf2)
		if err3 != nil {
			fmt.Fprintf(os.Stderr, "%s [ERROR] failed to create retry request: %v\n", now, err3)
			return err3
		}
		req2.Header = req.Header.Clone()

		resp2, err3 := c.client.Do(req2)
		if _, err4 := c.handleResponse(now, resp2, err3); err4 != nil {
			// 重试仍失败，直接返回错误。
			return err4
		}
		return nil
	} else if err2 != nil {
		return err2
	}

	return nil
}

// handleResponse 处理一次 HTTP 请求结果，返回 (shouldRetry, error)。
func (c *Client) handleResponse(ts string, resp *http.Response, err error) (bool, error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s [ERROR] failed to send logs: %v\n", ts, err)
		// 认为是可重试错误。
		fmt.Fprintf(os.Stderr, "%s [INFO] retrying after 1s due to send error\n", ts)
		return true, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Fprintf(os.Stdout, "%s [INFO] sent logs successfully, status=%d, body_len=%d\n", ts, resp.StatusCode, len(body))
		return false, nil
	}

	// 4xx 视为不可重试
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		fmt.Fprintf(os.Stderr, "%s [ERROR] non-retryable status=%d, body=%s\n", ts, resp.StatusCode, string(body))
		return false, fmt.Errorf("non-retryable status: %d", resp.StatusCode)
	}

	// 5xx 可重试一次
	if resp.StatusCode >= 500 {
		fmt.Fprintf(os.Stderr, "%s [ERROR] server error status=%d, body=%s\n", ts, resp.StatusCode, string(body))
		fmt.Fprintf(os.Stderr, "%s [INFO] retrying after 1s due to server error\n", ts)
		return true, fmt.Errorf("server error status: %d", resp.StatusCode)
	}

	// 其他情况统一视为错误但不重试。
	fmt.Fprintf(os.Stderr, "%s [ERROR] unexpected status=%d, body=%s\n", ts, resp.StatusCode, string(body))
	return false, fmt.Errorf("unexpected status: %d", resp.StatusCode)
}

