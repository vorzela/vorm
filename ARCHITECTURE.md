# vorm architecture

## Layout

```
.              ← Go implementation (github.com/vorzela/vorm)
  cmd/vorm/    ← the CLI
  migrate/     ← in-process migration runner
  introspect/  ← PostgreSQL / MySQL schema reader
  generate/    ← models, enums, relations, typed queries
  query/       ← runtime builder, scanning, relations, errors
  schema/      ← Laravel-style Blueprint → SQL
  config/      ← .vorm project settings
  lint/        ← migration linter
  scaffold/    ← vorm make
  vmtool/      ← optional shell-out to the vm binary
typescript/    ← planned
python/        ← planned
```

Languages share the product ideas (declarative schema, `vorm:query` stubs
lowered to parameterized SQL). Runtimes and emitters stay per-language; there is
no single binary spanning all of them.

## Goals

- Laravel-like **authoring**: schema builder, fluent queries
- **Runtime** is parameterized SQL through `query.DB` and a real driver
  (pgx v5, lib/pq, MySQL) — no ORM-invented query language, no sqlc
- **Codegen-first**: models come from the live database, queries from stubs
- **Self-contained**: migrations run in-process; the `vm` binary is optional
- **Security**: bind every value, validate and quote identifiers, never
  `SELECT *`

## Query pipeline

```
  // vorm:query stub (fluent builder chain)   OR   Users.Where(...).Get(ctx, db)
           │                                            │
           ▼                                            ▼
  parse (go/ast) → StubFunc IR                   compile at runtime
           │                                            │
           ▼                                            ▼
  plan → segments (text | placeholder | IN)       parameterized SQL
           │
           ▼
  emit → vorm/gen: Row + Params + SQL const (or strings.Builder when dynamic)
```

A stub that cannot be lowered — options decided at runtime, transaction
plumbing — is reported with a reason and keeps running through the builder.
Generation never emits a function that fails at runtime.

## Model pipeline

```
  live database
        │
        ▼
  introspect → Schema{Tables, Columns, Indexes, ForeignKeys, Enums, Functions}
        │
        ▼
  generate → models/: structs, enum types, relation loaders, function wrappers
```

The blueprint path (`schema/migrations/*.go`) is a fallback for when no database
is reachable; introspection is the source of truth because it is the only one
that knows nullability, real enum values and actual indexes.

## Migration pipeline

```
  vorm make migration …   OR   schema.Facade.Create(...)
           │
           ▼
  migrations/*.sql  (+ declarative extensions.sql / enums.sql / functions.sql)
           │
           ▼
  migrate.Runner: lint → lock → transaction → checksum → tracking table
```

## Relationship to Vorzela Migrate (`vm`)

| Piece | Responsibility |
|-------|----------------|
| `vm` | Migrations, schema drift detection, online/zero-downtime DDL |
| `vorm` | Migrations, schema DSL, introspection, codegen, runtime data layer |

The two overlap on migrations by design. vorm's runner uses the same file
format, `migrations` tracking table, checksums, batches and locks, so a
directory works with either tool. vorm does not import `vm`'s packages and does
not require its binary; `RUNNER=vm` shells out to it for teams that want drift
detection or online DDL, which vorm does not implement.
