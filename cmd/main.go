package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"processmanager/internal/action"
	"processmanager/internal/config"
	"processmanager/internal/logger"
	"processmanager/internal/manager"
	"processmanager/internal/utils"

	"github.com/urfave/cli/v3"
)

func main() {
	utils.ProcessManagerName = filepath.Base(os.Args[0])

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

	cmd := &cli.Command{
		Name:  utils.ProcessManagerName,
		Usage: "Process manager",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "Enable debug logging",
			},
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			if cmd.Bool("debug") {
				logger.SetDebug(true)
			}
			return ctx, nil
		},
		Commands: append(
			action.GetProcessCommands(),
			&cli.Command{
				Name:  "daemon",
				Usage: "Manage " + utils.ProcessManagerName + " daemon",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "kind",
						Usage: "Daemon kind: UserAgent, GlobalAgent, GlobalDaemon, SystemDaemon. only support MacOS. eq PM_DAEMON_KIND (default: UserAgent)",
					},
					&cli.StringFlag{
						Name:  "name",
						Usage: "Daemon name. eq PM_DAEMON_NAME (default: " + utils.ProcessManagerName + ")",
					},
				},
				Commands: action.GetDaemonCommands(),
			},
			&cli.Command{
				Name:     "config",
				Usage:    "Manage configuration",
				Commands: action.GetConfigCommands(),
			},
			&cli.Command{
				Name:   "daemon-run",
				Hidden: true,
				Action: func(ctx context.Context, cmd *cli.Command) error {
					pm := manager.NewProcessManager(cfg)
					return pm.StartDaemon()
				},
			},
		),
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
