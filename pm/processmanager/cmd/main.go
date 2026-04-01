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
	"processmanager/internal/utils"

	"github.com/takama/daemon"
	"github.com/urfave/cli/v2"
)

// GetDaemonKind 获取守护进程类型
func GetDaemonKind(c *cli.Context) daemon.Kind {
	// 优先使用命令行参数
	kindStr := os.Getenv(utils.PMENV_DAEMON_KIND)
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
	default: // 默认 UserAgent
		return daemon.UserAgent
	}
}

func GetDaemonName(c *cli.Context) string {
	// 优先使用命令行参数
	name := os.Getenv(utils.PMENV_DAEMON_NAME)
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
	socketPath := utils.GetSocketPath()
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
func sendCommand(action string, args any) (*utils.Response, error) {
	// 连接到 Unix socket
	socketPath := utils.GetSocketPath()
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
	cmd := utils.Command{
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
		return &utils.Response{Success: true, Message: ""}, nil
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
	var resp utils.Response
	if err := json.Unmarshal(buf, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &resp, nil
}

func main() {
	// 获取工作目录
	workspace := utils.GetWorkspacePath()

	// 首先尝试从工作目录加载配置文件
	configFile := filepath.Join(workspace, utils.PMConfigFile)
	var cfg = &utils.Config{}
	err := config.LoadConfig(configFile, cfg)
	if err != nil {
		// 如果工作目录中没有配置文件，使用默认配置
		fmt.Printf("Config file not found, using default config: %v\n", err)
		// 设置默认值
		if cfg.Log.Path == "" {
			cfg.Log.Path = filepath.Join(workspace, utils.PMLogDir)
		}
		if cfg.Log.MaxSize == 0 {
			cfg.Log.MaxSize = 100 // 100MB
		}
		if cfg.Log.MaxFiles == 0 {
			cfg.Log.MaxFiles = 10
		}
		if cfg.StateFile == "" {
			cfg.StateFile = filepath.Join(workspace, utils.PMStateFile)
		}
		if cfg.MaxRestarts == 0 {
			cfg.MaxRestarts = 255 // 默认最大重启次数为 255
		}
	}

	// 初始化日志
	logger.InitLogger(cfg.Log)

	// 创建命令行应用
	app := &cli.App{
		Name:  utils.ProcessManagerName,
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
				Name:    "version",
				Usage:   "Show version information",
				Aliases: []string{"v"},
				Action: func(c *cli.Context) error {
					fmt.Printf("%s %s\nGo: %s\n", utils.ProcessManagerName, utils.Version, utils.GoVersion)
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
				Name:  "config",
				Usage: "Manage configuration",
				Subcommands: []*cli.Command{
					{
						Name:  "show",
						Usage: "Show current configuration",
						Action: func(c *cli.Context) error {
							// 加载配置文件
							cfgPath := filepath.Join(workspace, utils.PMConfigFile)
							var cfg utils.Config
							if err := config.LoadConfig(cfgPath, &cfg); err != nil {
								return fmt.Errorf("failed to load config: %w", err)
							}

							// 打印配置文件路径
							fmt.Printf("Config file: %s\n\n", cfgPath)

							// 格式化输出 JSON
							data, err := json.MarshalIndent(cfg, "", "  ")
							if err != nil {
								return fmt.Errorf("failed to marshal config: %w", err)
							}
							fmt.Println(string(data))
							return nil
						},
					},
					{
						Name:  "log",
						Usage: "Configure log settings",
						Flags: []cli.Flag{
							&cli.IntFlag{
								Name:  "size",
								Usage: "Max size of each log file in MB",
							},
							&cli.IntFlag{
								Name:  "files",
								Usage: "Max number of log files to keep",
							},
							&cli.BoolFlag{
								Name:  "compress",
								Usage: "Enable compression for old log files",
							},
						},
						Action: func(c *cli.Context) error {
							cfgPath := filepath.Join(workspace, utils.PMConfigFile)
							var cfg utils.Config
							if err := config.LoadConfig(cfgPath, &cfg); err != nil {
								return fmt.Errorf("failed to load config: %w", err)
							}

							updated := false
							if c.IsSet("size") {
								cfg.Log.MaxSize = c.Int("size")
								updated = true
							}
							if c.IsSet("files") {
								cfg.Log.MaxFiles = c.Int("files")
								updated = true
							}
							if c.IsSet("compress") {
								cfg.Log.Compress = c.Bool("compress")
								updated = true
							}

							if !updated {
								fmt.Printf("Current log config:\n")
								fmt.Printf("  max_size: %d MB\n", cfg.Log.MaxSize)
								fmt.Printf("  max_files: %d\n", cfg.Log.MaxFiles)
								fmt.Printf("  compress: %v\n", cfg.Log.Compress)
								return nil
							}

							if err := config.SaveConfig(cfgPath, &cfg); err != nil {
								return fmt.Errorf("failed to save config: %w", err)
							}

							fmt.Printf("Log config updated:\n")
							fmt.Printf("  max_size: %d MB\n", cfg.Log.MaxSize)
							fmt.Printf("  max_files: %d\n", cfg.Log.MaxFiles)
							fmt.Printf("  compress: %v\n", cfg.Log.Compress)
							return nil
						},
					},
					{
						Name:  "channel",
						Usage: "Manage notification channels",
						Subcommands: []*cli.Command{
							{
								Name:  "add",
								Usage: "Add a notification channel",
								Flags: []cli.Flag{
									&cli.StringFlag{
										Name:  "name",
										Usage: "Channel name",
									},
									&cli.StringFlag{
										Name:  "type",
										Usage: "Channel type: wecombot or mail",
									},
									&cli.StringFlag{
										Name:  "key",
										Usage: "Webhook key (for wecombot)",
									},
									&cli.StringFlag{
										Name:  "to",
										Usage: "Recipient email (for mail)",
									},
									&cli.StringFlag{
										Name:  "from",
										Usage: "Sender email (for mail)",
									},
									&cli.StringFlag{
										Name:  "smtp",
										Usage: "SMTP server (user:passwd@host:port)",
									},
								},
								Action: func(c *cli.Context) error {
									name := c.String("name")
									if name == "" {
										return fmt.Errorf("channel name is required")
									}
									chType := c.String("type")
									if chType == "" {
										return fmt.Errorf("channel type is required")
									}

									cfgPath := filepath.Join(workspace, utils.PMConfigFile)
									var cfg utils.Config
									if err := config.LoadConfig(cfgPath, &cfg); err != nil {
										return fmt.Errorf("failed to load config: %w", err)
									}

									if cfg.Channels == nil {
										cfg.Channels = make(map[string]utils.ChanConfig)
									}

									if _, exists := cfg.Channels[name]; exists {
										return fmt.Errorf("channel '%s' already exists", name)
									}

									ch := utils.ChanConfig{Type: chType}
									switch chType {
									case "wecombot":
										ch.Key = c.String("key")
										if ch.Key == "" {
											return fmt.Errorf("key is required for wecombot")
										}
									case "mail":
										ch.To = c.String("to")
										ch.From = c.String("from")
										ch.SMTP = c.String("smtp")
										if ch.To == "" || ch.From == "" || ch.SMTP == "" {
											return fmt.Errorf("to, from, and smtp are required for mail")
										}
									default:
										return fmt.Errorf("unknown channel type: %s", chType)
									}

									cfg.Channels[name] = ch
									if err := config.SaveConfig(cfgPath, &cfg); err != nil {
										return fmt.Errorf("failed to save config: %w", err)
									}

									fmt.Printf("Channel '%s' added successfully\n", name)
									return nil
								},
							},
							{
								Name:  "remove",
								Usage: "Remove a notification channel",
								Flags: []cli.Flag{
									&cli.StringFlag{
										Name:  "name",
										Usage: "Channel name",
									},
								},
								Action: func(c *cli.Context) error {
									name := c.String("name")
									if name == "" {
										return fmt.Errorf("channel name is required")
									}

									cfgPath := filepath.Join(workspace, utils.PMConfigFile)
									var cfg utils.Config
									if err := config.LoadConfig(cfgPath, &cfg); err != nil {
										return fmt.Errorf("failed to load config: %w", err)
									}

									if cfg.Channels == nil {
										return fmt.Errorf("no channels configured")
									}

									if _, exists := cfg.Channels[name]; !exists {
										return fmt.Errorf("channel '%s' not found", name)
									}

									delete(cfg.Channels, name)
									if err := config.SaveConfig(cfgPath, &cfg); err != nil {
										return fmt.Errorf("failed to save config: %w", err)
									}

									fmt.Printf("Channel '%s' removed successfully\n", name)
									return nil
								},
							},
							{
								Name:  "edit",
								Usage: "Edit a notification channel",
								Flags: []cli.Flag{
									&cli.StringFlag{
										Name:  "name",
										Usage: "Channel name",
									},
									&cli.StringFlag{
										Name:  "key",
										Usage: "Webhook key (for wecombot)",
									},
									&cli.StringFlag{
										Name:  "to",
										Usage: "Recipient email (for mail)",
									},
									&cli.StringFlag{
										Name:  "from",
										Usage: "Sender email (for mail)",
									},
									&cli.StringFlag{
										Name:  "smtp",
										Usage: "SMTP server (user:passwd@host:port)",
									},
								},
								Action: func(c *cli.Context) error {
									name := c.String("name")
									if name == "" {
										return fmt.Errorf("channel name is required")
									}

									cfgPath := filepath.Join(workspace, utils.PMConfigFile)
									var cfg utils.Config
									if err := config.LoadConfig(cfgPath, &cfg); err != nil {
										return fmt.Errorf("failed to load config: %w", err)
									}

									if cfg.Channels == nil {
										return fmt.Errorf("no channels configured")
									}

									ch, exists := cfg.Channels[name]
									if !exists {
										return fmt.Errorf("channel '%s' not found", name)
									}

									updated := false
									if c.IsSet("key") {
										ch.Key = c.String("key")
										updated = true
									}
									if c.IsSet("to") {
										ch.To = c.String("to")
										updated = true
									}
									if c.IsSet("from") {
										ch.From = c.String("from")
										updated = true
									}
									if c.IsSet("smtp") {
										ch.SMTP = c.String("smtp")
										updated = true
									}

									if !updated {
										return fmt.Errorf("no changes specified")
									}

									cfg.Channels[name] = ch
									if err := config.SaveConfig(cfgPath, &cfg); err != nil {
										return fmt.Errorf("failed to save config: %w", err)
									}

									fmt.Printf("Channel '%s' updated successfully\n", name)
									return nil
								},
							},
							{
								Name:  "list",
								Usage: "List all notification channels",
								Action: func(c *cli.Context) error {
									cfgPath := filepath.Join(workspace, utils.PMConfigFile)
									var cfg utils.Config
									if err := config.LoadConfig(cfgPath, &cfg); err != nil {
										return fmt.Errorf("failed to load config: %w", err)
									}

									if len(cfg.Channels) == 0 {
										fmt.Println("No channels configured")
										return nil
									}

									fmt.Println("Notification Channels:")
									fmt.Println("----------------------")
									for name, ch := range cfg.Channels {
										fmt.Printf("  Name: %s\n", name)
										fmt.Printf("  Type: %s\n", ch.Type)
										switch ch.Type {
										case "wecombot":
											fmt.Printf("  Key: %s\n", ch.Key)
										case "mail":
											fmt.Printf("  To: %s\n", ch.To)
											fmt.Printf("  From: %s\n", ch.From)
											fmt.Printf("  SMTP: %s\n", ch.SMTP)
										}
										fmt.Println()
									}
									return nil
								},
							},
						},
					},
					{
						Name:  "notice",
						Usage: "Manage notification rules",
						Subcommands: []*cli.Command{
							{
								Name:  "add",
								Usage: "Add a notification rule",
								Flags: []cli.Flag{
									&cli.StringFlag{
										Name:  "name",
										Usage: "Rule name (process name, pid, or * for all)",
									},
									&cli.StringFlag{
										Name:  "expr",
										Usage: "Expression to match log content",
									},
									&cli.StringSliceFlag{
										Name:  "channel",
										Usage: "Channel names to notify",
									},
								},
								Action: func(c *cli.Context) error {
									name := c.String("name")
									if name == "" {
										return fmt.Errorf("rule name is required")
									}

									cfgPath := filepath.Join(workspace, utils.PMConfigFile)
									var cfg utils.Config
									if err := config.LoadConfig(cfgPath, &cfg); err != nil {
										return fmt.Errorf("failed to load config: %w", err)
									}

									if cfg.Notice == nil {
										cfg.Notice = make(map[string]utils.NoticeRule)
									}

									if _, exists := cfg.Notice[name]; exists {
										return fmt.Errorf("notice rule '%s' already exists", name)
									}

									rule := utils.NoticeRule{
										Expr:    c.String("expr"),
										Channel: c.StringSlice("channel"),
									}

									cfg.Notice[name] = rule
									if err := config.SaveConfig(cfgPath, &cfg); err != nil {
										return fmt.Errorf("failed to save config: %w", err)
									}

									fmt.Printf("Notice rule '%s' added successfully\n", name)
									return nil
								},
							},
							{
								Name:  "remove",
								Usage: "Remove a notification rule",
								Flags: []cli.Flag{
									&cli.StringFlag{
										Name:  "name",
										Usage: "Rule name",
									},
								},
								Action: func(c *cli.Context) error {
									name := c.String("name")
									if name == "" {
										return fmt.Errorf("rule name is required")
									}

									cfgPath := filepath.Join(workspace, utils.PMConfigFile)
									var cfg utils.Config
									if err := config.LoadConfig(cfgPath, &cfg); err != nil {
										return fmt.Errorf("failed to load config: %w", err)
									}

									if cfg.Notice == nil {
										return fmt.Errorf("no notice rules configured")
									}

									if _, exists := cfg.Notice[name]; !exists {
										return fmt.Errorf("notice rule '%s' not found", name)
									}

									delete(cfg.Notice, name)
									if err := config.SaveConfig(cfgPath, &cfg); err != nil {
										return fmt.Errorf("failed to save config: %w", err)
									}

									fmt.Printf("Notice rule '%s' removed successfully\n", name)
									return nil
								},
							},
							{
								Name:  "edit",
								Usage: "Edit a notification rule",
								Flags: []cli.Flag{
									&cli.StringFlag{
										Name:  "name",
										Usage: "Rule name",
									},
									&cli.StringFlag{
										Name:  "expr",
										Usage: "Expression to match log content",
									},
									&cli.StringSliceFlag{
										Name:  "channel",
										Usage: "Channel names to notify",
									},
								},
								Action: func(c *cli.Context) error {
									name := c.String("name")
									if name == "" {
										return fmt.Errorf("rule name is required")
									}

									cfgPath := filepath.Join(workspace, utils.PMConfigFile)
									var cfg utils.Config
									if err := config.LoadConfig(cfgPath, &cfg); err != nil {
										return fmt.Errorf("failed to load config: %w", err)
									}

									if cfg.Notice == nil {
										return fmt.Errorf("no notice rules configured")
									}

									rule, exists := cfg.Notice[name]
									if !exists {
										return fmt.Errorf("notice rule '%s' not found", name)
									}

									updated := false
									if c.IsSet("expr") {
										rule.Expr = c.String("expr")
										updated = true
									}
									if c.IsSet("channel") {
										rule.Channel = c.StringSlice("channel")
										updated = true
									}

									if !updated {
										return fmt.Errorf("no changes specified")
									}

									cfg.Notice[name] = rule
									if err := config.SaveConfig(cfgPath, &cfg); err != nil {
										return fmt.Errorf("failed to save config: %w", err)
									}

									fmt.Printf("Notice rule '%s' updated successfully\n", name)
									return nil
								},
							},
							{
								Name:  "list",
								Usage: "List all notification rules",
								Action: func(c *cli.Context) error {
									cfgPath := filepath.Join(workspace, utils.PMConfigFile)
									var cfg utils.Config
									if err := config.LoadConfig(cfgPath, &cfg); err != nil {
										return fmt.Errorf("failed to load config: %w", err)
									}

									if len(cfg.Notice) == 0 {
										fmt.Println("No notice rules configured")
										return nil
									}

									fmt.Println("Notification Rules:")
									fmt.Println("-------------------")
									for name, rule := range cfg.Notice {
										fmt.Printf("  Name: %s\n", name)
										fmt.Printf("  Expr: %s\n", rule.Expr)
										fmt.Printf("  Channels: %v\n", rule.Channel)
										fmt.Println()
									}
									return nil
								},
							},
						},
					},
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
