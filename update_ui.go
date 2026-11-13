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
	check, automatic       bool
	release                *desktopRelease
	target, staged, digest string
	err                    error
}

type updateControls struct {
	window                         *app.Window
	notify                         func()
	results                        chan updateResult
	check, install, openRelease    widget.Clickable
	footerInstall, whatsNew        widget.Clickable
	autoCheck                      widget.Bool
	release                        *desktopRelease
	status                         string
	busy, portable                 bool
	checking, manualCheck, started bool
	macBundle                      bool
	helper                         io.WriteCloser
	cancelDownload                 context.CancelFunc
}

func (u *updateControls) init(notify func(), portable bool) {
	u.notify, u.portable = notify, portable
	u.results = make(chan updateResult, 1)
	u.macBundle = runtime.GOOS == "darwin"
	if _, ok := stableVersion(version); !ok {
		u.status = "Automatic installation is available in release builds."
	}
}

func (u *updateControls) submit(result updateResult) {
	u.results <- result
	u.notify()
}

func (u *updateControls) checkRelease(automatic bool) {
	u.checking, u.manualCheck = true, !automatic
	if !automatic {
		u.status = "Checking GitHub for updates…"
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		release, err := fetchRelease(ctx, updateClient, releaseAPI, version, platformAsset(runtime.GOOS, runtime.GOARCH))
		u.submit(updateResult{check: true, automatic: automatic, release: release, err: err})
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

func (u *updateControls) update(gtx layout.Context, d *desktop) {
	if !u.started && d.storeStatus == "" {
		u.started = true
		u.autoCheck.Value = !d.cfg.Settings.DisableUpdateCheck
		if _, stable := stableVersion(version); stable && u.autoCheck.Value {
			u.checkRelease(true)
		}
	}
	select {
	case result := <-u.results:
		if result.check {
			u.checking, u.manualCheck = false, false
			switch {
			case result.err != nil:
				if !result.automatic {
					u.status = "Could not check for updates. Try again later."
				}
			case !result.automatic || !d.cfg.Settings.DisableUpdateCheck:
				u.release = result.release
				u.status = ""
				if !u.available() && !result.automatic {
					u.status = "No newer release found."
					if _, stable := stableVersion(version); !stable && u.release != nil {
						u.status = "Latest published release"
					}
				}
			}
			break
		}
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
		}
	default:
	}
	if d.storeStatus != "" {
		return
	}
	if u.autoCheck.Update(gtx) && d.page == 1 && d.settingsTab == 2 {
		cfg := d.cfg
		cfg.Settings.DisableUpdateCheck = !u.autoCheck.Value
		d.persist(cfg, func(err error) {
			u.autoCheck.Value = !d.cfg.Settings.DisableUpdateCheck
			if err == nil && u.autoCheck.Value && !u.checking && !u.busy {
				if _, stable := stableVersion(version); stable {
					u.checkRelease(true)
				}
			}
		})
		return
	}
	if u.whatsNew.Clicked(gtx) && u.available() {
		d.cancelConnectionProbe()
		d.page, d.settingsTab = 1, 2
		d.settingsList.Position = layout.Position{}
		d.interfaceOpen, d.languageOpen = false, false
	}
	if u.check.Clicked(gtx) && !u.busy && !u.checking {
		u.checkRelease(false)
	}
	install := u.install.Clicked(gtx)
	footerInstall := u.footerInstall.Clicked(gtx)
	if (install || footerInstall) && !u.busy && !u.checking && u.available() {
		if u.macBundle {
			_ = openReleasePage(u.release)
		} else {
			u.download()
		}
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

func (u *updateControls) installLabel() string {
	if u.macBundle {
		return "Download on GitHub"
	}
	return "Install and restart"
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
		message := "Installing restarts Wireproxy and disconnects the proxy."
		if u.macBundle {
			message = "Download the new app from GitHub and replace your current copy."
		}
		status = d.tr("Available version") + " " + u.release.Tag + ". " + d.tr(message)
	}
	rows := []layout.FlexChild{
		layout.Rigid(d.rawLabel(13, d.tr("App updates")+"  ·  "+version, textColor, font.Bold)),
		gap(5),
		layout.Rigid(d.rawLabel(13, status, muted, font.Normal)),
		gap(8),
		layout.Rigid(material.CheckBox(d.theme, &u.autoCheck, d.tr("Check for updates automatically")).Layout),
		layout.Rigid(stack(layout.Horizontal,
			layout.Rigid(d.updateButton(&u.check, "Check for updates", u.manualCheck, !u.busy && !u.checking)),
			gap(7),
			layout.Rigid(d.updateButton(&u.install, u.installLabel(), u.busy, !u.busy && !u.checking && u.available())), gap(7),
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
