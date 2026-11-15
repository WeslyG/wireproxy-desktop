// Command peer provides a disposable WireGuard peer and HTTP fixture for Fedora QA.
package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/netip"
	"os"
	"strings"

	"github.com/amnezia-vpn/amneziawg-go/v3/conn"
	"github.com/amnezia-vpn/amneziawg-go/v3/device"
	"github.com/amnezia-vpn/amneziawg-go/v3/tun/netstack"
)

func readKey(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil || len(key) != 32 {
		log.Fatal("invalid fixture key")
	}
	return key
}

func main() {
	if len(os.Args) != 3 {
		log.Fatal("usage: peer PRIVATE_KEY PEER_PUBLIC_KEY")
	}
	local := netip.MustParseAddr("10.77.0.1")
	tun, network, err := netstack.CreateNetTUN([]netip.Addr{local}, nil, 1420)
	if err != nil {
		log.Fatal(err)
	}
	dev := device.NewDevice(tun, conn.NewDefaultBind(), device.NewLogger(device.LogLevelSilent, ""))
	defer dev.Close()
	config := fmt.Sprintf("private_key=%x\nlisten_port=51820\npublic_key=%x\nallowed_ip=10.77.0.2/32\n",
		readKey(os.Args[1]), readKey(os.Args[2]))
	if err := dev.IpcSet(config); err != nil {
		log.Fatal("configure fixture peer")
	}
	if err := dev.Up(); err != nil {
		log.Fatal("start fixture peer")
	}
	listener, err := network.ListenTCPAddrPort(netip.AddrPortFrom(local, 8080))
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "wireproxy-smoke\n")
	})))
}
