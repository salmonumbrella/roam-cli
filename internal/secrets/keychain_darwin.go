//go:build darwin

package secrets

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/term"
)

const errSecInteractionNotAllowed = "-25308"

var (
	errKeychainPathUnknown = errors.New("cannot determine login keychain path")
	errKeychainNoTTY       = errors.New("keychain is locked and no TTY available for password prompt")
	errKeychainUnlock      = errors.New("unlock keychain: incorrect password or keychain error")
)

// IsKeychainLockedError returns true if the error string indicates a locked keychain.
func IsKeychainLockedError(errStr string) bool {
	return strings.Contains(errStr, errSecInteractionNotAllowed)
}

// loginKeychainPath returns the path to the user's login keychain.
func loginKeychainPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "Keychains", "login.keychain-db")
}

// CheckKeychainLocked checks if the login keychain is locked.
func CheckKeychainLocked() bool {
	path := loginKeychainPath()
	if path == "" {
		return false
	}
	cmd := exec.CommandContext(context.Background(), "security", "show-keychain-info", path)
	return cmd.Run() != nil
}

// UnlockKeychain prompts for password and unlocks the login keychain.
func UnlockKeychain() error {
	path := loginKeychainPath()
	if path == "" {
		return errKeychainPathUnknown
	}

	if !term.IsTerminal(syscall.Stdin) {
		return fmt.Errorf("%w\n\nTo unlock manually, run:\n  security unlock-keychain ~/Library/Keychains/login.keychain-db", errKeychainNoTTY)
	}

	fmt.Fprint(os.Stderr, "Keychain is locked. Enter your macOS login password to unlock: ")
	password, err := term.ReadPassword(syscall.Stdin)
	fmt.Fprintln(os.Stderr)

	if err != nil {
		return fmt.Errorf("read password: %w", err)
	}

	cmd := exec.CommandContext(context.Background(), "security", "unlock-keychain", path)
	cmd.Stdin = strings.NewReader(string(password) + "\n")

	if err := cmd.Run(); err != nil {
		return errKeychainUnlock
	}
	return nil
}

// EnsureKeychainAccess checks if keychain is accessible and unlocks if needed.
func EnsureKeychainAccess() error {
	if !CheckKeychainLocked() {
		return nil
	}
	return UnlockKeychain()
}
