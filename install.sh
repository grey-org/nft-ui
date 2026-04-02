#!/bin/bash
set -e

REPO="grey-org/nft-ui"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="nft-ui"
BETA_MODE=false
SPECIFIC_TAG=""
GITHUB_PROXY="${GITHUB_PROXY:-}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() { echo -e "${GREEN}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

usage() {
    echo "Usage: $0 [--beta] [--tag <version>] [--github-proxy <url>]"
}

normalize_proxy() {
    local proxy="$1"

    case "$proxy" in
        http://*|https://*) ;;
        *) error "GitHub proxy must start with http:// or https://" ;;
    esac

    printf '%s/' "${proxy%/}"
}

build_url() {
    local url="$1"

    if [ -n "$GITHUB_PROXY" ]; then
        printf '%s%s' "$GITHUB_PROXY" "$url"
    else
        printf '%s' "$url"
    fi
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --beta)
            BETA_MODE=true
            shift
            ;;
        --tag|--version)
            if [ -z "${2:-}" ]; then
                error "Option $1 requires a value"
            fi
            SPECIFIC_TAG="$2"
            shift 2
            ;;
        --github-proxy|--proxy)
            if [ -z "${2:-}" ]; then
                error "Option $1 requires a value"
            fi
            GITHUB_PROXY="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

if [ -n "$GITHUB_PROXY" ]; then
    GITHUB_PROXY=$(normalize_proxy "$GITHUB_PROXY")
fi

INSTALL_BRANCH="main"
if [ "$BETA_MODE" = true ]; then
    INSTALL_BRANCH="dev"
fi

INSTALL_SCRIPT_URL=$(build_url "https://raw.githubusercontent.com/${REPO}/${INSTALL_BRANCH}/install.sh")
ROOT_CMD="curl -fsSL ${INSTALL_SCRIPT_URL} | sudo bash"

if [ "$BETA_MODE" = true ] || [ -n "$SPECIFIC_TAG" ] || [ -n "$GITHUB_PROXY" ]; then
    ROOT_CMD="${ROOT_CMD} -s --"
    if [ "$BETA_MODE" = true ]; then
        ROOT_CMD="${ROOT_CMD} --beta"
    fi
    if [ -n "$SPECIFIC_TAG" ]; then
        ROOT_CMD="${ROOT_CMD} --tag ${SPECIFIC_TAG}"
    fi
    if [ -n "$GITHUB_PROXY" ]; then
        ROOT_CMD="${ROOT_CMD} --github-proxy ${GITHUB_PROXY}"
    fi
fi

# Check root
if [ "$EUID" -ne 0 ]; then
    error "Please run as root: ${ROOT_CMD}"
fi

# Detect architecture
ARCH=$(uname -m)
case $ARCH in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    *)       error "Unsupported architecture: $ARCH" ;;
esac

info "Detected architecture: $ARCH"

# Check for jq (needed for beta mode)
if [ "$BETA_MODE" = true ] && ! command -v jq &> /dev/null; then
    error "jq is required for beta mode. Install it with: apt install jq"
fi

if [ -n "$GITHUB_PROXY" ]; then
    info "Using GitHub proxy: $GITHUB_PROXY"
fi

# Get release version
if [ -n "$SPECIFIC_TAG" ]; then
    # Use specified tag
    LATEST="$SPECIFIC_TAG"
    info "Using specified version: $LATEST"
    
    # Verify tag exists
    if ! curl -fsSL "$(build_url "https://api.github.com/repos/${REPO}/releases/tags/${LATEST}")" | grep -q '"tag_name"'; then
        error "Tag '${LATEST}' not found in repository"
    fi
elif [ "$BETA_MODE" = true ]; then
    info "Fetching latest beta/pre-release..."
    LATEST=$(curl -fsSL "$(build_url "https://api.github.com/repos/${REPO}/releases")" | jq -r '[.[] | select(.prerelease==true)][0].tag_name')
    if [ -z "$LATEST" ] || [ "$LATEST" = "null" ]; then
        error "No pre-release found. Try without --beta flag for stable version."
    fi
else
    info "Fetching latest stable release..."
    LATEST=$(curl -fsSL "$(build_url "https://api.github.com/repos/${REPO}/releases/latest")" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
fi

if [ -z "$LATEST" ]; then
    error "Failed to get latest release"
fi

info "Latest version: $LATEST"

# Download binary
DOWNLOAD_URL=$(build_url "https://github.com/${REPO}/releases/download/${LATEST}/${BINARY_NAME}-linux-${ARCH}")
info "Downloading from: $DOWNLOAD_URL"

curl -fsSL -o "/tmp/${BINARY_NAME}" "$DOWNLOAD_URL" || error "Download failed"

# Install
info "Installing to ${INSTALL_DIR}/${BINARY_NAME}..."
mv "/tmp/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

# Verify
if command -v "$BINARY_NAME" &> /dev/null; then
    # Create data directory for ruleset persistence
    mkdir -p /var/lib/nft-ui

    info "Installation successful!"
    echo ""
    echo "Usage:"
    echo "  $BINARY_NAME                    # Run with default settings"
    echo "  $BINARY_NAME -h                 # Show help"
    echo ""
    echo "Configuration (environment variables):"
    echo "  NFT_UI_LISTEN_ADDR=:8080        # Listen address"
    echo "  NFT_UI_AUTH_USER=admin          # Basic auth username"
    echo "  NFT_UI_AUTH_PASSWORD=secret     # Basic auth password"
    echo "  NFT_UI_READ_ONLY=false          # Read-only mode"
    echo ""
else
    error "Installation failed"
fi
