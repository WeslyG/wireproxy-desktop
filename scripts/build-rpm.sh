#!/bin/sh
set -eu

out=$(CDPATH= cd -- "$1" && pwd -P)
tag=$2
target=$3
case "$target:$(uname -m)" in
    linux-amd64:x86_64|linux-arm64:aarch64) ;;
    *) printf '%s\n' 'Build the RPM on a matching Linux architecture.' >&2; exit 2 ;;
esac
if [ "$tag" = dev ]; then
    revision=$4
    printf '%s\n' "$revision" | grep -Eq '^[0-9a-f]{12}$'
    version=0.0.0
    release=0.dev.$revision
else
    printf '%s\n' "$tag" | grep -Eq '^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'
    version=${tag#v}
    release=1
fi

cd "$(dirname "$0")/.."
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
mkdir "$work/SOURCES"
install -m 755 "$out/wireproxy-desktop-$target" "$work/SOURCES/wireproxy-desktop"
cp assets/icon.png LICENSE.wireproxy-awg extensions/gnome/extension.js extensions/gnome/metadata.json \
    packaging/rpm/*.desktop packaging/rpm/*.service "$work/SOURCES/"
rpmbuild -bb --define "_topdir $work" --define "wireproxy_version $version" \
    --define "wireproxy_release $release" packaging/rpm/wireproxy-desktop.spec
install -m 644 "$work"/RPMS/*/*.rpm "$out/"
