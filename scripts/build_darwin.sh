#!/bin/sh

# Note:
#  While testing, if you double-click on the app bundle
#  some state is left on MacOS and subsequent attempts
#  to build again will fail with:
#
#    hdiutil: create failed - Operation not permitted
#
#  To work around, specify another volume name with:
#
#    VOL_NAME="$(date)" ./scripts/build_darwin.sh
#

# Product identity. These must stay in sync with app/branding/branding.go, which
# is what the app and the updater compile against.
BUNDLE_NAME="OhMyLlama"
BUNDLE_ID="com.ohmyllama.app"

VOL_NAME=${VOL_NAME:-"Oh My Llama"}
export VERSION=${VERSION:-$(git describe --tags --first-parent --abbrev=7 --long --dirty --always | sed -e "s/^v//g")}

# Linker overrides baked into the app binary. UPDATE_FEED_URL is optional and only
# needed to point a test build at a different feed; the default lives in
# app/branding. Both targets are plain string vars initialized from constants,
# which is what -X requires.
GO_LDFLAGS_X="-X=github.com/ollama/ollama/app/version.Version=${VERSION}"
if [ -n "$UPDATE_FEED_URL" ]; then
    GO_LDFLAGS_X="$GO_LDFLAGS_X -X=github.com/ollama/ollama/app/updater.UpdateCheckURLBase=${UPDATE_FEED_URL}"
fi

export CGO_CFLAGS="-O3 -mmacosx-version-min=14.0"
export CGO_CXXFLAGS="-O3 -mmacosx-version-min=14.0"
export CGO_LDFLAGS="-mmacosx-version-min=14.0"

set -e

status() { echo >&2 ">>> $@"; }
usage() {
    echo "usage: $(basename $0) [build package app sign]"
    exit 1
}

mkdir -p dist

ARCHS="arm64 amd64"
while getopts "a:h" OPTION; do
    case $OPTION in
        a) ARCHS=$OPTARG ;;
        h) usage ;;
    esac
done

shift $(( $OPTIND - 1 ))

_build_darwin() {
    BUILD_CPUS=$(getconf _NPROCESSORS_ONLN)
    BUILD_JOBS=${OLLAMA_BUILD_PARALLEL:-$BUILD_CPUS}
    BUILD_LOAD=${OLLAMA_BUILD_LOAD:-$BUILD_CPUS}
    status "Build parallelism: $BUILD_JOBS, load limit: $BUILD_LOAD"

    SOURCE_BUILD=build/darwin-sources
    status "Preparing shared native sources"
    cmake -S . -B "$SOURCE_BUILD" -DOLLAMA_MLX_BACKENDS=metal_v3 -DOLLAMA_LLAMA_BACKENDS=
    cmake --build "$SOURCE_BUILD" --target ollama-llama-cpp-source --target ollama-mlx-sources
    LLAMA_CPP_SHARED_SRC="$(pwd)/$SOURCE_BUILD/_deps/llama_cpp-src"
    MLX_SHARED_SRC="$(pwd)/$SOURCE_BUILD/_deps/mlx-src"
    MLX_C_SHARED_SRC="$(pwd)/$SOURCE_BUILD/_deps/mlx-c-src"

    for ARCH in $ARCHS; do
        status "Building darwin $ARCH"
        INSTALL_PREFIX=dist/darwin-$ARCH/
        BUILD_DIR=build/darwin-$ARCH

        if [ "$ARCH" = "amd64" ]; then
            CMAKE_ARCH=x86_64
            MLX_BACKENDS=metal_v3
            MLX_EXTRA_ARGS="-DMLX_ENABLE_X64_MAC=ON"
            MLX_CGO_CFLAGS="-O3 -mmacosx-version-min=14.0"
            MLX_CGO_LDFLAGS="-ldl -lc++ -framework Accelerate -mmacosx-version-min=14.0"
        else
            CMAKE_ARCH=arm64
            MLX_BACKENDS="metal_v3;metal_v4"
            MLX_EXTRA_ARGS=
            MLX_CGO_CFLAGS="-O3 -mmacosx-version-min=14.0"
            MLX_CGO_LDFLAGS="-lc++ -framework Metal -framework Foundation -framework Accelerate -mmacosx-version-min=14.0"
        fi

        cmake -S . -B "$BUILD_DIR" \
            -DCMAKE_BUILD_TYPE=Release \
            -DCMAKE_OSX_ARCHITECTURES=$CMAKE_ARCH \
            -DCMAKE_OSX_DEPLOYMENT_TARGET=14.0 \
            -DCMAKE_INSTALL_PREFIX=$INSTALL_PREFIX \
            -DOLLAMA_PAYLOAD_INSTALL_PREFIX=$INSTALL_PREFIX \
            -DOLLAMA_GO_OUTPUT=$INSTALL_PREFIX/ollama \
            -DOLLAMA_VERSION="$VERSION" \
            -DOLLAMA_MLX_BACKENDS="$MLX_BACKENDS" \
            -DOLLAMA_LLAMA_BACKENDS= \
            -DFETCHCONTENT_SOURCE_DIR_LLAMA_CPP=$LLAMA_CPP_SHARED_SRC \
            -DFETCHCONTENT_SOURCE_DIR_MLX=$MLX_SHARED_SRC \
            -DFETCHCONTENT_SOURCE_DIR_MLX-C=$MLX_C_SHARED_SRC \
            $MLX_EXTRA_ARGS

        GOOS=darwin GOARCH=$ARCH CGO_ENABLED=1 CGO_CFLAGS="$MLX_CGO_CFLAGS" CGO_LDFLAGS="$MLX_CGO_LDFLAGS" \
            cmake --build "$BUILD_DIR" --target ollama-local --target ollama-mlx-backends --parallel "$BUILD_JOBS" -- -l "$BUILD_LOAD"
    done
}

_merge_darwin_payload() {
    status "Preparing universal Darwin runtime payload"
    rm -rf dist/darwin/lib
    mkdir -p dist/darwin/lib/ollama

    for ROOT in dist/darwin-amd64/lib/ollama dist/darwin-arm64/lib/ollama; do
        [ -d "$ROOT" ] || continue
        for F in "$ROOT"/*; do
            [ -e "$F" ] || continue
            BASE=$(basename "$F")
            case "$BASE" in
                llama-server|llama-quantize|mlx_*) continue ;;
            esac
            [ -e "dist/darwin/lib/ollama/$BASE" ] || cp -P "$F" dist/darwin/lib/ollama/
        done
    done

    for VARIANT in dist/darwin-arm64/lib/ollama/mlx_metal_v*/; do
        [ -d "$VARIANT" ] || continue
        VNAME=$(basename "$VARIANT")
        DEST=dist/darwin/lib/ollama/$VNAME
        AMD_VARIANT=dist/darwin-amd64/lib/ollama/$VNAME
        [ -d "$AMD_VARIANT" ] || AMD_VARIANT=dist/darwin-amd64/lib/ollama
        mkdir -p "$DEST"

        for LIB in libmlx.dylib libmlxc.dylib; do
            if [ -f "$AMD_VARIANT/$LIB" ] && [ -f "$VARIANT$LIB" ]; then
                lipo -create -output "$DEST/$LIB" "$AMD_VARIANT/$LIB" "$VARIANT$LIB"
            elif [ -f "$VARIANT$LIB" ]; then
                cp "$VARIANT$LIB" "$DEST/"
            elif [ -f "$AMD_VARIANT/$LIB" ]; then
                cp "$AMD_VARIANT/$LIB" "$DEST/"
            fi
        done

        for F in "$VARIANT"*; do
            [ -f "$F" ] && [ ! -L "$F" ] || continue
            case "$(basename "$F")" in
                libmlx.dylib|libmlxc.dylib) continue ;;
            esac
            cp "$F" "$DEST/"
        done
    done
}

# _lipo_arches DEST RELATIVE_PATH combines the per-arch copies of one binary into
# DEST. Unlike upstream this tolerates a single-arch build (./build_darwin.sh -a
# arm64), which is what CI uses - a thin arm64 binary is a valid Mach-O, and
# building the x86_64 half roughly doubles the build time for machines we do not
# target.
_lipo_arches() {
    DEST=$1
    RELPATH=$2
    INPUTS=""
    VERIFY=""
    for ARCH in $ARCHS; do
        F="dist/darwin-$ARCH/$RELPATH"
        [ -f "$F" ] || { echo "missing $F" >&2; exit 1; }
        INPUTS="$INPUTS $F"
        [ "$ARCH" = "amd64" ] && VERIFY="$VERIFY x86_64" || VERIFY="$VERIFY $ARCH"
    done
    lipo -create -output "$DEST" $INPUTS
    chmod +x "$DEST"
    lipo "$DEST" -verify_arch $VERIFY
}

_prepare_darwin_runtime() {
    status "Combining binaries for: $ARCHS"
    mkdir -p dist/darwin
    _lipo_arches dist/darwin/ollama ollama
    _lipo_arches dist/darwin/llama-server lib/ollama/llama-server
    _lipo_arches dist/darwin/llama-quantize lib/ollama/llama-quantize

    _merge_darwin_payload
}

_create_darwin_runtime_tarball() {
    status "Creating universal tarball..."
    rm -f dist/ollama-darwin.tar dist/ollama-darwin.tgz
    tar -cf dist/ollama-darwin.tar --strip-components 2 dist/darwin/ollama dist/darwin/llama-server dist/darwin/llama-quantize
    tar -rf dist/ollama-darwin.tar --strip-components 4 dist/darwin/lib/ollama
    gzip -9vc <dist/ollama-darwin.tar >dist/ollama-darwin.tgz
}

_package_darwin_runtime() {
    _prepare_darwin_runtime
    _create_darwin_runtime_tarball
}

# _codesign signs one file, using a real Developer ID when APPLE_IDENTITY is set
# and falling back to an ad-hoc signature otherwise.
#
# The fallback matters: the updater's macOS download check runs
# SecStaticCodeCheckValidityWithErrors with no Team ID pin, so an ad-hoc
# signature passes it - but an *unsigned* bundle does not. Upstream simply skips
# signing without an identity, which would make every self-update fail
# verification. Ad-hoc signatures carry no timestamp and cannot use the hardened
# runtime, hence the separate flags.
_codesign() {
    IDENTIFIER=$1
    shift
    if [ -n "$APPLE_IDENTITY" ]; then
        codesign -f --timestamp -s "$APPLE_IDENTITY" --identifier "$IDENTIFIER" --options=runtime "$@"
    else
        codesign -f -s - --identifier "$IDENTIFIER" "$@"
    fi
}

_sign_darwin() {
    _prepare_darwin_runtime
    for F in dist/darwin/ollama dist/darwin/llama-server dist/darwin/llama-quantize dist/darwin/lib/ollama/* dist/darwin/lib/ollama/mlx_metal_v*/*; do
        [ -f "$F" ] && [ ! -L "$F" ] || continue
        _codesign "$BUNDLE_ID" "$F"
    done

    if [ -n "$APPLE_IDENTITY" ]; then
        # create a temporary zip for notarization
        TEMP=$(mktemp -u).zip
        ditto -c -k --keepParent dist/darwin/ollama "$TEMP"
        xcrun notarytool submit "$TEMP" --wait --timeout 20m --apple-id $APPLE_ID --password $APPLE_PASSWORD --team-id $APPLE_TEAM_ID
        rm -f "$TEMP"
    fi

    _create_darwin_runtime_tarball
}

_build_macapp() {
    if ! command -v npm &> /dev/null; then
        echo "npm is not installed. Please install Node.js and npm first:"
        echo "   Visit: https://nodejs.org/"
        exit 1
    fi

    if ! command -v tsc &> /dev/null; then
        echo "Installing TypeScript compiler..."
        npm install -g typescript
    fi

    echo "Installing required Go tools..."

    cd app/ui/app
    npm install
    npm run build
    cd ../../..

    APP=dist/$BUNDLE_NAME.app

    # Build the app bundle
    rm -rf "$APP"
    cp -a "./app/darwin/$BUNDLE_NAME.app" "$APP"

    # update the modified date of the app bundle to now
    touch "$APP"

    go clean -cache
    APP_BINS=""
    for ARCH in $ARCHS; do
        GOARCH=$ARCH CGO_ENABLED=1 GOOS=darwin go build -o "dist/darwin-app-$ARCH" -ldflags="-s -w $GO_LDFLAGS_X" ./app/cmd/app
        APP_BINS="$APP_BINS dist/darwin-app-$ARCH"
    done
    mkdir -p "$APP/Contents/MacOS"
    lipo -create -output "$APP/Contents/MacOS/$BUNDLE_NAME" $APP_BINS
    rm -f $APP_BINS

    # Create a mock Squirrel.framework bundle
    mkdir -p "$APP/Contents/Frameworks/Squirrel.framework/Versions/A/Resources/"
    cp -a "$APP/Contents/MacOS/$BUNDLE_NAME" "$APP/Contents/Frameworks/Squirrel.framework/Versions/A/Squirrel"
    ln -s ../Squirrel "$APP/Contents/Frameworks/Squirrel.framework/Versions/A/Resources/ShipIt"
    cp -a ./app/cmd/squirrel/Info.plist "$APP/Contents/Frameworks/Squirrel.framework/Versions/A/Resources/Info.plist"
    ln -s A "$APP/Contents/Frameworks/Squirrel.framework/Versions/Current"
    ln -s Versions/Current/Resources "$APP/Contents/Frameworks/Squirrel.framework/Resources"
    ln -s Versions/Current/Squirrel "$APP/Contents/Frameworks/Squirrel.framework/Squirrel"

    # Update the version in the Info.plist
    plutil -replace CFBundleShortVersionString -string "$VERSION" "$APP/Contents/Info.plist"
    plutil -replace CFBundleVersion -string "$VERSION" "$APP/Contents/Info.plist"

    # Setup the ollama binaries
    mkdir -p "$APP/Contents/Resources"
    [ -d dist/darwin/lib/ollama ] || _merge_darwin_payload
    cp -a dist/darwin/ollama "$APP/Contents/Resources/ollama"
    cp dist/darwin/llama-server "$APP/Contents/Resources/"
    cp dist/darwin/llama-quantize "$APP/Contents/Resources/"
    if [ -d dist/darwin/lib/ollama ]; then
        cp -a dist/darwin/lib/ollama/. "$APP/Contents/Resources/"
    fi
    chmod a+x "$APP/Contents/Resources/ollama"

    # Sign. Unlike upstream this runs even without an APPLE_IDENTITY, falling back
    # to an ad-hoc signature - see _codesign.
    status "Signing $APP"
    _codesign "$BUNDLE_ID" "$APP/Contents/Resources/ollama"
    _codesign "$BUNDLE_ID" "$APP/Contents/Resources/llama-server"
    _codesign "$BUNDLE_ID" "$APP/Contents/Resources/llama-quantize"
    for lib in "$APP"/Contents/Resources/*.so "$APP"/Contents/Resources/*.dylib "$APP"/Contents/Resources/*.metallib "$APP"/Contents/Resources/mlx_metal_v*/*.dylib "$APP"/Contents/Resources/mlx_metal_v*/*.metallib "$APP"/Contents/Resources/mlx_metal_v*/*.so; do
        [ -f "$lib" ] || continue
        _codesign "$BUNDLE_ID" "$lib"
    done
    _codesign "$BUNDLE_ID" --deep "$APP"

    # The updater rejects a bundle whose signature does not validate, so fail the
    # build here rather than shipping something that can never self-update.
    codesign --verify --deep --strict "$APP"

    rm -f "dist/$BUNDLE_NAME-darwin.zip"
    ditto -c -k --norsrc --keepParent "$APP" "dist/$BUNDLE_NAME-darwin.zip"
    (cd "$APP/Contents/Resources/"; tar -cf - ollama llama-server llama-quantize *.so *.dylib *.metallib mlx_metal_v*/ 2>/dev/null) | gzip -9vc > dist/ollama-darwin.tgz

    # Notarize and Staple. Only possible with a real Developer ID; an ad-hoc build
    # stays unnotarized, which means Gatekeeper quarantines the *first* download on
    # each Mac (right-click -> Open once). In-place self-updates unzip without a
    # quarantine flag, so they are unaffected.
    if [ -n "$APPLE_IDENTITY" ]; then
        $(xcrun -f notarytool) submit "dist/$BUNDLE_NAME-darwin.zip" --wait --timeout 20m --apple-id "$APPLE_ID" --password "$APPLE_PASSWORD" --team-id "$APPLE_TEAM_ID"
        rm -f "dist/$BUNDLE_NAME-darwin.zip"
        $(xcrun -f stapler) staple "$APP"
        ditto -c -k --norsrc --keepParent "$APP" "dist/$BUNDLE_NAME-darwin.zip"
    else
        echo "WARNING: ad-hoc signed (no APPLE_IDENTITY). Self-update works; the first install on each Mac needs right-click -> Open."
    fi

    rm -f "dist/$BUNDLE_NAME.dmg"
    (cd dist && ../scripts/create-dmg.sh \
        --volname "${VOL_NAME}" \
        --volicon "../app/darwin/$BUNDLE_NAME.app/Contents/Resources/icon.icns" \
        --background ../app/assets/background.png \
        --window-pos 200 120 \
        --window-size 800 400 \
        --icon-size 128 \
        --icon "$BUNDLE_NAME.app" 200 190 \
        --hide-extension "$BUNDLE_NAME.app" \
        --app-drop-link 600 190 \
        --text-size 12 \
        "$BUNDLE_NAME.dmg" \
        "$BUNDLE_NAME.app" \
    ; )
    rm -f dist/rw*.dmg

    _codesign "$BUNDLE_ID" "dist/$BUNDLE_NAME.dmg"
    if [ -n "$APPLE_IDENTITY" ]; then
        $(xcrun -f notarytool) submit "dist/$BUNDLE_NAME.dmg" --wait --timeout 20m --apple-id "$APPLE_ID" --password "$APPLE_PASSWORD" --team-id "$APPLE_TEAM_ID"
        $(xcrun -f stapler) staple "dist/$BUNDLE_NAME.dmg"
    fi
}

if [ "$#" -eq 0 ]; then
    _build_darwin
    _sign_darwin
    _build_macapp
    exit 0
fi

for CMD in "$@"; do
    case $CMD in
        build) _build_darwin ;;
        package) _package_darwin_runtime ;;
        sign) _sign_darwin ;;
        app) _build_macapp ;;
        *) usage ;;
    esac
done
