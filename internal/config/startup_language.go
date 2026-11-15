package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// Startup preferences contain no credentials and can be read before the Keychain.
type StartupPreferences struct {
	dir, scope string
}

func (s Store) StartupPreferences() StartupPreferences {
	if runtime.GOOS != "darwin" || s.scope == "" {
		return StartupPreferences{}
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return StartupPreferences{}
	}
	return StartupPreferences{dir: filepath.Join(dir, "Wireproxy Desktop"), scope: s.scope}
}

func (p StartupPreferences) read(name string) string {
	if p.dir != "" {
		if data, err := os.ReadFile(filepath.Join(p.dir, name+p.scope)); err == nil {
			return string(data)
		}
	}
	return ""
}

func (p StartupPreferences) write(name, value string) {
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

func (p StartupPreferences) Language() string {
	switch Language := p.read("language-"); Language {
	case "en", "ru":
		return Language
	default:
		return "system"
	}
}

func (p StartupPreferences) RememberLanguage(Language string) {
	if Language != "en" && Language != "ru" {
		Language = "system"
	}
	p.write("language-", Language)
}

func (p StartupPreferences) NeedsKeychainNotice() bool {
	return p.read("keychain-read-") != "1"
}

func (p StartupPreferences) RecordVerifiedKeychainRead() {
	p.write("keychain-read-", "1")
}

func (p StartupPreferences) Available() bool { return p.dir != "" }
