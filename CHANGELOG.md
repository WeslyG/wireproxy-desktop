# Changelog

## v1.0.2

- Connect and disconnect from the macOS menu bar. On macOS and Windows, connecting from the tray uses the default server and requires one to be configured.
- Retry macOS Keychain access after an error without restarting the app.
- Refine update controls, Windows tray and window behavior, and macOS app packaging.

## v1.0.1

- Fix a macOS startup freeze and use native window controls. The Dock reopens the window at its last position.
- Keep the SOCKS5 proxy running when the window closes to the tray. Show a red connection indicator after a confirmed handshake and prevent duplicate desktop instances.
- Build macOS app bundles for Intel and Apple Silicon. Download the ZIP and open the app from Finder. Bundle updates are available through GitHub Releases.
- Follow the system language by default on macOS, Windows, and Linux. On macOS, explain Keychain access before requesting it and show the message in the selected language.
- Try a fallback HTTPS endpoint when the primary SOCKS5 diagnostic endpoint fails.

## v1.0.0

- Store profiles and proxy credentials in Windows DPAPI, macOS Keychain, or Linux Secret Service. Migrate legacy JSON after verifying the protected copy.
- Detect AmneziaWG 3.1 profiles and display the bundled engine version separately.
- Require a confirmed tunnel before enabling SOCKS5 testing or copying the proxy URL. Cancel an active check when the tunnel stops responding.
- Mark the default server in the list, add a cancel action for removal, and refine status feedback and proxy settings.
- Refresh the application and tray icons, add Russian and English documentation with screenshots, and include a Linux development container.

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
