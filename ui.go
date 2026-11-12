package main

import (
	"context"
	"image"
	"image/color"
	"io"
	"math"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/io/clipboard"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

var keychainLockIcon, _ = widget.NewIcon(icons.ActionLock)

var (
	engineVersion = tunnelEngineVersion()
	background    = hexColor(0x101722)
	surface       = hexColor(0x192330)
	inputFill     = hexColor(0x111a26)
	selection     = hexColor(0x233849)
	textColor     = hexColor(0xe9f1f7)
	muted         = hexColor(0xa3b5c7)
	accent        = hexColor(0x63e6c5)
	warning       = hexColor(0xffb597)
)

type desktop struct {
	logJump                                               bool
	settingsMenuList                                      widget.List
	dismissMenu                                           widget.Clickable
	cancelProbe                                           context.CancelFunc
	followLogs                                            widget.Bool
	settingsList                                          widget.List
	profileEditorList                                     widget.List
	username, password, link                              widget.Editor
	auth, pin                                             widget.Bool
	linkTab, iniTab, copyConfig, probe                    widget.Clickable
	settingsTab                                           int
	settingsTabs                                          [3]widget.Clickable
	languageMenu, interfaceMenu, checkPort, usePort       widget.Clickable
	interfaceOpen, languageOpen                           bool
	host                                                  string
	interfaces                                            []interfaceOption
	languageOptions                                       [len(languageChoices)]widget.Clickable
	suggestedPort                                         int
	convertedLink, convertedINI                           string
	editINI, probing                                      bool
	probeResult                                           chan profileProbeResult
	selectedProtocol                                      string
	updates                                               updateControls
	theme                                                 *material.Theme
	store                                                 configStore
	storeResults                                          chan configStoreResult
	storeDone                                             func(error)
	keychainNotice                                        bool
	keychainContinue                                      widget.Clickable
	storeStatus, migrationNotice                          string
	cfg                                                   configuration
	session                                               proxySession
	loadErr                                               error
	message                                               string
	messageError                                          bool
	messageUntil                                          time.Time
	lastCheckResult                                       *socksCheckResult
	page, selected                                        int
	rows                                                  []*widget.Clickable
	profilesList, logsList                                widget.List
	name, tunnel, port                                    widget.Editor
	verbose, showTunnel                                   widget.Bool
	tabs                                                  [3]widget.Clickable
	add, edit, connect, check, copy, save, cancel, remove widget.Clickable
	cancelRemoval                                         widget.Clickable
	saveSettings, clearLogs, copyLogs                     widget.Clickable
	editing, deleting                                     bool
}

type configStoreResult struct {
	cfg    configuration
	notice string
	err    error
}

func newDesktop(store configStore, storeErr error, notify func()) *desktop {
	d := &desktop{theme: material.NewTheme(), store: store, selected: -1, loadErr: storeErr, cfg: defaultConfig(),
		storeResults: make(chan configStoreResult, 1),
		probeResult:  make(chan profileProbeResult, 1)}
	d.theme.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	d.theme.Palette = material.Palette{
		Bg: background, Fg: textColor, ContrastBg: accent, ContrastFg: background,
	}
	d.session.notify = notify
	d.profilesList.Axis, d.logsList.Axis = layout.Vertical, layout.Vertical
	d.followLogs.Value = true
	d.settingsList.Axis = layout.Vertical
	d.settingsMenuList.Axis = layout.Vertical
	d.profileEditorList.Axis = layout.Vertical
	d.name.SingleLine, d.port.SingleLine = true, true
	d.username.SingleLine, d.password.SingleLine = true, true
	d.link.WrapPolicy = text.WrapGraphemes
	d.password.Mask = '•'
	d.port.Filter = "0123456789"
	d.tunnel.WrapPolicy = text.WrapGraphemes
	switch {
	case storeErr != nil:
		d.applyLoadedConfig(configStoreResult{err: storeErr})
	case runtime.GOOS == "darwin":
		d.cfg.Settings.Language = store.startupLanguage()
		d.keychainNotice = true
	default:
		d.loadProtectedConfig(notify)
	}
	return d
}

func (d *desktop) loadProtectedConfig(notify func()) {
	d.storeStatus = "Opening protected storage…"
	go func() {
		cfg, notice, err := d.store.load()
		if err == nil {
			d.store.rememberStartupLanguage(cfg.Settings.Language)
		}
		d.storeResults <- configStoreResult{cfg: cfg, notice: notice, err: err}
		notify()
	}()
}

func (d *desktop) applyLoadedConfig(result configStoreResult) {
	d.cfg, d.loadErr, d.migrationNotice = result.cfg, result.err, result.notice
	if d.cfg.Settings.Port == 0 {
		d.cfg.Settings = defaultConfig().Settings
	}
	d.loadSettings()
	d.refreshRows()
	if len(d.cfg.Profiles) > 0 {
		d.selected = 0
		d.refreshProtocol()
	}
}

func (d *desktop) refreshRows() {
	d.rows = make([]*widget.Clickable, len(d.cfg.Profiles))
	for index := range d.rows {
		d.rows[index] = new(widget.Clickable)
	}
}

func (d *desktop) loadSettings() {
	d.port.SetText(strconv.Itoa(d.cfg.Settings.Port))
	d.host = d.cfg.Settings.Host
	d.username.SetText(d.cfg.Settings.Username)
	d.password.SetText(d.cfg.Settings.Password)
	d.auth.Value = d.cfg.Settings.Username != ""
	d.interfaceOpen, d.languageOpen = false, false
	d.suggestedPort = 0
	d.verbose.Value = d.cfg.Settings.Verbose
}

func (d *desktop) feedback(message string, failed bool) {
	d.message, d.messageError = d.tr(message), failed
	d.messageUntil = time.Now().Add(6 * time.Second)
	if failed {
		d.messageUntil = time.Now().Add(10 * time.Second)
	}
}

func (d *desktop) persist(cfg configuration, done func(error)) {
	if d.loadErr != nil {
		d.feedback("Unlock protected storage and restart before saving changes.", true)
		done(d.loadErr)
		return
	}
	d.storeStatus, d.storeDone = "Saving to protected storage…", done
	go func() {
		err := d.store.save(cfg)
		if err == nil {
			d.store.rememberStartupLanguage(cfg.Settings.Language)
		}
		d.storeResults <- configStoreResult{cfg: cfg, err: err}
		d.session.notify()
	}()
}

func (d *desktop) updateConfigStore() {
	select {
	case result := <-d.storeResults:
		d.storeStatus = ""
		if d.storeDone == nil {
			d.applyLoadedConfig(result)
			return
		}
		done := d.storeDone
		d.storeDone = nil
		if result.err != nil {
			d.feedback(d.tr("Could not save to protected storage")+" · "+result.err.Error(), true)
		} else {
			d.cfg = result.cfg
		}
		done(result.err)
	default:
	}
}

func (d *desktop) beginEdit(index int) {
	d.cancelConnectionProbe()
	d.selected, d.editing, d.deleting = index, true, false
	d.profileEditorList.Position = layout.Position{}
	d.name.SetText("")
	d.tunnel.SetText("")
	d.link.SetText("")
	d.convertedLink, d.convertedINI = "", ""
	d.editINI = false
	d.showTunnel.Value = false
	if index >= 0 {
		d.name.SetText(d.cfg.Profiles[index].Name)
		config := d.cfg.Profiles[index].Config
		d.editINI = !strings.HasPrefix(strings.TrimSpace(config), amneziaScheme)
		if d.editINI {
			d.tunnel.SetText(config)
		} else {
			d.link.SetText(config)
		}
	}
}

func (d *desktop) endEdit() {
	d.cancelConnectionProbe()
	d.editing = false
	d.tunnel.SetText("")
	d.link.SetText("")
	if d.selected < 0 && len(d.cfg.Profiles) > 0 {
		d.selected = 0
	}
	d.refreshProtocol()
}

func (d *desktop) saveProfile() {
	p := profile{Name: strings.TrimSpace(d.name.Text()), Config: d.draftConfig()}
	if d.editINI && p.Config != "" && p.Config == strings.TrimSpace(d.convertedINI) {
		p.Config = d.convertedLink
	}
	if p.Name == "" || utf8.RuneCountInString(p.Name) > 96 || p.Config == "" || len(p.Config) > maxConfigLength {
		d.feedback("Enter a server name and a WireGuard configuration.", true)
		return
	}
	for i, existing := range d.cfg.Profiles {
		if i != d.selected && strings.EqualFold(existing.Name, p.Name) {
			d.feedback("A server with this name already exists.", true)
			return
		}
	}
	cfg := d.cfg
	cfg.Profiles = append([]profile(nil), cfg.Profiles...)
	index := d.selected
	if index < 0 {
		index = len(cfg.Profiles)
		cfg.Profiles = append(cfg.Profiles, p)
	} else {
		if cfg.DefaultProfile == cfg.Profiles[index].Name {
			cfg.DefaultProfile = p.Name
		}
		cfg.Profiles[index] = p
	}
	d.persist(cfg, func(err error) {
		if err != nil {
			return
		}
		d.selected = index
		d.endEdit()
		d.refreshRows()
		d.feedback("Server saved.", false)
	})
}

func (d *desktop) deleteProfile() {
	cfg := d.cfg
	cfg.Profiles = append([]profile(nil), cfg.Profiles...)
	if cfg.DefaultProfile == cfg.Profiles[d.selected].Name {
		cfg.DefaultProfile = ""
	}
	cfg.Profiles = append(cfg.Profiles[:d.selected], cfg.Profiles[d.selected+1:]...)
	d.persist(cfg, func(err error) {
		if err != nil {
			return
		}
		d.selected = min(d.selected, len(cfg.Profiles)-1)
		d.deleting = false
		d.refreshRows()
		d.refreshProtocol()
		d.feedback("Server removed.", false)
	})
}

func (d *desktop) actions(gtx layout.Context, view sessionView) {
	d.profileActions(gtx)
	if d.storeStatus != "" {
		return
	}
	for index := range d.tabs {
		if d.tabs[index].Clicked(gtx) && d.page != index {
			d.cancelConnectionProbe()
			d.page = index
			if index == 2 {
				d.logJump = true
			}
			d.feedback("", false)
			d.deleting = false
			if index == 1 {
				d.loadSettings()
			}
		}
	}
	if d.page == 0 {
		for index, row := range d.rows {
			if row.Clicked(gtx) {
				d.selected, d.deleting = index, false
				d.endEdit()
				d.feedback("", false)
			}
		}
		if d.add.Clicked(gtx) {
			d.beginEdit(-1)
			d.feedback("", false)
		}
		if d.edit.Clicked(gtx) && d.selected >= 0 {
			d.beginEdit(d.selected)
		}
		if d.cancel.Clicked(gtx) {
			d.endEdit()
			d.deleting = false
			d.feedback("", false)
		}
		if d.save.Clicked(gtx) && d.editing {
			d.saveProfile()
			return
		}
		cancelRemoval := d.cancelRemoval.Clicked(gtx)
		if cancelRemoval {
			d.deleting = false
		}
		if d.remove.Clicked(gtx) && !cancelRemoval && d.selected >= 0 {
			if d.deleting {
				d.deleteProfile()
			} else {
				d.deleting = true
			}
		}
	}
	if d.connect.Clicked(gtx) {
		if view.active {
			d.session.stop()
		} else if target, ok := d.connectionTarget(); ok {
			d.connectProfile(target)
		}
	}
	if view.tunnelReady() && !view.checking && d.check.Clicked(gtx) && d.session.check() {
		d.feedback("", false)
	}
	if d.copy.Clicked(gtx) {
		current := d.session.view(false)
		if current.tunnelReady() {
			gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader("socks5://" + proxyClientAddress(current.address)))})
			d.feedback("SOCKS5 URL copied. Paste it into the app’s proxy settings.", false)
		}
	}
	if d.page == 1 {
		d.settingsActions(gtx, view)
	}
	if d.page == 2 {
		d.followLogs.Update(gtx)
		if d.clearLogs.Clicked(gtx) {
			d.session.clearLogs()
		}
		if d.copyLogs.Clicked(gtx) {
			gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(strings.Join(view.logs, "\n")))})
			d.feedback("Logs copied.", false)
		}
	}
}

func hexColor(value uint32) color.NRGBA {
	return color.NRGBA{R: byte(value >> 16), G: byte(value >> 8), B: byte(value), A: 255}
}

func (d *desktop) label(size unit.Sp, value string, ink color.NRGBA, weight font.Weight) layout.Widget {
	return d.rawLabel(size, d.tr(value), ink, weight)
}

func (d *desktop) rawLabel(size unit.Sp, value string, ink color.NRGBA, weight font.Weight) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		style := material.Label(d.theme, size, value)
		style.Color, style.Font.Weight = ink, weight
		return style.Layout(gtx)
	}
}

func gap(size unit.Dp) layout.FlexChild {
	return layout.Rigid(layout.Spacer{Width: size, Height: size}.Layout)
}

func stack(axis layout.Axis, children ...layout.FlexChild) layout.Widget {
	align := layout.Middle
	if axis == layout.Vertical {
		align = layout.Start
	}
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: axis, Alignment: align}.Layout(gtx, children...)
	}
}

func panel(fill color.NRGBA, radius, inset unit.Dp, content layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		minimum := gtx.Constraints.Min
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				r := clip.UniformRRect(image.Rectangle{Max: gtx.Constraints.Min}, gtx.Dp(radius))
				paint.FillShape(gtx.Ops, fill, r.Op(gtx.Ops))
				return layout.Dimensions{Size: gtx.Constraints.Min}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				dims := layout.UniformInset(inset).Layout(gtx, content)
				dims.Size.X = max(dims.Size.X, minimum.X)
				dims.Size.Y = max(dims.Size.Y, minimum.Y)
				return dims
			}),
		)
	}
}

func (d *desktop) button(click *widget.Clickable, name string, fill, ink color.NRGBA, enabled bool) layout.Widget {
	return d.rawButton(click, d.tr(name), fill, ink, enabled)
}

func (d *desktop) rawButton(click *widget.Clickable, name string, fill, ink color.NRGBA, enabled bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		buttonInk := ink
		if !enabled {
			gtx = gtx.Disabled()
			buttonInk = hexColor(0x718396)
		}
		style := material.Button(d.theme, click, name)
		style.Background, style.Color, style.TextSize, style.CornerRadius = fill, buttonInk, 13, 8
		style.Inset = layout.Inset{Top: 9, Bottom: 9, Left: 15, Right: 15}
		return style.Layout(gtx)
	}
}

func (d *desktop) connectionButton(name string, enabled bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		style := material.Button(d.theme, &d.connect, d.tr(name))
		style.Background, style.Color = accent, background
		style.Font.Weight, style.TextSize, style.CornerRadius = font.Bold, 15, 9
		style.Inset = layout.Inset{Top: 11, Bottom: 11, Left: 20, Right: 20}
		gtx.Constraints.Min.X = min(gtx.Dp(138), gtx.Constraints.Max.X)
		if !enabled {
			gtx = gtx.Disabled()
			style.Background, style.Color = selection, muted
		}
		return style.Layout(gtx)
	}
}

func (d *desktop) field(editor *widget.Editor, hint string, height unit.Dp) layout.Widget {
	return panel(inputFill, 8, 12, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		gtx.Constraints.Min.Y = min(gtx.Dp(height), gtx.Constraints.Max.Y)
		gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
		style := material.Editor(d.theme, editor, d.tr(hint))
		style.TextSize = 14
		return style.Layout(gtx)
	})
}

func (d *desktop) layout(gtx layout.Context) {
	if d.keychainNotice {
		if d.keychainContinue.Clicked(gtx) {
			d.keychainNotice = false
			d.loadProtectedConfig(d.session.notify)
		} else {
			paint.FillShape(gtx.Ops, background, clip.Rect{Max: gtx.Constraints.Max}.Op())
			layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(460))
				return stack(layout.Vertical,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(36), gtx.Dp(36)))
						return keychainLockIcon.Layout(gtx, warning)
					}), gap(10),
					layout.Rigid(d.label(28, "Keychain access", textColor, font.Bold)), gap(12),
					layout.Rigid(d.label(15, "After you continue, macOS may ask for Keychain access. Wireproxy Desktop uses its own Keychain entry to load and save your profiles and proxy settings.", textColor, font.Normal)), gap(16),
					layout.Rigid(d.button(&d.keychainContinue, "Continue", selection, textColor, true)),
				)(gtx)
			})
			return
		}
	}
	view := d.session.view(d.page == 2)
	if d.storeStatus == "" {
		d.actions(gtx, view)
	}
	if d.storeStatus != "" {
		gtx = gtx.Disabled()
	}
	view = d.session.view(d.page == 2)
	if result := view.checkResult; result != nil && result != d.lastCheckResult {
		d.lastCheckResult = result
		if result.err != nil {
			d.feedback(d.tr("SOCKS5 test failed")+" · "+result.err.Error(), true)
		} else {
			d.feedback(d.tr("SOCKS5 test passed")+" · HTTP "+result.status, false)
		}
	}
	paint.FillShape(gtx.Ops, background, clip.Rect{Max: gtx.Constraints.Max}.Op())
	layout.UniformInset(22).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(d.header()),
			gap(18),
			layout.Rigid(d.statusBar(view)),
			gap(17),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				switch d.page {
				case 1:
					return d.settingsPage(gtx)
				case 2:
					return d.logsPage(gtx, view)
				default:
					return d.serversPage(gtx, view)
				}
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.Y = min(gtx.Dp(42), gtx.Constraints.Max.Y)
				gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
				message, failed := d.message, d.messageError
				if d.storeStatus != "" {
					message, failed = d.tr(d.storeStatus), false
				} else if message != "" && gtx.Now.Before(d.messageUntil) {
					gtx.Execute(op.InvalidateCmd{At: d.messageUntil})
				} else if view.checking {
					message, failed = d.tr("Checking HTTPS through SOCKS5…"), false
				} else if d.loadErr != nil {
					message, failed = d.tr("Could not open protected storage")+" · "+d.loadErr.Error(), true
				} else if d.migrationNotice != "" {
					message, failed = d.tr(d.migrationNotice), true
				} else {
					return layout.Spacer{}.Layout(gtx)
				}
				ink := accent
				if failed {
					ink = warning
				}
				return layout.Inset{Top: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					style := material.Label(d.theme, 13, message)
					style.Color, style.Font.Weight, style.MaxLines = ink, font.Medium, 2
					return style.Layout(gtx)
				})
			}),
		)
	})
}

func (d *desktop) layoutWindow(gtx layout.Context, decoration *widget.Decorations, customTitle bool) {
	if !customTitle {
		d.layout(gtx)
		return
	}
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			style := material.Decorations(d.theme, decoration,
				system.ActionClose|system.ActionMinimize|system.ActionMaximize, "Wireproxy Desktop")
			style.Background, style.Foreground = selection, textColor
			style.Title.Color, style.Title.Font.Weight, style.Title.TextSize = textColor, font.Bold, 14
			return style.Layout(gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			d.layout(gtx)
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}),
	)
}

func (d *desktop) header() layout.Widget {
	return stack(layout.Horizontal,
		layout.Rigid(d.label(22, [...]string{"Servers", "Settings", "Logs"}[d.page], textColor, font.Bold)),
		layout.Flexed(1, layout.Spacer{}.Layout),
		layout.Rigid(d.tab(0, "Servers")), gap(7),
		layout.Rigid(d.tab(1, "Settings")), gap(7),
		layout.Rigid(d.tab(2, "Logs")),
	)
}

func (d *desktop) tab(index int, name string) layout.Widget {
	if index == d.page {
		return d.button(&d.tabs[index], name, selection, accent, true)
	}
	return d.button(&d.tabs[index], name, background, muted, true)
}

func (d *desktop) statusBar(view sessionView) layout.Widget {
	status, details, ink := view.status, "", muted
	server, pinned := d.defaultProfile()
	name := view.profile
	if status == "" {
		status = "Disconnected"
	}
	if view.ready {
		details = view.address
		if view.protocol != "" {
			details += "  ·  " + view.protocol
		}
		ink = accent
		if status == "Tunnel unavailable" {
			ink = warning
		}
	} else if status == "Proxy failed" {
		details = "See Logs for details"
		ink = warning
	} else if !view.active {
		details = "SOCKS5  ·  " + d.cfg.Settings.bindAddress()
		name = ""
	} else {
		details = view.address
	}
	pinRow := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		if !pinned {
			return layout.Dimensions{}
		}
		return layout.Inset{Bottom: 8}.Layout(gtx, d.rawLabel(14, d.tr("Default server")+"  ·  "+server.Name, muted, font.Medium))
	})
	actionLabel := "Connect"
	target, hasTarget := d.connectionTarget()
	if view.active {
		actionLabel = "Disconnect"
	} else if hasTarget && status != "Proxy failed" {
		name = target.Name
	}
	details = d.tr(details)
	if name != "" && (!pinned || name != server.Name) {
		details = name + "  ·  " + details
	}
	if status == "Stopping" {
		actionLabel = "Disconnecting…"
	}
	return panel(surface, 12, 13, stack(layout.Vertical, pinRow, layout.Rigid(stack(layout.Horizontal,
		layout.Flexed(1, stack(layout.Vertical,
			layout.Rigid(d.label(15, status, ink, font.Bold)),
			layout.Rigid(d.rawLabel(12, details, muted, font.Normal)),
		)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if status != "Stopping" {
				return layout.Dimensions{}
			}
			return layout.Inset{Right: 8}.Layout(gtx, material.Loader(d.theme).Layout)
		}),
		layout.Rigid(d.socksAction(&d.copy, "Copy SOCKS5 URL", surface, muted, view.tunnelReady())),
		gap(7),
		layout.Rigid(d.socksCheckButton(view)),
		gap(7),
		layout.Rigid(d.connectionButton(actionLabel, (view.active || hasTarget) && status != "Stopping")),
	))))
}

func (d *desktop) socksAction(click *widget.Clickable, label string, fill, ink color.NRGBA, enabled bool) layout.Widget {
	if enabled {
		return d.button(click, label, fill, ink, true)
	}
	// The disconnected control has no input tag, gesture handler, or ripple.
	return panel(fill, 8, 0, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 9, Bottom: 9, Left: 15, Right: 15}.Layout(gtx,
			d.label(13, label, hexColor(0x718396), font.Medium))
	})
}

func (d *desktop) socksCheckButton(view sessionView) layout.Widget {
	if !view.tunnelReady() || !view.checking {
		return d.socksAction(&d.check, "Test SOCKS5", inputFill, textColor, view.tunnelReady())
	}
	return panel(inputFill, 8, 0, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 9, Bottom: 9, Left: 13, Right: 13}.Layout(gtx, stack(layout.Horizontal,
			layout.Rigid(d.label(13, "Testing…", textColor, font.Medium)), gap(6),
			layout.Rigid(socksCheckSpinner),
		))
	})
}

func socksCheckSpinner(gtx layout.Context) layout.Dimensions {
	const interval = 80 * time.Millisecond
	size := gtx.Dp(14)
	radius := float64(size) / 3
	dot := max(1, gtx.Dp(1))
	frame := int(gtx.Now.UnixNano() / int64(interval) % 8)
	for i := range 8 {
		angle := float64(i) * math.Pi / 4
		x := size/2 + int(math.Round(radius*math.Cos(angle)))
		y := size/2 + int(math.Round(radius*math.Sin(angle)))
		ink := textColor
		ink.A = uint8(255 - 26*((frame-i+8)%8))
		paint.FillShape(gtx.Ops, ink, clip.Ellipse(image.Rect(x-dot, y-dot, x+dot, y+dot)).Op(gtx.Ops))
	}
	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(interval)})
	return layout.Dimensions{Size: image.Pt(size, size)}
}

func (d *desktop) serversPage(gtx layout.Context, view sessionView) layout.Dimensions {
	return stack(layout.Horizontal,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(220), gtx.Dp(220)
			gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
			return panel(surface, 12, 12, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(stack(layout.Horizontal,
						layout.Flexed(1, d.label(15, "Saved servers", textColor, font.Bold)),
						layout.Rigid(d.button(&d.add, "+ Add", selection, accent, true)),
					)),
					gap(10),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						if len(d.cfg.Profiles) == 0 {
							return d.label(13, "No servers yet", muted, font.Normal)(gtx)
						}
						return material.List(d.theme, &d.profilesList).Layout(gtx, len(d.rows), func(gtx layout.Context, index int) layout.Dimensions {
							return d.serverRow(gtx, index)
						})
					}),
				)
			})(gtx)
		}),
		gap(14),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			var content layout.Widget
			switch {
			case d.editing:
				content = func(gtx layout.Context) layout.Dimensions {
					return material.List(d.theme, &d.profileEditorList).Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
						return d.profileForm()(gtx)
					})
				}
			case d.selected >= 0:
				content = d.profileDetails()
			default:
				content = stack(layout.Vertical,
					layout.Rigid(d.label(22, "Add a server", textColor, font.Bold)),
					gap(10),
					layout.Rigid(d.label(14, "Paste an Amnezia vpn:// link or the contents of a WireGuard configuration file.", muted, font.Normal)),
				)
			}
			return panel(surface, 12, 20, content)(gtx)
		}),
	)(gtx)
}

func (d *desktop) serverRow(gtx layout.Context, index int) layout.Dimensions {
	server, click := d.cfg.Profiles[index], d.rows[index]
	fill, ink := surface, textColor
	if index == d.selected {
		fill, ink = selection, accent
	} else if click.Hovered() {
		fill = inputFill
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return click.Layout(gtx, panel(fill, 8, 0, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: 12, Right: 10, Top: 9, Bottom: 9}.Layout(gtx, stack(layout.Horizontal,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				size := image.Pt(gtx.Dp(7), gtx.Dp(7))
				if server.Name == d.cfg.DefaultProfile {
					paint.FillShape(gtx.Ops, accent, clip.Ellipse(image.Rectangle{Max: size}).Op(gtx.Ops))
				}
				return layout.Dimensions{Size: size}
			}), gap(8),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				style := material.Label(d.theme, 13, server.Name)
				style.Color, style.MaxLines = ink, 1
				return style.Layout(gtx)
			}),
		))
	}))
}

func (d *desktop) profileForm() layout.Widget {
	title := "Edit server"
	if d.selected < 0 {
		title = "Add server"
	}
	if d.showTunnel.Value {
		d.tunnel.Mask = 0
		d.link.Mask = 0
	} else {
		d.tunnel.Mask = '•'
		d.link.Mask = '•'
	}
	editor, hint := &d.link, "vpn://…"
	linkFill, iniFill := selection, inputFill
	if d.editINI {
		editor, hint = &d.tunnel, "[Interface] …"
		linkFill, iniFill = inputFill, selection
	}
	probeLabel := "Check connection"
	if d.probing {
		probeLabel = "Checking…"
	}
	return stack(layout.Vertical,
		layout.Rigid(stack(layout.Horizontal,
			layout.Flexed(1, d.label(22, title, textColor, font.Bold)),
			layout.Rigid(d.button(&d.cancel, "← Back", inputFill, textColor, true)),
		)), gap(14),
		layout.Rigid(d.label(13, "Server name", muted, font.Medium)), gap(5),
		layout.Rigid(d.field(&d.name, "e.g. Home server", 20)), gap(12),
		layout.Rigid(stack(layout.Horizontal,
			layout.Rigid(d.button(&d.linkTab, "vpn://", linkFill, textColor, true)), gap(6),
			layout.Rigid(d.button(&d.iniTab, "INI", iniFill, textColor, true)), gap(6),
			layout.Flexed(1, layout.Spacer{}.Layout),
			layout.Rigid(d.button(&d.copyConfig, "Copy configuration", inputFill, textColor, editor.Len() > 0)),
		)), gap(6),
		layout.Rigid(d.field(editor, hint, 115)),
		layout.Rigid(material.CheckBox(d.theme, &d.showTunnel, d.tr("Show configuration")).Layout),
		gap(9),
		layout.Rigid(stack(layout.Horizontal,
			layout.Rigid(d.button(&d.save, "Save server", selection, textColor, true)), gap(8),
			layout.Rigid(d.button(&d.probe, probeLabel, hexColor(0x354d74), textColor, !d.probing)), gap(8),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if d.probing {
					return material.Loader(d.theme).Layout(gtx)
				}
				return layout.Dimensions{}
			}),
		)),
	)
}

func (d *desktop) profileDetails() layout.Widget {
	p := d.cfg.Profiles[d.selected]
	removeLabel := "Remove"
	if d.deleting {
		removeLabel = "Confirm removal"
	}
	removalActions := []layout.FlexChild{
		layout.Rigid(d.button(&d.edit, "Edit", selection, textColor, true)), gap(7),
		layout.Rigid(d.button(&d.remove, removeLabel, inputFill, warning, true)),
	}
	if d.deleting {
		removalActions = append(removalActions, gap(7), layout.Rigid(d.button(&d.cancelRemoval, "Cancel", selection, textColor, true)))
	}
	return stack(layout.Vertical,
		layout.Rigid(d.rawLabel(24, p.Name, textColor, font.Bold)), gap(8),
		layout.Rigid(d.rawLabel(14, d.tr("Profile protocol")+"  ·  "+d.tr(d.selectedProtocol), accent, font.Medium)), gap(7),
		layout.Rigid(d.rawLabel(13, d.tr("AmneziaWG engine")+"  ·  "+engineVersion, muted, font.Normal)), gap(16),
		layout.Rigid(material.CheckBox(d.theme, &d.pin, d.tr("Default server")).Layout), gap(18),
		layout.Rigid(stack(layout.Horizontal, removalActions...)),
	)
}

func (d *desktop) logsPage(gtx layout.Context, view sessionView) layout.Dimensions {
	gtx.Constraints.Min = gtx.Constraints.Max
	return panel(surface, 12, 18, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(stack(layout.Horizontal,
				layout.Flexed(1, d.label(22, "Activity log", textColor, font.Bold)),
				layout.Rigid(material.CheckBox(d.theme, &d.followLogs, d.tr("Follow log")).Layout), gap(7),
				layout.Rigid(d.button(&d.copyLogs, "Copy logs", selection, textColor, len(view.logs) > 0)), gap(7),
				layout.Rigid(d.button(&d.clearLogs, "Clear", inputFill, textColor, len(view.logs) > 0)),
			)),
			gap(12),
			layout.Flexed(1, panel(inputFill, 8, 12, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				if len(view.logs) == 0 {
					return d.label(13, "Connect to a server to see activity here.", muted, font.Normal)(gtx)
				}
				d.logsList.ScrollToEnd = d.followLogs.Value || d.logJump
				if d.logsList.ScrollToEnd {
					d.logsList.Position.BeforeEnd = false
				}
				d.logJump = false
				if d.followLogs.Value {
					gtx = gtx.Disabled()
				}
				return material.List(d.theme, &d.logsList).Layout(gtx, len(view.logs), func(gtx layout.Context, i int) layout.Dimensions {
					return layout.Inset{Bottom: 6}.Layout(gtx, d.rawLabel(13, view.logs[i], textColor, font.Normal))
				})
			})),
		)
	})(gtx)
}
