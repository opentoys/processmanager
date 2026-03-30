package main

import (
	"fmt"
	"os"

	"processmanager/internal/config"
	"processmanager/internal/logger"
	"processmanager/internal/manager"

	"github.com/urfave/cli/v2"
)

func main() {
	// 加载配置
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		fmt.Printf("Failed to load config: %v", err)
		os.Exit(1)
	}

	// 初始化日志
	logger.InitLogger(cfg.Log)

	// 初始化进程管理器
	pm := manager.NewProcessManager(cfg)

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
						Name:     "name",
						Usage:    "Process name",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "script",
						Usage:    "Script or command to execute",
						Required: true,
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
					return pm.StartProcess(c)
				},
			},
			{
				Name:  "list",
				Usage: "List all managed processes",
				Action: func(c *cli.Context) error {
					return pm.ListProcesses(c)
				},
			},
			{
				Name:  "env",
				Usage: "Show environment variables for a process",
				Action: func(c *cli.Context) error {
					return pm.ShowEnv(c)
				},
			},
			{
				Name:  "log",
				Usage: "Show logs for a process",
				Action: func(c *cli.Context) error {
					return pm.ShowLog(c)
				},
			},
			{
				Name:  "logs",
				Usage: "Show logs for all processes",
				Action: func(c *cli.Context) error {
					return pm.ShowAllLogs(c)
				},
			},
			{
				Name:  "stop",
				Usage: "Stop a process",
				Action: func(c *cli.Context) error {
					return pm.StopProcess(c)
				},
			},
			{
				Name:  "restart",
				Usage: "Restart a process",
				Action: func(c *cli.Context) error {
					return pm.RestartProcess(c)
				},
			},
			{
				Name:  "delete",
				Usage: "Delete a process",
				Action: func(c *cli.Context) error {
					return pm.DeleteProcess(c)
				},
			},
			{
				Name:  "status",
				Usage: "Show status for a process",
				Action: func(c *cli.Context) error {
					return pm.ShowStatus(c)
				},
			},
			{
				Name:  "reload",
				Usage: "Reload configuration",
				Action: func(c *cli.Context) error {
					return pm.ReloadConfig(c)
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
