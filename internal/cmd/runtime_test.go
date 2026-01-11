package cmd

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"

	"github.com/salmonumbrella/roam-cli/internal/api"
	"github.com/salmonumbrella/roam-cli/internal/config"
	"github.com/salmonumbrella/roam-cli/internal/secrets"
)

func TestFlagChanged_NilCmd(t *testing.T) {
	if flagChanged(nil, "output") {
		t.Error("expected false for nil cmd")
	}
}

func TestFlagChanged_UnsetFlag(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("output", "text", "")

	if flagChanged(cmd, "output") {
		t.Error("expected false for unset flag")
	}
}

func TestFlagChanged_SetFlag(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("output", "text", "")
	if err := cmd.Flags().Set("output", "json"); err != nil {
		t.Fatalf("failed to set flag: %v", err)
	}

	if !flagChanged(cmd, "output") {
		t.Error("expected true for set flag")
	}
}

func TestFlagChanged_InheritedFlag(t *testing.T) {
	parent := &cobra.Command{}
	parent.PersistentFlags().String("format", "text", "")

	child := &cobra.Command{}
	parent.AddCommand(child)

	// Inherit flags
	if err := parent.PersistentFlags().Set("format", "json"); err != nil {
		t.Fatalf("failed to set flag: %v", err)
	}

	if !flagChanged(child, "format") {
		t.Error("expected true for inherited flag")
	}
}

func TestApplyOutputFormat_NilConfig(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("output", "text", "")

	prevFmt := outputFmt
	outputFmt = "text"
	defer func() { outputFmt = prevFmt }()

	applyOutputFormat(cmd, nil)

	if outputFmt != "text" {
		t.Errorf("expected outputFmt unchanged, got %q", outputFmt)
	}
}

func TestApplyOutputFormat_EmptyConfigFormat(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("output", "text", "")

	prevFmt := outputFmt
	outputFmt = "text"
	defer func() { outputFmt = prevFmt }()

	cfg := &config.Config{OutputFormat: ""}
	applyOutputFormat(cmd, cfg)

	if outputFmt != "text" {
		t.Errorf("expected outputFmt unchanged, got %q", outputFmt)
	}
}

func TestApplyOutputFormat_AppliesConfig(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("output", "text", "")
	cmd.Flags().String("format", "text", "")

	prevFmt := outputFmt
	outputFmt = "text"
	defer func() { outputFmt = prevFmt }()

	cfg := &config.Config{OutputFormat: "json"}
	applyOutputFormat(cmd, cfg)

	if outputFmt != "json" {
		t.Errorf("expected outputFmt = 'json', got %q", outputFmt)
	}
}

func TestApplyOutputFormat_FlagOverridesConfig(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("output", "text", "")
	if err := cmd.Flags().Set("output", "yaml"); err != nil {
		t.Fatalf("failed to set flag: %v", err)
	}

	prevFmt := outputFmt
	outputFmt = "yaml"
	defer func() { outputFmt = prevFmt }()

	cfg := &config.Config{OutputFormat: "json"}
	applyOutputFormat(cmd, cfg)

	// Flag takes precedence, so format should remain yaml
	if outputFmt != "yaml" {
		t.Errorf("expected outputFmt = 'yaml' (flag override), got %q", outputFmt)
	}
}

func TestClientOptionsFromConfig_Nil(t *testing.T) {
	opts := clientOptionsFromConfig(nil)
	if opts != nil {
		t.Errorf("expected nil for nil config, got %v", opts)
	}
}

func TestClientOptionsFromConfig_EmptyBaseURL(t *testing.T) {
	cfg := &config.Config{BaseURL: ""}
	opts := clientOptionsFromConfig(cfg)
	if opts != nil {
		t.Errorf("expected nil for empty base URL, got %v", opts)
	}
}

func TestClientOptionsFromConfig_WhitespaceBaseURL(t *testing.T) {
	cfg := &config.Config{BaseURL: "   "}
	opts := clientOptionsFromConfig(cfg)
	if opts != nil {
		t.Errorf("expected nil for whitespace-only base URL, got %v", opts)
	}
}

func TestClientOptionsFromConfig_WithBaseURL(t *testing.T) {
	cfg := &config.Config{BaseURL: "https://api.example.com"}
	opts := clientOptionsFromConfig(cfg)
	if len(opts) != 1 {
		t.Errorf("expected 1 option, got %d", len(opts))
	}
}

func TestFormatConfigLoadError_Nil(t *testing.T) {
	err := formatConfigLoadError(nil)
	if err != nil {
		t.Errorf("expected nil for nil input, got %v", err)
	}
}

func TestFormatConfigLoadError_WrapsError(t *testing.T) {
	original := errors.New("file not found")
	err := formatConfigLoadError(original)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "load config: file not found" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// Mock secrets store for testing resolveCredentials
type mockSecretsStore struct {
	tokens map[string]secrets.Token
	err    error
}

func (m *mockSecretsStore) Keys() ([]string, error) { return nil, nil }

func (m *mockSecretsStore) GetToken(profile string) (secrets.Token, error) {
	if m.err != nil {
		return secrets.Token{}, m.err
	}
	if tok, ok := m.tokens[profile]; ok {
		return tok, nil
	}
	return secrets.Token{}, errors.New("token not found")
}

func (m *mockSecretsStore) SetToken(string, secrets.Token) error { return nil }
func (m *mockSecretsStore) DeleteToken(string) error             { return nil }
func (m *mockSecretsStore) ListTokens() ([]secrets.Token, error) { return nil, nil }
func (m *mockSecretsStore) GetDefaultAccount() (string, error)   { return "", nil }
func (m *mockSecretsStore) SetDefaultAccount(string) error       { return nil }

func TestResolveCredentials_FromEnv(t *testing.T) {
	// Save and restore global state
	prevEnv := envGet
	prevStore := openSecretsStore
	prevToken := apiToken
	prevGraph := graphName
	prevLocal := useLocal
	defer func() {
		envGet = prevEnv
		openSecretsStore = prevStore
		apiToken = prevToken
		graphName = prevGraph
		useLocal = prevLocal
	}()

	// Set up mocks
	envGet = func(key string) string {
		switch key {
		case "ROAM_API_TOKEN":
			return "env-token"
		case "ROAM_GRAPH_NAME":
			return "env-graph"
		}
		return ""
	}
	openSecretsStore = func() (secrets.Store, error) {
		return nil, errors.New("keyring unavailable")
	}
	apiToken = ""
	graphName = ""
	useLocal = false

	cmd := &cobra.Command{}
	cmd.Flags().String("token", "", "")
	cmd.Flags().String("graph", "", "")

	token, graph, mode, err := resolveCredentials(cmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if token != "env-token" {
		t.Errorf("expected token = 'env-token', got %q", token)
	}
	if graph != "env-graph" {
		t.Errorf("expected graph = 'env-graph', got %q", graph)
	}
	if mode != "" {
		t.Errorf("expected mode = '', got %q", mode)
	}
}

func TestResolveCredentials_FromKeyring(t *testing.T) {
	// Save and restore global state
	prevEnv := envGet
	prevStore := openSecretsStore
	prevToken := apiToken
	prevGraph := graphName
	prevLocal := useLocal
	defer func() {
		envGet = prevEnv
		openSecretsStore = prevStore
		apiToken = prevToken
		graphName = prevGraph
		useLocal = prevLocal
	}()

	// Set up mocks - empty env
	envGet = func(key string) string { return "" }
	openSecretsStore = func() (secrets.Store, error) {
		return &mockSecretsStore{
			tokens: map[string]secrets.Token{
				defaultProfile:                  {RefreshToken: "keyring-token", Mode: "cloud"},
				graphKeyPrefix + defaultProfile: {RefreshToken: "keyring-graph"},
			},
		}, nil
	}
	apiToken = ""
	graphName = ""
	useLocal = false

	cmd := &cobra.Command{}
	cmd.Flags().String("token", "", "")
	cmd.Flags().String("graph", "", "")

	token, graph, mode, err := resolveCredentials(cmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if token != "keyring-token" {
		t.Errorf("expected token = 'keyring-token', got %q", token)
	}
	if graph != "keyring-graph" {
		t.Errorf("expected graph = 'keyring-graph', got %q", graph)
	}
	if mode != "cloud" {
		t.Errorf("expected mode = 'cloud', got %q", mode)
	}
}

func TestResolveCredentials_FromConfig(t *testing.T) {
	// Save and restore global state
	prevEnv := envGet
	prevStore := openSecretsStore
	prevToken := apiToken
	prevGraph := graphName
	prevLocal := useLocal
	defer func() {
		envGet = prevEnv
		openSecretsStore = prevStore
		apiToken = prevToken
		graphName = prevGraph
		useLocal = prevLocal
	}()

	// Set up mocks - empty env and keyring
	envGet = func(key string) string { return "" }
	openSecretsStore = func() (secrets.Store, error) {
		return nil, errors.New("keyring unavailable")
	}
	apiToken = ""
	graphName = ""
	useLocal = false

	cmd := &cobra.Command{}
	cmd.Flags().String("token", "", "")
	cmd.Flags().String("graph", "", "")

	cfg := &config.Config{
		Token:     "config-token",
		GraphName: "config-graph",
	}

	token, graph, mode, err := resolveCredentials(cmd, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if token != "config-token" {
		t.Errorf("expected token = 'config-token', got %q", token)
	}
	if graph != "config-graph" {
		t.Errorf("expected graph = 'config-graph', got %q", graph)
	}
	if mode != "" {
		t.Errorf("expected mode = '', got %q", mode)
	}
}

func TestResolveCredentials_LocalFlag(t *testing.T) {
	// Save and restore global state
	prevEnv := envGet
	prevStore := openSecretsStore
	prevToken := apiToken
	prevGraph := graphName
	prevLocal := useLocal
	defer func() {
		envGet = prevEnv
		openSecretsStore = prevStore
		apiToken = prevToken
		graphName = prevGraph
		useLocal = prevLocal
	}()

	envGet = func(key string) string {
		if key == "ROAM_GRAPH_NAME" {
			return "my-graph"
		}
		return ""
	}
	openSecretsStore = func() (secrets.Store, error) {
		return nil, errors.New("keyring unavailable")
	}
	apiToken = ""
	graphName = ""
	useLocal = true // --local flag set

	cmd := &cobra.Command{}
	cmd.Flags().String("token", "", "")
	cmd.Flags().String("graph", "", "")

	_, _, mode, err := resolveCredentials(cmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mode != secrets.ModeEncrypted {
		t.Errorf("expected mode = %q, got %q", secrets.ModeEncrypted, mode)
	}
}

func TestResolveCredentials_FlagsPrecedence(t *testing.T) {
	// Save and restore global state
	prevEnv := envGet
	prevStore := openSecretsStore
	prevToken := apiToken
	prevGraph := graphName
	prevLocal := useLocal
	defer func() {
		envGet = prevEnv
		openSecretsStore = prevStore
		apiToken = prevToken
		graphName = prevGraph
		useLocal = prevLocal
	}()

	// Set up mocks - everything has values
	envGet = func(key string) string {
		switch key {
		case "ROAM_API_TOKEN":
			return "env-token"
		case "ROAM_GRAPH_NAME":
			return "env-graph"
		}
		return ""
	}
	openSecretsStore = func() (secrets.Store, error) {
		return &mockSecretsStore{
			tokens: map[string]secrets.Token{
				defaultProfile:                  {RefreshToken: "keyring-token"},
				graphKeyPrefix + defaultProfile: {RefreshToken: "keyring-graph"},
			},
		}, nil
	}

	// Set global vars as if flags were set
	apiToken = "flag-token"
	graphName = "flag-graph"
	useLocal = false

	cmd := &cobra.Command{}
	cmd.Flags().String("token", "", "")
	cmd.Flags().String("graph", "", "")
	// Mark flags as changed
	if err := cmd.Flags().Set("token", "flag-token"); err != nil {
		t.Fatalf("failed to set token flag: %v", err)
	}
	if err := cmd.Flags().Set("graph", "flag-graph"); err != nil {
		t.Fatalf("failed to set graph flag: %v", err)
	}

	cfg := &config.Config{
		Token:     "config-token",
		GraphName: "config-graph",
	}

	token, graph, _, err := resolveCredentials(cmd, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Flags should take precedence
	if token != "flag-token" {
		t.Errorf("expected token = 'flag-token', got %q", token)
	}
	if graph != "flag-graph" {
		t.Errorf("expected graph = 'flag-graph', got %q", graph)
	}
}

// TestResolveCredentials_FullPrecedenceChain verifies the complete precedence:
// flags > env > keyring > config
// Each level is tested by having all sources provide values, then removing
// the higher-precedence sources one by one.
//
// Mode behavior note:
// - Only the keyring stores mode ("cloud" or "encrypted") alongside credentials
// - Flags, env vars, and config files only provide token/graph, not mode
// - The --local flag sets mode separately (tested in TestResolveCredentials_LocalFlag)
// - When keyring is skipped (token found in flags/env), mode remains empty unless --local is set
func TestResolveCredentials_FullPrecedenceChain(t *testing.T) {
	tests := []struct {
		name       string
		flagSet    bool
		envSet     bool
		keyringSet bool
		configSet  bool
		wantToken  string
		wantGraph  string
		wantMode   string // Only keyring provides mode; flags/env/config don't store it
	}{
		{
			name:       "all sources set - flags win",
			flagSet:    true,
			envSet:     true,
			keyringSet: true,
			configSet:  true,
			wantToken:  "flag-token",
			wantGraph:  "flag-graph",
			wantMode:   "", // Keyring not consulted when flags provide token/graph
		},
		{
			name:       "no flags - env wins",
			flagSet:    false,
			envSet:     true,
			keyringSet: true,
			configSet:  true,
			wantToken:  "env-token",
			wantGraph:  "env-graph",
			wantMode:   "", // Keyring not consulted when env provides token/graph
		},
		{
			name:       "no flags or env - keyring wins",
			flagSet:    false,
			envSet:     false,
			keyringSet: true,
			configSet:  true,
			wantToken:  "keyring-token",
			wantGraph:  "keyring-graph",
			wantMode:   "cloud", // Keyring is the ONLY source that provides mode
		},
		{
			name:       "only config - config wins",
			flagSet:    false,
			envSet:     false,
			keyringSet: false,
			configSet:  true,
			wantToken:  "config-token",
			wantGraph:  "config-graph",
			wantMode:   "", // Config file doesn't store mode
		},
		{
			name:       "nothing set - empty",
			flagSet:    false,
			envSet:     false,
			keyringSet: false,
			configSet:  false,
			wantToken:  "",
			wantGraph:  "",
			wantMode:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore global state
			prevEnv := envGet
			prevStore := openSecretsStore
			prevToken := apiToken
			prevGraph := graphName
			prevLocal := useLocal
			defer func() {
				envGet = prevEnv
				openSecretsStore = prevStore
				apiToken = prevToken
				graphName = prevGraph
				useLocal = prevLocal
			}()

			// Set up env mock
			envGet = func(key string) string {
				if !tt.envSet {
					return ""
				}
				switch key {
				case "ROAM_API_TOKEN":
					return "env-token"
				case "ROAM_GRAPH_NAME":
					return "env-graph"
				}
				return ""
			}

			// Set up keyring mock
			openSecretsStore = func() (secrets.Store, error) {
				if !tt.keyringSet {
					return nil, errors.New("keyring unavailable")
				}
				return &mockSecretsStore{
					tokens: map[string]secrets.Token{
						defaultProfile:                  {RefreshToken: "keyring-token", Mode: "cloud"},
						graphKeyPrefix + defaultProfile: {RefreshToken: "keyring-graph"},
					},
				}, nil
			}

			// Set up flags
			apiToken = ""
			graphName = ""
			useLocal = false

			cmd := &cobra.Command{}
			cmd.Flags().String("token", "", "")
			cmd.Flags().String("graph", "", "")

			if tt.flagSet {
				apiToken = "flag-token"
				graphName = "flag-graph"
				if err := cmd.Flags().Set("token", "flag-token"); err != nil {
					t.Fatalf("failed to set token flag: %v", err)
				}
				if err := cmd.Flags().Set("graph", "flag-graph"); err != nil {
					t.Fatalf("failed to set graph flag: %v", err)
				}
			}

			// Set up config
			var cfg *config.Config
			if tt.configSet {
				cfg = &config.Config{
					Token:     "config-token",
					GraphName: "config-graph",
				}
			}

			token, graph, mode, err := resolveCredentials(cmd, cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if token != tt.wantToken {
				t.Errorf("token = %q, want %q", token, tt.wantToken)
			}
			if graph != tt.wantGraph {
				t.Errorf("graph = %q, want %q", graph, tt.wantGraph)
			}
			if mode != tt.wantMode {
				t.Errorf("mode = %q, want %q", mode, tt.wantMode)
			}
		})
	}
}

// Verify the mock implements the interface
var (
	_ secrets.Store    = (*mockSecretsStore)(nil)
	_ api.ClientOption = api.WithBaseURL("")
)
