package manager

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"processmanager/internal/config"
	"processmanager/internal/logger"

	"github.com/urfave/cli/v2"
)

// ProcessState 进程状态
type ProcessState struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Status       string            `json:"status"`
	PID          int               `json:"pid"`
	StartTime    time.Time         `json:"start_time"`
	CreatedAt    time.Time         `json:"created_at"`
	Restarts     int               `json:"restarts"`
	Script       string            `json:"script"`
	Args         []string          `json:"args"`
	Env          map[string]string `json:"env"`
	LogPath      string            `json:"log_path"`
	Cwd          string            `json:"cwd"`
	MaxRestarts  int               `json:"max_restarts"`
	RestartDelay int               `json:"restart_delay"`
	FullEnv      []string          `json:"full_env"` // 存储完整的环境变量
}

// StateFile 状态文件
type StateFile struct {
	Processes map[string]ProcessState `json:"processes"`
}

// ProcessManager 进程管理器
type ProcessManager struct {
	config     *config.Config
	processes  map[string]*Process
	logManager *logger.LogManager
	stateFile  string
}

// NewProcessManager 创建进程管理器
func NewProcessManager(cfg *config.Config) *ProcessManager {
	// 确保状态文件路径是绝对路径
	stateFile := cfg.StateFile
	if !filepath.IsAbs(stateFile) {
		absPath, err := filepath.Abs(stateFile)
		if err != nil {
			slog.Error("Failed to get absolute path for state file", "error", err)
		} else {
			stateFile = absPath
			slog.Debug("Using absolute path for state file", "path", stateFile)
		}
	}

	pm := &ProcessManager{
		config:     cfg,
		processes:  make(map[string]*Process),
		logManager: logger.NewLogManager(cfg.Log),
		stateFile:  stateFile,
	}

	// 加载进程状态
	if err := pm.loadState(); err != nil {
		slog.Error("Failed to load state", "error", err)
	}

	return pm
}

// StartProcess 启动进程
func (pm *ProcessManager) StartProcess(c *cli.Context) error {
	name := c.String("name")
	script := c.String("script")
	args := c.StringSlice("args")
	envFile := c.String("env")
	logPath := c.String("log")
	cwd := c.String("cwd")

	// 检查进程是否已存在
	if _, ok := pm.processes[name]; ok {
		return fmt.Errorf("process %s already exists", name)
	}

	// 读取环境变量
	env := make(map[string]string)
	if envFile != "" {
		if err := loadEnvFile(envFile, env); err != nil {
			return fmt.Errorf("failed to load env file: %w", err)
		}
	}

	// 设置工作目录
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	// 设置日志路径
	if logPath == "" {
		logPath = filepath.Join(pm.config.Log.Path, name+"-output.log")
	}

	// 创建进程配置
	procConfig := &config.ProcessConfig{
		Name:         name,
		Script:       script,
		Args:         args,
		Env:          env,
		LogPath:      logPath,
		Cwd:          cwd,
		MaxRestarts:  10,
		RestartDelay: 5,
	}

	// 创建进程
	process, err := NewProcess(procConfig, pm.logManager)
	if err != nil {
		return fmt.Errorf("failed to create process: %w", err)
	}

	// 启动进程
	if err := process.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	// 添加到进程列表
	pm.processes[name] = process
	process.SetManager(pm)

	// 打印进程列表大小
	slog.Debug("Process added to list", "count", len(pm.processes), "process", name)

	// 同步保存状态
	slog.Debug("Saving state...")
	if err := pm.saveState(); err != nil {
		slog.Error("Failed to save state", "error", err)
	} else {
		slog.Debug("State saved successfully")
	}

	fmt.Printf("Process %s started successfully\n", name)
	return nil
}

// ListProcesses 列出所有进程
func (pm *ProcessManager) ListProcesses(c *cli.Context) error {
	// 打印表格顶部边框
	fmt.Println("+-----+--------------------+----------+----------+----------+-----------------+----------+")
	// 打印表头
	fmt.Printf("| %-3s | %-18s | %-8s | %-8s | %-8s | %-15s | %-8s |\n", "ID", "Name", "Status", "PID", "CPU", "Memory", "Uptime")
	// 打印表头分隔线
	fmt.Println("+-----+--------------------+----------+----------+----------+-----------------+----------+")

	// 将进程转换为切片，以便使用索引
	var processes []*Process
	for _, process := range pm.processes {
		processes = append(processes, process)
	}

	// 遍历进程切片，使用索引作为 ID
	for i, process := range processes {
		// 检查进程是否还在运行
		if process.status == "running" {
			// 检查进程是否存在
			if process.pid > 0 {
				processObj, err := os.FindProcess(process.pid)
				if err != nil {
					process.status = "stopped"
				} else {
					// 向进程发送信号 0 来检查进程是否存在
					if err := processObj.Signal(syscall.Signal(0)); err != nil {
						process.status = "stopped"
					}
				}
			} else {
				process.status = "stopped"
			}
		}

		status := process.GetStatus()
		// 使用索引+1作为 ID
		id := i + 1
		// 打印进程信息
		fmt.Printf("| %-3d | %-18s | %-8s | %-8d | %-8.2f | %-15s | %-8s |\n",
			id,
			status.Name,
			status.Status,
			status.PID,
			status.CPU,
			formatMemory(status.Memory),
			formatUptime(status.Uptime),
		)
	}

	// 打印表格底部边框
	fmt.Println("+-----+--------------------+----------+----------+----------+-----------------+----------+")

	// 异步保存状态
	go pm.saveState()

	return nil
}

// ShowEnv 显示进程环境变量
func (pm *ProcessManager) ShowEnv(c *cli.Context) error {
	nameOrID := c.Args().First()
	if nameOrID == "" {
		return fmt.Errorf("process name or ID is required")
	}

	process, err := pm.GetProcessByNameOrID(nameOrID)
	if err != nil {
		return err
	}

	fmt.Printf("Environment variables for process %s:\n", process.config.Name)
	if process.env != nil {
		for _, envVar := range process.env {
			fmt.Printf("%s\n", envVar)
		}
	} else {
		// 如果没有记录的环境变量，显示配置中的环境变量
		for key, value := range process.config.Env {
			fmt.Printf("%s=%s\n", key, value)
		}
	}

	return nil
}

// ShowLog 显示进程日志
func (pm *ProcessManager) ShowLog(c *cli.Context) error {
	nameOrID := c.Args().First()
	if nameOrID == "" {
		return fmt.Errorf("process name or ID is required")
	}

	process, err := pm.GetProcessByNameOrID(nameOrID)
	if err != nil {
		return err
	}

	return pm.logManager.TailLog(process.config.LogPath)
}

// ShowAllLogs 显示所有进程日志
func (pm *ProcessManager) ShowAllLogs(c *cli.Context) error {
	for name, process := range pm.processes {
		fmt.Printf("=== Logs for process %s ===\n", name)
		if err := pm.logManager.TailLog(process.config.LogPath); err != nil {
			fmt.Printf("Failed to show logs for process %s: %v\n", name, err)
		}
		fmt.Println()
	}

	return nil
}

// StopProcess 停止进程
func (pm *ProcessManager) StopProcess(c *cli.Context) error {
	nameOrID := c.Args().First()
	if nameOrID == "" {
		return fmt.Errorf("process name or ID is required")
	}

	process, err := pm.GetProcessByNameOrID(nameOrID)
	if err != nil {
		return err
	}

	if err := process.Stop(); err != nil {
		return fmt.Errorf("failed to stop process: %w", err)
	}

	// 异步保存状态
	go func() {
		if err := pm.saveState(); err != nil {
			slog.Error("Failed to save state", "error", err)
		}
	}()

	fmt.Printf("Process %s stopped successfully\n", process.config.Name)
	return nil
}

// RestartProcess 重启进程
func (pm *ProcessManager) RestartProcess(c *cli.Context) error {
	nameOrID := c.Args().First()
	if nameOrID == "" {
		return fmt.Errorf("process name or ID is required")
	}

	process, err := pm.GetProcessByNameOrID(nameOrID)
	if err != nil {
		return err
	}

	if err := process.Restart(); err != nil {
		return fmt.Errorf("failed to restart process: %w", err)
	}

	// 同步保存状态
	slog.Debug("Saving state...")
	if err := pm.saveState(); err != nil {
		slog.Error("Failed to save state", "error", err)
	} else {
		slog.Debug("State saved successfully")
	}

	fmt.Printf("Process %s restarted successfully\n", process.config.Name)
	return nil
}

// DeleteProcess 删除进程
func (pm *ProcessManager) DeleteProcess(c *cli.Context) error {
	nameOrID := c.Args().First()
	if nameOrID == "" {
		return fmt.Errorf("process name or ID is required")
	}

	process, err := pm.GetProcessByNameOrID(nameOrID)
	if err != nil {
		return err
	}

	if err := process.Stop(); err != nil {
		return fmt.Errorf("failed to stop process: %w", err)
	}

	delete(pm.processes, process.config.Name)

	// 保存状态
	pm.saveState()

	fmt.Printf("Process %s deleted successfully\n", process.config.Name)
	return nil
}

// ShowStatus 显示进程状态
func (pm *ProcessManager) ShowStatus(c *cli.Context) error {
	nameOrID := c.Args().First()
	if nameOrID == "" {
		return fmt.Errorf("process name or ID is required")
	}

	process, err := pm.GetProcessByNameOrID(nameOrID)
	if err != nil {
		return err
	}

	status := process.GetStatus()
	fmt.Printf("Process %s status:\n", process.config.Name)
	fmt.Printf("ID: %s\n", status.ID)
	fmt.Printf("Name: %s\n", status.Name)
	fmt.Printf("Status: %s\n", status.Status)
	fmt.Printf("PID: %d\n", status.PID)
	fmt.Printf("CPU: %.2f%%\n", status.CPU)
	fmt.Printf("Memory: %d(%s)\n", status.Memory, formatMemory(status.Memory))
	fmt.Printf("Uptime: %s\n", formatUptime(status.Uptime))
	fmt.Printf("Restarts: %d\n", status.Restarts)
	fmt.Printf("Created At: %s\n", status.CreatedAt)
	fmt.Printf("Started At: %s\n", status.StartedAt)
	fmt.Printf("Log Path: %s\n", status.LogPath)

	return nil
}

// ReloadConfig 重新加载配置
func (pm *ProcessManager) ReloadConfig(c *cli.Context) error {
	newConfig, err := config.LoadConfig("config.yaml")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	pm.config = newConfig
	pm.logManager.UpdateConfig(newConfig.Log)

	// 保存状态
	pm.saveState()

	fmt.Println("Configuration reloaded successfully")
	return nil
}

// formatMemory 将字节转换为更友好的单位
func formatMemory(bytes uint64) string {
	const (
		_          = iota // ignore first value by assigning to blank identifier
		KB float64 = 1 << (10 * iota)
		MB
		GB
		TB
	)

	var unit string
	var size float64

	switch {
	case bytes >= uint64(TB):
		size = float64(bytes) / TB
		unit = "TB"
	case bytes >= uint64(GB):
		size = float64(bytes) / GB
		unit = "GB"
	case bytes >= uint64(MB):
		size = float64(bytes) / MB
		unit = "MB"
	case bytes >= uint64(KB):
		size = float64(bytes) / KB
		unit = "KB"
	default:
		size = float64(bytes)
		unit = "B"
	}

	return fmt.Sprintf("%.2f %s", size, unit)
}

// formatUptime 将秒数转换为人性化的格式，如 1d12h36m
func formatUptime(seconds int64) string {
	if seconds <= 0 {
		return "0s"
	}

	const (
		day    = 24 * 60 * 60
		hour   = 60 * 60
		minute = 60
	)

	days := seconds / day
	seconds %= day
	hours := seconds / hour
	seconds %= hour
	minutes := seconds / minute
	seconds %= minute

	parts := make([]string, 0, 4)

	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if seconds > 0 && len(parts) < 3 {
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}

	// 最多显示三个单位
	if len(parts) > 3 {
		parts = parts[:3]
	}

	return strings.Join(parts, "")
}

// GetProcessByNameOrID 根据名称或 ID 查找进程
func (pm *ProcessManager) GetProcessByNameOrID(nameOrID string) (*Process, error) {
	// 检查是否为数字 ID
	id, err := strconv.Atoi(nameOrID)
	if err == nil {
		// 按 ID 查找进程（ID 是索引+1）
		var processes []*Process
		for _, process := range pm.processes {
			processes = append(processes, process)
		}
		if id > 0 && id <= len(processes) {
			return processes[id-1], nil
		}
	}

	// 按名称查找进程
	if process, ok := pm.processes[nameOrID]; ok {
		return process, nil
	}

	return nil, fmt.Errorf("process %s not found", nameOrID)
}

// loadEnvFile 加载环境变量文件
func loadEnvFile(filePath string, env map[string]string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if len(line) > 0 && line[0] != '#' {
			parts := strings.Split(line, "=")
			if len(parts) >= 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(strings.Join(parts[1:], "="))
				env[key] = value
			}
		}
	}

	return nil
}

// saveState 保存进程状态
func (pm *ProcessManager) saveState() error {
	// 打印调试信息
	slog.Debug("Starting to save state", "stateFile", pm.stateFile, "processCount", len(pm.processes))

	// 确保状态文件路径是绝对路径
	stateFile := pm.stateFile
	if !filepath.IsAbs(stateFile) {
		absPath, err := filepath.Abs(stateFile)
		if err != nil {
			slog.Error("Failed to get absolute path for state file", "error", err)
			return fmt.Errorf("failed to get absolute path for state file: %w", err)
		}
		stateFile = absPath
		slog.Debug("Using absolute path for state file", "path", stateFile)
	}

	// 确保状态文件所在目录存在
	dir := filepath.Dir(stateFile)
	slog.Debug("Creating directory for state file", "dir", dir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("Failed to create directory for state file", "error", err)
		return fmt.Errorf("failed to create directory for state file: %w", err)
	}

	state := StateFile{
		Processes: make(map[string]ProcessState),
	}

	// 打印进程列表
	slog.Debug("Processes to save", "count", len(pm.processes))
	for name, process := range pm.processes {
		slog.Debug("Saving process", "name", name, "status", process.status, "pid", process.pid)
		state.Processes[name] = ProcessState{
			ID:           process.id,
			Name:         process.config.Name,
			Status:       process.status,
			PID:          process.pid,
			StartTime:    process.startTime,
			CreatedAt:    process.createdAt,
			Restarts:     process.restarts,
			Script:       process.config.Script,
			Args:         process.config.Args,
			Env:          process.config.Env,
			LogPath:      process.config.LogPath,
			Cwd:          process.config.Cwd,
			MaxRestarts:  process.config.MaxRestarts,
			RestartDelay: process.config.RestartDelay,
			FullEnv:      process.env,
		}
	}

	// 打印状态内容
	slog.Debug("State to save", "processCount", len(state.Processes))

	data, err := json.Marshal(state)
	if err != nil {
		slog.Error("Failed to marshal state", "error", err)
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// 打印序列化后的数据
	slog.Debug("Serialized state data", "data", string(data))

	slog.Debug("Writing state file", "path", stateFile)
	if err := os.WriteFile(stateFile, data, 0644); err != nil {
		slog.Error("Failed to write state file", "error", err, "path", stateFile)
		return fmt.Errorf("failed to write state file: %w", err)
	}

	// 验证文件是否被写入
	fileInfo, err := os.Stat(stateFile)
	if err != nil {
		slog.Error("Failed to stat state file", "error", err, "path", stateFile)
	} else {
		slog.Debug("State file written successfully", "path", stateFile, "size", fileInfo.Size())
	}

	slog.Debug("State saved successfully", "path", stateFile)
	return nil
}

// loadState 加载进程状态
func (pm *ProcessManager) loadState() error {
	// 确保状态文件路径是绝对路径
	stateFile := pm.stateFile
	if !filepath.IsAbs(stateFile) {
		absPath, err := filepath.Abs(stateFile)
		if err != nil {
			slog.Error("Failed to get absolute path for state file", "error", err)
			return fmt.Errorf("failed to get absolute path for state file: %w", err)
		}
		stateFile = absPath
	}

	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Debug("State file not found, creating new one", "path", stateFile)
			// 创建空的状态文件
			emptyState := StateFile{Processes: make(map[string]ProcessState)}
			data, err := json.Marshal(emptyState)
			if err != nil {
				slog.Error("Failed to marshal empty state", "error", err)
				return fmt.Errorf("failed to marshal empty state: %w", err)
			}
			if err := os.WriteFile(stateFile, data, 0644); err != nil {
				slog.Error("Failed to write empty state file", "error", err, "path", stateFile)
				return fmt.Errorf("failed to write empty state file: %w", err)
			}
			return nil
		}
		slog.Error("Failed to read state file", "error", err, "path", stateFile)
		return fmt.Errorf("failed to read state file: %w", err)
	}

	var state StateFile
	if err := json.Unmarshal(data, &state); err != nil {
		slog.Error("Failed to unmarshal state", "error", err, "path", stateFile)
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	// 加载进程配置
	for name, processState := range state.Processes {
		// 创建进程配置
		procConfig := &config.ProcessConfig{
			Name:         processState.Name,
			Script:       processState.Script,
			Args:         processState.Args,
			Env:          processState.Env,
			LogPath:      processState.LogPath,
			Cwd:          processState.Cwd,
			MaxRestarts:  processState.MaxRestarts,
			RestartDelay: processState.RestartDelay,
		}

		// 创建进程
		process, err := NewProcess(procConfig, pm.logManager)
		if err != nil {
			slog.Error("Failed to create process", "error", err, "process", processState.Name)
			continue
		}

		// 恢复进程状态
		process.id = processState.ID
		process.status = processState.Status
		process.pid = processState.PID
		process.startTime = processState.StartTime
		process.createdAt = processState.CreatedAt
		process.restarts = processState.Restarts
		process.env = processState.FullEnv

		// 设置管理器
		process.SetManager(pm)

		// 添加到进程列表
		pm.processes[name] = process
		slog.Debug("Process loaded from state", "process", name)
	}

	slog.Debug("State loaded successfully", "path", stateFile)
	return nil
}
