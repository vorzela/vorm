# VORM v1.0.0

## 🎉 Initial Release

This is the first stable release of VORM, a powerful PostgreSQL migration management tool optimized for large-scale applications.

## ✨ What's New

* **Complete Migration Management** - Full Laravel Eloquent-inspired migration system
* **Production Safety** - Environment protection and typed confirmations for destructive operations
* **Multi-Platform Support** - Binaries for Linux (AMD64/ARM64), macOS (Intel/Apple Silicon), and Windows
* **Colored Terminal Output** - Beautiful, easy-to-read command line interface
* **Comprehensive Logging** - Detailed logging with rotation and compression
* **Environment Variables** - Flexible configuration with VORM_ prefixed variables
* **Database Operations** - Create, drop, reset databases with safety checks
* **Migration Operations** - Generate, run, rollback migrations with batch tracking
* **Configuration Management** - YAML-based configuration with validation

## 🚀 Installation

### Download Binary

Download the appropriate binary for your platform from the release assets.

### Linux/macOS
```bash
# Download and extract
tar -xzf vorm-v1.0.0-linux-amd64.tar.gz
cd vorm-v1.0.0

# Install
sudo cp vorm /usr/local/bin/
chmod +x /usr/local/bin/vorm
```

### Windows
```bash
# Extract the archive and add vorm.exe to your PATH
tar -xzf vorm-v1.0.0-windows-amd64.tar.gz
```

## 🔧 Quick Start

```bash
# Initialize project
vorm init

# Edit configuration
nano config/database.yaml

# Setup database
vorm setup

# Create migration
vorm make:migration create_users_table

# Run migrations
vorm migrate

# Check status
vorm status
```

## 📋 Available Commands

- `vorm init` - Initialize migration system
- `vorm setup` - Setup database and migrations table
- `vorm make:migration <name>` - Create new migration
- `vorm migrate` - Run pending migrations
- `vorm rollback` - Rollback migrations
- `vorm status` - Show migration status
- `vorm list` - List all migrations
- `vorm history` - Show migration history
- `vorm db:create` - Create database
- `vorm db:drop` - Drop database
- `vorm db:reset` - Reset database
- `vorm config:show` - Show configuration
- `vorm config:validate` - Validate configuration

## 🔒 Production Safety

- Environment protection prevents destructive operations in production
- Typed confirmations required for dangerous operations
- Connection validation before migration execution
- Transaction support for atomic operations
- Migration checksums for integrity verification

## 📦 Platform Support

- **Linux** - AMD64, ARM64
- **macOS** - Intel (AMD64), Apple Silicon (ARM64)  
- **Windows** - AMD64

## 🔍 Verification

Verify the installation:
```bash
vorm --version
```

## 🔐 Checksums

See `checksums.txt` for file verification.

## 📖 Documentation

- [Installation Guide](docs/installation.md)
- [Development Guide](docs/DEVELOPMENT.md)
- [Commands Reference](docs/commands.md)
- [Contributing](CONTRIBUTING.md)

---

**Full Changelog**: Initial release

## 🙏 Acknowledgments

- Inspired by Laravel Eloquent migrations
- Built with [Cobra](https://github.com/spf13/cobra) CLI framework
- Uses [pgx](https://github.com/jackc/pgx) PostgreSQL driver
