package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/salmonumbrella/roam-cli/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
	Long: `Manage CLI configuration stored in ~/.config/roam/config.yaml.

You can view, set, or unset config keys such as base_url, graph_name,
token, keyring_backend, and output_format.`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfigFromFlag()
		if err != nil {
			return formatConfigLoadError(err)
		}
		if structuredOutputRequested() {
			return printStructured(configOutput(cfg))
		}

		fmt.Println("Config:")
		fmt.Printf("  base_url: %s\n", cfg.BaseURL)
		fmt.Printf("  graph_name: %s\n", cfg.GraphName)
		fmt.Printf("  token: %s\n", maskToken(cfg.Token))
		fmt.Printf("  keyring_backend: %s\n", cfg.KeyringBackend)
		fmt.Printf("  output_format: %s\n", cfg.OutputFormat)
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE:  runConfigSet,
}

var configUnsetCmd = &cobra.Command{
	Use:   "unset <key>",
	Short: "Unset a configuration value",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigUnset,
}

var configKeysCmd = &cobra.Command{
	Use:   "keys",
	Short: "List supported configuration keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		keys := supportedConfigKeys()
		sort.Strings(keys)

		if structuredOutputRequested() {
			return printStructured(keys)
		}

		fmt.Println("Supported keys:")
		for _, key := range keys {
			fmt.Printf("  %s\n", key)
		}
		return nil
	},
}

func configPath() (string, error) {
	if strings.TrimSpace(configFile) != "" {
		return configFile, nil
	}
	return config.DefaultConfigPath()
}

func supportedConfigKeys() []string {
	return []string{
		"base_url",
		"graph_name",
		"token",
		"keyring_backend",
		"output_format",
	}
}

func applyConfigValue(cfg *config.Config, key, value string) error {
	switch key {
	case "base_url":
		cfg.BaseURL = value
	case "graph_name":
		cfg.GraphName = value
	case "token":
		cfg.Token = value
	case "keyring_backend":
		cfg.KeyringBackend = value
	case "output_format":
		cfg.OutputFormat = value
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}
	return nil
}

func clearConfigValue(cfg *config.Config, key string) error {
	switch key {
	case "base_url":
		cfg.BaseURL = ""
	case "graph_name":
		cfg.GraphName = ""
	case "token":
		cfg.Token = ""
	case "keyring_backend":
		cfg.KeyringBackend = ""
	case "output_format":
		cfg.OutputFormat = ""
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}
	return nil
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configUnsetCmd)
	configCmd.AddCommand(configKeysCmd)

	rootCmd.AddCommand(configCmd)
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := strings.ToLower(strings.TrimSpace(args[0]))
	value := strings.TrimSpace(args[1])

	cfg, err := loadConfigFromFlag()
	if err != nil {
		return formatConfigLoadError(err)
	}

	if err := applyConfigValue(cfg, key, value); err != nil {
		return err
	}

	path, err := configPath()
	if err != nil {
		return err
	}
	if err := cfg.Save(path); err != nil {
		return err
	}

	if structuredOutputRequested() {
		if key == "token" {
			value = maskToken(value)
		}
		return printStructured(map[string]string{
			"status": "updated",
			"key":    key,
			"value":  value,
		})
	}

	fmt.Printf("Updated %s\n", key)
	return nil
}

func runConfigUnset(cmd *cobra.Command, args []string) error {
	key := strings.ToLower(strings.TrimSpace(args[0]))

	cfg, err := loadConfigFromFlag()
	if err != nil {
		return formatConfigLoadError(err)
	}

	if err := clearConfigValue(cfg, key); err != nil {
		return err
	}

	path, err := configPath()
	if err != nil {
		return err
	}
	if err := cfg.Save(path); err != nil {
		return err
	}

	if structuredOutputRequested() {
		return printStructured(map[string]string{
			"status": "unset",
			"key":    key,
		})
	}

	fmt.Printf("Unset %s\n", key)
	return nil
}

func configOutput(cfg *config.Config) map[string]interface{} {
	return map[string]interface{}{
		"base_url":        cfg.BaseURL,
		"graph_name":      cfg.GraphName,
		"token":           maskToken(cfg.Token),
		"token_set":       cfg.Token != "",
		"keyring_backend": cfg.KeyringBackend,
		"output_format":   cfg.OutputFormat,
	}
}
