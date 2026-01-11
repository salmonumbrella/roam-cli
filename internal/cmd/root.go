package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/salmonumbrella/roam-cli/internal/api"
	"github.com/salmonumbrella/roam-cli/internal/config"
	"github.com/salmonumbrella/roam-cli/internal/output"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	// Version is set at build time
	version = "dev"
	// Commit is set at build time
	commit = "none"
	// Date is set at build time
	date = "unknown"
)

// SetVersionInfo sets the version information from build flags
func SetVersionInfo(v, c, d string) {
	version = v
	commit = c
	date = d
}

// Global flags
var (
	graphName   string
	apiToken    string
	outputFmt   string
	outputType  output.Format
	debug       bool
	configFile  string
	useLocal    bool
	queryExpr   string
	queryFile   string
	errorFmt    string
	quietFlag   bool
	yesFlag     bool
	resultLimit int
	resultSort  string
	resultDesc  bool
)

// client is the shared API client
var client api.RoamAPI

var rootCmd = &cobra.Command{
	Use:   "roam",
	Short: "CLI for Roam Research",
	Long: `roam is a command-line interface for interacting with Roam Research.

It provides commands for managing your Roam graphs, pages, and blocks
from the terminal.

Environment Variables:
  ROAM_API_TOKEN   API token for authentication
  ROAM_GRAPH_NAME  Default graph name`,
	Version: version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceErrors = true

		skipConfigLoad := cmd.Name() == "config" || (cmd.Parent() != nil && cmd.Parent().Name() == "config")
		var cfg *config.Config
		if !skipConfigLoad {
			loadedCfg, err := loadConfigFromFlag()
			if err != nil {
				return formatConfigLoadError(err)
			}
			cfg = loadedCfg
		}

		// Output format selection: --output > config > default
		formatStr := outputFmt
		if !flagChanged(cmd, "output") && !flagChanged(cmd, "format") && cfg != nil && strings.TrimSpace(cfg.OutputFormat) != "" {
			formatStr = strings.TrimSpace(cfg.OutputFormat)
		}
		if !flagChanged(cmd, "output") && !flagChanged(cmd, "format") && !isTerminal(cmd.OutOrStdout()) {
			formatStr = "json"
		}
		format, err := output.ParseFormat(formatStr)
		if err != nil {
			return err
		}
		outputType = format
		outputFmt = string(format)

		// jq query
		if queryExpr != "" && queryFile != "" {
			return fmt.Errorf("use only one of --query or --query-file")
		}
		if queryFile != "" {
			loaded, err := readInputSource(queryFile, cmd.InOrStdin())
			if err != nil {
				return err
			}
			queryExpr = loaded
		}

		// Default quiet mode for non-interactive structured output
		if !flagChanged(cmd, "quiet") && !isTerminal(cmd.OutOrStdout()) && output.IsStructured(outputType) {
			quietFlag = true
		}

		ctx := cmd.Context()
		ctx = withIO(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		ctx = output.WithFormat(ctx, outputType)
		ctx = output.WithQuery(ctx, queryExpr)
		ctx = output.WithYes(ctx, yesFlag)
		ctx = output.WithLimit(ctx, resultLimit)
		ctx = output.WithSort(ctx, resultSort, resultDesc)
		ctx = output.WithQuiet(ctx, quietFlag)
		ctx = WithErrorFormat(ctx, errorFmt)
		cmd.SetContext(ctx)

		if err := validateErrorFormat(errorFmt); err != nil {
			return err
		}
		if effectiveErrorFormat(ctx) != "text" {
			cmd.SilenceUsage = true
		}

		// Skip client initialization for auth/config/help/completion commands.
		if cmd.Name() == "login" || cmd.Name() == "logout" || cmd.Name() == "status" ||
			cmd.Name() == "config" || cmd.Name() == "completion" || cmd.Name() == "help" ||
			cmd.Parent() != nil && cmd.Parent().Name() == "config" {
			return nil
		}
		// Skip client initialization for local subcommands (they use Local API without token)
		if cmd.Name() == "local" || (cmd.Parent() != nil && cmd.Parent().Name() == "local") {
			return nil
		}

		// Resolve credentials with consistent precedence.
		token, graph, mode, err := resolveCredentials(cmd, cfg)
		if err != nil {
			return err
		}
		apiToken = token
		graphName = graph

		// Validate required credentials
		// Token not required for encrypted mode (local API doesn't need authentication)
		if apiToken == "" && mode != "encrypted" {
			return fmt.Errorf("API token required. Set ROAM_API_TOKEN or use --token flag.\nRun 'roam auth login' to configure authentication.")
		}
		if graphName == "" {
			return fmt.Errorf("Graph name required. Set ROAM_GRAPH_NAME or use --graph flag.")
		}

		// Initialize client using factory
		client, err = newClientFromCredsFunc(graphName, apiToken, mode, clientOptionsFromConfig(cfg)...)
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		// Enable debug if requested (only for cloud client)
		if debug {
			if cloudClient, ok := client.(*api.Client); ok {
				cloudClient.SetDebug(true)
			}
		}
		return nil
	},
}

// Execute runs the root command
func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		printCommandError(rootCmd.Context(), err)
		return err
	}
	return nil
}

// GetClient returns the initialized API client
func GetClient() api.RoamAPI {
	return client
}

// GetOutputFormat returns the configured output format
func GetOutputFormat() output.Format {
	if outputType != "" {
		return outputType
	}
	parsed, err := output.ParseFormat(outputFmt)
	if err != nil {
		return output.FormatText
	}
	return parsed
}

// GetOutputFormatString returns the output format as a string.
func GetOutputFormatString() string {
	if outputType != "" {
		return string(outputType)
	}
	return outputFmt
}

func init() {
	rootCmd.SetVersionTemplate(fmt.Sprintf("roam version %s (commit: %s, built: %s)\n", version, commit, date))

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&graphName, "graph", "g", "", "Graph name (env: ROAM_GRAPH_NAME)")
	rootCmd.PersistentFlags().StringVar(&apiToken, "token", "", "API token (env: ROAM_API_TOKEN)")
	rootCmd.PersistentFlags().StringVarP(&outputFmt, "output", "o", "text", "Output format (text|json|ndjson|table|yaml)")
	rootCmd.PersistentFlags().StringVar(&outputFmt, "format", "text", "Alias for --output")
	rootCmd.PersistentFlags().StringVar(&queryExpr, "query", "", "jq expression to filter JSON output")
	rootCmd.PersistentFlags().StringVar(&queryFile, "query-file", "", "Read jq expression from file (use - for stdin)")
	rootCmd.PersistentFlags().StringVar(&errorFmt, "error-format", "auto", "Error output format (auto|text|json|yaml)")
	rootCmd.PersistentFlags().BoolVar(&quietFlag, "quiet", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVarP(&yesFlag, "yes", "y", false, "Skip confirmation prompts (for automation)")
	rootCmd.PersistentFlags().BoolVar(&yesFlag, "no-input", false, "Alias for --yes (non-interactive)")
	rootCmd.PersistentFlags().IntVar(&resultLimit, "result-limit", 0, "Limit number of results in output (0 = unlimited)")
	rootCmd.PersistentFlags().StringVar(&resultSort, "result-sort-by", "", "Sort output results by field")
	rootCmd.PersistentFlags().BoolVar(&resultDesc, "result-desc", false, "Sort output results in descending order")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug output")
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "Config file (default: ~/.config/roam/config.yaml)")
	rootCmd.PersistentFlags().BoolVar(&useLocal, "local", false, "Use Local API (requires Roam desktop app)")
}

func isTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}
