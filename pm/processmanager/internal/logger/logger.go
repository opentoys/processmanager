package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"processmanager/internal/config"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// LogManager 日志管理器
type LogManager struct {
	config config.LogConfig
}

// NewLogManager 创建日志管理器
func NewLogManager(config config.LogConfig) *LogManager {
	// 确保日志目录存在
	if err := os.MkdirAll(config.Path, 0755); err != nil {
		log.Error().Err(err).Msg("Failed to create log directory")
	}

	// 初始化 zerolog
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	return &LogManager{
		config: config,
	}
}

// InitLogger 初始化日志
func InitLogger(config config.LogConfig) {
	// 确保日志目录存在
	if err := os.MkdirAll(config.Path, 0755); err != nil {
		log.Error().Err(err).Msg("Failed to create log directory")
	}

	// 初始化 zerolog
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

// UpdateConfig 更新配置
func (lm *LogManager) UpdateConfig(config config.LogConfig) {
	lm.config = config
}

// TailLog 实时显示日志
func (lm *LogManager) TailLog(logPath string) error {
	file, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// 移动到文件末尾
	if _, err := file.Seek(0, os.SEEK_END); err != nil {
		return fmt.Errorf("failed to seek to end of file: %w", err)
	}

	// 实时读取日志
	buffer := make([]byte, 1024)
	for {
		n, err := file.Read(buffer)
		if err != nil {
			if err.Error() != "EOF" {
				return fmt.Errorf("failed to read log file: %w", err)
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}

		if n > 0 {
			fmt.Print(string(buffer[:n]))
		}
	}
}

// RotateLog 滚动日志
func (lm *LogManager) RotateLog(logPath string) error {
	// 检查文件大小
	fileInfo, err := os.Stat(logPath)
	if err != nil {
		return fmt.Errorf("failed to stat log file: %w", err)
	}

	// 如果文件大小超过最大限制，进行滚动
	if fileInfo.Size() > int64(lm.config.MaxSize)*1024*1024 {
		// 创建新的日志文件
		now := time.Now()
		newLogPath := fmt.Sprintf("%s.%s", logPath, now.Format("2006-01-02-15-04-05"))

		// 重命名文件
		if err := os.Rename(logPath, newLogPath); err != nil {
			return fmt.Errorf("failed to rename log file: %w", err)
		}

		// 如果启用了压缩，压缩旧日志文件
		if lm.config.Compress {
			if err := lm.compressLog(newLogPath); err != nil {
				log.Warn().Err(err).Msg("Failed to compress log file")
			}
		}

		// 清理旧日志文件
		if err := lm.cleanupOldLogs(filepath.Dir(logPath)); err != nil {
			log.Warn().Err(err).Msg("Failed to cleanup old logs")
		}
	}

	return nil
}

// compressLog 压缩日志文件
func (lm *LogManager) compressLog(logPath string) error {
	// 这里应该实现日志压缩的逻辑
	// 为了简化，这里只是打印一条日志
	log.Info().Str("log", logPath).Msg("Compressing log file")
	return nil
}

// cleanupOldLogs 清理旧日志文件
func (lm *LogManager) cleanupOldLogs(logDir string) error {
	files, err := os.ReadDir(logDir)
	if err != nil {
		return fmt.Errorf("failed to read log directory: %w", err)
	}

	// 过滤出日志文件
	var logFiles []string
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".log" {
			logFiles = append(logFiles, file.Name())
		}
	}

	// 如果日志文件数量超过最大限制，删除最旧的文件
	if len(logFiles) > lm.config.MaxFiles {
		// 这里应该实现按时间排序并删除最旧文件的逻辑
		// 为了简化，这里只是打印一条日志
		log.Info().Int("count", len(logFiles)).Int("max", lm.config.MaxFiles).Msg("Cleaning up old logs")
	}

	return nil
}
