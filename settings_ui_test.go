package main

import (
	"net"
	"strconv"
	"testing"
)

func TestPortCheckSuggestsAnAvailablePort(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	d := &desktop{host: "127.0.0.1"}
	d.port.SetText(strconv.Itoa(occupied.Addr().(*net.TCPAddr).Port))
	d.verifyPort(sessionView{})
	if !d.messageError || d.suggestedPort == 0 {
		t.Fatalf("expected a bind error and alternative port, got %q and %d", d.message, d.suggestedPort)
	}
	d.port.SetText(strconv.Itoa(d.suggestedPort))
	d.verifyPort(sessionView{})
	if d.messageError || d.suggestedPort != 0 {
		t.Fatalf("expected the suggested port to be available, got %q", d.message)
	}
}

func TestPortCheckRecognizesOwnWildcardListener(t *testing.T) {
	d := &desktop{host: "127.0.0.1"}
	d.port.SetText("1080")
	d.verifyPort(sessionView{address: "0.0.0.0:1080", active: true, ready: true})
	if d.messageError || d.suggestedPort != 0 || d.message != d.tr("Wireproxy is using this port")+" · 0.0.0.0:1080" {
		t.Fatalf("expected running listener status, got %q and %d", d.message, d.suggestedPort)
	}
}
