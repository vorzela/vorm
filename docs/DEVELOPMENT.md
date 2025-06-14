# VORM Developer Guide

This guide contains comprehensive information for developers who want to contribute to or understand the VORM PostgreSQL Migration Management Tool.

## Table of Contents

- [Development Setup](#development-setup)
- [Project Architecture](#project-architecture)
- [Code Style Guidelines](#code-style-guidelines)
- [Building and Testing](#building-and-testing)
- [Contributing](#contributing)
- [Release Process](#release-process)

## Development Setup

### Prerequisites

- **Go 1.19+** - [Install Go](https://golang.org/doc/install)
- **PostgreSQL 12+** - For testing database operations
- **Git** - For version control
- **Make** (optional) - For build automation

### Environment Setup

1. **Clone the repository:**

   ```bash
   git clone https://github.com/vorzela/vorm.git
   cd vorm
   ```

2. **Install dependencies:**

   ```bash
   go mod tidy
   ```

3. **Set up development environment:**

   ```bash
   # Copy environment template
   cp config/.env.example .env

   # Edit configuration for your local database
   vim config/database.yaml
   ```

4. **Build the project:**

   ```bash
   ./scripts/build.sh
   ```

5. **Run tests:**
   ```bash
   go test ./...
   ```

## Project Architecture

### Directory Structure

```
vorm/
├── cmd/vorm/               # CLI application entry point
├── internal/              # Internal packages (not exported)
│   ├── config/            # Configuration management
│   ├── console/           # Terminal output and colors
│   ├── database/          # Database operations
│   ├── logger/            # Logging system
│   ├── migration/         # Migration operations
│   └── utils/             # Utility functions
├── pkg/                   # Public API packages
│   ├── errors/            # Custom error types
│   └── vorm/              # Public client interface
├── config/                # Configuration templates
├── scripts/               # Build and release scripts
├── docs/                  # Documentation
├── tests/                 # Test files
└── migrations/            # Example migrations
```

### Core Components

#### 1. Configuration System (`internal/config/`)

- **`config.go`** - Main configuration structure and loading
- **`validator.go`** - Configuration validation logic

**Key Features:**

- YAML-based configuration with environment variable overrides
- DATABASE_URL parsing support
- Multi-environment support (dev/staging/production)
- Validation with detailed error messages

#### 2. Console System (`internal/console/`)

- **`colors.go`** - Color definitions and terminal styling
- **`output.go`** - Formatted output functions
- **`prompts.go`** - Interactive user prompts and confirmations

**Color Scheme:**

- 🟢 Green: Success messages
- 🔴 Red: Error messages
- 🟡 Yellow: Warning messages
- 🔵 Blue: Information messages

#### 3. Database System (`internal/database/`)

- **`connection.go`** - Database connection management
- **`creator.go`** - Database and table creation
- **`validator.go`** - Connection validation

**Design Principles:**

- Simple connection management (no pooling for CLI tool)
- Admin connections for database operations
- Comprehensive error handling

#### 4. Migration System (`internal/migration/`)

- **`generator.go`** - Migration file generation
- **`tracker.go`** - Migration state tracking
- **`executor.go`** - Migration execution
- **`manager.go`** - High-level migration coordination

**Migration Features:**

- Laravel-style templates with Up/Down sections
- Batch tracking for rollbacks
- Checksum validation for integrity
- Transaction support for atomicity

#### 5. Logging System (`internal/logger/`)

- **`logger.go`** - Comprehensive logging with file rotation

**Logging Features:**

- Multiple log levels (DEBUG, INFO, SUCCESS, WARNING, ERROR, FATAL)
- File rotation with compression
- Console output with colors
- Migration-specific logging methods

### Public API (`pkg/`)

The public API is designed for external integrations:

```go
// Example usage
import "github.com/vorzela/vorm/pkg/vorm"

client, err := vorm.NewClient(config)
if err != nil {
    log.Fatal(err)
}

// Run migrations
err = client.Migrate()
```

## Code Style Guidelines

### Go Standards

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `gofmt` for code formatting
- Use `golint` for style checking
- Use `go vet` for static analysis

### Naming Conventions

#### Database Objects

- **Tables:** Plural names (`users`, `products`, `order_items`)
- **Pivot Tables:** Alphabetical ordering (`product_user`, not `user_product`)
- **Migrations:** Descriptive names (`create_users_table`, `add_index_to_products`)

#### Go Code

- **Packages:** lowercase, single word when possible
- **Functions:** PascalCase for exported, camelCase for internal
- **Variables:** camelCase
- **Constants:** PascalCase or UPPER_CASE for package-level

### Error Handling

Use custom error types from `pkg/errors/`:

```go
// Good
return errors.NewMigrationError("Migration failed", err.Error(), migrationName)

// Bad
return fmt.Errorf("migration failed: %v", err)
```

### Comments

- Document all exported functions and types
- Use complete sentences
- Explain the "why", not just the "what"

```go
// GenerateMigration creates a new migration file with Laravel-style structure.
// It ensures proper naming conventions and generates boilerplate SQL templates.
func GenerateMigration(name string) (*MigrationFile, error) {
    // Implementation...
}
```

## Building and Testing

### Build Commands

```bash
# Build for current platform
./scripts/build.sh

# Build for all platforms
./scripts/build.sh --all

# Build with specific version
VERSION=v1.0.0 ./scripts/build.sh
```

### Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/config

# Run integration tests (requires PostgreSQL)
go test ./tests/integration

# Run with verbose output
go test -v ./...
```

### Test Structure

```
tests/
├── fixtures/              # Test data and fixtures
├── integration/           # Integration tests
└── unit/                  # Unit tests
```

### Database Testing

For tests requiring a database:

1. Set up test database:

   ```sql
   CREATE DATABASE vorm_test;
   ```

2. Set test environment:
   ```bash
   export VORM_DB_NAME=vorm_test
   export VORM_ENVIRONMENT=test
   ```

## Contributing

### Workflow

1. **Fork the repository**
2. **Create a feature branch:**

   ```bash
   git checkout -b feature/your-feature-name
   ```

3. **Make your changes**
4. **Add tests** for new functionality
5. **Ensure all tests pass:**

   ```bash
   go test ./...
   ```

6. **Format code:**

   ```bash
   gofmt -w .
   ```

7. **Commit with descriptive messages:**

   ```bash
   git commit -m "feat: add new migration rollback feature"
   ```

8. **Push and create pull request**

### Commit Message Format

Use [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` - New features
- `fix:` - Bug fixes
- `docs:` - Documentation changes
- `style:` - Code style changes
- `refactor:` - Code refactoring
- `test:` - Test additions or modifications
- `chore:` - Maintenance tasks

### Pull Request Guidelines

- **Title:** Clear and descriptive
- **Description:** Explain what changes were made and why
- **Tests:** Include tests for new functionality
- **Documentation:** Update docs if needed
- **Breaking Changes:** Clearly document any breaking changes

## Release Process

### Version Numbering

VORM follows [Semantic Versioning](https://semver.org/):

- **MAJOR:** Breaking changes
- **MINOR:** New features (backward compatible)
- **PATCH:** Bug fixes (backward compatible)

### Release Steps

1. **Update version in documentation**
2. **Update CHANGELOG.md**
3. **Create release branch:**

   ```bash
   git checkout -b release/v1.1.0
   ```

4. **Build and test:**

   ```bash
   ./scripts/build.sh --all
   go test ./...
   ```

5. **Create release:**

   ```bash
   ./scripts/release.sh v1.1.0
   ```

6. **Push tag:**
   ```bash
   git push origin v1.1.0
   ```

### Release Script

The release script (`scripts/release.sh`) automatically:

- Validates version format
- Builds multi-platform binaries
- Creates release archives
- Generates checksums
- Updates CHANGELOG.md
- Creates git tag

### GitHub Release

After pushing the tag:

1. Go to GitHub Releases
2. Create new release from tag
3. Upload release artifacts
4. Update release notes

## Debugging

### Enable Debug Logging

```bash
export VORM_LOG_LEVEL=debug
vorm migrate
```

### Common Issues

#### Database Connection

```bash
# Test connection
vorm config validate

# Check database exists
vorm db:create
```

#### Migration Issues

```bash
# Check migration status
vorm status

# Validate migration files
vorm list
```

### Profiling

For performance analysis:

```bash
# Build with profiling
go build -ldflags "-X main.profile=true" ./cmd/vorm

# Run with CPU profiling
vorm migrate -cpuprofile=cpu.prof

# Analyze profile
go tool pprof cpu.prof
```

## IDE Setup

### VS Code

Recommended extensions:

- **Go** - Official Go extension
- **Go Doc** - Documentation viewer
- **Go Test Explorer** - Test runner
- **Git Lens** - Git integration

### GoLand/IntelliJ

- Enable Go modules support
- Configure code style to match project
- Set up run configurations for tests

## Resources

- [Go Documentation](https://golang.org/doc/)
- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
- [Cobra CLI Framework](https://github.com/spf13/cobra)
- [Viper Configuration](https://github.com/spf13/viper)
- [VORM Issue Tracker](https://github.com/vorzela/vorm/issues)

## Getting Help

- **Documentation:** Check the [docs/](../docs/) directory
- **Issues:** Create an issue on GitHub
- **Discussions:** Use GitHub Discussions for questions
- **Contributing:** See [CONTRIBUTING.md](../CONTRIBUTING.md)
