package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// AppName is the application name used for keyring and config
const AppName = "roam"

// Config holds CLI configuration
type Config struct {
	BaseURL        string `yaml:"base_url,omitempty"`
	GraphName      string `yaml:"graph_name,omitempty"`
	Token          string `yaml:"token,omitempty"`
	KeyringBackend string `yaml:"keyring_backend,omitempty"` // auto, keychain, file
	OutputFormat   string `yaml:"output_format,omitempty"`   // text, json, yaml, table
}

// ConfigDir returns the config directory path
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".config", "roam"), nil
}

// DefaultConfigPath returns the default config file path
func DefaultConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// EnsureKeyringDir ensures the keyring directory exists and returns its path
func EnsureKeyringDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	keyringDir := filepath.Join(dir, "keyring")
	if err := os.MkdirAll(keyringDir, 0o700); err != nil {
		return "", fmt.Errorf("creating keyring directory: %w", err)
	}
	return keyringDir, nil
}

// ReadConfig reads the config file from the default location
func ReadConfig() (*Config, error) {
	path, err := DefaultConfigPath()
	if err != nil {
		return nil, err
	}
	return Load(path)
}

// Load loads config from the given path
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return &cfg, nil
}

// Save saves config to the given path
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}
