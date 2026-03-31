package config

import (
	"encoding/json"
	"fmt"
	"os"
	"processmanager/internal/utils"
)

// LoadConfig 加载配置文件
func LoadConfig(filePath string, config *utils.Config) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 设置默认值
	if config.Log.Path == "" {
		config.Log.Path = utils.PMLogDir
	}
	if config.Log.MaxSize == 0 {
		config.Log.MaxSize = 100 // 100MB
	}
	if config.Log.MaxFiles == 0 {
		config.Log.MaxFiles = 10
	}
	if config.StateFile == "" {
		config.StateFile = utils.PMStateFile
	}
	if config.MaxRestarts == 0 {
		config.MaxRestarts = 255 // 默认最大重启次数为 255
	}

	return nil
}

// SaveConfig 保存配置文件
func SaveConfig(filePath string, config *utils.Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
