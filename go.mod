module github.com/WeslyG/wireproxy-desktop

go 1.26.11

require (
	fyne.io/systray v1.12.2
	gioui.org v0.10.3
	github.com/amnezia-vpn/amneziawg-go/v3 v3.1.20260814
	github.com/artem-russkikh/wireproxy-awg v0.0.0
	github.com/go-ini/ini v1.67.0
	github.com/godbus/dbus/v5 v5.2.2
	github.com/things-go/go-socks5 v0.0.5
	github.com/zalando/go-keyring v0.2.6
	golang.org/x/net v0.48.0
	golang.org/x/sys v0.40.0
)

require (
	al.essio.dev/pkg/shellescape v1.5.1 // indirect
	gioui.org/shader v1.0.9 // indirect
	github.com/MakeNowJust/heredoc/v2 v2.0.1 // indirect
	github.com/danieljoos/wincred v1.2.2 // indirect
	github.com/go-text/typesetting v0.3.5 // indirect
	github.com/google/btree v1.1.3 // indirect
	golang.org/x/crypto v0.46.0 // indirect
	golang.org/x/exp/shiny v0.0.0-20250408133849-7e4ce0ab07d0 // indirect
	golang.org/x/image v0.26.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	golang.org/x/time v0.9.0 // indirect
	golang.zx2c4.com/wintun v0.0.0-20230126152724-0fa3db229ce2 // indirect
	gvisor.dev/gvisor v0.0.0-20250718015824-35000683b6d7 // indirect
)

replace github.com/artem-russkikh/wireproxy-awg => github.com/WeslyG/wireproxy-awg v0.0.0-20261112122723-d9037fe17fc1
