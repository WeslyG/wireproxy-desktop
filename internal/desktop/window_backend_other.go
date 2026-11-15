//go:build !linux

package desktop

import "gioui.org/app"

func windowDriverDetached(app.ViewEvent) bool { return false }

func gnomeWindowHeader() *gnomeHeader { return nil }

func usesX11(app.ViewEvent) bool { return false }
