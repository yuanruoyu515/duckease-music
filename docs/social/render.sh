#!/bin/sh
# Renders a social post: fetches the service logos, renders the app's
# screenshots at 2x, and screenshots each page of the post's HTML.
# Usage: docs/social/render.sh docs/social/example-dual.html out/
set -eu
cd "$(dirname "$0")/../.."
page=${1:-docs/social/example-dual.html}
out=${2:-build/social}
dir=docs/social
mkdir -p "$dir/logos" "$dir/shots" "$out"

# Simple Icons (CC0 drawings; the marks stay their owners' trademarks).
for n in spotify neteasecloudmusic; do
	[ -f "$dir/logos/$n.svg" ] || curl -sfL "https://cdn.jsdelivr.net/npm/simple-icons@13.21.0/icons/$n.svg" -o "$dir/logos/$n.svg"
done

NM_PRESS_SHOTS="$PWD/$dir/shots" NM_PRESS_SCALE=2 CGO_ENABLED=0 \
	go test ./internal/app -run '^TestPressScreenshots$' -count=1

chrome="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
url="file://$(python3 -I -c 'import urllib.parse,sys,os;print(urllib.parse.quote(os.path.abspath(sys.argv[1])))' "$page")"
n=$(grep -c 'class="page"' "$page")
i=1
while [ "$i" -le "$n" ]; do
	"$chrome" --headless=new --disable-gpu --hide-scrollbars --allow-file-access-from-files \
		--force-device-scale-factor=2 --virtual-time-budget=3000 --window-size=1242,1656 \
		--screenshot="$out/$i.png" "$url#$i" >/dev/null 2>&1
	i=$((i + 1))
done
echo "rendered $n pages to $out"
