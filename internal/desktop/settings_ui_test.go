package desktop

import (
	"net"
	"strconv"
	"testing"

	"github.com/WeslyG/wireproxy-desktop/internal/tunnel"
)

func TestPortCheckSuggestsAnAvailablePort(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	d := &desktop{application: &application{}, host: "127.0.0.1"}
	d.port.SetText(strconv.Itoa(occupied.Addr().(*net.TCPAddr).Port))
	d.verifyPort(tunnel.View{})
	if !d.messageError || d.suggestedPort == 0 {
		t.Fatalf("expected a bind error and alternative port, got %q and %d", d.message, d.suggestedPort)
	}
	d.port.SetText(strconv.Itoa(d.suggestedPort))
	d.verifyPort(tunnel.View{})
	if d.messageError || d.suggestedPort != 0 {
		t.Fatalf("expected the suggested port to be available, got %q", d.message)
	}
}

func TestPortCheckRecognizesOwnWildcardListener(t *testing.T) {
	d := &desktop{application: &application{}, host: "127.0.0.1"}
	d.port.SetText("1080")
	d.verifyPort(tunnel.View{Address: "0.0.0.0:1080", Active: true, Ready: true})
	if d.messageError || d.suggestedPort != 0 || d.message != d.tr("Wireproxy is using this port")+" · 0.0.0.0:1080" {
		t.Fatalf("expected running listener status, got %q and %d", d.message, d.suggestedPort)
	}
}
