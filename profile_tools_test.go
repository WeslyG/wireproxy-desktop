package main

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
)

func TestProfileProtocolUsesActiveFeatures(t *testing.T) {
	for _, tc := range []struct{ name, config, want string }{
		{"inactive defaults", "H1=1\nH2=2-2\nS3=0\nS4=0\nJmin=10\nJc=0\nContentPaddingAddition=0 - 0", "WireGuard v1"},
		{"legacy padding", "S1=12\nS3=0\nS4=0", "AmneziaWG 1.0"},
		{"legacy signatures", "H1=90\nI1=<r 32>\nS3=0\nS4=0", "AmneziaWG 1.5"},
		{"ranged headers", "H1=90-100\nI1=<r 32>", "AmneziaWG 2.0+"},
		{"active extra padding", "S4=16", "AmneziaWG 2.0+"},
		{"ranged keepalive", "S3=12\n[Peer]\nPersistentKeepalive=20-25", "AmneziaWG 3.0"},
		{"trailer extension", "RandomTrailers=on", "AmneziaWG 3.1"},
		{"disabled cookie extension", "H1=90-100\nDisableCookies=off", "AmneziaWG 3.1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := profileProtocol("[Interface]\n" + tc.config); got != tc.want {
				t.Fatalf("profileProtocol = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestConvertAmneziaLinkPreservesTunnelParameters(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	client, err := json.Marshal(map[string]string{
		"mtu": "1376",
		"config": "[Interface]\nAddress = 10.0.0.2/32\nPrivateKey = " + key +
			"\nDNS = $PRIMARY_DNS\nS3 = 10\n[Peer]\nPublicKey = " + key +
			"\nEndpoint = 192.0.2.1:51820\nAllowedIPs = 0.0.0.0/0\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	var profile amneziaProfile
	if err := json.Unmarshal([]byte(`{"defaultContainer":"amnezia-awg2","dns1":"192.0.2.53","containers":[{"container":"amnezia-awg2","awg":{"protocol_version":"3.1"}}]}`), &profile); err != nil {
		t.Fatal(err)
	}
	profile.Containers[0].AWG.LastConfig = string(client)
	payload, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	var packed bytes.Buffer
	packed.Write(binary.BigEndian.AppendUint32(nil, uint32(len(payload))))
	compressor := zlib.NewWriter(&packed)
	if _, err := compressor.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := compressor.Close(); err != nil {
		t.Fatal(err)
	}
	link := amneziaScheme + base64.RawURLEncoding.EncodeToString(packed.Bytes())
	if got := profileProtocol(link); got != "AmneziaWG 3.1" {
		t.Fatalf("declared protocol = %q, want AmneziaWG 3.1", got)
	}
	converted, err := convertProfile(link)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(converted, "$PRIMARY_DNS") || profileProtocol(converted) != "AmneziaWG 2.0+" {
		t.Fatal("converted INI lost DNS or protocol parameters")
	}
	if !strings.Contains(converted, "[Interface]") || !strings.Contains(converted, "PrivateKey") {
		t.Fatal("conversion changed WireGuard section or key spelling")
	}
	for _, config := range []string{link, converted} {
		tunnel, err := parseTunnel(config)
		if err != nil {
			t.Fatal(err)
		}
		if tunnel.MTU != 1376 || len(tunnel.Peers) != 1 ||
			len(tunnel.DNS) != 1 || tunnel.DNS[0].String() != "192.0.2.53" {
			t.Fatal("imported tunnel parameters differ")
		}
	}
}
