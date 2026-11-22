#!/bin/sh
set -eu

output_dir=${1:-dist}
version=${2:-dev}
mkdir -p -- "$output_dir"
output_dir=$(CDPATH= cd -- "$output_dir" && pwd -P)
cd "$(dirname "$0")/.."
if [ "$version" != dev ]; then
    printf '%s\n' "$version" | grep -Eq '^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'
fi
arch=$(go env GOARCH)
case "$arch" in amd64|arm64) ;; *) exit 2 ;; esac
output="$output_dir/wireproxy-desktop-linux-$arch"
CGO_ENABLED=1 go build -tags novulkan -trimpath \
    -ldflags="-s -w -X main.version=$version -X gioui.org/app.ID=io.github.WeslyG.WireproxyDesktop" \
    -o "$output" .

bundle=$(mktemp -d)
trap 'rm -rf -- "$bundle"' EXIT
name=${output##*/}
install -D -m 755 "$output" "$bundle/$name/wireproxy-desktop"
install -m 755 scripts/install-linux.sh "$bundle/$name/install.sh"
install -m 644 assets/icon.png "$bundle/$name/icon.png"
install -m 644 LICENSE.wireproxy-awg "$bundle/$name/"
mkdir "$bundle/$name/gnome-extension"
install -m 644 extensions/gnome/metadata.json extensions/gnome/extension.js "$bundle/$name/gnome-extension/"
tar -czf "$output.tar.gz" -C "$bundle" "$name"
