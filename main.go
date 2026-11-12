package main

import (
	"fmt"
	"os"
	"slices"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/op"
	"gioui.org/widget"
)

func main() {
	args := os.Args[1:]
	if len(args) == 1 && args[0] == probeWorkerFlag {
		if err := runProbeWorker(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(args) == 1 && args[0] == "--version" {
		fmt.Println(version)
		return
	}
	if len(args) > 0 && args[0] == applyUpdateFlag {
		if err := runUpdateHelper(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == workerFlag {
		if err := runWorker(os.Stdin); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if index := slices.Index(args, "--cleanup-update"); index >= 0 && index+1 < len(args) {
		if executable, err := os.Executable(); err == nil {
			go cleanUpdateHelper(executable, args[index+1])
		}
	}

	tray := newDesktopTray()
	go func() {
		err := runDesktop(args, tray)
		tray.close()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}()
	app.Main()
}

func runDesktop(args []string, tray *desktopTray) error {
	portable := slices.Contains(args, "--portable")
	store, err := openConfigStore(portable)
	ui := newDesktop(store, err, tray.invalidate)
	ui.updates.init(tray.invalidate, portable)
	defer ui.updates.shutdown()
	defer ui.cancelConnectionProbe()
	if slices.Contains(args, "--update-error") {
		ui.feedback("Update installation failed. The previous version is still available.", true)
	}
	for !tray.quitting.Load() {
		background, err := runWindow(ui, tray)
		if done := ui.session.stop(); done != nil {
			<-done
		}
		tray.update(ui, false)
		if err != nil || !background || tray.quitting.Load() || !tray.awaitWindow() {
			return err
		}
	}
	return nil
}

func runWindow(ui *desktop, tray *desktopTray) (bool, error) {
	window := new(app.Window)
	window.Option(app.Title("Wireproxy Desktop"), app.Decorated(false), app.Size(1020, 720), app.MinSize(760, 630))
	ui.updates.window = window
	tray.window.Store(window)
	defer tray.window.Store(nil)
	customTitle, background := true, false
	closeAfterSave := false
	var decoration widget.Decorations
	var ops op.Ops
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
			background = !tray.quitting.Load() && ui.updates.helper == nil && tray.available()
			ui.session.stop()
		case app.FrameEvent:
			gtx := app.NewContext(&ops, event)
			ui.updateConfigStore()
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
			ui.updates.update(gtx)
			ui.layoutWindow(gtx, &decoration, customTitle)
			event.Frame(gtx.Ops)
			tray.update(ui, ui.session.view(false).ready)
		case app.DestroyEvent:
			return background, event.Err
		default:
			setWindowIcon(event)
		}
	}
}
