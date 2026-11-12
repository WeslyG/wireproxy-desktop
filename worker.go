package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/amnezia-vpn/amneziawg-go/v3/conn"
	"github.com/amnezia-vpn/amneziawg-go/v3/device"
	"github.com/amnezia-vpn/amneziawg-go/v3/tun/netstack"
	wireproxy "github.com/artem-russkikh/wireproxy-awg"
	"github.com/go-ini/ini"
	socks5 "github.com/things-go/go-socks5"
)

const (
	workerFlag      = "--proxy-worker"
	listenerReady   = "[wireproxy-desktop] listening"
	handshakeReady  = "[wireproxy-desktop] handshake"
	handshakeLost   = "[wireproxy-desktop] handshake expired"
	protocolStatus  = "[wireproxy-desktop] protocol "
	maxConfigLength = 6 << 20
)

type workerConfig struct {
	Config string
	settings
}

func proxyAddress(port int) string {
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
}

func (s settings) bindAddress() string {
	host := s.Host
	if host == "" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, strconv.Itoa(s.Port))
}

func (s settings) clientAddress() string {
	return proxyClientAddress(s.bindAddress())
}

func proxyClientAddress(address string) string {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return address
	}
	ip, err := netip.ParseAddr(host)
	if err == nil && ip.Unmap().IsUnspecified() {
		if ip.Unmap().Is4() {
			return net.JoinHostPort("127.0.0.1", port)
		}
		return net.JoinHostPort("::1", port)
	}
	return address
}

func (s settings) validate() error {
	if s.Port < 1 || s.Port > 65535 {
		return errors.New("Enter a port between 1 and 65535.")
	}
	if s.Host != "" {
		if _, err := netip.ParseAddr(s.Host); err != nil {
			return errors.New("Enter an IPv4 or IPv6 listen address.")
		}
	}
	if (s.Username == "") != (s.Password == "") || len(s.Username) > 255 || len(s.Password) > 255 {
		return errors.New("Enter both SOCKS5 credentials, up to 255 bytes each.")
	}
	return nil
}

func loadTunnelINI(config string) (*ini.File, error) {
	parsed, _, err := loadProfileINI(config)
	return parsed, err
}

func loadProfileINI(config string) (*ini.File, string, error) {
	config = strings.TrimSpace(strings.TrimPrefix(config, "\uFEFF"))
	options := ini.LoadOptions{
		Insensitive: true, AllowShadows: true, AllowNonUniqueSections: true,
	}
	if strings.HasPrefix(config, amneziaScheme) {
		return loadAmneziaConfig(options, config)
	}
	parsed, err := ini.LoadSources(options, []byte(config))
	return parsed, "", err
}

func parseTunnel(config string) (*wireproxy.DeviceConfig, error) {
	iniConfig, err := loadTunnelINI(config)
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
	if len(config.Config) > maxConfigLength {
		return errors.New("invalid configuration size")
	}
	if err := config.validate(); err != nil {
		return err
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
	listener, err := net.Listen("tcp", config.bindAddress())
	if err != nil {
		return fmt.Errorf("could not open the SOCKS5 listener: %w", err)
	}
	defer listener.Close()

	level := device.LogLevelError
	if config.Verbose {
		level = device.LogLevelVerbose
	}
	tun, err := startTunnel(tunnel, level)
	if err != nil {
		return fmt.Errorf("WireGuard: %w", err)
	}
	defer tun.Dev.Close()
	tun.StartPingIPs()
	options := []socks5.Option{
		socks5.WithDial(tun.Tnet.DialContext),
		socks5.WithResolver(tun),
		socks5.WithLogger(socks5.NewLogger(log.New(os.Stderr, "SOCKS5: ", 0))),
	}
	if config.Username != "" {
		options = append(options, socks5.WithAuthMethods([]socks5.Authenticator{
			socks5.UserPassAuthenticator{Credentials: socks5.StaticCredentials{config.Username: config.Password}},
		}))
	}
	server := socks5.NewServer(options...)
	fmt.Println(protocolStatus + profileProtocol(config.Config))
	fmt.Println(listenerReady)
	go monitorHandshake(ctx, tun.Dev, tunnel.Peers)
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	if err := server.Serve(listener); err != nil && ctx.Err() == nil {
		return fmt.Errorf("SOCKS5 listener stopped: %w", err)
	}
	return nil
}

func initiateHandshakes(ctx context.Context, dev *device.Device, peers []wireproxy.PeerConfig) {
	for _, config := range peers {
		if ctx.Err() != nil {
			return
		}
		if config.Endpoint == nil {
			continue
		}
		var key device.NoisePublicKey
		if err := key.FromHex(config.PublicKey); err != nil {
			continue
		}
		if peer := dev.LookupPeer(key); peer != nil {
			_ = peer.SendHandshakeInitiation(false)
		}
	}
}

func startTunnel(conf *wireproxy.DeviceConfig, level int) (*wireproxy.VirtualTun, error) {
	setting, err := wireproxy.CreateIPCRequest(conf)
	if err != nil {
		return nil, errors.New("invalid tunnel parameters")
	}
	tun, network, err := netstack.CreateNetTUN(setting.DeviceAddr, setting.DNS, setting.MTU)
	if err != nil {
		return nil, fmt.Errorf("create network stack: %w", err)
	}
	dev := device.NewDevice(tun, conn.NewDefaultBind(), device.NewLogger(level, "WireGuard: "))
	if err := dev.IpcSet(setting.IpcRequest); err != nil {
		dev.Close()
		return nil, errors.New("the WireGuard engine rejected the tunnel settings")
	}
	if err := dev.Up(); err != nil {
		dev.Close()
		return nil, fmt.Errorf("start WireGuard device: %w", err)
	}
	return &wireproxy.VirtualTun{
		Dev: dev, Tnet: network, Conf: conf, SystemDNS: len(setting.DNS) == 0,
		PingRecord: make(map[string]uint64), PingRecordLock: new(sync.Mutex),
	}, nil
}

func monitorHandshake(ctx context.Context, dev *device.Device, peers []wireproxy.PeerConfig) {
	const refreshInterval = 20 * time.Second
	var lastAttempt time.Time
	established := false
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		now := time.Now()
		status, err := dev.IpcGet()
		last := latestHandshake(status)
		available := err == nil && handshakeFresh(last, now)
		if established != available {
			established = available
			if established {
				fmt.Println(handshakeReady)
			} else {
				fmt.Println(handshakeLost)
			}
		}
		if now.Sub(last) >= refreshInterval && now.Sub(lastAttempt) >= refreshInterval {
			lastAttempt = now
			initiateHandshakes(ctx, dev, peers)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func handshakeFresh(last, now time.Time) bool {
	age := now.Sub(last)
	return !last.IsZero() && age >= 0 && age < 30*time.Second
}

func latestHandshake(status string) time.Time {
	var latest int64
	for line := range strings.SplitSeq(status, "\n") {
		seconds, found := strings.CutPrefix(line, "last_handshake_time_sec=")
		if found {
			if value, err := strconv.ParseInt(seconds, 10, 64); err == nil && value > latest {
				latest = value
			}
		}
	}
	if latest == 0 {
		return time.Time{}
	}
	return time.Unix(latest, 0)
}
