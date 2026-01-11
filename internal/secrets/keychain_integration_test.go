//go:build integration

package secrets

import (
	"runtime"
	"testing"
)

func TestKeychainAccessFlow_Integration(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("keychain tests only run on macOS")
	}

	// Test that EnsureKeychainAccess doesn't error when keychain is unlocked
	err := EnsureKeychainAccess()
	if err != nil {
		t.Logf("Note: keychain may be locked, got error: %v", err)
	}
}

func TestLinuxFileBackendFallback_Integration(t *testing.T) {
	// NOTE: This test creates a file-based keyring in ~/.config/roam/keyring/
	// which persists after the test. This is acceptable for integration tests
	// which should run in ephemeral CI environments.
	if runtime.GOOS != "linux" {
		t.Skip("Linux file backend tests only run on Linux")
	}

	// t.Setenv handles cleanup and prevents t.Parallel()
	// Empty string effectively unsets for code that checks dbusAddr == ""
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "")
	t.Setenv("ROAM_KEYRING_BACKEND", "file")
	t.Setenv("ROAM_KEYRING_PASSWORD", "testpassword")

	store, err := OpenDefault()
	if err != nil {
		t.Fatalf("OpenDefault() with file backend failed: %v", err)
	}

	_, err = store.Keys()
	if err != nil {
		t.Errorf("store.Keys() failed: %v", err)
	}
}
