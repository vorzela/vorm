# vorm — usage guide

A walkthrough of a real project, from an empty directory to typed queries running
against PostgreSQL. Every command here is one you will actually type; the output
shown is what vorm prints. For the full method catalog see [`API.md`](API.md).

- [Install](#install)
- [Start a project](#start-a-project)
- [Write a migration](#write-a-migration)
- [Generate models](#generate-models)
- [Query at runtime](#query-at-runtime)
- [Generate typed queries](#generate-typed-queries)
- [Relations](#relations)
- [Pagination](#pagination)
- [Fast terminals](#fast-terminals)
- [Errors](#errors)
- [Logging](#logging)
- [Transactions](#transactions)
- [PostgreSQL prerequisites](#postgresql-prerequisites)
- [Changing the schema later](#changing-the-schema-later)
- [MySQL and MariaDB](#mysql-and-mariadb)
- [Recipes](#recipes)
- [Troubleshooting](#troubleshooting)

## Install

```bash
go install github.com/vorzela/vorm/cmd/vorm@latest
vorm version
```

The CLI runs migrations in-process.

## Start a project

```bash
mkdir myapp && cd myapp
go mod init myapp
go get github.com/vorzela/vorm

export DATABASE_URL="postgres://user:pass@localhost:5432/myapp?sslmode=disable"
vorm init
```

`vorm init` writes `.vorm` with the dialect detected from `DATABASE_URL`:

```
wrote .vorm (PACKAGE=gen, DIALECT=postgres)
next: export DATABASE_URL=… && vorm migrate && vorm generate
```

Check what vorm resolved at any time:

```bash
vorm config          # effective values and their source
vorm config lint     # catch typos and bad combinations
```

Keep `DATABASE_URL` in the environment rather than in `.vorm`; the environment
always wins, which is what makes the same config work in CI and production.

## Write a migration

The usual path is a numbered Blueprint file under `migrations/`. Legacy numbered
`.sql` files still apply.

**Write the SQL yourself.** Create
`migrations/<unix_timestamp>_create_users_table.sql` with both directions:

```sql
-- migrations/1700000000_create_users_table.sql

-- ⬆ Up (Run when migrating forward)
CREATE TYPE user_status AS ENUM ('active', 'invited', 'banned');

CREATE TABLE users (
    id         BIGSERIAL PRIMARY KEY,
    email      VARCHAR(255) NOT NULL UNIQUE,
    name       TEXT,
    status     user_status NOT NULL DEFAULT 'invited',
    age        INT NOT NULL DEFAULT 0,
    active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX users_status_idx ON users (status);

-- ⬇ Down (Run when rolling back)
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS user_status;
```

**Or describe the table in Go.** `vorm make migration users` writes a numbered
Blueprint file under `migrations/`:

```go
func Up(s *schema.Facade) {
	s.Create("users", func(t *schema.Blueprint) {
		t.ID()
		t.String("email").Unique()
		t.Enum("status", "active", "invited", "banned")
		t.Boolean("active").Default(true)
		t.Timestamps()
		t.SoftDeletes()
	})
}

func Down(s *schema.Facade) {
	s.DropIfExists("users")
}
```

`vorm migrate` compiles `Up`/`Down` to SQL and applies it. Then
`vorm generate models` introspects the live database (or `--from-blueprint` to
parse the same Go files).

**Relationship migrations.** These write an alter or a pivot instead of a full
create:

```bash
vorm make belongs-to posts users          # posts.user_id → users
vorm make belongs-to posts users author_id
vorm make has-one users profiles          # unique profiles.user_id
vorm make has-many users posts            # same FK as belongs-to, from the parent side
vorm make belongs-to-many posts tags      # s.BelongsToMany("posts", "tags")
vorm make morphs comments commentable     # commentable_type + commentable_id
vorm make morph-to-many tags taggable     # taggables pivot (tag_id + morphs)
```

```bash
vorm migrate
```

```
  migrate  1700000000_create_users_table.go  ok (36ms)
migrate: 1 migration(s) in batch 1
```

`vorm migrate` lints first, takes a lock so two deploys cannot collide, runs the
file in a transaction, and stores a checksum. Related commands:

```bash
vorm status                    # applied / pending / changed since applied
vorm migrate --dry-run         # show what would run
vorm rollback                  # undo the last batch
vorm rollback --steps=all
vorm fresh --force             # roll everything back, then re-apply
```

## Generate models

```bash
vorm generate models
```

```
vorm generate models: 2 table(s) from database introspection, package=models (DO NOT EDIT)
  models/user_gen.go
  models/enums_gen.go
  models/vorm_gen.go
```

Relation loaders and Attach/Sync helpers are generated into the owning table
file (`user_gen.go`), not a separate dump.

Models come from the live database, so nullability, enum values, indexes and
foreign keys are exact:

```go
type User struct {
	ID        int64      `json:"id" db:"id"`
	Email     string     `json:"email" db:"email"`
	Name      *string    `json:"name" db:"name"`     // NULL-able → pointer
	Status    UserStatus `json:"status" db:"status"` // enum → typed
	Age       int32      `json:"age" db:"age"`
	Active    bool       `json:"active" db:"active"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	DeletedAt *time.Time `json:"deleted_at" db:"deleted_at"`
}
```

Each database enum becomes a Go type with constants, `Valid()`, `String()`, and
`sql.Scanner` / `driver.Valuer`, so an unknown value fails at scan time instead
of spreading through the program:

```go
const (
	UserStatusActive  UserStatus = "active"
	UserStatusInvited UserStatus = "invited"
	UserStatusBanned  UserStatus = "banned"
)
```

Never edit anything under `models/` — the next `vorm generate` overwrites it.

To see the schema vorm reads without generating anything:

```bash
vorm introspect          # human readable
vorm introspect --json   # machine readable
```

## Query at runtime

```go
db, err := query.Open(ctx, os.Getenv("DATABASE_URL"))
if err != nil {
	return err
}
defer db.Close()

users, err := models.Users.
	Where("active", true).
	Where("age", ">=", 18).
	WhereIn("status", models.UserStatusActive, models.UserStatusInvited).
	OrderBy("created_at", "DESC").
	Limit(20).
	Get(ctx, db)
```

Rows scan into structs through a cached reflection plan — no mapper to register.
The common shapes:

```go
u, err := models.Users.FindByID(ctx, db, 42)              // *User, nil when absent
u, err := models.Users.Where("email", e).First(ctx, db)   // *User, (nil, nil) when absent
u, err := models.Users.Where("email", e).FirstOrFail(ctx, db) // query.ErrNoRows when absent

n, err := models.Users.New().Count(ctx, db)
ok, err := models.Users.Where("email", e).Exists(ctx, db)

id, err := models.Users.New().Create(ctx, db, map[string]any{
	"email": "ada@example.com", "status": models.UserStatusActive,
})
n, err := models.Users.Where("id", id).Update(ctx, db, map[string]any{"active": false})

n, err := models.Users.Where("id", id).SoftDelete(ctx, db)   // sets deleted_at
n, err := models.Users.Where("id", id).Restore(ctx, db)      // clears it
n, err := models.Users.Where("id", id).ForceDelete(ctx, db)  // real DELETE
```

Soft-deleted rows are excluded automatically; `WithTrashed()` includes them.

Filters:

```go
.Where("age", ">", 18)                       // operator form
.Where("status", query.In("active", "invited"))
.Where("name", query.IsNotNull())
.WhereNull("deleted_at")
.WhereNotIn("status", "banned")
.WhereSearch([]string{"name", "email"}, term) // ILIKE on Postgres, LIKE on MySQL
.OrWhere("role", "admin")
.Join("posts", "posts.user_id = users.id")
.GroupBy("status").Having("COUNT(*)", ">", 5)
```

### Type safety

Columns and value types are checked against the model before any SQL is sent:

```go
models.Users.Where("actve", true)         // unknown column "actve"
models.Users.Where("age", "not-a-number") // column "age" expects int32, got string
```

Identifiers are validated and quoted per dialect, operators come from a
whitelist, `ORDER BY` takes only `ASC`/`DESC`, and every value is bound as `$n`
or `?`. There is no code path that concatenates a value into SQL.

## Generate typed queries

Write the query once as a stub. It is ordinary, callable Go:

```go
// queries/users.go
package queries

// vorm:query name=SearchUsers
func SearchUsers(ctx context.Context, db query.DB, q string, limit int) ([]models.User, error) {
	return models.Users.WhereSearch([]string{"name", "email"}, q).
		OrderBy("name").Limit(limit).Get(ctx, db)
}
```

```bash
vorm generate
```

```
vorm generate queries: 4 → ./vorm/gen (package gen, postgres/pgx)
  vorm/gen/db.go
  vorm/gen/users.sql.go
```

Each stub file becomes one `{source}.sql.go` (`queries/users.go` → `users.sql.go`);
`db.go` holds the dialect and driver constants.

You get an sqlc-style `Row`, a `Params` struct, and the SQL:

```go
type SearchUsersRow struct {
	ID     int64             `json:"id" db:"id"`
	Email  string            `json:"email" db:"email"`
	Status models.UserStatus `json:"status" db:"status"`
	// … exactly the projected columns
}

type SearchUsersParams struct {
	Q     string
	Limit int
}

func SearchUsers(ctx context.Context, db query.DB, arg SearchUsersParams) ([]SearchUsersRow, error)
```

Call the generated function from application code:

```go
rows, err := gen.SearchUsers(ctx, db, gen.SearchUsersParams{Q: "ada", Limit: 20})
```

The SQL is a constant whenever the shape is fixed. Only a variable-length `IN`
forces assembly at call time, and even then every element is bound:

```go
sb.WriteString(query.InClause(query.DialectPostgres, `"id"`, len(args)+1, len(arg.Ids)))
for _, v := range arg.Ids {
	args = append(args, v)
}
```

### When a stub cannot be lowered

A stub that depends on runtime values the generator cannot see is reported
rather than emitted broken:

```
2 stub(s) stayed on the runtime builder — call them directly:
  ListActiveAdultsFind (users.go): Find takes options decided at runtime
  ActivateUsersInTx (users.go): transaction stubs run through query.Transaction
```

Those stubs keep working; you call the function in `queries/` instead of `gen`.
Generation never produces a function that fails at runtime.

## Relations

Generated models register a loader per association, so `With` resolves by name
and never issues one query per parent. Column lists come from `Meta.Columns`;
relation SQL never uses `SELECT *`.

```go
users, err := models.Users.Where("active", true).
	With("posts.comments", "profile").
	Get(ctx, db)
```

`With("posts.comments")` is three batched queries: users, then all of those
users' posts in one `WHERE user_id IN (…)`, then all of those posts' comments
in one `WHERE post_id IN (…)`.

A join table with two foreign keys is many-to-many even when it has extra
columns (`pinned`, timestamps, …). Attach SQL follows the real pivot:
`created_at` / `updated_at` are written only when those columns exist, and
`ON CONFLICT` / MySQL `INSERT IGNORE` only when the two FK columns are unique.

Go cannot reuse the eager-load field name as a method, so the generated API is:

```go
err := post.AttachTags(ctx, db, tagID)
err := post.TagsRelation().AttachWith(ctx, db, tagID, map[string]any{"pinned": true})
err := post.DetachTags(ctx, db, tagID) // no IDs → detach all
err := post.SyncTags(ctx, db, tagIDs...)
err := post.ToggleTags(ctx, db, tagIDs...)
```

`t.Morphs("commentable")` adds `commentable_type` + `commentable_id` (no FK).
The child gets `morphTo`; every other table gets `morphMany`. The type column
stores the **table name** (`posts`, not a PHP class):

```go
comments, err := models.Posts.With("comments").Get(ctx, db)
// SELECT … FROM comments WHERE commentable_id IN (…) AND commentable_type = $n
// $n = "posts"
```

Hand-written loaders use the same primitives:

```go
query.LoadHasMany(ctx, db, rows, query.HasMany[User, Post]{
	Related:    models.Posts,
	ForeignKey: "user_id",
	ParentKey:  func(u *User) any { return u.ID },
	ChildKey:   func(p *Post) any { return p.UserID },
	Assign:     func(u *User, ps []Post) { u.Posts = ps },
})
```

`LoadBelongsTo`, `LoadBelongsToMany`, `LoadHasManyThrough`, `LoadHasOneOfMany` and `LoadMorphTo` cover the other directions.

## Pagination

Offset pagination (Laravel `paginate`) runs the page query plus `COUNT(*)`:

```go
page, err := models.Users.Where("active", true).OrderBy("id").
	Paginate(ctx, db, query.PageRequest{Page: 2, PerPage: 25})

page.Data        // []User
page.TotalCount() // int64, -1 when no count was run
page.Pages
page.HasMore
```

`SimplePaginate` is the same `LIMIT`/`OFFSET` page without `COUNT(*)` (`HasMore`
comes from a peek row). For deep pagination use cursor/keyset style, which does
not scan skipped rows:

```go
page, err := models.Users.SimplePaginate(ctx, db, 2, 25)

page, err := models.Users.OrderBy("id").CursorPaginate(ctx, db, cursor, 50)
// SELECT … FROM "users" WHERE "id" > $1 ORDER BY "id" ASC LIMIT 51
next := page.NextCursor
```

`PageResult` marshals to JSON directly, so an HTTP handler can return it as-is.

## Fast terminals

These stay parameterized and never emit `SELECT *`. `ChunkByID` is keyset (not
OFFSET). Aggregates, `Pluck`, `Increment`, and `Upsert` compile to a single
statement:

```go
err := models.Users.Where("active", true).ChunkByID(ctx, db, 1000, func(rows []models.User) error {
	return nil
})
emails, err := models.Users.Where("active", true).Pluck(ctx, db, "email")
sum, err := models.Users.Sum(ctx, db, "age")
_, err = models.Users.Where("id", id).Increment(ctx, db, "age", 1)
_, err = models.Users.Upsert(ctx, db, []map[string]any{{"email": e, "name": n}}, []string{"email"}, []string{"name"})
row, err := models.Users.FirstOrCreate(ctx, db, map[string]any{"email": e}, map[string]any{"name": n})
```

`WhereHas` / `WithCount` are correlated subqueries, not N+1:

```go
users, err := models.Users.WhereHas("posts").WithCount("posts").Get(ctx, db)
// WHERE EXISTS (SELECT 1 FROM "posts" WHERE "posts"."user_id" = "users"."id")
// SELECT …, (SELECT COUNT(*) FROM "posts" WHERE …) AS "posts_count"
```

Postgres FTS and JSON containment:

```go
models.Users.WhereFullText("search_vector", "ada & lovelace")
models.Posts.WhereJsonContains("metadata", `{"draft":false}`)
```

Generated function wrappers never use `SELECT *`: a set-returning routine is
`SELECT v FROM fn($1) AS t(v)`. Declarative `migrations/{extensions,enums,functions}.sql`
are synced with `vorm extensions|enums|functions`.

## Errors

Driver errors are classified, so callers branch on meaning rather than on
message text:

```go
if _, err := models.Users.New().Create(ctx, db, values); err != nil {
	switch {
	case query.IsUniqueViolation(err):
		return fmt.Errorf("email already registered (%s)", query.Constraint(err))
	case query.IsForeignKeyViolation(err):
		return errBadReference
	case query.IsRetryable(err):
		return retry(ctx)
	default:
		return err
	}
}
```

The message keeps the detail without needing to be parsed:

```
vorm/query: insert users [unique_violation 23505] constraint=users_email_key:
ERROR: duplicate key value violates unique constraint "users_email_key"
```

Available: `IsUniqueViolation`, `IsForeignKeyViolation`, `IsNotNullViolation`,
`IsCheckViolation`, `IsDeadlock`, `IsSerializationFailure`, `IsRetryable`,
`IsNotFound`, plus `query.Code` (SQLSTATE or errno), `query.Constraint`, and
`query.Classify`. PostgreSQL and MySQL both map onto the same kinds.

## Logging

```go
query.SetDefaultLogger(query.NewSlogLogger(slog.Default()))
```

Every statement then logs its SQL, arguments, duration, rows affected and error.
Per-request logging keeps the trace context:

```go
ctx = query.WithLogger(ctx, query.NewSlogLogger(reqLogger))
```

Anything implementing the one-method `query.Logger` interface works; use
`query.LoggerFunc` for a quick adapter (metrics, slow-query alerts).

## Transactions

```go
err := query.Transaction(ctx, db, func(ctx context.Context, tx query.Tx) error {
	id, err := models.Users.New().Create(ctx, tx, userValues)
	if err != nil {
		return err // rolls back
	}
	_, err = models.Posts.New().Create(ctx, tx, postValues(id))
	return err
})
```

The callback receives a `query.Tx` that satisfies `query.DB`, so builders and
generated functions both take it. Returning an error rolls back; returning nil
commits. `query.TransactionOpts` sets the isolation level.

## PostgreSQL prerequisites

Extensions, enum types and functions are declarative files next to the
migrations. An uncommented `CREATE` line means enabled, a commented one means
disabled:

```sql
-- migrations/extensions.sql
CREATE EXTENSION IF NOT EXISTS citext;
-- CREATE EXTENSION IF NOT EXISTS postgis;
```

```bash
vorm extensions             # sync the file
vorm enums                  # create types, add new values
vorm functions              # CREATE OR REPLACE every function
vorm enums status           # compare file against database
vorm enums --dry-run        # print the SQL instead of running it
vorm enums --drop-disabled  # also remove commented-out types
```

The first run writes a commented template, so nothing happens until you opt in.
Syncing is re-runnable: an existing enum gets
`ALTER TYPE … ADD VALUE IF NOT EXISTS` rather than a `CREATE TYPE` that would
fail, and a disabled type is kept while any column still uses it:

```
enums.sql: 1 enum type(s) applied
  kept enum type user_status (still in use)
```

These files are applied before the migrations, and only when they changed. Use
them for schema-wide objects; when a change must be pinned to a specific
migration, use `vorm make enum` or `vorm make extension` instead, which write
ordinary migration files.

## Changing the schema later

```bash
# 1. write migrations/<timestamp>_add_phone_to_users.sql
# 2. apply it
vorm migrate
# 3. regenerate — the models are stale until you do
vorm generate
```

While a migration is still local, editing the original create file and running
`vorm fresh --force` keeps history readable. Once it has been applied somewhere
you cannot reset, write the alter instead. `vorm status` tells you which case
you are in: a file edited after it was applied shows as
`CHANGED SINCE APPLIED`.

## MySQL and MariaDB

Set a MySQL DSN and everything else is the same; placeholders become `?`,
`WhereSearch` uses `LIKE`, and locking uses `GET_LOCK`:

```bash
export DATABASE_URL="mysql://user:pass@tcp(localhost:3306)/myapp?parseTime=true"
vorm init && vorm migrate && vorm generate
```

```go
db, err := query.OpenMySQL("user:pass@tcp(localhost:3306)/myapp?parseTime=true")
```

Extensions, enum types as first-class objects and `vorm functions` are
PostgreSQL-only; those commands refuse to run against MySQL rather than
generating SQL that cannot work.

## Recipes

**Search box with paging**

```go
page, err := models.Users.
	WhereSearch([]string{"name", "email"}, q).
	Where("active", true).
	OrderBy("name").
	Paginate(ctx, db, query.PageRequest{Page: p, PerPage: 25})
```

**Filter by a list from the request**

```go
users, err := models.Users.WhereIn("id", ids...).Get(ctx, db)
```

An empty slice is handled correctly: `IN ()` becomes `1=0` rather than a syntax
error.

**Soft-delete lifecycle**

```go
models.Users.Where("id", id).SoftDelete(ctx, db)
models.Users.WithTrashed().Where("id", id).First(ctx, db)
models.Users.Where("id", id).Restore(ctx, db)
```

**Row lock inside a transaction**

```go
query.Transaction(ctx, db, func(ctx context.Context, tx query.Tx) error {
	u, err := models.Users.Where("id", id).LockForUpdate().FirstOrFail(ctx, tx)
	if err != nil {
		return err
	}
	_, err = models.Users.Where("id", id).Update(ctx, tx, map[string]any{"balance": u.Balance - amount})
	return err
})
```

**Call a database function**

Stored functions become typed wrappers in `models/functions_gen.go`:

```go
name, err := models.UserDisplayName(ctx, db, userID)
```

Trigger functions are skipped, with the reason listed in the generated file.

## Troubleshooting

**`cannot resolve model/entity "models.Users"`** — the stub references a model
that has not been generated. Run `vorm generate models` first, or `vorm generate`
which does both in order.

**`column "x" expects int32, got string`** — the value type does not match the
model field. This is the type check doing its job before the query is sent.

**`vorm lint failed`** — a migration is missing an `Up`/`Down` marker or a
`DROP` that matches its `CREATE`. Fix it, or pass `--no-lint` when you have a
reason. The declarative prerequisite files are not linted as migrations.

**`CHANGED SINCE APPLIED` in `vorm status`** — a file was edited after being
applied. Restore the file, or use `vorm fresh --force` in an environment you can
rebuild.

**Migration lock errors** — another migrator holds the lock. Wait for it, check
for a stuck session, then rerun `vorm status` and `vorm migrate`.

**Generated code does not compile** — regenerate rather than editing it; if it
still fails, the stub is the likely cause, and `vorm generate` names the stub
and the reason.

## See also

- [`../README.md`](../README.md) — overview and configuration reference
- [`API.md`](API.md) — catalog of every CLI command and Go API
- [`MIGRATIONS.md`](MIGRATIONS.md) — file format, locks, checksums, drift
- [`../LLM.md`](../LLM.md) — rules for agents working in a vorm project
- [`../examples/`](../examples/) — stubs next to their generated output
