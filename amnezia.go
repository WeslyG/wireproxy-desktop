// Amnezia profile decoding adapted from the pinned wireproxy-awg engine (ISC).

package main

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/go-ini/ini"
)

const (
	amneziaScheme         = "vpn://"
	maxAmneziaProfileSize = 4 << 20
)

type amneziaProfile struct {
	DefaultContainer string `json:"defaultContainer"`
	DNS1             string `json:"dns1"`
	DNS2             string `json:"dns2"`
	Containers       []struct {
		Name      string          `json:"container"`
		AWG       amneziaProtocol `json:"awg"`
		WireGuard amneziaProtocol `json:"wireguard"`
	} `json:"containers"`
}

type amneziaProtocol struct {
	LastConfig      string          `json:"last_config"`
	ProtocolVersion json.RawMessage `json:"protocol_version,omitempty"`
}

func loadAmneziaConfig(options ini.LoadOptions, link string) (*ini.File, string, error) {
	profile, err := decodeAmneziaProfile(link)
	if err != nil {
		return nil, "", err
	}

	var selected amneziaProtocol
	for _, container := range profile.Containers {
		if profile.DefaultContainer != "" && container.Name != profile.DefaultContainer {
			continue
		}
		candidate := container.AWG
		if candidate.LastConfig == "" {
			candidate = amneziaProtocol{LastConfig: container.WireGuard.LastConfig}
		}
		if candidate.LastConfig == "" {
			continue
		}
		if selected.LastConfig != "" {
			return nil, "", errors.New("Amnezia profile has multiple matching VPN configurations")
		}
		selected = candidate
	}
	if selected.LastConfig == "" {
		return nil, "", errors.New("Amnezia profile has no selected AmneziaWG or WireGuard client configuration")
	}

	var client map[string]json.RawMessage
	if err := json.Unmarshal([]byte(selected.LastConfig), &client); err != nil {
		return nil, "", errors.New("invalid Amnezia client configuration JSON")
	}
	var nativeConfig string
	if err := json.Unmarshal(client["config"], &nativeConfig); err != nil || strings.TrimSpace(nativeConfig) == "" {
		return nil, "", errors.New("Amnezia client configuration has no embedded WireGuard INI")
	}
	cfg, err := ini.LoadSources(options, []byte(nativeConfig))
	if err != nil {
		return nil, "", errors.New("invalid embedded WireGuard INI in Amnezia profile")
	}
	for _, iface := range cfg.Sections() {
		if !strings.EqualFold(iface.Name(), "Interface") {
			continue
		}
		if dns := amneziaKey(iface, "DNS"); dns != nil {
			dns.SetValue(strings.NewReplacer("$PRIMARY_DNS", profile.DNS1, "$SECONDARY_DNS", profile.DNS2).Replace(dns.Value()))
		}
		if err := applyAmneziaParameters(iface, client); err != nil {
			return nil, "", err
		}
	}
	version := amneziaVersion(selected.ProtocolVersion)
	if version == "" {
		version = amneziaVersion(client["protocol_version"])
	}
	return cfg, version, nil
}

func amneziaVersion(raw json.RawMessage) string {
	value := string(raw)
	if strings.HasPrefix(value, "\"") {
		if err := json.Unmarshal(raw, &value); err != nil {
			return ""
		}
	}
	switch strings.TrimSpace(value) {
	case "3.1":
		return "AmneziaWG 3.1"
	case "3.0":
		return "AmneziaWG 3.0"
	case "2", "2.0":
		return "AmneziaWG 2.0"
	case "1.5":
		return "AmneziaWG 1.5"
	case "1", "1.0":
		return "AmneziaWG 1.0"
	default:
		return ""
	}
}

func amneziaKey(section *ini.Section, name string) *ini.Key {
	for _, key := range section.Keys() {
		if strings.EqualFold(key.Name(), name) {
			return key
		}
	}
	return nil
}

func decodeAmneziaProfile(link string) (*amneziaProfile, error) {
	payload := strings.TrimPrefix(link, amneziaScheme)
	if len(payload) > base64.URLEncoding.EncodedLen(maxAmneziaProfileSize+4) {
		return nil, errors.New("Amnezia VPN link is too large")
	}
	encoding := base64.RawURLEncoding
	if strings.HasSuffix(payload, "=") {
		encoding = base64.URLEncoding
	}
	packed, err := encoding.DecodeString(payload)
	if err != nil || len(packed) < 4 {
		return nil, errors.New("invalid Amnezia VPN link encoding")
	}
	// Qt qCompress prefixes the zlib stream with its uncompressed size.
	size := binary.BigEndian.Uint32(packed[:4])
	if size == 0 || size > maxAmneziaProfileSize {
		return nil, errors.New("invalid Amnezia profile size")
	}
	reader, err := zlib.NewReader(bytes.NewReader(packed[4:]))
	if err != nil {
		return nil, errors.New("invalid compressed Amnezia profile")
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, int64(size)+1))
	if err != nil || len(data) != int(size) {
		return nil, errors.New("invalid compressed Amnezia profile")
	}

	var profile amneziaProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, errors.New("invalid Amnezia profile JSON")
	}
	return &profile, nil
}

func applyAmneziaParameters(iface *ini.Section, client map[string]json.RawMessage) error {
	// Amnezia also exports protocol parameters as fields alongside the INI.
	for _, name := range []string{
		"MTU", "Jc", "Jmin", "Jmax", "S1", "S2", "S3", "S4",
		"H1", "H2", "H3", "H4", "I1", "I2", "I3", "I4", "I5",
		"HeaderProtectionKey", "ContentPaddingAddition", "RekeyAfterTime",
		"RekeyTimeout", "RejectAfterTime", "KeepaliveTimeout",
		"MaxHandshakeAttempts", "RandomTrailers", "DisableCookies",
	} {
		raw, ok := client[name]
		if !ok && name == "MTU" {
			raw, ok = client["mtu"]
		}
		if !ok || string(raw) == "null" {
			continue
		}
		value := string(raw)
		switch value[0] {
		case '"':
			if err := json.Unmarshal(raw, &value); err != nil {
				return errors.New("invalid Amnezia parameter " + name)
			}
		case '{', '[':
			return errors.New("invalid Amnezia parameter " + name)
		}
		if value != "" {
			if key := amneziaKey(iface, name); key != nil {
				key.SetValue(value)
			} else if _, err := iface.NewKey(name, value); err != nil {
				return err
			}
		}
	}
	return nil
}
