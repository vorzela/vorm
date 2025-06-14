#!/bin/bash

# Release script for VORM PostgreSQL Migration Tool
# This script creates a new release with proper versioning and artifacts

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
RELEASE_DIR="./release"
BUILD_DIR="./bin"
VERSION=$1

# Validate input
if [ -z "$VERSION" ]; then
    print_error "Usage: $0 <version>"
    print_info "Example: $0 v1.0.0"
    exit 1
fi

# Validate version format
if [[ ! $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+)?$ ]]; then
    print_error "Invalid version format. Use semantic versioning (e.g., v1.0.0)"
    exit 1
fi

print_info "Creating release $VERSION for VORM PostgreSQL Migration Tool"

# Check if git repository is clean
if ! git diff-index --quiet HEAD --; then
    print_error "Git repository has uncommitted changes"
    print_info "Please commit or stash your changes before creating a release"
    exit 1
fi

# Check if tag already exists
if git rev-parse --verify --quiet "$VERSION" >/dev/null; then
    print_error "Tag $VERSION already exists"
    exit 1
fi

# Create release directory
if [ -d "$RELEASE_DIR" ]; then
    rm -rf "$RELEASE_DIR"
fi
mkdir -p "$RELEASE_DIR"

# Build for all platforms
print_info "Building binaries for all platforms..."
VERSION="$VERSION" ./scripts/build.sh --all

if [ $? -ne 0 ]; then
    print_error "Build failed"
    exit 1
fi

# Create release archives
print_info "Creating release archives..."

PLATFORMS=(
    "linux-amd64"
    "linux-arm64"
    "darwin-amd64"
    "darwin-arm64"
    "windows-amd64"
)

for platform in "${PLATFORMS[@]}"; do
    print_info "Creating archive for $platform..."
    
    # Determine file extension
    if [[ $platform == *"windows"* ]]; then
        binary="vorm-${platform}.exe"
        archive_name="vorm-${VERSION}-${platform}.zip"
    else
        binary="vorm-${platform}"
        archive_name="vorm-${VERSION}-${platform}.tar.gz"
    fi
    
    # Check if binary exists
    if [ ! -f "$BUILD_DIR/$binary" ]; then
        print_warning "Binary $binary not found, skipping..."
        continue
    fi
    
    # Create temporary directory for archive contents
    temp_dir=$(mktemp -d)
    archive_dir="$temp_dir/vorm-${VERSION}"
    mkdir -p "$archive_dir"
    
    # Copy binary
    cp "$BUILD_DIR/$binary" "$archive_dir/"
    if [[ $platform == *"windows"* ]]; then
        mv "$archive_dir/$binary" "$archive_dir/vorm.exe"
    else
        mv "$archive_dir/$binary" "$archive_dir/vorm"
        chmod +x "$archive_dir/vorm"
    fi
    
    # Copy documentation and configs
    cp README.md "$archive_dir/" 2>/dev/null || print_warning "README.md not found"
    cp LICENSE "$archive_dir/" 2>/dev/null || print_warning "LICENSE not found"
    cp CHANGELOG.md "$archive_dir/" 2>/dev/null || print_warning "CHANGELOG.md not found"
    
    # Copy config examples
    mkdir -p "$archive_dir/config"
    cp config/database.yaml "$archive_dir/config/" 2>/dev/null || print_warning "config/database.yaml not found"
    cp config/.env.example "$archive_dir/config/" 2>/dev/null || true
    
    # Create archive
    cd "$temp_dir"
    if [[ $platform == *"windows"* ]]; then
        zip -r "$archive_name" "vorm-${VERSION}/" >/dev/null
    else
        tar -czf "$archive_name" "vorm-${VERSION}/" >/dev/null
    fi
    
    # Move to release directory
    mv "$archive_name" "$RELEASE_DIR/"
    cd - >/dev/null
    
    # Cleanup
    rm -rf "$temp_dir"
    
    # Calculate checksum
    if command -v sha256sum >/dev/null; then
        cd "$RELEASE_DIR"
        sha256sum "$archive_name" >> checksums.txt
        cd - >/dev/null
    fi
    
    print_success "Created $archive_name"
done

# Create release notes
print_info "Creating release notes..."
cat > "$RELEASE_DIR/RELEASE_NOTES.md" << EOF
# VORM $VERSION

## What's New

* Add your release notes here
* Document new features
* Document bug fixes
* Document breaking changes

## Installation

### Download Binary

Download the appropriate binary for your platform from the release assets.

### Linux/macOS
\`\`\`bash
# Download and extract
tar -xzf vorm-$VERSION-linux-amd64.tar.gz
cd vorm-$VERSION

# Install
sudo cp vorm /usr/local/bin/
chmod +x /usr/local/bin/vorm
\`\`\`

### Windows
\`\`\`powershell
# Extract the ZIP file and add vorm.exe to your PATH
\`\`\`

## Verification

Verify the installation:
\`\`\`bash
vorm --version
\`\`\`

## Checksums

See checksums.txt for file verification.

---

**Full Changelog**: https://github.com/vorzela/vorm/compare/previous-version...$VERSION
EOF

# Create or update changelog
print_info "Updating changelog..."
if [ ! -f "CHANGELOG.md" ]; then
    cat > "CHANGELOG.md" << EOF
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [$VERSION] - $(date +%Y-%m-%d)

### Added
- Initial release of VORM PostgreSQL Migration Tool

### Changed

### Deprecated

### Removed

### Fixed

### Security

EOF
else
    # Add new version to existing changelog
    temp_file=$(mktemp)
    echo "# Changelog" > "$temp_file"
    echo "" >> "$temp_file"
    echo "All notable changes to this project will be documented in this file." >> "$temp_file"
    echo "" >> "$temp_file"
    echo "## [$VERSION] - $(date +%Y-%m-%d)" >> "$temp_file"
    echo "" >> "$temp_file"
    echo "### Added" >> "$temp_file"
    echo "- Add your changes here" >> "$temp_file"
    echo "" >> "$temp_file"
    tail -n +5 "CHANGELOG.md" >> "$temp_file"
    mv "$temp_file" "CHANGELOG.md"
fi

# Commit and tag
print_info "Creating git tag..."
git add CHANGELOG.md
git commit -m "chore: prepare release $VERSION" || true
git tag -a "$VERSION" -m "Release $VERSION"

print_success "Release $VERSION created successfully!"
print_info ""
print_info "Release artifacts created in: $RELEASE_DIR"
print_info "Git tag created: $VERSION"
print_info ""
print_info "Next steps:"
print_info "1. Review the release notes in $RELEASE_DIR/RELEASE_NOTES.md"
print_info "2. Push the tag: git push origin $VERSION"
print_info "3. Create a GitHub release with the artifacts"
print_info "4. Update documentation as needed"
print_info ""
print_info "Files in release:"
ls -la "$RELEASE_DIR/"
