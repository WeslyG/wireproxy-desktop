package main

import "C"

var dockReopenTray *desktopTray

//export wireproxyDockReopen
func wireproxyDockReopen() {
	go dockReopenTray.requestWindow()
}
