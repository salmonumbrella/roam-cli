package api

import (
	"fmt"

	"github.com/salmonumbrella/roam-cli/internal/secrets"
)

// NewClientFromCredentials creates the appropriate API client based on credentials.
// For encrypted mode (local graphs), it returns a LocalClient that communicates
// with the Roam desktop app. For cloud mode (or empty mode for backwards
// compatibility), it returns a Client that communicates with the Roam cloud API.
func NewClientFromCredentials(graphName, token, mode string, opts ...ClientOption) (RoamAPI, error) {
	switch mode {
	case secrets.ModeEncrypted:
		return NewLocalClient(graphName)
	case secrets.ModeCloud, "":
		return NewClient(graphName, token, opts...), nil
	default:
		return nil, fmt.Errorf("unknown mode: %s", mode)
	}
}
