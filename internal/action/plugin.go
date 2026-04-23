package action

import (
	"context"

	"github.com/urfave/cli/v3"
)

// GetPluginCommand plugin 命令
func GetPluginCommand() *cli.Command {
	return &cli.Command{
		Name:  "plugin",
		Usage: "Manage plugins",
		Commands: []*cli.Command{
			{
				Name:      "add",
				UsageText: "plugin path",
				Action: func(ctx context.Context, c *cli.Command) (e error) {
					return
				},
			},
			{
				Name: "list",
				Action: func(ctx context.Context, c *cli.Command) (e error) {
					return
				},
			},
			{
				Name:    "remove",
				Aliases: []string{"rm"},
				Action: func(ctx context.Context, c *cli.Command) (e error) {
					return
				},
			},
		},
	}
}
