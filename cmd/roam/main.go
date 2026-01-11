package main

import (
	"os"

	"github.com/salmonumbrella/roam-cli/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
