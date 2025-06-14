#!/bin/bash

# Installation script for VORM PostgreSQL Migration Tool
# This script installs VORM to the system

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

# Configuration
BINARY_NAME="vorm"
INSTALL_DIR="/usr/local/bin"
BUILD_DIR="./bin"
CONFIG_DIR="$HOME/.vorm"

print_info "Installing VORM PostgreSQL Migration Tool"

# Check if binary exists
if [ ! -f "$BUILD_DIR/$BINARY_NAME" ]; then
    print_error "Binary not found at $BUILD_DIR/$BINARY_NAME"
    print_info "Please run './scripts/build.sh' first"
    exit 1
fi

# Check if we have sudo access for system installation
if [ ! -w "$INSTALL_DIR" ]; then
    print_info "Installing to system directory requires sudo privileges"
    NEEDS_SUDO=true
else
    NEEDS_SUDO=false
fi

# Install binary
print_info "Installing binary to $INSTALL_DIR..."
if [ "$NEEDS_SUDO" = true ]; then
    sudo cp "$BUILD_DIR/$BINARY_NAME" "$INSTALL_DIR/"
    sudo chmod +x "$INSTALL_DIR/$BINARY_NAME"
else
    cp "$BUILD_DIR/$BINARY_NAME" "$INSTALL_DIR/"
    chmod +x "$INSTALL_DIR/$BINARY_NAME"
fi

if [ $? -eq 0 ]; then
    print_success "Binary installed successfully"
else
    print_error "Failed to install binary"
    exit 1
fi

# Create user configuration directory
if [ ! -d "$CONFIG_DIR" ]; then
    mkdir -p "$CONFIG_DIR"
    print_info "Created configuration directory: $CONFIG_DIR"
fi

# Copy example configuration files
if [ -f "./config/database.yaml" ]; then
    if [ ! -f "$CONFIG_DIR/database.yaml" ]; then
        cp "./config/database.yaml" "$CONFIG_DIR/"
        print_info "Copied example database configuration"
    else
        print_warning "Configuration file already exists at $CONFIG_DIR/database.yaml"
    fi
fi

if [ -f "./config/.env.example" ]; then
    if [ ! -f "$CONFIG_DIR/.env.example" ]; then
        cp "./config/.env.example" "$CONFIG_DIR/"
        print_info "Copied example environment configuration"
    fi
fi

# Test installation
print_info "Testing installation..."
if command -v vorm >/dev/null 2>&1; then
    INSTALLED_VERSION=$(vorm --version 2>/dev/null | head -n1)
    print_success "Installation test passed"
    print_info "Installed version: $INSTALLED_VERSION"
else
    print_error "Installation test failed - command not found"
    print_info "You may need to add $INSTALL_DIR to your PATH"
    exit 1
fi

print_success "Installation completed successfully"
print_info ""
print_info "Next steps:"
print_info "1. Copy and modify the configuration file: $CONFIG_DIR/database.yaml"
print_info "2. Set up your environment variables (see $CONFIG_DIR/.env.example)"
print_info "3. Initialize your project with: vorm init"
print_info "4. Create your first migration with: vorm make:migration create_users_table"
