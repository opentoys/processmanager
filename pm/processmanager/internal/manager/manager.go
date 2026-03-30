package manager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"processmanager/internal/config"
	"processmanager/internal/logger"

	"github.com/urfave/cli/v2"
)

// ProcessManager 进程管理器
type ProcessManager struct {
	config     *config.Config
	processes  map[string]*Process
	logManager *logger.LogManager
}

// NewProcessManager 创建进程管理器
func NewProcessManager(cfg *config.Config) *ProcessManager {
	return &ProcessManager{
		config:     cfg,
		processes:  make(map[string]*Process),
		logManager: logger.NewLogManager(cfg.Log),
	}
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

	// 保存配置
	pm.config.Processes = append(pm.config.Processes, *procConfig)
	if err := config.SaveConfig("config.yaml", pm.config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Process %s started successfully\n", name)
	return nil
}

// ListProcesses 列出所有进程
func (pm *ProcessManager) ListProcesses(c *cli.Context) error {
	fmt.Println("ID	Name	Status	PID	CPU	Memory	Uptime")
	fmt.Println("---	----	------	---	---	------	------")

	for _, process := range pm.processes {
		status := process.GetStatus()
		fmt.Printf("%s	%s	%s	%d	%.2f	%d	%d\n",
			status.ID,
			status.Name,
			status.Status,
			status.PID,
			status.CPU,
			status.Memory,
			status.Uptime,
		)
	}

	return nil
}

// ShowEnv 显示进程环境变量
func (pm *ProcessManager) ShowEnv(c *cli.Context) error {
	name := c.Args().First()
	if name == "" {
		return fmt.Errorf("process name is required")
	}

	process, ok := pm.processes[name]
	if !ok {
		return fmt.Errorf("process %s not found", name)
	}

	fmt.Printf("Environment variables for process %s:\n", name)
	for key, value := range process.config.Env {
		fmt.Printf("%s=%s\n", key, value)
	}

	return nil
}

// ShowLog 显示进程日志
func (pm *ProcessManager) ShowLog(c *cli.Context) error {
	name := c.Args().First()
	if name == "" {
		return fmt.Errorf("process name is required")
	}

	process, ok := pm.processes[name]
	if !ok {
		return fmt.Errorf("process %s not found", name)
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
	name := c.Args().First()
	if name == "" {
		return fmt.Errorf("process name is required")
	}

	process, ok := pm.processes[name]
	if !ok {
		return fmt.Errorf("process %s not found", name)
	}

	if err := process.Stop(); err != nil {
		return fmt.Errorf("failed to stop process: %w", err)
	}

	fmt.Printf("Process %s stopped successfully\n", name)
	return nil
}

// RestartProcess 重启进程
func (pm *ProcessManager) RestartProcess(c *cli.Context) error {
	name := c.Args().First()
	if name == "" {
		return fmt.Errorf("process name is required")
	}

	process, ok := pm.processes[name]
	if !ok {
		return fmt.Errorf("process %s not found", name)
	}

	if err := process.Restart(); err != nil {
		return fmt.Errorf("failed to restart process: %w", err)
	}

	fmt.Printf("Process %s restarted successfully\n", name)
	return nil
}

// DeleteProcess 删除进程
func (pm *ProcessManager) DeleteProcess(c *cli.Context) error {
	name := c.Args().First()
	if name == "" {
		return fmt.Errorf("process name is required")
	}

	process, ok := pm.processes[name]
	if !ok {
		return fmt.Errorf("process %s not found", name)
	}

	if err := process.Stop(); err != nil {
		return fmt.Errorf("failed to stop process: %w", err)
	}

	delete(pm.processes, name)

	// 更新配置
	var newProcesses []config.ProcessConfig
	for _, p := range pm.config.Processes {
		if p.Name != name {
			newProcesses = append(newProcesses, p)
		}
	}
	pm.config.Processes = newProcesses

	if err := config.SaveConfig("config.yaml", pm.config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Process %s deleted successfully\n", name)
	return nil
}

// ShowStatus 显示进程状态
func (pm *ProcessManager) ShowStatus(c *cli.Context) error {
	name := c.Args().First()
	if name == "" {
		return fmt.Errorf("process name is required")
	}

	process, ok := pm.processes[name]
	if !ok {
		return fmt.Errorf("process %s not found", name)
	}

	status := process.GetStatus()
	fmt.Printf("Process %s status:\n", name)
	fmt.Printf("ID: %s\n", status.ID)
	fmt.Printf("Name: %s\n", status.Name)
	fmt.Printf("Status: %s\n", status.Status)
	fmt.Printf("PID: %d\n", status.PID)
	fmt.Printf("CPU: %.2f%%\n", status.CPU)
	fmt.Printf("Memory: %d bytes\n", status.Memory)
	fmt.Printf("Uptime: %d seconds\n", status.Uptime)
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

	// 重启进程
	for _, procConfig := range newConfig.Processes {
		if process, ok := pm.processes[procConfig.Name]; ok {
			if err := process.Restart(); err != nil {
				fmt.Printf("Failed to restart process %s: %v\n", procConfig.Name, err)
			}
		} else {
			// 启动新进程
			process, err := NewProcess(&procConfig, pm.logManager)
			if err != nil {
				fmt.Printf("Failed to create process %s: %v\n", procConfig.Name, err)
				continue
			}

			if err := process.Start(); err != nil {
				fmt.Printf("Failed to start process %s: %v\n", procConfig.Name, err)
				continue
			}

			pm.processes[procConfig.Name] = process
		}
	}

	// 删除不存在的进程
	for name, process := range pm.processes {
		found := false
		for _, procConfig := range newConfig.Processes {
			if procConfig.Name == name {
				found = true
				break
			}
		}

		if !found {
			if err := process.Stop(); err != nil {
				fmt.Printf("Failed to stop process %s: %v\n", name, err)
			}
			delete(pm.processes, name)
		}
	}

	fmt.Println("Configuration reloaded successfully")
	return nil
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
