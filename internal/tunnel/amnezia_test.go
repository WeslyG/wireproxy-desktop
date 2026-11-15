package tunnel

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"

	"github.com/WeslyG/wireproxy-desktop/internal/profile"
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
	fixture := map[string]any{
		"defaultContainer": "amnezia-awg2",
		"dns1":             "192.0.2.53",
		"containers": []any{map[string]any{
			"container": "amnezia-awg2",
			"awg":       map[string]string{"protocol_version": "3.1", "last_config": string(client)},
		}},
	}
	payload, err := json.Marshal(fixture)
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
	link := profile.AmneziaScheme + base64.RawURLEncoding.EncodeToString(packed.Bytes())
	if got := profile.Protocol(link); got != "AmneziaWG 3.1" {
		t.Fatalf("declared protocol = %q, want AmneziaWG 3.1", got)
	}
	converted, err := profile.Convert(link)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(converted, "$PRIMARY_DNS") || profile.Protocol(converted) != "AmneziaWG 2.0+" {
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
