package utils

import "encoding/json"

// Command 客户端发送的命令
type Command struct {
	Action string          `json:"action"`
	Args   json.RawMessage `json:"args"`
}

// Response 服务端返回的响应
type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Config 应用配置
type Config struct {
	Log         LogConfig              `json:"log"`
	StateFile   string                 `json:"state_file"`
	MaxRestarts int                    `json:"max_restarts"`
	Channels    map[string]ChanConfig  `json:"channels,omitempty"`
	Notice      map[string]NoticeRule  `json:"notice,omitempty"`
}

// ChanConfig 通知通道配置
type ChanConfig struct {
	Type string `json:"type"` // wecombot | mail
	// 企微机器人
	Key string `json:"key,omitempty"`
	// 邮件
	To   string `json:"to,omitempty"`
	From string `json:"from,omitempty"`
	SMTP string `json:"smtp,omitempty"` // user:passwd@host:port
}

// NoticeRule 通知规则
type NoticeRule struct {
	Expr    string   `json:"expr"`              // expr 表达式，匹配日志内容
	Channel []string `json:"channel,omitempty"` // 通知通道名称列表
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
