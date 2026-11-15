package desktop

import (
	"fmt"
	"os"
	"runtime"
	"slices"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/WeslyG/wireproxy-desktop/internal/config"
	"github.com/WeslyG/wireproxy-desktop/internal/updater"
)

func Run(args []string, version string) {
	cleanupIndex := slices.Index(args, "--cleanup-update")
	restarting := cleanupIndex >= 0 && cleanupIndex+1 < len(args)
	instance, err := acquireDesktopInstance(restarting)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if instance == nil {
		if slices.Contains(args, "--background") || openQuickSettingsWindow() {
			return
		}
		fmt.Fprintln(os.Stderr, "Wireproxy Desktop is already running. Open it from the system tray.")
		return
	}
	defer instance.Close()
	if restarting {
		if executable, err := os.Executable(); err == nil {
			go updater.CleanHelper(executable, args[cleanupIndex+1])
		}
	}

	portable := slices.Contains(args, "--portable")
	store, err := config.OpenConfigStore(portable)
	tray := newDesktopTray(store, err)
	tray.watchDockReopen()
	go func() {
		err := runDesktop(args, tray, portable, version)
		tray.close()
		_ = instance.Close()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}()
	app.Main()
}

func runDesktop(args []string, tray *desktopTray, portable bool, version string) error {
	state := tray.app
	backgroundOnly := slices.Contains(args, "--background")
	if backgroundOnly && tray.quick == nil {
		return nil
	}
	prefs := state.store.StartupPreferences()
	keychainNotice := state.loadErr == nil && prefs.Available() && prefs.NeedsKeychainNotice()
	if prefs.Available() {
		state.cfg.Settings.Language = prefs.Language()
	}
	if state.loadErr == nil && !keychainNotice {
		state.loadProtectedConfig()
	}
	defer func() {
		if done := state.session.Stop(); done != nil {
			<-done
		}
	}()
	if backgroundOnly && !awaitWindow(state, tray, true) {
		return nil
	}
	ui := newDesktop(state, keychainNotice)
	ui.updates.init(tray.invalidate, portable, version)
	defer ui.updates.shutdown()
	defer ui.cancelConnectionProbe()
	if slices.Contains(args, "--update-error") {
		ui.feedback("Update installation failed. The previous version is still available.", true)
	}
	for !tray.quitting.Load() {
		background, err := runWindow(ui, tray)
		if err != nil || !background || tray.quitting.Load() {
			return err
		}
		loaded := state.loadGeneration
		if !awaitWindow(state, tray, tray.background) {
			return nil
		}
		if loaded != state.loadGeneration {
			ui.syncConfigWidgets()
		}
	}
	return nil
}

func runWindow(ui *desktop, tray *desktopTray) (bool, error) {
	window := new(app.Window)
	gnome := gnomeWindowHeader()
	nativeTitle := runtime.GOOS != "windows" && gnome == nil
	window.Option(app.Title("Wireproxy Desktop"), app.Decorated(nativeTitle), app.Size(1020, 720), app.MinSize(760, 630))
	ui.updates.window = window
	tray.window.Store(window)
	defer tray.window.Store(nil)
	customTitle, background := !nativeTitle, false
	closeAfterSave, closing := false, false
	var decoration widget.Decorations
	var ops op.Ops
	var placement windowPlacement
	for {
		switch event := window.Event().(type) {
		case app.ConfigEvent:
			customTitle = !event.Config.Decorated
			decoration.Maximized = event.Config.Mode == app.Maximized
		case *app.ClosingEvent:
			if ui.storeDone != nil {
				event.Abort()
				closeAfterSave = true
				continue
			}
			ui.cancelConnectionProbe()
			placement.save(ui.store.Scope())
			background = !tray.quitting.Load() && ui.updates.helper == nil && (tray.background || tray.available())
			closing = true
		case app.FrameEvent:
			gtx := app.NewContext(&ops, event)
			ui.updateConfigStore()
			select {
			case action := <-ui.application.tunnel:
				if !tray.quitting.Load() {
					if err := ui.applyTunnelAction(action); err != nil {
						ui.feedback(err.Error(), true)
					}
				}
			default:
			}
			actions := system.Action(0)
			if customTitle {
				actions = decoration.Update(gtx)
			}
			select {
			case <-tray.wake:
				actions |= system.ActionRaise
			default:
			}
			if (tray.quitting.Load() || closeAfterSave) && ui.storeDone == nil {
				actions = system.ActionClose
			} else if actions&system.ActionClose != 0 && ui.storeDone != nil {
				closeAfterSave = true
				actions &^= system.ActionClose
			}
			if actions != 0 {
				window.Perform(actions)
			}
			ui.updates.update(gtx, ui)
			ui.layoutWindow(gtx, &decoration, customTitle, gnome)
			event.Frame(gtx.Ops)
			tray.update(ui.application, ui.session.View(false))
		case app.DestroyEvent:
			return background, event.Err
		case app.ViewEvent:
			if closing && windowDriverDetached(event) {
				return background, nil
			}
			if gnome != nil && usesX11(event) {
				gnome = nil
				window.Option(app.Decorated(true))
			}
			configureNativeWindow(window, event)
			placement.attach(event, ui.store.Scope())
		}
	}
}
