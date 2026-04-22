# Process Manager (pm)

[English Version](README.en.md)

## 项目简介

Process Manager (pm) 是一个用 Go 语言编写的轻量级进程管理工具，用于管理、监控和自动重启系统进程。它提供了简单易用的命令行界面，支持进程的启动、停止、重启、状态查看等操作，并具有自动重启崩溃进程的能力。

## 主要功能

- 进程管理：启动、停止、重启、删除进程
- 进程监控：自动检测进程状态，崩溃时自动重启
- 日志管理：进程日志收集和轮转
- 配置管理：灵活的配置选项
- 通知功能：支持邮件和企业微信机器人通知
- 守护进程模式：作为系统服务运行

## 快速开始

### 安装

```bash
# 克隆代码库
git clone <repository-url>

# 进入项目目录
cd processmanager

# 编译
make

# 安装到系统路径
./pm daemon install
```

### 基本使用

```bash
# 启动守护进程
pm daemon start

# 启动一个进程
pm start --name "myapp" --script "/path/to/script.sh" --args "arg1" "arg2"

# 列出所有进程
pm list

# 查看进程状态
pm status <name-or-id>

# 停止进程
pm stop <name-or-id>

# 重启进程
pm restart <name-or-id>

# 查看进程日志
pm log <name-or-id>

# 保存进程状态
pm save

# 恢复进程
pm resurrect
```

## 命令参考

### 进程管理命令

- `pm start`: 启动一个新进程
- `pm stop`: 停止一个进程
- `pm restart`: 重启一个进程
- `pm delete`: 删除一个进程
- `pm list`: 列出所有进程
- `pm status`: 查看进程状态
- `pm log`: 查看进程日志
- `pm logs`: 查看所有进程日志
- `pm save`: 保存进程状态到文件
- `pm resurrect`: 从保存文件恢复进程

### 守护进程命令

- `pm daemon start`: 启动守护进程
- `pm daemon stop`: 停止守护进程
- `pm daemon status`: 查看守护进程状态

### 配置命令

- `pm config show`: 显示当前配置
- `pm config log`: 配置日志选项
- `pm config channel`: 管理通知通道
- `pm config notice`: 管理通知规则

## 配置文件

配置文件位于工作目录下的 `config.json`，主要配置项包括：

- `log`: 日志配置（路径、大小、文件数）
- `maxRestarts`: 最大重启次数
- `stateFile`: 状态文件路径
- `channels`: 通知通道配置
- `notices`: 通知规则配置

## 通知功能

支持以下通知方式：

- 邮件通知
- 企业微信机器人通知

## 工作目录

默认工作目录为 `~/.pm`，可通过环境变量 `PM_WORKSPACE` 自定义。

## 系统要求

- Go 1.25.5 或更高版本
- 支持的操作系统：Linux, macOS

## 依赖项

- github.com/expr-lang/expr v1.17.8
- github.com/shirou/gopsutil/v4 v4.26.2
- github.com/takama/daemon v1.0.0
- github.com/urfave/cli/v2 v2.25.7
- gopkg.in/natefinch/lumberjack.v2 v2.2.1

## 项目结构

```
├── cmd/
│   └── main.go          # 主入口文件
├── internal/
│   ├── action/          # 命令处理
│   ├── config/          # 配置管理
│   ├── logger/          # 日志管理
│   ├── manager/         # 进程管理核心
│   ├── notifier/        # 通知功能
│   └── utils/           # 工具函数
├── .gitignore
├── Makefile
├── README.md
├── go.mod
└── go.sum
```

## 鸣谢

- [Ascii Text](https://patorjk.com/software/taag/#f=Wet+Letter)

## 许可证

MIT License
