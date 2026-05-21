package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage OmniTun CLI configuration",
		Long:  "View and modify CLI configuration stored in ~/.omnitun/config.json.",
	}
	cmd.AddCommand(newConfigListCmd())
	cmd.AddCommand(newConfigSetCmd())
	cmd.AddCommand(newConfigGetCmd())
	cmd.AddCommand(newConfigUnsetCmd())
	return cmd
}

func configFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to locate home directory: %w", err)
	}
	dir := filepath.Join(home, ".omnitun")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}
	return filepath.Join(dir, "config.json"), nil
}

func loadConfig() (map[string]string, error) {
	path, err := configFilePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	var cfg map[string]string
	if err := json.Unmarshal(data, &cfg); err != nil {
		return make(map[string]string), nil
	}
	return cfg, nil
}

func saveConfig(cfg map[string]string) error {
	path, err := configFilePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

func newConfigListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all config keys and values",
		RunE:  runConfigList,
	}
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(2),
		RunE:  runConfigSet,
	}
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a config value",
		Args:  cobra.ExactArgs(1),
		RunE:  runConfigGet,
	}
}

func newConfigUnsetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unset <key>",
		Short: "Remove a config key (reset to default)",
		Args:  cobra.ExactArgs(1),
		RunE:  runConfigUnset,
	}
}

func runConfigList(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if len(cfg) == 0 {
		fmt.Println(dim("No configuration keys set."))
		return nil
	}
	fmt.Println()
	for k, v := range cfg {
		fmt.Printf("  %-30s %s\n", bold(k), dim(v))
	}
	fmt.Println()
	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key, value := args[0], args[1]
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg[key] = value
	if err := saveConfig(cfg); err != nil {
		return err
	}
	fmt.Printf("%s %s = %s\n", green("✓"), bold(key), dim(value))
	return nil
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	key := args[0]
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	value, ok := cfg[key]
	if !ok {
		fmt.Println(dim("(not set)"))
		return nil
	}
	fmt.Println(value)
	return nil
}

func runConfigUnset(cmd *cobra.Command, args []string) error {
	key := args[0]
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if _, ok := cfg[key]; !ok {
		fmt.Println(dim("Key not found."))
		return nil
	}
	delete(cfg, key)
	if err := saveConfig(cfg); err != nil {
		return err
	}
	fmt.Printf("%s %s unset\n", green("✓"), bold(key))
	return nil
}
