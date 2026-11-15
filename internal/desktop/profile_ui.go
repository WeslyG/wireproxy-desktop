package desktop

import (
	"context"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"gioui.org/io/clipboard"
	"gioui.org/layout"

	"github.com/WeslyG/wireproxy-desktop/internal/config"
	"github.com/WeslyG/wireproxy-desktop/internal/profile"
	"github.com/WeslyG/wireproxy-desktop/internal/tunnel"
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
		d.selectedProtocol = profile.Protocol(d.cfg.Profiles[d.selected].Config)
		if d.selectedProtocol == "" {
			d.selectedProtocol = "Unknown"
		}
	}
}

func (d *desktop) connectionTarget() (config.Profile, bool) {
	if d.page == 0 && !d.editing && d.selected >= 0 {
		return d.cfg.Profiles[d.selected], true
	}
	if server, ok := d.defaultProfile(); ok {
		return server, true
	}
	if d.selected >= 0 && !d.editing {
		return d.cfg.Profiles[d.selected], true
	}
	return config.Profile{}, false
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
			converted, err := profile.Convert(link)
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
			err := tunnel.Probe(ctx, config)
			d.probeResult <- profileProbeResult{config, err}
			d.session.Notify()
		}()
	}
}

func (d *desktop) cancelConnectionProbe() {
	if d.cancelProbe != nil {
		d.cancelProbe()
		d.cancelProbe = nil
	}
}

func (d *desktop) connectProfile(server config.Profile) {
	d.feedback("", false)
	if err := d.session.Start(server, d.cfg.Settings); err != nil {
		d.feedback(err.Error(), true)
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
		d.editINI = !strings.HasPrefix(strings.TrimSpace(config), profile.AmneziaScheme)
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
	p := config.Profile{Name: strings.TrimSpace(d.name.Text()), Config: d.draftConfig()}
	if d.editINI && p.Config != "" && p.Config == strings.TrimSpace(d.convertedINI) {
		p.Config = d.convertedLink
	}
	if p.Name == "" || utf8.RuneCountInString(p.Name) > 96 || p.Config == "" || len(p.Config) > profile.MaxConfigLength {
		d.feedback("Enter a server name and a WireGuard configuration.", true)
		return
	}
	if !profile.ValidStructure(p.Config) {
		d.feedback("Enter a valid WireGuard or AmneziaWG configuration.", true)
		return
	}
	for i, existing := range d.cfg.Profiles {
		if i != d.selected && strings.EqualFold(existing.Name, p.Name) {
			d.feedback("A server with this name already exists.", true)
			return
		}
	}
	cfg := d.cfg
	cfg.Profiles = append([]config.Profile(nil), cfg.Profiles...)
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
	cfg.Profiles = append([]config.Profile(nil), cfg.Profiles...)
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
