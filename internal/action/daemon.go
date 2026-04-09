package action

import (
	"errors"
	"fmt"
	"processmanager/internal/utils"
	"runtime"

	"github.com/takama/daemon"
	"github.com/urfave/cli/v2"
)

// GetDaemonKind 获取守护进程类型
func GetDaemonKind(c *cli.Context) daemon.Kind {
	if runtime.GOOS != "darwin" {
		return daemon.SystemDaemon
	}
	switch c.String("kind") {
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

// GetDaemonName 获取守护进程名称
func GetDaemonName(c *cli.Context) string {
	return "com.github.opentoys.pm"
}

// GetDaemonService 获取守护进程服务
func GetDaemonService(c *cli.Context) (daemon.Daemon, error) {
	return daemon.New(GetDaemonName(c), "Process manager daemon", GetDaemonKind(c))
}

// DaemonStartAction daemon start 命令的 Action
func DaemonStartAction(c *cli.Context) error {
	service, err := GetDaemonService(c)
	if err != nil {
		return fmt.Errorf("failed to create daemon: %w", err)
	}

	status, err := service.Start()
	if err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	fmt.Printf(utils.ProcessManagerName+" daemon started: %v\n", status)
	return nil
}

// DaemonStopAction daemon stop 命令的 Action
func DaemonStopAction(c *cli.Context) error {
	service, err := GetDaemonService(c)
	if err != nil {
		return fmt.Errorf("failed to create daemon: %w", err)
	}

	status, err := service.Stop()
	if err != nil {
		return fmt.Errorf("failed to stop daemon: %w", err)
	}

	fmt.Printf(utils.ProcessManagerName+" daemon stopped: %v\n", status)
	return nil
}

// DaemonStatusAction daemon status 命令的 Action
func DaemonStatusAction(c *cli.Context) error {
	service, err := GetDaemonService(c)
	if err != nil {
		return fmt.Errorf("failed to create daemon: %w", err)
	}

	systemStatus, err := service.Status()
	if err != nil {
		return fmt.Errorf("failed to get system service status: %w", err)
	}

	fmt.Printf("System service status: %v\n", systemStatus)

	if !IsDaemonRunning() {
		fmt.Println(utils.ProcessManagerName + " daemon is not running")
		return nil
	}

	resp, err := SendCommand("daemon-status", nil)
	if err != nil {
		return err
	}

	if !resp.Success {
		return errors.New(resp.Message)
	}

	fmt.Println(resp.Message)
	return nil
}

// DaemonInstallAction daemon install 命令的 Action
func DaemonInstallAction(c *cli.Context) error {
	service, err := GetDaemonService(c)
	if err != nil {
		return fmt.Errorf("failed to create daemon: %w", err)
	}

	status, err := service.Install("daemon-run")
	if err != nil {
		return fmt.Errorf("failed to install daemon: %w", err)
	}

	fmt.Printf(utils.ProcessManagerName+" daemon installed: %v\n", status)
	return nil
}

// DaemonRemoveAction daemon remove 命令的 Action
func DaemonRemoveAction(c *cli.Context) error {
	service, err := GetDaemonService(c)
	if err != nil {
		return fmt.Errorf("failed to create daemon: %w", err)
	}

	status, err := service.Remove()
	if err != nil {
		return fmt.Errorf("failed to remove daemon: %w", err)
	}

	fmt.Printf(utils.ProcessManagerName+" daemon removed: %v\n", status)
	return nil
}

// GetDaemonCommands 返回 daemon 相关命令
func GetDaemonCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:   "start",
			Usage:  "Start " + utils.ProcessManagerName + " system service",
			Action: DaemonStartAction,
		},
		{
			Name:   "stop",
			Usage:  "Stop " + utils.ProcessManagerName + " system service",
			Action: DaemonStopAction,
		},
		{
			Name:   "status",
			Usage:  "Show " + utils.ProcessManagerName + " daemon status",
			Action: DaemonStatusAction,
		},
		{
			Name:   "install",
			Usage:  "Install " + utils.ProcessManagerName + " as a system service",
			Action: DaemonInstallAction,
		},
		{
			Name:   "remove",
			Usage:  "Remove " + utils.ProcessManagerName + " system service",
			Action: DaemonRemoveAction,
		},
	}
}
