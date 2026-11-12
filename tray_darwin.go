package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>
#import <objc/runtime.h>

extern void wireproxyDockReopen(void);

static BOOL reopenFromDock(id delegate, SEL selector, NSApplication *app, BOOL hasVisibleWindows) {
	wireproxyDockReopen();
	return YES;
}

static void observeDockReopen(void) {
	[[NSNotificationCenter defaultCenter]
		addObserverForName:NSApplicationWillFinishLaunchingNotification
		object:NSApp queue:nil usingBlock:^(NSNotification *notification) {
			id delegate = NSApp.delegate;
			SEL selector = @selector(applicationShouldHandleReopen:hasVisibleWindows:);
			struct objc_method_description description =
				protocol_getMethodDescription(@protocol(NSApplicationDelegate), selector, NO, YES);
			// Gio owns the delegate. Add the optional callback without replacing its implementation.
			if (delegate && description.types &&
				class_addMethod(object_getClass(delegate), selector, (IMP)reopenFromDock, description.types)) {
				NSApp.delegate = nil;
				NSApp.delegate = delegate;
			}
		}];
}

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

func (t *desktopTray) watchDockReopen() {
	dockReopenTray = t
	C.observeDockReopen()
}
