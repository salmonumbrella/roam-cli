//go:build !darwin

package secrets

import "testing"

func TestEnsureKeychainAccess_NoOp(t *testing.T) {
	if err := EnsureKeychainAccess(); err != nil {
		t.Errorf("EnsureKeychainAccess() on non-darwin = %v, want nil", err)
	}
}

func TestCheckKeychainLocked_NoOp(t *testing.T) {
	if CheckKeychainLocked() {
		t.Error("CheckKeychainLocked() on non-darwin = true, want false")
	}
}

func TestUnlockKeychain_NoOp(t *testing.T) {
	if err := UnlockKeychain(); err != nil {
		t.Errorf("UnlockKeychain() on non-darwin = %v, want nil", err)
	}
}

func TestIsKeychainLockedError_NoOp(t *testing.T) {
	if IsKeychainLockedError("any error") {
		t.Error("IsKeychainLockedError() on non-darwin = true, want false")
	}
}
