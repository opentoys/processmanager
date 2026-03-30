package manager

import (
	"fmt"
	"log/slog"
	"time"
)

// Monitor 进程监控
type Monitor struct {
	processes map[string]*Process
}

// NewMonitor 创建监控
func NewMonitor() *Monitor {
	return &Monitor{
		processes: make(map[string]*Process),
	}
}

// AddProcess 添加进程到监控
func (m *Monitor) AddProcess(process *Process) {
	m.processes[process.config.Name] = process
}

// RemoveProcess 从监控中移除进程
func (m *Monitor) RemoveProcess(name string) {
	delete(m.processes, name)
}

// Start 开始监控
func (m *Monitor) Start() {
	go func() {
		for {
			for name, process := range m.processes {
				status := process.GetStatus()
				slog.Debug("Process status", "process", name, "status", status.Status, "pid", status.PID)
			}
			time.Sleep(5 * time.Second)
		}
	}()
}

// GetProcess 获取进程
func (m *Monitor) GetProcess(name string) (*Process, error) {
	process, ok := m.processes[name]
	if !ok {
		return nil, fmt.Errorf("process %s not found", name)
	}
	return process, nil
}
