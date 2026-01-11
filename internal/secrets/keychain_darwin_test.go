//go:build darwin

package secrets

import (
	"fmt"
	"strings"
	"testing"
)

func TestIsKeychainLockedError(t *testing.T) {
	tests := []struct {
		name     string
		errStr   string
		expected bool
	}{
		{"locked error", "The user name or passphrase you entered is not correct. errSecInteractionNotAllowed -25308", true},
		{"other error", "some other error", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsKeychainLockedError(tt.errStr); got != tt.expected {
				t.Errorf("IsKeychainLockedError(%q) = %v, want %v", tt.errStr, got, tt.expected)
			}
		})
	}
}

func TestLoginKeychainPath(t *testing.T) {
	path := loginKeychainPath()
	if path == "" {
		t.Error("loginKeychainPath() returned empty string")
	}
	if !strings.Contains(path, "login.keychain-db") {
		t.Errorf("loginKeychainPath() = %q, want path containing 'login.keychain-db'", path)
	}
}

func TestWrapKeychainError_IncludesRecoveryInstructions_Darwin(t *testing.T) {
	// Test locked keychain error - this only works on macOS where IsKeychainLockedError
	// returns true for errSecInteractionNotAllowed errors
	lockedErr := fmt.Errorf("operation failed: errSecInteractionNotAllowed -25308")
	wrapped := wrapKeychainError(lockedErr)

	errStr := wrapped.Error()
	if !strings.Contains(errStr, "security unlock-keychain") {
		t.Errorf("wrapKeychainError() should include unlock instructions, got: %s", errStr)
	}
}
