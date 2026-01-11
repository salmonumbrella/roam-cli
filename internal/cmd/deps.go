package cmd

import (
	"os"

	"github.com/salmonumbrella/roam-cli/internal/api"
	"github.com/salmonumbrella/roam-cli/internal/secrets"
)

var (
	openSecretsStore       = secrets.OpenDefault
	newClientFromCredsFunc = api.NewClientFromCredentials
	envGet                 = os.Getenv
	newLocalClientFunc     = func(graphName string) (localAPI, error) {
		return api.NewLocalClient(graphName)
	}
)
