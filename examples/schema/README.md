# Schema + models from CLI

```bash
vorm make migration posts
vorm make migration post_tag
```

Creates numbered Blueprint files in `migrations/`:

| File | Purpose |
|------|---------|
| `migrations/<ts>_create_posts_table.go` | `Up`/`Down` with `s.Create("posts", …)` |
| `migrations/<ts>_create_post_tag_table.go` | `s.BelongsToMany("posts", "tags")` |

Edit columns, then:

```bash
vorm migrate
vorm generate models
```

## Workflow

1. **Edit** the Blueprint (columns, enums, FKs, morphs):

```go
func Up(s *schema.Facade) {
	s.Create("posts", func(t *schema.Blueprint) {
		t.ID()
		t.String("title")
		t.Enum("status", "draft", "published", "archived")
		t.ForeignId("user_id").Constrained("users").CascadeOnDelete()
		t.Timestamps()
		t.SoftDeletes()
	})
}
```

2. **Apply:** `vorm migrate` compiles `Up`/`Down` to SQL (the functions are never executed).

3. **Refresh models** from the database (relations, attach/sync, nested `.With`):

```bash
vorm generate models
```

4. **Write queries** in `queries/*.go`, then `vorm generate`.

Hand-written Facade examples remain in this folder for reference.
