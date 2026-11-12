package main

import (
	"strings"

	"gioui.org/layout"
)

type profileProbeResult struct {
	config, protocol string
	err              error
}

func (d *desktop) draftConfig() string {
	if d.editINI {
		return strings.TrimSpace(d.tunnel.Text())
	}
	return strings.TrimSpace(d.link.Text())
}

func (d *desktop) refreshProtocol() {
	d.selectedProtocol = ""
	if d.selected >= 0 {
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

func (d *desktop) profileActions(gtx layout.Context, view sessionView) {
	select {
	case result := <-d.probeResult:
		d.probing = false
		if d.editing && result.config == d.draftConfig() {
			switch {
			case result.err == nil:
				d.feedback(d.tr("Tunnel test passed")+" · "+result.protocol, false)
			case result.protocol != "":
				d.feedback(d.tr("Tunnel test failed")+" · "+result.protocol+" · "+result.err.Error(), true)
			default:
				d.feedback(result.err.Error(), true)
			}
		}
	default:
	}
	if d.quickConnect.Clicked(gtx) && !view.active {
		if server, ok := d.defaultProfile(); ok {
			d.connectProfile(server)
		}
	}
	if d.page != 0 {
		return
	}
	if d.pin.Clicked(gtx) && !d.editing && d.selected >= 0 {
		cfg := d.cfg
		cfg.DefaultProfile = d.cfg.Profiles[d.selected].Name
		if cfg.DefaultProfile == d.cfg.DefaultProfile {
			cfg.DefaultProfile = ""
		}
		d.persist(cfg)
	}
	if !d.editing {
		return
	}
	if d.linkTab.Clicked(gtx) {
		d.editINI = false
	}
	if d.iniTab.Clicked(gtx) {
		d.editINI = true
	}
	if d.convert.Clicked(gtx) {
		converted, err := convertProfile(d.link.Text())
		if err != nil {
			d.feedback(d.tr("Conversion failed")+" · "+d.tr(err.Error()), true)
		} else {
			d.tunnel.SetText(converted)
			d.editINI = true
			d.showTunnel.Value = true
			d.feedback("Converted to INI.", false)
		}
	}
	if d.probe.Clicked(gtx) && !d.probing {
		config := d.draftConfig()
		d.probing = true
		d.feedback("Checking the tunnel…", false)
		go func() {
			protocol, err := probeProfile(config)
			d.probeResult <- profileProbeResult{config, protocol, err}
			d.session.notify()
		}()
	}
}

func (d *desktop) connectProfile(server profile) {
	d.feedback("", false)
	if err := d.session.start(server, d.cfg.Settings); err != nil {
		d.feedback(err.Error(), true)
	}
}
