//go:build !linux

package desktop

func startQuickSettings(*desktopTray) quickSettings { return nil }

func openQuickSettingsWindow() bool { return false }
