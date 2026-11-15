package desktop

import (
	"os"
	"path/filepath"
)

const linuxAppID = "io.github.WeslyG.WireproxyDesktop"

func linuxDataDir() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return dir
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "share")
}

func installedLinuxBundle() bool {
	executable, err := os.Executable()
	if err != nil || executable != filepath.Join(os.Getenv("HOME"), ".local", "bin", "wireproxy-desktop") {
		return false
	}
	_, err = os.Stat(filepath.Join(linuxDataDir(), "applications", linuxAppID+".desktop"))
	return err == nil
}
