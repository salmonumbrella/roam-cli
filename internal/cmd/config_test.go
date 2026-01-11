package cmd

import (
	"testing"

	"github.com/salmonumbrella/roam-cli/internal/config"
)

func TestConfigApplyAndClear(t *testing.T) {
	cfg := &config.Config{}

	if err := applyConfigValue(cfg, "graph_name", "my-graph"); err != nil {
		t.Fatalf("apply graph_name: %v", err)
	}
	if cfg.GraphName != "my-graph" {
		t.Fatalf("expected graph_name set, got %q", cfg.GraphName)
	}

	if err := clearConfigValue(cfg, "graph_name"); err != nil {
		t.Fatalf("clear graph_name: %v", err)
	}
	if cfg.GraphName != "" {
		t.Fatalf("expected graph_name cleared, got %q", cfg.GraphName)
	}

	if err := applyConfigValue(cfg, "unknown", "x"); err == nil {
		t.Fatalf("expected error for unknown key")
	}
}

func TestSupportedConfigKeys(t *testing.T) {
	keys := supportedConfigKeys()
	if len(keys) == 0 {
		t.Fatalf("expected supported keys")
	}

	seen := map[string]bool{}
	for _, k := range keys {
		seen[k] = true
	}

	for _, k := range []string{"base_url", "graph_name", "token", "keyring_backend", "output_format"} {
		if !seen[k] {
			t.Fatalf("missing key %s", k)
		}
	}
}

func TestConfigOutputMasksToken(t *testing.T) {
	cfg := &config.Config{
		Token: "abcdefghijklmnop",
	}

	output := configOutput(cfg)
	token, ok := output["token"].(string)
	if !ok {
		t.Fatalf("expected token string in output")
	}
	if token == cfg.Token {
		t.Fatalf("expected masked token, got raw")
	}
	if output["token_set"] != true {
		t.Fatalf("expected token_set true")
	}
}
