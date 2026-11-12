package main

import (
	"context"
	"os"
	"time"

	"fyne.io/systray"
	"github.com/godbus/dbus/v5"
)

func traySupported() bool {
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		return false
	}
	bus, err := dbus.SessionBus()
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	const watcher = "org.kde.StatusNotifierWatcher"
	var property dbus.Variant
	err = bus.Object(watcher, "/StatusNotifierWatcher").
		CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, watcher, "IsStatusNotifierHostRegistered").
		Store(&property)
	registered, _ := property.Value().(bool)
	return err == nil && registered
}

func startNativeTray(ready, exited func()) func() {
	go systray.Run(ready, exited)
	return systray.Quit
}

func setWindowIcon(event any) {}
