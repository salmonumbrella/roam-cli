package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"

	"github.com/salmonumbrella/roam-cli/internal/api"
)

func TestCLIHarnessPageListJSON(t *testing.T) {
	restore := snapshotCLIState()
	defer restore()

	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	in := &bytes.Buffer{}

	rootCmd.SetOut(out)
	rootCmd.SetErr(errBuf)
	rootCmd.SetIn(in)
	rootCmd.SetContext(withIO(context.Background(), in, out, errBuf))

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(""), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	prevEnvGet := envGet
	envGet = func(key string) string {
		return ""
	}
	defer func() { envGet = prevEnvGet }()

	var gotGraph string
	var gotToken string
	prevNewClient := newClientFromCredsFunc
	newClientFromCredsFunc = func(graphName, token, mode string, opts ...api.ClientOption) (api.RoamAPI, error) {
		gotGraph = graphName
		gotToken = token
		return &fakeClient{
			ListPagesFunc: func(modifiedToday bool, limit int) ([][]interface{}, error) {
				return [][]interface{}{{"Page One", "uid1", float64(123)}}, nil
			},
		}, nil
	}
	defer func() { newClientFromCredsFunc = prevNewClient }()

	rootCmd.SetArgs([]string{"--config", cfgPath, "--output", "json", "--token", "tok", "--graph", "graph", "page", "list"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if gotGraph != "graph" {
		t.Fatalf("expected graph 'graph', got %q", gotGraph)
	}
	if gotToken != "tok" {
		t.Fatalf("expected token 'tok', got %q", gotToken)
	}

	var pages []struct {
		Title string `json:"title"`
		UID   string `json:"uid"`
	}
	if err := json.Unmarshal(out.Bytes(), &pages); err != nil {
		t.Fatalf("parse output: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	if pages[0].Title != "Page One" || pages[0].UID != "uid1" {
		t.Fatalf("unexpected page output: %+v", pages[0])
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", errBuf.String())
	}
}

func snapshotCLIState() func() {
	prevGraph := graphName
	prevToken := apiToken
	prevOutputFmt := outputFmt
	prevOutputType := outputType
	prevDebug := debug
	prevConfig := configFile
	prevUseLocal := useLocal
	prevQueryExpr := queryExpr
	prevQueryFile := queryFile
	prevErrorFmt := errorFmt
	prevQuiet := quietFlag
	prevYes := yesFlag
	prevResultLimit := resultLimit
	prevResultSort := resultSort
	prevResultDesc := resultDesc
	prevClient := client

	prevOut := rootCmd.OutOrStdout()
	prevErr := rootCmd.ErrOrStderr()
	prevIn := rootCmd.InOrStdin()
	prevCtx := rootCmd.Context()

	return func() {
		graphName = prevGraph
		apiToken = prevToken
		outputFmt = prevOutputFmt
		outputType = prevOutputType
		debug = prevDebug
		configFile = prevConfig
		useLocal = prevUseLocal
		queryExpr = prevQueryExpr
		queryFile = prevQueryFile
		errorFmt = prevErrorFmt
		quietFlag = prevQuiet
		yesFlag = prevYes
		resultLimit = prevResultLimit
		resultSort = prevResultSort
		resultDesc = prevResultDesc
		client = prevClient

		rootCmd.SetOut(prevOut)
		rootCmd.SetErr(prevErr)
		rootCmd.SetIn(prevIn)
		rootCmd.SetContext(prevCtx)
		rootCmd.SetArgs(nil)
		resetFlagChanges(rootCmd)
	}
}

func resetFlagChanges(cmdFlagSet interface {
	Flags() *pflag.FlagSet
	PersistentFlags() *pflag.FlagSet
	InheritedFlags() *pflag.FlagSet
},
) {
	if cmdFlagSet == nil {
		return
	}
	cmdFlagSet.Flags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
	})
	cmdFlagSet.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
	})
	cmdFlagSet.InheritedFlags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
	})
}
