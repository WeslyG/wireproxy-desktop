//go:build !linux

package desktop

func installedLinuxBundle() bool { return false }

func installedLinuxRPM() bool { return false }
