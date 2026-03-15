package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLoadEnvIfPresent_Success 确认在环境变量完整时可以正常加载。
func TestLoadEnvIfPresent_Success(t *testing.T) {
	t.Setenv("ALIYUN_SLS_PROJECT", "proj")
	t.Setenv("ALIYUN_SLS_HOST", "cn-hz.log.aliyuncs.com")
	t.Setenv("ALIYUN_SLS_LOGSTORE", "store")
	t.Setenv("ALIYUN_SLS_LOG_GROUP", "group")

	env, err := LoadEnvIfPresent()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if env.Project != "proj" || env.Host != "cn-hz.log.aliyuncs.com" ||
		env.Logstore != "store" || env.LogGroup != "group" {
		t.Fatalf("unexpected env config: %+v", env)
	}
}

// TestLoadEnvIfPresent_Missing 校验缺失变量时报错。
func TestLoadEnvIfPresent_Missing(t *testing.T) {
	// 确保变量为空。
	t.Setenv("ALIYUN_SLS_PROJECT", "")
	t.Setenv("ALIYUN_SLS_HOST", "")
	t.Setenv("ALIYUN_SLS_LOGSTORE", "")
	t.Setenv("ALIYUN_SLS_LOG_GROUP", "")

	_, err := LoadEnvIfPresent()
	if err == nil {
		t.Fatalf("expected error for missing env vars, got nil")
	}
}

// TestLoadAppConfig_Valid 检查 YAML 解析、默认值与校验逻辑。
func TestLoadAppConfig_Valid(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	yaml := `
sources:
  - name: app-main
    enabled: true
    patterns:
      - /var/log/app/*.log

batch:
  interval: 3s
  max_size: 100
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write yaml failed: %v", err)
	}

	cfg, err := LoadAppConfig(cfgPath)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(cfg.Sources))
	}
	if cfg.Batch.Interval != 3*time.Second {
		t.Fatalf("unexpected batch interval: %v", cfg.Batch.Interval)
	}
	if cfg.Batch.MaxSize != 100 {
		t.Fatalf("unexpected batch max_size: %d", cfg.Batch.MaxSize)
	}
	// 默认值检查。
	if cfg.SourcesScanInterval <= 0 {
		t.Fatalf("expected default SourcesScanInterval > 0")
	}
	if cfg.OffsetFile == "" {
		t.Fatalf("expected default OffsetFile not empty")
	}
	if cfg.OffsetSaveInterval <= 0 {
		t.Fatalf("expected default OffsetSaveInterval > 0")
	}
}

