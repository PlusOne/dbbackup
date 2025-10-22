#!/bin/bash
# Cross-platform build script for dbbackup
# Builds binaries for all major platforms and architectures

set -e

# Check prerequisites
if ! command -v go &> /dev/null; then
    echo "❌ Error: Go is not installed or not in PATH"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo "🔧 Using Go version: $GO_VERSION"

# Configuration
APP_NAME="dbbackup"
VERSION="1.1.0"
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S_UTC')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BIN_DIR="bin"

# Build flags
LDFLAGS="-w -s -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.gitCommit=${GIT_COMMIT}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# Platform configurations
# Format: "GOOS/GOARCH:binary_suffix:description"
PLATFORMS=(
    "linux/amd64::Linux 64-bit (Intel/AMD)"
    "linux/arm64::Linux 64-bit (ARM)"
    "linux/arm:_armv7:Linux 32-bit (ARMv7)"
    "darwin/amd64::macOS 64-bit (Intel)"
    "darwin/arm64::macOS 64-bit (Apple Silicon)"
    "windows/amd64:.exe:Windows 64-bit (Intel/AMD)"
    "windows/arm64:.exe:Windows 64-bit (ARM)"
    "freebsd/amd64::FreeBSD 64-bit (Intel/AMD)"
    "openbsd/amd64::OpenBSD 64-bit (Intel/AMD)"
    "netbsd/amd64::NetBSD 64-bit (Intel/AMD)"
)

echo -e "${BOLD}${BLUE}🔨 Cross-Platform Build Script for ${APP_NAME}${NC}"
echo -e "${BOLD}${BLUE}================================================${NC}"
echo -e "Version: ${YELLOW}${VERSION}${NC}"
echo -e "Build Time: ${YELLOW}${BUILD_TIME}${NC}"
echo -e "Git Commit: ${YELLOW}${GIT_COMMIT}${NC}"
echo ""

# Create bin directory
mkdir -p "${BIN_DIR}"

# Clean previous builds
echo -e "${CYAN}🧹 Cleaning previous builds...${NC}"
rm -f "${BIN_DIR}"/*

# Build counter
total_platforms=${#PLATFORMS[@]}
current=0

echo -e "${CYAN}🏗️  Building for ${total_platforms} platforms...${NC}"
echo ""

# Build for each platform
for platform_config in "${PLATFORMS[@]}"; do
    current=$((current + 1))
    
    # Parse platform configuration
    IFS=':' read -r platform suffix description <<< "$platform_config"
    IFS='/' read -r GOOS GOARCH <<< "$platform"
    
    # Generate binary name
    binary_name="${APP_NAME}_${GOOS}_${GOARCH}${suffix}"
    
    echo -e "${YELLOW}[$current/$total_platforms]${NC} Building for ${BOLD}$description${NC} (${platform})"
    
    # Set environment and build
    if env GOOS=$GOOS GOARCH=$GOARCH go build -ldflags "$LDFLAGS" -o "${BIN_DIR}/${binary_name}" . 2>/dev/null; then
        # Get file size
        if [[ "$OSTYPE" == "darwin"* ]]; then
            size=$(stat -f%z "${BIN_DIR}/${binary_name}" 2>/dev/null || echo "0")
        else
            size=$(stat -c%s "${BIN_DIR}/${binary_name}" 2>/dev/null || echo "0")
        fi
        
        # Format size
        if [ $size -gt 1048576 ]; then
            size_mb=$((size / 1048576))
            size_formatted="${size_mb}M"
        elif [ $size -gt 1024 ]; then
            size_kb=$((size / 1024))
            size_formatted="${size_kb}K"
        else
            size_formatted="${size}B"
        fi
        
        echo -e "  ${GREEN}✅ Success${NC} - ${binary_name} (${size_formatted})"
        
        # Test binary validity (quick check)
        if [[ "$GOOS" == "$(go env GOOS)" && "$GOARCH" == "$(go env GOARCH)" ]]; then
            if "${BIN_DIR}/${binary_name}" --help >/dev/null 2>&1; then
                echo -e "  ${GREEN}  ✓ Binary test passed${NC}"
            else
                echo -e "  ${YELLOW}  ⚠ Binary test failed (may still work)${NC}"
            fi
        fi
    else
        echo -e "  ${RED}❌ Failed${NC} - ${binary_name}"
        echo -e "  ${RED}  Error during compilation${NC}"
    fi
done

echo ""
echo -e "${BOLD}${GREEN}🎉 Build completed!${NC}"
echo ""

# Show build results
echo -e "${BOLD}${PURPLE}📦 Build Results:${NC}"
echo -e "${PURPLE}================${NC}"

ls -la "${BIN_DIR}/" | tail -n +2 | while read -r line; do
    filename=$(echo "$line" | awk '{print $9}')
    size=$(echo "$line" | awk '{print $5}')
    
    if [[ "$filename" == *"linux_amd64"* ]]; then
        echo -e "  🐧 $filename (${size} bytes)"
    elif [[ "$filename" == *"linux_arm"* ]]; then
        echo -e "  🤖 $filename (${size} bytes)"
    elif [[ "$filename" == *"darwin"* ]]; then
        echo -e "  🍎 $filename (${size} bytes)"
    elif [[ "$filename" == *"windows"* ]]; then
        echo -e "  🪟 $filename (${size} bytes)"
    elif [[ "$filename" == *"freebsd"* ]]; then
        echo -e "  😈 $filename (${size} bytes)"
    elif [[ "$filename" == *"openbsd"* ]]; then
        echo -e "  🐡 $filename (${size} bytes)"
    elif [[ "$filename" == *"netbsd"* ]]; then
        echo -e "  🐅 $filename (${size} bytes)"
    else
        echo -e "  📦 $filename (${size} bytes)"
    fi
done

echo ""

# Generate README for bin directory
cat > "${BIN_DIR}/README.md" << EOF
# DB Backup Tool - Pre-compiled Binaries

This directory contains pre-compiled binaries for the DB Backup Tool across multiple platforms and architectures.

## Build Information
- **Version**: ${VERSION}
- **Build Time**: ${BUILD_TIME}
- **Git Commit**: ${GIT_COMMIT}

## Recent Updates (v1.1.0)
- ✅ Fixed TUI progress display with line-by-line output
- ✅ Added interactive configuration settings menu
- ✅ Improved menu navigation and responsiveness  
- ✅ Enhanced completion status handling
- ✅ Better CPU detection and optimization
- ✅ Silent mode support for TUI operations

## Available Binaries

### Linux
- \`dbbackup_linux_amd64\` - Linux 64-bit (Intel/AMD)
- \`dbbackup_linux_arm64\` - Linux 64-bit (ARM)
- \`dbbackup_linux_arm_armv7\` - Linux 32-bit (ARMv7)

### macOS
- \`dbbackup_darwin_amd64\` - macOS 64-bit (Intel)
- \`dbbackup_darwin_arm64\` - macOS 64-bit (Apple Silicon)

### Windows
- \`dbbackup_windows_amd64.exe\` - Windows 64-bit (Intel/AMD)
- \`dbbackup_windows_arm64.exe\` - Windows 64-bit (ARM)

### BSD Systems
- \`dbbackup_freebsd_amd64\` - FreeBSD 64-bit
- \`dbbackup_openbsd_amd64\` - OpenBSD 64-bit
- \`dbbackup_netbsd_amd64\` - NetBSD 64-bit

## Usage

1. Download the appropriate binary for your platform
2. Make it executable (Unix-like systems): \`chmod +x dbbackup_*\`
3. Run: \`./dbbackup_* --help\`

## Interactive Mode

Launch the interactive TUI menu for easy configuration and operation:

\`\`\`bash
# Interactive mode with TUI menu
./dbbackup_linux_amd64

# Features:
# - Interactive configuration settings
# - Real-time progress display
# - Operation history and status
# - CPU detection and optimization
\`\`\`

## Command Line Mode

Direct command line usage with line-by-line progress:

\`\`\`bash
# Show CPU information and optimization settings
./dbbackup_linux_amd64 cpu

# Auto-optimize for your hardware
./dbbackup_linux_amd64 backup cluster --auto-detect-cores

# Manual CPU configuration  
./dbbackup_linux_amd64 backup single mydb --jobs 8 --dump-jobs 4

# Line-by-line progress output
./dbbackup_linux_amd64 backup cluster --progress-type line
\`\`\`

## CPU Detection

All binaries include advanced CPU detection capabilities:
- Automatic core detection for optimal parallelism
- Support for different workload types (CPU-intensive, I/O-intensive, balanced)
- Platform-specific optimizations for Linux, macOS, and Windows
- Interactive CPU configuration in TUI mode

## Support

For issues or questions, please refer to the main project documentation.
EOF

echo -e "${BOLD}${CYAN}📄 Generated ${BIN_DIR}/README.md${NC}"
echo ""

# Count successful builds
success_count=$(ls -1 "${BIN_DIR}"/dbbackup_* 2>/dev/null | wc -l)
echo -e "${BOLD}${GREEN}✨ Build Summary:${NC}"
echo -e "  ${GREEN}✅ ${success_count}/${total_platforms} binaries built successfully${NC}"

if [ $success_count -eq $total_platforms ]; then
    echo -e "  ${GREEN}🎉 All binaries are ready for distribution!${NC}"
else
    failed_count=$((total_platforms - success_count))
    echo -e "  ${YELLOW}⚠️  ${failed_count} builds failed${NC}"
fi

# Detect current platform binary
CURRENT_OS=$(uname -s | tr '[:upper:]' '[:lower:]')
CURRENT_ARCH=$(uname -m)

# Map architecture names
case "$CURRENT_ARCH" in
    "x86_64") CURRENT_ARCH="amd64";;
    "aarch64") CURRENT_ARCH="arm64";;
    "armv7l") CURRENT_ARCH="arm_armv7";;
esac

CURRENT_BINARY="${BIN_DIR}/dbbackup_${CURRENT_OS}_${CURRENT_ARCH}"
if [[ "$CURRENT_OS" == "windows" ]]; then
    CURRENT_BINARY="${CURRENT_BINARY}.exe"
fi

echo ""
echo -e "${BOLD}${BLUE}📋 Next Steps:${NC}"
if [[ -f "$CURRENT_BINARY" ]]; then
    echo -e "  1. Test current platform: ${CYAN}${CURRENT_BINARY} --help${NC}"
    echo -e "  2. Interactive mode: ${CYAN}${CURRENT_BINARY}${NC}"
else
    echo -e "  1. Test binary (adjust for your platform): ${CYAN}./bin/dbbackup_*${NC}"
fi
echo -e "  3. Create release: ${CYAN}git tag v${VERSION} && git push --tags${NC}"
echo -e "  4. Archive builds: ${CYAN}tar -czf dbbackup-v${VERSION}-all-platforms.tar.gz bin/${NC}"
echo ""