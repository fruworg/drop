#!/bin/bash
# Build script for Drop CLI client
# Usage: ./scripts/build-client.sh [platform] [arch]

set -e

# Default values
PLATFORM=${1:-"all"}
ARCH=${2:-"all"}
OUTPUT_DIR="dist"
VERSION=${VERSION:-"dev"}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Build flags
LDFLAGS="-s -w -X main.version=${VERSION} -X main.commit=$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown') -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)"

echo -e "${GREEN}Building Drop CLI client...${NC}"
echo "Version: ${VERSION}"
echo "Platform: ${PLATFORM}"
echo "Architecture: ${ARCH}"
echo "Output directory: ${OUTPUT_DIR}"
echo ""

# Create output directory
mkdir -p ${OUTPUT_DIR}

# Function to build for a specific platform/arch
build_binary() {
    local os=$1
    local arch=$2
    local output_name="drop-${os}-${arch}"
    
    if [[ "$os" == "windows" ]]; then
        output_name="${output_name}.exe"
    fi
    
    echo -e "${YELLOW}Building ${output_name}...${NC}"
    
    CGO_ENABLED=0 GOOS=${os} GOARCH=${arch} go build \
        -ldflags="${LDFLAGS}" \
        -o "${OUTPUT_DIR}/${output_name}" \
        ./cmd/client
    
    echo -e "${GREEN}✓ Built ${output_name}${NC}"
}

# Build based on parameters
case "${PLATFORM}" in
    "linux")
        if [[ "${ARCH}" == "all" ]]; then
            build_binary "linux" "amd64"
            build_binary "linux" "arm64"
        else
            build_binary "linux" "${ARCH}"
        fi
        ;;
    "darwin"|"macos")
        if [[ "${ARCH}" == "all" ]]; then
            build_binary "darwin" "amd64"
            build_binary "darwin" "arm64"
        else
            build_binary "darwin" "${ARCH}"
        fi
        ;;
    "windows")
        if [[ "${ARCH}" == "all" ]]; then
            build_binary "windows" "amd64"
        else
            build_binary "windows" "${ARCH}"
        fi
        ;;
    "all")
        echo -e "${YELLOW}Building for all platforms...${NC}"
        build_binary "linux" "amd64"
        build_binary "linux" "arm64"
        build_binary "darwin" "amd64"
        build_binary "darwin" "arm64"
        build_binary "windows" "amd64"
        ;;
    *)
        echo -e "${RED}Error: Unknown platform '${PLATFORM}'${NC}"
        echo "Usage: $0 [platform] [arch]"
        echo "Platforms: linux, darwin, windows, all"
        echo "Architectures: amd64, arm64, all"
        exit 1
        ;;
esac

echo ""
echo -e "${GREEN}Build completed! Binaries are in ${OUTPUT_DIR}/${NC}"
ls -la ${OUTPUT_DIR}/

# Create checksums
echo ""
echo -e "${YELLOW}Creating checksums...${NC}"
cd ${OUTPUT_DIR}
if command -v sha256sum >/dev/null 2>&1; then
    sha256sum * > checksums.txt
elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 * > checksums.txt
else
    echo -e "${RED}Warning: No checksum tool found${NC}"
fi
cd ..

echo -e "${GREEN}✓ Checksums created${NC}"
