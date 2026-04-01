package manager

import (
	"io"
	"sync"

	"gopkg.in/natefinch/lumberjack.v2"
)

// LogWriter 实现了 io.Writer 接口
// 将日志数据同时写入文件（通过 lumberjack 支持轮转压缩）和所有注册的监听器 channel
type LogWriter struct {
	lj        *lumberjack.Logger
	listeners map[chan []byte]struct{}
	mu        sync.Mutex
}

// LogWriterConfig LogWriter 配置
type LogWriterConfig struct {
	Filename string // 日志文件路径
	MaxSize  int    // 单个日志文件最大大小（MB），默认 100
	MaxFiles int    // 保留的旧日志文件最大数量，默认保留所有
	Compress bool   // 是否压缩旧日志文件
}

// NewLogWriter 创建一个 LogWriter
func NewLogWriter(cfg LogWriterConfig) *LogWriter {
	maxSize := cfg.MaxSize
	if maxSize <= 0 {
		maxSize = 100
	}

	lj := &lumberjack.Logger{
		Filename:   cfg.Filename,
		MaxSize:    maxSize,
		MaxBackups: cfg.MaxFiles,
		Compress:   cfg.Compress,
		LocalTime:  true,
	}

	return &LogWriter{
		lj:        lj,
		listeners: make(map[chan []byte]struct{}),
	}
}

// Write 实现 io.Writer 接口
// 数据同时写入文件和所有监听器
func (w *LogWriter) Write(p []byte) (n int, err error) {
	// 写入文件（lumberjack 处理轮转）
	n, err = w.lj.Write(p)
	if err != nil {
		return n, err
	}

	// 广播给所有监听器
	w.mu.Lock()
	defer w.mu.Unlock()
	data := make([]byte, len(p))
	copy(data, p)
	for ch := range w.listeners {
		// 使用 select 避免 channel 满时阻塞主日志写入
		select {
		case ch <- data:
		default:
			// 监听器消费不过来，丢弃数据，避免阻塞子进程
		}
	}

	return n, nil
}

// AddListener 注册一个日志监听器，返回一个 channel 用于接收日志数据
func (w *LogWriter) AddListener() chan []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	ch := make(chan []byte, 256) // 带缓冲，减少数据丢失
	w.listeners[ch] = struct{}{}
	return ch
}

// RemoveListener 移除一个日志监听器
func (w *LogWriter) RemoveListener(ch chan []byte) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.listeners, ch)
	close(ch)
}

// Close 关闭日志写入器
func (w *LogWriter) Close() error {
	return w.lj.Close()
}

// UpdateConfig 更新日志轮转配置（热更新，无需重启进程）
func (w *LogWriter) UpdateConfig(cfg LogWriterConfig) {
	maxSize := cfg.MaxSize
	if maxSize <= 0 {
		maxSize = 100
	}
	w.lj.MaxSize = maxSize
	w.lj.MaxBackups = cfg.MaxFiles
	w.lj.Compress = cfg.Compress
}

// Ensure LogWriter implements io.Writer
var _ io.Writer = (*LogWriter)(nil)
