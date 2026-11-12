package main

import (
	"fmt"
	"os"

	"gioui.org/app"
	"gioui.org/op"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == workerFlag {
		if err := runWorker(os.Stdin); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	go func() {
		window := new(app.Window)
		window.Option(app.Title("Wireproxy Desktop"), app.Size(1020, 720), app.MinSize(760, 630))
		path, err := configPath(len(os.Args) > 1 && os.Args[1] == "--portable")
		ui := newDesktop(window, path, err)
		var ops op.Ops
		for {
			switch event := window.Event().(type) {
			case app.FrameEvent:
				gtx := app.NewContext(&ops, event)
				ui.layout(gtx)
				event.Frame(gtx.Ops)
			case app.DestroyEvent:
				if done := ui.session.stop(); done != nil {
					<-done
				}
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
