package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"processmanager/internal/config"
)

// Rotator 日志滚动器
type Rotator struct {
	config config.LogConfig
}

// NewRotator 创建日志滚动器
func NewRotator(config config.LogConfig) *Rotator {
	return &Rotator{
		config: config,
	}
}

// Rotate 滚动日志
func (r *Rotator) Rotate(logPath string) error {
	// 检查文件大小
	fileInfo, err := os.Stat(logPath)
	if err != nil {
		return fmt.Errorf("failed to stat log file: %w", err)
	}

	// 如果文件大小超过最大限制，进行滚动
	if fileInfo.Size() > int64(r.config.MaxSize)*1024*1024 {
		// 创建新的日志文件
		now := time.Now()
		newLogPath := fmt.Sprintf("%s.%s", logPath, now.Format("2006-01-02-15-04-05"))

		// 重命名文件
		if err := os.Rename(logPath, newLogPath); err != nil {
			return fmt.Errorf("failed to rename log file: %w", err)
		}

		// 如果启用了压缩，压缩旧日志文件
		if r.config.Compress {
			if err := r.compressLog(newLogPath); err != nil {
				return fmt.Errorf("failed to compress log file: %w", err)
			}
		}

		// 清理旧日志文件
		if err := r.cleanupOldLogs(filepath.Dir(logPath)); err != nil {
			return fmt.Errorf("failed to cleanup old logs: %w", err)
		}
	}

	return nil
}

// compressLog 压缩日志文件
func (r *Rotator) compressLog(logPath string) error {
	// 这里应该实现日志压缩的逻辑
	// 为了简化，这里只是打印一条日志
	fmt.Printf("Compressing log file: %s\n", logPath)
	return nil
}

// cleanupOldLogs 清理旧日志文件
func (r *Rotator) cleanupOldLogs(logDir string) error {
	files, err := os.ReadDir(logDir)
	if err != nil {
		return fmt.Errorf("failed to read log directory: %w", err)
	}

	// 过滤出日志文件
	var logFiles []struct {
		name string
		time time.Time
	}

	for _, file := range files {
		if !file.IsDir() && (filepath.Ext(file.Name()) == ".log" || filepath.Ext(file.Name()) == ".log.gz") {
			// 尝试解析文件名中的时间
			timeStr := filepath.Base(file.Name())
			timeStr = timeStr[:len(timeStr)-len(filepath.Ext(timeStr))]
			if filepath.Ext(timeStr) == ".log" {
				timeStr = timeStr[:len(timeStr)-4]
			}

			t, err := time.Parse("2006-01-02-15-04-05", timeStr)
			if err == nil {
				logFiles = append(logFiles, struct {
					name string
					time time.Time
				}{
					name: file.Name(),
					time: t,
				})
			}
		}
	}

	// 按时间排序
	sort.Slice(logFiles, func(i, j int) bool {
		return logFiles[i].time.Before(logFiles[j].time)
	})

	// 如果日志文件数量超过最大限制，删除最旧的文件
	if len(logFiles) > r.config.MaxFiles {
		for i := 0; i < len(logFiles)-r.config.MaxFiles; i++ {
			logPath := filepath.Join(logDir, logFiles[i].name)
			if err := os.Remove(logPath); err != nil {
				fmt.Printf("Failed to remove old log file: %v\n", err)
			} else {
				fmt.Printf("Removed old log file: %s\n", logPath)
			}
		}
	}

	return nil
}
