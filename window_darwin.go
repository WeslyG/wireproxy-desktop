package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>
#import <objc/runtime.h>

static NSString *const windowPositionKey = @"WireproxyDesktopWindowTopLeft";
static char tracksPositionKey;

static void trackWindowPosition(uintptr_t viewRef) {
	NSView *view = (NSView *)viewRef;
	NSWindow *window = view.window;
	if (!window || objc_getAssociatedObject(window, &tracksPositionKey)) {
		return;
	}
	objc_setAssociatedObject(window, &tracksPositionKey, @NO, OBJC_ASSOCIATION_RETAIN_NONATOMIC);

	static dispatch_once_t observer;
	dispatch_once(&observer, ^{
		[[NSNotificationCenter defaultCenter]
			addObserverForName:NSWindowDidMoveNotification
			object:nil queue:nil usingBlock:^(NSNotification *notification) {
				NSWindow *moved = notification.object;
				if (![objc_getAssociatedObject(moved, &tracksPositionKey) boolValue] ||
					!moved.visible || moved.zoomed || (moved.styleMask & NSWindowStyleMaskFullScreen)) {
					return;
				}
				NSRect frame = moved.frame;
				NSPoint topLeft = NSMakePoint(NSMinX(frame), NSMaxY(frame));
				[NSUserDefaults.standardUserDefaults setObject:NSStringFromPoint(topLeft) forKey:windowPositionKey];
			}];
	});

	// Gio positions its window after attaching the view. Restore our position
	// after that work, before allowing move notifications to save a new one.
	dispatch_async(dispatch_get_main_queue(), ^{
		if (view.window != window) {
			return;
		}
		NSString *position = [NSUserDefaults.standardUserDefaults stringForKey:windowPositionKey];
		if (position) {
			NSPoint topLeft = NSPointFromString(position);
			for (NSScreen *screen in NSScreen.screens) {
				if (!NSPointInRect(NSMakePoint(topLeft.x, topLeft.y - 1), screen.frame)) {
					continue;
				}
				NSRect visible = screen.visibleFrame;
				NSSize size = window.frame.size;
				topLeft.x = MAX(NSMinX(visible), MIN(topLeft.x, NSMaxX(visible) - size.width));
				topLeft.y = MIN(NSMaxY(visible), MAX(topLeft.y, NSMinY(visible) + size.height));
				[window setFrameTopLeftPoint:topLeft];
				break;
			}
		}
		objc_setAssociatedObject(window, &tracksPositionKey, @YES, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
	});
}
*/
import "C"

import "gioui.org/app"

func configureNativeWindow(window *app.Window, event any) {
	if view, ok := event.(app.AppKitViewEvent); ok && view.Valid() {
		window.Run(func() {
			C.trackWindowPosition(C.uintptr_t(view.View))
		})
	}
}
