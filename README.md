# vorm (Go)

A codegen-first data layer for PostgreSQL, MySQL and MariaDB: migrations, models
generated from the live database, and sqlc-style typed queries — one binary, no
external migration tool required.

```bash
go install github.com/vorzela/vorm/cmd/vorm@latest

export DATABASE_URL=postgres://user:pass@localhost:5432/app?sslmode=disable
vorm init                 # write vorm.yaml (dialect detected from DATABASE_URL)
vorm make migration posts # timestamped Blueprint Up/Down in migrations/
vorm make belongs-to-many posts tags
vorm migrate              # apply migrations in-process
vorm generate             # models from the database + typed queries from stubs
```

## What generates what

| Path | Who writes it | Contents |
|------|---------------|----------|
| `migrations/` | you / `vorm make` | numbered Blueprint `*.go` (and legacy `*.sql`) |
| `migrations/{extensions,enums,functions}.sql` | you | declarative PostgreSQL prerequisites |
| `queries/` | you | `// vorm:query` stubs |
| **`models/`** | `vorm generate models` | **never hand-edit** — structs, enums, indexes, relations |
| **`vorm/gen/`** | `vorm generate` | **never hand-edit** — `db.go` + one `{source}.sql.go` per stub file |

## Migrations run in-process

The runner is a Go package (`migrate`), not a subprocess. It records applied
files in a `migrations` table with checksums, batch numbers and advisory locks.

```bash
vorm migrate [--steps=N] [--dry-run] [--verbose]
vorm status
vorm rollback [--steps=1] [--migration=create_users] [--steps=all]
vorm fresh --force          # roll everything back, then re-apply
```

A migration file is numbered Blueprint Go (`{unix}_{name}.go`) with `Up`/`Down`.
Legacy numbered `.sql` files still apply. `vorm make migration posts` writes Go;
`vorm make belongs-to` / `belongs-to-many` / `morphs` write relationship
Blueprints. The runner compiles the AST to SQL and never executes `Up`/`Down`.

```go
func Up(s *schema.Facade) {
	s.Create("posts", func(t *schema.Blueprint) {
		t.ID()
		t.String("title")
		t.Jsonb("meta")
		t.Timestamps() // TIMESTAMPTZ on Postgres
	})
}

func Down(s *schema.Facade) {
	s.DropIfExists("posts")
}
```

Built-in column helpers cover `Jsonb`, `TimestampTz`, `Decimal`, `Inet`, and more.
Extension types (`citext`, PostGIS) use `t.CustomType("…").Column("…")` after
`CREATE EXTENSION` (see `migrations/extensions.sql`).

`vorm migrate` lints first (`--no-lint` to skip), takes a lock so two deploys
cannot race, runs each file in a transaction where the dialect allows it, and
records a SHA-256 of the file. `vorm status` flags a file that changed after it
was applied.

### PostgreSQL prerequisites

`extensions.sql`, `enums.sql` and `functions.sql` sit next to the migrations and
are declarative: an uncommented `CREATE` line means "I want this", a commented
one means "I do not". They are applied before the migrations, and only when the
file changed.

```bash
vorm extensions            # sync migrations/extensions.sql
vorm enums                 # create types, add new values
vorm functions             # CREATE OR REPLACE every function
vorm enums status          # compare the file against the database
vorm enums --drop-disabled # also remove commented-out types
```

Syncing is re-runnable: enums become a create-or-add-values block rather than a
`CREATE TYPE` that fails the second time, and a disabled enum is only dropped
when no column still uses it.

## When to use vorm, vm, or sqlc

Three related tools cover migrations and typed Go queries. Pick by what you want
to own:

| | **[vorm](https://github.com/vorzela/vorm)** | **[vm](https://github.com/vorzela/vorzela-migrate)** | **[sqlc](https://sqlc.dev/)** |
|---|----------|----------|----------|
| Role | Codegen data layer (models + queries + migrations) | Migration CLI only | SQL → typed Go codegen |
| Query authoring | Fluent Go stubs (`// vorm:query`) lowered to SQL | — (pair with vorm or sqlc) | Hand-written `.sql` |
| Models | Generated from live DB (or Blueprint) | — | You define / override in config |
| Relations / soft deletes | Built-in (`With`, morphs, `SoftDeletes`) | — | You write joins and helpers |
| Migrations | In-process Blueprint / SQL runner | SQL migrations: drift detection, online DDL, locks, checksums | External (often goose / golang-migrate) |
| Best when | One binary for schema → models → typed queries | Production-grade SQL migrator (zero-downtime, drift) | SQL-first reviews; thinnest query codegen |

**Use vorm** when you want introspected models, Go query stubs, and migrations
without a second toolchain.

**Use vm** ([vorzela-migrate](https://github.com/vorzela/vorzela-migrate)) when
migrations are the hard problem — schema drift, online DDL, or a SQL-only
migration workflow — and you may still generate queries with vorm or sqlc.

**Use sqlc** when queries are SQL-first and you prefer reviewing `.sql` files.

You can combine them: `vm migrate` for schema, then `vorm generate` or
`sqlc generate` for typed queries (point outputs at different packages).

## Models come from the database

`vorm generate models` introspects the live schema, so nullability, enums,
indexes and foreign keys are exact rather than inferred:

```go
type UserStatus string

const (
	UserStatusActive  UserStatus = "active"
	UserStatusInvited UserStatus = "invited"
)

type User struct {
	ID        int64      `json:"id" db:"id"`
	Email     string     `json:"email" db:"email"`
	Name      *string    `json:"name" db:"name"`     // nullable → pointer
	Status    UserStatus `json:"status" db:"status"` // enum → typed constant
	DeletedAt *time.Time `json:"deleted_at" db:"deleted_at"`

	Posts []Post `json:"posts,omitempty" db:"-"` // from the foreign key
}

var Users = query.Model[User](query.Meta{
	Table:       UserTable,
	Columns:     UserColumnList,
	PrimaryKey:  "id",
	SoftDeletes: true,
	Indexes:     UserIndexes,
})
```

Without a reachable database, `--from-blueprint` parses numbered `migrations/*.go`
instead. `vorm introspect [--json]` prints exactly what vorm reads.

When `DATABASE_URL` is set, `vorm generate queries` and `vorm check` verify that
on-disk `models/` still match the live schema (tables, columns, Go types) — the
same class of gate sqlc applies to queries. Drift fails with `run: vorm generate models`.

To add a column to an existing table:

```bash
vorm make migration add_phone_to_users   # scaffolds t.String("phone") / DropColumn
# edit the Blueprint if you need Text / Integer / … instead of String
vorm migrate && vorm generate
```

## Queries: runtime builder and generated functions

Write a stub once. It is real, callable Go — and the generator lowers it to
static SQL where it can.

```go
// vorm:query name=SearchUsers
func SearchUsers(ctx context.Context, db query.DB, q string, limit int) ([]models.User, error) {
	return models.Users.WhereSearch([]string{"name", "email"}, q).OrderBy("name").Limit(limit).Get(ctx, db)
}
```

`vorm generate` emits a typed `Row`, a `Params` struct and the SQL:

```go
type SearchUsersRow struct { /* exactly the projected columns */ }
type SearchUsersParams struct { Q string; Limit int }

func SearchUsers(ctx context.Context, db query.DB, arg SearchUsersParams) ([]SearchUsersRow, error) {
	pattern1 := query.LikePattern(arg.Q)
	const searchUsersSQL = `SELECT "id", … FROM "users" WHERE ("name" ILIKE $1 OR "email" ILIKE $2) AND "deleted_at" IS NULL ORDER BY "name" ASC LIMIT $3`
	// …
}
```

Call `gen.SearchUsers` from application code. A stub the generator cannot lower
completely is reported by name and reason, and keeps working through the runtime
builder — generation never silently produces a function that fails.

### The builder

```go
users, err := models.Users.
	Where("active", true).
	Where("age", ">=", 18).
	WhereIn("status", "active", "invited").
	With("posts").              // batched eager load, no N+1
	OrderBy("created_at", "DESC").
	Limit(20).
	Get(ctx, db)

page, err := models.Users.Where("active", true).OrderBy("id").
	Paginate(ctx, db, query.PageRequest{Page: 2, PerPage: 25})
fmt.Println(page.TotalCount(), page.HasMore)

n, err := models.Users.Where("id", id).SoftDelete(ctx, db)
```

Rows scan into structs automatically through a cached reflection plan; no mapper
registration is needed.

## Type safety

Column names are checked against `Meta.Columns` and values against the model's
Go types, before any SQL is sent:

```go
models.Users.Where("actve", true)          // unknown column "actve"
models.Users.Where("age", "not-a-number")  // column "age" expects int32, got string
```

Identifiers are validated and quoted per dialect, operators come from a
whitelist, `ORDER BY` accepts only `ASC`/`DESC`, and values are always bound as
`$n` / `?`. Generated code never emits `SELECT *`.

## Errors and logging

Driver errors are classified, so callers branch on meaning instead of matching
strings:

```go
if _, err := models.Users.New().Create(ctx, db, values); err != nil {
	switch {
	case query.IsUniqueViolation(err):
		return fmt.Errorf("email %s is taken (%s)", email, query.Constraint(err))
	case query.IsRetryable(err):
		return retry(ctx)
	}
}
```

`query.Code`, `query.Constraint`, `query.Classify` and the `Is*` helpers cover
PostgreSQL SQLSTATE and MySQL errno. Logging is an interface with an `slog`
implementation:

```go
query.SetDefaultLogger(query.NewSlogLogger(slog.Default()))
ctx = query.WithLogger(ctx, myLogger) // or per-request
```

Each event carries the SQL, arguments, duration, rows affected and error.

## Relations

Generated models register associations from foreign keys, pivot tables and
`t.Morphs(...)`, so `With("posts")` / `With("posts.comments")` load a whole
batch in a bounded number of queries. Projection is always `Meta.Columns`.

```go
query.RegisterRelation(query.Relation{
	Name: "posts", Kind: query.RelationHasMany, Table: "posts",
	LocalKey: "id", ForeignKey: "user_id",
}, func(ctx context.Context, db query.DB, rows []*User) error {
	return query.LoadHasMany(ctx, db, rows, query.HasMany[User, Post]{
		Related:    Posts,
		ForeignKey: "user_id",
		ParentKey:  func(m *User) any { return m.ID },
		ChildKey:   func(r *Post) any { return r.UserID },
		Assign:     func(m *User, rows []Post) { m.Posts = rows },
	})
})
```

Many-to-many uses `TagsRelation().Attach/Detach/Sync/Toggle` (and `AttachTags`,
…) because Go cannot reuse the `Tags` field name for a method. `t.Morphs("commentable")`
stores the related **table name** in `commentable_type`.

`LoadHasMany`, `LoadBelongsTo`, `LoadBelongsToMany` and `LoadMorphTo` are also
usable directly.

## Drivers

```go
db, err := query.Open(ctx, os.Getenv("DATABASE_URL"))       // picks the driver
db, err := query.OpenPostgres(ctx, url)                     // pgx v5 (default)
db, err := query.OpenPostgres(ctx, url, query.WithDriver(query.PostgresPQ))
db, err := query.OpenMySQL("user:pass@tcp(localhost:3306)/app?parseTime=true")
```

## Config (`vorm.yaml`)

`vorm init` writes `vorm.yaml` (or use `vorm.yml`). Legacy `.vorm` KEY=value
still loads when no YAML file is present. JSON tags on generated structs default
to **lowercase snake_case** (column `display_name` → `json:"display_name"`).

```bash
vorm config                     # effective values and where they came from
vorm config set PACKAGE=vormgen # avoid a clash with another "gen" package
vorm config lint
```

Full key list (YAML names; uppercase equivalents work in `.vorm`):

| YAML key | Default | Meaning |
|----------|---------|---------|
| `version` | `"1"` | config schema version (informational) |
| `package` | `gen` | Go package name for generated queries |
| `out_dir` | `./vorm/<package>` | where `db.go` and `{source}.sql.go` are written |
| `query_dir` | `./queries` | `// vorm:query` stubs |
| `model_dir` | `./models` | generated model package directory |
| `schema_dir` | `./migrations` | Blueprint / schema sources for `--from-blueprint` |
| `model_package` | `models` | Go package name for models |
| `model_import` | from `go.mod` | import path used by generated queries |
| `driver` | `pgx` | `pgx` or `pq` |
| `dialect` | `postgres` | `postgres`, `mysql`, `mariadb` |
| `migration_path` | `./migrations` | migration files for `vorm migrate` |
| `model_source` | `db` | `db` (introspect) or `blueprint` |
| `schema_name` | `public` | Postgres schema / MySQL database name |
| `emit_relations` | `true` | relation fields + loaders on models |
| `emit_functions` | `true` | stored-routine wrappers |
| `include_views` | `false` | generate models for views |
| `emit_sql_as_comment` | `false` | top-level alias for SQL comments on gen funcs |
| `database_url` | — | prefer `DATABASE_URL` in the environment |
| `gen.go.emit_sql_as_comment` | `false` | sqlc-shaped: write SQL as a comment above each gen function |

```yaml
# vorm.yaml — all keys (omit any to keep defaults)
version: "1"

package: gen
out_dir: ./vorm/gen
query_dir: ./queries
model_dir: ./models
schema_dir: ./migrations
model_package: models
# model_import: github.com/acme/app/models   # empty = <module>/models from go.mod

driver: pgx          # pgx | pq
dialect: postgres    # postgres | mysql | mariadb
migration_path: ./migrations
model_source: db     # db | blueprint
schema_name: public

emit_relations: true
emit_functions: true
include_views: false

# Prefer DATABASE_URL in the environment over database_url here.
# database_url: postgres://user:pass@localhost:5432/app?sslmode=disable

gen:
  go:
    emit_sql_as_comment: false
```

See [`docs/API.md`](docs/API.md#config-vormyaml) for the same reference with
legacy `.vorm` key names.

## Further reading

- [`CHANGELOG.md`](CHANGELOG.md) — release history
- [`docs/API.md`](docs/API.md) — catalog of every CLI command and Go API
- [`docs/USAGE.md`](docs/USAGE.md) — end-to-end usage guide, from install to recipes
- [`docs/MIGRATIONS.md`](docs/MIGRATIONS.md) — file format, locks, prerequisites
- [`ARCHITECTURE.md`](ARCHITECTURE.md) — pipelines and package boundaries
- [`LLM.md`](LLM.md) — rules for agents working in a vorm project
- [`examples/`](examples/) — stubs and their generated output

## License

MIT — see [LICENSE](LICENSE).
