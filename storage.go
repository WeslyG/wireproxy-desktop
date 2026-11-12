package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type profile struct {
	Name   string `json:"name"`
	Config string `json:"config"`
}

type settings struct {
	Host     string `json:"host"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Language string `json:"language"`
	Port     int    `json:"port"`
	Verbose  bool   `json:"verbose"`
}

type configuration struct {
	DefaultProfile string    `json:"defaultProfile,omitempty"`
	Profiles       []profile `json:"profiles"`
	Settings       settings  `json:"settings"`
}

var errConfigMissing = errors.New("protected configuration does not exist")

type configStore struct {
	scope, legacyPath string
}

func openConfigStore(portable bool) (configStore, error) {
	if portable {
		executable, err := os.Executable()
		if err != nil {
			return configStore{}, err
		}
		dir := filepath.Dir(executable)
		identity := dir
		if runtime.GOOS == "windows" {
			identity = strings.ToLower(identity)
		}
		return configStore{scope: fmt.Sprintf("portable-%x", sha256.Sum256([]byte(identity))),
			legacyPath: filepath.Join(dir, "wireproxy-desktop.json")}, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return configStore{}, err
	}
	return configStore{scope: "default", legacyPath: filepath.Join(dir, "Wireproxy Desktop", "profiles.json")}, nil
}

func defaultConfig() configuration {
	return configuration{Settings: settings{Host: "127.0.0.1", Port: 1080, Language: "en", Verbose: true}}
}

func decodeConfig(data []byte) (configuration, error) {
	cfg := defaultConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, errors.New("the saved configuration contains invalid JSON")
	}
	if err := cfg.Settings.validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (s configStore) load() (configuration, string, error) {
	return loadStoredConfig(s.legacyPath, func() ([]byte, error) { return readProtectedConfig(s.scope) },
		func(data []byte) error { return writeProtectedConfig(s.scope, data) })
}

func loadStoredConfig(path string, read func() ([]byte, error), write func([]byte) error) (configuration, string, error) {
	data, err := read()
	if err != nil && !errors.Is(err, errConfigMissing) {
		return configuration{}, "", err
	}
	if err == nil {
		cfg, decodeErr := decodeConfig(data)
		if decodeErr != nil {
			return cfg, "", decodeErr
		}
		if old, readErr := os.ReadFile(path); readErr == nil {
			// Preserve a legacy file written independently by an older version.
			if legacy, legacyErr := decodeConfig(old); legacyErr == nil {
				a, _ := json.Marshal(cfg)
				b, _ := json.Marshal(legacy)
				if bytes.Equal(a, b) {
					return cfg, removeLegacyConfig(path), nil
				}
			}
			return cfg, "An older configuration file still exists and differs from protected storage.", nil
		} else if !errors.Is(readErr, os.ErrNotExist) {
			return cfg, "Could not inspect the old configuration file.", nil
		}
		return cfg, "", nil
	}
	legacy, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaultConfig(), "", nil
	}
	if err != nil {
		return configuration{}, "", err
	}
	cfg, err := decodeConfig(legacy)
	if err != nil {
		return cfg, "", err
	}
	data, err = json.Marshal(cfg)
	if err != nil {
		return cfg, "", err
	}
	if err := write(data); err != nil {
		return cfg, "", fmt.Errorf("migrate configuration: %w", err)
	}
	saved, err := read()
	if err != nil {
		return cfg, "", fmt.Errorf("verify migrated configuration: %w", err)
	}
	if !bytes.Equal(data, saved) {
		return cfg, "", errors.New("could not verify the migrated configuration")
	}
	current, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, "", nil
	}
	if err != nil || !bytes.Equal(legacy, current) {
		return cfg, "The old configuration file changed during migration.", nil
	}
	return cfg, removeLegacyConfig(path), nil
}

func removeLegacyConfig(path string) string {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "Could not remove the old configuration file."
	}
	return ""
}

func (s configStore) save(cfg configuration) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return writeProtectedConfig(s.scope, data)
}
