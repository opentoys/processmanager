package action

import (
	"context"
	"errors"
	"fmt"
	"os"

	"processmanager/internal/utils"

	"github.com/urfave/cli/v3"
)

// GetCrontabCommand cron 命令
func GetCrontabCommand() *cli.Command {
	return &cli.Command{
		Name:    "crontab",
		Aliases: []string{"cron"},
		Usage:   "Manage cron jobs",
		Commands: []*cli.Command{
			{
				Name:      "set",
				Aliases:   []string{"add"},
				Usage:     "Add a cron job",
				ArgsUsage: "script",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Aliases:  []string{"n"},
						Usage:    "Cron job name",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "spec",
						Aliases:  []string{"s"},
						Usage:    "Cron spec (with seconds): e.g. \"*/5 * * * * *\"",
						Required: true,
					},
					&cli.StringSliceFlag{
						Name:    "args",
						Aliases: []string{"a"},
						Usage:   "Script arguments",
					},
					&cli.StringSliceFlag{
						Name:    "env",
						Aliases: []string{"e"},
						Usage:   "Environment variables (KEY=VALUE)",
					},
					&cli.StringFlag{
						Name:  "cwd",
						Usage: "Working directory",
					},
				},
				Action: CronSetAction,
			},
			{
				Name:    "list",
				Aliases: []string{"ls"},
				Usage:   "List all cron jobs",
				Action:  CronListAction,
			},
			{
				Name:    "remove",
				Aliases: []string{"rm"},
				Usage:   "Remove a cron job",
				Action:  CronRemoveAction,
			},
			{
				Name:   "log",
				Usage:  "Show log for a cron job",
				Action: CronLogAction,
			},
			{
				Name:    "logs",
				Usage:   "Show logs for all cron jobs",
				Aliases: []string{"lg"},
				Action:  CronLogsAction,
			},
		},
	}
}

// CronSetAction 添加定时任务
func CronSetAction(ctx context.Context, cmd *cli.Command) error {
	if !IsDaemonRunning() {
		return errors.New(utils.ProcessManagerName + " daemon is not running")
	}

	name := cmd.String("name")
	spec := cmd.String("spec")
	script := cmd.Args().First()

	if script == "" {
		return errors.New("script is required")
	}

	cwd := cmd.String("cwd")
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	args := map[string]any{
		"name":   name,
		"spec":   spec,
		"script": script,
		"args":   cmd.StringSlice("args"),
		"env":    cmd.StringSlice("env"),
		"cwd":    cwd,
	}

	resp, err := SendCommand("cron-set", args)
	if err != nil {
		return err
	}

	if !resp.Success {
		return errors.New(resp.Message)
	}

	fmt.Println(resp.Message)
	return nil
}

// CronListAction 列出所有定时任务
func CronListAction(ctx context.Context, cmd *cli.Command) error {
	if !IsDaemonRunning() {
		return errors.New(utils.ProcessManagerName + " daemon is not running")
	}

	resp, err := SendCommand("cron-list", nil)
	if err != nil {
		return err
	}

	if !resp.Success {
		return errors.New(resp.Message)
	}

	fmt.Println(resp.Message)
	return nil
}

// CronRemoveAction 删除定时任务
func CronRemoveAction(ctx context.Context, cmd *cli.Command) error {
	if !IsDaemonRunning() {
		return errors.New(utils.ProcessManagerName + " daemon is not running")
	}

	nameOrID := cmd.Args().First()
	if nameOrID == "" {
		return errors.New("cron job name or ID is required")
	}

	args := map[string]any{
		"nameOrID": nameOrID,
	}

	resp, err := SendCommand("cron-remove", args)
	if err != nil {
		return err
	}

	if !resp.Success {
		return errors.New(resp.Message)
	}

	fmt.Println(resp.Message)
	return nil
}

// CronLogAction 查看定时任务日志（实时监听）
func CronLogAction(ctx context.Context, cmd *cli.Command) error {
	if !IsDaemonRunning() {
		return errors.New(utils.ProcessManagerName + " daemon is not running")
	}

	nameOrID := cmd.Args().First()
	if nameOrID == "" {
		return errors.New("cron job name or ID is required")
	}

	args := map[string]any{
		"nameOrID": nameOrID,
	}

	resp, err := SendCommand("cron-log", args)
	if err != nil {
		return err
	}

	if !resp.Success {
		return errors.New(resp.Message)
	}

	return nil
}

// CronLogsAction 查看所有定时任务日志（实时监听）
func CronLogsAction(ctx context.Context, cmd *cli.Command) error {
	if !IsDaemonRunning() {
		return errors.New(utils.ProcessManagerName + " daemon is not running")
	}

	resp, err := SendCommand("cron-logs", nil)
	if err != nil {
		return err
	}

	if !resp.Success {
		return errors.New(resp.Message)
	}

	return nil
}
