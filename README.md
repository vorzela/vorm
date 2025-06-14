# VORM - PostgreSQL Migration Management Tool

[![Go Version](https://img.shields.io/badge/Go-1.19+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/vorzela/vorm)](https://github.com/vorzela/vorm/releases)

VORM is a powerful, production-ready PostgreSQL migration management tool specifically optimized for large-scale applications (8+ million users). It provides Laravel Eloquent-inspired syntax with strict safety requirements, comprehensive logging, and colored terminal output.

## Features

- ✅ **Laravel-inspired** syntax and migration structure
- ✅ **Production safety** with environment protection and typed confirmations
- ✅ **Colored terminal output** with clear success/error indicators
- ✅ **Comprehensive logging** with rotation and compression
- ✅ **Strict naming conventions** (plural table names, alphabetical pivot ordering)
- ✅ **Transaction support** for atomic migrations
- ✅ **Rollback capabilities** with batch tracking
- ✅ **Multi-platform support** (Linux, macOS, Windows)
- ✅ **Environment variable support** with DATABASE_URL parsing
- ✅ **Configuration validation** and connection testing

## Quick Start

### Installation

#### Download Binary

Download the latest release for your platform from [GitHub Releases](https://github.com/vorzela/vorm/releases).

#### Linux/macOS

```bash
# Download and extract
tar -xzf vorm-v1.0.0-linux-amd64.tar.gz
cd vorm-v1.0.0

# Install system-wide
sudo cp vorm /usr/local/bin/
chmod +x /usr/local/bin/vorm
```

#### Windows

```powershell
# Extract the ZIP file and add vorm.exe to your PATH
```

#### From Source

```bash
# Clone repository
git clone https://github.com/vorzela/vorm.git
cd vorm

# Build
./scripts/build.sh

# Install
./scripts/install.sh
```

### Initialize Project

```bash
# Initialize VORM in your project
vorm init

# Edit configuration
nano config/database.yaml

# Set environment variables
nano .env

# Setup database and migrations table
vorm setup
```

### Basic Usage

```bash
# Create a new migration
vorm make:migration create_users_table

# Run pending migrations
vorm migrate

# Check migration status
vorm status

# Rollback last batch
vorm rollback

# Show migration history
vorm history
```

## Configuration

### Database Configuration (`config/database.yaml`)

```yaml
database:
  driver: postgres
  host: localhost
  port: 5432
  name: your_database_name
  username: your_username
  password: your_password
  sslmode: disable
  timezone: UTC
  max_connections: 10
  max_idle_connections: 5
  connection_max_lifetime: 1h

migration:
  table: migrations
  directory: migrations
  lock_timeout: 15m
  transaction_timeout: 30m

logging:
  level: info
  file: logs/vorm.log
  max_size: 100 # MB
  max_backups: 5
  max_age: 30 # days
  compress: true

environment: development

production:
  require_confirmation: true
  disable_destructive_operations: true
```

### Environment Variables (`.env`)

```bash
# Database connection
DATABASE_URL=postgres://username:password@localhost:5432/database_name?sslmode=disable

# Or individual settings
VORM_DB_HOST=localhost
VORM_DB_PORT=5432
VORM_DB_NAME=your_database
VORM_DB_USERNAME=your_username
VORM_DB_PASSWORD=your_password
VORM_ENVIRONMENT=development
```

## Commands

### Migration Operations

```bash
# Run pending migrations
vorm migrate
vorm migrate --step 3          # Run specific number of migrations

# Rollback migrations
vorm rollback                  # Rollback last batch
vorm rollback --step 2         # Rollback specific number of steps
vorm rollback --to 20250614_180302_create_users_table

# Create new migration
vorm make:migration create_products_table
```

### Status and Information

```bash
vorm status                    # Show migration status
vorm list                     # List all migrations
vorm history                  # Show executed migrations
vorm config show             # Show current configuration
vorm config validate         # Validate configuration
```

### Database Operations

```bash
vorm db:create                # Create database
vorm db:drop                  # Drop database (with confirmation)
vorm db:reset                 # Drop and recreate database
```

### Destructive Operations (Production Protected)

```bash
vorm reset                    # Reset all migrations
vorm fresh                    # Drop all tables and re-run migrations
vorm refresh                  # Rollback and re-run all migrations
```

## Migration Structure

VORM follows Laravel-style migration structure with proper naming conventions:

```sql
-- migrations/2025_06_14_180302_create_users_table.sql

-- Migration: create_users_table
-- Created: 2025-06-14 18:03:02

-- +migrate Up
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    name VARCHAR(255),
    email_verified_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- +migrate Down
DROP TABLE IF EXISTS users;
```

### Naming Conventions

- **Tables**: Plural names (e.g., `users`, `products`, `order_items`)
- **Pivot tables**: Alphabetical ordering (e.g., `product_user` not `user_product`)
- **Migrations**: Descriptive names (e.g., `create_users_table`, `add_index_to_products`)

## Production Safety

VORM includes multiple safety features for production environments:

### Environment Protection

- **Destructive operations disabled** in production
- **Typed confirmations** required for dangerous operations
- **Connection validation** before executing migrations

### Confirmation Requirements

```bash
# Production requires typing "DROP" to confirm
vorm db:drop
> Type 'DROP' to confirm: DROP

# Development requires simple confirmation
vorm db:drop
> Are you sure? (y/N): y
```

### Logging

All operations are logged with:

- **Execution time** tracking
- **Error details** and stack traces
- **Migration checksums** for integrity verification
- **Batch tracking** for rollback capabilities

## Development

### Building from Source

```bash
# Clone repository
git clone https://github.com/vorzela/vorm.git
cd vorm

# Install dependencies
go mod tidy

# Build
./scripts/build.sh

# Build for all platforms
./scripts/build.sh --all

# Run tests
go test ./...
```

### Project Structure

```
vorm/
├── cmd/vorm/           # CLI application
├── internal/           # Internal packages
│   ├── config/         # Configuration management
│   ├── console/        # Terminal output and colors
│   ├── database/       # Database operations
│   ├── logger/         # Logging system
│   ├── migration/      # Migration operations
│   └── utils/          # Utility functions
├── pkg/                # Public API
├── config/             # Configuration templates
├── scripts/            # Build and release scripts
└── docs/               # Documentation
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

- **Documentation**: [GitHub Wiki](https://github.com/vorzela/vorm/wiki)
- **Issues**: [GitHub Issues](https://github.com/vorzela/vorm/issues)
- **Discussions**: [GitHub Discussions](https://github.com/vorzela/vorm/discussions)

## Acknowledgments

- Inspired by Laravel Eloquent migrations
- Built with [Cobra](https://github.com/spf13/cobra) CLI framework
- Uses [pgx](https://github.com/jackc/pgx) PostgreSQL driver
