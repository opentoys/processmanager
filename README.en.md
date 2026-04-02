# Process Manager (pm)

[中文 Version](README.md)

## Project Introduction

Process Manager (pm) is a lightweight process management tool written in Go, designed to manage, monitor, and automatically restart system processes. It provides a simple and user-friendly command-line interface, supporting operations such as starting, stopping, restarting, and status checking of processes, with the ability to automatically restart crashed processes.

## Key Features

- Process management: start, stop, restart, delete processes
- Process monitoring: automatically detect process status and restart on crash
- Log management: process log collection and rotation
- Configuration management: flexible configuration options
- Notification functionality: support for email and WeChat Work bot notifications
- Daemon mode: run as a system service

## Quick Start

### Installation

```bash
# Clone the repository
git clone <repository-url>

# Enter the project directory
cd processmanager

# Compile
make

# Install to system path
make install
```

### Basic Usage

```bash
# Start the daemon
pm daemon start

# Start a process
pm start --name "myapp" --script "/path/to/script.sh" --args "arg1" "arg2"

# List all processes
pm list

# Check process status
pm status <name-or-id>

# Stop a process
pm stop <name-or-id>

# Restart a process
pm restart <name-or-id>

# View process logs
pm log <name-or-id>

# Save process state
pm save

# Restore processes
pm resurrect
```

## Command Reference

### Process Management Commands

- `pm start`: Start a new process
- `pm stop`: Stop a process
- `pm restart`: Restart a process
- `pm delete`: Delete a process
- `pm list`: List all processes
- `pm status`: Check process status
- `pm log`: View process logs
- `pm logs`: View all process logs
- `pm save`: Save process state to file
- `pm resurrect`: Restore processes from save file

### Daemon Commands

- `pm daemon start`: Start the daemon
- `pm daemon stop`: Stop the daemon
- `pm daemon status`: Check daemon status

### Configuration Commands

- `pm config show`: Show current configuration
- `pm config log`: Configure log options
- `pm config channel`: Manage notification channels
- `pm config notice`: Manage notification rules

## Configuration File

The configuration file is located at `pm.json` in the workspace directory. Main configuration items include:

- `log`: Log configuration (path, size, number of files)
- `maxRestarts`: Maximum number of restarts
- `stateFile`: State file path
- `channels`: Notification channel configuration
- `notices`: Notification rule configuration

## Notification Functionality

Supports the following notification methods:

- Email notifications
- WeChat Work bot notifications

## Workspace

The default workspace directory is `~/.pm`, which can be customized via the `PM_WORKSPACE` environment variable.

## System Requirements

- Go 1.25.5 or higher
- Supported operating systems: Linux, macOS

## Dependencies

- github.com/expr-lang/expr v1.17.8
- github.com/shirou/gopsutil/v4 v4.26.2
- github.com/takama/daemon v1.0.0
- github.com/urfave/cli/v2 v2.25.7
- gopkg.in/natefinch/lumberjack.v2 v2.2.1

## Project Structure

```
├── cmd/
│   └── main.go          # Main entry file
├── internal/
│   ├── action/          # Command processing
│   ├── config/          # Configuration management
│   ├── logger/          # Log management
│   ├── manager/         # Core process management
│   ├── notifier/        # Notification functionality
│   └── utils/           # Utility functions
├── .gitignore
├── Makefile
├── README.md
├── go.mod
└── go.sum
```

## License

MIT License
