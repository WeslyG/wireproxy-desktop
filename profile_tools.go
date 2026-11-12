package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/amnezia-vpn/amneziawg-go/v3/device"
)

func profileProtocol(config string) string {
	parsed, err := loadTunnelINI(config)
	if err != nil {
		return ""
	}
	iface, err := parsed.GetSection("Interface")
	if err != nil {
		return ""
	}
	for _, version := range []struct {
		name string
		keys []string
	}{
		{"AmneziaWG 3.1", []string{"RandomTrailers", "DisableCookies"}},
		{"AmneziaWG 3.0", []string{"HeaderProtectionKey", "ContentPaddingAddition", "RekeyAfterTime", "RekeyTimeout", "RejectAfterTime", "KeepaliveTimeout", "MaxHandshakeAttempts"}},
		{"AmneziaWG 2.0", []string{"S3", "S4", "I1", "I2", "I3", "I4", "I5"}},
		{"AmneziaWG 1.0", []string{"Jc", "Jmin", "Jmax", "S1", "S2", "H1", "H2", "H3", "H4"}},
	} {
		for _, name := range version.keys {
			key, err := iface.GetKey(name)
			if err != nil || strings.TrimSpace(key.Value()) == "" {
				continue
			}
			value := strings.ToLower(strings.TrimSpace(key.Value()))
			if version.name == "AmneziaWG 3.1" && (value == "off" || value == "false" || value == "0") {
				continue
			}
			return version.name
		}
	}
	return "WireGuard v1"
}

func convertProfile(link string) (string, error) {
	link = strings.TrimSpace(link)
	if !strings.HasPrefix(link, amneziaScheme) || len(link) > maxConfigLength {
		return "", errors.New("Paste a complete vpn:// link.")
	}
	parsed, err := loadTunnelINI(link)
	if err != nil {
		return "", err
	}
	var output strings.Builder
	if _, err := parsed.WriteTo(&output); err != nil {
		return "", err
	}
	return output.String(), nil
}

func probeProfile(config string) (string, error) {
	if len(config) == 0 || len(config) > maxConfigLength {
		return "", errors.New("Enter a configuration to check.")
	}
	conf, err := parseTunnel(config)
	if err != nil {
		return "", errors.New("Invalid profile or unresolved endpoint. Check the configuration.")
	}
	protocol := profileProtocol(config)
	tunnel, err := startTunnel(conf, device.LogLevelSilent)
	if err != nil {
		return protocol, err
	}
	defer tunnel.Dev.Close()
	transport := &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		addresses, err := tunnel.LookupAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		var dialErr error
		for _, ip := range addresses {
			connection, err := tunnel.Tnet.DialContext(ctx, network, net.JoinHostPort(ip, port))
			if err == nil {
				return connection, nil
			}
			dialErr = err
		}
		return nil, errors.Join(errors.New("could not reach the test site through the tunnel"), dialErr)
	}}
	defer transport.CloseIdleConnections()
	client := http.Client{Transport: transport, Timeout: 12 * time.Second}
	response, err := client.Get("https://example.com/")
	if err != nil {
		return protocol, err
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return protocol, fmt.Errorf("HTTP %d", response.StatusCode)
	}
	return protocol, nil
}
