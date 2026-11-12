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
	if err := json.Unmarshal([]byte(`{"defaultContainer":"amnezia-awg2","dns1":"192.0.2.53","containers":[{"container":"amnezia-awg2","awg":{}}]}`), &profile); err != nil {
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
	converted, err := convertProfile(link)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(converted, "$PRIMARY_DNS") || profileProtocol(converted) != "AmneziaWG 2.0" {
		t.Fatal("converted INI lost DNS or protocol parameters")
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
