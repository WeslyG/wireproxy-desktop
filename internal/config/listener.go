package config

import (
	"errors"
	"net"
	"net/netip"
	"strconv"
)

func (s Settings) BindAddress() string {
	host := s.Host
	if host == "" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, strconv.Itoa(s.Port))
}

func (s Settings) ClientAddress() string {
	return ClientAddress(s.BindAddress())
}

func ClientAddress(address string) string {
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

func (s Settings) Validate() error {
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
