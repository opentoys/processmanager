package action

import (
	"github.com/urfave/cli/v3"
)

// GetConfigCommands 返回配置管理相关命令
func GetConfigCommands() []*cli.Command {
	return []*cli.Command{
		GetConfigShowCommand(),
		GetConfigLogCommand(),
		GetConfigListCommand(),
		GetConfigSetCommand(),
		GetReloadCommand(),
		{
			Name:     "channel",
			Usage:    "Manage notification channels",
			Commands: GetConfigChannelCommands(),
		},
		{
			Name:     "notice",
			Usage:    "Manage notification rules",
			Commands: GetConfigNoticeCommands(),
		},
	}
}
