#!/bin/sh
set -eu

app=$1
: "${MACOS_CERTIFICATE_BASE64:?Developer ID Application certificate is required}"
: "${MACOS_CERTIFICATE_PASSWORD:?PKCS#12 password is required}"
: "${APPLE_ID:?Apple Developer account is required}"
: "${APPLE_TEAM_ID:?Apple Developer Team ID is required}"
: "${APPLE_APP_PASSWORD:?Apple app-specific password is required}"

work=$(mktemp -d)
keychain="$work/signing.keychain-db"
cleanup() {
    security delete-keychain "$keychain" >/dev/null 2>&1 || true
    rm -rf -- "$work"
}
trap cleanup EXIT

password=$(openssl rand -hex 32)
printf '%s' "$MACOS_CERTIFICATE_BASE64" | base64 -D > "$work/certificate.p12"
security create-keychain -p "$password" "$keychain"
security unlock-keychain -p "$password" "$keychain"
security import "$work/certificate.p12" -k "$keychain" -P "$MACOS_CERTIFICATE_PASSWORD" -T /usr/bin/codesign
security set-key-partition-list -S apple-tool:,apple: -s -k "$password" "$keychain" >/dev/null
identity=$(security find-identity -v -p codesigning "$keychain" |
    awk -v team="$APPLE_TEAM_ID" '/Developer ID Application:/ && index($0, "(" team ")") { print $2; exit }')
if [ -z "$identity" ]; then
    echo "No Developer ID Application identity for the configured Apple Team." >&2
    exit 1
fi

codesign --force --sign "$identity" --keychain "$keychain" --options runtime --timestamp "$app"
codesign --verify --deep --strict "$app"
ditto -c -k --keepParent "$app" "$work/notarization.zip"
xcrun notarytool submit "$work/notarization.zip" --apple-id "$APPLE_ID" \
    --team-id "$APPLE_TEAM_ID" --password "$APPLE_APP_PASSWORD" --wait --timeout 30m
xcrun stapler staple "$app"
xcrun stapler validate "$app"
codesign --verify --deep --strict "$app"
