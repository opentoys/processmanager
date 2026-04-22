package action

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"processmanager/internal/config"
	"processmanager/internal/utils"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"github.com/urfave/cli/v3"
)

// ConfigShowAction config show 命令的 Action
func ConfigShowAction(ctx context.Context, cmd *cli.Command) error {
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
func ConfigLogAction(ctx context.Context, cmd *cli.Command) error {
	cfgPath := filepath.Join(Workspace, utils.PMConfigFile)
	var cfg utils.Config
	if err := config.LoadConfig(cfgPath, &cfg); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	updated := false
	if cmd.IsSet("size") {
		cfg.Log.MaxSize = cmd.Int("size")
		updated = true
	}
	if cmd.IsSet("files") {
		cfg.Log.MaxFiles = cmd.Int("files")
		updated = true
	}
	if cmd.IsSet("compress") {
		cfg.Log.Compress = cmd.Bool("compress")
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

// ConfigListAction config list 命令的 Action
func ConfigListAction(ctx context.Context, cmd *cli.Command) error {
	cfgPath := filepath.Join(Workspace, utils.PMConfigFile)
	buf, e := os.ReadFile(cfgPath)
	if e != nil {
		return e
	}

	var key = cmd.String("key")
	if key == "" {
		key = cmd.Args().First()
	}
	var result gjson.Result
	if key != "" {
		result = gjson.GetBytes(buf, key)
	} else {
		result = gjson.ParseBytes(buf)
	}
	var data = gjsonmap(key, result)
	fmt.Printf("% 8s   %s\n", "Type", "JSON Path")
	var keys []string
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("% 8s   %s\n", data[k], k)
	}
	fmt.Println("")
	return nil
}

func gjsonmap(k string, result gjson.Result) (data map[string]string) {
	data = make(map[string]string)
	if k != "" {
		k += "."
	}
	result.ForEach(func(key, value gjson.Result) bool {
		switch value.Type {
		case gjson.JSON:
			mm := gjsonmap(k+key.String(), value)
			for k, v := range mm {
				data[k] = v
			}
		default:
			data[k+key.String()] = value.Type.String()
		}
		return true
	})
	return
}

// ConfigSetAction config set 命令的 Action
func ConfigSetAction(ctx context.Context, cmd *cli.Command) error {
	cfgPath := filepath.Join(Workspace, utils.PMConfigFile)

	buf, e := os.ReadFile(cfgPath)
	if e != nil {
		return e
	}

	var key = cmd.String("key")
	var value = cmd.String("value")
	if key == "" && value == "" {
		key = cmd.Args().First()
		value = cmd.Args().Get(1)
	}
	if key != "" && value == "" {
		value = cmd.Args().First()
	}
	sval, e := sjson.Set(string(buf), key, value)
	if e != nil {
		return e
	}

	if err := os.WriteFile(cfgPath, []byte(sval), 0o644); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
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

// GetConfigListCommand 返回 config list 命令
func GetConfigListCommand() *cli.Command {
	return &cli.Command{
		Name:    "list",
		Usage:   "Configure log settings",
		Aliases: []string{"ls", "l"},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "key",
				Aliases: []string{"k"},
				Usage:   "Max size of each log file in MB",
			},
		},
		Action: ConfigListAction,
	}
}

// GetConfigSetCommand 设置 config 命令
func GetConfigSetCommand() *cli.Command {
	return &cli.Command{
		Name:      "set",
		Usage:     "Configure log settings",
		ArgsUsage: "[k.name value]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "key",
				Aliases: []string{"k"},
				Usage:   "Max size of each log file in MB",
			},
			&cli.StringFlag{
				Name:    "value",
				Aliases: []string{"v"},
				Usage:   "Max number of log files to keep",
			},
		},
		Action: ConfigSetAction,
	}
}
