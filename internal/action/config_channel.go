package action

import (
	"context"
	"fmt"
	"path/filepath"

	"processmanager/internal/config"
	"processmanager/internal/utils"

	"github.com/urfave/cli/v3"
)

// ConfigChannelAddAction config channel add 命令的 Action
func ConfigChannelAddAction(ctx context.Context, cmd *cli.Command) error {
	name := cmd.String("name")
	if name == "" {
		return fmt.Errorf("channel name is required")
	}
	chType := cmd.String("type")
	if chType == "" {
		return fmt.Errorf("channel type is required")
	}

	cfgPath := filepath.Join(Workspace, utils.PMConfigFile)
	var cfg utils.Config
	if err := config.LoadConfig(cfgPath, &cfg); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.Channels == nil {
		cfg.Channels = make(map[string]utils.ChanConfig)
	}

	if _, exists := cfg.Channels[name]; exists {
		return fmt.Errorf("channel '%s' already exists", name)
	}

	ch := utils.ChanConfig{Type: chType}
	switch chType {
	case "wecombot":
		ch.Key = cmd.String("key")
		if ch.Key == "" {
			return fmt.Errorf("key is required for wecombot")
		}
	case "mail":
		ch.To = cmd.String("to")
		ch.From = cmd.String("from")
		ch.SMTP = cmd.String("smtp")
		if ch.To == "" || ch.From == "" || ch.SMTP == "" {
			return fmt.Errorf("to, from, and smtp are required for mail")
		}
	default:
		return fmt.Errorf("unknown channel type: %s", chType)
	}

	cfg.Channels[name] = ch
	if err := config.SaveConfig(cfgPath, &cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Channel '%s' added successfully\n", name)
	return nil
}

// ConfigChannelRemoveAction config channel remove 命令的 Action
func ConfigChannelRemoveAction(ctx context.Context, cmd *cli.Command) error {
	name := cmd.String("name")
	if name == "" {
		return fmt.Errorf("channel name is required")
	}

	cfgPath := filepath.Join(Workspace, utils.PMConfigFile)
	var cfg utils.Config
	if err := config.LoadConfig(cfgPath, &cfg); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.Channels == nil {
		return fmt.Errorf("no channels configured")
	}

	if _, exists := cfg.Channels[name]; !exists {
		return fmt.Errorf("channel '%s' not found", name)
	}

	delete(cfg.Channels, name)
	if err := config.SaveConfig(cfgPath, &cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Channel '%s' removed successfully\n", name)
	return nil
}

// ConfigChannelEditAction config channel edit 命令的 Action
func ConfigChannelEditAction(ctx context.Context, cmd *cli.Command) error {
	name := cmd.String("name")
	if name == "" {
		return fmt.Errorf("channel name is required")
	}

	cfgPath := filepath.Join(Workspace, utils.PMConfigFile)
	var cfg utils.Config
	if err := config.LoadConfig(cfgPath, &cfg); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.Channels == nil {
		return fmt.Errorf("no channels configured")
	}

	ch, exists := cfg.Channels[name]
	if !exists {
		return fmt.Errorf("channel '%s' not found", name)
	}

	updated := false
	if cmd.IsSet("key") {
		ch.Key = cmd.String("key")
		updated = true
	}
	if cmd.IsSet("to") {
		ch.To = cmd.String("to")
		updated = true
	}
	if cmd.IsSet("from") {
		ch.From = cmd.String("from")
		updated = true
	}
	if cmd.IsSet("smtp") {
		ch.SMTP = cmd.String("smtp")
		updated = true
	}

	if !updated {
		return fmt.Errorf("no changes specified")
	}

	cfg.Channels[name] = ch
	if err := config.SaveConfig(cfgPath, &cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Channel '%s' updated successfully\n", name)
	return nil
}

// ConfigChannelListAction config channel list 命令的 Action
func ConfigChannelListAction(ctx context.Context, cmd *cli.Command) error {
	cfgPath := filepath.Join(Workspace, utils.PMConfigFile)
	var cfg utils.Config
	if err := config.LoadConfig(cfgPath, &cfg); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.Channels) == 0 {
		fmt.Println("No channels configured")
		return nil
	}

	fmt.Println("Notification Channels:")
	fmt.Println("----------------------")
	for name, ch := range cfg.Channels {
		fmt.Printf("  Name: %s\n", name)
		fmt.Printf("  Type: %s\n", ch.Type)
		switch ch.Type {
		case "wecombot":
			fmt.Printf("  Key: %s\n", ch.Key)
		case "mail":
			fmt.Printf("  To: %s\n", ch.To)
			fmt.Printf("  From: %s\n", ch.From)
			fmt.Printf("  SMTP: %s\n", ch.SMTP)
		}
		fmt.Println()
	}
	return nil
}

// GetConfigChannelCommands 返回 config channel 子命令
func GetConfigChannelCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:  "add",
			Usage: "Add a notification channel",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "name",
					Usage: "Channel name",
				},
				&cli.StringFlag{
					Name:  "type",
					Usage: "Channel type: wecombot or mail",
				},
				&cli.StringFlag{
					Name:  "key",
					Usage: "Webhook key (for wecombot)",
				},
				&cli.StringFlag{
					Name:  "to",
					Usage: "Recipient email (for mail)",
				},
				&cli.StringFlag{
					Name:  "from",
					Usage: "Sender email (for mail)",
				},
				&cli.StringFlag{
					Name:  "smtp",
					Usage: "SMTP server (user:passwd@host:port)",
				},
			},
			Action: ConfigChannelAddAction,
		},
		{
			Name:  "remove",
			Usage: "Remove a notification channel",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "name",
					Usage: "Channel name",
				},
			},
			Action: ConfigChannelRemoveAction,
		},
		{
			Name:  "edit",
			Usage: "Edit a notification channel",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "name",
					Usage: "Channel name",
				},
				&cli.StringFlag{
					Name:  "key",
					Usage: "Webhook key (for wecombot)",
				},
				&cli.StringFlag{
					Name:  "to",
					Usage: "Recipient email (for mail)",
				},
				&cli.StringFlag{
					Name:  "from",
					Usage: "Sender email (for mail)",
				},
				&cli.StringFlag{
					Name:  "smtp",
					Usage: "SMTP server (user:passwd@host:port)",
				},
			},
			Action: ConfigChannelEditAction,
		},
		{
			Name:   "list",
			Usage:  "List all notification channels",
			Action: ConfigChannelListAction,
		},
	}
}
