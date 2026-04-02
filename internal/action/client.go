package action

import (
	"encoding/json"
	"fmt"
	"net"
	"os"

	"processmanager/internal/utils"
)

// Workspace 是配置文件所在目录
var Workspace string

// IsDaemonRunning 检查守护进程是否正在运行
func IsDaemonRunning() bool {
	socketPath := utils.GetSocketPath()
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		return false
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return false
	}
	conn.Close()

	return true
}

// SendCommand 发送命令到守护进程
func SendCommand(action string, args any) (*utils.Response, error) {
	socketPath := utils.GetSocketPath()
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to daemon: %w", err)
	}

	argsJSON, err := json.Marshal(args)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to marshal args: %w", err)
	}

	cmd := utils.Command{
		Action: action,
		Args:   argsJSON,
	}

	cmdJSON, err := json.Marshal(cmd)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}

	if _, err := conn.Write(cmdJSON); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to write command: %w", err)
	}

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

	var resp utils.Response
	if err := json.Unmarshal(buf, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &resp, nil
}
