#!/bin/sh
# Builds the macOS app locally: ad hoc signed, not
# notarized, not uploaded. Output: build/darwin-arm64/ (.app and .dmg).
# Pass extra mygo build flags through, e.g. -platform darwin/universal.
set -eu
cd "$(dirname "$0")/.."

# actool (the app icon) ships with Xcode, not the Command Line Tools.
if [ -z "${DEVELOPER_DIR:-}" ] && [ -d /Applications/Xcode.app ]; then
	export DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer
fi

CGO_ENABLED=0 go test ./internal/...
go tool mygo build -sign - -skip-notarize "$@"
