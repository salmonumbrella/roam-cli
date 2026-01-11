package secrets

import (
	"errors"
	"testing"
	"time"

	"github.com/99designs/keyring"
)

// fakeKeyring implements keyring.Keyring for testing
type fakeKeyring struct{}

func (f *fakeKeyring) Get(_ string) (keyring.Item, error) {
	return keyring.Item{}, nil
}

func (f *fakeKeyring) GetMetadata(_ string) (keyring.Metadata, error) {
	return keyring.Metadata{}, nil
}

func (f *fakeKeyring) Set(_ keyring.Item) error {
	return nil
}

func (f *fakeKeyring) Remove(_ string) error {
	return nil
}

func (f *fakeKeyring) Keys() ([]string, error) {
	return nil, nil
}

func TestOpenKeyringWithTimeout_Success(t *testing.T) {
	// Save original function
	originalOpen := keyringOpenFunc
	defer func() { keyringOpenFunc = originalOpen }()

	keyringOpenFunc = func(_ keyring.Config) (keyring.Keyring, error) {
		return &fakeKeyring{}, nil
	}

	ring, err := openKeyringWithTimeout(keyring.Config{}, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("openKeyringWithTimeout() error = %v", err)
	}
	if ring == nil {
		t.Error("openKeyringWithTimeout() returned nil ring")
	}
}

func TestOpenKeyringWithTimeout_Timeout(t *testing.T) {
	originalOpen := keyringOpenFunc

	// Channel to signal when mock function has completed
	mockDone := make(chan struct{})

	// Mock a slow keyring open that blocks longer than timeout
	keyringOpenFunc = func(_ keyring.Config) (keyring.Keyring, error) {
		defer close(mockDone)
		time.Sleep(500 * time.Millisecond)
		return &fakeKeyring{}, nil
	}

	_, err := openKeyringWithTimeout(keyring.Config{}, 50*time.Millisecond)

	// Wait for goroutine to finish before restoring original function
	<-mockDone
	keyringOpenFunc = originalOpen

	if err == nil {
		t.Fatal("openKeyringWithTimeout() expected error, got nil")
	}
	if !errors.Is(err, errKeyringTimeout) {
		t.Errorf("openKeyringWithTimeout() error = %v, want errKeyringTimeout", err)
	}
}

func TestShouldForceFileBackend(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		backend  string
		dbusAddr string
		expected bool
	}{
		{"linux auto no dbus", "linux", "auto", "", true},
		{"linux auto with dbus", "linux", "auto", "/run/user/1000/bus", false},
		{"linux explicit keychain", "linux", "keychain", "", false},
		{"darwin auto", "darwin", "auto", "", false},
		{"linux file backend", "linux", "file", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := KeyringBackendInfo{Value: tt.backend}
			if got := shouldForceFileBackend(tt.goos, info, tt.dbusAddr); got != tt.expected {
				t.Errorf("shouldForceFileBackend() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestShouldUseKeyringTimeout(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		backend  string
		dbusAddr string
		expected bool
	}{
		{"linux auto with dbus", "linux", "auto", "/run/user/1000/bus", true},
		{"linux auto no dbus", "linux", "auto", "", false},
		{"linux file backend", "linux", "file", "/run/user/1000/bus", false},
		{"darwin auto", "darwin", "auto", "/run/user/1000/bus", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := KeyringBackendInfo{Value: tt.backend}
			if got := shouldUseKeyringTimeout(tt.goos, info, tt.dbusAddr); got != tt.expected {
				t.Errorf("shouldUseKeyringTimeout() = %v, want %v", got, tt.expected)
			}
		})
	}
}
