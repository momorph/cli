#!/bin/bash
#
# MoMorph CLI Installer
# Usage: curl -fsSL https://momorph.ai/cli/stable/install.sh | bash
#
# Environment variables:
#   VERSION           - Specific version to install (default: latest)
#   INSTALL_DIR       - Installation directory (default: /usr/local/bin)
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Configuration
GITHUB_REPO="momorph/cli"
BINARY_NAME="momorph"
DEFAULT_INSTALL_DIR="/usr/local/bin"

# Print colored message
print_info() {
    printf "${BLUE}[INFO]${NC} %s\n" "$1"
}

print_success() {
    printf "${GREEN}[SUCCESS]${NC} %s\n" "$1"
}

print_warning() {
    printf "${YELLOW}[WARNING]${NC} %s\n" "$1"
}

print_error() {
    printf "${RED}[ERROR]${NC} %s\n" "$1" >&2
}

# Detect OS
detect_os() {
    local os
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    case "$os" in
        linux*)  echo "linux" ;;
        darwin*) echo "darwin" ;;
        mingw*|msys*|cygwin*)
            print_error "Windows detected. Please use Chocolatey instead:"
            print_error "  choco install momorph-cli"
            exit 1
            ;;
        *)
            print_error "Unsupported operating system: $os"
            exit 1
            ;;
    esac
}

# Detect architecture
detect_arch() {
    local arch
    arch=$(uname -m)
    case "$arch" in
        x86_64|amd64)  echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        i386|i686)     echo "386" ;;
        *)
            print_error "Unsupported architecture: $arch"
            exit 1
            ;;
    esac
}

# Get latest version from GitHub API
get_latest_version() {
    local version
    version=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$version" ]; then
        print_error "Failed to fetch latest version"
        exit 1
    fi
    echo "$version"
}

# Get current installed version
get_current_version() {
    if command -v momorph &> /dev/null; then
        local ver
        ver=$(momorph version 2>/dev/null | grep -i "version" | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "")
        if [ -n "$ver" ]; then
            echo "v${ver}"
        fi
    fi
}

# Check if installed via package manager
detect_package_manager() {
    local momorph_path
    momorph_path=$(which momorph 2>/dev/null || echo "")

    if [ -z "$momorph_path" ]; then
        echo "none"
        return
    fi

    case "$momorph_path" in
        /opt/homebrew/*)
            echo "homebrew"
            ;;
        /usr/local/Cellar/*|/home/linuxbrew/*)
            echo "homebrew"
            ;;
        *)
            echo "manual"
            ;;
    esac
}

# Download and verify checksum
download_and_verify() {
    local version="$1"
    local os="$2"
    local arch="$3"
    local install_dir="$4"
    local tmp_dir

    tmp_dir=$(mktemp -d)
    trap "rm -rf $tmp_dir" EXIT

    # Remove 'v' prefix for filename
    local version_num="${version#v}"
    local filename="momorph-cli_${version_num}_${os}_${arch}.tar.gz"
    local download_url="https://github.com/${GITHUB_REPO}/releases/download/${version}/${filename}"
    local checksums_url="https://github.com/${GITHUB_REPO}/releases/download/${version}/checksums.txt"

    print_info "Downloading MoMorph CLI ${version} for ${os}/${arch}..."

    # Download binary
    if ! curl -fsSL "$download_url" -o "${tmp_dir}/${filename}"; then
        print_error "Failed to download: $download_url"
        exit 1
    fi

    # Download checksums
    if ! curl -fsSL "$checksums_url" -o "${tmp_dir}/checksums.txt"; then
        print_error "Failed to download checksums"
        exit 1
    fi

    # Verify checksum
    print_info "Verifying checksum..."
    local expected_checksum
    expected_checksum=$(grep "${filename}" "${tmp_dir}/checksums.txt" | awk '{print $1}')

    if [ -z "$expected_checksum" ]; then
        print_error "Checksum not found for ${filename}"
        exit 1
    fi

    local actual_checksum
    if command -v sha256sum &> /dev/null; then
        actual_checksum=$(sha256sum "${tmp_dir}/${filename}" | awk '{print $1}')
    elif command -v shasum &> /dev/null; then
        actual_checksum=$(shasum -a 256 "${tmp_dir}/${filename}" | awk '{print $1}')
    else
        print_warning "sha256sum/shasum not found, skipping checksum verification"
        actual_checksum="$expected_checksum"
    fi

    if [ "$expected_checksum" != "$actual_checksum" ]; then
        print_error "Checksum verification failed!"
        print_error "Expected: $expected_checksum"
        print_error "Actual:   $actual_checksum"
        exit 1
    fi

    print_success "Checksum verified"

    # Extract binary
    print_info "Extracting..."
    tar -xzf "${tmp_dir}/${filename}" -C "${tmp_dir}"

    # Install binary
    print_info "Installing to ${install_dir}..."

    if [ ! -d "$install_dir" ]; then
        print_info "Creating directory ${install_dir}..."
        if [ -w "$(dirname "$install_dir")" ]; then
            mkdir -p "$install_dir"
        else
            sudo mkdir -p "$install_dir"
        fi
    fi

    if [ -w "$install_dir" ]; then
        mv "${tmp_dir}/${BINARY_NAME}" "${install_dir}/${BINARY_NAME}"
        chmod +x "${install_dir}/${BINARY_NAME}"
    else
        print_info "Requesting sudo permission to install..."
        sudo mv "${tmp_dir}/${BINARY_NAME}" "${install_dir}/${BINARY_NAME}"
        sudo chmod +x "${install_dir}/${BINARY_NAME}"
    fi
}

# Verify installation
verify_installation() {
    local install_dir="$1"
    local expected_version="$2"

    # Check the installed binary directly
    if [ -x "${install_dir}/${BINARY_NAME}" ]; then
        local installed_version ver
        ver=$("${install_dir}/${BINARY_NAME}" version 2>/dev/null | grep -i "version" | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "")
        if [ -n "$ver" ]; then
            installed_version="v${ver}"
        else
            installed_version="unknown"
        fi

        print_success "MoMorph CLI installed successfully!"
        print_info "Version: ${installed_version}"
        print_info "Location: ${install_dir}/${BINARY_NAME}"

        # Check PATH priority
        local which_path
        which_path=$(which momorph 2>/dev/null || echo "")

        if [ "$which_path" != "${install_dir}/${BINARY_NAME}" ] && [ -n "$which_path" ]; then
            echo ""
            print_warning "Another momorph binary has higher PATH priority:"
            print_warning "  Active: ${which_path}"
            print_warning "  Installed: ${install_dir}/${BINARY_NAME}"
            echo ""
            print_info "To use the newly installed version, either:"
            echo "  1. Uninstall the other version (e.g., 'brew uninstall momorph-cli')"
            echo "  2. Add ${install_dir} to the beginning of your PATH:"
            echo "     export PATH=\"${install_dir}:\$PATH\""
            echo "  3. Run directly: ${install_dir}/${BINARY_NAME}"
        fi

        echo ""
        print_info "Get started with:"
        echo "  momorph login     # Authenticate with GitHub"
        echo "  momorph init .    # Initialize a MoMorph project"
        echo "  momorph --help    # Show help"
    else
        print_error "Installation verification failed"
        exit 1
    fi
}

# Prompt for confirmation
confirm() {
    local prompt="$1"
    local default="${2:-y}"
    local response

    if [ "$default" = "y" ]; then
        prompt="${prompt} [Y/n] "
    else
        prompt="${prompt} [y/N] "
    fi

    # Check if running interactively
    if [ -t 0 ]; then
        printf "${CYAN}${prompt}${NC}"
        read -r response
        response=${response:-$default}
    else
        # Non-interactive, use default
        response=$default
    fi

    case "$response" in
        [yY][eE][sS]|[yY]) return 0 ;;
        *) return 1 ;;
    esac
}

# Main
main() {
    echo ""
    printf "${BLUE}╔════════════════════════════════════════╗${NC}\n"
    printf "${BLUE}║${NC}       ${BOLD}MoMorph CLI Installer${NC}            ${BLUE}║${NC}\n"
    printf "${BLUE}╚════════════════════════════════════════╝${NC}\n"
    echo ""

    # Check for required commands
    for cmd in curl tar; do
        if ! command -v "$cmd" &> /dev/null; then
            print_error "Required command not found: $cmd"
            exit 1
        fi
    done

    # Detect platform
    local os arch target_version current_version pkg_manager install_dir
    os=$(detect_os)
    arch=$(detect_arch)

    print_info "Detected platform: ${os}/${arch}"

    # Get target version
    if [ -n "$VERSION" ]; then
        target_version="$VERSION"
        # Ensure version starts with 'v'
        [[ "$target_version" != v* ]] && target_version="v$target_version"
        print_info "Target version: ${target_version}"
    else
        print_info "Fetching latest version..."
        target_version=$(get_latest_version)
        print_info "Latest version: ${target_version}"
    fi

    # Get current version
    current_version=$(get_current_version)
    if [ -n "$current_version" ]; then
        print_info "Current version: ${current_version}"
    fi

    # Check if already up to date
    if [ "$current_version" = "$target_version" ]; then
        print_success "MoMorph CLI is already up to date"
        exit 0
    fi

    # Detect package manager installation
    pkg_manager=$(detect_package_manager)

    # Determine install directory
    install_dir="${INSTALL_DIR:-$DEFAULT_INSTALL_DIR}"

    if [ "$pkg_manager" = "homebrew" ]; then
        echo ""
        print_warning "MoMorph CLI ${current_version} is currently installed via Homebrew."
        print_warning "Installing via this script will create a separate installation."
        echo ""
        print_info "Recommended: Update via Homebrew instead:"
        echo "  brew upgrade momorph-cli"
        echo ""

        if ! confirm "Do you want to continue with manual installation anyway?"; then
            print_info "Installation cancelled."
            exit 0
        fi

        echo ""
        print_info "To use the manually installed version after installation,"
        print_info "you may need to uninstall the Homebrew version:"
        echo "  brew uninstall momorph-cli"
        echo ""
    elif [ -n "$current_version" ]; then
        echo ""
        if ! confirm "Upgrade MoMorph CLI from ${current_version} to ${target_version}?"; then
            print_info "Installation cancelled."
            exit 0
        fi
        echo ""
    else
        echo ""
        if ! confirm "Install MoMorph CLI ${target_version}?"; then
            print_info "Installation cancelled."
            exit 0
        fi
        echo ""
    fi

    # Download and install
    download_and_verify "$target_version" "$os" "$arch" "$install_dir"

    # Verify
    verify_installation "$install_dir" "$target_version"
}

main "$@"
