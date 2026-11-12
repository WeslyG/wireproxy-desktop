package main

import (
	"fmt"
	"image"
	"net"
	"net/netip"
	"strconv"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type interfaceOption struct {
	address, name string
	click         widget.Clickable
}

var languageChoices = [...]struct{ code, name string }{
	{"system", "System language"},
	{"en", "English"},
	{"ru", "Русский"},
}

func listenInterfaces() ([]interfaceOption, error) {
	options := []interfaceOption{
		{address: "0.0.0.0", name: "All IPv4 interfaces"},
		{address: "::", name: "All IPv6 interfaces"},
	}
	interfaces, err := net.Interfaces()
	seen := make(map[string]bool)
	for _, device := range interfaces {
		if device.Flags&net.FlagUp == 0 {
			continue
		}
		addresses, err := device.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			prefix, err := netip.ParsePrefix(address.String())
			if err != nil {
				continue
			}
			ip := prefix.Addr()
			if ip.Is6() && ip.IsLinkLocalUnicast() {
				ip = ip.WithZone(strconv.Itoa(device.Index))
			}
			if !seen[ip.String()] {
				options = append(options, interfaceOption{address: ip.String(), name: device.Name})
				seen[ip.String()] = true
			}
		}
	}
	return options, err
}

func (d *desktop) settingsActions(gtx layout.Context, view sessionView) {
	if d.dismissMenu.Clicked(gtx) {
		d.interfaceOpen, d.languageOpen = false, false
	}
	for index := range d.settingsTabs {
		if d.settingsTabs[index].Clicked(gtx) && d.settingsTab != index {
			d.settingsTab = index
			d.settingsList.Position = layout.Position{}
			d.interfaceOpen, d.languageOpen = false, false
			d.feedback("", false)
		}
	}
	if d.settingsTab == 1 {
		if d.languageMenu.Clicked(gtx) {
			d.languageOpen = !d.languageOpen
			d.settingsMenuList.Position = layout.Position{}
		}
		for index, choice := range languageChoices {
			if d.languageOptions[index].Clicked(gtx) {
				cfg := d.cfg
				cfg.Settings.Language = choice.code
				cfg.Settings.LanguageOverride = choice.code != "system"
				d.persist(cfg, func(err error) {
					if err == nil {
						d.languageOpen = false
						d.feedback("Language saved.", false)
					}
				})
				return
			}
		}
		return
	}
	if d.settingsTab != 0 {
		return
	}
	d.auth.Update(gtx)
	if d.interfaceMenu.Clicked(gtx) {
		d.interfaceOpen = !d.interfaceOpen
		if d.interfaceOpen {
			d.settingsMenuList.Position = layout.Position{}
			var err error
			d.interfaces, err = listenInterfaces()
			if err != nil {
				d.feedback(d.tr("Could not list interfaces")+" · "+err.Error(), true)
			}
		}
	}
	for index := range d.interfaces {
		option := &d.interfaces[index]
		if option.click.Clicked(gtx) {
			d.host, d.interfaceOpen, d.suggestedPort = option.address, false, 0
			d.feedback("", false)
		}
	}
	if d.checkPort.Clicked(gtx) {
		d.verifyPort(view)
	}
	if d.usePort.Clicked(gtx) && d.suggestedPort != 0 {
		d.port.SetText(strconv.Itoa(d.suggestedPort))
		d.suggestedPort = 0
		d.feedback("Suggested port selected.", false)
	}
	if d.saveSettings.Clicked(gtx) {
		port, _ := strconv.Atoi(d.port.Text())
		next := d.cfg.Settings
		next.Host, next.Port, next.Verbose = d.host, port, d.verbose.Value
		next.Username, next.Password = "", ""
		if d.auth.Value {
			next.Username, next.Password = d.username.Text(), d.password.Text()
			if next.Username == "" || next.Password == "" {
				d.feedback("Enter both SOCKS5 credentials, up to 255 bytes each.", true)
				return
			}
		}
		if err := next.validate(); err != nil {
			d.feedback(err.Error(), true)
			return
		}
		cfg := d.cfg
		cfg.Settings = next
		d.persist(cfg, func(err error) {
			if err != nil {
				return
			}
			message := "Settings saved."
			if view.active {
				message = "Settings saved. Reconnect to apply them."
			}
			d.feedback(message, false)
		})
	}
}

func (d *desktop) verifyPort(view sessionView) {
	d.suggestedPort = 0
	port, _ := strconv.Atoi(d.port.Text())
	options := settings{Host: d.host, Port: port}
	if err := options.validate(); err != nil {
		d.feedback(err.Error(), true)
		return
	}
	if view.active && listenerOverlaps(view.address, options.bindAddress()) {
		message := "Wireproxy is using this port"
		if !view.ready {
			message = "Wireproxy is changing the listener state"
		}
		d.feedback(d.tr(message)+" · "+view.address, false)
		return
	}
	listener, err := net.Listen("tcp", options.bindAddress())
	if err == nil {
		listener.Close()
		d.feedback(d.tr("Port is available")+" · "+options.bindAddress(), false)
		return
	}
	alternative := options
	alternative.Port = 0
	if free, bindErr := net.Listen("tcp", alternative.bindAddress()); bindErr == nil {
		d.suggestedPort = free.Addr().(*net.TCPAddr).Port
		free.Close()
	}
	d.feedback(d.tr("Cannot bind to this port")+" · "+err.Error(), true)
}

func listenerOverlaps(first, second string) bool {
	a, errA := netip.ParseAddrPort(first)
	b, errB := netip.ParseAddrPort(second)
	if errA != nil || errB != nil || a.Port() != b.Port() {
		return false
	}
	hostA, hostB := a.Addr().Unmap(), b.Addr().Unmap()
	return hostA == hostB || (hostA.Is4() == hostB.Is4() && (hostA.IsUnspecified() || hostB.IsUnspecified()))
}

func (d *desktop) settingsPage(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Min = gtx.Constraints.Max
	return panel(surface, 12, 20, func(gtx layout.Context) layout.Dimensions {
		if d.interfaceOpen || d.languageOpen {
			macro := op.Record(gtx.Ops)
			d.dismissMenu.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
			op.Defer(gtx.Ops, macro.Stop())
		}
		content := d.proxySettings
		switch d.settingsTab {
		case 1:
			content = d.languageSettings
		case 2:
			content = d.updates.layout(d)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(d.settingsTabRail), gap(20),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return material.List(d.theme, &d.settingsList).Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
					return layout.Inset{Right: 7, Bottom: 8}.Layout(gtx, content)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if d.settingsTab != 0 {
					return layout.Dimensions{}
				}
				return d.proxySettingsFooter(gtx)
			}),
		)
	})(gtx)
}

func (d *desktop) settingsTabRail(gtx layout.Context) layout.Dimensions {
	width := gtx.Constraints.Max.X
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			size := gtx.Constraints.Min
			paint.FillShape(gtx.Ops, selection, clip.Rect(image.Rect(0, size.Y-1, size.X, size.Y)).Op())
			return layout.Dimensions{Size: size}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			tabs := make([]layout.FlexChild, 0, len(d.settingsTabs))
			for index, name := range [...]string{"Proxy", "Language", "Updates"} {
				tabs = append(tabs, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return d.settingsTabs[index].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						ink := muted
						if index == d.settingsTab || d.settingsTabs[index].Hovered() {
							ink = textColor
						}
						dims := layout.Inset{Left: 20, Right: 20, Top: 10, Bottom: 12}.Layout(gtx, d.label(15, name, ink, font.Medium))
						if index == d.settingsTab {
							line := image.Rect(0, dims.Size.Y-gtx.Dp(3), dims.Size.X, dims.Size.Y)
							paint.FillShape(gtx.Ops, accent, clip.Rect(line).Op())
						}
						return dims
					})
				}))
			}
			dims := stack(layout.Horizontal, tabs...)(gtx)
			dims.Size.X = width
			return dims
		}),
	)
}

type selectOption struct {
	title, subtitle string
	selected        bool
	click           *widget.Clickable
}

func (d *desktop) settingsSelect(click *widget.Clickable, title string, expanded bool, options []selectOption) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		width := gtx.Constraints.Max.X
		gtx.Constraints.Min.X = width
		outline := widget.Border{Color: selection, Width: 1, CornerRadius: 8}
		dims := click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return outline.Layout(gtx, panel(inputFill, 8, 0, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: 12, Right: 12, Top: 10, Bottom: 10}.Layout(gtx, stack(layout.Horizontal,
					layout.Flexed(1, d.rawLabel(14, title, textColor, font.Medium)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return selectGlyph(gtx, false, true)
					}),
				))
			}))
		})
		if !expanded || len(options) == 0 {
			return dims
		}
		macro := op.Record(gtx.Ops)
		shift := op.Offset(image.Pt(0, dims.Size.Y+gtx.Dp(4))).Push(gtx.Ops)
		menu := gtx
		menu.Constraints.Min = image.Pt(width, 0)
		menu.Constraints.Max = image.Pt(width, gtx.Dp(222))
		outline.Layout(menu, panel(hexColor(0x202f40), 8, 4, func(gtx layout.Context) layout.Dimensions {
			return material.List(d.theme, &d.settingsMenuList).Layout(gtx, len(options), func(gtx layout.Context, index int) layout.Dimensions {
				option := options[index]
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				fill := hexColor(0x202f40)
				if option.selected {
					fill = selection
				}
				if option.click.Hovered() {
					fill = hexColor(0x31485c)
				}
				return option.click.Layout(gtx, panel(fill, 5, 8, stack(layout.Horizontal,
					layout.Flexed(1, stack(layout.Vertical,
						layout.Rigid(d.rawLabel(14, option.title, textColor, font.Medium)),
						layout.Rigid(d.rawLabel(11, option.subtitle, muted, font.Normal)),
					)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return selectGlyph(gtx, true, option.selected)
					}),
				)))
			})
		}))
		shift.Pop()
		op.Defer(gtx.Ops, macro.Stop())
		return dims
	}
}

func selectGlyph(gtx layout.Context, check, visible bool) layout.Dimensions {
	size := gtx.Dp(16)
	if visible {
		unit := float32(size) / 16
		var path clip.Path
		path.Begin(gtx.Ops)
		if check {
			path.MoveTo(f32.Pt(3*unit, 8*unit))
			path.LineTo(f32.Pt(6*unit, 11*unit))
			path.LineTo(f32.Pt(13*unit, 4*unit))
		} else {
			path.MoveTo(f32.Pt(3*unit, 6*unit))
			path.LineTo(f32.Pt(8*unit, 11*unit))
			path.LineTo(f32.Pt(13*unit, 6*unit))
		}
		paint.FillShape(gtx.Ops, accent, clip.Stroke{Path: path.End(), Width: 1.7 * unit}.Op())
	}
	return layout.Dimensions{Size: image.Pt(size, size)}
}

func (d *desktop) languageSettings(gtx layout.Context) layout.Dimensions {
	selected := d.tr(languageChoices[0].name)
	options := make([]selectOption, 0, len(languageChoices))
	for index, choice := range languageChoices {
		current := choice.code == d.cfg.Settings.Language || (choice.code == "system" && d.cfg.Settings.Language == "")
		if current {
			selected = d.tr(choice.name)
		}
		options = append(options, selectOption{title: d.tr(choice.name), selected: current, click: &d.languageOptions[index]})
	}
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(390))
	gtx.Constraints.Min.X = 0
	return stack(layout.Vertical,
		layout.Rigid(d.label(14, "Interface language", muted, font.Medium)), gap(7),
		layout.Rigid(d.settingsSelect(&d.languageMenu, selected, d.languageOpen, options)),
	)(gtx)
}

func (d *desktop) proxySettings(gtx layout.Context) layout.Dimensions {
	host := d.host
	if host == "" {
		host = "127.0.0.1"
	}
	options := make([]selectOption, 0, len(d.interfaces))
	for index := range d.interfaces {
		item := &d.interfaces[index]
		options = append(options, selectOption{item.address, d.tr(item.name), item.address == host, &item.click})
	}
	listener := []layout.FlexChild{
		layout.Rigid(d.label(17, "Listener", textColor, font.Bold)), gap(14),
		layout.Rigid(d.label(13, "SOCKS5 listen address", muted, font.Medium)), gap(6),
		layout.Rigid(d.settingsSelect(&d.interfaceMenu, host, d.interfaceOpen, options)), gap(14),
		layout.Rigid(d.label(13, "SOCKS5 port", muted, font.Medium)), gap(6),
		layout.Rigid(stack(layout.Horizontal,
			layout.Flexed(1, d.field(&d.port, "1080", 20)), gap(10),
			layout.Rigid(d.button(&d.checkPort, "Check port", selection, textColor, true)),
		)),
	}
	if d.suggestedPort != 0 {
		listener = append(listener, gap(8), layout.Rigid(d.rawButton(&d.usePort, fmt.Sprintf("%s %d", d.tr("Use port"), d.suggestedPort), selection, textColor, true)))
	}
	access := []layout.FlexChild{
		layout.Rigid(d.label(17, "Authentication", textColor, font.Bold)), gap(9),
		layout.Rigid(material.CheckBox(d.theme, &d.auth, d.tr("Use SOCKS5 authentication")).Layout),
	}
	if d.auth.Value {
		access = append(access,
			gap(8), layout.Rigid(d.label(13, "SOCKS5 username", muted, font.Medium)), gap(5),
			layout.Rigid(d.field(&d.username, "", 20)),
			gap(9), layout.Rigid(d.label(13, "SOCKS5 password", muted, font.Medium)), gap(5),
			layout.Rigid(d.field(&d.password, "", 20)),
		)
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
		layout.Flexed(1, stack(layout.Vertical, listener...)), gap(27),
		layout.Flexed(1, stack(layout.Vertical, access...)),
	)
}

func (d *desktop) proxySettingsFooter(gtx layout.Context) layout.Dimensions {
	return stack(layout.Vertical,
		layout.Rigid(material.CheckBox(d.theme, &d.verbose, d.tr("Verbose WireGuard logs")).Layout),
		gap(8),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			size := image.Pt(gtx.Constraints.Max.X, 1)
			paint.FillShape(gtx.Ops, selection, clip.Rect{Max: size}.Op())
			return layout.Dimensions{Size: size}
		}), gap(12),
		layout.Rigid(stack(layout.Horizontal,
			layout.Flexed(1, stack(layout.Vertical,
				layout.Rigid(d.label(13, "Protected storage", muted, font.Medium)),
				layout.Rigid(d.rawLabel(12, protectedStoreName(), muted, font.Normal)),
			)),
			gap(14),
			layout.Rigid(d.button(&d.saveSettings, "Save settings", selection, textColor, true)),
		)),
	)(gtx)
}
