package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"processmanager/internal/config"
	"processmanager/internal/logger"
	"processmanager/internal/manager"

	"github.com/takama/daemon"
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

// GetWorkspacePath 获取工作目录路径
func GetWorkspacePath() string {
	// 检查 PM_WORKSPACE 环境变量
	if workspace := os.Getenv("PM_WORKSPACE"); workspace != "" {
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
	return filepath.Join(GetWorkspacePath(), "pm.sock")
}

// GetDaemonKind 获取守护进程类型
func GetDaemonKind(c *cli.Context) daemon.Kind {
	// 优先使用命令行参数
	kindStr := os.Getenv("PM_DAEMON_KIND")
	// 如果命令行参数未设置，尝试从环境变量获取
	if k := c.String("kind"); k != "" {
		kindStr = k
	}

	// 转换为 daemon.Kind
	switch kindStr {
	case "GlobalAgent":
		return daemon.GlobalAgent
	case "GlobalDaemon":
		return daemon.GlobalDaemon
	case "SystemDaemon":
		return daemon.SystemDaemon
	default:
		return daemon.UserAgent
	}
}

func GetDaemonName(c *cli.Context) string {
	// 优先使用命令行参数
	name := os.Getenv("PM_DAEMON_NAME")
	// 如果命令行参数未设置，尝试从环境变量获取
	if k := c.String("name"); k != "" {
		name = k
	}
	if name == "" {
		name = "pm"
	}
	return name
}

func GetDaemonService(c *cli.Context) (daemon.Daemon, error) {
	return daemon.New(GetDaemonName(c), "Process manager daemon", GetDaemonKind(c))
}

// isDaemonRunning 检查守护进程是否正在运行
func isDaemonRunning() bool {
	// 检查 Unix socket 是否存在
	socketPath := GetSocketPath()
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		return false
	}

	// 尝试连接到 Unix socket
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return false
	}
	conn.Close()

	return true
}

// sendCommand 发送命令到守护进程
func sendCommand(action string, args any) (*Response, error) {
	// 连接到 Unix socket
	socketPath := GetSocketPath()
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to daemon: %w", err)
	}

	// 序列化参数
	argsJSON, err := json.Marshal(args)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to marshal args: %w", err)
	}

	// 创建命令
	cmd := Command{
		Action: action,
		Args:   argsJSON,
	}

	// 序列化命令
	cmdJSON, err := json.Marshal(cmd)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}

	// 发送命令
	if _, err := conn.Write(cmdJSON); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to write command: %w", err)
	}

	// 对于 log 和 logs 命令，实时接收日志
	if action == "log" || action == "logs" {
		readBuf := make([]byte, 4096)
		for {
			n, err := conn.Read(readBuf)
			if err != nil {
				break
			}
			fmt.Print(string(readBuf[:n]))
		}
		conn.Close()
		return &Response{Success: true, Message: ""}, nil
	}

	// 对于其他命令，读取完整响应
	var buf []byte
	readBuf := make([]byte, 4096)
	for {
		n, err := conn.Read(readBuf)
		if err != nil {
			break
		}
		buf = append(buf, readBuf[:n]...)
	}
	conn.Close()

	// 反序列化响应
	var resp Response
	if err := json.Unmarshal(buf, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &resp, nil
}

func main() {
	// 获取工作目录
	workspace := GetWorkspacePath()

	// 首先尝试从工作目录加载配置文件
	configFile := filepath.Join(workspace, "config.json")
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		// 如果工作目录中没有配置文件，使用默认配置
		fmt.Printf("Config file not found, using default config: %v\n", err)
		cfg = &config.Config{}
		// 设置默认值
		if cfg.Log.Path == "" {
			cfg.Log.Path = filepath.Join(workspace, "logs")
		}
		if cfg.Log.MaxSize == 0 {
			cfg.Log.MaxSize = 100 // 100MB
		}
		if cfg.Log.MaxFiles == 0 {
			cfg.Log.MaxFiles = 10
		}
		if cfg.StateFile == "" {
			cfg.StateFile = filepath.Join(workspace, "pm.state")
		}
		if cfg.MaxRestarts == 0 {
			cfg.MaxRestarts = 255 // 默认最大重启次数为 255
		}
	}

	// 初始化日志
	logger.InitLogger(cfg.Log)

	// 创建命令行应用
	app := &cli.App{
		Name:  "pm",
		Usage: "Process manager",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "Enable debug logging",
			},
		},
		Before: func(c *cli.Context) error {
			// 根据 debug 标志设置日志级别
			if c.Bool("debug") {
				logger.SetDebug(true)
			}
			return nil
		},
		Commands: []*cli.Command{
			{
				Name:  "start",
				Usage: "Start a new process",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "name",
						Usage: "Process name",
					},
					&cli.StringFlag{
						Name:  "script",
						Usage: "Script or command to execute",
					},
					&cli.StringSliceFlag{
						Name:  "args",
						Usage: "Arguments to pass to the script",
					},
					&cli.StringFlag{
						Name:  "env",
						Usage: "Environment variable file path",
					},
					&cli.StringFlag{
						Name:  "log",
						Usage: "Log file path",
					},
					&cli.StringFlag{
						Name:  "cwd",
						Usage: "Working directory",
					},
				},
				Action: func(c *cli.Context) error {
					// 检查守护进程是否正在运行
					if !isDaemonRunning() {
						return fmt.Errorf("pm daemon is not running")
					}

					// 获取脚本路径
					script := c.String("script")
					if script == "" {
						// 如果没有指定脚本，使用第一个位置参数
						script = c.Args().First()
						if script == "" {
							return fmt.Errorf("script is required")
						}
					}

					// 获取进程名称
					name := c.String("name")
					if name == "" {
						// 如果没有指定名称，使用脚本文件名
						name = filepath.Base(script)
					}

					// 构建命令参数
					args := map[string]any{
						"name":   name,
						"script": script,
						"args":   c.StringSlice("args"),
						"env":    c.String("env"),
						"log":    c.String("log"),
						"cwd":    c.String("cwd"),
					}

					// 发送命令
					resp, err := sendCommand("start", args)
					if err != nil {
						return err
					}

					if !resp.Success {
						return errors.New(resp.Message)
					}

					fmt.Println(resp.Message)
					return nil
				},
			},
			{
				Name:    "list",
				Aliases: []string{"ls", "l"},
				Usage:   "List all managed processes",
				Action: func(c *cli.Context) error {
					// 检查守护进程是否正在运行
					if !isDaemonRunning() {
						return fmt.Errorf("pm daemon is not running")
					}

					// 发送命令
					resp, err := sendCommand("list", nil)
					if err != nil {
						return err
					}

					if !resp.Success {
						return errors.New(resp.Message)
					}

					// 打印进程列表
					fmt.Println(resp.Message)
					return nil
				},
			},
			{
				Name:  "env",
				Usage: "Show environment variables for a process",
				Action: func(c *cli.Context) error {
					// 检查守护进程是否正在运行
					if !isDaemonRunning() {
						return errors.New("pm daemon is not running")
					}

					// 构建命令参数
					args := map[string]string{
						"nameOrID": c.Args().First(),
					}

					// 发送命令
					resp, err := sendCommand("env", args)
					if err != nil {
						return err
					}

					if !resp.Success {
						return errors.New(resp.Message)
					}

					fmt.Println(resp.Message)
					return nil
				},
			},
			{
				Name:  "log",
				Usage: "Show logs for a process",
				Action: func(c *cli.Context) error {
					// 检查守护进程是否正在运行
					if !isDaemonRunning() {
						return fmt.Errorf("pm daemon is not running")
					}

					// 构建命令参数
					args := map[string]string{
						"nameOrID": c.Args().First(),
					}

					// 发送命令
					resp, err := sendCommand("log", args)
					if err != nil {
						return err
					}

					if !resp.Success {
						return errors.New(resp.Message)
					}

					fmt.Println(resp.Message)
					return nil
				},
			},
			{
				Name:  "logs",
				Usage: "Show logs for all processes",
				Action: func(c *cli.Context) error {
					// 检查守护进程是否正在运行
					if !isDaemonRunning() {
						return fmt.Errorf("pm daemon is not running")
					}

					// 发送命令
					resp, err := sendCommand("logs", nil)
					if err != nil {
						return err
					}

					if !resp.Success {
						return errors.New(resp.Message)
					}

					fmt.Println(resp.Message)
					return nil
				},
			},
			{
				Name:  "stop",
				Usage: "Stop a process",
				Action: func(c *cli.Context) error {
					// 检查守护进程是否正在运行
					if !isDaemonRunning() {
						return fmt.Errorf("pm daemon is not running")
					}

					// 构建命令参数
					args := map[string]string{
						"nameOrID": c.Args().First(),
					}

					// 发送命令
					resp, err := sendCommand("stop", args)
					if err != nil {
						return err
					}

					if !resp.Success {
						return errors.New(resp.Message)
					}

					fmt.Println(resp.Message)
					return nil
				},
			},
			{
				Name:  "restart",
				Usage: "Restart a process",
				Action: func(c *cli.Context) error {
					// 检查守护进程是否正在运行
					if !isDaemonRunning() {
						return fmt.Errorf("pm daemon is not running")
					}

					// 构建命令参数
					args := map[string]string{
						"nameOrID": c.Args().First(),
					}

					// 发送命令
					resp, err := sendCommand("restart", args)
					if err != nil {
						return err
					}

					if !resp.Success {
						return errors.New(resp.Message)
					}

					fmt.Println(resp.Message)
					return nil
				},
			},
			{
				Name:  "delete",
				Usage: "Delete a process",
				Action: func(c *cli.Context) error {
					// 检查守护进程是否正在运行
					if !isDaemonRunning() {
						return fmt.Errorf("pm daemon is not running")
					}

					// 构建命令参数
					args := map[string]string{
						"nameOrID": c.Args().First(),
					}

					// 发送命令
					resp, err := sendCommand("delete", args)
					if err != nil {
						return err
					}

					if !resp.Success {
						return errors.New(resp.Message)
					}

					fmt.Println(resp.Message)
					return nil
				},
			},
			{
				Name:  "status",
				Usage: "Show status for a process",
				Action: func(c *cli.Context) error {
					// 检查守护进程是否正在运行
					if !isDaemonRunning() {
						return fmt.Errorf("pm daemon is not running")
					}

					// 构建命令参数
					args := map[string]string{
						"nameOrID": c.Args().First(),
					}

					// 发送命令
					resp, err := sendCommand("status", args)
					if err != nil {
						return err
					}

					if !resp.Success {
						return errors.New(resp.Message)
					}

					fmt.Println(resp.Message)
					return nil
				},
			},
			{
				Name:  "reload",
				Usage: "Reload configuration",
				Action: func(c *cli.Context) error {
					// 检查守护进程是否正在运行
					if !isDaemonRunning() {
						return fmt.Errorf("pm daemon is not running")
					}

					// 发送命令
					resp, err := sendCommand("reload", nil)
					if err != nil {
						return err
					}

					if !resp.Success {
						return errors.New(resp.Message)
					}

					fmt.Println(resp.Message)
					return nil
				},
			},
			{
				Name:  "daemon",
				Usage: "Manage pm daemon",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "kind",
						Usage: "Daemon kind: UserAgent, GlobalAgent, GlobalDaemon, SystemDaemon.  eq PM_DAEMON_KIND (default: UserAgent)",
					},
					&cli.StringFlag{
						Name:  "name",
						Usage: "Daemon name. eq PM_DAEMON_NAME (default: pm)",
					},
				},
				Subcommands: []*cli.Command{
					{
						Name:  "start",
						Usage: "Start pm system service",
						Action: func(c *cli.Context) error {
							// 获取守护进程类型
							// 定义守护进程
							service, err := GetDaemonService(c)
							if err != nil {
								return fmt.Errorf("failed to create daemon: %w", err)
							}

							// 启动服务
							status, err := service.Start()
							if err != nil {
								return fmt.Errorf("failed to start daemon: %w", err)
							}

							fmt.Printf("pm daemon started: %v\n", status)
							return nil
						},
					},
					{
						Name:  "stop",
						Usage: "Stop pm system service",
						Action: func(c *cli.Context) error {
							// 获取守护进程类型
							// 定义守护进程
							service, err := GetDaemonService(c)
							if err != nil {
								return fmt.Errorf("failed to create daemon: %w", err)
							}

							// 停止服务
							status, err := service.Stop()
							if err != nil {
								return fmt.Errorf("failed to stop daemon: %w", err)
							}

							fmt.Printf("pm daemon stopped: %v\n", status)
							return nil
						},
					},
					{
						Name:  "status",
						Usage: "Show pm daemon status",
						Action: func(c *cli.Context) error {
							// 获取守护进程类型
							// 定义守护进程
							service, err := GetDaemonService(c)
							if err != nil {
								return fmt.Errorf("failed to create daemon: %w", err)
							}

							// 获取系统服务状态
							systemStatus, err := service.Status()
							if err != nil {
								return fmt.Errorf("failed to get system service status: %w", err)
							}

							// 输出系统服务状态
							fmt.Printf("System service status: %v\n", systemStatus)

							// 检查守护进程是否正在运行
							if !isDaemonRunning() {
								fmt.Println("pm daemon is not running")
								return nil
							}

							// 发送命令
							resp, err := sendCommand("daemon-status", nil)
							if err != nil {
								return err
							}

							if !resp.Success {
								return errors.New(resp.Message)
							}

							fmt.Println(resp.Message)
							return nil
						},
					},
					{
						Name:  "install",
						Usage: "Install pm as a system service",
						Action: func(c *cli.Context) error {
							// 获取守护进程类型
							// 定义守护进程
							service, err := GetDaemonService(c)
							if err != nil {
								return fmt.Errorf("failed to create daemon: %w", err)
							}

							// 安装服务
							status, err := service.Install("daemon-run")
							if err != nil {
								return fmt.Errorf("failed to install daemon: %w", err)
							}

							fmt.Printf("pm daemon installed: %v\n", status)
							return nil
						},
					},
					{
						Name:  "remove",
						Usage: "Remove pm system service",
						Action: func(c *cli.Context) error {
							// 获取守护进程类型
							// 定义守护进程
							service, err := GetDaemonService(c)
							if err != nil {
								return fmt.Errorf("failed to create daemon: %w", err)
							}

							// 卸载服务
							status, err := service.Remove()
							if err != nil {
								return fmt.Errorf("failed to remove daemon: %w", err)
							}

							fmt.Printf("pm daemon removed: %v\n", status)
							return nil
						},
					},
				},
			},
			{
				Name:  "save",
				Usage: "Save current managed processes and their states",
				Action: func(c *cli.Context) error {
					// 检查守护进程是否正在运行
					if !isDaemonRunning() {
						return fmt.Errorf("pm daemon is not running")
					}

					// 发送命令
					resp, err := sendCommand("save", nil)
					if err != nil {
						return err
					}

					if !resp.Success {
						return errors.New(resp.Message)
					}

					fmt.Println(resp.Message)
					return nil
				},
			},
			{
				Name:  "resurrect",
				Usage: "Restart services from save file",
				Action: func(c *cli.Context) error {
					// 检查守护进程是否正在运行
					if !isDaemonRunning() {
						return fmt.Errorf("pm daemon is not running")
					}

					// 发送命令
					resp, err := sendCommand("resurrect", nil)
					if err != nil {
						return err
					}

					if !resp.Success {
						return errors.New(resp.Message)
					}

					fmt.Println(resp.Message)
					return nil
				},
			},
			{
				Name:   "daemon-run",
				Hidden: true,
				Action: func(c *cli.Context) error {
					// 启动守护进程
					pm := manager.NewProcessManager(cfg)
					return pm.StartDaemon()
				},
			},
		},
	}

	// 运行命令行应用
	if err := app.Run(os.Args); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
