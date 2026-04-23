package manager

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"processmanager/internal/utils"

	"github.com/robfig/cron/v3"
)

// CronManager 定时任务管理器
type CronManager struct {
	scheduler *cron.Cron
	jobs      map[string]*cronJobEntry
	filePath  string
	mu        sync.RWMutex
	parse     cron.Parser
}

// cronJobEntry 单个定时任务条目
type cronJobEntry struct {
	state   utils.CronJobState
	cronID  cron.EntryID
	logFile string
}

// NewCronManager 创建定时任务管理器
func NewCronManager() *CronManager {
	filePath := filepath.Join(utils.GetWorkspacePath(), utils.PMCronFile)

	// 确保日志目录存在
	cronLogDir := filepath.Join(utils.GetWorkspacePath(), utils.PMCronLogDir)
	if err := os.MkdirAll(cronLogDir, 0755); err != nil {
		slog.Error("Failed to create cron log directory", "error", err)
	}

	cm := &CronManager{
		scheduler: cron.New(cron.WithSeconds()),
		jobs:      make(map[string]*cronJobEntry),
		filePath:  filePath,
		parse: cron.NewParser(
			cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
		),
	}

	// 加载已保存的定时任务
	if err := cm.load(); err != nil {
		slog.Error("Failed to load cron jobs", "error", err)
	}

	return cm
}

// Start 启动定时任务调度器
func (cm *CronManager) Start() {
	cm.scheduler.Start()
	slog.Info("Cron scheduler started")
}

// Stop 停止定时任务调度器
func (cm *CronManager) Stop() {
	cm.scheduler.Stop()
	slog.Info("Cron scheduler stopped")
}

// AddJob 添加定时任务
func (cm *CronManager) AddJob(cfg utils.CronJobConfig) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	name := cfg.Name
	if name == "" {
		return fmt.Errorf("cron job name is required")
	}

	// 检查是否已存在
	if _, ok := cm.jobs[name]; ok {
		return fmt.Errorf("cron job %s already exists, remove it first", name)
	}

	// 验证 cron 表达式
	_, err := cm.parse.Parse(cfg.Spec)
	if err != nil {
		return fmt.Errorf("invalid cron spec %q: %w", cfg.Spec, err)
	}

	// 准备日志文件路径
	cronLogDir := filepath.Join(utils.GetWorkspacePath(), utils.PMCronLogDir)
	logFile := filepath.Join(cronLogDir, name+".log")

	// 创建任务状态
	state := utils.CronJobState{
		Name:       name,
		Spec:       cfg.Spec,
		Script:     cfg.Script,
		Args:       cfg.Args,
		Env:        cfg.Env,
		Cwd:        cfg.Cwd,
		Enabled:    cfg.Enabled,
		LastStatus: "pending",
	}

	// 创建执行函数的闭包
	jobFunc := cm.wrapJobFunc(state, logFile)

	// 添加到调度器
	cronID, err := cm.scheduler.AddFunc(cfg.Spec, jobFunc)
	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	// 获取下次执行时间
	entry := cm.scheduler.Entry(cronID)
	nextRun := int64(0)
	if !entry.Next.IsZero() {
		nextRun = entry.Next.Unix()
	}

	state.NextRunAt = nextRun

	cm.jobs[name] = &cronJobEntry{
		state:   state,
		cronID:  cronID,
		logFile: logFile,
	}

	// 持久化
	if err := cm.save(); err != nil {
		slog.Error("Failed to save cron state", "error", err)
	}

	slog.Info("Cron job added", "name", name, "spec", cfg.Spec, "script", cfg.Script)
	return nil
}

// wrapJobFunc 包装任务执行函数，处理日志、状态更新
func (cm *CronManager) wrapJobFunc(state utils.CronJobState, logFile string) func() {
	return func() {
		cm.mu.Lock()
		state.LastStatus = "running"
		state.LastRunAt = time.Now().Unix()
		cm.jobs[state.Name].state = state
		cm.mu.Unlock()

		slog.Info("Cron job executing", "name", state.Name, "script", state.Script)

		// 执行命令
		err := cm.executeScript(state.Script, state.Args, state.Env, state.Cwd, logFile)

		cm.mu.Lock()
		entry, ok := cm.jobs[state.Name]
		if ok {
			entry.state.LastStatus = "pending"
			if err != nil {
				entry.state.LastStatus = "failed"
				entry.state.TotalFail++
				slog.Error("Cron job failed", "name", state.Name, "error", err)
			} else {
				entry.state.LastStatus = "success"
				slog.Info("Cron job executed successfully", "name", state.Name)
			}
			entry.state.TotalRun++

			// 更新下次执行时间
			cronEntry := cm.scheduler.Entry(entry.cronID)
			if !cronEntry.Next.IsZero() {
				entry.state.NextRunAt = cronEntry.Next.Unix()
			}
		}
		cm.mu.Unlock()

		// 持久化
		_ = cm.save()
	}
}

// executeScript 执行脚本并记录输出到日志文件
func (cm *CronManager) executeScript(script string, args []string, env []string, cwd string, logFile string) error {
	if script == "" {
		return fmt.Errorf("script is empty")
	}

	// 构建完整命令字符串
	var cmdStr string
	if len(args) > 0 {
		cmdStr = script + " " + strings.Join(args, " ")
	} else {
		cmdStr = script
	}

	cmd := exec.Command("sh", "-c", cmdStr)
	if cwd != "" {
		cmd.Dir = cwd
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	// 打开日志文件（追加模式）
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer f.Close()

	// 写入分隔行
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(f, "\n--- [%s] executing: %s ---\n", timestamp, cmdStr)

	cmd.Stdout = f
	cmd.Stderr = f

	return cmd.Run()
}

// RemoveJob 根据 name 或 ID 删除定时任务
func (cm *CronManager) RemoveJob(nameOrID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 检查是否为数字 ID
	id, err := strconv.Atoi(nameOrID)
	if err == nil && id > 0 {
		var entries []*cronJobEntry
		for _, entry := range cm.jobs {
			entries = append(entries, entry)
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].state.Name < entries[j].state.Name
		})
		if id <= len(entries) {
			name := entries[id-1].state.Name
			cm.scheduler.Remove(entries[id-1].cronID)
			delete(cm.jobs, name)
			if err := cm.save(); err != nil {
				slog.Error("Failed to save cron state after remove", "error", err)
			}
			return nil
		}
	}

	// 按名称查找
	entry, ok := cm.jobs[nameOrID]
	if !ok {
		return fmt.Errorf("cron job %s not found", nameOrID)
	}

	cm.scheduler.Remove(entry.cronID)
	delete(cm.jobs, nameOrID)

	if err := cm.save(); err != nil {
		slog.Error("Failed to save cron state after remove", "error", err)
	}

	slog.Info("Cron job removed", "name", nameOrID)
	return nil
}

// ListJobs 列出所有定时任务
func (cm *CronManager) ListJobs() []utils.CronJobState {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var result []utils.CronJobState
	for _, entry := range cm.jobs {
		result = append(result, entry.state)
	}
	return result
}

// GetJob 根据 name 或 ID 查找定时任务
func (cm *CronManager) GetJob(nameOrID string) (utils.CronJobState, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// 检查是否为数字 ID
	id, err := strconv.Atoi(nameOrID)
	if err == nil && id > 0 {
		var jobs []utils.CronJobState
		for _, entry := range cm.jobs {
			jobs = append(jobs, entry.state)
		}
		// sort by name for stable ordering
		sort.Slice(jobs, func(i, j int) bool {
			return jobs[i].Name < jobs[j].Name
		})
		if id <= len(jobs) {
			return jobs[id-1], nil
		}
	}

	// 按名称查找
	entry, ok := cm.jobs[nameOrID]
	if !ok {
		return utils.CronJobState{}, fmt.Errorf("cron job %s not found", nameOrID)
	}
	return entry.state, nil
}

// GetJobLogPath 根据 name 或 ID 获取定时任务日志路径
func (cm *CronManager) GetJobLogPath(nameOrID string) (string, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// 检查是否为数字 ID
	id, err := strconv.Atoi(nameOrID)
	if err == nil && id > 0 {
		var entries []*cronJobEntry
		for _, entry := range cm.jobs {
			entries = append(entries, entry)
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].state.Name < entries[j].state.Name
		})
		if id <= len(entries) {
			return entries[id-1].logFile, nil
		}
	}

	// 按名称查找
	entry, ok := cm.jobs[nameOrID]
	if !ok {
		return "", fmt.Errorf("cron job %s not found", nameOrID)
	}
	return entry.logFile, nil
}

// save 持久化定时任务状态
func (cm *CronManager) save() error {
	stateFile := utils.CronStateFile{
		Jobs: make(map[string]utils.CronJobState),
	}
	for name, entry := range cm.jobs {
		stateFile.Jobs[name] = entry.state
	}

	data, err := json.MarshalIndent(stateFile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cron state: %w", err)
	}

	dir := filepath.Dir(cm.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(cm.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cron state file: %w", err)
	}

	return nil
}

// load 加载已保存的定时任务
func (cm *CronManager) load() error {
	data, err := os.ReadFile(cm.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read cron state file: %w", err)
	}

	var stateFile utils.CronStateFile
	if err := json.Unmarshal(data, &stateFile); err != nil {
		return fmt.Errorf("failed to unmarshal cron state: %w", err)
	}

	for name, state := range stateFile.Jobs {
		if !state.Enabled {
			// 跳过禁用的任务，但保留状态
			cronLogDir := filepath.Join(utils.GetWorkspacePath(), utils.PMCronLogDir)
			logFile := filepath.Join(cronLogDir, name+".log")
			cm.jobs[name] = &cronJobEntry{
				state:   state,
				cronID:  0,
				logFile: logFile,
			}
			continue
		}

		// 验证 cron 表达式
		if _, err := cron.ParseStandard(state.Spec); err != nil {
			slog.Error("Invalid cron spec in saved state, skipping", "name", name, "spec", state.Spec, "error", err)
			continue
		}

		cronLogDir := filepath.Join(utils.GetWorkspacePath(), utils.PMCronLogDir)
		logFile := filepath.Join(cronLogDir, name+".log")

		// 创建执行闭包
		jobFunc := cm.wrapJobFunc(state, logFile)

		cronID, err := cm.scheduler.AddFunc(state.Spec, jobFunc)
		if err != nil {
			slog.Error("Failed to add cron job from saved state", "name", name, "error", err)
			continue
		}

		// 更新下次执行时间
		entry := cm.scheduler.Entry(cronID)
		if !entry.Next.IsZero() {
			state.NextRunAt = entry.Next.Unix()
		}

		cm.jobs[name] = &cronJobEntry{
			state:   state,
			cronID:  cronID,
			logFile: logFile,
		}

		slog.Debug("Loaded cron job", "name", name, "spec", state.Spec)
	}

	return nil
}
