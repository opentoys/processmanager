package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config 应用配置
type Config struct {
	Log       LogConfig       `yaml:"log"`
	Processes []ProcessConfig `yaml:"processes"`
	StateFile string          `yaml:"state_file"`
}

// LogConfig 日志配置
type LogConfig struct {
	Path     string `yaml:"path"`
	MaxSize  int    `yaml:"max_size"`
	MaxFiles int    `yaml:"max_files"`
	Compress bool   `yaml:"compress"`
}

// ProcessConfig 进程配置
type ProcessConfig struct {
	Name         string            `yaml:"name"`
	Script       string            `yaml:"script"`
	Args         []string          `yaml:"args"`
	Env          map[string]string `yaml:"env"`
	LogPath      string            `yaml:"log_path"`
	Cwd          string            `yaml:"cwd"`
	MaxRestarts  int               `yaml:"max_restarts"`
	RestartDelay int               `yaml:"restart_delay"`
}

// LoadConfig 加载配置文件
func LoadConfig(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 设置默认值
	if config.Log.Path == "" {
		config.Log.Path = "./logs"
	}
	if config.Log.MaxSize == 0 {
		config.Log.MaxSize = 100 // 100MB
	}
	if config.Log.MaxFiles == 0 {
		config.Log.MaxFiles = 10
	}
	if config.StateFile == "" {
		config.StateFile = "./pm.state"
	}

	return &config, nil
}

// SaveConfig 保存配置文件
func SaveConfig(filePath string, config *Config) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
