package main

import (
	"os"
	"path/filepath"
	"runtime"
)

// Startup preferences contain no credentials and can be read before the Keychain.
type startupPreferences struct {
	dir, scope string
}

func (s configStore) startupPreferences() startupPreferences {
	if runtime.GOOS != "darwin" || s.scope == "" {
		return startupPreferences{}
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return startupPreferences{}
	}
	return startupPreferences{dir: filepath.Join(dir, "Wireproxy Desktop"), scope: s.scope}
}

func (p startupPreferences) read(name string) string {
	if p.dir != "" {
		if data, err := os.ReadFile(filepath.Join(p.dir, name+p.scope)); err == nil {
			return string(data)
		}
	}
	return ""
}

func (p startupPreferences) write(name, value string) {
	if p.dir == "" || p.read(name) == value {
		return
	}
	if err := os.MkdirAll(p.dir, 0700); err != nil {
		return
	}
	file, err := os.CreateTemp(p.dir, ".startup-*")
	if err != nil {
		return
	}
	defer os.Remove(file.Name())
	_, writeErr := file.WriteString(value)
	if closeErr := file.Close(); writeErr == nil && closeErr == nil {
		_ = os.Rename(file.Name(), filepath.Join(p.dir, name+p.scope))
	}
}

func (p startupPreferences) language() string {
	switch language := p.read("language-"); language {
	case "en", "ru":
		return language
	default:
		return "system"
	}
}

func (p startupPreferences) rememberLanguage(language string) {
	if language != "en" && language != "ru" {
		language = "system"
	}
	p.write("language-", language)
}

func (p startupPreferences) needsKeychainNotice() bool {
	return p.read("keychain-read-") != "1"
}

func (p startupPreferences) recordVerifiedKeychainRead() {
	p.write("keychain-read-", "1")
}
