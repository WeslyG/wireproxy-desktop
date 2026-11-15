#!/bin/sh
# Run in a private D-Bus session with a disposable home.
set -eu
test "$#" -eq 2
archive=$(realpath "$1")
tools=$(realpath "$2")
tests=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
qa=$WIREPROXY_TEST_ROOT
pids=
cleanup() {
    for pid in $pids; do kill "$pid" 2>/dev/null || :; done
    wait 2>/dev/null || :
    rm -rf -- "$qa"
}
trap cleanup EXIT
export HOME="$qa/home" XDG_CONFIG_HOME="$qa/config" XDG_DATA_HOME="$qa/data"
export XDG_RUNTIME_DIR="$qa/runtime" XDG_CACHE_HOME="$qa/cache"
export PATH="$tools/usr/bin:$PATH" LD_LIBRARY_PATH="$tools/usr/lib64"
# The temporary profile and WireGuard fixture use ports 18880 and 51820.
export WIREPROXY_QA_DIR="$qa" GDK_BACKEND=x11 LANGUAGE=en LC_ALL=C.UTF-8
export GIO_USE_VFS=local GSETTINGS_BACKEND=memory
export XDG_SESSION_TYPE=x11 XDG_CURRENT_DESKTOP=GNOME
unset WAYLAND_DISPLAY WAYLAND_SOCKET
mkdir -p "$HOME" "$XDG_DATA_HOME" "$XDG_CONFIG_HOME/Wireproxy Desktop"
mkdir -m 700 "$XDG_RUNTIME_DIR"
umask 077

wg genkey > "$qa/client.key"
wg genkey > "$qa/server.key"
wg pubkey < "$qa/client.key" > "$qa/client.pub"
wg pubkey < "$qa/server.key" > "$qa/server.pub"
"$tools/peer-fixture" "$qa/server.key" "$qa/client.pub" > "$qa/peer.log" 2>&1 &
pids="$! $pids"
python3 - "$qa" "$XDG_CONFIG_HOME/Wireproxy Desktop/profiles.json" <<'PY'
import json, pathlib, sys
q, path = map(pathlib.Path, sys.argv[1:])
config = f"""[Interface]
PrivateKey = {(q / 'client.key').read_text().strip()}
Address = 10.77.0.2/32
[Peer]
PublicKey = {(q / 'server.pub').read_text().strip()}
AllowedIPs = 10.77.0.0/24
Endpoint = 127.0.0.1:51820
PersistentKeepalive = 1
"""
path.write_text(json.dumps({"defaultProfile": "Fedora smoke profile",
    "profiles": [{"name": "Fedora smoke profile", "config": config}],
    "settings": {"port": 18880, "host": "127.0.0.1", "language": "en", "languageOverride": True}}))
PY
tar -xzf "$archive" -C "$qa"
bundle="$qa/wireproxy-desktop-linux-$(uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/')"
cd "$bundle"
printf 'fixture-password' | gnome-keyring-daemon --unlock --components=secrets > "$qa/keyring.log" 2>&1 || { cat "$qa/keyring.log"; exit 1; }
sh install.sh --gnome
sh install.sh
binary="$HOME/.local/bin/wireproxy-desktop"
cmp "$binary" ./wireproxy-desktop
ext="$XDG_DATA_HOME/gnome-shell/extensions/wireproxy-quick-settings@weslyg.github.com"
cmp "$ext/extension.js" gnome-extension/extension.js
cp "$tests/tile.js" "$ext/test-tile.js"
sed -i '/this\._refresh(true);/a\        import("./test-tile.js").then(({exerciseTile}) => exerciseTile(this));' "$ext/extension.js"

Xvfb -displayfd 3 -screen 0 1024x768x24 3>"$qa/display" >"$qa/xvfb.log" 2>&1 &
pids="$! $pids"
wait_for() {
    for count in $(seq 1 120); do
        if "$@"; then return 0; fi
        sleep .2
    done
    printf 'Timed out waiting for %s\n' "$*" >&2
    if [ -f "$qa/gnome.log" ]; then tail -40 "$qa/gnome.log" >&2; fi
    if [ -f "$qa/peer.log" ]; then cat "$qa/peer.log" >&2; fi
    return 1
}
test_display() { test -s "$qa/display"; }
wait_for test_display
export DISPLAY=":$(cat "$qa/display")"
dbus-update-activation-environment DISPLAY HOME XDG_CONFIG_HOME XDG_DATA_HOME XDG_RUNTIME_DIR XDG_CACHE_HOME XDG_SESSION_TYPE XDG_CURRENT_DESKTOP
"$binary" > "$qa/app.log" 2>&1 &
pids="$! $pids"
app_window() { xdotool search --name '^Wireproxy Desktop$' 2>/dev/null; }
no_window() { ! xdotool search --name '^(Wireproxy Desktop|Gio)$' >/dev/null 2>&1; }
wait_for app_window
window=$(app_window | head -1)
python3 "$tests/close-window.py" "$window"
wait_for no_window
printf '%s\n' 'WIREPROXY_WINDOW_CLOSED'

mkdir "$qa/schemas"
cp /usr/share/glib-2.0/schemas/org.gnome.shell.gschema.xml "$qa/schemas/"
cat > "$qa/schemas/90-wireproxy.gschema.override" <<'SCHEMAS'
[org.gnome.shell]
disable-user-extensions=false
enabled-extensions=['wireproxy-quick-settings@weslyg.github.com']
SCHEMAS
glib-compile-schemas "$qa/schemas"
export GSETTINGS_SCHEMA_DIR="$qa/schemas" GSETTINGS_BACKEND=memory
MUTTER_DEBUG_DUMMY_MODE_SPECS=1024x768 gnome-shell --wayland --headless --no-x11 --virtual-monitor 1024x768 > "$qa/gnome.log" 2>&1 &
pids="$! $pids"
tile() { grep 'WIREPROXY_TILE ' "$qa/gnome.log" | tail -1 | grep -F 'Fedora smoke profile | '"$1"; }
wait_for tile Disconnected
app_id=io.github.WeslyG.WireproxyDesktop
no_owner() {
    gdbus call --session --dest org.freedesktop.DBus --object-path /org/freedesktop/DBus \
        --method org.freedesktop.DBus.NameHasOwner "$app_id" | grep -Fq false
}
app_pid() {
    gdbus call --session --dest org.freedesktop.DBus --object-path /org/freedesktop/DBus \
        --method org.freedesktop.DBus.GetConnectionUnixProcessID "$app_id" | sed -n 's/^(uint32 \([0-9]\+\),)$/\1/p'
}
if [ "${WIREPROXY_MEASURE:-0}" = 1 ]; then
    no_window
    python3 "$tests/metrics.py" idle "$(app_pid)" 8
    no_window
fi
wait_for no_owner
wait_for tile Disconnected
# Headless Mutter updates the activation environment at startup. Point the
# app launched later by D-Bus to the Xvfb display used for window checks.
export WAYLAND_DISPLAY=/dev/null
dbus-update-activation-environment DISPLAY WAYLAND_DISPLAY XDG_SESSION_TYPE XDG_CURRENT_DESKTOP
: > "$qa/click-1"
wait_for tile Connected
curl --fail --silent --max-time 5 --noproxy '' --proxy socks5h://127.0.0.1:18880 http://10.77.0.1:8080/ | grep -Fx wireproxy-smoke
no_window
"$binary"
wait_for app_window
python3 "$tests/close-window.py" "$(app_window | head -1)"
wait_for no_window
wait_for tile Connected
printf '%s\n' 'WIREPROXY_WINDOW_REOPENED_AND_CLOSED'
if [ "${WIREPROXY_MEASURE:-0}" = 1 ]; then
    python3 "$tests/metrics.py" connected "$(app_pid)" 20
    no_window
    wait_for tile Connected
fi
: > "$qa/click-2"
wait_for grep -q 'WIREPROXY_CLICK 2' "$qa/gnome.log"
wait_for tile Disconnected
no_window
if curl --fail --silent --max-time 2 --noproxy '' --proxy socks5h://127.0.0.1:18880 http://10.77.0.1:8080/; then exit 1; fi

sh install.sh --uninstall
test ! -e "$binary"
test ! -e "$ext/extension.js"
test ! -e "$XDG_CONFIG_HOME/Wireproxy Desktop/profiles.json"
secret-tool lookup service 'Wireproxy Desktop' username default >/dev/null
printf '%s\n' 'WIREPROXY_FEDORA_SMOKE_PASS'
