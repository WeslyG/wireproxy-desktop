package main

import (
	"context"
	"errors"
	"io"
	"strings"

	"gioui.org/io/clipboard"
	"gioui.org/layout"
)

type profileProbeResult struct {
	config string
	err    error
}

func (d *desktop) draftConfig() string {
	if d.editINI {
		return strings.TrimSpace(d.tunnel.Text())
	}
	return strings.Join(strings.Fields(d.link.Text()), "")
}

func (d *desktop) refreshProtocol() {
	d.selectedProtocol = ""
	d.pin.Value = false
	if d.selected >= 0 {
		d.pin.Value = d.cfg.DefaultProfile == d.cfg.Profiles[d.selected].Name
		d.selectedProtocol = profileProtocol(d.cfg.Profiles[d.selected].Config)
		if d.selectedProtocol == "" {
			d.selectedProtocol = "Unknown"
		}
	}
}

func (d *desktop) defaultProfile() (profile, bool) {
	for _, server := range d.cfg.Profiles {
		if server.Name == d.cfg.DefaultProfile {
			return server, true
		}
	}
	return profile{}, false
}

func (d *desktop) connectionTarget() (profile, bool) {
	if d.page == 0 && !d.editing && d.selected >= 0 {
		return d.cfg.Profiles[d.selected], true
	}
	if server, ok := d.defaultProfile(); ok {
		return server, true
	}
	if d.selected >= 0 && !d.editing {
		return d.cfg.Profiles[d.selected], true
	}
	return profile{}, false
}

func (d *desktop) profileActions(gtx layout.Context) {
	select {
	case result := <-d.probeResult:
		d.probing = false
		d.cancelConnectionProbe()
		if d.editing && result.config == d.draftConfig() && !errors.Is(result.err, context.Canceled) {
			if result.err == nil {
				d.feedback("HTTPS works through the tunnel.", false)
			} else {
				d.feedback(d.tr("Tunnel test failed")+" · "+d.tr(result.err.Error()), true)
			}
		}
	default:
	}
	if d.page != 0 {
		return
	}
	if !d.editing && d.selected >= 0 {
		d.pin.Update(gtx)
		name := d.cfg.Profiles[d.selected].Name
		pinned := d.cfg.DefaultProfile == name
		if d.pin.Value != pinned {
			cfg := d.cfg
			cfg.DefaultProfile = ""
			if d.pin.Value {
				cfg.DefaultProfile = name
			}
			d.persist(cfg, func(err error) {
				if err != nil {
					d.pin.Value = pinned
				}
			})
		}
	}
	if !d.editing {
		return
	}
	d.showTunnel.Update(gtx)
	if d.linkTab.Clicked(gtx) {
		d.editINI = false
	}
	if d.iniTab.Clicked(gtx) {
		d.editINI = true
		link := strings.Join(strings.Fields(d.link.Text()), "")
		if link != "" && link != d.convertedLink && (d.tunnel.Len() == 0 || d.tunnel.Text() == d.convertedINI) {
			converted, err := convertProfile(link)
			if err != nil {
				d.editINI = false
				d.feedback(d.tr("Conversion failed")+" · "+d.tr(err.Error()), true)
			} else {
				d.tunnel.SetText(converted)
				d.convertedLink, d.convertedINI = link, converted
				d.feedback("", false)
			}
		}
	}
	if d.copyConfig.Clicked(gtx) {
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(d.draftConfig()))})
		d.feedback("Configuration copied.", false)
	}
	if d.probe.Clicked(gtx) && !d.probing {
		config := d.draftConfig()
		d.probing = true
		d.feedback("Checking the tunnel…", false)
		ctx, cancel := context.WithCancel(context.Background())
		d.cancelProbe = cancel
		go func() {
			err := probeInProcess(ctx, config)
			d.probeResult <- profileProbeResult{config, err}
			d.session.notify()
		}()
	}
}

func (d *desktop) cancelConnectionProbe() {
	if d.cancelProbe != nil {
		d.cancelProbe()
		d.cancelProbe = nil
	}
}

func (d *desktop) connectProfile(server profile) {
	d.feedback("", false)
	if err := d.session.start(server, d.cfg.Settings); err != nil {
		d.feedback(err.Error(), true)
	}
}
