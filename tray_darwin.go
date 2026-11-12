package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>

static void setDockIcon(const void *bytes, size_t length) {
	@autoreleasepool {
		NSApplication *app = [NSApplication sharedApplication];
		NSData *data = [NSData dataWithBytes:bytes length:length];
		NSImage *image = [[NSImage alloc] initWithData:data];
		[app setApplicationIconImage:image];
		[image release];
	}
}
*/
import "C"

import (
	"unsafe"

	"fyne.io/systray"
)

func traySupported() bool { return true }

func startNativeTray(ready, exited func()) func() {
	C.setDockIcon(unsafe.Pointer(&applicationIcon[0]), C.size_t(len(applicationIcon)))
	start, stop := systray.RunWithExternalLoop(ready, exited)
	start()
	return stop
}

func setWindowIcon(event any) {}
