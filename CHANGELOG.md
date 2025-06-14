# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Initial release of VORM PostgreSQL Migration Tool

## [1.0.0] - 2025-06-14

### Added

- **Core Features**
  - Laravel Eloquent-inspired migration management
  - PostgreSQL-specific optimizations for large-scale applications
  - Support for 8+ million user databases
- **CLI Commands**

  - `vorm init` - Initialize migration system with config files
  - `vorm setup` - Setup database and migrations table
  - `vorm make:migration` - Create new migration files
  - `vorm migrate` - Run pending migrations
  - `vorm rollback` - Rollback migrations with batch support
  - `vorm status` - Show migration status
  - `vorm list` - List all migrations
  - `vorm history` - Show migration execution history
  - Database operations: `db:create`, `db:drop`, `db:reset`
  - Configuration commands: `config:show`, `config:validate`
  - Destructive operations: `reset`, `fresh`, `refresh`

- **Safety Features**

  - Production environment protection
  - Typed confirmations for destructive operations
  - Connection validation before migration execution
  - Transaction support for atomic operations
  - Migration checksums for integrity verification

- **Output and Logging**

  - Colored terminal output (green=success, red=error, yellow=warning, blue=info)
  - Comprehensive logging with rotation and compression
  - Execution time tracking
  - Detailed error reporting

- **Configuration System**

  - YAML-based configuration with validation
  - Environment variable support with DATABASE_URL parsing
  - Multi-environment support (development/production)
  - Flexible database connection settings

- **Migration Features**

  - Laravel-style migration templates
  - Proper naming conventions (plural tables, alphabetical pivot ordering)
  - Automatic timestamp generation
  - Up/Down migration support
  - Batch tracking for rollback capabilities

- **Build and Release**
  - Multi-platform build support (Linux, macOS, Windows)
  - Automated release scripts
  - Proper version embedding
  - Installation scripts

### Security

- Production environment checks for destructive operations
- Input validation for all commands
- Safe database connection handling

---

**Full Changelog**: https://github.com/vorzela/vorm/releases
