package cmd

import (
	"fmt"
	"strings"

	"github.com/salmonumbrella/roam-cli/internal/api"
	"github.com/salmonumbrella/roam-cli/internal/config"
	"github.com/salmonumbrella/roam-cli/internal/secrets"
	"github.com/spf13/cobra"
)

// loadConfigFromFlag loads config from --config if provided, otherwise from default path.
func loadConfigFromFlag() (*config.Config, error) {
	if strings.TrimSpace(configFile) != "" {
		return config.Load(configFile)
	}
	return config.ReadConfig()
}

func flagChanged(cmd *cobra.Command, name string) bool {
	if cmd == nil {
		return false
	}
	if cmd.Flags().Changed(name) {
		return true
	}
	return cmd.InheritedFlags().Changed(name)
}

// applyOutputFormat applies config output_format if the user did not set --output.
func applyOutputFormat(cmd *cobra.Command, cfg *config.Config) {
	if cfg == nil || cfg.OutputFormat == "" {
		return
	}
	if !flagChanged(cmd, "output") && !flagChanged(cmd, "format") && strings.TrimSpace(outputFmt) == "text" {
		outputFmt = strings.TrimSpace(cfg.OutputFormat)
	}
}

// resolveCredentials resolves token/graph/mode with precedence:
// flags > env > keyring > config. Mode comes from --local flag or keyring.
func resolveCredentials(cmd *cobra.Command, cfg *config.Config) (string, string, string, error) {
	token := strings.TrimSpace(apiToken)
	graph := strings.TrimSpace(graphName)
	mode := ""

	// --local flag overrides mode
	if useLocal {
		mode = secrets.ModeEncrypted
	}

	// Flags (only if explicitly set)
	if !flagChanged(cmd, "token") {
		token = ""
	}
	if !flagChanged(cmd, "graph") {
		graph = ""
	}

	// Environment
	if token == "" {
		if v := strings.TrimSpace(envGet("ROAM_API_TOKEN")); v != "" {
			token = v
		}
	}
	if graph == "" {
		if v := strings.TrimSpace(envGet("ROAM_GRAPH_NAME")); v != "" {
			graph = v
		}
	}

	// Keyring (only if still missing)
	needKeyring := token == "" || graph == ""
	if needKeyring {
		store, err := openSecretsStore()
		if err == nil {
			if token == "" {
				if tok, err := store.GetToken(defaultProfile); err == nil {
					token = tok.RefreshToken
					// Only use keyring mode if --local wasn't specified
					if mode == "" {
						mode = tok.Mode
					}
				}
			}
			if graph == "" {
				if graphTok, err := store.GetToken(graphKeyPrefix + defaultProfile); err == nil {
					graph = graphTok.RefreshToken
				}
			}
		}
	}

	// Config fallback
	if token == "" && cfg != nil {
		token = strings.TrimSpace(cfg.Token)
	}
	if graph == "" && cfg != nil {
		graph = strings.TrimSpace(cfg.GraphName)
	}

	return token, graph, mode, nil
}

// clientOptionsFromConfig builds API client options from config.
func clientOptionsFromConfig(cfg *config.Config) []api.ClientOption {
	if cfg == nil {
		return nil
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil
	}
	return []api.ClientOption{api.WithBaseURL(strings.TrimSpace(cfg.BaseURL))}
}

func formatConfigLoadError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("load config: %w", err)
}
