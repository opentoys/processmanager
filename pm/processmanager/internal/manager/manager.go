package manager

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"processmanager/internal/config"
	"processmanager/internal/logger"
	"processmanager/internal/utils"

	"github.com/fsnotify/fsnotify"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/urfave/cli/v2"
)

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
	config     *utils.Config
	processes  map[string]*Process
	logManager *logger.LogManager
	stateFile  string
	running    bool          // 守护进程运行状态
	stopChan   chan struct{} // 停止信号通道
	pidFile    string        // 存储守护进程 PID 的文件
	socketPath string        // Unix socket 路径
	startTime  time.Time     // 守护进程启动时间
}

// GetWorkspacePath 获取工作目录路径
func GetWorkspacePath() string {
	// 检查 PM_WORKSPACE 环境变量
	if workspace := os.Getenv(utils.PMENV_WORKSPACE); workspace != "" {
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
	return filepath.Join(GetWorkspacePath(), utils.PMSocketFile)
}

// NewProcessManager 创建进程管理器
func NewProcessManager(cfg *utils.Config) *ProcessManager {
	// 确保工作目录存在
	workspace := GetWorkspacePath()
	if err := os.MkdirAll(workspace, 0755); err != nil {
		slog.Error("Failed to create workspace directory", "error", err)
	}

	// 确保日志目录存在
	logDir := filepath.Join(workspace, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		slog.Error("Failed to create log directory", "error", err)
	}

	// 状态文件路径
	stateFile := filepath.Join(workspace, utils.PMStateFile)

	// 配置文件路径
	configFile := filepath.Join(workspace, utils.PMConfigFile)

	// 保存配置到文件
	if err := config.SaveConfig(configFile, cfg); err != nil {
		slog.Error("Failed to save config file", "error", err)
	}

	// 更新日志路径
	cfg.Log.Path = logDir

	// PID 文件路径
	pidFile := filepath.Join(workspace, utils.PMPidFile)

	// Socket 路径
	socketPath := GetSocketPath()

	pm := &ProcessManager{
		config:     cfg,
		processes:  make(map[string]*Process),
		logManager: logger.NewLogManager(cfg.Log),
		stateFile:  stateFile,
		running:    false,
		stopChan:   make(chan struct{}),
		pidFile:    pidFile,
		socketPath: socketPath,
		startTime:  time.Now(),
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
		// 确保日志目录存在
		if err := os.MkdirAll(pm.config.Log.Path, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}
		logPath = filepath.Join(pm.config.Log.Path, name+"-output.log")
	}

	// 创建进程配置
	procConfig := &utils.ProcessConfig{
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
		if process.status == utils.ProcessStatusRunning {
			// 检查进程是否存在
			if process.pid > 0 {
				processObj, err := os.FindProcess(process.pid)
				if err != nil {
					process.status = utils.ProcessStatusStopped
				} else {
					// 向进程发送信号 0 来检查进程是否存在
					if err := processObj.Signal(syscall.Signal(0)); err != nil {
						process.status = utils.ProcessStatusStopped
					}
				}
			} else {
				process.status = utils.ProcessStatusStopped
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

// IsRunning 检查守护进程是否正在运行
func (pm *ProcessManager) IsRunning() bool {
	// 检查 PID 文件是否存在
	if _, err := os.Stat(pm.pidFile); os.IsNotExist(err) {
		return false
	}

	// 读取 PID 文件
	data, err := os.ReadFile(pm.pidFile)
	if err != nil {
		return false
	}

	// 解析 PID
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return false
	}

	// 检查进程是否存在
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// 发送信号 0 来检查进程是否存在
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return false
	}

	return true
}

// StartDaemon 启动守护进程
func (pm *ProcessManager) StartDaemon() error {
	// 检查是否已经在运行
	if pm.IsRunning() {
		return fmt.Errorf("pm daemon is already running")
	}

	// 检查是否存在 pm.save 文件
	workspace := GetWorkspacePath()

	saveFile := filepath.Join(workspace, utils.PMSaveFile)
	if _, err := os.Stat(saveFile); err == nil {
		// 存在 pm.save 文件，加载它
		slog.Info("Loading processes from pm.save file", "file", saveFile)
		data, err := os.ReadFile(saveFile)
		if err == nil {
			var saveData StateFile
			if err := json.Unmarshal(data, &saveData); err == nil {
				// 加载保存的进程
				for name, processState := range saveData.Processes {
					// 创建进程配置
					procConfig := &utils.ProcessConfig{
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
					if err == nil {
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
						slog.Info("Loaded process from pm.save", "process", processState.Name)

						// 如果进程之前是运行状态，启动它
						if processState.Status == utils.ProcessStatusRunning {
							if err := process.Start(); err != nil {
								slog.Error("Failed to start process from save data", "error", err, "process", processState.Name)
							}
						}
					}
				}
			}
		}
	}

	// 写入 PID 文件
	pid := os.Getpid()
	if err := os.WriteFile(pm.pidFile, []byte(strconv.Itoa(pid)), 0644); err != nil {
		return fmt.Errorf("failed to write pid file: %w", err)
	}

	// 清理旧的 Unix socket 文件
	if err := os.Remove(pm.socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove old socket file: %w", err)
	}

	// 创建 Unix socket 监听器
	listener, err := net.Listen("unix", pm.socketPath)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	defer listener.Close()

	// 设置运行状态
	pm.running = true
	slog.Info("pm daemon started", "pid", pid, "socket", pm.socketPath)

	// 启动 Unix socket 服务器
	go pm.runSocketServer(listener)

	// 监控循环
	for {
		select {
		case <-pm.stopChan:
			slog.Info("pm daemon stopping")
			pm.running = false
			// 删除 PID 文件
			os.Remove(pm.pidFile)
			// 删除 Unix socket 文件
			os.Remove(pm.socketPath)
			// 删除状态文件
			os.Remove(pm.stateFile)
			return nil
		default:
			// 检查进程状态
			pm.checkProcesses()
			// 休眠 1 秒
			time.Sleep(1 * time.Second)
		}
	}
}

// runSocketServer 运行 Unix socket 服务器
func (pm *ProcessManager) runSocketServer(listener net.Listener) {
	for pm.running {
		// 接受连接
		conn, err := listener.Accept()
		if err != nil {
			if !pm.running {
				break
			}
			slog.Error("Failed to accept connection", "error", err)
			continue
		}

		// 处理连接
		go pm.handleConnection(conn)
	}
}

// handleConnection 处理客户端连接
func (pm *ProcessManager) handleConnection(conn net.Conn) {
	defer conn.Close()

	// 读取命令
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		slog.Error("Failed to read command", "error", err)
		return
	}

	// 反序列化命令
	var cmd Command
	if err := json.Unmarshal(buf[:n], &cmd); err != nil {
		slog.Error("Failed to unmarshal command", "error", err)
		pm.sendResponse(conn, false, "Invalid command format", nil)
		return
	}

	// 处理命令
	pm.handleCommand(conn, cmd)
}

// handleCommand 处理客户端命令
func (pm *ProcessManager) handleCommand(conn net.Conn, cmd Command) {
	switch cmd.Action {
	case "start":
		pm.handleStartCommand(conn, cmd.Args)
	case "list":
		pm.handleListCommand(conn)
	case "env":
		pm.handleEnvCommand(conn, cmd.Args)
	case "log":
		pm.handleLogCommand(conn, cmd.Args)
	case "logs":
		pm.handleLogsCommand(conn)
	case "stop":
		pm.handleStopCommand(conn, cmd.Args)
	case "restart":
		pm.handleRestartCommand(conn, cmd.Args)
	case "delete":
		pm.handleDeleteCommand(conn, cmd.Args)
	case "status":
		pm.handleStatusCommand(conn, cmd.Args)
	case "reload":
		pm.handleReloadCommand(conn)
	case "stop-daemon":
		pm.handleStopDaemonCommand(conn)
	case "daemon-status":
		pm.handleDaemonStatusCommand(conn)
	case "save":
		pm.handleSaveCommand(conn)
	case "resurrect":
		pm.handleResurrectCommand(conn)
	default:
		pm.sendResponse(conn, false, "Unknown command", nil)
	}
}

// sendResponse 发送响应给客户端
func (pm *ProcessManager) sendResponse(conn net.Conn, success bool, message string, data any) {
	resp := Response{
		Success: success,
		Message: message,
		Data:    data,
	}

	// 序列化响应
	respJSON, err := json.Marshal(resp)
	if err != nil {
		slog.Error("Failed to marshal response", "error", err)
		return
	}

	// 发送响应
	if _, err := conn.Write(respJSON); err != nil {
		slog.Error("Failed to write response", "error", err)
	}
}

// handleStartCommand 处理 start 命令
func (pm *ProcessManager) handleStartCommand(conn net.Conn, argsJSON json.RawMessage) {
	// 反序列化参数
	var args map[string]any
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		pm.sendResponse(conn, false, "Invalid arguments", nil)
		return
	}

	// 提取参数
	script, ok := args["script"].(string)
	if !ok {
		pm.sendResponse(conn, false, "Missing or invalid script", nil)
		return
	}

	// 提取名称，如果没有指定，使用脚本文件名
	name, ok := args["name"].(string)
	if !ok || name == "" {
		// 使用脚本文件名作为默认名称
		name = filepath.Base(script)
	}

	// 提取可选参数
	var argsSlice []string
	if args["args"] != nil {
		if argsList, ok := args["args"].([]any); ok {
			for _, arg := range argsList {
				if argStr, ok := arg.(string); ok {
					argsSlice = append(argsSlice, argStr)
				}
			}
		}
	}

	envFile := ""
	if args["env"] != nil {
		if envStr, ok := args["env"].(string); ok {
			envFile = envStr
		}
	}

	logPath := ""
	if args["log"] != nil {
		if logStr, ok := args["log"].(string); ok {
			logPath = logStr
		}
	}

	cwd := ""
	if args["cwd"] != nil {
		if cwdStr, ok := args["cwd"].(string); ok {
			cwd = cwdStr
		}
	}

	// 这里直接调用底层方法，不需要模拟 CLI 上下文

	// 检查进程是否已存在
	if _, ok := pm.processes[name]; ok {
		pm.sendResponse(conn, false, fmt.Sprintf("Process %s already exists", name), nil)
		return
	}

	// 读取环境变量
	env := make(map[string]string)
	if envFile != "" {
		if err := loadEnvFile(envFile, env); err != nil {
			pm.sendResponse(conn, false, fmt.Sprintf("Failed to load env file: %v", err), nil)
			return
		}
	}

	// 设置工作目录
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	// 设置日志路径
	if logPath == "" {
		// 确保日志目录存在
		if err := os.MkdirAll(pm.config.Log.Path, 0755); err != nil {
			pm.sendResponse(conn, false, fmt.Sprintf("Failed to create log directory: %v", err), nil)
			return
		}
		logPath = filepath.Join(pm.config.Log.Path, name+"-output.log")
	}

	// 提取最大重启次数和重启延迟
	maxRestarts := pm.config.MaxRestarts
	if args["max_restarts"] != nil {
		if mr, ok := args["max_restarts"].(float64); ok {
			maxRestarts = int(mr)
		}
	}

	restartDelay := 5
	if args["restart_delay"] != nil {
		if rd, ok := args["restart_delay"].(float64); ok {
			restartDelay = int(rd)
		}
	}

	// 创建进程配置
	procConfig := &utils.ProcessConfig{
		Name:         name,
		Script:       script,
		Args:         argsSlice,
		Env:          env,
		LogPath:      logPath,
		Cwd:          cwd,
		MaxRestarts:  maxRestarts,
		RestartDelay: restartDelay,
	}

	// 创建进程
	process, err := NewProcess(procConfig, pm.logManager)
	if err != nil {
		pm.sendResponse(conn, false, fmt.Sprintf("Failed to create process: %v", err), nil)
		return
	}

	// 启动进程
	if err := process.Start(); err != nil {
		pm.sendResponse(conn, false, fmt.Sprintf("Failed to start process: %v", err), nil)
		return
	}

	// 添加到进程列表
	pm.processes[name] = process
	process.SetManager(pm)

	// 保存状态
	if err := pm.saveState(); err != nil {
		slog.Error("Failed to save state", "error", err)
	}

	pm.sendResponse(conn, true, fmt.Sprintf("Process %s started successfully", name), nil)
}

// handleListCommand 处理 list 命令
func (pm *ProcessManager) handleListCommand(conn net.Conn) {
	// 构建表格
	var output strings.Builder

	// 打印表格顶部边框
	output.WriteString("+-----+--------------------+----------+----------+----------+-----------------+----------+----------+\n")
	// 打印表头
	output.WriteString(fmt.Sprintf("| %-3s | %-18s | %-8s | %-8s | %-8s | %-15s | %-8s | %-8s |\n", "ID", "Name", "Status", "PID", "CPU", "Memory", "Uptime", "Restarts"))
	// 打印表头分隔线
	output.WriteString("+-----+--------------------+----------+----------+----------+-----------------+----------+----------+\n")

	// 将进程转换为切片，以便使用索引
	var processes []*Process
	for _, process := range pm.processes {
		processes = append(processes, process)
	}

	// 遍历进程切片，使用索引作为 ID
	for i, process := range processes {
		// 检查进程是否还在运行
		if process.status == utils.ProcessStatusRunning {
			// 检查进程是否存在
			if process.pid > 0 {
				processObj, err := os.FindProcess(process.pid)
				if err != nil {
					process.status = utils.ProcessStatusStopped
				} else {
					// 向进程发送信号 0 来检查进程是否存在
					if err := processObj.Signal(syscall.Signal(0)); err != nil {
						process.status = utils.ProcessStatusStopped
					}
				}
			} else {
				process.status = utils.ProcessStatusStopped
			}
		}

		status := process.GetStatus()
		// 使用索引+1作为 ID
		id := i + 1
		// 打印进程信息
		output.WriteString(fmt.Sprintf("| %-3d | %-18s | %-8s | %-8d | %-8.2f | %-15s | %-8s | %-8d |\n",
			id,
			status.Name,
			status.Status,
			status.PID,
			status.CPU,
			formatMemory(status.Memory),
			formatUptime(status.Uptime),
			status.Restarts,
		))
	}

	// 打印表格底部边框
	output.WriteString("+-----+--------------------+----------+----------+----------+-----------------+----------+----------+\n")

	// 保存状态
	pm.saveState()

	pm.sendResponse(conn, true, output.String(), nil)
}

// handleEnvCommand 处理 env 命令
func (pm *ProcessManager) handleEnvCommand(conn net.Conn, argsJSON json.RawMessage) {
	// 反序列化参数
	var args map[string]string
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		pm.sendResponse(conn, false, "Invalid arguments", nil)
		return
	}

	nameOrID := args["nameOrID"]
	if nameOrID == "" {
		pm.sendResponse(conn, false, "Process name or ID is required", nil)
		return
	}

	process, err := pm.GetProcessByNameOrID(nameOrID)
	if err != nil {
		pm.sendResponse(conn, false, err.Error(), nil)
		return
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Environment variables for process %s:\n", process.config.Name))
	if process.env != nil {
		for _, envVar := range process.env {
			output.WriteString(fmt.Sprintf("%s\n", envVar))
		}
	} else {
		// 如果没有记录的环境变量，显示配置中的环境变量
		for key, value := range process.config.Env {
			output.WriteString(fmt.Sprintf("%s=%s\n", key, value))
		}
	}

	pm.sendResponse(conn, true, output.String(), nil)
}

// handleLogCommand 处理 log 命令
func (pm *ProcessManager) handleLogCommand(conn net.Conn, argsJSON json.RawMessage) {
	// 反序列化参数
	var args map[string]string
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		pm.sendResponse(conn, false, "Invalid arguments", nil)
		return
	}

	nameOrID := args["nameOrID"]
	if nameOrID == "" {
		pm.sendResponse(conn, false, "Process name or ID is required", nil)
		return
	}

	process, err := pm.GetProcessByNameOrID(nameOrID)
	if err != nil {
		pm.sendResponse(conn, false, err.Error(), nil)
		return
	}

	// 打开日志文件
	file, err := os.Open(process.config.LogPath)
	if err != nil {
		pm.sendResponse(conn, true, fmt.Sprintf("Log file not found or cannot be read: %v\n", err), nil)
		return
	}
	defer file.Close()

	// 定位到文件末尾
	fileInfo, err := file.Stat()
	if err != nil {
		pm.sendResponse(conn, true, fmt.Sprintf("Failed to stat log file: %v\n", err), nil)
		return
	}
	file.Seek(fileInfo.Size(), 0)

	// 创建文件监听器
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		pm.sendResponse(conn, true, fmt.Sprintf("Failed to create file watcher: %v\n", err), nil)
		return
	}
	defer watcher.Close()

	// 监听日志文件所在目录
	logDir := filepath.Dir(process.config.LogPath)
	if err := watcher.Add(logDir); err != nil {
		pm.sendResponse(conn, true, fmt.Sprintf("Failed to add watch: %v\n", err), nil)
		return
	}

	// 读取初始日志（如果有）
	buf := make([]byte, 4096)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		fmt.Fprintf(conn, "Error reading log file: %v\n", err)
	}
	if n > 0 {
		if _, err := conn.Write(buf[:n]); err != nil {
			// 客户端断开连接，停止发送
			return
		}
	}

	// 持续监听文件变化
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			// 只处理写入事件
			if event.Name == process.config.LogPath && (event.Op&fsnotify.Write == fsnotify.Write) {
				// 读取新的日志数据
				buf := make([]byte, 4096)
				n, err := file.Read(buf)
				if err != nil && err != io.EOF {
					fmt.Fprintf(conn, "Error reading log file: %v\n", err)
				}
				if n > 0 {
					if _, err := conn.Write(buf[:n]); err != nil {
						// 客户端断开连接，停止发送
						return
					}
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			fmt.Fprintf(conn, "Error watching log file: %v\n", err)
		}
	}
}

// handleLogsCommand 处理 logs 命令
func (pm *ProcessManager) handleLogsCommand(conn net.Conn) {
	// 打开所有进程的日志文件
	files := make(map[string]*os.File)
	logPaths := make(map[string]string)
	for name, process := range pm.processes {
		file, err := os.Open(process.config.LogPath)
		if err != nil {
			fmt.Fprintf(conn, "=== Logs for process %s ===\n", name)
			fmt.Fprintf(conn, "Log file not found or cannot be read: %v\n\n", err)
			continue
		}
		files[name] = file
		logPaths[name] = process.config.LogPath
		// 定位到文件末尾
		fileInfo, err := file.Stat()
		if err == nil {
			file.Seek(fileInfo.Size(), 0)
		}
	}
	defer func() {
		for _, file := range files {
			file.Close()
		}
	}()

	// 创建文件监听器
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(conn, "Failed to create file watcher: %v\n", err)
		return
	}
	defer watcher.Close()

	// 监听所有日志文件所在目录
	dirs := make(map[string]bool)
	for _, logPath := range logPaths {
		logDir := filepath.Dir(logPath)
		if !dirs[logDir] {
			if err := watcher.Add(logDir); err != nil {
				fmt.Fprintf(conn, "Failed to add watch for directory %s: %v\n", logDir, err)
			}
			dirs[logDir] = true
		}
	}

	// 持续监听文件变化
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			// 检查是否有进程的日志文件发生变化
			for name, logPath := range logPaths {
				if event.Name == logPath && (event.Op&fsnotify.Write == fsnotify.Write) {
					file := files[name]
					if file == nil {
						continue
					}
					// 读取新的日志数据
					buf := make([]byte, 4096)
					n, err := file.Read(buf)
					if err != nil && err != io.EOF {
						fmt.Fprintf(conn, "Error reading log file for process %s: %v\n", name, err)
						continue
					}
					if n > 0 {
						// 发送新的日志数据
						if _, err := conn.Write(buf[:n]); err != nil {
							// 客户端断开连接，停止发送
							return
						}
						fmt.Fprintf(conn, "\n")
					}
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			fmt.Fprintf(conn, "Error watching log files: %v\n", err)
		}
	}
}

// handleStopCommand 处理 stop 命令
func (pm *ProcessManager) handleStopCommand(conn net.Conn, argsJSON json.RawMessage) {
	// 反序列化参数
	var args map[string]string
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		pm.sendResponse(conn, false, "Invalid arguments", nil)
		return
	}

	nameOrID := args["nameOrID"]
	if nameOrID == "" {
		pm.sendResponse(conn, false, "Process name or ID is required", nil)
		return
	}

	process, err := pm.GetProcessByNameOrID(nameOrID)
	if err != nil {
		pm.sendResponse(conn, false, err.Error(), nil)
		return
	}

	if err := process.Stop(); err != nil {
		pm.sendResponse(conn, false, fmt.Sprintf("Failed to stop process: %v", err), nil)
		return
	}

	// 保存状态
	pm.saveState()

	pm.sendResponse(conn, true, fmt.Sprintf("Process %s stopped successfully", process.config.Name), nil)
}

// handleRestartCommand 处理 restart 命令
func (pm *ProcessManager) handleRestartCommand(conn net.Conn, argsJSON json.RawMessage) {
	// 反序列化参数
	var args map[string]string
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		pm.sendResponse(conn, false, "Invalid arguments", nil)
		return
	}

	nameOrID := args["nameOrID"]
	if nameOrID == "" {
		pm.sendResponse(conn, false, "Process name or ID is required", nil)
		return
	}

	process, err := pm.GetProcessByNameOrID(nameOrID)
	if err != nil {
		pm.sendResponse(conn, false, err.Error(), nil)
		return
	}

	if err := process.Restart(); err != nil {
		pm.sendResponse(conn, false, fmt.Sprintf("Failed to restart process: %v", err), nil)
		return
	}

	// 保存状态
	pm.saveState()

	pm.sendResponse(conn, true, fmt.Sprintf("Process %s restarted successfully", process.config.Name), nil)
}

// handleDeleteCommand 处理 delete 命令
func (pm *ProcessManager) handleDeleteCommand(conn net.Conn, argsJSON json.RawMessage) {
	// 反序列化参数
	var args map[string]string
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		pm.sendResponse(conn, false, "Invalid arguments", nil)
		return
	}

	nameOrID := args["nameOrID"]
	if nameOrID == "" {
		pm.sendResponse(conn, false, "Process name or ID is required", nil)
		return
	}

	process, err := pm.GetProcessByNameOrID(nameOrID)
	if err != nil {
		pm.sendResponse(conn, false, err.Error(), nil)
		return
	}

	if err := process.Stop(); err != nil {
		pm.sendResponse(conn, false, fmt.Sprintf("Failed to stop process: %v", err), nil)
		return
	}

	delete(pm.processes, process.config.Name)

	// 保存状态
	pm.saveState()

	pm.sendResponse(conn, true, fmt.Sprintf("Process %s deleted successfully", process.config.Name), nil)
}

// handleStatusCommand 处理 status 命令
func (pm *ProcessManager) handleStatusCommand(conn net.Conn, argsJSON json.RawMessage) {
	// 反序列化参数
	var args map[string]string
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		pm.sendResponse(conn, false, "Invalid arguments", nil)
		return
	}

	nameOrID := args["nameOrID"]
	if nameOrID == "" {
		pm.sendResponse(conn, false, "Process name or ID is required", nil)
		return
	}

	process, err := pm.GetProcessByNameOrID(nameOrID)
	if err != nil {
		pm.sendResponse(conn, false, err.Error(), nil)
		return
	}

	status := process.GetStatus()
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Process %s status:\n", process.config.Name))
	output.WriteString(fmt.Sprintf("ID: %s\n", status.ID))
	output.WriteString(fmt.Sprintf("Name: %s\n", status.Name))
	output.WriteString(fmt.Sprintf("Status: %s\n", status.Status))
	output.WriteString(fmt.Sprintf("PID: %d\n", status.PID))
	output.WriteString(fmt.Sprintf("CPU: %.2f%%\n", status.CPU))
	output.WriteString(fmt.Sprintf("Memory: %d(%s)\n", status.Memory, formatMemory(status.Memory)))
	output.WriteString(fmt.Sprintf("Uptime: %s\n", formatUptime(status.Uptime)))
	output.WriteString(fmt.Sprintf("Restarts: %d\n", status.Restarts))
	output.WriteString(fmt.Sprintf("Created At: %s\n", status.CreatedAt))
	output.WriteString(fmt.Sprintf("Started At: %s\n", status.StartedAt))
	output.WriteString(fmt.Sprintf("Log Path: %s\n", status.LogPath))

	pm.sendResponse(conn, true, output.String(), nil)
}

// handleReloadCommand 处理 reload 命令
func (pm *ProcessManager) handleReloadCommand(conn net.Conn) {
	wsp := GetWorkspacePath()
	err := config.LoadConfig(filepath.Join(wsp, utils.PMConfigFile), pm.config)
	if err != nil {
		pm.sendResponse(conn, false, fmt.Sprintf("Failed to load config: %v", err), nil)
		return
	}
	pm.logManager.UpdateConfig(pm.config.Log)

	// 保存状态
	pm.saveState()

	pm.sendResponse(conn, true, "Configuration reloaded successfully", nil)
}

// handleStopDaemonCommand 处理 stop-daemon 命令
func (pm *ProcessManager) handleStopDaemonCommand(conn net.Conn) {
	// 终止所有管理的进程
	for name, process := range pm.processes {
		slog.Info("Stopping process before daemon shutdown", "process", name)
		if err := process.Stop(); err != nil {
			slog.Error("Failed to stop process", "process", name, "error", err)
		} else {
			slog.Info("Process stopped successfully", "process", name)
		}
	}

	// 发送停止信号
	pm.stopChan <- struct{}{}

	pm.sendResponse(conn, true, "pm daemon stopped", nil)
}

// handleDaemonStatusCommand 处理 daemon-status 命令
func (pm *ProcessManager) handleDaemonStatusCommand(conn net.Conn) {
	// 获取守护进程的 PID
	pid := os.Getpid()

	// 计算守护进程的运行时间
	startTime := time.Now().Unix() - int64(time.Since(pm.startTime).Seconds())

	// 获取内存和 CPU 占用信息
	var cpuPercent float64
	var memoryBytes uint64

	// 尝试获取进程信息
	if procInfo, err := getProcessInfo(pid); err == nil {
		cpuPercent = procInfo.CPU
		memoryBytes = procInfo.Memory
	}

	// 构建状态信息
	var output strings.Builder
	output.WriteString("pm daemon status:\n")
	output.WriteString(fmt.Sprintf("PID: %d\n", pid))
	output.WriteString("Status: running\n")
	output.WriteString(fmt.Sprintf("CPU: %.2f%%\n", cpuPercent))
	output.WriteString(fmt.Sprintf("Memory: %d(%s)\n", memoryBytes, formatMemory(memoryBytes)))
	output.WriteString(fmt.Sprintf("Uptime: %s\n", formatUptime(time.Now().Unix()-startTime)))
	output.WriteString(fmt.Sprintf("Started At: %s\n", pm.startTime.Format(time.RFC3339)))
	output.WriteString(fmt.Sprintf("Managed Processes: %d\n", len(pm.processes)))

	pm.sendResponse(conn, true, output.String(), nil)
}

// handleSaveCommand 处理 save 命令
func (pm *ProcessManager) handleSaveCommand(conn net.Conn) {
	// 获取工作目录
	workspace := GetWorkspacePath()

	// 确保工作目录存在
	if err := os.MkdirAll(workspace, 0755); err != nil {
		pm.sendResponse(conn, false, fmt.Sprintf("Failed to create workspace directory: %v", err), nil)
		return
	}

	// 保存文件路径
	saveFile := filepath.Join(workspace, utils.PMSaveFile)

	// 构建保存数据
	saveData := StateFile{
		Processes: make(map[string]ProcessState),
	}

	// 保存每个进程的状态
	for name, process := range pm.processes {
		saveData.Processes[name] = ProcessState{
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

	// 序列化保存数据
	data, err := json.Marshal(saveData)
	if err != nil {
		pm.sendResponse(conn, false, fmt.Sprintf("Failed to marshal save data: %v", err), nil)
		return
	}

	// 写入保存文件
	if err := os.WriteFile(saveFile, data, 0644); err != nil {
		pm.sendResponse(conn, false, fmt.Sprintf("Failed to write save file: %v", err), nil)
		return
	}

	pm.sendResponse(conn, true, fmt.Sprintf("Successfully saved %d processes to %s", len(pm.processes), saveFile), nil)
}

// handleResurrectCommand 处理 resurrect 命令
func (pm *ProcessManager) handleResurrectCommand(conn net.Conn) {
	// 获取工作目录
	workspace := GetWorkspacePath()

	// 保存文件路径
	saveFile := filepath.Join(workspace, utils.PMSaveFile)

	// 读取保存文件
	data, err := os.ReadFile(saveFile)
	if err != nil {
		pm.sendResponse(conn, false, fmt.Sprintf("Failed to read save file: %v", err), nil)
		return
	}

	// 反序列化保存数据
	var saveData StateFile
	if err := json.Unmarshal(data, &saveData); err != nil {
		pm.sendResponse(conn, false, fmt.Sprintf("Failed to unmarshal save data: %v", err), nil)
		return
	}

	// 重启每个进程
	restarted := 0
	for name, processState := range saveData.Processes {
		// 检查进程是否已经存在
		if _, ok := pm.processes[name]; ok {
			// 进程已经存在，跳过
			continue
		}

		// 创建进程配置
		// 确保日志路径在 PM_WORKSPACE/logs 下
		logPath := processState.LogPath
		if !strings.HasPrefix(logPath, pm.config.Log.Path) {
			// 生成新的日志路径
			logPath = filepath.Join(pm.config.Log.Path, processState.Name+"-output.log")
		}

		procConfig := &utils.ProcessConfig{
			Name:         processState.Name,
			Script:       processState.Script,
			Args:         processState.Args,
			Env:          processState.Env,
			LogPath:      logPath,
			Cwd:          processState.Cwd,
			MaxRestarts:  processState.MaxRestarts,
			RestartDelay: processState.RestartDelay,
		}

		// 创建进程
		process, err := NewProcess(procConfig, pm.logManager)
		if err != nil {
			slog.Error("Failed to create process from save data", "error", err, "process", processState.Name)
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

		// 如果进程之前是运行状态，启动它
		if processState.Status == utils.ProcessStatusRunning {
			if err := process.Start(); err != nil {
				slog.Error("Failed to start process from save data", "error", err, "process", processState.Name)
			} else {
				restarted++
				slog.Info("Process restarted from save data", "process", processState.Name)
			}
		}
	}

	// 保存状态
	pm.saveState()

	pm.sendResponse(conn, true, fmt.Sprintf("Successfully resurrected %d processes", restarted), nil)
}

// StopDaemon 停止守护进程
func (pm *ProcessManager) StopDaemon() error {
	// 检查是否在运行
	if !pm.IsRunning() {
		return fmt.Errorf("pm daemon is not running")
	}

	// 读取 PID 文件
	data, err := os.ReadFile(pm.pidFile)
	if err != nil {
		return fmt.Errorf("failed to read pid file: %w", err)
	}

	// 解析 PID
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return fmt.Errorf("invalid pid in pid file: %w", err)
	}

	// 发送 SIGTERM 信号
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send signal: %w", err)
	}

	// 等待进程退出
	for i := 0; i < 10; i++ {
		if !pm.IsRunning() {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if pm.IsRunning() {
		return fmt.Errorf("failed to stop daemon")
	}

	// 清理残留文件
	os.Remove(pm.pidFile)
	os.Remove(pm.socketPath)
	os.Remove(pm.stateFile)

	fmt.Println("pm daemon stopped")
	return nil
}

// checkProcesses 检查所有进程的状态
func (pm *ProcessManager) checkProcesses() {
	for name, process := range pm.processes {
		if process.status == utils.ProcessStatusRunning {
			// 检查进程是否存在
			if process.pid > 0 {
				processObj, err := os.FindProcess(process.pid)
				if err != nil {
					// 进程不存在，标记为 stopped 并尝试重启
					process.status = utils.ProcessStatusStopped
					slog.Info("Process not found, marking as stopped", "process", name)

					// 尝试重启进程
					maxRestarts := process.config.MaxRestarts
					if maxRestarts == 0 {
						maxRestarts = pm.config.MaxRestarts
					}
					if process.restarts < maxRestarts {
						slog.Info("Auto-restarting process", "process", name, "restarts", process.restarts, "max_restarts", maxRestarts)
						if err := process.Restart(); err != nil {
							slog.Error("Failed to restart process", "process", name, "error", err)
						} else {
							slog.Info("Process restarted successfully", "process", name, "restarts", process.restarts)
						}
					} else {
						slog.Info("Max restarts reached, stopping", "process", name, "max_restarts", maxRestarts)
					}
				} else {
					// 向进程发送信号 0 来检查进程是否存在
					if err := processObj.Signal(syscall.Signal(0)); err != nil {
						// 进程不存在，标记为 stopped 并尝试重启
						process.status = utils.ProcessStatusStopped
						slog.Info("Process not responding, marking as stopped", "process", name)

						// 尝试重启进程
						maxRestarts := process.config.MaxRestarts
						if maxRestarts == 0 {
							maxRestarts = pm.config.MaxRestarts
						}
						if process.restarts < maxRestarts {
							slog.Info("Auto-restarting process", "process", name, "restarts", process.restarts, "max_restarts", maxRestarts)
							if err := process.Restart(); err != nil {
								slog.Error("Failed to restart process", "process", name, "error", err)
							} else {
								slog.Info("Process restarted successfully", "process", name, "restarts", process.restarts)
							}
						} else {
							slog.Info("Max restarts reached, stopping", "process", name, "max_restarts", maxRestarts)
						}
					}
				}
			} else {
				// 进程没有 PID，标记为 stopped 并尝试重启
				process.status = utils.ProcessStatusStopped
				slog.Info("Process has no PID, marking as stopped", "process", name)

				// 尝试重启进程
				maxRestarts := process.config.MaxRestarts
				if maxRestarts == 0 {
					maxRestarts = pm.config.MaxRestarts
				}
				if process.restarts < maxRestarts {
					slog.Info("Auto-restarting process", "process", name, "restarts", process.restarts, "max_restarts", maxRestarts)
					if err := process.Restart(); err != nil {
						slog.Error("Failed to restart process", "process", name, "error", err)
					} else {
						slog.Info("Process restarted successfully", "process", name, "restarts", process.restarts)
					}
				} else {
					slog.Info("Max restarts reached, stopping", "process", name, "max_restarts", maxRestarts)
				}
			}
		} else if process.status == utils.ProcessStatusStopped {
			// 检查是否需要重启已停止的进程
			maxRestarts := process.config.MaxRestarts
			if maxRestarts == 0 {
				maxRestarts = pm.config.MaxRestarts
			}
			if process.restarts < maxRestarts {
				slog.Info("Auto-restarting stopped process", "process", name, "restarts", process.restarts, "max_restarts", maxRestarts)
				if err := process.Restart(); err != nil {
					slog.Error("Failed to restart process", "process", name, "error", err)
				} else {
					slog.Info("Process restarted successfully", "process", name, "restarts", process.restarts)
				}
			} else {
				slog.Info("Max restarts reached, stopping", "process", name, "max_restarts", maxRestarts)
			}
		}
	}

	// 保存状态
	pm.saveState()
}

// formatMemory 将字节转换为更友好的单位
func formatMemory(bytes uint64) string {

	var unit string
	var size float64

	switch {
	case bytes >= uint64(utils.TB):
		size = float64(bytes) / utils.TB
		unit = "TB"
	case bytes >= uint64(utils.GB):
		size = float64(bytes) / utils.GB
		unit = "GB"
	case bytes >= uint64(utils.MB):
		size = float64(bytes) / utils.MB
		unit = "MB"
	case bytes >= uint64(utils.KB):
		size = float64(bytes) / utils.KB
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

	days := seconds / utils.Day
	seconds %= utils.Day
	hours := seconds / utils.Hour
	seconds %= utils.Hour
	minutes := seconds / utils.Minute
	seconds %= utils.Minute

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

// getProcessInfo 获取进程的 CPU 和内存使用情况
type processInfo struct {
	CPU    float64
	Memory uint64
}

func getProcessInfo(pid int) (processInfo, error) {
	// 使用 gopsutil 获取进程信息
	p, err := process.NewProcess(int32(pid))
	if err != nil {
		return processInfo{}, err
	}

	// 获取 CPU 使用率
	cpuPercent, err := p.CPUPercent()
	if err != nil {
		cpuPercent = 0
	}

	// 获取内存使用情况
	memInfo, err := p.MemoryInfo()
	if err != nil {
		return processInfo{}, err
	}

	return processInfo{
		CPU:    cpuPercent,
		Memory: memInfo.RSS,
	}, nil
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
		procConfig := &utils.ProcessConfig{
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
