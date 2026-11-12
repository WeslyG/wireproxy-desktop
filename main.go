package main

import (
	"fmt"
	"os"
	"slices"

	"gioui.org/app"
	"gioui.org/op"
	"gioui.org/widget"
)

func main() {
	args := os.Args[1:]
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

	go func() {
		window := new(app.Window)
		window.Option(app.Title("Wireproxy Desktop"), app.Decorated(false), app.Size(1020, 720), app.MinSize(760, 630))
		portable := slices.Contains(args, "--portable")
		path, err := configPath(portable)
		ui := newDesktop(window, path, err)
		ui.updates.init(window, portable)
		if slices.Contains(args, "--update-error") {
			ui.feedback("Update installation failed. The previous version is still available.", true)
		}
		customTitle := true
		var decoration widget.Decorations
		var ops op.Ops
		for {
			switch event := window.Event().(type) {
			case app.ConfigEvent:
				customTitle = !event.Config.Decorated
				decoration.Maximized = event.Config.Mode == app.Maximized
			case app.FrameEvent:
				gtx := app.NewContext(&ops, event)
				if customTitle {
					if actions := decoration.Update(gtx); actions != 0 {
						window.Perform(actions)
					}
				}
				ui.updates.update(gtx)
				ui.layoutWindow(gtx, &decoration, customTitle)
				event.Frame(gtx.Ops)
			case app.DestroyEvent:
				if done := ui.session.stop(); done != nil {
					<-done
				}
				ui.updates.shutdown()
				if event.Err != nil {
					fmt.Fprintln(os.Stderr, event.Err)
					os.Exit(1)
				}
				os.Exit(0)
			}
		}
	}()
	app.Main()
}
