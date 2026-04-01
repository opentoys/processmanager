package main

import (
	"fmt"
	"os"
	"path/filepath"

	"processmanager/internal/action"
	"processmanager/internal/config"
	"processmanager/internal/logger"
	"processmanager/internal/manager"
	"processmanager/internal/utils"

	"github.com/urfave/cli/v2"
)

func main() {
	// 获取工作目录
	action.Workspace = utils.GetWorkspacePath()

	// 首先尝试从工作目录加载配置文件
	configFile := filepath.Join(action.Workspace, utils.PMConfigFile)
	var cfg = &utils.Config{}
	err := config.LoadConfig(configFile, cfg)
	if err != nil {
		fmt.Printf("Config file not found, using default config: %v\n", err)
		if cfg.Log.Path == "" {
			cfg.Log.Path = filepath.Join(action.Workspace, utils.PMLogDir)
		}
		if cfg.Log.MaxSize == 0 {
			cfg.Log.MaxSize = 100
		}
		if cfg.Log.MaxFiles == 0 {
			cfg.Log.MaxFiles = 10
		}
		if cfg.StateFile == "" {
			cfg.StateFile = filepath.Join(action.Workspace, utils.PMStateFile)
		}
		if cfg.MaxRestarts == 0 {
			cfg.MaxRestarts = 255
		}
	}

	logger.InitLogger(cfg.Log)

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
			if c.Bool("debug") {
				logger.SetDebug(true)
			}
			return nil
		},
		Commands: append(
			action.GetProcessCommands(),
			&cli.Command{
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
				Subcommands: action.GetDaemonCommands(),
			},
			&cli.Command{
				Name:        "config",
				Usage:       "Manage configuration",
				Subcommands: action.GetConfigCommands(),
			},
			&cli.Command{
				Name:   "daemon-run",
				Hidden: true,
				Action: func(c *cli.Context) error {
					pm := manager.NewProcessManager(cfg)
					return pm.StartDaemon()
				},
			},
		),
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
