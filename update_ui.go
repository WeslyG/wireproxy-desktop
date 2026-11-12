package main

import (
	"context"
	"io"
	"os"
	"runtime"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/widget"
)

type updateResult struct {
	release                *desktopRelease
	target, staged, digest string
	err                    error
}

type updateControls struct {
	window         *app.Window
	results        chan updateResult
	check, install widget.Clickable
	release        *desktopRelease
	status         string
	busy, portable bool
	helper         io.WriteCloser
	cancelDownload context.CancelFunc
}

func (u *updateControls) init(window *app.Window, portable bool) {
	u.window, u.portable = window, portable
	u.results = make(chan updateResult, 1)
	if _, ok := stableVersion(version); ok {
		u.checkRelease()
	} else {
		u.status = "Updates are available in tagged release builds."
	}
}

func (u *updateControls) submit(result updateResult) {
	u.results <- result
	u.window.Invalidate()
}

func (u *updateControls) checkRelease() {
	u.busy, u.status = true, "Checking GitHub for updates…"
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		release, err := fetchRelease(ctx, updateClient, releaseAPI, version, platformAsset(runtime.GOOS, runtime.GOARCH))
		u.submit(updateResult{release: release, err: err})
	}()
}

func (u *updateControls) download() {
	asset := u.release.Asset
	u.busy, u.status = true, "Downloading and verifying the update…"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	u.cancelDownload = cancel
	go func() {
		defer cancel()
		target, err := os.Executable()
		if err != nil {
			u.submit(updateResult{err: err})
			return
		}
		staged, err := stageUpdate(ctx, updateClient, target, asset)
		u.submit(updateResult{target: target, staged: staged, digest: asset.Digest, err: err})
	}()
}

func (u *updateControls) update(gtx layout.Context) {
	select {
	case result := <-u.results:
		u.busy = false
		u.cancelDownload = nil
		switch {
		case result.err != nil:
			u.status = result.err.Error()
		case result.staged != "":
			helper, err := startUpdateHelper(result.target, result.staged, result.digest, u.portable)
			if err != nil {
				_ = os.Remove(result.staged)
				u.status = "Could not start the updater: " + err.Error()
				break
			}
			u.helper, u.busy, u.status = helper, true, "Restarting to install the update…"
			u.window.Perform(system.ActionClose)
		case result.release != nil:
			u.release = result.release
			u.status = ""
		default:
			u.release = nil
			u.status = "No newer release found."
		}
	default:
	}
	if u.check.Clicked(gtx) && !u.busy {
		u.checkRelease()
	}
	if u.install.Clicked(gtx) && !u.busy && u.release != nil {
		u.download()
	}
}

func (u *updateControls) shutdown() {
	if u.cancelDownload != nil {
		u.cancelDownload()
		result := <-u.results
		if result.staged != "" {
			_ = os.Remove(result.staged)
		}
	}
	if u.helper != nil {
		_ = u.helper.Close()
	}
}

func (u *updateControls) layout(d *desktop) layout.Widget {
	_, released := stableVersion(version)
	status := d.tr(u.status)
	if u.release != nil && status == "" {
		status = d.tr("Available version") + " " + u.release.Tag + ". " + d.tr("Installing restarts Wireproxy and disconnects the proxy.")
	}
	return stack(layout.Vertical,
		layout.Rigid(d.rawLabel(13, d.tr("App updates")+"  ·  "+version, textColor, font.Bold)),
		gap(5),
		layout.Rigid(d.label(13, u.status, muted, font.Normal)),
		gap(8),
		layout.Rigid(stack(layout.Horizontal,
			layout.Rigid(d.button(&u.check, "Check for updates", selection, textColor, released && !u.busy)),
			gap(7),
			layout.Rigid(d.button(&u.install, "Install and restart", accent, background, !u.busy && u.release != nil)),
		)),
	)
}
