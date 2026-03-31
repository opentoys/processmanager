package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadEnvFile 加载环境变量文件
func LoadEnvFile(filePath string) (map[string]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read env file: %w", err)
	}

	env := make(map[string]string)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 0 && line[0] != '#' {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) >= 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				env[key] = value
			}
		}
	}

	return env, nil
}

// GetEnv 获取环境变量
func GetEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// GetWorkspacePath 获取工作目录路径
func GetWorkspacePath() string {
	// 检查 PM_WORKSPACE 环境变量
	if workspace := os.Getenv(PMENV_WORKSPACE); workspace != "" {
		return workspace
	}

	// 默认使用 $HOME/.pm/
	home, err := os.UserHomeDir()
	if err != nil {
		return "./"
	}
	return filepath.Join(home, ".pm")
}

// GetSocketPath 获取 Unix socket 路径
func GetSocketPath() string {
	return filepath.Join(GetWorkspacePath(), "pm.sock")
}
