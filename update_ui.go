package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type updateResult struct {
	release                *desktopRelease
	target, staged, digest string
	err                    error
}

type updateControls struct {
	window                      *app.Window
	notify                      func()
	results                     chan updateResult
	check, install, openRelease widget.Clickable
	release                     *desktopRelease
	status                      string
	busy, portable              bool
	helper                      io.WriteCloser
	cancelDownload              context.CancelFunc
}

func (u *updateControls) init(notify func(), portable bool) {
	u.notify, u.portable = notify, portable
	u.results = make(chan updateResult, 1)
	if _, ok := stableVersion(version); ok {
		u.checkRelease()
	} else {
		u.status = "Automatic installation is available in release builds."
	}
}

func (u *updateControls) submit(result updateResult) {
	u.results <- result
	u.notify()
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
			if result.release != nil {
				u.release = result.release
			}
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
			u.status = "No newer release found."
			if u.available() {
				u.status = ""
			}
			if _, stable := stableVersion(version); !stable {
				u.status = "Latest published release"
			}
		default:
			u.release = nil
			u.status = "No newer release found."
		}
	default:
	}
	if u.check.Clicked(gtx) && !u.busy {
		u.checkRelease()
	}
	if u.install.Clicked(gtx) && !u.busy && u.available() {
		u.download()
	}
	if u.openRelease.Clicked(gtx) {
		if err := openReleasePage(u.release); err != nil {
			u.status = err.Error()
		}
	}
}

func (u *updateControls) available() bool {
	return u.release != nil && u.release.Asset.Name != "" && isNewer(u.release.Tag, version)
}

func openReleasePage(release *desktopRelease) error {
	url := "https://github.com/WeslyG/wireproxy-desktop/releases"
	if release != nil {
		if _, ok := stableVersion(release.Tag); ok {
			url += "/tag/" + release.Tag
		}
	}
	command := exec.Command("xdg-open", url)
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", url)
	case "windows":
		command = exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url)
		hideWorkerWindow(command)
	}
	if err := command.Start(); err != nil {
		return err
	}
	go command.Wait()
	return nil
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
	if !released && !u.busy && u.release != nil && u.status == "Latest published release" {
		status += " " + u.release.Tag
	}
	if u.available() && status == "" {
		status = d.tr("Available version") + " " + u.release.Tag + ". " + d.tr("Installing restarts Wireproxy and disconnects the proxy.")
	}
	rows := []layout.FlexChild{
		layout.Rigid(d.rawLabel(13, d.tr("App updates")+"  ·  "+version, textColor, font.Bold)),
		gap(5),
		layout.Rigid(d.rawLabel(13, status, muted, font.Normal)),
		gap(8),
		layout.Rigid(stack(layout.Horizontal,
			layout.Rigid(d.button(&u.check, "Check for updates", selection, textColor, !u.busy)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !u.busy {
					return layout.Dimensions{}
				}
				return layout.Inset{Left: 7}.Layout(gtx, material.Loader(d.theme).Layout)
			}),
			gap(7),
			layout.Rigid(d.button(&u.install, "Install and restart", selection, textColor, !u.busy && u.available())), gap(7),
			layout.Rigid(d.button(&u.openRelease, "Open release on GitHub", inputFill, textColor, true)),
		)),
	}
	if u.release != nil && strings.TrimSpace(u.release.Notes) != "" {
		notes := []rune(strings.TrimSpace(u.release.Notes))
		if len(notes) > 12000 {
			notes = append(notes[:12000], '…')
		}
		rows = append(rows, gap(16), layout.Rigid(d.label(17, "What's new", textColor, font.Bold)),
			gap(6), layout.Rigid(d.rawLabel(14, string(notes), muted, font.Normal)))
	}
	return stack(layout.Vertical, rows...)
}
