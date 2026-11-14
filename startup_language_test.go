package main

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestKeychainNoticeAfterRestart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Wireproxy Desktop")
	first := startupPreferences{dir: dir, scope: "default"}
	if !first.needsKeychainNotice() {
		t.Fatal("first launch should show the notice")
	}
	legacyPath := filepath.Join(dir, "profiles.json")
	for _, readErr := range []error{errConfigMissing, errors.New("access denied")} {
		_, _, verified, err := loadStoredConfig(legacyPath, func() ([]byte, error) {
			return nil, readErr
		}, nil)
		if verified || (err == nil) != errors.Is(readErr, errConfigMissing) {
			t.Fatalf("load with %v: verified = %t, error = %v", readErr, verified, err)
		}
	}
	cfg, _, verified, err := loadStoredConfig(legacyPath, func() ([]byte, error) {
		return []byte(`{"settings":{"language":"ru"}}`), nil
	}, nil)
	if err != nil || !verified {
		t.Fatalf("protected read: verified = %t, error = %v", verified, err)
	}
	first.recordVerifiedKeychainRead()
	first.rememberLanguage(cfg.Settings.Language)
	reopened := startupPreferences{dir: dir, scope: "default"}
	if reopened.needsKeychainNotice() || reopened.language() != "ru" {
		t.Fatal("restart should skip the notice and keep the interface language")
	}
	portable := startupPreferences{dir: dir, scope: "portable-test"}
	if !portable.needsKeychainNotice() {
		t.Fatal("a separate storage scope should still show the notice")
	}
}
