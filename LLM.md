# vorm (Go) — agent guide

## Hard rules

1. **Never hand-edit `models/` or `vorm/gen/`.** Regenerate: `vorm generate`.
2. **Models come from the live database.** After changing SQL: `vorm migrate && vorm generate`.
3. **Call `gen.*` in application code.** The stubs in `queries/` are the source for codegen; they stay callable, but the generated function is the fast path.
4. **Annotation is `// vorm:query name=FuncName`** — one space after `//`, above a function whose body is a single builder chain ending in a terminal call.
5. **No `SELECT *`.** Projection comes from `Meta.Columns` or `.Select(...)`.
6. **Bind values, never concatenate.** Identifiers go through `SafeIdent`, operators through `SafeOp`.
7. **Columns must exist on `Meta`** and the value's Go type must match the model field.
8. **No sqlc, no external migration binary.** `vorm` does both.

## Workflow

```bash
vorm make migration posts   # timestamped Blueprint Go in migrations/
vorm make belongs-to posts users
vorm make belongs-to-many posts tags
vorm make morphs comments commentable
# edit migrations/<ts>_*.go
vorm migrate                # compile Blueprint, lint, lock, apply, checksum
vorm generate               # models from the DB, then typed queries
```

`vorm generate models` alone regenerates only `models/`. `vorm introspect --json`
shows what vorm sees in the database.

## What lowers to static SQL

`Where` (incl. operator helpers), `WhereIn` / `WhereNotIn`, `WhereNull` /
`WhereNotNull`, `WhereSearch`, `WhereRaw`, `OrWhere`, `Join` / `LeftJoin`,
`GroupBy`, `Having`, `OrderBy`, `Limit`, `Offset`, `Distinct`, `WithTrashed`,
locks, and the terminals `Get`, `First`, `FirstOrFail`, `FindByID`, `Count`, `Exists`,
`Paginate`, `SimplePaginate`, `CursorPaginate`, `Create`, `Update`, `Delete`,
`SoftDelete`, `ForceDelete`, `Restore`, `Pluck`, `Sum` / `Avg` / `Min` / `Max`,
`Increment` / `Decrement`, `OnlyTrashed`.

Anything else is reported as pending with a reason and keeps running through the
builder. Pending stubs are never emitted as broken functions. Runtime-only
terminals (`ChunkByID`, `Upsert`, `FirstOrCreate`, `UpdateOrCreate`) still emit
parameterized SQL when called on the builder.

`WhereHas` / `WhereDoesntHave` / `WhereRelation`, `WithCount` / `WithExists`,
`WhereFullText`, `WhereJsonContains`, and `DistinctOn` also lower when names and
columns are literals.

## Generated shape

- `vorm/gen/db.go` — `GeneratedDialect` / `GeneratedDriver`
- `vorm/gen/{source}.sql.go` — one file per `queries/` stub file (`users.go` → `users.sql.go`; nested dirs become `admin_users.sql.go`)
- `FuncNameRow` — exactly the projected columns, with model types (enums are qualified, e.g. `models.UserStatus`)
- `FuncNameParams` — bound arguments, when the stub takes any
- The function itself: a SQL const when the shape is fixed, `strings.Builder` assembly only when a variable-length `IN` requires it
- `scanFuncNameRow` aligned to the projection
- Relation loaders live on the table model (`models/user_gen.go`), not a separate `relations_gen.go`

## Errors

```go
query.IsUniqueViolation(err) / IsForeignKeyViolation / IsNotNullViolation /
IsCheckViolation / IsDeadlock / IsSerializationFailure / IsRetryable / IsNotFound
query.Code(err)       // SQLSTATE or errno
query.Constraint(err) // constraint name
query.Classify(err)   // query.Kind
```

`First` returns `(nil, nil)` when nothing matches; `FirstOrFail` returns
`query.ErrNoRows`.

## Config

`.vorm`, `KEY=value`. `DATABASE_URL` from the environment always wins.

Keys: `PACKAGE`, `OUT_DIR`, `DRIVER`, `DIALECT`, `MIGRATION_PATH`,
`MODEL_SOURCE`, `MODEL_DIR`, `MODEL_PACKAGE`, `MODEL_IMPORT`, `QUERY_DIR`,
`SCHEMA_DIR`, `SCHEMA_NAME`, `EMIT_RELATIONS`, `EMIT_FUNCTIONS`.

## PostgreSQL prerequisites

`migrations/extensions.sql`, `enums.sql`, `functions.sql` are declarative:
uncommented `CREATE` = enabled, commented = disabled. Sync with `vorm extensions`,
`vorm enums`, `vorm functions`; add `--drop-disabled` to remove what was
commented out. They also run automatically before `vorm migrate` when changed.

Prefer editing these files over writing a migration for an extension or enum;
use `vorm make enum` / `vorm make extension` only when the change must be
versioned in a specific migration.

## Anti-patterns

- Editing generated files, then wondering why the next `vorm generate` reverts them
- `Where("actve", …)` — fails the column check
- `Where("active", "yes")` on a bool column — fails the type check
- Adding a column in SQL and not rerunning `vorm generate`

## Relations

`With("posts.comments")` is nested eager load (one batched `IN` query per path
segment). Pivot tables with extra columns are still many-to-many. Generated
helpers are `TagsRelation().Attach/Detach/Sync/Toggle` and `AttachTags` — Go
cannot reuse the `Tags` field as a method. `t.Morphs("commentable")` → morphTo on the child and morphMany on other tables;
`commentable_type` stores the table name. A join table with one real FK plus
`{name}_type`/`{name}_id` is morphToMany. `hasManyThrough` is planned when A
hasMany B and B hasMany C; `latest_*` / `oldest_*` are hasOneOfMany (Postgres
`DISTINCT ON`, MySQL `MAX`/`MIN` join).
