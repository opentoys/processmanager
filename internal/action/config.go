package action

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"processmanager/internal/config"
	"processmanager/internal/utils"

	"github.com/urfave/cli/v2"
)

// ConfigShowAction config show 命令的 Action
func ConfigShowAction(c *cli.Context) error {
	cfgPath := filepath.Join(Workspace, utils.PMConfigFile)
	var cfg utils.Config
	if err := config.LoadConfig(cfgPath, &cfg); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Printf("Config file: %s\n\n", cfgPath)

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// ConfigLogAction config log 命令的 Action
func ConfigLogAction(c *cli.Context) error {
	cfgPath := filepath.Join(Workspace, utils.PMConfigFile)
	var cfg utils.Config
	if err := config.LoadConfig(cfgPath, &cfg); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	updated := false
	if c.IsSet("size") {
		cfg.Log.MaxSize = c.Int("size")
		updated = true
	}
	if c.IsSet("files") {
		cfg.Log.MaxFiles = c.Int("files")
		updated = true
	}
	if c.IsSet("compress") {
		cfg.Log.Compress = c.Bool("compress")
		updated = true
	}

	if !updated {
		fmt.Printf("Current log config:\n")
		fmt.Printf("  max_size: %d MB\n", cfg.Log.MaxSize)
		fmt.Printf("  max_files: %d\n", cfg.Log.MaxFiles)
		fmt.Printf("  compress: %v\n", cfg.Log.Compress)
		return nil
	}

	if err := config.SaveConfig(cfgPath, &cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Log config updated:\n")
	fmt.Printf("  max_size: %d MB\n", cfg.Log.MaxSize)
	fmt.Printf("  max_files: %d\n", cfg.Log.MaxFiles)
	fmt.Printf("  compress: %v\n", cfg.Log.Compress)
	return nil
}

// GetConfigShowCommand 返回 config show 命令
func GetConfigShowCommand() *cli.Command {
	return &cli.Command{
		Name:   "show",
		Usage:  "Show current configuration",
		Action: ConfigShowAction,
	}
}

// GetConfigLogCommand 返回 config log 命令
func GetConfigLogCommand() *cli.Command {
	return &cli.Command{
		Name:  "log",
		Usage: "Configure log settings",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  "size",
				Usage: "Max size of each log file in MB",
			},
			&cli.IntFlag{
				Name:  "files",
				Usage: "Max number of log files to keep",
			},
			&cli.BoolFlag{
				Name:  "compress",
				Usage: "Enable compression for old log files",
			},
		},
		Action: ConfigLogAction,
	}
}
