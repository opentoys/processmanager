package action

import (
	"github.com/urfave/cli/v2"
)

// GetConfigCommands 返回配置管理相关命令
func GetConfigCommands() []*cli.Command {
	return []*cli.Command{
		GetConfigShowCommand(),
		GetConfigLogCommand(),
		{
			Name:        "channel",
			Usage:       "Manage notification channels",
			Subcommands: GetConfigChannelCommands(),
		},
		{
			Name:        "notice",
			Usage:       "Manage notification rules",
			Subcommands: GetConfigNoticeCommands(),
		},
	}
}
