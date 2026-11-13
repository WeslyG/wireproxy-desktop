package main

import (
	_ "embed"
	"runtime"
	"strings"
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
	start, stop        *systray.MenuItem
}

type trayTunnelAction bool

const (
	startTrayTunnel trayTunnelAction = true
	stopTrayTunnel  trayTunnelAction = false
)

func trayTunnelLabels() (string, string) {
	if runtime.GOOS == "darwin" {
		return "Connect", "Disconnect"
	}
	return "Start tunnel", "Stop tunnel"
}

type desktopTray struct {
	window       atomic.Pointer[app.Window]
	items        atomic.Pointer[trayItems]
	quitting     atomic.Bool
	wake         chan struct{}
	tunnel       chan trayTunnelAction
	exited       chan struct{}
	stop         func()
	lastLanguage string
	lastActive   bool
	lastStatus   string
	lastCanStart bool
	lastCanStop  bool
}

func newDesktopTray() *desktopTray {
	t := &desktopTray{wake: make(chan struct{}, 1), tunnel: make(chan trayTunnelAction, 1), exited: make(chan struct{})}
	if traySupported() {
		t.stop = startNativeTray(t.ready, func() { close(t.exited) })
	}
	return t
}

func (t *desktopTray) ready() {
	if runtime.GOOS == "windows" {
		systray.SetOnTapped(func() { go t.requestWindow() })
	}
	t.setIcon(false)
	setTrayTooltip("Proxy off")
	items := &trayItems{
		status: systray.AddMenuItem("Proxy off", ""),
		open:   systray.AddMenuItem("Open Wireproxy Desktop", ""),
	}
	var start, stop <-chan struct{}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		startLabel, stopLabel := trayTunnelLabels()
		items.start = systray.AddMenuItem(startLabel, "")
		items.stop = systray.AddMenuItem(stopLabel, "")
		items.start.Disable()
		items.stop.Disable()
		start, stop = items.start.ClickedCh, items.stop.ClickedCh
	}
	items.quit = systray.AddMenuItem("Quit", "")
	items.status.Disable()
	t.items.Store(items)
	t.invalidate()
	go func() {
		for {
			select {
			case <-items.open.ClickedCh:
				t.requestWindow()
			case <-start:
				t.requestTunnel(startTrayTunnel)
			case <-stop:
				t.requestTunnel(stopTrayTunnel)
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

func (t *desktopTray) requestTunnel(action trayTunnelAction) {
	select {
	case t.tunnel <- action:
	default:
	}
	t.invalidate()
}

func (t *desktopTray) applyTunnelAction(d *desktop, action trayTunnelAction) {
	view := d.session.view(false)
	if t.quitting.Load() {
		return
	}
	if action == stopTrayTunnel {
		d.session.stop()
	} else if !view.active && d.storeStatus == "" {
		if server, ok := d.defaultProfile(); ok {
			d.connectProfile(server)
		}
	}
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

func (t *desktopTray) awaitWindow(ui *desktop) bool {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		t.update(ui, ui.session.view(false))
		select {
		case action := <-t.tunnel:
			t.applyTunnelAction(ui, action)
		case <-t.wake:
			return !t.quitting.Load()
		case <-ticker.C:
			if !t.available() {
				return !t.quitting.Load()
			}
		}
	}
}

func (t *desktopTray) update(d *desktop, view sessionView) {
	items := t.items.Load()
	active := view.tunnelReady()
	status := "Proxy off"
	if active {
		status = "Proxy on"
	}
	if runtime.GOOS == "windows" {
		status = view.status
		if status == "" {
			status = "Disconnected"
		}
	}
	language := d.cfg.Settings.Language
	_, hasTarget := d.defaultProfile()
	canStart := hasTarget && !view.active && d.storeStatus == ""
	canStop := view.active && view.status != "Stopping"
	if items == nil || (t.lastLanguage == language && t.lastActive == active && t.lastStatus == status &&
		t.lastCanStart == canStart && t.lastCanStop == canStop) {
		return
	}
	iconChanged := t.lastActive != active
	t.lastLanguage, t.lastActive, t.lastStatus = language, active, status
	t.lastCanStart, t.lastCanStop = canStart, canStop
	update := func() {
		items.status.SetTitle(d.tr(status))
		items.open.SetTitle(d.tr("Open Wireproxy Desktop"))
		items.quit.SetTitle(d.tr("Quit"))
		if items.start != nil {
			startLabel, stopLabel := trayTunnelLabels()
			items.start.SetTitle(d.tr(startLabel))
			items.stop.SetTitle(d.tr(stopLabel))
			setTrayItemEnabled(items.start, canStart)
			setTrayItemEnabled(items.stop, canStop)
		}
		if iconChanged {
			t.setIcon(active)
		}
		setTrayTooltip(d.tr(status))
	}
	if window := t.window.Load(); runtime.GOOS == "darwin" && window != nil {
		// Gio waits for the client to finish processing each event. Run the
		// synchronous AppKit calls on Gio's thread while it is waiting.
		window.Run(update)
		return
	}
	update()
}

func setTrayItemEnabled(item *systray.MenuItem, enabled bool) {
	if enabled {
		item.Enable()
	} else {
		item.Disable()
	}
}

func setTrayTooltip(status string) {
	tooltip := "Wireproxy Desktop · " + status
	if runtime.GOOS == "windows" {
		// systray copies the tooltip into a reused buffer without clearing its tail.
		tooltip += strings.Repeat(" ", max(0, 64-len([]rune(tooltip))))
	}
	systray.SetTooltip(tooltip)
}

func (t *desktopTray) setIcon(active bool) {
	if active {
		systray.SetIcon(connectedTrayIcon())
		return
	}
	switch runtime.GOOS {
	case "windows":
		systray.SetIcon(trayIdleICO)
	case "darwin":
		systray.SetTemplateIcon(trayTemplate, trayIdlePNG)
	default:
		systray.SetIcon(trayIdlePNG)
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
