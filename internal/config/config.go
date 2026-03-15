package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// AppConfig 表示主配置结构，从 YAML 解析。
type AppConfig struct {
	Sources []SourceConfig `yaml:"sources"`
	Batch   BatchConfig    `yaml:"batch"`

	// 可选：动态发现与断点配置
	SourcesScanInterval time.Duration `yaml:"sources_scan_interval"`
	OffsetFile          string        `yaml:"offset_file"`
	OffsetSaveInterval  time.Duration `yaml:"offset_save_interval"`
}

// SourceConfig 表示单个日志源的配置。
type SourceConfig struct {
	Enabled  *bool    `yaml:"enabled"`
	Name     string   `yaml:"name"`
	Patterns []string `yaml:"patterns"`
}

// BatchConfig 表示批量聚合配置。
type BatchConfig struct {
	Interval time.Duration `yaml:"interval"`
	MaxSize  int           `yaml:"max_size"`
}

// EnvConfig 表示从环境变量读取到的 SLS 相关配置。
type EnvConfig struct {
	Project  string
	Host     string
	Logstore string
	LogGroup string
}

// LoadEnvIfPresent 从工作目录下加载 .env 文件（若存在），然后读取并校验需要的环境变量。
func LoadEnvIfPresent() (*EnvConfig, error) {
	// 优先尝试加载当前工作目录下的 .env 文件；若不存在则忽略错误。
	_ = godotenv.Load()

	project := os.Getenv("ALIYUN_SLS_PROJECT")
	host := os.Getenv("ALIYUN_SLS_HOST")
	logstore := os.Getenv("ALIYUN_SLS_LOGSTORE")
	group := os.Getenv("ALIYUN_SLS_LOG_GROUP")

	missing := make([]string, 0, 4)
	if project == "" {
		missing = append(missing, "ALIYUN_SLS_PROJECT")
	}
	if host == "" {
		missing = append(missing, "ALIYUN_SLS_HOST")
	}
	if logstore == "" {
		missing = append(missing, "ALIYUN_SLS_LOGSTORE")
	}
	if group == "" {
		missing = append(missing, "ALIYUN_SLS_LOG_GROUP")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %v", missing)
	}

	return &EnvConfig{
		Project:  project,
		Host:     host,
		Logstore: logstore,
		LogGroup: group,
	}, nil
}

// LoadAppConfig 从给定路径读取并解析 YAML 配置。
func LoadAppConfig(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file failed: %w", err)
	}

	var cfg AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal yaml failed: %w", err)
	}

	applyDefaults(&cfg)

	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// applyDefaults 为部分可选字段设置默认值。
func applyDefaults(cfg *AppConfig) {
	if cfg.Batch.Interval == 0 {
		cfg.Batch.Interval = 3 * time.Second
	}
	if cfg.Batch.MaxSize == 0 {
		cfg.Batch.MaxSize = 5000
	}
	if cfg.SourcesScanInterval == 0 {
		cfg.SourcesScanInterval = 10 * time.Second
	}
	if cfg.OffsetFile == "" {
		cfg.OffsetFile = "log-collector-offsets.json"
	}
	if cfg.OffsetSaveInterval == 0 {
		cfg.OffsetSaveInterval = 30 * time.Second
	}
}

// validateConfig 校验配置的必要字段。
func validateConfig(cfg *AppConfig) error {
	if len(cfg.Sources) == 0 {
		return fmt.Errorf("no sources configured")
	}

	for i, s := range cfg.Sources {
		if s.Enabled == nil {
			return fmt.Errorf("source[%d] enabled is required", i)
		}
		if *s.Enabled {
			if s.Name == "" {
				return fmt.Errorf("source[%d] name is required when enabled", i)
			}
			if len(s.Patterns) == 0 {
				return fmt.Errorf("source[%d] patterns is required when enabled", i)
			}
		}
	}

	return nil
}

