## my-hq-log-collector

基于 Go 和阿里云 SLS Web Tracking API 的本地日志采集工具，支持：

- 多日志源（sources）与 glob pattern；
- 基于 `github.com/nxadm/tail` 的动态文件发现与 tail -F 行为；
- 每 3 秒批量上报到阿里云 SLS Web Tracking（`PutWebtracking`）；
- 断点续传（offset 持久化）与优雅退出；
- 简单的 stdout/stderr 英文日志输出。

### 环境准备

在工作目录下创建 `.env` 文件（也可以通过 shell 环境变量注入），至少包含：

```env
ALIYUN_SLS_PROJECT=your_project
ALIYUN_SLS_HOST=cn-hangzhou.log.aliyuncs.com
ALIYUN_SLS_LOGSTORE=your_logstore
ALIYUN_SLS_LOG_GROUP=your_group
```

程序启动时会优先尝试加载当前工作目录下的 `.env`，再从进程环境中读取变量。

### 配置文件

参考 `log-collector.example.yaml`：

```yaml
sources:
  - name: app-main
    enabled: true
    patterns:
      - /var/log/app/*.log
      - /var/log/app/*.out

batch:
  interval: 3s
  max_size: 5000

sources_scan_interval: 10s
offset_file: log-collector-offsets.json
offset_save_interval: 30s
```

- `sources[].enabled`：必填，是否启用该日志源；
- `sources[].patterns`：glob pattern 列表；
- `batch.interval`：批量上报时间窗口；
- `batch.max_size`：单批最大日志条数；
- `sources_scan_interval`：周期性 glob，发现新文件；
- `offset_file`：offset 持久化文件路径；
- `offset_save_interval`：offset 保存周期。

### 运行

```bash
go run ./cmd/log-collector --config log-collector.yaml
```

程序会：

- 使用 nxadm/tail 按 sources 配置 tail 日志文件；
- 每隔 `batch.interval` 或达到 `batch.max_size` 时批量上报；
- 每次成功/失败上报时，在 stdout/stderr 打印一行带时间戳的英文日志；
- 在收到 `SIGINT`/`SIGTERM` 时优雅退出，flush 剩余批次并保存 offset。

