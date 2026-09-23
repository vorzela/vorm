# vorm API reference

Everything you call from the CLI or from Go. Walkthroughs live in
[`USAGE.md`](USAGE.md); this file is the catalog.

Import paths:

```go
import (
	"github.com/vorzela/vorm/query"
	"github.com/vorzela/vorm/schema" // migrations only
	"myapp/models"                   // generated — never hand-edit
	"myapp/vorm/gen"                 // generated typed queries
)
```

Never edit `models/` or `vorm/gen/`. After schema changes: `vorm migrate && vorm generate`.

---

## Contents

1. [CLI](#cli)
2. [Connect](#connect)
3. [Generated models](#generated-models)
4. [Query builder](#query-builder)
5. [Read terminals](#read-terminals)
6. [Write terminals](#write-terminals)
7. [Pagination and chunking](#pagination-and-chunking)
8. [Relations](#relations)
9. [Typed query stubs (`vorm/gen`)](#typed-query-stubs-vormgen)
10. [Schema Blueprint](#schema-blueprint)
11. [Transactions](#transactions)
12. [Errors](#errors)
13. [Logging](#logging)
14. [Operators and helpers](#operators-and-helpers)
15. [Config (`.vorm`)](#config-vorm)

---

## CLI

```bash
vorm init [--force]                 # write .vorm
vorm config                         # effective values and their source
vorm config get KEY
vorm config set KEY=value
vorm config keys
vorm config lint [.vorm]

vorm make migration <name>          # create | pivot (post_tag) | alter (add_*_to_*)
vorm make belongs-to <child> <parent> [column]
vorm make has-one <parent> <child> [column]
vorm make has-many <parent> <child> [column]
vorm make belongs-to-many <left> <right>
vorm make morphs <child> <name>
vorm make morph-to-many <related> <morph>
vorm make relation <kind> …         # same as the kinds above
vorm make enum <type> value1,value2
vorm make extension <name>

vorm lint [migrations/]
vorm migrate [--dry-run] [--steps=N] [--verbose] [--no-lint] [--dsn=] [--path=] [--skip-lock]
vorm status
vorm rollback [--steps=1] [--migration=name] [--steps=all] [--force]
vorm fresh --force                  # roll back everything, then re-apply
vorm refresh --force

vorm extensions [--force] [--dry-run] [--drop-disabled]
vorm enums [status] [--force] [--dry-run] [--drop-disabled]
vorm functions [--force] [--dry-run] [--drop-disabled]

vorm introspect [--json] [--dsn=]
vorm generate [models|queries|all]
    --from-db | --from-blueprint
    --dsn=… --driver=pgx|pq --package=gen

vorm version
vorm help
```

Singular table names are pluralized (`user` → `users`). Relationship commands
write numbered Blueprint files under `migrations/`.

Typical loop:

```bash
vorm make migration posts
# edit Up/Down
vorm migrate
vorm generate
```

---

## Connect

`query.DB` is the handle every terminal takes (`Get`, `Create`, …). `query.Conn`
adds `Close` / `BeginTx`.

```go
db, err := query.Open(ctx, os.Getenv("DATABASE_URL")) // dialect from the URL
defer db.Close()

db, err := query.OpenPostgres(ctx, url)                              // pgx v5
db, err := query.OpenPostgres(ctx, url, query.WithDriver(query.PostgresPQ))
db, err := query.OpenPostgresPQ(url)
db, err := query.OpenMySQL("user:pass@tcp(localhost:3306)/app?parseTime=true")
db, err := query.OpenMariaDB(dsn)
```

| Function | Notes |
|----------|--------|
| `query.Open` | Picks Postgres / MySQL / MariaDB from the URL |
| `query.OpenPostgres` | pgx v5 unless `WithDriver(PostgresPQ)` |
| `query.OpenPostgresPgx` | pgx pool |
| `query.OpenPostgresPQ` | `database/sql` + lib/pq |
| `query.OpenMySQL` / `OpenMariaDB` | `?` placeholders, `parseTime=true` |
| `query.DetectDialect(url)` | `postgres` / `mysql` / `mariadb` |
| `query.SetDefaultDialect` / `DefaultDialect` | Placeholder style for builders |

Generated models call `SetDefaultDialect` in `models/vorm_gen.go`.

---

## Generated models

`vorm generate models` writes one file per table (`models/user_gen.go`) plus
`enums_gen.go`, `functions_gen.go`, `vorm_gen.go`. Relation loaders live on the
table file.

```go
type User struct {
	ID        int64      `json:"id" db:"id"`
	Email     string     `json:"email" db:"email"`
	Name      *string    `json:"name,omitempty" db:"name"` // nullable → pointer
	Status    UserStatus `json:"status" db:"status"`       // enum → typed
	DeletedAt *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`

	Posts      []Post `json:"posts,omitempty" db:"-"`
	PostsCount int64  `json:"posts_count" db:"posts_count"`
	PostsExists bool  `json:"posts_exists" db:"posts_exists"`
}

const UserTable = "users"

var UserColumns = struct{ ID, Email, Name string }{ /* db names */ }
var UserColumnList = []string{"id", "email", /* … */}
var Users = query.Model[User](query.Meta{
	Table: UserTable, Columns: UserColumnList, PrimaryKey: "id", SoftDeletes: true,
})
```

Entry point is the entity (`Users`, `Posts`, …). It is both a builder starter
and a set of one-shot terminals.

Enums get constants, `Valid()`, `String()`, `sql.Scanner`, `driver.Valuer`.

Stored routines become typed wrappers in `functions_gen.go`:

```go
n, err := models.UserPostCount(ctx, db, userID)
err := models.RefreshStats(ctx, db)
ids, err := models.ActiveUserIDs(ctx, db, minID)
```

---

## Query builder

Start from the entity or `New()`:

```go
models.Users.Where("active", true).Limit(10).Get(ctx, db)
models.Users.New().Dialect(query.DialectPostgres).Get(ctx, db)
```

Every chain method returns `*query.Builder[T]`. Columns must exist on
`Meta.Columns`; values must match the model field type. Identifiers are quoted;
values are bound (`$n` / `?`). There is no `SELECT *`.

### Filters

```go
.Where("active", true)
.Where("age", ">", 18)
.Where("age", query.MoreThan(18))
.WhereIn("status", "active", "invited")
.WhereNotIn("id", ids...)
.WhereNull("deleted_at")
.WhereNotNull("email")
.OrWhere("role", "admin")
.WhereSearch([]string{"name", "email"}, term) // ILIKE (Postgres) / LIKE (MySQL)
.WhereRaw(`"age" > ?`, 18)                    // placeholders only; never concatenate values
.WhereExists(`SELECT 1 FROM posts WHERE posts.user_id = users.id`)
.WhereNotExists(...)
.WhereFullText("name", q)                     // Postgres tsquery / MySQL MATCH
.WhereJsonContains("metadata", value)         // @> jsonb / JSON_CONTAINS
.WhereHas("posts")
.WhereDoesntHave("posts")
.WhereRelation("posts", "state", "published")
```

`Where` accepts 2 args (`col, value`) or 3 (`col, op, value`). `ILIKE` is
PostgreSQL-only.

### Projection, order, page window

```go
.Select("id", "email")          // subset of Meta.Columns
.OrderBy("created_at")          // ASC
.OrderBy("created_at", "DESC")
.OrderByDesc("id")
.Limit(20)
.Offset(40)
.Distinct()
.DistinctOn("user_id")          // PostgreSQL
```

### Joins, grouping, locks

```go
.Join("posts", "posts.user_id = users.id")
.LeftJoin(...)
.RightJoin(...)
.GroupBy("status")
.Having("COUNT(*)", ">", 5)
.LockForUpdate()   // FOR UPDATE
.ForUpdate()       // alias
.LockForShare()
.SkipLocked()
```

### Soft deletes

```go
.WithTrashed()   // include deleted_at IS NOT NULL
.OnlyTrashed()   // only those rows
```

Default: soft-deleted rows are excluded when `Meta.SoftDeletes` is true.

### Eager load / extras

```go
.With("posts", "profile")
.With("posts.comments")      // nested; one batched IN per segment
.WithCount("posts")          // posts_count
.WithExists("posts")         // posts_exists
```

### Find options (map style)

```go
rows, err := models.Users.Find(ctx, db, query.FindOptions{
	Where:  query.WhereMap{"active": true, "age": query.MoreThan(18)},
	Order:  query.OrderMap{"name": query.Asc},
	Take:   10,
	Skip:   0,
	Select: []string{"id", "email"},
})
row, err := models.Users.FindOne(ctx, db, opts)
.ApplyFind(opts) // on a builder
```

### Compile without running

```go
sql, args, err := models.Users.Where("active", true).CompileSelect()
```

---

## Read terminals

Call on a builder **or** on the entity (entity methods start a fresh builder).

| Method | Returns | Notes |
|--------|---------|--------|
| `Get(ctx, db)` | `([]T, error)` | All matching rows |
| `First(ctx, db)` | `(*T, error)` | `(nil, nil)` when missing |
| `FirstOrFail(ctx, db)` | `(*T, error)` | `query.ErrNoRows` when missing |
| `FindByID(ctx, db, id)` | `(*T, error)` | PK lookup |
| `Count(ctx, db)` | `(int64, error)` | `COUNT(*)` |
| `Exists(ctx, db)` | `(bool, error)` | `SELECT 1 … LIMIT 1` |
| `Each(ctx, db, fn)` | `error` | Streams `Get` results |
| `Pluck(ctx, db, col)` | `([]any, error)` | One column |
| `Value(ctx, db, col)` | `(any, error)` | First row, one column |
| `Sum` / `Avg` / `Min` / `Max(ctx, db, col)` | `(float64, error)` | Aggregates |

```go
u, err := models.Users.Where("email", e).First(ctx, db)
u, err := models.Users.FindByID(ctx, db, 42)
n, err := models.Users.Where("active", true).Count(ctx, db)
ok, err := models.Users.Where("email", e).Exists(ctx, db)
emails, err := models.Users.Where("active", true).Pluck(ctx, db, "email")
sum, err := models.Users.Sum(ctx, db, "age")
```

---

## Write terminals

Values are `map[string]any` keyed by **database** column names. Generated
columns (`Meta.Generated`) are omitted from inserts.

| Method | Returns | Notes |
|--------|---------|--------|
| `Create(ctx, db, values)` | `(int64, error)` | Insert; returns last id when available |
| `CreateMany(ctx, db, rows)` | `(int64, error)` | Batch insert |
| `Update(ctx, db, values)` | `(int64, error)` | Rows affected |
| `Delete(ctx, db)` | `(int64, error)` | Soft-delete if the model has `deleted_at`, else hard |
| `SoftDelete(ctx, db)` | `(int64, error)` | Sets `deleted_at` |
| `ForceDelete(ctx, db)` | `(int64, error)` | Real `DELETE` |
| `Restore(ctx, db)` | `(int64, error)` | Clears `deleted_at` |
| `Increment(ctx, db, col, amount...)` | `(int64, error)` | Default amount `1` |
| `Decrement(ctx, db, col, amount...)` | `(int64, error)` | |
| `Upsert(ctx, db, rows, uniqueCols, updateCols)` | `(int64, error)` | `ON CONFLICT` / `ON DUPLICATE KEY` |
| `FirstOrCreate(ctx, db, attrs, values...)` | `(*T, error)` | Select, then insert |
| `UpdateOrCreate(ctx, db, attrs, values)` | `(*T, error)` | Select, then update or insert |

Entity shortcuts:

```go
id, err := models.Users.Create(ctx, db, map[string]any{"email": e, "status": models.UserStatusActive})
n, err := models.Users.Where("id", id).Update(ctx, db, map[string]any{"active": false})
n, err := models.Users.SoftDelete(ctx, db, id)     // by primary key
n, err := models.Users.ForceDelete(ctx, db, id)
n, err := models.Users.Where("id", id).Restore(ctx, db)
_, err = models.Users.Where("id", id).Increment(ctx, db, "age", 1)
_, err = models.Users.Upsert(ctx, db,
	[]map[string]any{{"email": e, "name": n}},
	[]string{"email"}, []string{"name"})
row, err := models.Users.FirstOrCreate(ctx, db,
	map[string]any{"email": e},
	map[string]any{"name": n})
```

---

## Pagination and chunking

```go
page, err := models.Users.Where("active", true).OrderBy("id").
	Paginate(ctx, db, query.PageRequest{Page: 2, PerPage: 25})

page, err := models.Users.SimplePaginate(ctx, db, 2, 25)
page, err := models.Users.OrderBy("id").CursorPaginate(ctx, db, cursor, 50)
page, err := models.Users.Paginate(ctx, db, query.PageRequest{
	Style: query.PageCursor, Cursor: cursor, PerPage: 50, OrderBy: "id",
})
```

| Style | Method | Count | Mechanism |
|-------|--------|-------|-----------|
| `PageOffset` | `Paginate` (default) | yes | `LIMIT`/`OFFSET` + `COUNT(*)` |
| `PageSimple` | `SimplePaginate` | no | `LIMIT perPage+1` peek |
| `PageCursor` | `CursorPaginate` | no | keyset `WHERE id > $1` |

`PageResult[T]`:

| Field | Meaning |
|-------|---------|
| `Data` | `[]T` |
| `Style` | `"offset"` / `"simple"` / `"cursor"` |
| `PerPage`, `Page` | offset/simple |
| `Pages` / `LastPage` | offset only |
| `Total` | offset only; use `TotalCount()` (`-1` if no count) |
| `NextCursor` | cursor only |
| `HasMore` | more rows exist |

JSON tags match those names (`data`, `per_page`, `has_more`, …). The struct
marshals as an HTTP payload.

Also:

```go
.OffsetPage(ctx, db, page, perPage)
.CursorPage(ctx, db, cursor, perPage, orderBy, desc)
query.EncodeCursor(v) / query.DecodeCursor(s)
query.WithCursorValue(ctx, fn) // custom PK reader for cursor/chunk
```

Chunking is keyset, never `OFFSET`:

```go
err := models.Users.Where("active", true).ChunkByID(ctx, db, 1000, func(rows []models.User) error {
	return nil
})
err := models.Users.Chunk(ctx, db, 1000, fn)          // alias of ChunkByID
err := models.Users.LazyByID(ctx, db, 1000, func(u models.User) error { return nil })
```

---

## Relations

### Runtime (generated)

```go
users, err := models.Users.Where("active", true).
	With("posts.comments", "profile").
	WithCount("posts").
	Get(ctx, db)
```

| Kind | How it appears | Typical schema |
|------|----------------|----------------|
| belongsTo | `Author *User` | FK on this table |
| hasOne | `Profile *Profile` | unique FK on the other table |
| hasMany | `Posts []Post` | FK on the other table |
| belongsToMany | `Tags []Tag` | two-FK pivot |
| morphTo | `Commentable any` | `{name}_type` + `{name}_id` |
| morphMany | `Comments []Comment` | inverse of morphs |
| morphToMany | `Tags []Tag` | one real FK + morph pair on the pivot |
| hasManyThrough | `Comments []Comment` | A→B→C |
| hasOneOfMany | `LatestPost *Post` | latest/oldest child |

`{Rel}Count` / `{Rel}Exists` are filled by `WithCount` / `WithExists`.

Many-to-many (and morphToMany) attach API — the field is `Tags`, so the method
is `TagsRelation()`:

```go
err := post.AttachTags(ctx, db, tagID)
err := post.DetachTags(ctx, db, tagID) // no IDs → detach all
err := post.SyncTags(ctx, db, tagIDs...)
err := post.ToggleTags(ctx, db, tagIDs...)
err := post.TagsRelation().AttachWith(ctx, db, tagID, map[string]any{"pinned": true})
```

`BelongsToManyAssoc`: `Attach`, `AttachWith`, `Detach`, `Sync`, `Toggle`.
`created_at` / `updated_at` are written only when those pivot columns exist.
`ON CONFLICT` / MySQL `INSERT IGNORE` only when the FK pair is unique.
Morph pivots also bind `MorphType` (the **table name**, e.g. `"posts"`).

### Hand-written loaders

```go
query.RegisterRelation(query.Relation{
	Name: "posts", Kind: query.RelationHasMany, Table: "posts",
	LocalKey: "id", ForeignKey: "user_id",
}, func(ctx context.Context, db query.DB, rows []*User) error {
	return query.LoadHasMany(ctx, db, rows, query.HasMany[User, Post]{
		Related:    models.Posts,
		ForeignKey: "user_id",
		ParentKey:  func(u *User) any { return u.ID },
		ChildKey:   func(p *Post) any { return p.UserID },
		Assign:     func(u *User, ps []Post) { u.Posts = ps },
	})
})
```

| Loader | Kind |
|--------|------|
| `LoadHasMany` | hasMany, hasOne (assign first), morphMany (`Modify` for type) |
| `LoadBelongsTo` | belongsTo |
| `LoadBelongsToMany` | belongsToMany, morphToMany |
| `LoadMorphTo` + `MorphLoad` | morphTo |
| `LoadHasManyThrough` | hasManyThrough |
| `LoadHasOneOfMany` | latest/oldest (Postgres `DISTINCT ON`, MySQL `MAX` join) |

`query.Relations[T]()` lists what was registered for type `T`.

---

## Typed query stubs (`vorm/gen`)

Write a callable stub. Annotate it. Generate.

```go
// queries/users.go
// vorm:query name=SearchUsers
func SearchUsers(ctx context.Context, db query.DB, q string, limit int) ([]models.User, error) {
	return models.Users.WhereSearch([]string{"name", "email"}, q).
		OrderBy("name").Limit(limit).Get(ctx, db)
}
```

```bash
vorm generate
```

Output:

| File | Contents |
|------|----------|
| `vorm/gen/db.go` | `GeneratedDialect`, `GeneratedDriver` |
| `vorm/gen/users.sql.go` | one file per stub source (`admin/users.go` → `admin_users.sql.go`) |

```go
type SearchUsersRow struct { /* projected columns only */ }
type SearchUsersParams struct { Q string; Limit int }

func SearchUsers(ctx context.Context, db query.DB, arg SearchUsersParams) ([]SearchUsersRow, error)
```

```go
rows, err := gen.SearchUsers(ctx, db, gen.SearchUsersParams{Q: "ada", Limit: 20})
```

Call `gen.*` in application code. A stub that cannot be lowered is reported
(`Pending`) and stays on the runtime builder — generation never emits a function
that fails. `query.ErrGeneratePending` is unused in new output; pending stubs
are simply not generated.

### PostGIS

PostGIS is a Postgres extension, not a portable column type. Name the SQL type
yourself. Uncomment `postgis` in `migrations/extensions.sql`, then
`vorm migrate`. Generated models scan `geography` and `geometry` as `string`
(EWKB hex). A spatial stub uses `WhereRaw`; each `?` becomes `$n` in the
generated const. App code calls `gen.*` — see
[`examples/postgis`](../examples/postgis/README.md).

```go
t.CustomType("GEOGRAPHY(POINT, 4326)").Column("location")
```

```go
// queries/places.go
// vorm:query name=NearbyPlaces
func NearbyPlaces(ctx context.Context, db query.DB, lon, lat float64, meters int) ([]models.Place, error) {
	return models.Places.WhereRaw(
		"ST_DWithin(location, ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography, ?)",
		lon, lat, meters,
	).OrderBy("id").Get(ctx, db)
}

// service/places.go
func (s *Service) Nearby(ctx context.Context, lon, lat float64, meters int) ([]gen.NearbyPlacesRow, error) {
	return gen.NearbyPlaces(ctx, s.db, gen.NearbyPlacesParams{Lon: lon, Lat: lat, Meters: meters})
}
```

Lowers when the chain is a single terminal with literal columns: `Where` /
`WhereIn` / `WhereNull` / `WhereSearch` / `WhereRaw` / `OrWhere` / joins /
`GroupBy` / `Having` / `OrderBy` / `Limit` / `Offset` / `Distinct` /
`WithTrashed` / `OnlyTrashed` / locks, plus terminals `Get`, `First`,
`FirstOrFail`, `FindByID`, `Count`, `Exists`, `Paginate`, `SimplePaginate`,
`CursorPaginate`, `Create`, `Update`, `Delete`, `SoftDelete`, `ForceDelete`,
`Restore`, `Pluck`, `Sum`/`Avg`/`Min`/`Max`, `Increment`/`Decrement`.

Also lowers when names are literals: `WhereHas`, `WhereDoesntHave`,
`WhereRelation`, `WithCount`, `WithExists`, `WhereFullText`,
`WhereJsonContains`, `DistinctOn`.

Runtime-only (still parameterized SQL): `ChunkByID`, `Upsert`, `FirstOrCreate`,
`UpdateOrCreate`, `query.Transaction`.

---

## Schema Blueprint

Used in `migrations/<ts>_*.go`. `vorm migrate` compiles `Up`/`Down` to SQL and
**never executes** those functions.

```go
//go:build ignore

package migrations

import "github.com/vorzela/vorm/schema"

func Up(s *schema.Facade) {
	s.Create("posts", func(t *schema.Blueprint) {
		t.ID()
		t.String("title", 120).Unique()
		t.Text("body")
		t.Boolean("active").Default(true)
		t.Integer("age").Nullable()
		t.ForeignId("user_id").Constrained("users").CascadeOnDelete()
		t.BelongsTo("editor_id", "users") // BIGINT FK, ON DELETE CASCADE
		t.Enum("status", "draft", "published")
		t.Morphs("commentable")
		t.UUID("public_id")
		t.Timestamps()
		t.SoftDeletes()
		t.Index("title")
		t.Unique("email")
	})
	s.BelongsToMany("posts", "tags")
	s.MorphToMany("tags", "taggable")
}

func Down(s *schema.Facade) {
	s.DropIfExists("taggables")
	s.DropIfExists("post_tag")
	s.DropIfExists("posts")
}
```

Alter:

```go
func Up(s *schema.Facade) {
	s.Table("users", func(t *schema.Blueprint) {
		t.String("phone", 32)
		t.BelongsTo("manager_id", "users")
		t.Morphs("commentable")
	})
}
func Down(s *schema.Facade) {
	s.Table("users", func(t *schema.Blueprint) {
		t.DropColumn("phone")
		t.DropIndex("idx_users_manager_id")
	})
}
```

### Facade

| Method | Purpose |
|--------|---------|
| `Create(table, fn)` | `CREATE TABLE` |
| `Table(table, fn)` | `ALTER TABLE` |
| `DropIfExists` / `Drop` | drop table |
| `BelongsToMany(left, right)` | pivot `post_tag` (alphabetical singulars) |
| `MorphToMany(related, morph)` | pivot `taggables` (real FK + morph pair) |
| `CreateExtension` / `CreateEnum` / `CreateFunction` | Postgres extras (prefer declarative `*.sql` files) |

### Blueprint columns

| Method | SQL-ish result |
|--------|----------------|
| `ID()` / `Id()` | bigserial / bigint AI PK `id` |
| `BigIncrements(name)` | named AI PK |
| `UUID(name)` | Postgres `UUID DEFAULT gen_random_uuid()`; MariaDB `UUID DEFAULT UUID_v4()`; MySQL `CHAR(36)` |
| `String(name, length...)` | `VARCHAR(255)` default |
| `Text(name)` | `TEXT NULL` |
| `Boolean` / `Integer` / `BigInteger` | NOT NULL |
| `ForeignID` / `ForeignId` | `BIGINT NOT NULL` |
| `ForeignIDNullable` | nullable FK |
| `BelongsTo(col, table)` | constrained FK, cascade delete |
| `Morphs(name)` | `{name}_type` + `{name}_id` + index |
| `CustomType(sqlType).Column(name)` | extension or dialect type, written through unchanged |
| `Enum(col, values...)` | PG type / MySQL ENUM |
| `Timestamps()` | `created_at`, `updated_at` |
| `SoftDeletes()` | `deleted_at` + index |
| `Index` / `Unique` | indexes |
| `DropColumn` / `DropIndex` | alter down |
| `Raw(up, down)` | dialect SQL appended as-is |

`t.UUID` uses a random version-4 default where the server can generate one.
Collision chance for v4 is negligible; `.Primary()` or `.Unique()` makes the
database reject a duplicate if one is ever inserted.

```go
t.UUID("id").Primary()
t.UUID("public_id").Unique()
// Postgres:  UUID NOT NULL DEFAULT gen_random_uuid()   -- 13+
// MariaDB:   UUID NOT NULL DEFAULT UUID_v4()           -- type since 10.7, UUID_v4() since 11.7
// MySQL:     CHAR(36) NOT NULL                         -- UUID() is version 1; no UUID type, no v4 function
```

### Column chain

`Column(name)` (after `CustomType` only), `Primary()`, `Nullable()`, `NotNull()`, `Unique()`, `Default(v)`, `DefaultCurrent()`,
`Constrained(table)`, `References(table, column)`,
`CascadeOnDelete()`, `RestrictOnDelete()`, `NullOnDelete()`,
`CascadeOnUpdate()`.

Helpers: `schema.Singularize`, `Pluralize`, `PivotName`.

---

## Transactions

```go
err := query.Transaction(ctx, db, func(ctx context.Context, tx query.Tx) error {
	_, err := models.Users.Create(ctx, tx, values)
	return err // nil → commit; error or panic → rollback
})

err := query.TransactionOpts(ctx, db, &query.TxOptions{
	Isolation: sql.LevelSerializable,
	ReadOnly:  false,
}, fn)
```

`query.Tx` implements `query.DB`. Pass `tx` into `Get` / `Create` / etc.
`Beginner` is implemented by the Open* handles.

---

## Errors

```go
if err != nil {
	switch {
	case query.IsNotFound(err):            // FirstOrFail empty
	case query.IsUniqueViolation(err):
		_ = query.Constraint(err)
	case query.IsForeignKeyViolation(err):
	case query.IsNotNullViolation(err):
	case query.IsCheckViolation(err):
	case query.IsDeadlock(err):
	case query.IsSerializationFailure(err):
	case query.IsRetryable(err):           // deadlock / serialization / lock
	case query.IsValidationError(err):     // unknown column, bad type, bad ident
	}
	_ = query.Code(err)      // SQLSTATE or MySQL errno
	_ = query.Classify(err)  // query.Kind
}
```

| Kind | Typical cause |
|------|----------------|
| `KindNoRows` | `ErrNoRows` |
| `KindUnique` / `KindForeignKey` / `KindNotNull` / `KindCheck` | constraints |
| `KindDeadlock` / `KindSerialization` / `KindLockNotAvailable` | concurrency |
| `KindUndefinedTable` / `KindUndefinedColumn` | missing objects |
| `KindValidation` | vorm rejected the query before the driver |
| `KindConnection` / `KindTimeout` / `KindPermission` / `KindSyntax` / `KindDataException` | driver |

`First` missing → `(nil, nil)`. `FirstOrFail` missing → `query.ErrNoRows`
(`errors.Is(err, sql.ErrNoRows)` also works). Bound values are never put in
error messages.

---

## Logging

```go
query.SetDefaultLogger(query.NewSlogLogger(slog.Default()))
ctx = query.WithLogger(ctx, query.LoggerFunc(func(ctx context.Context, ev query.Event) {
	// ev.Op, Table, SQL, ArgCount, Rows, Duration, Err
	// ev.Args is nil unless you opt in — bound values often contain PII
}))
```

One `Event` per executed statement.

---

## Operators and helpers

```go
query.Eq(v)  query.Not(v)
query.MoreThan(v)  query.MoreThanOrEqual(v)
query.LessThan(v)  query.LessThanOrEqual(v)
query.Like(v)  query.ILike(v)          // ILike = Postgres
query.In(vals...)  query.NotIn(vals...)
query.IsNull()  query.IsNotNull()
query.Asc  query.Desc
```

SQL assembly (generated code and advanced stubs):

```go
query.Placeholder(dialect, n)           // $1 or ?
query.InClause(d, `"id"`, start, n)
query.NotInClause(...)
query.LikePattern(term)                 // %term%
query.PrefixPattern(term)               // term%
query.QuoteIdent(dialect, name)
query.SafeIdent / SafeOp / SafeOrderDir / SafeOnClause
```

Scanning (rarely needed; `Get` already scans into structs):

```go
row, err := query.ScanStruct[User](rows, cols)
list, err := query.ScanStructRows[User](rows)
ctx = query.WithMapper(ctx, func(rows query.Rows) (User, error) { ... })
```

`vorm.Model` is an optional embed (`ID`, timestamps, `DeletedAt`). Generated
models usually declare those fields explicitly from introspection.

---

## Config (`.vorm`)

`KEY=value`. `DATABASE_URL` from the environment always wins.

| Key | Default | Meaning |
|-----|---------|---------|
| `PACKAGE` | `gen` | generated query package name |
| `OUT_DIR` | `./vorm/<PACKAGE>` | `db.go` + `{source}.sql.go` |
| `QUERY_DIR` | `./queries` | `// vorm:query` stubs |
| `MODEL_DIR` | `./models` | generated models |
| `MODEL_PACKAGE` | `models` | Go package name |
| `MODEL_IMPORT` | from `go.mod` | import path in generated queries |
| `DRIVER` | `pgx` | `pgx` or `pq` |
| `DIALECT` | `postgres` | `postgres`, `mysql`, `mariadb` |
| `MIGRATION_PATH` / `SCHEMA_DIR` | `./migrations` | Blueprint files |
| `MODEL_SOURCE` | `db` | `db` or `blueprint` |
| `SCHEMA_NAME` | `public` | PG schema / MySQL database |
| `EMIT_RELATIONS` | `true` | loaders + relation fields |
| `EMIT_FUNCTIONS` | `true` | stored-routine wrappers |
| `INCLUDE_VIEWS` | `false` | generate models for views |
| `DATABASE_URL` | — | prefer the environment |

```bash
vorm config
vorm config set PACKAGE=vormgen
vorm config lint
```
