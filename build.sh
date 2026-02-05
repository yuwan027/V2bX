#!/bin/bash

# V2bX Build Script
# Usage: ./build.sh [all|linux|clean]

set -e

# Project info
APP_NAME="V2bX"
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build tags
BUILD_TAGS="sing xray with_quic with_grpc with_utls with_wireguard with_acme with_gvisor"

# Build flags
LDFLAGS="-X 'github.com/InazumaV/V2bX/cmd.version=${VERSION}' -s -w -buildid="
GOEXPERIMENT="jsonv2"
CGO_ENABLED=0

# Output directory
OUTPUT_DIR="build"
mkdir -p "${OUTPUT_DIR}"

# Download geo data files (always fetch latest)
download_geo_data() {
    echo "Downloading geo data files..."
    for file in geoip geosite; do
        echo "  Downloading ${file}.dat..."
        curl -sL "https://raw.githubusercontent.com/Loyalsoldier/v2ray-rules-dat/release/${file}.dat" \
            -o "${OUTPUT_DIR}/${file}.dat"
    done
    echo "  Geo data updated."
}

# Build function
build() {
    local os=$1
    local arch=$2
    local suffix=""
    local output_name="${APP_NAME}-${os}-${arch}"

    if [ "$os" = "windows" ]; then
        suffix=".exe"
    fi

    echo "Building ${output_name}${suffix}..."

    env CGO_ENABLED=${CGO_ENABLED} \
        GOOS=${os} \
        GOARCH=${arch} \
        GOEXPERIMENT=${GOEXPERIMENT} \
        go build -v -trimpath \
        -tags "${BUILD_TAGS}" \
        -ldflags "${LDFLAGS}" \
        -o "${OUTPUT_DIR}/${output_name}${suffix}" \
        .

    echo "  -> ${OUTPUT_DIR}/${output_name}${suffix}"
}

# Main
echo "========================================"
echo "  ${APP_NAME} Build Script"
echo "  Version: ${VERSION}"
echo "  Commit:  ${COMMIT}"
echo "========================================"

case "${1:-all}" in
    linux|all)
        download_geo_data
        echo ""
        echo "=== Building for Linux ==="
        build linux amd64
        build linux arm64
        ;;
    clean)
        echo "Cleaning build directory..."
        rm -rf "${OUTPUT_DIR}"
        echo "Done."
        exit 0
        ;;
    *)
        echo "Usage: $0 [all|linux|clean]"
        echo ""
        echo "Options:"
        echo "  all    - Build for Linux (amd64, arm64)"
        echo "  linux  - Build for Linux (amd64, arm64)"
        echo "  clean  - Clean build directory"
        exit 1
        ;;
esac

echo ""
echo "========================================"
echo "  Build Complete!"
echo "  Output: ${OUTPUT_DIR}/"
echo "========================================"
ls -lh "${OUTPUT_DIR}/"
