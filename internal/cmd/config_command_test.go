package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestConfigSetUnsetCommands(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	// set config file path for this test
	prevConfig := configFile
	configFile = cfgPath
	t.Cleanup(func() { configFile = prevConfig })

	// ensure output is plain text to avoid dependency on formatter
	prevOutput := outputFmt
	outputFmt = "text"
	t.Cleanup(func() { outputFmt = prevOutput })

	// set graph_name
	setCmd := &cobra.Command{}
	if err := runConfigSet(setCmd, []string{"graph_name", "test-graph"}); err != nil {
		t.Fatalf("config set failed: %v", err)
	}

	// verify file exists
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("config file not written: %v", err)
	}

	// unset graph_name
	unsetCmd := &cobra.Command{}
	if err := runConfigUnset(unsetCmd, []string{"graph_name"}); err != nil {
		t.Fatalf("config unset failed: %v", err)
	}
}
