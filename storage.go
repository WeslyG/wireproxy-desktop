package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

func configPath(portable bool) (string, error) {
	if portable {
		executable, err := os.Executable()
		if err != nil {
			return "", err
		}
		return filepath.Join(filepath.Dir(executable), "wireproxy-desktop.json"), nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Wireproxy Desktop", "profiles.json"), nil
}

func loadConfig(path string) (configuration, error) {
	cfg := configuration{Settings: settings{Host: "127.0.0.1", Port: 1080, Language: "en", Verbose: true}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, errors.New("the profiles file contains invalid JSON")
	}
	if cfg.Settings.Port < 1 || cfg.Settings.Port > 65535 {
		return cfg, errors.New("the profiles file contains an invalid SOCKS5 port")
	}
	return cfg, nil
}

func saveConfig(path string, cfg configuration) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".profiles-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(append(data, '\n'))
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return fmt.Errorf("write profiles: %w", err)
	}
	return os.Rename(file.Name(), path)
}
