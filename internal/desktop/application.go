package desktop

import (
	"github.com/WeslyG/wireproxy-desktop/internal/config"
	"github.com/WeslyG/wireproxy-desktop/internal/tunnel"
)

type tunnelAction uint8

const (
	startTrayTunnel tunnelAction = iota
	stopTrayTunnel
	stopPrimaryTunnel
)

// application owns the saved configuration and the running tunnel. Its owner
// processes commands and storage results on the window loop or the background loop.
type application struct {
	store                        config.Store
	storeResults                 chan configStoreResult
	storeStatus, migrationNotice string
	cfg                          config.Configuration
	session                      tunnel.Session
	loadErr                      error
	loadGeneration               uint64
	tunnel                       chan tunnelAction
	changed                      chan struct{}
}

type configStoreResult struct {
	cfg    config.Configuration
	notice string
	err    error
}

func newApplication(store config.Store, err error, invalidate func()) *application {
	d := &application{store: store, loadErr: err, cfg: config.DefaultConfig(),
		storeResults: make(chan configStoreResult, 1), tunnel: make(chan tunnelAction, 1), changed: make(chan struct{}, 1)}
	d.session.Notify = func() {
		select {
		case d.changed <- struct{}{}:
		default:
		}
		invalidate()
	}
	return d
}

func (d *application) loadProtectedConfig() {
	d.storeStatus = "Opening protected storage…"
	go func() {
		cfg, notice, verified, err := d.store.Load()
		if verified {
			prefs := d.store.StartupPreferences()
			prefs.RecordVerifiedKeychainRead()
			prefs.RememberLanguage(cfg.Settings.Language)
		}
		d.storeResults <- configStoreResult{cfg: cfg, notice: notice, err: err}
		d.session.Notify()
	}()
}

func (d *application) saveProtectedConfig(cfg config.Configuration) {
	d.storeStatus = "Saving to protected storage…"
	go func() {
		err := d.store.Save(cfg)
		d.storeResults <- configStoreResult{cfg: cfg, err: err}
		d.session.Notify()
	}()
}

func (d *application) applyLoadedConfig(result configStoreResult) {
	d.loadGeneration++
	d.storeStatus = ""
	d.cfg, d.loadErr, d.migrationNotice = result.cfg, result.err, result.notice
	if d.cfg.Settings.Port == 0 {
		d.cfg.Settings = config.DefaultConfig().Settings
	}
}

func (d *application) defaultProfile() (config.Profile, bool) {
	for _, server := range d.cfg.Profiles {
		if server.Name == d.cfg.DefaultProfile {
			return server, true
		}
	}
	return config.Profile{}, false
}

func (d *application) requestTunnel(action tunnelAction) bool {
	select {
	case d.tunnel <- action:
		d.session.Notify()
		return true
	default:
		return false
	}
}

func (d *application) applyTunnelAction(action tunnelAction) error {
	view := d.session.View(false)
	if action == stopPrimaryTunnel && view.Profile != d.cfg.DefaultProfile {
		return nil
	}
	if action == stopTrayTunnel || action == stopPrimaryTunnel {
		d.session.Stop()
	} else if server, ok := d.startablePrimary(view); ok {
		return d.session.Start(server, d.cfg.Settings)
	}
	return nil
}

func (d *application) startablePrimary(view tunnel.View) (config.Profile, bool) {
	if view.Active || d.storeStatus != "" || d.loadErr != nil {
		return config.Profile{}, false
	}
	return d.defaultProfile()
}
