package desktop

import (
	"os"
	"path/filepath"
	"strings"
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

const rpmExecutable = "/usr/bin/wireproxy-desktop"

func installedLinuxRPM() bool {
	exe, err := os.Executable()
	return err == nil && exe == rpmExecutable && activationServiceExists()
}

func activationServiceExists() bool {
	dirs := os.Getenv("XDG_DATA_DIRS")
	if dirs == "" {
		dirs = "/usr/local/share:/usr/share"
	}
	for _, dir := range append([]string{linuxDataDir()}, filepath.SplitList(dirs)...) {
		if strings.HasPrefix(dir, "/") {
			if _, err := os.Stat(filepath.Join(dir, "dbus-1", "services", linuxAppID+".service")); err == nil {
				return true
			}
		}
	}
	return false
}
