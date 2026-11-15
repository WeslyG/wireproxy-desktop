#!/bin/sh
set -eu

source_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
app_id=io.github.WeslyG.WireproxyDesktop
binary="$HOME/.local/bin/wireproxy-desktop"
data_dir=${XDG_DATA_HOME:-"$HOME/.local/share"}
desktop="$data_dir/applications/$app_id.desktop"
icon="$data_dir/icons/hicolor/128x128/apps/$app_id.png"

service="$data_dir/dbus-1/services/$app_id.service"
uuid=wireproxy-quick-settings@weslyg.github.com
extension="$data_dir/gnome-shell/extensions/$uuid"

case "${1:-}" in
    ''|--gnome|--uninstall) ;;
    *) printf '%s\n' 'Usage: sh install.sh [--gnome|--uninstall]' >&2; exit 2 ;;
esac

if [ "${1:-}" = --uninstall ]; then
    if command -v gnome-extensions >/dev/null 2>&1; then
        gnome-extensions disable "$uuid" 2>/dev/null || :
    fi
    rm -f -- "$binary" "$desktop" "$icon" "$service" "$extension/extension.js" "$extension/metadata.json"
    rmdir -- "$extension" 2>/dev/null || :
    printf '%s\n' 'Uninstalled Wireproxy Desktop. Saved profiles were kept.'
    exit 0
fi

if [ ! -f "$source_dir/wireproxy-desktop" ] || [ ! -f "$source_dir/icon.png" ]; then
    printf '%s\n' 'Run install.sh from the extracted Wireproxy Desktop archive.' >&2
    exit 1
fi

install_gnome=false
if [ "${1:-}" = --gnome ] || [ -f "$service" ]; then
    for file in metadata.json extension.js; do
        if [ ! -f "$source_dir/gnome-extension/$file" ]; then
            printf '%s\n' 'The archive is missing GNOME extension files.' >&2
            exit 1
        fi
    done
    install_gnome=true
fi

install -D -m 755 "$source_dir/wireproxy-desktop" "$binary"
install -D -m 644 "$source_dir/icon.png" "$icon"
mkdir -p -- "$(dirname -- "$desktop")"

# Escape the path for both desktop-entry parsing and the quoted Exec argument.
quoted_path=$(printf '%s' "$binary" | sed 's/\\/\\\\\\\\/g; s/["`$]/\\\\&/g')
exec_path=$(printf '%s' "$quoted_path" | sed 's/%/%%/g')
temp=$(mktemp "$desktop.XXXXXX")
trap 'rm -f -- "$temp"' EXIT
cat > "$temp" <<EOF
[Desktop Entry]
Type=Application
Name=Wireproxy Desktop
Exec="$exec_path"
Icon=$app_id
Terminal=false
Categories=Network;
StartupWMClass=$app_id
EOF
chmod 644 "$temp"
mv -f -- "$temp" "$desktop"

# GNOME Files can also show a per-file icon on the installed executable.
if command -v gio >/dev/null 2>&1; then
    icon_uri=$(LC_ALL=C gio info -a standard::type "$icon" 2>/dev/null | sed -n 's/^uri: //p')
    if [ -n "$icon_uri" ]; then
        gio set "$binary" metadata::custom-icon "$icon_uri" 2>/dev/null || :
    fi
fi

printf '%s\n' 'Installed Wireproxy Desktop. Open it from your applications menu.'

if [ "$install_gnome" = true ]; then
    mkdir -p -- "$(dirname -- "$service")"
    temp=$(mktemp "$service.XXXXXX")
    cat > "$temp" <<EOF
[D-BUS Service]
Name=$app_id
Exec="$quoted_path" --background
EOF
    chmod 644 "$temp"
    mv -f -- "$temp" "$service"
    if command -v gdbus >/dev/null 2>&1; then
        gdbus call --session --timeout 2 --dest org.freedesktop.DBus \
            --object-path /org/freedesktop/DBus --method org.freedesktop.DBus.ReloadConfig >/dev/null 2>&1 || :
    fi
    install -D -m 644 "$source_dir/gnome-extension/metadata.json" "$extension/metadata.json"
    install -m 644 "$source_dir/gnome-extension/extension.js" "$extension/extension.js"
    printf '%s\n' 'Sign out and back in to load the installed GNOME extension.'
    if ! command -v gnome-extensions >/dev/null 2>&1 || ! gnome-extensions enable "$uuid" 2>/dev/null; then
        printf 'Sign out and back in, then run: gnome-extensions enable %s\n' "$uuid"
    fi
fi
