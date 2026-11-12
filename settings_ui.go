package main

import (
	"fmt"
	"net"
	"net/netip"
	"strconv"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type interfaceOption struct {
	address, name string
	click         widget.Clickable
}

var languageChoices = [...]struct{ code, name string }{
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
		}
		for index, choice := range languageChoices {
			if d.languageOptions[index].Clicked(gtx) {
				cfg := d.cfg
				cfg.Settings.Language = choice.code
				if d.persist(cfg) {
					d.languageOpen = false
					d.feedback("Language saved.", false)
				}
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
		d.verifyPort()
	}
	if d.usePort.Clicked(gtx) && d.suggestedPort != 0 {
		d.port.SetText(strconv.Itoa(d.suggestedPort))
		d.suggestedPort = 0
		d.feedback("Suggested port selected.", false)
	}
	if d.saveSettings.Clicked(gtx) {
		port, _ := strconv.Atoi(d.port.Text())
		next := settings{Host: d.host, Port: port, Verbose: d.verbose.Value, Language: d.cfg.Settings.Language}
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
		if d.persist(cfg) {
			message := "Settings saved."
			if view.active {
				message = "Settings saved. Reconnect to apply them."
			}
			d.feedback(message, false)
		}
	}
}

func (d *desktop) verifyPort() {
	d.suggestedPort = 0
	port, _ := strconv.Atoi(d.port.Text())
	options := settings{Host: d.host, Port: port}
	if err := options.validate(); err != nil {
		d.feedback(err.Error(), true)
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

func (d *desktop) settingsPage(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Min = gtx.Constraints.Max
	return panel(surface, 12, 20, func(gtx layout.Context) layout.Dimensions {
		tabs := []layout.FlexChild{}
		for index, name := range [...]string{"Proxy", "Language", "Updates"} {
			fill := inputFill
			if index == d.settingsTab {
				fill = selection
			}
			tabs = append(tabs, layout.Rigid(d.button(&d.settingsTabs[index], name, fill, textColor, true)), gap(8))
		}
		rows := d.settingsRows()
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(stack(layout.Horizontal, tabs...)), gap(16),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return material.List(d.theme, &d.settingsList).Layout(gtx, len(rows), func(gtx layout.Context, index int) layout.Dimensions {
					return layout.Inset{Bottom: 10, Right: 5}.Layout(gtx, rows[index])
				})
			}),
		)
	})(gtx)
}

func (d *desktop) settingsRows() []layout.Widget {
	if d.settingsTab == 1 {
		selected := languageChoices[0].name
		for _, choice := range languageChoices {
			if choice.code == d.cfg.Settings.Language {
				selected = choice.name
			}
		}
		rows := []layout.Widget{d.rawButton(&d.languageMenu, selected+"  ▾", selection, textColor, true)}
		if d.languageOpen {
			for index, choice := range languageChoices {
				rows = append(rows, d.rawButton(&d.languageOptions[index], choice.name, inputFill, textColor, true))
			}
		}
		return rows
	}
	if d.settingsTab == 2 {
		return []layout.Widget{d.updates.layout(d)}
	}
	return d.proxySettingsRows()
}

func (d *desktop) proxySettingsRows() []layout.Widget {
	field := func(label, hint string, editor *widget.Editor) layout.Widget {
		return stack(layout.Vertical,
			layout.Rigid(d.label(13, label, muted, font.Medium)), gap(5),
			layout.Rigid(d.field(editor, hint, 20)),
		)
	}
	host := d.host
	if host == "" {
		host = "127.0.0.1"
	}
	rows := []layout.Widget{
		d.label(13, "SOCKS5 listen address", muted, font.Medium),
		d.rawButton(&d.interfaceMenu, host+"  ▾", selection, textColor, true),
	}
	if d.interfaceOpen {
		for index := range d.interfaces {
			option := &d.interfaces[index]
			rows = append(rows, d.rawButton(&option.click, option.address+"  ·  "+d.tr(option.name), inputFill, textColor, true))
		}
	}
	rows = append(rows,
		d.label(13, "SOCKS5 port", muted, font.Medium),
		stack(layout.Horizontal,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(115))
				gtx.Constraints.Min.X = 0
				return d.field(&d.port, "1080", 20)(gtx)
			}), gap(10),
			layout.Rigid(d.button(&d.checkPort, "Check port", selection, textColor, true)),
		),
	)
	if d.suggestedPort != 0 {
		rows = append(rows, d.rawButton(&d.usePort, fmt.Sprintf("%s %d", d.tr("Use port"), d.suggestedPort), selection, textColor, true))
	}
	rows = append(rows, material.CheckBox(d.theme, &d.auth, d.tr("Use SOCKS5 authentication")).Layout)
	if d.auth.Value {
		rows = append(rows, stack(layout.Horizontal,
			layout.Flexed(1, field("SOCKS5 username", "", &d.username)), gap(12),
			layout.Flexed(1, field("SOCKS5 password", "", &d.password)),
		))
	}
	return append(rows,
		material.CheckBox(d.theme, &d.verbose, d.tr("Verbose WireGuard logs")).Layout,
		d.button(&d.saveSettings, "Save settings", selection, textColor, true),
		d.label(13, "Profiles file", muted, font.Medium),
		d.rawLabel(13, d.path, textColor, font.Normal),
	)
}
