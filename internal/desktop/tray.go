package desktop

import (
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"fyne.io/systray"
	"gioui.org/app"

	"github.com/WeslyG/wireproxy-desktop/assets"
	"github.com/WeslyG/wireproxy-desktop/internal/config"
	"github.com/WeslyG/wireproxy-desktop/internal/tunnel"
)

type trayItems struct {
	open, status, quit *systray.MenuItem
	start, stop        *systray.MenuItem
}

type quickSettings interface {
	publish(*application, tunnel.View)
	close()
}

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
	app          *application
	exited       chan struct{}
	stop         func()
	lastLanguage string
	lastActive   bool
	lastStatus   string
	lastCanStart bool
	lastCanStop  bool
	quick        quickSettings
	background   bool
}

func newDesktopTray(store config.Store, storeErr error) *desktopTray {
	t := &desktopTray{wake: make(chan struct{}, 1), exited: make(chan struct{})}
	t.app = newApplication(store, storeErr, t.invalidate)
	t.quick = startQuickSettings(t)
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
	t.app.session.Notify()
	go func() {
		for {
			select {
			case <-items.open.ClickedCh:
				t.requestWindow()
			case <-start:
				t.app.requestTunnel(startTrayTunnel)
			case <-stop:
				t.app.requestTunnel(stopTrayTunnel)
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

func (t *desktopTray) update(d *application, view tunnel.View) {
	if t.quick != nil {
		t.quick.publish(d, view)
	}
	items := t.items.Load()
	active := view.TunnelReady()
	status := "Proxy off"
	if active {
		status = "Proxy on"
	}
	if runtime.GOOS == "windows" {
		status = string(view.Status)
		if status == "" {
			status = "Disconnected"
		}
	}
	language := d.cfg.Settings.Language
	_, canStart := d.startablePrimary(view)
	canStop := view.Active && view.Status != tunnel.Stopping
	if items == nil || (t.lastLanguage == language && t.lastActive == active && t.lastStatus == status &&
		t.lastCanStart == canStart && t.lastCanStop == canStop) {
		return
	}
	iconChanged := t.lastActive != active
	t.lastLanguage, t.lastActive, t.lastStatus = language, active, status
	t.lastCanStart, t.lastCanStop = canStart, canStop
	update := func() {
		items.status.SetTitle(translate(language, status))
		items.open.SetTitle(translate(language, "Open Wireproxy Desktop"))
		items.quit.SetTitle(translate(language, "Quit"))
		if items.start != nil {
			startLabel, stopLabel := trayTunnelLabels()
			items.start.SetTitle(translate(language, startLabel))
			items.stop.SetTitle(translate(language, stopLabel))
			setTrayItemEnabled(items.start, canStart)
			setTrayItemEnabled(items.stop, canStop)
		}
		if iconChanged {
			t.setIcon(active)
		}
		setTrayTooltip(translate(language, status))
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
		systray.SetIcon(assets.TrayIdleICO)
	case "darwin":
		systray.SetTemplateIcon(assets.TrayTemplate, assets.TrayIdlePNG)
	default:
		systray.SetIcon(assets.TrayIdlePNG)
	}
}

func (t *desktopTray) close() {
	if t.quick != nil {
		t.quick.close()
	}
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
