# Example generated output (no sqlc binary — vorm owns codegen)

| Sample | Maps to |
|--------|---------|
| `models/*.go` | `./models/` — **DO NOT EDIT** |
| `vorm/gen/db.go` | dialect/driver constants |
| `vorm/gen/{source}.sql.go` | sqlc-style `*Row` / `*Params` + SQL, one file per stub source |

```go
rows, err := gen.ListActiveAdults(ctx, db) // []ListActiveAdultsRow
u, err := gen.GetUserByEmail(ctx, db, gen.GetUserByEmailParams{Email: "a@b.c"})
u, err := gen.UserByID(ctx, db, gen.UserByIDParams{Id: 42})
```
