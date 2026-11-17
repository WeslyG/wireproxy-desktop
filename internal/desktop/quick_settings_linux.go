package desktop

import (
	"context"
	"os"
	"sync/atomic"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/WeslyG/wireproxy-desktop/internal/tunnel"
)

const quickSettingsBus = linuxAppID
const quickSettingsInterface = quickSettingsBus + ".QuickSettings"
const quickSettingsPath = dbus.ObjectPath("/io/github/WeslyG/WireproxyDesktop")

type quickSettingsService struct {
	bus   *dbus.Conn
	tray  *desktopTray
	state atomic.Value
}

type quickSettingsStatus struct{ state, profile string }

func startQuickSettings(tray *desktopTray) quickSettings {
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		return nil
	}
	bus, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil
	}
	service := &quickSettingsService{bus: bus, tray: tray}
	service.state.Store(quickSettingsStatus{state: "loading"})
	if err := bus.Export(service, quickSettingsPath, quickSettingsInterface); err == nil {
		if reply, err := bus.RequestName(quickSettingsBus, dbus.NameFlagDoNotQueue); err == nil && reply == dbus.RequestNameReplyPrimaryOwner {
			tray.background = activationServiceExists()
			return service
		}
	}
	service.close()
	return nil
}

func openQuickSettingsWindow() bool {
	bus, err := dbus.SessionBus()
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return bus.Object(quickSettingsBus, quickSettingsPath).
		CallWithContext(ctx, quickSettingsInterface+".OpenWindow", dbus.FlagNoAutoStart).Err == nil
}

func (s *quickSettingsService) OpenWindow() *dbus.Error {
	s.tray.requestWindow()
	return nil
}

func (s *quickSettingsService) close() { _ = s.bus.Close() }

func (s *quickSettingsService) GetState() (string, *dbus.Error) {
	return s.state.Load().(quickSettingsStatus).state, nil
}

func (s *quickSettingsService) GetStatus() (string, string, *dbus.Error) {
	status := s.state.Load().(quickSettingsStatus)
	return status.state, status.profile, nil
}

func (s *quickSettingsService) SetEnabled(enabled bool) *dbus.Error {
	action := stopPrimaryTunnel
	if enabled {
		action = startTrayTunnel
	}
	if !s.tray.app.requestTunnel(action) {
		return dbus.NewError(quickSettingsInterface+".Busy", []any{"Wireproxy Desktop is busy"})
	}
	return nil
}

func (s *quickSettingsService) publish(d *application, view tunnel.View) {
	next := quickSettingsStatus{state: quickSettingsState(d, view), profile: d.cfg.DefaultProfile}
	if old := s.state.Swap(next).(quickSettingsStatus); old != next {
		_ = s.bus.Emit(quickSettingsPath, quickSettingsInterface+".StatusChanged", next.state, next.profile)
	}
}

func quickSettingsState(d *application, view tunnel.View) string {
	if view.Active {
		switch {
		case view.Profile != d.cfg.DefaultProfile:
			return "other-profile"
		case view.TunnelReady():
			return "connected"
		case view.Status == tunnel.Stopping:
			return "disconnecting"
		case view.Status == tunnel.Unavailable:
			return "unavailable"
		default:
			return "connecting"
		}
	}
	switch {
	case d.storeStatus != "":
		return "loading"
	case d.loadErr != nil:
		return "storage-error"
	case d.cfg.DefaultProfile == "":
		return "no-profile"
	case view.Status == tunnel.Failed && view.Profile == d.cfg.DefaultProfile:
		return "error"
	default:
		return "disconnected"
	}
}
