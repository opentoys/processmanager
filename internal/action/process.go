package action

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"processmanager/internal/utils"

	"github.com/urfave/cli/v3"
)

// GetStartCommand 返回 start 命令
func GetStartCommand() *cli.Command {
	return &cli.Command{
		Name:      "start",
		Usage:     "Start a new process",
		ArgsUsage: "script",
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
				Name:    "args",
				Aliases: []string{"a"},
				Usage:   "Arguments to pass to the script",
			},
			&cli.StringSliceFlag{
				Name:    "env",
				Aliases: []string{"e"},
				Usage:   "Environment variable file path",
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
		Action: StartAction,
	}
}

// StartAction start 命令的 Action
func StartAction(ctx context.Context, cmd *cli.Command) error {
	if !IsDaemonRunning() {
		return errors.New(utils.ProcessManagerName + " daemon is not running")
	}

	script := cmd.String("script")
	if script == "" {
		script = cmd.Args().First()
		if script == "" {
			return errors.New("script is required")
		}
	}

	name := cmd.String("name")
	if name == "" {
		name = filepath.Base(script)
	}
	cwd := cmd.String("cwd")
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	args := map[string]any{
		"name":   name,
		"script": script,
		"args":   cmd.StringSlice("args"),
		"env":    cmd.StringSlice("env"),
		"log":    cmd.String("log"),
		"cwd":    cwd,
	}

	fmt.Println(args)

	resp, err := SendCommand("start", args)
	if err != nil {
		return err
	}

	if !resp.Success {
		return errors.New(resp.Message)
	}

	fmt.Println(resp.Message)
	return nil
}

// GetListCommand 返回 list 命令
func GetListCommand() *cli.Command {
	return &cli.Command{
		Name:    "list",
		Aliases: []string{"ls", "l"},
		Usage:   "List all managed processes",
		Action:  ListAction,
	}
}

// ListAction list 命令的 Action
func ListAction(ctx context.Context, cmd *cli.Command) error {
	if !IsDaemonRunning() {
		return errors.New(utils.ProcessManagerName + " daemon is not running")
	}

	resp, err := SendCommand("list", nil)
	if err != nil {
		return err
	}

	if !resp.Success {
		return errors.New(resp.Message)
	}

	fmt.Println(resp.Message)
	return nil
}

// GetEnvCommand 返回 env 命令
func GetEnvCommand() *cli.Command {
	return &cli.Command{
		Name:   "env",
		Usage:  "Show environment variables for a process",
		Action: EnvAction,
	}
}

// EnvAction env 命令的 Action
func EnvAction(ctx context.Context, cmd *cli.Command) error {
	if !IsDaemonRunning() {
		return errors.New(utils.ProcessManagerName + "daemon is not running")
	}

	args := map[string]string{
		"nameOrID": cmd.Args().First(),
	}

	resp, err := SendCommand("env", args)
	if err != nil {
		return err
	}

	if !resp.Success {
		return errors.New(resp.Message)
	}

	fmt.Println(resp.Message)
	return nil
}

// GetLogCommand 返回 log 命令
func GetLogCommand() *cli.Command {
	return &cli.Command{
		Name:   "log",
		Usage:  "Show logs for a process",
		Action: LogAction,
	}
}

// LogAction log 命令的 Action
func LogAction(ctx context.Context, cmd *cli.Command) error {
	if !IsDaemonRunning() {
		return errors.New(utils.ProcessManagerName + "daemon is not running")
	}

	args := map[string]string{
		"nameOrID": cmd.Args().First(),
	}

	resp, err := SendCommand("log", args)
	if err != nil {
		return err
	}

	if !resp.Success {
		return errors.New(resp.Message)
	}

	fmt.Println(resp.Message)
	return nil
}

// GetLogsCommand 返回 logs 命令
func GetLogsCommand() *cli.Command {
	return &cli.Command{
		Name:   "logs",
		Usage:  "Show logs for all processes",
		Action: LogsAction,
	}
}

// LogsAction logs 命令的 Action
func LogsAction(ctx context.Context, cmd *cli.Command) error {
	if !IsDaemonRunning() {
		return errors.New(utils.ProcessManagerName + "daemon is not running")
	}

	resp, err := SendCommand("logs", nil)
	if err != nil {
		return err
	}

	if !resp.Success {
		return errors.New(resp.Message)
	}

	fmt.Println(resp.Message)
	return nil
}

// GetStopCommand 返回 stop 命令
func GetStopCommand() *cli.Command {
	return &cli.Command{
		Name:   "stop",
		Usage:  "Stop a process",
		Action: StopAction,
	}
}

// StopAction stop 命令的 Action
func StopAction(ctx context.Context, cmd *cli.Command) error {
	if !IsDaemonRunning() {
		return errors.New(utils.ProcessManagerName + "daemon is not running")
	}

	args := map[string]string{
		"nameOrID": cmd.Args().First(),
	}

	resp, err := SendCommand("stop", args)
	if err != nil {
		return err
	}

	if !resp.Success {
		return errors.New(resp.Message)
	}

	fmt.Println(resp.Message)
	return nil
}

// GetRestartCommand 返回 restart 命令
func GetRestartCommand() *cli.Command {
	return &cli.Command{
		Name:   "restart",
		Usage:  "Restart a process",
		Action: RestartAction,
	}
}

// RestartAction restart 命令的 Action
func RestartAction(ctx context.Context, cmd *cli.Command) error {
	if !IsDaemonRunning() {
		return errors.New(utils.ProcessManagerName + "daemon is not running")
	}

	args := map[string]string{
		"nameOrID": cmd.Args().First(),
	}

	resp, err := SendCommand("restart", args)
	if err != nil {
		return err
	}

	if !resp.Success {
		return errors.New(resp.Message)
	}

	fmt.Println(resp.Message)
	return nil
}

// GetDeleteCommand 返回 delete 命令
func GetDeleteCommand() *cli.Command {
	return &cli.Command{
		Name:   "delete",
		Usage:  "Delete a process",
		Action: DeleteAction,
	}
}

// DeleteAction delete 命令的 Action
func DeleteAction(ctx context.Context, cmd *cli.Command) error {
	if !IsDaemonRunning() {
		return errors.New(utils.ProcessManagerName + "daemon is not running")
	}

	args := map[string]string{
		"nameOrID": cmd.Args().First(),
	}

	resp, err := SendCommand("delete", args)
	if err != nil {
		return err
	}

	if !resp.Success {
		return errors.New(resp.Message)
	}

	fmt.Println(resp.Message)
	return nil
}

// GetStatusCommand 返回 status 命令
func GetStatusCommand() *cli.Command {
	return &cli.Command{
		Name:    "status",
		Usage:   "Show status for a process",
		Aliases: []string{"show", "info"},
		Action:  StatusAction,
	}
}

// StatusAction status 命令的 Action
func StatusAction(ctx context.Context, cmd *cli.Command) error {
	if !IsDaemonRunning() {
		return errors.New(utils.ProcessManagerName + "daemon is not running")
	}

	args := map[string]string{
		"nameOrID": cmd.Args().First(),
	}

	resp, err := SendCommand("status", args)
	if err != nil {
		return err
	}

	if !resp.Success {
		return errors.New(resp.Message)
	}

	fmt.Println(resp.Message)
	return nil
}

// GetReloadCommand 返回 reload 命令
func GetReloadCommand() *cli.Command {
	return &cli.Command{
		Name:   "reload",
		Usage:  "Reload configuration",
		Action: ReloadAction,
	}
}

// ReloadAction reload 命令的 Action
func ReloadAction(ctx context.Context, cmd *cli.Command) error {
	if !IsDaemonRunning() {
		return errors.New(utils.ProcessManagerName + "daemon is not running")
	}

	resp, err := SendCommand("reload", nil)
	if err != nil {
		return err
	}

	if !resp.Success {
		return errors.New(resp.Message)
	}

	fmt.Println(resp.Message)
	return nil
}

// GetSaveCommand 返回 save 命令
func GetSaveCommand() *cli.Command {
	return &cli.Command{
		Name:   "save",
		Usage:  "Save current managed processes and their states",
		Action: SaveAction,
	}
}

// SaveAction save 命令的 Action
func SaveAction(ctx context.Context, cmd *cli.Command) error {
	if !IsDaemonRunning() {
		return errors.New(utils.ProcessManagerName + "daemon is not running")
	}

	resp, err := SendCommand("save", nil)
	if err != nil {
		return err
	}

	if !resp.Success {
		return errors.New(resp.Message)
	}

	fmt.Println(resp.Message)
	return nil
}

// GetResurrectCommand 返回 resurrect 命令
func GetResurrectCommand() *cli.Command {
	return &cli.Command{
		Name:   "resurrect",
		Usage:  "Restart services from save file",
		Action: ResurrectAction,
	}
}

// ResurrectAction resurrect 命令的 Action
func ResurrectAction(ctx context.Context, cmd *cli.Command) error {
	if !IsDaemonRunning() {
		return errors.New(utils.ProcessManagerName + "daemon is not running")
	}

	resp, err := SendCommand("resurrect", nil)
	if err != nil {
		return err
	}

	if !resp.Success {
		return errors.New(resp.Message)
	}

	fmt.Println(resp.Message)
	return nil
}

// GetVersionCommand 返回 version 命令
func GetVersionCommand() *cli.Command {
	return &cli.Command{
		Name:    "version",
		Usage:   "Show version information",
		Aliases: []string{"v"},
		Action:  VersionAction,
	}
}

// VersionAction version 命令的 Action
func VersionAction(ctx context.Context, cmd *cli.Command) error {
	fmt.Printf("%s %s\nGo: %s\n", utils.ProcessManagerName, utils.Version, utils.GoVersion)
	return nil
}

// GetDaemonCommand 返回 daemon 命令
func GetDaemonCommand() *cli.Command {
	return &cli.Command{
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
		Commands: GetDaemonCommands(),
	}
}

// GetConfigCommand 返回 config 命令
func GetConfigCommand() *cli.Command {
	return &cli.Command{
		Name:     "config",
		Usage:    "Manage configuration",
		Commands: GetConfigCommands(),
	}
}

// GetProcessCommands 返回所有进程管理相关命令
func GetProcessCommands() []*cli.Command {
	return []*cli.Command{
		GetStartCommand(),
		GetStopCommand(),
		GetRestartCommand(),
		GetDeleteCommand(),
		GetListCommand(),
		GetStatusCommand(),
		GetEnvCommand(),
		GetLogCommand(),
		GetLogsCommand(),
		GetSaveCommand(),
		GetResurrectCommand(),
		GetPluginCommand(),
		GetCrontabCommand(),
		GetConfigCommand(),
		GetDaemonCommand(),
		GetVersionCommand(),
	}
}
