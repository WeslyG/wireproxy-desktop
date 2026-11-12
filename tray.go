package main

import (
	_ "embed"
	"runtime"
	"sync/atomic"
	"time"

	"fyne.io/systray"
	"gioui.org/app"
)

//go:generate go run github.com/tc-hib/go-winres@v0.3.3 simply --arch amd64,arm64 --manifest gui --icon assets/favicon.ico --out icon

//go:embed assets/icon.png
var applicationIcon []byte

//go:embed assets/tray-idle.png
var trayIdlePNG []byte

//go:embed assets/tray-template.png
var trayTemplate []byte

//go:embed assets/favicon.ico
var applicationICO []byte

//go:embed assets/tray-idle.ico
var trayIdleICO []byte

type trayItems struct {
	open, status, quit *systray.MenuItem
}

type desktopTray struct {
	window       atomic.Pointer[app.Window]
	items        atomic.Pointer[trayItems]
	quitting     atomic.Bool
	wake         chan struct{}
	exited       chan struct{}
	stop         func()
	lastLanguage string
	lastActive   bool
}

func newDesktopTray() *desktopTray {
	t := &desktopTray{wake: make(chan struct{}, 1), exited: make(chan struct{})}
	if traySupported() {
		t.stop = startNativeTray(t.ready, func() { close(t.exited) })
	}
	return t
}

func (t *desktopTray) ready() {
	t.setIcon(false)
	items := &trayItems{
		status: systray.AddMenuItem("Proxy off", ""),
		open:   systray.AddMenuItem("Open Wireproxy Desktop", ""),
		quit:   systray.AddMenuItem("Quit", ""),
	}
	items.status.Disable()
	t.items.Store(items)
	t.invalidate()
	go func() {
		for {
			select {
			case <-items.open.ClickedCh:
				t.requestWindow()
			case <-items.quit.ClickedCh:
				t.quitting.Store(true)
				t.requestWindow()
				return
			case <-t.exited:
				return
			}
		}
	}()
}

func (t *desktopTray) requestWindow() {
	select {
	case t.wake <- struct{}{}:
	default:
	}
	t.invalidate()
}

func (t *desktopTray) invalidate() {
	if window := t.window.Load(); window != nil {
		window.Invalidate()
	}
}

func (t *desktopTray) available() bool {
	return t.items.Load() != nil && traySupported()
}

func (t *desktopTray) awaitWindow() bool {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-t.wake:
			return !t.quitting.Load()
		case <-ticker.C:
			if !t.available() {
				return !t.quitting.Load()
			}
		}
	}
}

func (t *desktopTray) update(d *desktop, active bool) {
	items := t.items.Load()
	if items == nil || (t.lastLanguage == d.cfg.Settings.Language && t.lastActive == active) {
		return
	}
	t.lastLanguage, t.lastActive = d.cfg.Settings.Language, active
	status := "Proxy off"
	if active {
		status = "Proxy on"
	}
	items.status.SetTitle(d.tr(status))
	items.open.SetTitle(d.tr("Open Wireproxy Desktop"))
	items.quit.SetTitle(d.tr("Quit"))
	t.setIcon(active)
	systray.SetTooltip("Wireproxy Desktop · " + d.tr(status))
}

func (t *desktopTray) setIcon(active bool) {
	switch runtime.GOOS {
	case "windows":
		icon := trayIdleICO
		if active {
			icon = applicationICO
		}
		systray.SetIcon(icon)
	case "darwin":
		if active {
			systray.SetIcon(applicationIcon)
		} else {
			systray.SetTemplateIcon(trayTemplate, trayIdlePNG)
		}
	default:
		icon := trayIdlePNG
		if active {
			icon = applicationIcon
		}
		systray.SetIcon(icon)
	}
}

func (t *desktopTray) close() {
	if t.stop == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		t.stop()
		<-t.exited
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
	}
}
