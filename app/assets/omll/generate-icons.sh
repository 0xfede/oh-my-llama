#!/bin/sh
# Regenerate the Oh My Llama bundle icons from the SVG sources in this directory.
#
# Requires rsvg-convert (brew install librsvg) and iconutil (Xcode CLT).
# Run from the repo root:  ./app/assets/omll/generate-icons.sh

set -e

SRC=$(cd "$(dirname "$0")" && pwd)
DEST=${1:-app/darwin/OhMyLlama.app/Contents/Resources}

status() { echo >&2 ">>> $@"; }

if ! command -v rsvg-convert >/dev/null 2>&1; then
    echo "rsvg-convert not found. Install it with: brew install librsvg" >&2
    exit 1
fi

[ -d "$DEST" ] || { echo "destination $DEST does not exist" >&2; exit 1; }

status "Building icon.icns"
ICONSET=$(mktemp -d)/icon.iconset
mkdir -p "$ICONSET"
for SPEC in "16 icon_16x16" "32 icon_16x16@2x" "32 icon_32x32" "64 icon_32x32@2x" \
            "128 icon_128x128" "256 icon_128x128@2x" "256 icon_256x256" \
            "512 icon_256x256@2x" "512 icon_512x512" "1024 icon_512x512@2x"; do
    SIZE=${SPEC% *}
    NAME=${SPEC#* }
    rsvg-convert -w "$SIZE" -h "$SIZE" "$SRC/appicon.svg" -o "$ICONSET/$NAME.png"
done
iconutil -c icns "$ICONSET" -o "$DEST/icon.icns"
rm -rf "$(dirname "$ICONSET")"

# Menu bar icons. These are macOS template images: the glyph is opaque black on
# transparency and AppKit tints it to match the menu bar. The app still asks for
# a *Dark suffixed name, so we ship pre-inverted copies under those names.
#
# The 1x (16px) renders come from a separate, simplified drawing: at that size
# the muzzle outline and nostrils blur into an unreadable blob.
for SPEC in "tray ollama" "tray-update ollamaUpdate"; do
    SVG=${SPEC% *}
    NAME=${SPEC#* }
    status "Building $NAME"
    rsvg-convert -w 16 -h 16 "$SRC/$SVG-16.svg" -o "$DEST/$NAME.png"
    rsvg-convert -w 32 -h 32 "$SRC/$SVG.svg" -o "$DEST/$NAME@2x.png"
    # Inverted variants for the dark menu bar
    magick "$DEST/$NAME.png" -channel RGB -negate "$DEST/${NAME}Dark.png"
    magick "$DEST/$NAME@2x.png" -channel RGB -negate "$DEST/${NAME}Dark@2x.png"
done

# Raster copy of the app icon for the README header: GitHub sanitizes inline SVG
# and renders linked SVGs inconsistently, so the docs point at this instead.
status "Building appicon.png"
rsvg-convert -w 320 -h 320 "$SRC/appicon.svg" -o "$SRC/appicon.png"

status "Wrote icons to $DEST"
