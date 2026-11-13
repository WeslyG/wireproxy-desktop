package main

import (
	"encoding/binary"
	"unsafe"

	"gioui.org/app"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	getWindowPlacement = windows.NewLazySystemDLL("user32.dll").NewProc("GetWindowPlacement")
	setWindowPlacement = windows.NewLazySystemDLL("user32.dll").NewProc("SetWindowPlacement")
)

const placementKey = `Software\Wireproxy Desktop\Window`

type windowPlacement struct {
	hwnd uintptr
}

func (p *windowPlacement) attach(event any, scope string) {
	view, ok := event.(app.Win32ViewEvent)
	if !ok || view.HWND == 0 || p.hwnd == view.HWND {
		return
	}
	p.hwnd = view.HWND
	key, err := registry.OpenKey(registry.CURRENT_USER, placementKey, registry.QUERY_VALUE)
	if err != nil {
		return
	}
	defer key.Close()
	data, _, err := key.GetBinaryValue(scope)
	var placement [44]byte
	if err != nil || len(data) != len(placement) {
		return
	}
	copy(placement[:], data)
	binary.LittleEndian.PutUint32(placement[:4], uint32(len(placement)))
	// WPF_ASYNCWINDOWPLACEMENT posts the change to the owning UI thread.
	binary.LittleEndian.PutUint32(placement[4:8], 4)
	if binary.LittleEndian.Uint32(placement[8:12]) != 3 {
		binary.LittleEndian.PutUint32(placement[8:12], 1)
	}
	setWindowPlacement.Call(p.hwnd, uintptr(unsafe.Pointer(&placement[0])))
}

func (p *windowPlacement) save(scope string) {
	if p.hwnd == 0 || scope == "" {
		return
	}
	var placement [44]byte
	binary.LittleEndian.PutUint32(placement[:4], uint32(len(placement)))
	ok, _, _ := getWindowPlacement.Call(p.hwnd, uintptr(unsafe.Pointer(&placement[0])))
	if ok == 0 {
		return
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, placementKey, registry.SET_VALUE)
	if err == nil {
		defer key.Close()
		_ = key.SetBinaryValue(scope, placement[:])
	}
}
