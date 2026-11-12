# Changelog

## v0.4.0

- A single Connect / Disconnect control follows the current session state.
- Profile editing wraps Amnezia links, copies either format, and converts to INI on tab switch.
- Proxy, language, and update settings now have separate tabs.
- The proxy settings list local interfaces, check bind availability, and suggest a port when the chosen one is busy.
- The updates tab shows release notes and links to GitHub Releases.

## v0.3.0

- Close the window to the system tray with SOCKS5 disconnected.
- Reopen the window or quit from the tray on Windows, macOS, and Linux desktops with a tray host.
- Added a favicon, Windows executable icon, macOS Dock icon, and tray icons.

## v0.2.0

- Added Russian and English UI, draft tunnel checks, Amnezia-to-INI conversion, and protocol labels.
- Added a pinned default server with a Connect button in the status bar.
- Added configurable IPv4/IPv6 listening and optional SOCKS5 authentication.
- Exposed WireGuard engine diagnostics and improved disconnect feedback.

## v0.1.0

- Added a desktop client for AmneziaWG and WireGuard SOCKS5 proxies.
- Added server profiles, portable settings, activity logs, and a SOCKS5 test.
- Added a themed window title bar and in-app updates from GitHub Releases.
- Added portable binaries for Windows, Linux, and macOS on x64 and ARM64.
