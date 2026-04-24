package action

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"processmanager/internal/utils"

	"github.com/urfave/cli/v3"
)

// ServeStaticAction 内置静态文件服务器（作为子进程运行）
func ServeStaticAction(ctx context.Context, cmd *cli.Command) error {
	dir := cmd.Args().First()
	if dir == "" {
		dir = "."
	}

	port := cmd.Int("port")
	if port <= 0 {
		port = 8080
	}

	fs := http.FileServer(http.Dir(dir))
	addr := ":" + strconv.Itoa(port)

	fmt.Printf("Serving %s on http://localhost:%d\n", dir, port)
	return http.ListenAndServe(addr, fs)
}

// GetServeStaticCommand 返回内置 serve-static 子命令
func GetServeStaticCommand() *cli.Command {
	return &cli.Command{
		Name:      "serve-static",
		Usage:     "Start a static file server (internal use)",
		Hidden:    true,
		ArgsUsage: "[directory]",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:    "port",
				Aliases: []string{"p"},
				Usage:   "Port to listen on",
				Value:   8080,
			},
		},
		Action: ServeStaticAction,
	}
}

// ServeAction serve 命令 - 将静态服务器注册为 pm 管理的进程
func ServeAction(ctx context.Context, cmd *cli.Command) error {
	if !IsDaemonRunning() {
		return errors.New(utils.ProcessManagerName + " daemon is not running")
	}

	dir := cmd.Args().First()
	if dir == "" {
		dir = "."
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("invalid directory: %w", err)
	}

	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("directory %s does not exist", dir)
	}

	port := cmd.Int("port")
	if port <= 0 {
		port = 8080
	}

	name := cmd.String("name")
	if name == "" {
		name = "serve-" + strconv.Itoa(port)
	}

	// 构建启动参数
	args := []string{"serve-static", dir, "-p", strconv.Itoa(port)}

	// 获取当前可执行文件的绝对路径
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	startArgs := map[string]any{
		"name":   name,
		"script": execPath,
		"args":   args,
	}

	resp, err := SendCommand("start", startArgs)
	if err != nil {
		return err
	}

	if !resp.Success {
		return errors.New(resp.Message)
	}

	fmt.Println(resp.Message)
	return nil
}

// GetServeCommand 返回 serve 命令
func GetServeCommand() *cli.Command {
	return &cli.Command{
		Name:      "serve",
		Usage:     "Start a static file server managed by pm",
		ArgsUsage: "[directory]",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:    "port",
				Aliases: []string{"p"},
				Usage:   "Port to listen on",
				Value:   8080,
			},
			&cli.StringFlag{
				Name:    "name",
				Aliases: []string{"n"},
				Usage:   "Process name",
			},
		},
		Action: ServeAction,
	}
}
