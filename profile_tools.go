package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/amnezia-vpn/amneziawg-go/v3/device"
	"github.com/go-ini/ini"
)

func tunnelEngineVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range info.Deps {
			if dep.Path == "github.com/amnezia-vpn/amneziawg-go/v3" {
				if dep.Replace != nil {
					dep = dep.Replace
				}
				return dep.Version
			}
		}
	}
	return ""
}

func profileProtocol(config string) string {
	parsed, declared, err := loadProfileINI(config)
	if err != nil {
		return ""
	}
	iface, err := parsed.GetSection("Interface")
	if err != nil {
		return ""
	}
	value := func(name string) string {
		if key, err := iface.GetKey(name); err == nil {
			return strings.TrimSpace(key.Value())
		}
		return ""
	}
	enabled := func(names ...string) bool {
		for _, name := range names {
			raw := strings.ToLower(value(name))
			switch raw {
			case "", "0", "0-0", "off", "false":
				continue
			}
			if low, high, ok := strings.Cut(raw, "-"); ok && strings.TrimSpace(low) == "0" && strings.TrimSpace(high) == "0" {
				continue
			}
			return true
		}
		return false
	}
	if declared == "AmneziaWG 3.1" || value("RandomTrailers") != "" || value("DisableCookies") != "" {
		return "AmneziaWG 3.1"
	}
	if declared == "AmneziaWG 3.0" || enabled("HeaderProtectionKey", "ContentPaddingAddition", "RekeyAfterTime", "RekeyTimeout", "RejectAfterTime", "KeepaliveTimeout", "MaxHandshakeAttempts") {
		return "AmneziaWG 3.0"
	}
	peers, _ := parsed.SectionsByName("Peer")
	for _, peer := range peers {
		if key, err := peer.GetKey("PersistentKeepalive"); err == nil && numericRange(key.Value()) {
			return "AmneziaWG 3.0"
		}
	}
	if declared != "" {
		return declared
	}
	headers := [...]string{"H1", "H2", "H3", "H4"}
	for _, name := range headers {
		if numericRange(value(name)) {
			return "AmneziaWG 2.0+"
		}
	}
	if enabled("S3", "S4") {
		return "AmneziaWG 2.0+"
	}
	if enabled("I1", "I2", "I3", "I4", "I5") {
		return "AmneziaWG 1.5"
	}
	for index, name := range headers {
		raw := value(name)
		low, high, ranged := strings.Cut(raw, "-")
		expected := strconv.Itoa(index + 1)
		if raw != "" && raw != expected && !(ranged && strings.TrimSpace(low) == expected && strings.TrimSpace(high) == expected) {
			return "AmneziaWG 1.0"
		}
	}
	if enabled("Jc", "S1", "S2") {
		return "AmneziaWG 1.0"
	}
	return "WireGuard v1"
}

func numericRange(value string) bool {
	low, high, ok := strings.Cut(value, "-")
	if !ok {
		return false
	}
	minimum, first := strconv.ParseUint(strings.TrimSpace(low), 10, 32)
	maximum, second := strconv.ParseUint(strings.TrimSpace(high), 10, 32)
	return first == nil && second == nil && minimum < maximum
}

func convertProfile(link string) (string, error) {
	link = strings.TrimSpace(link)
	if !strings.HasPrefix(link, amneziaScheme) || len(link) > maxConfigLength {
		return "", errors.New("Paste a complete vpn:// link.")
	}
	parsed, _, err := loadAmneziaConfig(ini.LoadOptions{AllowShadows: true, AllowNonUniqueSections: true}, link)
	if err != nil {
		return "", err
	}
	var output strings.Builder
	if _, err := parsed.WriteTo(&output); err != nil {
		return "", err
	}
	return output.String(), nil
}

func probeProfile(config string) error {
	if len(config) == 0 || len(config) > maxConfigLength {
		return errors.New("Enter a configuration to check.")
	}
	conf, err := parseTunnel(config)
	if err != nil {
		return errors.New("Invalid profile or unresolved endpoint. Check the configuration.")
	}
	tunnel, err := startTunnel(conf, device.LogLevelSilent)
	if err != nil {
		return err
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
		return err
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", response.StatusCode)
	}
	return nil
}
