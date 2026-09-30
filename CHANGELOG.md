# Changelog

All notable changes to [vorm](https://github.com/vorzela/vorm) are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.10] — 2026-09-30

### Added

- CLI loads `.env` from the working directory (does not override existing env vars),
  so `DATABASE_URL` in `.env` works with `vorm generate` / `vorm migrate`.

## [0.2.9] — 2026-09-30

### Fixed

- Avoid inefficient `WriteString` concatenation when qualifying model types.
- Remove unused private helpers (dead emit/query/scaffold code) flagged by staticcheck.

## [0.2.8] — 2026-09-30

### Added

- `vorm check` / `vorm generate queries` verify `models/` against the live
  database schema (tables, columns, Go types) when `DATABASE_URL` is set.
- `vorm make migration add_<column>_to_<table>` scaffolds real
  `t.String("<column>")` / `t.DropColumn` Blueprint alters (default type
  `String`). `drop_<column>_from_<table>` is supported too.

## [0.2.7] — 2026-09-30

### Changed

- Project config prefers `vorm.yaml` / `vorm.yml` (sqlc-style); `vorm init`
  writes YAML. Legacy `.vorm` KEY=value still loads when no YAML is present.
- Generated model/row `json` tags default to lowercase snake_case.
- README: when to use vorm vs [vm](https://github.com/vorzela/vorzela-migrate)
  vs sqlc; docs list every `vorm.yaml` key.

### Fixed

- Wire `throughFarKey` into has-many-through EXISTS joins.

## [0.2.6] — 2026-09-30

### Fixed

- Avoid inefficient string concatenation when quoting morph type literals in relation SQL.

## [0.2.5] — 2026-09-30

### Changed

- `FirstOrCreate` / `UpdateOrCreate` require a unique index on the attrs columns.
  `vorm generate` fails when a stub uses them without one (attrs must be a map
  literal so columns can be checked against `Meta.Indexes`). The runtime builder
  enforces the same gate.

## [0.2.4] — 2026-09-30

### Fixed

- Runtime-builder `FirstOrCreate` / `UpdateOrCreate` (not lowered to `vorm/gen`) recover
  from a concurrent unique violation by re-selecting (then updating, for
  `UpdateOrCreate`). A unique index on the lookup columns is still required.
  Generated stubs should keep using `Create` / `Update` or hand-written
  `ON CONFLICT` SQL.

## [0.2.3] — 2026-09-30

### Added

- `HavingRaw(fragment, args...)` for bound HAVING fragments (mirrors `WhereRaw`).
- `Having` accepts safe aggregate expressions (`COUNT(*)`, `SUM(col)`, …), not only Meta columns.

### Fixed

- `From()` no longer defaults `SoftDeletes: true` (ad-hoc queries no longer filter on a missing `deleted_at`).
- `Restore()` (runtime and generated) only updates rows with `deleted_at IS NOT NULL`, avoiding unbounded full-table restores.
- Morph `Attach` `ON CONFLICT` includes `MorphTypeColumn` when set; morph pivot planning treats a unique `(parent, related, type)` index as `UniquePair`.

## [0.2.2] — 2026-09-30

### Added

- sqlc-style `emit_sql_as_comment`: generated query functions can include the SQL as a block comment above the function.
- Config via `vorm.yaml` / `vorm.yml` (`gen.go.emit_sql_as_comment`) or `.vorm` `EMIT_SQL_AS_COMMENT=true`.

```yaml
version: "1"
gen:
  go:
    emit_sql_as_comment: true
```

## [0.2.1] — 2026-09-23

### Added

- `t.UUID(name)` with a random version-4 default where the server can generate one:
  - Postgres: `UUID DEFAULT gen_random_uuid()`
  - MariaDB: `UUID DEFAULT UUID_v4()`
  - MySQL: `CHAR(36)` (no server v4 default)
- `Column.Primary()` for non-serial primary keys (including UUID).
- `.Unique()` on UUID (and other) columns so the database rejects duplicate inserts.

## [0.2.0] — 2026-09-23

### Added

- sqlc-style generated layout: `vorm/gen/db.go` plus one `{source}.sql.go` per stub file.
- `CustomType(sqlType).Column(name)` for extension / dialect types (PostGIS, citext, …).
- `vorm make belongs-to|has-one|has-many|belongs-to-many|morphs|morph-to-many` Blueprint scaffolds.
- PostGIS example under `examples/postgis/` (`WhereRaw` + `ST_DWithin`, service calls `gen.*`).
- `docs/API.md` catalog.

### Changed

- PostGIS `geography` / `geometry` map to Go `string` (EWKB hex).
- `WhereRaw` rewrites `?` placeholders into `$n` / `?` in order so generated SQL is a const.
- Entity handles that would collide with the model type (e.g. `post_tag` → `PostTags`) are pluralized.
- Introspection skips extension-owned tables and routines (e.g. PostGIS).

## [0.1.0] — 2026-09-22

### Added

- Laravel-style numbered Blueprint migrations compiled to SQL in-process.
- Eloquent-style relations: nested `With`, belongs-to-many attach/detach/sync/toggle, morphs.

## [0.0.1] — 2026-09-21

### Added

- First standalone release: in-process migrations, database-introspected models, and `// vorm:query` stubs lowered to parameterized sqlc-style Go.

```
go install github.com/vorzela/vorm/cmd/vorm@v0.2.10
```

[Unreleased]: https://github.com/vorzela/vorm/compare/v0.2.10...HEAD
[0.2.10]: https://github.com/vorzela/vorm/compare/v0.2.9...v0.2.10
[0.2.9]: https://github.com/vorzela/vorm/compare/v0.2.8...v0.2.9
[0.2.8]: https://github.com/vorzela/vorm/compare/v0.2.7...v0.2.8
[0.2.7]: https://github.com/vorzela/vorm/compare/v0.2.6...v0.2.7
[0.2.6]: https://github.com/vorzela/vorm/compare/v0.2.5...v0.2.6
[0.2.5]: https://github.com/vorzela/vorm/compare/v0.2.4...v0.2.5
[0.2.4]: https://github.com/vorzela/vorm/compare/v0.2.3...v0.2.4
[0.2.3]: https://github.com/vorzela/vorm/compare/v0.2.2...v0.2.3
[0.2.2]: https://github.com/vorzela/vorm/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/vorzela/vorm/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/vorzela/vorm/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/vorzela/vorm/compare/v0.0.1...v0.1.0
[0.0.1]: https://github.com/vorzela/vorm/releases/tag/v0.0.1
