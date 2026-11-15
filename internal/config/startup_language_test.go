package config

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestKeychainNoticeAfterRestart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Wireproxy Desktop")
	first := StartupPreferences{dir: dir, scope: "default"}
	if !first.NeedsKeychainNotice() {
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
	first.RecordVerifiedKeychainRead()
	first.RememberLanguage(cfg.Settings.Language)
	reopened := StartupPreferences{dir: dir, scope: "default"}
	if reopened.NeedsKeychainNotice() || reopened.Language() != "ru" {
		t.Fatal("restart should skip the notice and keep the interface language")
	}
	portable := StartupPreferences{dir: dir, scope: "portable-test"}
	if !portable.NeedsKeychainNotice() {
		t.Fatal("a separate storage scope should still show the notice")
	}
}
