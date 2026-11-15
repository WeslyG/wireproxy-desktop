package profile

import (
	"errors"
	"strconv"
	"strings"

	"github.com/go-ini/ini"
)

const MaxConfigLength = 6 << 20

func ParseINI(config string) (*ini.File, string, error) {
	config = strings.TrimSpace(strings.TrimPrefix(config, "\uFEFF"))
	options := ini.LoadOptions{
		Insensitive: true, AllowShadows: true, AllowNonUniqueSections: true,
	}
	if strings.HasPrefix(config, AmneziaScheme) {
		return loadAmneziaConfig(options, config)
	}
	parsed, err := ini.LoadSources(options, []byte(config))
	return parsed, "", err
}

func Protocol(config string) string {
	parsed, declared, err := ParseINI(config)
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

func Convert(link string) (string, error) {
	link = strings.TrimSpace(link)
	if !strings.HasPrefix(link, AmneziaScheme) || len(link) > MaxConfigLength {
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

func ValidStructure(config string) bool {
	parsed, _, err := ParseINI(config)
	if err != nil {
		return false
	}
	if _, err = parsed.GetSection("Interface"); err != nil {
		return false
	}
	peers, err := parsed.SectionsByName("Peer")
	return err == nil && len(peers) != 0
}
