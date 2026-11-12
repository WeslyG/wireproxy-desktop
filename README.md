# Wireproxy Desktop

A portable desktop app for AmneziaWG and WireGuard SOCKS5 proxies. The native
window uses Gio and the same executable runs the proxy engine from the
[wireproxy-awg fork](https://github.com/WeslyG/wireproxy-awg). It runs without
administrator privileges, a separate wireproxy executable, or an installer.

## Use the app

1. Under **Servers**, click **Add**, enter a name, and paste the entire
   `vpn://` link or the contents of an AmneziaWG or WireGuard INI file.
2. Save and click **Connect**. Set your browser or other client to
   `socks5://127.0.0.1:1080`, or use **Copy address**.
3. Click **Test SOCKS5** to send an HTTPS request through the tunnel to
   `example.com`. **Logs** shows the handshake and the test result.
4. Disconnect and select another saved profile to switch servers.

**Settings** has the local SOCKS5 port and a verbose logging toggle. Changes
apply on the next connection. The listener is local to the machine. Your
applications choose whether to use it. When importing INI, the app uses
`[Interface]` and `[Peer]` for the tunnel. The GUI supplies the SOCKS5 listener.

Profiles and settings are saved in the OS user configuration directory. See
**Settings** for the exact path. To store them alongside the executable,
launch it with `--portable`. Profiles contain private keys and are stored as
plain JSON under your user account. Keep the profile file private when copying
or backing up the executable. The activity log stays in memory.

## Build from source

Use Go 1.26.11 or newer. Build separately for each OS and CPU architecture.

| Platform | Build environment | Command |
| --- | --- | --- |
| Windows | Go | `go build -trimpath -ldflags="-s -w -H=windowsgui" -o wireproxy-desktop.exe .` |
| macOS | Go and Xcode Command Line Tools | `CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o wireproxy-desktop .` |
| Linux | Go, a C compiler, and Gio's X11/Wayland and EGL development libraries | `CGO_ENABLED=1 go build -tags novulkan -trimpath -ldflags="-s -w" -o wireproxy-desktop .` |

Linux uses the graphics libraries already present in a desktop session.
The macOS build is a raw executable that can be launched from Terminal.
See [Gio's installation guide](https://gioui.org/doc/install) for system
requirements and additional architectures.
