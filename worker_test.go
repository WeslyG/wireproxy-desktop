package main

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/amnezia-vpn/amneziawg-go/v3/device"
	wireproxy "github.com/artem-russkikh/wireproxy-awg"
	"golang.org/x/net/proxy"
)

func TestHandshakeFreshness(t *testing.T) {
	now := time.Unix(12345, 0)
	if !handshakeFresh(latestHandshake("last_handshake_time_sec=12344\n"), now) {
		t.Fatal("fresh handshake should enable tunnel actions")
	}
	for _, status := range []string{"", "last_handshake_time_sec=0\n", "last_handshake_time_sec=12315\n"} {
		if handshakeFresh(latestHandshake(status), now) {
			t.Fatalf("missing or expired handshake accepted: %q", status)
		}
	}
}

func TestWorkerRoutesSOCKS5ThroughWireGuard(t *testing.T) {
	serverKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	clientKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	server, err := wireproxy.StartWireguard(&wireproxy.DeviceConfig{
		SecretKey: hex.EncodeToString(serverKey.Bytes()),
		Endpoint:  []netip.Addr{netip.MustParseAddr("10.90.0.1")},
		MTU:       1420,
		Peers: []wireproxy.PeerConfig{{
			PublicKey:    hex.EncodeToString(clientKey.PublicKey().Bytes()),
			PreSharedKey: strings.Repeat("0", 64),
			AllowedIPs:   []netip.Prefix{netip.MustParsePrefix("10.90.0.2/32")},
		}},
	}, device.LogLevelSilent)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Dev.Close)
	status, err := server.Dev.IpcGet()
	if err != nil {
		t.Fatal(err)
	}
	var endpoint string
	for line := range strings.SplitSeq(status, "\n") {
		if port, ok := strings.CutPrefix(line, "listen_port="); ok && port != "0" {
			endpoint = net.JoinHostPort("127.0.0.1", port)
			break
		}
	}
	if endpoint == "" {
		t.Fatal("WireGuard peer did not bind a UDP port")
	}

	listener, err := server.Tnet.ListenTCPAddrPort(netip.MustParseAddrPort("10.90.0.1:8080"))
	if err != nil {
		t.Fatal(err)
	}
	httpServer := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "through WireGuard")
	})}
	t.Cleanup(func() { _ = httpServer.Close() })
	go func() { _ = httpServer.Serve(listener) }()

	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	_ = probe.Close()
	input, parent := io.Pipe()
	t.Cleanup(func() { _ = parent.Close() })
	exited := make(chan error, 1)
	go func() { exited <- runWorker(input) }()
	config := fmt.Sprintf("[Interface]\nAddress = 10.90.0.2/32\nPrivateKey = %s\n[Peer]\nPublicKey = %s\nEndpoint = %s\nAllowedIPs = 10.90.0.1/32\n",
		base64.StdEncoding.EncodeToString(clientKey.Bytes()), base64.StdEncoding.EncodeToString(serverKey.PublicKey().Bytes()), endpoint)
	options := settings{Host: "0.0.0.0", Port: port, Username: "test-user", Password: "test-password"}
	if err := json.NewEncoder(parent).Encode(workerConfig{Config: config, settings: options}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", proxyAddress(port), 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("SOCKS5 listener did not start")
		}
		time.Sleep(25 * time.Millisecond)
	}

	deadline = time.Now().Add(5 * time.Second)
	for {
		status, err := server.Dev.IpcGet()
		if err != nil {
			t.Fatal(err)
		}
		if !latestHandshake(status).IsZero() {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("tunnel did not handshake before the first SOCKS5 request")
		}
		time.Sleep(25 * time.Millisecond)
	}

	unauthenticated, err := proxy.SOCKS5("tcp", options.clientAddress(), nil, proxy.Direct)
	if err != nil {
		t.Fatal(err)
	}
	if conn, err := unauthenticated.Dial("tcp", "10.90.0.1:8080"); err == nil {
		conn.Close()
		t.Fatal("SOCKS5 accepted a client without credentials")
	}
	dialer, err := proxy.SOCKS5("tcp", options.clientAddress(), &proxy.Auth{User: options.Username, Password: options.Password}, proxy.Direct)
	if err != nil {
		t.Fatal(err)
	}
	transport := &http.Transport{DialContext: dialer.(proxy.ContextDialer).DialContext}
	defer transport.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://10.90.0.1:8080/", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := (&http.Client{Transport: transport}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil || response.StatusCode != http.StatusOK || string(data) != "through WireGuard" {
		t.Fatalf("unexpected response: %d %q (%v)", response.StatusCode, data, err)
	}
	_ = parent.Close()
	select {
	case err := <-exited:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("proxy did not stop when the parent pipe closed")
	}
}
