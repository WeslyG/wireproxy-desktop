package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/amnezia-vpn/amneziawg-go/v3/device"
	wireproxy "github.com/artem-russkikh/wireproxy-awg"
	"github.com/go-ini/ini"
	socks5 "github.com/things-go/go-socks5"
)

const (
	workerFlag      = "--proxy-worker"
	listenerReady   = "[wireproxy-desktop] listening"
	handshakeReady  = "[wireproxy-desktop] handshake"
	maxConfigLength = 6 << 20
)

type workerConfig struct {
	Config  string
	Port    int
	Verbose bool
}

func proxyAddress(port int) string {
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
}

func parseTunnel(config string) (*wireproxy.DeviceConfig, error) {
	config = strings.TrimSpace(strings.TrimPrefix(config, "\uFEFF"))
	if strings.HasPrefix(config, "vpn://") {
		parsed, err := wireproxy.ParseAmneziaConfig(config, "")
		if err != nil {
			return nil, err
		}
		return parsed.Device, nil
	}
	iniConfig, err := ini.LoadSources(ini.LoadOptions{
		Insensitive: true, AllowShadows: true, AllowNonUniqueSections: true,
	}, []byte(config))
	if err != nil {
		return nil, err
	}
	tunnel := &wireproxy.DeviceConfig{MTU: 1420}
	if err := wireproxy.ParseInterface(iniConfig, tunnel); err != nil {
		return nil, err
	}
	if err := wireproxy.ParsePeers(iniConfig, &tunnel.Peers); err != nil {
		return nil, err
	}
	return tunnel, nil
}

func runWorker(input io.Reader) error {
	decoder := json.NewDecoder(input)
	var config workerConfig
	if err := decoder.Decode(&config); err != nil {
		return errors.New("could not read the proxy configuration")
	}
	if len(config.Config) > maxConfigLength || config.Port < 1 || config.Port > 65535 {
		return errors.New("invalid configuration size or SOCKS5 port")
	}
	// The parent holds stdin open for the lifetime of the proxy.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_, _ = io.Copy(io.Discard, input)
		cancel()
	}()

	tunnel, err := parseTunnel(config.Config)
	if err != nil {
		return errors.New("the profile could not be parsed. Check the link, tunnel settings, and endpoint DNS")
	}
	listener, err := net.Listen("tcp", proxyAddress(config.Port))
	if err != nil {
		return fmt.Errorf("could not open the SOCKS5 listener: %w", err)
	}
	defer listener.Close()

	level := device.LogLevelError
	if config.Verbose {
		level = device.LogLevelVerbose
	}
	tun, err := wireproxy.StartWireguard(tunnel, level)
	if err != nil {
		return errors.New("could not start the WireGuard tunnel")
	}
	defer tun.Dev.Close()
	tun.StartPingIPs()
	server := socks5.NewServer(
		socks5.WithDial(tun.Tnet.DialContext),
		socks5.WithResolver(tun),
		socks5.WithLogger(socks5.NewLogger(log.New(os.Stderr, "SOCKS5: ", 0))),
	)
	fmt.Println(listenerReady)
	go reportHandshake(ctx, tun.Dev)
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	if err := server.Serve(listener); err != nil && ctx.Err() == nil {
		return fmt.Errorf("SOCKS5 listener stopped: %w", err)
	}
	return nil
}

func reportHandshake(ctx context.Context, dev *device.Device) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			status, err := dev.IpcGet()
			if err != nil {
				continue
			}
			for line := range strings.SplitSeq(status, "\n") {
				seconds, found := strings.CutPrefix(line, "last_handshake_time_sec=")
				if found && seconds != "" && seconds != "0" {
					fmt.Println(handshakeReady)
					return
				}
			}
		}
	}
}
