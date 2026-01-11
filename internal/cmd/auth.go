package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/salmonumbrella/roam-cli/internal/api"
	"github.com/salmonumbrella/roam-cli/internal/auth"
	"github.com/salmonumbrella/roam-cli/internal/secrets"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	// defaultProfile is the profile name used for credentials
	defaultProfile = "default"
	// graphKeyPrefix is used for storing graph name in keyring
	graphKeyPrefix = "graph:"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication credentials",
	Long: `Manage authentication credentials for Roam Research API.

Credentials are stored securely in your system keychain (macOS Keychain,
Windows Credential Manager, or encrypted file on Linux).

Examples:
  roam auth login --token YOUR_API_TOKEN --graph your-graph-name
  roam auth login  # Interactive prompt for credentials
  roam auth status
  roam auth logout`,
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Store API credentials",
	Long: `Store API credentials for Roam Research.

By default, opens a browser-based setup flow for a guided authentication
experience. Credentials are stored securely in your system keychain.

To obtain an API token:
  1. Go to your Roam graph settings
  2. Navigate to the API Tokens section
  3. Generate a new API token

For encrypted graphs (local-first graphs that store data only on your device):
  - Use --encrypted-graph flag
  - Requires Roam desktop app to be running
  - The desktop app must have API access enabled

Examples:
  roam auth login                    # Browser-based setup (recommended)
  roam auth login --no-browser       # Terminal prompts instead
  roam auth login --token TOKEN --graph GRAPH  # Non-interactive
  roam auth login --encrypted-graph  # For encrypted graphs`,
	RunE: runLogin,
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear stored credentials",
	Long: `Clear stored API credentials from the system keychain.

This removes all stored authentication data for Roam Research.

Examples:
  roam auth logout`,
	RunE: runLogout,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current authentication status",
	Long: `Display the current authentication status.

Shows whether you are authenticated and which graph is configured.
Can optionally verify the credentials against the Roam API.

Examples:
  roam auth status
  roam auth status --verify  # Also verify credentials with API`,
	RunE: runStatus,
}

var (
	loginToken     string
	loginGraph     string
	encryptedGraph bool
	noBrowser      bool
	verifyAuth     bool
)

func init() {
	// Add auth subcommands
	authCmd.AddCommand(loginCmd)
	authCmd.AddCommand(logoutCmd)
	authCmd.AddCommand(statusCmd)

	// Add auth to root
	rootCmd.AddCommand(authCmd)

	// Login flags
	loginCmd.Flags().StringVar(&loginToken, "token", "", "API token")
	loginCmd.Flags().StringVar(&loginGraph, "graph", "", "Graph name")
	loginCmd.Flags().BoolVar(&encryptedGraph, "encrypted-graph", false, "Configure for encrypted graph (requires desktop app)")
	loginCmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Use terminal prompts instead of browser-based setup")

	// Status flags
	statusCmd.Flags().BoolVar(&verifyAuth, "verify", false, "Verify credentials with API")
}

func runLogin(cmd *cobra.Command, args []string) error {
	store, err := openSecretsStore()
	if err != nil {
		return fmt.Errorf("failed to open credential store: %w", err)
	}

	structured := structuredOutputRequested()

	// Check if we should use browser flow
	// Use browser by default if no credentials are provided via flags/env and not in structured output mode
	token := loginToken
	if token == "" {
		token = envGet("ROAM_API_TOKEN")
	}
	graph := loginGraph
	if graph == "" {
		graph = envGet("ROAM_GRAPH_NAME")
	}

	useBrowser := !noBrowser && token == "" && graph == "" && !structured
	if useBrowser {
		return runBrowserLogin(cmd.Context(), store)
	}

	// Fall back to terminal-based login
	return runTerminalLogin(cmd.Context(), store, token, graph, structured)
}

// runBrowserLogin performs browser-based authentication
func runBrowserLogin(ctx context.Context, store secrets.Store) error {
	// Create save function that stores credentials
	saveFunc := func(profile string, graphName string, tok secrets.Token) error {
		// Store the token
		if err := store.SetToken(profile, tok); err != nil {
			return fmt.Errorf("failed to store credentials: %w", err)
		}

		// Store the graph name
		graphTok := secrets.Token{
			Profile:      graphKeyPrefix + profile,
			RefreshToken: graphName,
			CreatedAt:    time.Now().UTC(),
		}
		if err := store.SetToken(graphKeyPrefix+profile, graphTok); err != nil {
			return fmt.Errorf("failed to store graph name: %w", err)
		}

		// Set as default account
		if err := store.SetDefaultAccount(profile); err != nil {
			return fmt.Errorf("failed to set default account: %w", err)
		}

		return nil
	}

	server, err := auth.NewSetupServer(defaultProfile, auth.WithSaveFunc(saveFunc))
	if err != nil {
		return fmt.Errorf("failed to create setup server: %w", err)
	}

	result, err := server.Start(ctx)
	if err != nil {
		return fmt.Errorf("browser setup failed: %w", err)
	}

	if result.Error != nil {
		return result.Error
	}

	fmt.Printf("\nAuthenticated successfully!\n")
	fmt.Printf("Graph: %s\n", result.GraphName)
	if result.Mode == secrets.ModeEncrypted {
		fmt.Println("Type: Encrypted (requires desktop app)")
	} else {
		fmt.Println("Type: Cloud")
	}
	fmt.Println("\nYou can now use roam commands without specifying --token or --graph flags.")

	return nil
}

// runTerminalLogin performs terminal-based authentication (original flow)
func runTerminalLogin(ctx context.Context, store secrets.Store, token, graph string, structured bool) error {
	// Get token from prompt if not provided
	if token == "" {
		var promptErr error
		token, promptErr = promptSecret(ctx, "Enter API token: ")
		if promptErr != nil {
			return fmt.Errorf("failed to read token: %w", promptErr)
		}
	}

	if token == "" {
		return fmt.Errorf("API token is required")
	}

	// Get graph name from prompt if not provided
	if graph == "" {
		var promptErr error
		graph, promptErr = promptString(ctx, "Enter graph name: ")
		if promptErr != nil {
			return fmt.Errorf("failed to read graph name: %w", promptErr)
		}
	}

	if graph == "" {
		return fmt.Errorf("graph name is required")
	}

	// For encrypted graphs, verify desktop app is running
	if encryptedGraph {
		portFile := os.ExpandEnv("$HOME/.roam-api-port")
		if _, err := os.Stat(portFile); os.IsNotExist(err) {
			return fmt.Errorf("encrypted graph requires Roam desktop app to be running\n\n" +
				"Please:\n" +
				"  1. Open the Roam desktop app\n" +
				"  2. Enable API access in settings\n" +
				"  3. Try again")
		}
		if !structured {
			fmt.Println("Desktop app detected.")
		}
	}

	// Verify credentials by making a test API call
	if !structured {
		fmt.Println("Verifying credentials...")
	}

	// Determine graph mode for client creation
	testMode := secrets.ModeCloud
	if encryptedGraph {
		testMode = secrets.ModeEncrypted
	}

	cfg, err := loadConfigFromFlag()
	if err != nil {
		return formatConfigLoadError(err)
	}
	testClient, err := newClientFromCredsFunc(graph, token, testMode, clientOptionsFromConfig(cfg)...)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}
	_, err = testClient.Query("[:find ?e :where [?e :node/title] :limit 1]")
	if err != nil {
		// Check if it's an authentication error
		if _, ok := err.(api.AuthenticationError); ok {
			return fmt.Errorf("authentication failed: invalid API token")
		}
		// Other errors might be acceptable (rate limit, etc.) during login
		if !structured {
			fmt.Printf("Warning: Could not verify credentials: %v\n", err)
			fmt.Println("Proceeding with credential storage...")
		}
	} else {
		if !structured {
			fmt.Println("Credentials verified successfully!")
		}
	}

	// Determine graph mode
	mode := secrets.ModeCloud
	if encryptedGraph {
		mode = secrets.ModeEncrypted
	}

	// Store the token
	tok := secrets.Token{
		Profile:      defaultProfile,
		RefreshToken: token, // Using RefreshToken field to store the API token
		Mode:         mode,
		CreatedAt:    time.Now().UTC(),
	}

	if err := store.SetToken(defaultProfile, tok); err != nil {
		return fmt.Errorf("failed to store credentials: %w", err)
	}

	// Store the graph name
	// We use the SetToken mechanism with a special profile for the graph name
	graphTok := secrets.Token{
		Profile:      graphKeyPrefix + defaultProfile,
		RefreshToken: graph, // Store graph name in RefreshToken field
		CreatedAt:    time.Now().UTC(),
	}

	if err := store.SetToken(graphKeyPrefix+defaultProfile, graphTok); err != nil {
		return fmt.Errorf("failed to store graph name: %w", err)
	}

	// Set as default account
	if err := store.SetDefaultAccount(defaultProfile); err != nil {
		return fmt.Errorf("failed to set default account: %w", err)
	}

	if structured {
		modeLabel := "cloud"
		if encryptedGraph {
			modeLabel = "encrypted"
		}
		return printStructured(map[string]interface{}{
			"status": "authenticated",
			"graph":  graph,
			"mode":   modeLabel,
		})
	}

	fmt.Printf("\nAuthenticated successfully!\n")
	fmt.Printf("Graph: %s\n", graph)
	if encryptedGraph {
		fmt.Println("Type: Encrypted (requires desktop app)")
	} else {
		fmt.Println("Type: Cloud")
	}
	fmt.Println("\nYou can now use roam commands without specifying --token or --graph flags.")

	return nil
}

func runLogout(cmd *cobra.Command, args []string) error {
	store, err := openSecretsStore()
	if err != nil {
		return fmt.Errorf("failed to open credential store: %w", err)
	}

	// Delete the token
	if err := store.DeleteToken(defaultProfile); err != nil {
		// Ignore "not found" errors
		if !strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("failed to remove credentials: %w", err)
		}
	}

	// Delete the graph name
	if err := store.DeleteToken(graphKeyPrefix + defaultProfile); err != nil {
		// Ignore "not found" errors
		if !strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("failed to remove graph name: %w", err)
		}
	}

	if structuredOutputRequested() {
		return printStructured(map[string]interface{}{
			"status": "logged_out",
		})
	}

	fmt.Println("Logged out successfully.")
	fmt.Println("Credentials have been removed from the system keychain.")

	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	store, err := openSecretsStore()
	if err != nil {
		return fmt.Errorf("failed to open credential store: %w", err)
	}

	// Check for stored token
	tok, err := store.GetToken(defaultProfile)
	if err != nil {
		if structuredOutputRequested() {
			return printStructured(map[string]interface{}{
				"authenticated": false,
			})
		}
		fmt.Println("Status: Not authenticated")
		fmt.Println("\nRun 'roam auth login' to authenticate.")
		return nil
	}

	// Check for stored graph name
	graphTok, graphErr := store.GetToken(graphKeyPrefix + defaultProfile)

	structured := structuredOutputRequested()
	var verified *bool
	var verifyError string

	// Optionally verify with API
	if verifyAuth {
		if !structured {
			fmt.Println("\nVerifying credentials with API...")
		}

		if graphErr != nil || graphTok.RefreshToken == "" {
			if structured {
				verifyError = "graph name not configured"
				val := false
				verified = &val
			} else {
				fmt.Println("Cannot verify: graph name not configured")
				return nil
			}
		} else {
			cfg, err := loadConfigFromFlag()
			if err != nil {
				return formatConfigLoadError(err)
			}
			testClient, err := newClientFromCredsFunc(graphTok.RefreshToken, tok.RefreshToken, tok.Mode, clientOptionsFromConfig(cfg)...)
			if err != nil {
				if structured {
					verifyError = err.Error()
					val := false
					verified = &val
				} else {
					fmt.Printf("Verification: FAILED - %v\n", err)
					return nil
				}
			} else {
				_, err = testClient.Query("[:find ?e :where [?e :node/title] :limit 1]")
				if err != nil {
					if _, ok := err.(api.AuthenticationError); ok {
						if structured {
							verifyError = "invalid or expired token"
							val := false
							verified = &val
						} else {
							fmt.Println("Verification: FAILED - Invalid or expired token")
							return nil
						}
					} else if structured {
						verifyError = err.Error()
						val := false
						verified = &val
					} else {
						fmt.Printf("Verification: FAILED - %v\n", err)
						return nil
					}
				} else if structured {
					val := true
					verified = &val
				} else {
					fmt.Println("Verification: OK - Credentials are valid")
				}
			}
		}
	}

	if structured {
		modeLabel := "cloud"
		if tok.Mode == secrets.ModeEncrypted {
			modeLabel = "encrypted"
		}

		result := map[string]interface{}{
			"authenticated":    true,
			"profile":          tok.Profile,
			"graph":            graphTok.RefreshToken,
			"graph_configured": graphErr == nil && graphTok.RefreshToken != "",
			"mode":             modeLabel,
		}
		if !tok.CreatedAt.IsZero() {
			result["authenticated_at"] = tok.CreatedAt.Format(time.RFC3339)
		}
		if tok.RefreshToken != "" {
			result["token_preview"] = maskToken(tok.RefreshToken)
		}
		if verifyAuth {
			result["verified"] = verified
			if verifyError != "" {
				result["verify_error"] = verifyError
			}
		}
		return printStructured(result)
	}

	fmt.Println("Status: Authenticated")
	fmt.Printf("Profile: %s\n", tok.Profile)

	if !tok.CreatedAt.IsZero() {
		fmt.Printf("Authenticated at: %s\n", tok.CreatedAt.Format(time.RFC3339))
	}

	if graphErr == nil && graphTok.RefreshToken != "" {
		fmt.Printf("Graph: %s\n", graphTok.RefreshToken)
	} else {
		fmt.Println("Graph: Not configured")
	}

	// Show graph type
	if tok.Mode == secrets.ModeEncrypted {
		fmt.Println("Type: Encrypted (requires desktop app)")
	} else {
		fmt.Println("Type: Cloud")
	}

	// Show token preview (masked)
	if tok.RefreshToken != "" {
		masked := maskToken(tok.RefreshToken)
		fmt.Printf("Token: %s\n", masked)
	}

	return nil
}

// promptString prompts for a string input
func promptString(ctx context.Context, prompt string) (string, error) {
	fmt.Fprint(stderrFromContext(ctx), prompt)
	reader := bufio.NewReader(stdinFromContext(ctx))
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

// promptSecret prompts for a secret input (no echo)
func promptSecret(ctx context.Context, prompt string) (string, error) {
	fmt.Fprint(stderrFromContext(ctx), prompt)

	in := stdinFromContext(ctx)
	if file, ok := in.(*os.File); ok {
		if term.IsTerminal(int(file.Fd())) {
			password, err := term.ReadPassword(int(file.Fd()))
			fmt.Fprintln(stderrFromContext(ctx))
			if err != nil {
				return "", err
			}
			return strings.TrimSpace(string(password)), nil
		}
	}

	// Fall back to regular input for non-terminal (e.g., piped input)
	reader := bufio.NewReader(in)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

// maskToken masks a token for display, showing only first and last 4 characters
func maskToken(token string) string {
	if len(token) <= 12 {
		return "****"
	}
	return token[:4] + "..." + token[len(token)-4:]
}
