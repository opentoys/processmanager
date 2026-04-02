package logger

import (
	"log/slog"
	"os"

	"processmanager/internal/utils"
)

var logger *slog.Logger

// LogManager 日志管理器
type LogManager struct {
	config utils.LogConfig
}

// NewLogManager 创建日志管理器
func NewLogManager(config utils.LogConfig) *LogManager {
	// 确保日志目录存在
	if err := os.MkdirAll(config.Path, 0755); err != nil {
		slog.Error("Failed to create log directory", "error", err)
	}

	return &LogManager{
		config: config,
	}
}

// InitLogger 初始化日志
func InitLogger(config utils.LogConfig) {
	// 确保日志目录存在
	if err := os.MkdirAll(config.Path, 0755); err != nil {
		slog.Error("Failed to create log directory", "error", err)
	}

	// 初始化 slog
	logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)
}

// SetDebug 设置是否启用调试日志
func SetDebug(debug bool) {
	var level slog.Level
	if debug {
		level = slog.LevelDebug
	} else {
		level = slog.LevelInfo
	}

	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	}))
	slog.SetDefault(logger)
}

// UpdateConfig 更新配置
func (lm *LogManager) UpdateConfig(config utils.LogConfig) {
	lm.config = config
}

// MaxSize 返回单个日志文件最大大小（MB）
func (lm *LogManager) MaxSize() int {
	if lm.config.MaxSize <= 0 {
		return 100
	}
	return lm.config.MaxSize
}

// MaxFiles 返回保留的旧日志文件最大数量
func (lm *LogManager) MaxFiles() int {
	return lm.config.MaxFiles
}

// Compress 返回是否启用旧日志压缩
func (lm *LogManager) Compress() bool {
	return lm.config.Compress
}
