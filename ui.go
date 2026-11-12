package main

import (
	"image"
	"image/color"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/io/clipboard"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

var (
	background = hexColor(0x101722)
	surface    = hexColor(0x192330)
	inputFill  = hexColor(0x111a26)
	selection  = hexColor(0x233849)
	textColor  = hexColor(0xe9f1f7)
	muted      = hexColor(0xa3b5c7)
	accent     = hexColor(0x63e6c5)
	warning    = hexColor(0xffb597)
)

type desktop struct {
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
	path                                                  string
	cfg                                                   configuration
	session                                               proxySession
	loadErr                                               error
	message                                               string
	messageError                                          bool
	page, selected                                        int
	rows                                                  []*widget.Clickable
	profilesList, logsList                                widget.List
	name, tunnel, port                                    widget.Editor
	verbose, showTunnel                                   widget.Bool
	tabs                                                  [3]widget.Clickable
	add, edit, connect, check, copy, save, cancel, remove widget.Clickable
	saveSettings, clearLogs, copyLogs                     widget.Clickable
	editing, deleting                                     bool
}

func newDesktop(path string, pathErr error, notify func()) *desktop {
	d := &desktop{theme: material.NewTheme(), path: path, selected: -1, loadErr: pathErr,
		probeResult: make(chan profileProbeResult, 1)}
	d.theme.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	d.theme.Palette = material.Palette{
		Bg: background, Fg: textColor, ContrastBg: accent, ContrastFg: background,
	}
	d.session.notify = notify
	d.profilesList.Axis, d.logsList.Axis = layout.Vertical, layout.Vertical
	d.settingsList.Axis = layout.Vertical
	d.profileEditorList.Axis = layout.Vertical
	d.name.SingleLine, d.port.SingleLine = true, true
	d.username.SingleLine, d.password.SingleLine = true, true
	d.link.WrapPolicy = text.WrapGraphemes
	d.password.Mask = '•'
	d.port.Filter = "0123456789"
	d.tunnel.WrapPolicy = text.WrapGraphemes
	if d.loadErr == nil {
		d.cfg, d.loadErr = loadConfig(path)
	}
	if d.loadErr != nil {
		d.feedback("Could not load profiles. "+d.loadErr.Error(), true)
	}
	d.loadSettings()
	d.refreshRows()
	if len(d.cfg.Profiles) > 0 {
		d.selected = 0
		d.refreshProtocol()
	}
	return d
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
}

func (d *desktop) persist(cfg configuration) bool {
	if d.loadErr != nil {
		d.feedback("Fix the profiles file and restart before saving changes.", true)
		return false
	}
	if err := saveConfig(d.path, cfg); err != nil {
		d.feedback("Could not save profiles. "+err.Error(), true)
		return false
	}
	d.cfg = cfg
	return true
}

func (d *desktop) beginEdit(index int) {
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
	if d.persist(cfg) {
		d.selected = index
		d.endEdit()
		d.refreshRows()
		d.feedback("Server saved.", false)
	}
}

func (d *desktop) deleteProfile() {
	cfg := d.cfg
	cfg.Profiles = append([]profile(nil), cfg.Profiles...)
	if cfg.DefaultProfile == cfg.Profiles[d.selected].Name {
		cfg.DefaultProfile = ""
	}
	cfg.Profiles = append(cfg.Profiles[:d.selected], cfg.Profiles[d.selected+1:]...)
	if d.persist(cfg) {
		d.selected = min(d.selected, len(cfg.Profiles)-1)
		d.deleting = false
		d.refreshRows()
		d.refreshProtocol()
		d.feedback("Server removed.", false)
	}
}

func (d *desktop) actions(gtx layout.Context, view sessionView) {
	d.profileActions(gtx)
	for index := range d.tabs {
		if d.tabs[index].Clicked(gtx) && d.page != index {
			d.page = index
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
		}
		if d.remove.Clicked(gtx) && d.selected >= 0 {
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
	if d.check.Clicked(gtx) {
		d.session.check()
		d.page = 2
		d.logsList.Position.First = 0
	}
	if d.copy.Clicked(gtx) && view.ready {
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader("socks5://" + proxyClientAddress(view.address)))})
		d.feedback("SOCKS5 address copied.", false)
	}
	if d.page == 1 {
		d.settingsActions(gtx, view)
	}
	if d.page == 2 {
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
	view := d.session.view(d.page == 2)
	d.actions(gtx, view)
	view = d.session.view(d.page == 2)
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
				if d.message == "" {
					return layout.Dimensions{}
				}
				ink := accent
				if d.messageError {
					ink = warning
				}
				return layout.Inset{Top: 12}.Layout(gtx, d.label(13, d.message, ink, font.Medium))
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
	status, details, ink := view.status, view.profile, muted
	if status == "" {
		status = "Disconnected"
	}
	if view.ready {
		details = view.profile + "  ·  " + view.address
		if view.protocol != "" {
			details += "  ·  " + view.protocol
		}
		ink = accent
	} else if status == "Proxy failed" {
		details = "See Logs for details"
		ink = warning
	} else if !view.active {
		details = "SOCKS5  ·  " + d.cfg.Settings.bindAddress()
	}
	checkLabel := "Test SOCKS5"
	if view.checking {
		checkLabel = "Testing…"
	}
	server, pinned := d.defaultProfile()
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
		details = target.Name + "  ·  " + d.cfg.Settings.bindAddress()
	}
	if status == "Stopping" {
		actionLabel = "Disconnecting…"
	}
	return panel(surface, 12, 13, stack(layout.Vertical, pinRow, layout.Rigid(stack(layout.Horizontal,
		layout.Flexed(1, stack(layout.Vertical,
			layout.Rigid(d.label(15, status, ink, font.Bold)),
			layout.Rigid(d.label(12, details, muted, font.Normal)),
		)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if status != "Stopping" {
				return layout.Dimensions{}
			}
			return layout.Inset{Right: 8}.Layout(gtx, material.Loader(d.theme).Layout)
		}),
		layout.Rigid(d.button(&d.copy, "Copy address", inputFill, textColor, view.ready)),
		gap(7),
		layout.Rigid(d.button(&d.check, checkLabel, inputFill, textColor, view.ready && !view.checking)),
		gap(7),
		layout.Rigid(d.connectionButton(actionLabel, (view.active || hasTarget) && status != "Stopping")),
	))))
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
							fill, ink := surface, textColor
							if index == d.selected {
								fill, ink = selection, accent
							}
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							return d.rawButton(d.rows[index], d.cfg.Profiles[index].Name, fill, ink, true)(gtx)
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
			layout.Rigid(d.button(&d.probe, probeLabel, hexColor(0x354d74), textColor, !d.probing)),
		)),
	)
}

func (d *desktop) profileDetails() layout.Widget {
	p := d.cfg.Profiles[d.selected]
	format := "WireGuard / AmneziaWG INI"
	if strings.HasPrefix(p.Config, "vpn://") {
		format = "Amnezia VPN link"
	}
	removeLabel := "Remove"
	if d.deleting {
		removeLabel = "Confirm removal"
	}
	return stack(layout.Vertical,
		layout.Rigid(d.rawLabel(24, p.Name, textColor, font.Bold)), gap(8),
		layout.Rigid(d.rawLabel(14, d.tr("Protocol")+"  ·  "+d.tr(d.selectedProtocol), accent, font.Medium)), gap(7),
		layout.Rigid(d.label(14, format, muted, font.Normal)), gap(16),
		layout.Rigid(material.CheckBox(d.theme, &d.pin, d.tr("Default server")).Layout), gap(18),
		layout.Rigid(stack(layout.Horizontal,
			layout.Rigid(d.button(&d.edit, "Edit", selection, textColor, true)), gap(7),
			layout.Rigid(d.button(&d.remove, removeLabel, inputFill, warning, true)),
		)),
	)
}

func (d *desktop) logsPage(gtx layout.Context, view sessionView) layout.Dimensions {
	gtx.Constraints.Min = gtx.Constraints.Max
	return panel(surface, 12, 18, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(stack(layout.Horizontal,
				layout.Flexed(1, d.label(22, "Activity log", textColor, font.Bold)),
				layout.Rigid(d.button(&d.copyLogs, "Copy logs", selection, textColor, len(view.logs) > 0)), gap(7),
				layout.Rigid(d.button(&d.clearLogs, "Clear", inputFill, textColor, len(view.logs) > 0)),
			)),
			gap(12),
			layout.Flexed(1, panel(inputFill, 8, 12, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				if len(view.logs) == 0 {
					return d.label(13, "Connect to a server to see activity here.", muted, font.Normal)(gtx)
				}
				return material.List(d.theme, &d.logsList).Layout(gtx, len(view.logs), func(gtx layout.Context, i int) layout.Dimensions {
					return layout.Inset{Bottom: 6}.Layout(gtx, d.label(13, view.logs[len(view.logs)-1-i], textColor, font.Normal))
				})
			})),
		)
	})(gtx)
}
