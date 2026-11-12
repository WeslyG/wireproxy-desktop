# Wireproxy Desktop

A portable desktop app for AmneziaWG and WireGuard SOCKS5 proxies. The native
window uses Gio and the same executable runs the proxy engine from the
[wireproxy-awg fork](https://github.com/WeslyG/wireproxy-awg). It runs without
administrator privileges, a separate wireproxy executable, or an installer.

## Use the app

1. Under **Servers**, click **Add**, enter a name, and paste the entire
   `vpn://` link or the contents of an AmneziaWG or WireGuard INI file.
   **Check profile** tries an HTTPS request through the draft tunnel before
   saving. **Convert to INI** opens the decoded configuration in the INI tab.
2. Save and click **Connect**. Select **Set as default** to pin the server
   and its Connect button above the list. Set your browser or other client to
   `socks5://127.0.0.1:1080`, or use **Copy address**.
3. Click **Test SOCKS5** to send an HTTPS request through the tunnel to
   `example.com`. **Logs** shows the handshake, WireGuard engine messages,
   and the test result. Enable **Verbose WireGuard logs** for handshake attempts,
   retries, and device events. The protocol label is inferred from the profile.
4. Disconnect and select another saved profile to switch servers.

**Settings** has English and Russian, a SOCKS5 listen address and port, and
optional username/password authentication. Enter `0.0.0.0` for all IPv4
interfaces or `::` for IPv6. Proxy settings apply on the next connection.
When importing INI, the app uses
`[Interface]` and `[Peer]` for the tunnel. The GUI supplies the SOCKS5 listener.

Profiles and settings are saved in the OS user configuration directory. See
**Settings** for the exact path. To store them alongside the executable,
launch it with `--portable`. Private keys and SOCKS5 credentials are stored as
plain JSON under your user account. Keep the profile file private when copying
or backing up the executable. The activity log stays in memory.

Closing the window stops SOCKS5 and leaves Wireproxy Desktop in the system
tray. Use **Open Wireproxy Desktop** to reopen the window or **Quit** to exit.
Reopening leaves SOCKS5 disconnected until you click **Connect**. On Linux,
the tray requires a StatusNotifier host, such as KDE Plasma's system tray
or GNOME with an AppIndicator extension. Closing the window exits the app
if a tray host is unavailable.

## Updates and releases

Tagged builds check GitHub for a newer stable release when the app opens.
Open **Settings → App updates** to check again or choose **Install and
restart**. The app downloads the binary for the current OS and architecture,
verifies its SHA-256 digest, then disconnects the proxy and restarts. The
executable directory must be writable. Your profile file stays in place.

For a release, add a `## vX.Y.Z` section to `CHANGELOG.md`, then push that
tag. GitHub Actions tests and builds all six targets before publishing the
binaries and `SHA256SUMS`. The release notes combine the matching changelog
section with GitHub's generated notes. The updater reads the public Releases
API for `WeslyG/wireproxy-desktop`.

## Build from source

Use Go 1.26.11 or newer. Build separately for each OS and CPU architecture.

| Platform | Build environment | Command |
| --- | --- | --- |
| Windows | Go | `go build -trimpath -ldflags="-s -w -H=windowsgui" -o wireproxy-desktop.exe .` |
| macOS | Go and Xcode Command Line Tools | `CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o wireproxy-desktop .` |
| Linux | Go, a C compiler, and Gio's X11/Wayland and EGL development libraries | `CGO_ENABLED=1 go build -tags novulkan -trimpath -ldflags="-s -w" -o wireproxy-desktop .` |

Linux uses the graphics libraries already present in a desktop session.
The macOS build is a raw executable that can be launched from Terminal.
GitHub Actions applies an ad-hoc signature to both macOS binaries.
See [Gio's installation guide](https://gioui.org/doc/install) for system
requirements and additional architectures.
