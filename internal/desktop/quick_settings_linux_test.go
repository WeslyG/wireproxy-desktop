package desktop

import (
	"github.com/WeslyG/wireproxy-desktop/internal/config"
	"github.com/WeslyG/wireproxy-desktop/internal/tunnel"
)

import "testing"

func TestQuickSettingsReportsOnlyConfirmedPrimaryTunnel(t *testing.T) {
	d := &application{cfg: config.Configuration{DefaultProfile: "primary"}}
	view := tunnel.View{Profile: "primary", Active: true, Ready: true, Status: tunnel.Listening}
	if state := quickSettingsState(d, view); state != "connecting" {
		t.Fatalf("before handshake: got %q", state)
	}
	view.Status = tunnel.Established
	if state := quickSettingsState(d, view); state != "connected" {
		t.Fatalf("after handshake: got %q", state)
	}
	view.Profile = "another"
	if state := quickSettingsState(d, view); state != "other-profile" {
		t.Fatalf("with another profile active: got %q", state)
	}
}
