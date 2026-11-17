package desktop

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>
#import <objc/runtime.h>

static NSString *const windowPositionKey = @"WireproxyDesktopWindowTopLeft";
static char tracksPositionKey;

static void restoreWindowPosition(NSWindow *window) {
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
}

@interface NSWindow (WireproxyPosition)
- (void)wireproxy_orderWindow:(NSWindowOrderingMode)place relativeTo:(NSInteger)otherWindowNumber;
@end

@implementation NSWindow (WireproxyPosition)
- (void)wireproxy_orderWindow:(NSWindowOrderingMode)place relativeTo:(NSInteger)otherWindowNumber {
	// Restore after Gio configures and cascades the window, before AppKit shows it.
	// Only intercept our Gio window, excluding menus and system dialogs.
	if (place != NSWindowOut && [self.title isEqualToString:@"Wireproxy Desktop"] &&
		[self.contentView isKindOfClass:NSClassFromString(@"GioView")] &&
		!objc_getAssociatedObject(self, &tracksPositionKey)) {
		objc_setAssociatedObject(self, &tracksPositionKey, @NO, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
		restoreWindowPosition(self);
	}
	[self wireproxy_orderWindow:place relativeTo:otherWindowNumber];
}
@end

static void installWindowPositionTracking(void) {
	static dispatch_once_t installed;
	dispatch_once(&installed, ^{
		Method original = class_getInstanceMethod(NSWindow.class, @selector(orderWindow:relativeTo:));
		Method replacement = class_getInstanceMethod(NSWindow.class, @selector(wireproxy_orderWindow:relativeTo:));
		method_exchangeImplementations(original, replacement);
	});
	static dispatch_once_t observer;
	dispatch_once(&observer, ^{
		[[NSNotificationCenter defaultCenter]
			addObserverForName:NSWindowDidMoveNotification
			object:nil queue:nil usingBlock:^(NSNotification *notification) {
				NSWindow *moved = notification.object;
				if (![objc_getAssociatedObject(moved, &tracksPositionKey) boolValue] ||
					!moved.visible || moved.miniaturized || moved.zoomed || (moved.styleMask & NSWindowStyleMaskFullScreen)) {
					return;
				}
				NSRect frame = moved.frame;
				NSPoint topLeft = NSMakePoint(NSMinX(frame), NSMaxY(frame));
				[NSUserDefaults.standardUserDefaults setObject:NSStringFromPoint(topLeft) forKey:windowPositionKey];
			}];
	});
}
*/
import "C"

import "gioui.org/app"

func installWindowPositionTracking() { C.installWindowPositionTracking() }

func configureNativeWindow(*app.Window, any) {}
