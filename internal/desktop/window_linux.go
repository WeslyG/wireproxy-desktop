package desktop

/*
#cgo pkg-config: x11
#include <X11/Xlib.h>
#include <X11/Xatom.h>

static void setWindowIcon(Display *display, Window window, const unsigned long *icon, int count) {
	Atom property = XInternAtom(display, "_NET_WM_ICON", False);
	XChangeProperty(display, window, property, XA_CARDINAL, 32, PropModeReplace,
		(const unsigned char *)icon, count);
	XFlush(display);
}
*/
import "C"

import (
	"bytes"
	"context"
	"image"
	"image/draw"
	"image/png"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"gioui.org/app"
	"gioui.org/io/system"
	xdraw "golang.org/x/image/draw"

	"github.com/WeslyG/wireproxy-desktop/assets"
)

var x11WindowIcon = sync.OnceValue(func() []C.ulong {
	logo, err := png.Decode(bytes.NewReader(assets.ApplicationIcon))
	if err != nil {
		return nil
	}
	const size = 64
	icon := image.NewNRGBA(image.Rect(0, 0, size, size))
	xdraw.ApproxBiLinear.Scale(icon, icon.Bounds(), logo, logo.Bounds(), draw.Src, nil)
	data := make([]C.ulong, 2, 2+size*size)
	data[0], data[1] = size, size
	for y := range size {
		for x := range size {
			p := icon.NRGBAAt(x, y)
			data = append(data, C.ulong(p.A)<<24|C.ulong(p.R)<<16|C.ulong(p.G)<<8|C.ulong(p.B))
		}
	}
	return data
})

func configureNativeWindow(window *app.Window, event any) {
	view, ok := event.(app.X11ViewEvent)
	if !ok || !view.Valid() {
		return
	}
	icon := x11WindowIcon()
	if len(icon) == 0 {
		return
	}
	window.Run(func() {
		C.setWindowIcon((*C.Display)(view.Display), C.Window(view.Window), &icon[0], C.int(len(icon)))
	})
}

// Gio 0.10.3 clears its driver before delivering the invalid view. Another
// Window.Event call reinitializes the window and leaves an empty Gio window.
func windowDriverDetached(view app.ViewEvent) bool { return !view.Valid() }

func usesX11(event app.ViewEvent) bool {
	view, ok := event.(app.X11ViewEvent)
	return ok && view.Valid()
}

// GNOME does not provide server decorations on Wayland. Draw a header
// matching its controls without changing the selected display backend.
func gnomeWindowHeader() *gnomeHeader {
	if os.Getenv("XDG_SESSION_TYPE") != "wayland" {
		return nil
	}
	gnome := false
	for name := range strings.SplitSeq(os.Getenv("XDG_CURRENT_DESKTOP"), ":") {
		if strings.EqualFold(name, "GNOME") {
			gnome = true
			break
		}
	}
	if !gnome {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	read := func(schema, key string) string {
		value, _ := exec.CommandContext(ctx, "gsettings", "get", schema, key).Output()
		return strings.Trim(strings.TrimSpace(string(value)), "'")
	}
	dark := read("org.gnome.desktop.interface", "color-scheme") == "prefer-dark"
	left, right, ok := strings.Cut(read("org.gnome.desktop.wm.preferences", "button-layout"), ":")
	if !ok {
		right = "close"
	}
	return &gnomeHeader{dark: dark, left: gnomeWindowActions(left), right: gnomeWindowActions(right)}
}

func gnomeWindowActions(names string) []system.Action {
	var actions []system.Action
	for name := range strings.SplitSeq(names, ",") {
		switch name {
		case "close":
			actions = append(actions, system.ActionClose)
		case "minimize":
			actions = append(actions, system.ActionMinimize)
		case "maximize":
			actions = append(actions, system.ActionMaximize)
		}
	}
	return actions
}
