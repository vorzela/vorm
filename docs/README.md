# VORM Documentation

This directory contains comprehensive documentation for the VORM PostgreSQL Migration Tool.

## Contents

- [Installation Guide](installation.md) - How to install VORM
- [Configuration Guide](configuration.md) - Setting up VORM configuration
- [Migration Guide](migrations.md) - Creating and managing migrations
- [Commands Reference](commands.md) - Complete CLI command reference
- [Production Guide](production.md) - Production deployment best practices
- [Troubleshooting](troubleshooting.md) - Common issues and solutions
- [Development Guide](development.md) - Contributing to VORM

## Quick Links

- [Getting Started](#getting-started)
- [Common Workflows](#common-workflows)
- [FAQ](#faq)

## Getting Started

1. **Install VORM**: Follow the [Installation Guide](installation.md)
2. **Initialize Project**: Run `vorm init` in your project directory
3. **Configure Database**: Edit `config/database.yaml` and `.env`
4. **Setup Database**: Run `vorm setup` to create database and migrations table
5. **Create Migration**: Run `vorm make:migration create_users_table`
6. **Run Migration**: Run `vorm migrate`

## Common Workflows

### Creating a New Table

```bash
# Create migration
vorm make:migration create_products_table

# Edit the migration file
nano migrations/2025_06_14_180339_create_products_table.sql

# Run the migration
vorm migrate
```

### Adding a Column

```bash
# Create migration
vorm make:migration add_price_to_products

# Edit migration to add column
# Run migration
vorm migrate
```

### Rolling Back Changes

```bash
# Rollback last batch
vorm rollback

# Rollback specific number of steps
vorm rollback --step 2

# Check status
vorm status
```

## FAQ

### Q: How do I handle large databases?

A: VORM is optimized for large-scale applications. Use transactions, monitor execution times with logging, and test migrations on staging first.

### Q: Can I use VORM in production?

A: Yes! VORM includes production safety features like typed confirmations and environment checks.

### Q: How do I backup before migrations?

A: VORM recommends using PostgreSQL's `pg_dump` before running migrations in production.

## Support

For more help:

- Check the [Troubleshooting Guide](troubleshooting.md)
- Review [GitHub Issues](https://github.com/vorzela/vorm/issues)
- Join [GitHub Discussions](https://github.com/vorzela/vorm/discussions)
