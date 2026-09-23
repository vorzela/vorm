# Migrations

vorm applies migrations itself, in the process that calls it.

```bash
export DATABASE_URL=postgres://user:pass@localhost:5432/app?sslmode=disable
vorm migrate
```

## File format

One file per migration, named `<unix_timestamp>_<snake_case>.go`, with `Up` and
`Down` that call the Schema facade. The runner compiles that AST to SQL — it
never executes the functions (that would write extra files and AutoMigrate).

```go
//go:build ignore

package migrations

import "github.com/vorzela/vorm/schema"

func Up(s *schema.Facade) {
	s.Create("posts", func(t *schema.Blueprint) {
		t.ID()
		t.String("title")
		t.ForeignId("user_id").Constrained("users").CascadeOnDelete()
		t.Timestamps()
	})
}

func Down(s *schema.Facade) {
	s.DropIfExists("posts")
}
```

`vorm make migration posts` writes that file. `vorm make migration post_tag`
writes `s.BelongsToMany("posts", "tags")`. Dedicated relation commands do the
same job with explicit names:

```bash
vorm make belongs-to posts users          # alter posts: t.BelongsTo("user_id", "users")
vorm make has-one users profiles          # unique FK on profiles
vorm make has-many users posts            # FK on posts (inverse of belongs-to)
vorm make belongs-to-many posts tags      # pivot table
vorm make morphs comments commentable     # morphTo columns on comments
vorm make morph-to-many tags taggable     # polymorphic pivot taggables
```

Numbered `*.sql` files with Up/Down
markers still apply (legacy). Files without a numeric prefix are ignored, which
is what keeps `extensions.sql`, `enums.sql` and `functions.sql` out of the
sequence.

## Commands

```bash
vorm migrate [--steps=N] [--dry-run] [--verbose] [--no-lint]
vorm status                       # applied, pending, changed-since-applied
vorm rollback [--steps=1]         # by batch, newest first
vorm rollback --steps=all
vorm rollback --migration=create_posts
vorm fresh --force                # roll everything back, then re-apply
```

`--dry-run` reports what would run without touching the database.
`--skip-lock` is available for environments where advisory locks are
unavailable, and should otherwise be left alone.

## Safety

**Locking.** PostgreSQL advisory locks and MySQL named locks mean two deploys
cannot apply at the same time. On a lock error: wait for the other process,
check for a stuck session, then rerun `vorm status` and `vorm migrate`.

**Transactions.** Each migration runs in a transaction when the dialect allows
it, so a failing statement leaves nothing half-applied. MySQL DDL is implicitly
committed; that is a database limitation, not a vorm one.

**Checksums.** The SHA-256 of each applied file is stored. `vorm status` reports
`CHANGED SINCE APPLIED` when a file was edited afterwards. Prefer restoring the
file; `--force` exists but should be a deliberate choice.

**Linting.** `vorm migrate` lints the directory first and refuses to run on
errors. `vorm lint` runs it alone; `--no-lint` skips the gate.

## Editing schema

Prefer editing the original create migration while it is still local or
unreleased, then rebuilding with `vorm fresh --force`. Long `add_*` / `alter_*`
chains are worth it only once the migration has been applied somewhere you
cannot reset.

Once a table exists in an environment you cannot drop, write the alter
(`vorm make migration add_phone_to_users`):

```go
func Up(s *schema.Facade) {
	s.Table("users", func(t *schema.Blueprint) {
		t.String("phone", 32)
	})
}

func Down(s *schema.Facade) {
	s.Table("users", func(t *schema.Blueprint) {
		t.DropColumn("phone")
	})
}
```

Then `vorm generate` to pick the column up in the models.

## PostgreSQL prerequisites

`extensions.sql`, `enums.sql` and `functions.sql` live next to the migrations and
are declarative: an uncommented `CREATE` line is enabled, a commented one is
disabled. They are applied before the migrations, and only when their contents
changed since the last run (tracked in `.vorm_*_hash` sidecars).

```bash
vorm extensions             # sync migrations/extensions.sql
vorm enums                  # create types and add new values
vorm functions              # CREATE OR REPLACE every function
vorm enums status           # file versus database
vorm enums --drop-disabled  # remove commented-out types too
vorm enums --dry-run        # print the SQL instead of running it
```

Enum syncing is re-runnable: each type becomes a block that creates it when
missing and otherwise issues `ALTER TYPE … ADD VALUE IF NOT EXISTS`. PostgreSQL
cannot remove an enum value, so deleting one from the file is not synced.
Dropping a disabled type is guarded — it is kept while any column still uses it.

For a change that must be versioned alongside a table, use a migration instead:
`vorm make enum order_status pending,paid,shipped` or
`vorm make extension pgcrypto` write ordinary migration files.

| Concern | Declarative | Versioned |
|---------|-------------|-----------|
| Extension | `extensions.sql` + `vorm extensions` | `vorm make extension <name>` |
| Enum type | `enums.sql` + `vorm enums` | `vorm make enum <type> a,b,c` |
| Function / trigger helper | `functions.sql` + `vorm functions` | `Facade.CreateFunction` |

## After migrating

```bash
vorm generate    # models from the live schema, then typed queries
```

Models are regenerated from the database, so a migration is only fully applied
once `vorm generate` has run.
