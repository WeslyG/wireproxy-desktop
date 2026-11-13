#!/bin/sh
set -eu

if [ "$#" -lt 1 ] || [ "$#" -gt 3 ] || [ "$(uname -s)" != Darwin ]; then
    echo "Usage on macOS: sh scripts/build-macos-app.sh 'path/to/Wireproxy Desktop.app' [built-executable] [version]" >&2
    exit 1
fi

output=$1
case "$output" in
    /*.app) ;;
    *.app) output="$PWD/$output" ;;
    *) echo "The output path must end in .app." >&2; exit 1 ;;
esac
if [ -e "$output" ] || [ -L "$output" ]; then
    echo "The output already exists: $output" >&2
    exit 1
fi

binary=${2:-}
version=${3:-dev}
case "$version" in
    dev) version=0.0.0 ;;
    *) version=${version#v}
       printf '%s\n' "$version" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$' ;;
esac
case "$binary" in
    ""|/*) ;;
    *) binary="$PWD/$binary" ;;
esac
cd "$(dirname "$0")/.."
build_dir=$(mktemp -d)
trap 'rm -rf "$build_dir"' EXIT
bundle="$build_dir/Wireproxy Desktop.app"
mkdir -p "$bundle/Contents/MacOS" "$bundle/Contents/Resources"

if [ -n "$binary" ]; then
    cp -p "$binary" "$bundle/Contents/MacOS/wireproxy-desktop"
else
    MACOSX_DEPLOYMENT_TARGET=${MACOSX_DEPLOYMENT_TARGET:-15.0} CGO_ENABLED=1 \
        go build -trimpath -o "$bundle/Contents/MacOS/wireproxy-desktop" .
fi
sips -s format icns assets/icon.png --out "$bundle/Contents/Resources/AppIcon.icns" >/dev/null

cat > "$bundle/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleIdentifier</key>
    <string>com.weslyg.wireproxy-desktop</string>
    <key>CFBundleName</key>
    <string>Wireproxy Desktop</string>
    <key>CFBundleExecutable</key>
    <string>wireproxy-desktop</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleIconFile</key>
    <string>AppIcon.icns</string>
    <key>CFBundleShortVersionString</key>
    <string>$version</string>
    <key>CFBundleVersion</key>
    <string>$version</string>
</dict>
</plist>
EOF

plutil -lint "$bundle/Contents/Info.plist"
codesign --force --sign - "$bundle"
codesign --verify --strict "$bundle"
mv "$bundle" "$output"
