package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSOCKS5CheckRequiresEstablishedTunnel(t *testing.T) {
	for _, status := range []string{"Disconnected", "Starting", "SOCKS5 listening", "Tunnel unavailable", "Stopping"} {
		ready := status == "SOCKS5 listening" || status == "Tunnel unavailable"
		session := proxySession{status: status, ready: ready}
		if session.view(false).tunnelReady() {
			t.Fatalf("tunnel actions are available in state %q", status)
		}
		if session.check() {
			t.Fatalf("SOCKS5 test started in state %q", status)
		}
	}
	if !canCheckSOCKS(tunnelEstablished, true, false) {
		t.Fatal("SOCKS5 test should be available after a handshake")
	}
}

func TestSOCKS5CheckCancelsStalledHandshake(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := checkHTTPS(ctx, settings{Port: listener.Addr().(*net.TCPAddr).Port})
		result <- err
	}()
	listener.(*net.TCPListener).SetDeadline(time.Now().Add(time.Second))
	conn, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("SOCKS5 check returned %v after cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("SOCKS5 check is still waiting for the proxy")
	}
}

type socksTestTransport func(*http.Request) (*http.Response, error)

func (transport socksTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestSOCKS5CheckUsesFallbackAfterHTTPError(t *testing.T) {
	var hosts []string
	client := &http.Client{Transport: socksTestTransport(func(request *http.Request) (*http.Response, error) {
		hosts = append(hosts, request.URL.Host)
		code := http.StatusServiceUnavailable
		if request.URL.Host == "www.gstatic.com" {
			code = http.StatusNoContent
		}
		return &http.Response{
			StatusCode: code, Status: http.StatusText(code),
			Body: io.NopCloser(strings.NewReader("")),
		}, nil
	})}
	status, err := checkHTTPSEndpoints(context.Background(), client)
	if err != nil || status != http.StatusText(http.StatusNoContent) ||
		strings.Join(hosts, ",") != "example.com,www.gstatic.com" {
		t.Fatalf("status = %q, error = %v, hosts = %v", status, err, hosts)
	}
}
