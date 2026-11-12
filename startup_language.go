package main

import (
	"os"
	"path/filepath"
	"runtime"
)

// Only the interface language is stored here, so the welcome screen can
// respect the user's choice before requesting access to the Keychain.
func (s configStore) startupLanguagePath() string {
	if runtime.GOOS != "darwin" || s.scope == "" {
		return ""
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "Wireproxy Desktop", "language-"+s.scope)
}

func (s configStore) startupLanguage() string {
	if path := s.startupLanguagePath(); path != "" {
		if data, err := os.ReadFile(path); err == nil {
			switch string(data) {
			case "en", "ru":
				return string(data)
			}
		}
	}
	return "system"
}

func (s configStore) rememberStartupLanguage(language string) {
	path := s.startupLanguagePath()
	if path == "" {
		return
	}
	if language != "en" && language != "ru" {
		language = "system"
	}
	if s.startupLanguage() == language {
		return
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".language-*")
	if err != nil {
		return
	}
	defer os.Remove(file.Name())
	_, writeErr := file.WriteString(language)
	if closeErr := file.Close(); writeErr == nil && closeErr == nil {
		_ = os.Rename(file.Name(), path)
	}
}
