package desktop

import "time"

func awaitWindow(state *application, tray *desktopTray, windowless bool) bool {
	// Only check the tray periodically if it was started. On GNOME without a
	// tray, worker and storage changes wake this loop directly.
	var trayCheck <-chan time.Time
	trayPresent := tray.stop != nil
	if tray.stop != nil {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		trayCheck = ticker.C
	}
	const idleTimeout = 15 * time.Second
	var idle <-chan time.Time
	var timer *time.Timer
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		view := state.session.View(false)
		tray.update(state, view)
		if windowless && !view.Active && state.storeStatus == "" && (!trayPresent || tray.items.Load() == nil) {
			if idle == nil {
				if timer == nil {
					timer = time.NewTimer(idleTimeout)
				} else {
					timer.Reset(idleTimeout)
				}
				idle = timer.C
			}
		} else if idle != nil {
			timer.Stop()
			idle = nil
		}
		select {
		case action := <-state.tunnel:
			if !tray.quitting.Load() {
				_ = state.applyTunnelAction(action)
			}
		case result := <-state.storeResults:
			state.applyLoadedConfig(result)
		case <-state.changed:
		case <-tray.wake:
			return !tray.quitting.Load()
		case <-idle:
			return false
		case <-trayCheck:
			if !tray.available() {
				if !windowless {
					return !tray.quitting.Load()
				}
				trayCheck = nil
				trayPresent = false
			}
		}
	}
}
