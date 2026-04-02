# 进程管理工具设计文档

## 1. 项目概述

本项目是一个类似 Node.js PM2 的进程管理工具，用于管理和监控后台进程。它支持启动、停止、重启进程，以及查看进程状态、环境变量和日志等功能。

## 2. 功能需求

### 2.1 核心功能

1. **进程启动**：支持通过 `pm start` 命令启动进程
2. **进程列表**：支持通过 `pm list` 命令显示所有托管进程
3. **环境变量**：支持通过 `pm env xxx` 命令显示进程启动时的环境变量
4. **日志管理**：支持通过 `pm log xxx` 或 `pm logs` 命令实时显示托管进程日志
5. **日志配置**：支持设置日志保存路径及滚动压缩

### 2.2 扩展功能

1. **进程停止**：支持通过 `pm stop xxx` 命令停止进程
2. **进程重启**：支持通过 `pm restart xxx` 命令重启进程
3. **进程删除**：支持通过 `pm delete xxx` 命令删除进程
4. **进程状态**：支持通过 `pm status xxx` 命令查看进程状态
5. **进程监控**：支持监控进程的 CPU、内存使用情况

## 3. 技术架构

### 3.1 技术栈

- **语言**：Go 1.20+
- **依赖**：
  - `github.com/urfave/cli/v2` - 命令行界面
  - `github.com/gin-gonic/gin` - Web 服务器（可选，用于提供 REST API）
  - `github.com/go-resty/resty/v2` - HTTP 客户端（可选）
  - `github.com/rs/zerolog` - 日志库
  - `github.com/fsnotify/fsnotify` - 文件系统通知（用于日志监控）
  - `github.com/jinzhu/now` - 时间处理

### 3.2 目录结构

```
processmanager/
├── cmd/
│   └── pm/
│       └── main.go        # 命令行入口
├── internal/
│   ├── config/
│   │   └── config.go      # 配置管理
│   ├── manager/
│   │   ├── manager.go      # 进程管理器
│   │   ├── process.go      # 进程模型
│   │   └── monitor.go      # 进程监控
│   ├── logger/
│   │   ├── logger.go       # 日志管理
│   │   └── rotator.go      # 日志滚动
│   ├── api/
│   │   └── api.go          # REST API（可选）
│   └── utils/
│       └── utils.go        # 工具函数
├── pkg/
│   └── common/
│       └── common.go       # 公共函数
├── config.yaml             # 配置文件
├── go.mod
└── README.md
```

## 4. 核心功能实现

### 4.1 进程启动 (`pm start`)

- **功能**：启动一个新的进程并将其加入托管列表
- **参数**：
  - `--name`：进程名称
  - `--script`：要执行的脚本或命令
  - `--args`：传递给脚本的参数
  - `--env`：环境变量文件路径
  - `--log`：日志保存路径
  - `--cwd`：工作目录
- **实现**：
  1. 解析命令行参数
  2. 创建进程配置
  3. 启动进程
  4. 记录进程信息到托管列表
  5. 开始监控进程

### 4.2 进程列表 (`pm list`)

- **功能**：显示所有托管进程的状态信息
- **参数**：无
- **实现**：
  1. 读取托管进程列表
  2. 显示进程名称、ID、状态、CPU、内存等信息

### 4.3 环境变量 (`pm env xxx`)

- **功能**：显示指定进程启动时的环境变量
- **参数**：进程名称或 ID
- **实现**：
  1. 根据进程名称或 ID 查找进程
  2. 显示进程的环境变量

### 4.4 日志管理 (`pm log xxx` / `pm logs`)

- **功能**：实时显示托管进程的日志
- **参数**：
  - `pm log xxx`：显示指定进程的日志
  - `pm logs`：显示所有进程的日志
- **实现**：
  1. 根据进程名称或 ID 查找进程
  2. 打开进程的日志文件
  3. 实时读取并显示日志内容

### 4.5 日志配置

- **功能**：设置日志保存路径及滚动压缩
- **配置项**：
  - `log.path`：日志保存路径
  - `log.max_size`：单个日志文件最大大小（MB）
  - `log.max_files`：最大日志文件数量
  - `log.compress`：是否压缩日志文件
- **实现**：
  1. 读取配置文件中的日志配置
  2. 根据配置创建日志文件
  3. 实现日志滚动和压缩功能

## 5. 数据模型

### 5.1 进程配置

```go
type ProcessConfig struct {
    Name        string            `json:"name"`        // 进程名称
    Script      string            `json:"script"`      // 要执行的脚本或命令
    Args        []string          `json:"args"`        // 传递给脚本的参数
    Env         map[string]string `json:"env"`         // 环境变量
    LogPath     string            `json:"log_path"`    // 日志保存路径
    Cwd         string            `json:"cwd"`         // 工作目录
    MaxRestarts int               `json:"max_restarts"` // 最大重启次数
    RestartDelay int              `json:"restart_delay"` // 重启延迟（秒）
}
```

### 5.2 进程状态

```go
type ProcessStatus struct {
    ID          string    `json:"id"`          // 进程 ID
    Name        string    `json:"name"`        // 进程名称
    Status      string    `json:"status"`      // 进程状态（running, stopped, errored）
    PID         int       `json:"pid"`         // 系统进程 ID
    CPU         float64   `json:"cpu"`         // CPU 使用率
    Memory      uint64    `json:"memory"`      // 内存使用量（字节）
    Uptime      int64     `json:"uptime"`      // 运行时间（秒）
    Restarts    int       `json:"restarts"`    // 重启次数
    CreatedAt   time.Time `json:"created_at"`   // 创建时间
    StartedAt   time.Time `json:"started_at"`   // 启动时间
    LogPath     string    `json:"log_path"`     // 日志路径
}
```

## 6. 配置文件

```yaml
# 配置文件示例
log:
  path: ./logs
  max_size: 100 # MB
  max_files: 10
  compress: true

processes:
  - name: app1
    script: ./app1.js
    args:
      - --port
      - 3000
    env:
      NODE_ENV: production
    cwd: ./app1
    max_restarts: 10
    restart_delay: 5

  - name: app2
    script: ./app2.js
    args:
      - --port
      - 3001
    env:
      NODE_ENV: production
    cwd: ./app2
    max_restarts: 10
    restart_delay: 5
```

## 7. 命令行接口

### 7.1 命令列表

| 命令         | 描述             | 示例                                    |
| ------------ | ---------------- | --------------------------------------- |
| `pm start`   | 启动进程         | `pm start --name app --script ./app.js` |
| `pm stop`    | 停止进程         | `pm stop app`                           |
| `pm restart` | 重启进程         | `pm restart app`                        |
| `pm delete`  | 删除进程         | `pm delete app`                         |
| `pm list`    | 显示进程列表     | `pm list`                               |
| `pm status`  | 显示进程状态     | `pm status app`                         |
| `pm env`     | 显示环境变量     | `pm env app`                            |
| `pm log`     | 显示进程日志     | `pm log app`                            |
| `pm logs`    | 显示所有进程日志 | `pm logs`                               |
| `pm reload`  | 重新加载配置     | `pm reload`                             |
| `pm version` | 显示版本信息     | `pm version`                            |

## 8. 实现计划

### 8.1 阶段一：基础功能

1. 搭建项目结构
2. 实现命令行解析
3. 实现进程启动和管理
4. 实现进程列表功能

### 8.2 阶段二：核心功能

1. 实现环境变量管理
2. 实现日志管理和滚动压缩
3. 实现进程监控
4. 实现进程重启策略

### 8.3 阶段三：扩展功能

1. 实现 REST API
2. 实现 Web 界面
3. 实现集群管理
4. 实现插件系统

## 9. 注意事项

1. **权限问题**：确保进程管理工具具有足够的权限来启动和管理进程
2. **日志管理**：合理配置日志滚动和压缩，避免磁盘空间不足
3. **进程监控**：定期检查进程状态，及时处理异常情况
4. **配置管理**：提供合理的默认配置，同时支持自定义配置
5. **安全性**：避免执行恶意命令，确保进程管理的安全性

## 10. 测试计划

1. **功能测试**：测试所有命令的基本功能
2. **性能测试**：测试同时管理多个进程时的性能
3. **稳定性测试**：测试进程异常退出时的重启机制
4. **兼容性测试**：测试在不同操作系统上的兼容性

## 11. 部署计划

1. **构建**：使用 `go build` 构建可执行文件
2. **安装**：将可执行文件复制到系统 PATH 目录
3. **配置**：创建配置文件并设置合适的权限
4. **启动**：启动进程管理服务
5. **监控**：监控进程管理服务的运行状态

## 12. 结论

本进程管理工具将提供类似 PM2 的功能，帮助用户更方便地管理和监控后台进程。通过合理的设计和实现，它将成为一个可靠、高效的进程管理解决方案。
