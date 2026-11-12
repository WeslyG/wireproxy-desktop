package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigMigrationVerifiesBeforeRemovingJSON(t *testing.T) {
	for _, verify := range []bool{true, false} {
		name := "verified"
		if !verify {
			name = "verification failed"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profiles.json")
			legacy := []byte(`{"settings":{"port":1081,"username":"example","password":"synthetic"},"profiles":[{"name":"QA","config":"fixture"}]}`)
			if err := os.WriteFile(path, legacy, 0600); err != nil {
				t.Fatal(err)
			}
			var data []byte
			cfg, _, err := loadStoredConfig(path, func() ([]byte, error) {
				if data == nil {
					return nil, errConfigMissing
				}
				if !verify {
					return []byte("{}"), nil
				}
				return data, nil
			}, func(value []byte) error {
				data = value
				return nil
			})
			old, readErr := os.ReadFile(path)
			if verify {
				if err != nil || !errors.Is(readErr, os.ErrNotExist) || cfg.Settings.Port != 1081 || len(cfg.Profiles) != 1 {
					t.Fatalf("migration failed: %v, legacy read: %v", err, readErr)
				}
			} else if err == nil || readErr != nil || !bytes.Equal(legacy, old) {
				t.Fatal("unverified migration failed to preserve the original")
			}
		})
	}
}

func TestDecodeConfigMigratesLegacyEnglishDefault(t *testing.T) {
	for _, test := range []struct {
		name, data, language string
	}{
		{"missing", `{}`, "system"},
		{"legacy English default", `{"settings":{"language":"en"}}`, "system"},
		{"explicit English", `{"settings":{"language":"en","languageOverride":true}}`, "en"},
		{"legacy Russian selection", `{"settings":{"language":"ru"}}`, "ru"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := decodeConfig([]byte(test.data))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Settings.Language != test.language {
				t.Fatalf("language = %q, want %q", cfg.Settings.Language, test.language)
			}
		})
	}
}
