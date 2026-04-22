package utils

import (
	"net/url"
	"os"
	"path/filepath"
)

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

func DecodeURI(enc string) (dec string) {
	dec, _ = url.QueryUnescape(enc)
	return
}
