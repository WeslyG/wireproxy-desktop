package tunnel

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/amnezia-vpn/amneziawg-go/v3/device"

	"github.com/WeslyG/wireproxy-desktop/internal/profile"
)

func EngineVersion() string {
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

func probeProfile(config string) error {
	if len(config) == 0 || len(config) > profile.MaxConfigLength {
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
