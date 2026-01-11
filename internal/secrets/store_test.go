package secrets

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/99designs/keyring"
)

func TestWrapKeychainError_IncludesRecoveryInstructions(t *testing.T) {
	// Test locked keychain error
	lockedErr := fmt.Errorf("operation failed: errSecInteractionNotAllowed -25308")
	wrapped := wrapKeychainError(lockedErr)

	errStr := wrapped.Error()
	if !strings.Contains(errStr, "security unlock-keychain") {
		t.Errorf("wrapKeychainError() should include unlock instructions, got: %s", errStr)
	}
}

func TestWrapKeychainError_NilError(t *testing.T) {
	wrapped := wrapKeychainError(nil)
	if wrapped != nil {
		t.Errorf("wrapKeychainError(nil) should return nil, got: %v", wrapped)
	}
}

func TestWrapKeychainError_NonLockedError(t *testing.T) {
	originalErr := fmt.Errorf("some other error")
	wrapped := wrapKeychainError(originalErr)

	if wrapped != originalErr {
		t.Errorf("wrapKeychainError() should return original error unchanged for non-locked errors, got: %v", wrapped)
	}
}

func TestKeyringTimeoutError_IncludesRecoveryInstructions(t *testing.T) {
	// Save original function
	originalOpen := keyringOpenFunc

	// Channel to signal when mock function has completed
	mockDone := make(chan struct{})

	// Mock a slow keyring open that blocks longer than timeout
	keyringOpenFunc = func(_ keyring.Config) (keyring.Keyring, error) {
		defer close(mockDone)
		time.Sleep(200 * time.Millisecond)
		return &fakeKeyring{}, nil
	}

	_, err := openKeyringWithTimeout(keyring.Config{}, 50*time.Millisecond)

	// Wait for goroutine to finish before restoring original function
	<-mockDone
	keyringOpenFunc = originalOpen

	if err == nil {
		t.Fatal("openKeyringWithTimeout() expected error, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "ROAM_KEYRING_BACKEND=file") {
		t.Errorf("timeout error should mention file backend, got: %s", errStr)
	}
}
