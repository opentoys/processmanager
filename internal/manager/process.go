package manager

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"processmanager/internal/logger"
	"processmanager/internal/notifier"
	"processmanager/internal/utils"

	"github.com/shirou/gopsutil/v4/process"
)

// Process 进程模型
type Process struct {
	config     *utils.ProcessConfig
	logManager *logger.LogManager
	cmd        *exec.Cmd
	pid        int
	status     string
	startTime  time.Time
	createdAt  time.Time
	restarts   int
	id         string
	manager    *ProcessManager
	env        []string   // 存储启动时的环境变量
	logWriter  *LogWriter // 多写日志写入器（文件 + 监听器）
}

// NewProcess 创建进程
func NewProcess(config *utils.ProcessConfig, logManager *logger.LogManager) (*Process, error) {
	// 确保日志目录存在
	logDir := filepath.Dir(config.LogPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	return &Process{
		config:     config,
		logManager: logManager,
		status:     utils.ProcessStatusStopped,
		createdAt:  time.Now(),
		id:         fmt.Sprintf("%d", time.Now().UnixNano()),
	}, nil
}

// SetManager 设置进程管理器
func (p *Process) SetManager(manager *ProcessManager) {
	p.manager = manager
}

// getNotifier 获取通知器（从管理器）
func (p *Process) getNotifier() *notifier.Notifier {
	if p.manager != nil {
		return p.manager.notifier
	}
	return nil
}

// Start 启动进程
func (p *Process) Start() error {
	// 构建命令
	p.cmd = exec.Command(p.config.Script, p.config.Args...)

	// 设置工作目录
	if p.config.Cwd != "" {
		p.cmd.Dir = p.config.Cwd
	}

	// 设置环境变量
	env := os.Environ()
	for _, v := range p.config.Env {
		env = append(env, v)
	}
	p.cmd.Env = env
	p.env = env // 记录当前环境变量

	// 创建 LogWriter，使用 lumberjack 支持日志轮转压缩
	p.logWriter = NewLogWriter(LogWriterConfig{
		Filename:    p.config.LogPath,
		MaxSize:     p.logManager.MaxSize(),
		MaxFiles:    p.logManager.MaxFiles(),
		Compress:    p.logManager.Compress(),
		Notifier:    p.getNotifier(),
		ProcessName: p.config.Name,
	})

	// 获取 stdout 和 stderr 的管道
	stdoutPipe, err := p.cmd.StdoutPipe()
	if err != nil {
		p.logWriter.Close()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := p.cmd.StderrPipe()
	if err != nil {
		p.logWriter.Close()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// 启动进程
	if err := p.cmd.Start(); err != nil {
		p.logWriter.Close()
		return fmt.Errorf("failed to start process: %w", err)
	}

	p.pid = p.cmd.Process.Pid
	p.status = utils.ProcessStatusRunning
	p.startTime = time.Now()
	p.restarts = 0 // 重置重启次数为 0

	slog.Debug("Process started", "process", p.config.Name, "pid", p.pid)

	// 异步保存状态
	if p.manager != nil {
		go p.manager.saveState()
	}

	// 异步将 stdout/stderr 通过 io.Copy 写入 LogWriter
	go func() {
		io.Copy(p.logWriter, stdoutPipe)
		io.Copy(p.logWriter, stderrPipe)
		// stdout/stderr 都关闭后，关闭 logWriter 中的文件
		p.logWriter.Close()
	}()

	// 监控进程
	go p.monitor()

	return nil
}

// Stop 停止进程
func (p *Process) Stop() error {
	if p.status != utils.ProcessStatusRunning {
		p.status = utils.ProcessStatusStopped
		// 保存状态
		if p.manager != nil {
			p.manager.saveState()
		}
		return nil
	}

	// 尝试通过 cmd.Process 停止进程
	if p.cmd != nil && p.cmd.Process != nil {
		if err := p.cmd.Process.Kill(); err != nil {
			slog.Warn("Failed to kill process via cmd.Process", "process", p.config.Name, "error", err)
			// 继续尝试通过 PID 停止进程
		} else {
			_, err := p.cmd.Process.Wait()
			if err != nil {
				slog.Warn("Failed to wait for process", "process", p.config.Name, "error", err)
			}
		}
	}

	// 尝试通过 PID 停止进程
	if p.pid > 0 {
		process, err := os.FindProcess(p.pid)
		if err != nil {
			slog.Warn("Failed to find process by PID", "process", p.config.Name, "error", err)
		} else {
			if err := process.Kill(); err != nil {
				slog.Warn("Failed to kill process by PID", "process", p.config.Name, "error", err)
			} else {
				_, err := process.Wait()
				if err != nil {
					slog.Warn("Failed to wait for process", "process", p.config.Name, "error", err)
				}
			}
		}
	}

	p.status = utils.ProcessStatusStopped
	p.pid = 0
	slog.Debug("Process stopped", "process", p.config.Name)

	// 同步保存状态
	if p.manager != nil {
		slog.Debug("Saving state...")
		if err := p.manager.saveState(); err != nil {
			slog.Error("Failed to save state", "error", err)
		} else {
			slog.Debug("State saved successfully")
		}
	}

	return nil
}

// Restart 重启进程
func (p *Process) Restart() error {
	if err := p.Stop(); err != nil {
		slog.Warn("Failed to stop process", "process", p.config.Name, "error", err)
	}

	// 延迟重启
	time.Sleep(time.Duration(p.config.RestartDelay) * time.Second)

	if err := p.Start(); err != nil {
		return err
	}

	p.restarts++
	slog.Debug("Process restarted", "process", p.config.Name, "restarts", p.restarts)

	// 保存状态
	if p.manager != nil {
		p.manager.saveState()
	}

	return nil
}

// GetStatus 获取进程状态
func (p *Process) GetStatus() ProcessStatus {
	var cpu, memory float64
	var uptime int64

	if p.status == utils.ProcessStatusRunning {
		// 获取进程的 CPU 和内存占用
		cpu, memory = p.getProcessUsage()
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

// getProcessUsage 获取进程的 CPU 和内存占用
func (p *Process) getProcessUsage() (float64, float64) {
	if p.pid <= 0 {
		return 0, 0
	}

	// 使用 gopsutil 包获取进程信息
	proc, err := process.NewProcess(int32(p.pid))
	if err != nil {
		slog.Debug("Failed to create process object", "error", err)
		return 0, 0
	}

	// 获取 CPU 使用率
	cpu, err := proc.CPUPercent()
	if err != nil {
		slog.Debug("Failed to get CPU percent", "error", err)
		cpu = 0
	}

	// 获取内存使用情况
	memInfo, err := proc.MemoryInfo()
	if err != nil {
		slog.Debug("Failed to get memory info", "error", err)
		return cpu, 0
	}

	slog.Debug("Process usage", "cpu", cpu, "memory", memInfo.RSS)

	return cpu, float64(memInfo.RSS)
}

// monitor 监控进程
func (p *Process) monitor() {
	_, err := p.cmd.Process.Wait()
	if err != nil {
		slog.Error("Process exited with error", "process", p.config.Name, "error", err)
		// 只有当进程异常退出时才标记为停止
		p.status = utils.ProcessStatusStopped

		// 保存状态
		if p.manager != nil {
			p.manager.saveState()
		}

		// 不再在这里自动重启，由 checkProcesses 方法统一处理
		slog.Info("Process exited with error, will be checked by monitor", "process", p.config.Name)
	} else {
		slog.Debug("Process exited normally", "process", p.config.Name)
		p.status = utils.ProcessStatusStopped

		// 保存状态
		if p.manager != nil {
			p.manager.saveState()
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
