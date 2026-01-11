package secrets

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/99designs/keyring"
	"golang.org/x/term"

	"github.com/salmonumbrella/roam-cli/internal/config"
)

// Store defines the interface for credential storage
type Store interface {
	Keys() ([]string, error)
	SetToken(profile string, tok Token) error
	GetToken(profile string) (Token, error)
	DeleteToken(profile string) error
	ListTokens() ([]Token, error)
	GetDefaultAccount() (string, error)
	SetDefaultAccount(profile string) error
}

// KeyringStore implements Store using the system keyring
type KeyringStore struct {
	ring keyring.Keyring
}

// Token represents stored OAuth credentials
type Token struct {
	Profile      string    `json:"profile"`
	AccessToken  string    `json:"-"`
	RefreshToken string    `json:"-"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	Scopes       []string  `json:"scopes,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	Mode         string    `json:"mode,omitempty"` // "cloud" or "encrypted"
}

// Graph mode constants
const (
	ModeCloud     = "cloud"     // Cloud-hosted Roam graph (default)
	ModeEncrypted = "encrypted" // Locally encrypted Roam graph
)

const (
	// Environment variable names for keyring configuration
	keyringPasswordEnv = "ROAM_KEYRING_PASSWORD" //nolint:gosec // env var name, not a credential
	keyringBackendEnv  = "ROAM_KEYRING_BACKEND"  //nolint:gosec // env var name, not a credential
)

var (
	errMissingProfile        = errors.New("missing profile")
	errMissingRefreshToken   = errors.New("missing refresh token")
	errNoTTY                 = errors.New("no TTY available for keyring file backend password prompt")
	errInvalidKeyringBackend = errors.New("invalid keyring backend")
	errKeyringTimeout        = errors.New("keyring connection timed out")

	// Hook for testing
	keyringOpenFunc = keyring.Open
)

const keyringOpenTimeout = 5 * time.Second

// KeyringBackendInfo contains keyring backend configuration
type KeyringBackendInfo struct {
	Value  string
	Source string
}

const (
	keyringBackendSourceEnv     = "env"
	keyringBackendSourceConfig  = "config"
	keyringBackendSourceDefault = "default"
)

// ResolveKeyringBackendInfo determines which keyring backend to use
// Priority: environment variable > config file > default ("auto")
func ResolveKeyringBackendInfo() (KeyringBackendInfo, error) {
	// Check environment variable first
	if v := strings.TrimSpace(os.Getenv(keyringBackendEnv)); v != "" {
		return KeyringBackendInfo{Value: strings.ToLower(v), Source: keyringBackendSourceEnv}, nil
	}

	// Check config file
	cfg, err := config.ReadConfig()
	if err != nil {
		return KeyringBackendInfo{}, fmt.Errorf("read config: %w", err)
	}

	if cfg.KeyringBackend != "" {
		return KeyringBackendInfo{Value: cfg.KeyringBackend, Source: keyringBackendSourceConfig}, nil
	}

	// Default to auto (try keychain, fall back to file)
	return KeyringBackendInfo{Value: "auto", Source: keyringBackendSourceDefault}, nil
}

// allowedBackends returns the keyring backends to try based on configuration
func allowedBackends(info KeyringBackendInfo) ([]keyring.BackendType, error) {
	switch info.Value {
	case "", "auto":
		// nil means try all available backends
		return nil, nil
	case "keychain":
		return []keyring.BackendType{keyring.KeychainBackend}, nil
	case "file":
		return []keyring.BackendType{keyring.FileBackend}, nil
	default:
		return nil, fmt.Errorf("%w: %q (expected auto, keychain, or file)", errInvalidKeyringBackend, info.Value)
	}
}

// shouldForceFileBackend returns true if file backend should be forced.
func shouldForceFileBackend(goos string, backendInfo KeyringBackendInfo, dbusAddr string) bool {
	return goos == "linux" && backendInfo.Value == "auto" && dbusAddr == ""
}

// shouldUseKeyringTimeout returns true if keyring.Open should use a timeout.
func shouldUseKeyringTimeout(goos string, backendInfo KeyringBackendInfo, dbusAddr string) bool {
	return goos == "linux" && backendInfo.Value == "auto" && dbusAddr != ""
}

type keyringResult struct {
	ring keyring.Keyring
	err  error
}

// openKeyringWithTimeout wraps keyring.Open with a timeout.
//
// NOTE: When timeout occurs, the goroutine calling keyringOpenFunc is abandoned
// since keyring.Open does not accept a context for cancellation. This may cause
// a goroutine leak if the keyring eventually opens after timeout. This is acceptable
// for a CLI tool that exits quickly after each command.
func openKeyringWithTimeout(cfg keyring.Config, timeout time.Duration) (keyring.Keyring, error) {
	ch := make(chan keyringResult, 1)

	go func() {
		ring, err := keyringOpenFunc(cfg)
		ch <- keyringResult{ring, err}
	}()

	select {
	case res := <-ch:
		if res.err != nil {
			return nil, fmt.Errorf("open keyring: %w", res.err)
		}
		return res.ring, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("%w after %v (D-Bus SecretService may be unresponsive); "+
			"set ROAM_KEYRING_BACKEND=file and ROAM_KEYRING_PASSWORD=<password> to use encrypted file storage",
			errKeyringTimeout, timeout)
	}
}

// fileKeyringPasswordFuncFrom creates a password prompt function for file-based keyring
func fileKeyringPasswordFuncFrom(password string, isTTY bool) keyring.PromptFunc {
	if password != "" {
		return keyring.FixedStringPrompt(password)
	}

	if isTTY {
		return keyring.TerminalPrompt
	}

	return func(_ string) (string, error) {
		return "", fmt.Errorf("%w; set %s", errNoTTY, keyringPasswordEnv)
	}
}

// fileKeyringPasswordFunc returns the password prompt function for file-based keyring
func fileKeyringPasswordFunc() keyring.PromptFunc {
	return fileKeyringPasswordFuncFrom(os.Getenv(keyringPasswordEnv), term.IsTerminal(int(os.Stdin.Fd())))
}

// OpenDefault opens the default credential store
func OpenDefault() (Store, error) {
	keyringDir, err := config.EnsureKeyringDir()
	if err != nil {
		return nil, fmt.Errorf("ensure keyring dir: %w", err)
	}

	backendInfo, err := ResolveKeyringBackendInfo()
	if err != nil {
		return nil, err
	}

	// Pre-flight: ensure keychain is accessible for non-file backends
	if backendInfo.Value != "file" {
		if err := EnsureKeychainAccess(); err != nil {
			return nil, fmt.Errorf("keychain access: %w", err)
		}
	}

	backends, err := allowedBackends(backendInfo)
	if err != nil {
		return nil, err
	}

	dbusAddr := os.Getenv("DBUS_SESSION_BUS_ADDRESS")

	// On Linux with "auto" backend and no D-Bus session, force file backend
	if shouldForceFileBackend(runtime.GOOS, backendInfo, dbusAddr) {
		backends = []keyring.BackendType{keyring.FileBackend}
	}

	cfg := keyring.Config{
		ServiceName:              config.AppName,
		KeychainTrustApplication: runtime.GOOS == "darwin",
		AllowedBackends:          backends,
		FileDir:                  keyringDir,
		FilePasswordFunc:         fileKeyringPasswordFunc(),
	}

	// On Linux with D-Bus present, use timeout to prevent indefinite hangs
	if shouldUseKeyringTimeout(runtime.GOOS, backendInfo, dbusAddr) {
		ring, err := openKeyringWithTimeout(cfg, keyringOpenTimeout)
		if err != nil {
			return nil, err
		}
		return &KeyringStore{ring: ring}, nil
	}

	ring, err := keyringOpenFunc(cfg)
	if err != nil {
		return nil, fmt.Errorf("open keyring: %w", err)
	}

	return &KeyringStore{ring: ring}, nil
}

// Keys returns all keyring keys
func (s *KeyringStore) Keys() ([]string, error) {
	keys, err := s.ring.Keys()
	if err != nil {
		return nil, fmt.Errorf("list keyring keys: %w", err)
	}

	return keys, nil
}

// storedToken is the JSON structure stored in the keyring
type storedToken struct {
	RefreshToken string    `json:"refresh_token"`
	Scopes       []string  `json:"scopes,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	Mode         string    `json:"mode,omitempty"`
}

// storedAccessToken is stored separately for short-lived access tokens
type storedAccessToken struct {
	AccessToken string    `json:"access_token"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
}

// SetToken stores OAuth tokens for a profile
// Access token and refresh token are stored separately
func (s *KeyringStore) SetToken(profile string, tok Token) error {
	profile = normalize(profile)
	if profile == "" {
		return errMissingProfile
	}

	if tok.RefreshToken == "" {
		return errMissingRefreshToken
	}

	if tok.CreatedAt.IsZero() {
		tok.CreatedAt = time.Now().UTC()
	}

	// Default to cloud mode for backwards compatibility
	mode := tok.Mode
	if mode == "" {
		mode = ModeCloud
	}

	// Store refresh token
	refreshPayload, err := json.Marshal(storedToken{
		RefreshToken: tok.RefreshToken,
		Scopes:       tok.Scopes,
		CreatedAt:    tok.CreatedAt,
		Mode:         mode,
	})
	if err != nil {
		return fmt.Errorf("encode refresh token: %w", err)
	}

	if err := s.ring.Set(keyring.Item{
		Key:  refreshTokenKey(profile),
		Data: refreshPayload,
	}); err != nil {
		return wrapKeychainError(fmt.Errorf("store refresh token: %w", err))
	}

	// Store access token if present
	if tok.AccessToken != "" {
		accessPayload, err := json.Marshal(storedAccessToken{
			AccessToken: tok.AccessToken,
			ExpiresAt:   tok.ExpiresAt,
		})
		if err != nil {
			return fmt.Errorf("encode access token: %w", err)
		}

		if err := s.ring.Set(keyring.Item{
			Key:  accessTokenKey(profile),
			Data: accessPayload,
		}); err != nil {
			// Non-fatal: refresh token is already stored
			// Access token can be regenerated
			_ = err
		}
	}

	return nil
}

// GetToken retrieves OAuth tokens for a profile
func (s *KeyringStore) GetToken(profile string) (Token, error) {
	profile = normalize(profile)
	if profile == "" {
		return Token{}, errMissingProfile
	}

	// Get refresh token (required)
	refreshItem, err := s.ring.Get(refreshTokenKey(profile))
	if err != nil {
		return Token{}, fmt.Errorf("read refresh token: %w", err)
	}

	var st storedToken
	if err := json.Unmarshal(refreshItem.Data, &st); err != nil {
		return Token{}, fmt.Errorf("decode refresh token: %w", err)
	}

	// Default to cloud mode for backwards compatibility with existing tokens
	mode := st.Mode
	if mode == "" {
		mode = ModeCloud
	}

	tok := Token{
		Profile:      profile,
		RefreshToken: st.RefreshToken,
		Scopes:       st.Scopes,
		CreatedAt:    st.CreatedAt,
		Mode:         mode,
	}

	// Try to get access token (optional)
	if accessItem, err := s.ring.Get(accessTokenKey(profile)); err == nil {
		var sat storedAccessToken
		if err := json.Unmarshal(accessItem.Data, &sat); err == nil {
			tok.AccessToken = sat.AccessToken
			tok.ExpiresAt = sat.ExpiresAt
		}
	}

	return tok, nil
}

// DeleteToken removes all tokens for a profile
func (s *KeyringStore) DeleteToken(profile string) error {
	profile = normalize(profile)
	if profile == "" {
		return errMissingProfile
	}

	// Remove refresh token
	if err := s.ring.Remove(refreshTokenKey(profile)); err != nil {
		return fmt.Errorf("delete refresh token: %w", err)
	}

	// Remove access token (ignore errors - it may not exist)
	_ = s.ring.Remove(accessTokenKey(profile))

	return nil
}

// ListTokens returns all stored tokens
func (s *KeyringStore) ListTokens() ([]Token, error) {
	keys, err := s.Keys()
	if err != nil {
		return nil, fmt.Errorf("list tokens: %w", err)
	}

	out := make([]Token, 0)
	seen := make(map[string]bool)

	for _, k := range keys {
		profile, ok := parseRefreshTokenKey(k)
		if !ok {
			continue
		}

		if seen[profile] {
			continue
		}
		seen[profile] = true

		tok, err := s.GetToken(profile)
		if err != nil {
			return nil, fmt.Errorf("read token for %s: %w", profile, err)
		}

		out = append(out, tok)
	}

	return out, nil
}

// GetDefaultAccount returns the default profile name
func (s *KeyringStore) GetDefaultAccount() (string, error) {
	it, err := s.ring.Get(defaultAccountKey)
	if err != nil {
		if errors.Is(err, keyring.ErrKeyNotFound) {
			return "", nil
		}

		return "", fmt.Errorf("read default account: %w", err)
	}

	return string(it.Data), nil
}

// SetDefaultAccount sets the default profile name
func (s *KeyringStore) SetDefaultAccount(profile string) error {
	profile = normalize(profile)
	if profile == "" {
		return errMissingProfile
	}

	if err := s.ring.Set(keyring.Item{
		Key:  defaultAccountKey,
		Data: []byte(profile),
	}); err != nil {
		return fmt.Errorf("store default account: %w", err)
	}

	return nil
}

// Key format helpers
const defaultAccountKey = "default_account"

func refreshTokenKey(profile string) string {
	return fmt.Sprintf("%s:refresh_token", profile)
}

func accessTokenKey(profile string) string {
	return fmt.Sprintf("%s:access_token", profile)
}

func parseRefreshTokenKey(k string) (profile string, ok bool) {
	const suffix = ":refresh_token"
	if !strings.HasSuffix(k, suffix) {
		return "", false
	}
	rest := strings.TrimSuffix(k, suffix)

	if strings.TrimSpace(rest) == "" {
		return "", false
	}

	return rest, true
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// wrapKeychainError wraps keychain errors with helpful guidance on macOS
func wrapKeychainError(err error) error {
	if err == nil {
		return nil
	}

	if IsKeychainLockedError(err.Error()) {
		return fmt.Errorf("%w\n\nYour macOS keychain is locked. To unlock it, run:\n  security unlock-keychain ~/Library/Keychains/login.keychain-db", err)
	}

	return err
}
