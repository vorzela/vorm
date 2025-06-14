# Commands Reference

Complete reference for all VORM CLI commands.

## Global Options

All commands support these global options:

- `--help`, `-h`: Show help for command
- `--version`: Show version information

## Project Initialization

### `vorm init`

Initialize VORM in your project directory.

```bash
vorm init
```

**What it does:**

- Creates `config/database.yaml` configuration file
- Creates `.env` environment file
- Creates `migrations/` directory
- Creates `logs/` directory

**Options:** None

### `vorm setup`

Setup database and migrations table.

```bash
vorm setup
```

**What it does:**

- Creates the database if it doesn't exist
- Creates the migrations tracking table
- Validates database connection

**Prerequisites:**

- Configuration files must exist (run `vorm init` first)
- Database credentials must be configured

## Migration Creation

### `vorm make:migration <name>`

Create a new migration file.

```bash
vorm make:migration create_users_table
vorm make:migration add_email_index_to_users
vorm make:migration create_product_user_pivot_table
```

**Arguments:**

- `<name>`: Descriptive name for the migration

**What it creates:**

- SQL file with timestamp prefix
- Template with Up and Down sections
- Proper naming conventions (plural tables)

**Examples:**

```bash
# Table creation
vorm make:migration create_orders_table

# Column addition
vorm make:migration add_status_to_orders

# Index creation
vorm make:migration add_index_to_users_email

# Pivot table (alphabetically ordered)
vorm make:migration create_order_product_table
```

## Migration Execution

### `vorm migrate`

Run pending migrations.

```bash
vorm migrate                    # Run all pending
vorm migrate --step 3          # Run specific number
```

**Options:**

- `--step`, `-s <number>`: Run specific number of migrations

**What it does:**

- Executes pending migrations in chronological order
- Tracks executed migrations in database
- Runs in transactions for atomicity
- Logs execution time and details

### `vorm rollback`

Rollback migrations.

```bash
vorm rollback                   # Rollback last batch
vorm rollback --step 2         # Rollback 2 migrations
vorm rollback --to migration_name # Rollback to specific migration
```

**Options:**

- `--step`, `-s <number>`: Rollback specific number of steps
- `--to <migration>`: Rollback to specific migration

**Safety Features:**

- Requires confirmation in development
- Requires typed confirmation in production
- Disabled in production if configured

## Status and Information

### `vorm status`

Show migration status.

```bash
vorm status
```

**Output:**

- List of all migrations
- Execution status (Pending/Executed)
- Execution timestamp
- Batch number

### `vorm list`

List all migration files.

```bash
vorm list
```

**Output:**

- Migration names
- Filenames
- Checksums

### `vorm history`

Show migration execution history.

```bash
vorm history
```

**Output:**

- Executed migrations only
- Execution timestamps
- Batch numbers
- Execution times

## Database Operations

### `vorm db:create`

Create the database.

```bash
vorm db:create
```

**What it does:**

- Creates database if it doesn't exist
- Uses admin connection to PostgreSQL server
- Handles permissions and ownership

### `vorm db:drop`

Drop the database.

```bash
vorm db:drop
```

**Safety Features:**

- Requires confirmation
- Disabled in production environments
- Requires typed "DROP" confirmation

### `vorm db:reset`

Drop and recreate database.

```bash
vorm db:reset
```

**What it does:**

- Drops existing database
- Creates new empty database
- Requires strong confirmation

**Safety Features:**

- Disabled in production
- Requires typed "RESET" confirmation

## Configuration

### `vorm config:show`

Display current configuration.

```bash
vorm config:show
```

**Output:**

- Database connection settings
- Migration settings
- Logging configuration
- Environment information

### `vorm config:validate`

Validate configuration.

```bash
vorm config:validate
```

**What it checks:**

- Configuration file syntax
- Required fields presence
- Database connectivity
- File permissions

## Destructive Operations

⚠️ **Warning:** These operations can cause data loss!

### `vorm reset`

Reset all migrations (rollback everything).

```bash
vorm reset
```

**Safety Features:**

- Disabled in production
- Requires typed "RESET" confirmation
- Rolls back all executed migrations

### `vorm fresh`

Drop all tables and re-run migrations.

```bash
vorm fresh
```

**What it does:**

- Drops all database tables
- Re-runs all migrations from scratch
- Recreates migrations table

**Safety Features:**

- Disabled in production
- Requires typed "FRESH" confirmation

### `vorm refresh`

Rollback and re-run all migrations.

```bash
vorm refresh
```

**What it does:**

- Rolls back all migrations
- Re-runs all migrations
- Maintains database structure

**Safety Features:**

- Disabled in production
- Requires typed "REFRESH" confirmation

## Exit Codes

VORM uses standard exit codes:

- `0`: Success
- `1`: General error
- `2`: Configuration error
- `3`: Database connection error
- `4`: Migration error

## Examples

### Complete Workflow

```bash
# 1. Initialize project
vorm init

# 2. Configure database (edit config files)
nano config/database.yaml
nano .env

# 3. Setup database
vorm setup

# 4. Create first migration
vorm make:migration create_users_table

# 5. Edit migration file
nano migrations/2025_06_14_180302_create_users_table.sql

# 6. Run migration
vorm migrate

# 7. Check status
vorm status

# 8. Create another migration
vorm make:migration add_index_to_users_email

# 9. Run new migration
vorm migrate

# 10. View history
vorm history
```

### Rollback Workflow

```bash
# Check current status
vorm status

# Rollback last batch
vorm rollback

# Check status again
vorm status

# Rollback specific number of steps
vorm rollback --step 2

# Re-run migrations
vorm migrate
```
