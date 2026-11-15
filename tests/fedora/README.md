# Fedora GNOME smoke test

Build the Linux archive and the local WireGuard fixture on Linux.

```sh
sh scripts/build-linux.sh
go build -o dist/peer-fixture ./tests/fedora/peer
```

On a Fedora GNOME test machine, use GNOME Shell 51 with Xvfb, xdotool,
gnome-keyring, secret-tool, WireGuard tools, Python 3, and curl. Copy the
archive, the fixture, and this directory to the machine. Put the fixture
binary in a tools directory. The test also accepts RPMs extracted into
that directory as `usr/bin` and `usr/lib64`.

```sh
sh tests/fedora/run.sh wireproxy-desktop-linux-amd64.tar.gz /path/to/tools
```

The test needs UDP port 51820 and local TCP port 18880. It uses a separate
D-Bus session, disposable home and keyring, and synthetic WireGuard keys.
It starts GNOME Shell headless and exercises its actual QuickToggle. The
temporary copy of the extension contains a hook that emits button clicks
and reads the tile label. The packaged extension does not contain it.

A passing run verifies reinstall, legacy profile migration, closing and
reopening the window, D-Bus activation after idle exit, the primary
profile title, a handshake and HTTP via SOCKS5, shutdown with no Gio
window, and uninstall with the protected profile still present.

Set `WIREPROXY_MEASURE=1` to sample app CPU and memory after the window closes,
first with the profile off and then with the tunnel running. CPU is a
percentage of one logical core. RSS and PSS are sample means, in MiB.
The WireGuard worker is reported separately from the desktop process.
