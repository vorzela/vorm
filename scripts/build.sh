#!/bin/bash

# Build script for VORM PostgreSQL Migration Tool
# This script builds the VORM binary with proper versioning information

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
print_info() {
    echo -e "${BLUE}ℹ ${1}${NC}"
}

print_success() {
    echo -e "${GREEN}✓ ${1}${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ ${1}${NC}"
}

print_error() {
    echo -e "${RED}✗ ${1}${NC}"
}

# Get version information
VERSION=${VERSION:-"v1.0.0"}
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(date -u '+%Y-%m-%d_%H:%M:%S')
GO_VERSION=$(go version | awk '{print $3}')

# Build information
BUILD_DIR="./bin"
BINARY_NAME="vorm"
MAIN_PACKAGE="./cmd/vorm"

print_info "Building VORM PostgreSQL Migration Tool"
print_info "Version: $VERSION"
print_info "Commit: $COMMIT"
print_info "Date: $DATE"
print_info "Go Version: $GO_VERSION"

# Create build directory
if [ ! -d "$BUILD_DIR" ]; then
    mkdir -p "$BUILD_DIR"
    print_info "Created build directory: $BUILD_DIR"
fi

# Build flags
LDFLAGS="-s -w"
LDFLAGS="$LDFLAGS -X main.version=$VERSION"
LDFLAGS="$LDFLAGS -X main.commit=$COMMIT"
LDFLAGS="$LDFLAGS -X main.date=$DATE"

# Clean previous builds
if [ -f "$BUILD_DIR/$BINARY_NAME" ]; then
    rm "$BUILD_DIR/$BINARY_NAME"
    print_info "Cleaned previous build"
fi

# Build the binary
print_info "Building binary..."
go build -ldflags "$LDFLAGS" -o "$BUILD_DIR/$BINARY_NAME" "$MAIN_PACKAGE"

if [ $? -eq 0 ]; then
    print_success "Build completed successfully"
    print_info "Binary location: $BUILD_DIR/$BINARY_NAME"
    
    # Make binary executable
    chmod +x "$BUILD_DIR/$BINARY_NAME"
    
    # Show binary info
    BINARY_SIZE=$(du -h "$BUILD_DIR/$BINARY_NAME" | cut -f1)
    print_info "Binary size: $BINARY_SIZE"
    
    # Test the binary
    print_info "Testing binary..."
    if "$BUILD_DIR/$BINARY_NAME" --version >/dev/null 2>&1; then
        print_success "Binary test passed"
    else
        print_warning "Binary test failed"
    fi
else
    print_error "Build failed"
    exit 1
fi

print_success "Build script completed"

# Build for multiple platforms if requested
if [ "$1" = "--all" ]; then
    print_info "Building for multiple platforms..."
    
    # Define platforms
    PLATFORMS=(
        "linux/amd64"
        "linux/arm64"
        "darwin/amd64"
        "darwin/arm64"
        "windows/amd64"
    )
    
    for platform in "${PLATFORMS[@]}"; do
        IFS='/' read -r -a platform_split <<< "$platform"
        GOOS="${platform_split[0]}"
        GOARCH="${platform_split[1]}"
        
        output_name="$BUILD_DIR/${BINARY_NAME}-${GOOS}-${GOARCH}"
        if [ "$GOOS" = "windows" ]; then
            output_name+=".exe"
        fi
        
        print_info "Building for $GOOS/$GOARCH..."
        env GOOS="$GOOS" GOARCH="$GOARCH" go build -ldflags "$LDFLAGS" -o "$output_name" "$MAIN_PACKAGE"
        
        if [ $? -eq 0 ]; then
            platform_size=$(du -h "$output_name" | cut -f1)
            print_success "Built $output_name ($platform_size)"
        else
            print_error "Failed to build for $GOOS/$GOARCH"
        fi
    done
    
    print_success "Multi-platform build completed"
fi
