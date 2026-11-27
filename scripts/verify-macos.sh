#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

version=${1:-}
config_backup=''
cleanup() {
    if [[ -n "$config_backup" ]]; then
        cp "$config_backup" wails.json
        rm -f "$config_backup"
    fi
}
trap cleanup EXIT

unformatted=$(git ls-files -z --cached --others --exclude-standard '*.go' | xargs -0 gofmt -l)
if [[ -n "$unformatted" ]]; then
    printf 'Run gofmt on:\n%s\n' "$unformatted" >&2
    exit 1
fi

npm --prefix frontend ci
npm --prefix frontend test
npm --prefix frontend run build
go test ./... -count=1 -timeout=3m
go vet ./...

# The frontend already built against the committed bindings.
build_args=(-s -m -nosyncgomod -skipbindings -platform darwin/universal)
if [[ -n "$version" ]]; then
    if [[ ! "$version" =~ ^(0|[1-9][0-9]{0,4})\.(0|[1-9][0-9]{0,4})\.(0|[1-9][0-9]{0,4})$ ]]; then
        echo 'Use a three-part version without a v prefix.' >&2
        exit 1
    fi
    config_backup=$(mktemp)
    cp wails.json "$config_backup"
    node - "$version" <<'NODE'
const fs = require('node:fs');
const config = JSON.parse(fs.readFileSync('wails.json', 'utf8'));
config.info = { ...config.info, productName: 'Hostlight', productVersion: process.argv[2] };
fs.writeFileSync('wails.json', JSON.stringify(config, null, 2));
NODE
    build_args+=(-ldflags "-X main.applicationVersion=$version")
fi

# Match LSMinimumSystemVersion in build/darwin/Info.plist on both slices.
CGO_CFLAGS="${CGO_CFLAGS:-} -mmacosx-version-min=13.0" \
CGO_LDFLAGS="${CGO_LDFLAGS:-} -mmacosx-version-min=13.0" \
    wails build "${build_args[@]}"
bundle=build/bin/hostlight.app
plist="$bundle/Contents/Info.plist"
binary="$bundle/Contents/MacOS/hostlight"
plutil -lint "$plist"
archs=$(lipo -archs "$binary")
if [[ " $archs " != *" arm64 "* || " $archs " != *" x86_64 "* ]]; then
    printf 'Expected arm64 and x86_64 in the app binary; found %s\n' "$archs" >&2
    exit 1
fi
codesign --force --sign - "$bundle"
codesign --verify --deep --strict "$bundle"

if [[ -n "$version" ]]; then
    test "$(plutil -extract CFBundleVersion raw "$plist")" = "$version"
    test "$(plutil -extract CFBundleShortVersionString raw "$plist")" = "$version"
    output="build/bin/release/$version"
    archive="$output/hostlight-$version-macos-universal.zip"
    if [[ -e "$archive" ]]; then
        echo 'This Mac version is already packaged locally.' >&2
        exit 1
    fi
    staging=$(mktemp -d)
    trap 'rm -rf "$staging"; cleanup' EXIT
    ditto "$bundle" "$staging/Hostlight.app"
    cp LICENSE "$staging/"
    cat > "$staging/README.txt" <<NOTES
Hostlight $version — macOS preview for Apple Silicon and Intel

Extract the ZIP and open Hostlight.app. The app is ad-hoc signed
and has no Apple Developer ID signature or notarization. Gatekeeper
may block the first launch. Verify the ZIP with SHA256SUMS.txt from
the official release. See https://support.apple.com/en-us/102445.

Use TCP or HTTP/HTTPS monitors on macOS. Tray monitoring, ICMP ping,
and encrypted webhook credential storage are Windows-only.
Monitoring stops when you quit Hostlight.

License: GNU GPL version 3 (GPL-3.0-only). See LICENSE.
Matching source: https://github.com/DavFount/Hostlight/tree/v$version
NOTES
    ditto -c -k --keepParent "$staging/Hostlight.app" "$staging/app.zip"
    (cd "$staging" && zip -q app.zip LICENSE README.txt)
    mkdir -p "$output"
    mv "$staging/app.zip" "$archive"
fi
