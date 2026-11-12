package main

import (
	"runtime"

	"fyne.io/systray"
	"gioui.org/app"
	"golang.org/x/sys/windows"
)

func traySupported() bool { return true }

func startNativeTray(ready, exited func()) func() {
	go func() {
		runtime.LockOSThread()
		systray.Run(ready, exited)
		runtime.UnlockOSThread()
	}()
	return systray.Quit
}

var (
	loadImage       = windows.NewLazySystemDLL("user32.dll").NewProc("LoadImageW")
	postMessage     = windows.NewLazySystemDLL("user32.dll").NewProc("PostMessageW")
	getModuleHandle = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetModuleHandleW")
)

func configureNativeWindow(_ *app.Window, event any) {
	view, ok := event.(app.Win32ViewEvent)
	if !ok || view.HWND == 0 {
		return
	}
	const (
		imageIcon = 1
		shared    = 0x8000
		setIcon   = 0x0080
	)
	module, _, _ := getModuleHandle.Call(0)
	for index, size := range []uintptr{16, 32} {
		icon, _, _ := loadImage.Call(module, 1, imageIcon, size, size, shared)
		if icon != 0 {
			postMessage.Call(view.HWND, setIcon, uintptr(index), icon)
		}
	}
}
