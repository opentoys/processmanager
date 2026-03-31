package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config 应用配置
type Config struct {
	Log         LogConfig `json:"log"`
	StateFile   string    `json:"state_file"`
	MaxRestarts int       `json:"max_restarts"`
}

// LogConfig 日志配置
type LogConfig struct {
	Path     string `json:"path"`
	MaxSize  int    `json:"max_size"`
	MaxFiles int    `json:"max_files"`
	Compress bool   `json:"compress"`
}

// ProcessConfig 进程配置
type ProcessConfig struct {
	Name         string            `json:"name"`
	Script       string            `json:"script"`
	Args         []string          `json:"args"`
	Env          map[string]string `json:"env"`
	LogPath      string            `json:"log_path"`
	Cwd          string            `json:"cwd"`
	MaxRestarts  int               `json:"max_restarts"`
	RestartDelay int               `json:"restart_delay"`
}

// LoadConfig 加载配置文件
func LoadConfig(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
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
	if config.MaxRestarts == 0 {
		config.MaxRestarts = 255 // 默认最大重启次数为 255
	}

	return &config, nil
}

// SaveConfig 保存配置文件
func SaveConfig(filePath string, config *Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
