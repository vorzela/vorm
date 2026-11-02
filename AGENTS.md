# AGENTS.md — vorm

Short contract for AI agents and codegen automation. Full reference:
[`LLM.md`](LLM.md), [`docs/API.md`](docs/API.md), [`docs/USAGE.md`](docs/USAGE.md).

## Golden rules

1. **Never hand-edit `models/` or `vorm/gen/`.** Regenerate with `vorm generate`.
2. **Models come from the live database.** After a schema change: `vorm migrate && vorm generate`.
3. **Application code calls `gen.*`** when a generated query exists; `queries/` stubs are the source.
4. **No `SELECT *`.** `Meta.Columns` or `.Select(...)` is the projection.
5. **Bind values, never concatenate.** Identifiers use `query.QuoteIdent`/`SafeIdent`; operators use `query.SafeOp`.
6. **Use typed columns when available:** `models.Users.Col.Email` (runtime `Where`, `OrderBy`, `SelectCols`, `GroupByCols`, map keys).
7. **Verify before shipping:** `go test ./...`, `vorm check`, and `vorm generate queries`.

## Fast path

```bash
vorm init
vorm make migration posts
# edit migrations/<ts>_posts.go
vorm migrate
vorm generate models
# edit queries/posts.go stubs
vorm generate queries
go test ./...
```

## Query builder rules

```go
// NULL is automatic.
models.Users.Where("deleted_at", nil)        // IS NULL
models.Users.Where("deleted_at", "!=", nil)  // IS NOT NULL

// Group OR explicitly. SQL AND binds tighter than OR.
models.Users.Where("active", true).
    WhereGroup(func(q *query.Builder[models.User]) {
        q.Where("role", "admin").OrWhere("role", "owner")
    })
// active = $1 AND (role = $2 OR role = $3)

// Empty IN sets are safe and explicit.
WhereIn("id", ids)                 // accepts ids... or a slice
WhereBetween("age", 18, 65)
WhereLike("name", query.LikePattern(term))

// Full-text treats user input as plain words on Postgres.
WhereFullText("search_vector", q) // plainto_tsquery('english', $1)

// Raw subqueries: prefer ? placeholders; $1..$n is also accepted.
WhereExists(`SELECT 1 FROM posts WHERE posts.user_id = users.id AND posts.state = ?`, "published")
```

`Limit(0)` means **zero rows**. Omit `Limit` for unlimited. Negative limit/offset is rejected.

Soft deletes:
- `.Delete()` stays soft when `Meta.SoftDeletes` is true, including after `.WithTrashed()`.
- `.ForceDelete()` is the only hard delete.
- `.OnlyTrashed()` requires `Meta.SoftDeletes`.
- `.WithTrashed()` only changes the read scope.

Writes:
- `Create` returns `int64`; use `CreateID` for UUID/string PKs, and check `Meta.Generated`.
- `Upsert` on MySQL uses `ON DUPLICATE KEY UPDATE`; it does not swallow unrelated errors.
- `FirstOrCreate` / `UpdateOrCreate` require a unique index on the lookup columns.

## Efficiency checklist

- Prefer `WhereIn` + `With` batches over per-row loops; relation loaders issue a bounded number of queries.
- Use `ChunkByID` / `LazyByID` for large scans (keyset, no OFFSET).
- Use `CursorPaginate` for deep pagination; order by a unique column (usually the PK).
- Use `CountDistinct` after joins when the join can duplicate parent rows.
- Use `Select("id", "email")` to avoid fetching unused columns.
- Use `Meta.Indexed(col)` / `Meta.UniqueOn(cols...)` when choosing filters and conflict targets.
- Generated SQL is static where possible; runtime slices only make `IN` lists dynamic.

## Do not

- Do not add `BEGIN`/`COMMIT` to migrations; `vorm migrate` manages transactions.
- Do not edit `migrations/extensions.sql`, `enums.sql`, or `functions.sql` outside the `vorm extensions/enums/functions` sync flow unless the change is intentional.
- Do not use `WhereRaw`/`HavingRaw` with interpolated user input.
- Do not assume `WithTrashed().Delete()` is a hard delete; use `ForceDelete`.
