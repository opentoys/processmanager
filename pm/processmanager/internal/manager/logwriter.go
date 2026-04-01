package manager

import (
	"io"
	"os"
	"sync"
)

// LogWriter 实现了 io.Writer 接口
// 将日志数据同时写入文件和所有注册的监听器 channel
type LogWriter struct {
	file      *os.File
	listeners map[chan []byte]struct{}
	mu        sync.Mutex
}

// NewLogWriter 创建一个 LogWriter
func NewLogWriter(file *os.File) *LogWriter {
	return &LogWriter{
		file:      file,
		listeners: make(map[chan []byte]struct{}),
	}
}

// Write 实现 io.Writer 接口
// 数据同时写入文件和所有监听器
func (w *LogWriter) Write(p []byte) (n int, err error) {
	// 写入文件
	n, err = w.file.Write(p)
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
	ch := make(chan []byte, 256) // 带缓冲，避免丢失数据
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

// Close 关闭日志文件（不关闭监听器，由监听器自行管理生命周期）
func (w *LogWriter) Close() error {
	return w.file.Close()
}

// Ensure LogWriter implements io.Writer
var _ io.Writer = (*LogWriter)(nil)
