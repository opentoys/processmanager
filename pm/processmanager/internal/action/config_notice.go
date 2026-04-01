package action

import (
	"fmt"
	"path/filepath"

	"processmanager/internal/config"
	"processmanager/internal/utils"

	"github.com/urfave/cli/v2"
)

// ConfigNoticeAddAction config notice add 命令的 Action
func ConfigNoticeAddAction(c *cli.Context) error {
	name := c.String("name")
	if name == "" {
		return fmt.Errorf("rule name is required")
	}

	cfgPath := filepath.Join(Workspace, utils.PMConfigFile)
	var cfg utils.Config
	if err := config.LoadConfig(cfgPath, &cfg); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.Notice == nil {
		cfg.Notice = make(map[string]utils.NoticeRule)
	}

	if _, exists := cfg.Notice[name]; exists {
		return fmt.Errorf("notice rule '%s' already exists", name)
	}

	rule := utils.NoticeRule{
		Expr:    c.String("expr"),
		Channel: c.StringSlice("channel"),
	}

	cfg.Notice[name] = rule
	if err := config.SaveConfig(cfgPath, &cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Notice rule '%s' added successfully\n", name)
	return nil
}

// ConfigNoticeRemoveAction config notice remove 命令的 Action
func ConfigNoticeRemoveAction(c *cli.Context) error {
	name := c.String("name")
	if name == "" {
		return fmt.Errorf("rule name is required")
	}

	cfgPath := filepath.Join(Workspace, utils.PMConfigFile)
	var cfg utils.Config
	if err := config.LoadConfig(cfgPath, &cfg); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.Notice == nil {
		return fmt.Errorf("no notice rules configured")
	}

	if _, exists := cfg.Notice[name]; !exists {
		return fmt.Errorf("notice rule '%s' not found", name)
	}

	delete(cfg.Notice, name)
	if err := config.SaveConfig(cfgPath, &cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Notice rule '%s' removed successfully\n", name)
	return nil
}

// ConfigNoticeEditAction config notice edit 命令的 Action
func ConfigNoticeEditAction(c *cli.Context) error {
	name := c.String("name")
	if name == "" {
		return fmt.Errorf("rule name is required")
	}

	cfgPath := filepath.Join(Workspace, utils.PMConfigFile)
	var cfg utils.Config
	if err := config.LoadConfig(cfgPath, &cfg); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.Notice == nil {
		return fmt.Errorf("no notice rules configured")
	}

	rule, exists := cfg.Notice[name]
	if !exists {
		return fmt.Errorf("notice rule '%s' not found", name)
	}

	updated := false
	if c.IsSet("expr") {
		rule.Expr = c.String("expr")
		updated = true
	}
	if c.IsSet("channel") {
		rule.Channel = c.StringSlice("channel")
		updated = true
	}

	if !updated {
		return fmt.Errorf("no changes specified")
	}

	cfg.Notice[name] = rule
	if err := config.SaveConfig(cfgPath, &cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Notice rule '%s' updated successfully\n", name)
	return nil
}

// ConfigNoticeListAction config notice list 命令的 Action
func ConfigNoticeListAction(c *cli.Context) error {
	cfgPath := filepath.Join(Workspace, utils.PMConfigFile)
	var cfg utils.Config
	if err := config.LoadConfig(cfgPath, &cfg); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.Notice) == 0 {
		fmt.Println("No notice rules configured")
		return nil
	}

	fmt.Println("Notification Rules:")
	fmt.Println("-------------------")
	for name, rule := range cfg.Notice {
		fmt.Printf("  Name: %s\n", name)
		fmt.Printf("  Expr: %s\n", rule.Expr)
		fmt.Printf("  Channels: %v\n", rule.Channel)
		fmt.Println()
	}
	return nil
}

// GetConfigNoticeCommands 返回 config notice 子命令
func GetConfigNoticeCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:  "add",
			Usage: "Add a notification rule",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "name",
					Usage: "Rule name (process name, pid, or * for all)",
				},
				&cli.StringFlag{
					Name:  "expr",
					Usage: "Expression to match log content",
				},
				&cli.StringSliceFlag{
					Name:  "channel",
					Usage: "Channel names to notify",
				},
			},
			Action: ConfigNoticeAddAction,
		},
		{
			Name:  "remove",
			Usage: "Remove a notification rule",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "name",
					Usage: "Rule name",
				},
			},
			Action: ConfigNoticeRemoveAction,
		},
		{
			Name:  "edit",
			Usage: "Edit a notification rule",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "name",
					Usage: "Rule name",
				},
				&cli.StringFlag{
					Name:  "expr",
					Usage: "Expression to match log content",
				},
				&cli.StringSliceFlag{
					Name:  "channel",
					Usage: "Channel names to notify",
				},
			},
			Action: ConfigNoticeEditAction,
		},
		{
			Name:   "list",
			Usage:  "List all notification rules",
			Action: ConfigNoticeListAction,
		},
	}
}
