%global debug_package %{nil}
%global _build_id_links none
%global appid io.github.WeslyG.WireproxyDesktop
%global extension wireproxy-quick-settings@weslyg.github.com

Name: wireproxy-desktop
Version: %{wireproxy_version}
Release: %{wireproxy_release}
Summary: Desktop client for a WireGuard and AmneziaWG SOCKS5 proxy
License: LicenseRef-Wireproxy-Desktop AND ISC
URL: https://github.com/WeslyG/wireproxy-desktop

Source0: wireproxy-desktop
Source1: icon.png
Source2: LICENSE.wireproxy-awg
Source3: extension.js
Source4: metadata.json
Source5: %{appid}.desktop
Source6: %{appid}.service

%description
Wireproxy Desktop with an optional GNOME Shell Quick Settings tile.

%install
install -Dm755 %{SOURCE0} %{buildroot}%{_bindir}/wireproxy-desktop
install -Dm644 %{SOURCE1} %{buildroot}%{_datadir}/icons/hicolor/128x128/apps/%{appid}.png
install -Dm644 %{SOURCE2} %{buildroot}%{_licensedir}/%{name}/LICENSE.wireproxy-awg
install -Dm644 %{SOURCE3} %{buildroot}%{_datadir}/gnome-shell/extensions/%{extension}/extension.js
install -Dm644 %{SOURCE4} %{buildroot}%{_datadir}/gnome-shell/extensions/%{extension}/metadata.json
install -Dm644 %{SOURCE5} %{buildroot}%{_datadir}/applications/%{appid}.desktop
install -Dm644 %{SOURCE6} %{buildroot}%{_datadir}/dbus-1/services/%{appid}.service
desktop-file-validate %{buildroot}%{_datadir}/applications/%{appid}.desktop

%files
%{_bindir}/wireproxy-desktop
%{_datadir}/icons/hicolor/128x128/apps/%{appid}.png
%license %{_licensedir}/%{name}/LICENSE.wireproxy-awg
%{_datadir}/gnome-shell/extensions/%{extension}/
%{_datadir}/applications/%{appid}.desktop
%{_datadir}/dbus-1/services/%{appid}.service
