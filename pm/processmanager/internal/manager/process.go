package manager

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"processmanager/internal/config"
	"processmanager/internal/logger"

	"github.com/rs/zerolog/log"
)

// Process 进程模型
type Process struct {
	config     *config.ProcessConfig
	logManager *logger.LogManager
	cmd        *exec.Cmd
	pid        int
	status     string
	startTime  time.Time
	createdAt  time.Time
	restarts   int
	id         string
}

// NewProcess 创建进程
func NewProcess(config *config.ProcessConfig, logManager *logger.LogManager) (*Process, error) {
	// 确保日志目录存在
	logDir := filepath.Dir(config.LogPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	return &Process{
		config:     config,
		logManager: logManager,
		status:     "stopped",
		createdAt:  time.Now(),
		id:         fmt.Sprintf("%d", time.Now().UnixNano()),
	}, nil
}

// Start 启动进程
func (p *Process) Start() error {
	// 构建命令
	cmd := exec.Command(p.config.Script, p.config.Args...)

	// 设置工作目录
	if p.config.Cwd != "" {
		cmd.Dir = p.config.Cwd
	}

	// 设置环境变量
	env := os.Environ()
	for key, value := range p.config.Env {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	cmd.Env = env

	// 设置日志文件
	logFile, err := os.OpenFile(p.config.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer logFile.Close()

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// 启动进程
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	p.cmd = cmd
	p.pid = cmd.Process.Pid
	p.status = "running"
	p.startTime = time.Now()

	log.Info().Str("process", p.config.Name).Int("pid", p.pid).Msg("Process started")

	// 监控进程
	go p.monitor()

	return nil
}

// Stop 停止进程
func (p *Process) Stop() error {
	if p.cmd == nil || p.status != "running" {
		return fmt.Errorf("process is not running")
	}

	if err := p.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to kill process: %w", err)
	}

	_, err := p.cmd.Process.Wait()
	if err != nil {
		return fmt.Errorf("failed to wait for process: %w", err)
	}

	p.status = "stopped"
	log.Info().Str("process", p.config.Name).Msg("Process stopped")

	return nil
}

// Restart 重启进程
func (p *Process) Restart() error {
	if err := p.Stop(); err != nil {
		log.Warn().Str("process", p.config.Name).Err(err).Msg("Failed to stop process")
	}

	// 延迟重启
	time.Sleep(time.Duration(p.config.RestartDelay) * time.Second)

	if err := p.Start(); err != nil {
		return err
	}

	p.restarts++
	log.Info().Str("process", p.config.Name).Int("restarts", p.restarts).Msg("Process restarted")

	return nil
}

// GetStatus 获取进程状态
func (p *Process) GetStatus() ProcessStatus {
	var cpu, memory float64
	var uptime int64

	if p.status == "running" {
		// 这里应该实现获取 CPU 和内存使用率的逻辑
		// 为了简化，这里返回 0
		uptime = int64(time.Since(p.startTime).Seconds())
	}

	return ProcessStatus{
		ID:        p.id,
		Name:      p.config.Name,
		Status:    p.status,
		PID:       p.pid,
		CPU:       cpu,
		Memory:    uint64(memory),
		Uptime:    int64(uptime),
		Restarts:  p.restarts,
		CreatedAt: p.createdAt,
		StartedAt: p.startTime,
		LogPath:   p.config.LogPath,
	}
}

// monitor 监控进程
func (p *Process) monitor() {
	_, err := p.cmd.Process.Wait()
	if err != nil {
		log.Error().Str("process", p.config.Name).Err(err).Msg("Process exited with error")
	} else {
		log.Info().Str("process", p.config.Name).Msg("Process exited normally")
	}

	p.status = "stopped"

	// 自动重启
	if p.restarts < p.config.MaxRestarts {
		log.Info().Str("process", p.config.Name).Int("restarts", p.restarts).Msg("Auto-restarting process")
		if err := p.Restart(); err != nil {
			log.Error().Str("process", p.config.Name).Err(err).Msg("Failed to restart process")
		}
	}
}

// ProcessStatus 进程状态
type ProcessStatus struct {
	ID        string
	Name      string
	Status    string
	PID       int
	CPU       float64
	Memory    uint64
	Uptime    int64
	Restarts  int
	CreatedAt time.Time
	StartedAt time.Time
	LogPath   string
}
