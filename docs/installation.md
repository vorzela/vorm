# Installation Guide

This guide covers various methods to install VORM on your system.

## System Requirements

- **Go**: 1.19 or higher (for building from source)
- **PostgreSQL**: 12 or higher
- **Operating System**: Linux, macOS, or Windows

## Installation Methods

### Method 1: Download Binary (Recommended)

Download the latest release from [GitHub Releases](https://github.com/vorzela/vorm/releases).

#### Linux (x64)
```bash
# Download
curl -L https://github.com/vorzela/vorm/releases/latest/download/vorm-v1.0.0-linux-amd64.tar.gz -o vorm.tar.gz

# Extract
tar -xzf vorm.tar.gz
cd vorm-v1.0.0

# Install
sudo cp vorm /usr/local/bin/
sudo chmod +x /usr/local/bin/vorm

# Verify
vorm --version
```

#### Linux (ARM64)
```bash
# Download ARM64 version
curl -L https://github.com/vorzela/vorm/releases/latest/download/vorm-v1.0.0-linux-arm64.tar.gz -o vorm.tar.gz

# Extract and install (same as above)
tar -xzf vorm.tar.gz
cd vorm-v1.0.0
sudo cp vorm /usr/local/bin/
sudo chmod +x /usr/local/bin/vorm
```

#### macOS (Intel)
```bash
# Download
curl -L https://github.com/vorzela/vorm/releases/latest/download/vorm-v1.0.0-darwin-amd64.tar.gz -o vorm.tar.gz

# Extract and install
tar -xzf vorm.tar.gz
cd vorm-v1.0.0
sudo cp vorm /usr/local/bin/
sudo chmod +x /usr/local/bin/vorm
```

#### macOS (Apple Silicon)
```bash
# Download ARM64 version for Apple Silicon
curl -L https://github.com/vorzela/vorm/releases/latest/download/vorm-v1.0.0-darwin-arm64.tar.gz -o vorm.tar.gz

# Extract and install
tar -xzf vorm.tar.gz
cd vorm-v1.0.0
sudo cp vorm /usr/local/bin/
sudo chmod +x /usr/local/bin/vorm
```

#### Windows
```powershell
# Download the Windows ZIP file
# Extract vorm.exe from the ZIP
# Add the directory containing vorm.exe to your PATH
# Or copy vorm.exe to a directory already in PATH
```

### Method 2: Build from Source

```bash
# Clone repository
git clone https://github.com/vorzela/vorm.git
cd vorm

# Install dependencies
go mod tidy

# Build
./scripts/build.sh

# Install
./scripts/install.sh
```

### Method 3: Using the Install Script

```bash
# Clone and install in one step
git clone https://github.com/vorzela/vorm.git
cd vorm
./scripts/build.sh
./scripts/install.sh
```

## Verification

After installation, verify VORM is working:

```bash
# Check version
vorm --version

# Check help
vorm --help

# Test basic functionality
vorm init
```

## Uninstallation

To remove VORM:

```bash
# Remove binary
sudo rm /usr/local/bin/vorm

# Remove configuration (optional)
rm -rf ~/.vorm
```

## Troubleshooting

### Permission Denied
If you get permission denied errors:
```bash
sudo chmod +x /usr/local/bin/vorm
```

### Command Not Found
If `vorm` command is not found:
1. Check if `/usr/local/bin` is in your PATH
2. Add it to your shell profile:
   ```bash
   echo 'export PATH="/usr/local/bin:$PATH"' >> ~/.bashrc
   source ~/.bashrc
   ```

### macOS Security Warning
On macOS, you might see a security warning. To resolve:
1. Open System Preferences → Security & Privacy
2. Click "Allow Anyway" for vorm
3. Or run: `sudo xattr -d com.apple.quarantine /usr/local/bin/vorm`

## Next Steps

After installation:
1. Initialize your project with `vorm init`
2. Configure your database settings
3. Read the [Configuration Guide](configuration.md)
