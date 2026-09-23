package query

import (
	"context"
	"strings"
	"testing"
)

type fastUser struct {
	ID    int64  `db:"id"`
	Email string `db:"email"`
	Age   int    `db:"age"`
}

func fastUsers() *Entity[fastUser] {
	return Model[fastUser](Meta{
		Table:      "users",
		Columns:    []string{"id", "email", "age"},
		PrimaryKey: "id",
	})
}

func TestPluckAndValueSQL(t *testing.T) {
	db := (&fakeDB{}).on(`FROM "users"`, []string{"email"}, []any{"a@x.io"}, []any{"b@x.io"})
	got, err := fastUsers().Where("age", ">", 18).Pluck(context.Background(), db, "email")
	if err != nil {
		t.Fatal(err)
	}
	sql := db.statements()[0].SQL
	want := `SELECT "email" FROM "users" WHERE "age" > $1`
	if sql != want {
		t.Fatalf("pluck SQL:\n got: %s\nwant: %s", sql, want)
	}
	if strings.Contains(sql, "*") {
		t.Fatal(sql)
	}
	if len(got) != 2 {
		t.Fatalf("pluck=%v", got)
	}

	db = (&fakeDB{}).on(`FROM "users"`, []string{"email"}, []any{"a@x.io"})
	v, err := fastUsers().Where("id", 1).Value(context.Background(), db, "email")
	if err != nil {
		t.Fatal(err)
	}
	if db.statements()[0].SQL != `SELECT "email" FROM "users" WHERE "id" = $1 LIMIT 1` {
		t.Fatalf("value SQL: %s", db.statements()[0].SQL)
	}
	if v != "a@x.io" {
		t.Fatalf("value=%v", v)
	}
}

func TestAggregateSQL(t *testing.T) {
	db := (&fakeDB{}).on("SUM", []string{"sum"}, []any{float64(21)})
	n, err := fastUsers().Where("id", ">", 0).Sum(context.Background(), db, "age")
	if err != nil {
		t.Fatal(err)
	}
	if n != 21 {
		t.Fatalf("sum=%v", n)
	}
	want := `SELECT SUM("age") FROM "users" WHERE "id" > $1`
	if db.statements()[0].SQL != want {
		t.Fatalf("sum SQL:\n got: %s\nwant: %s", db.statements()[0].SQL, want)
	}
}

func TestIncrementDecrementSQL(t *testing.T) {
	db := &fakeDB{execRes: fakeResult{affected: 1}}
	if _, err := fastUsers().Where("id", 9).Increment(context.Background(), db, "age", 2); err != nil {
		t.Fatal(err)
	}
	got := db.statements()[0]
	want := `UPDATE "users" SET "age" = "age" + $1 WHERE "id" = $2`
	if got.SQL != want {
		t.Fatalf("increment:\n got: %s\nwant: %s", got.SQL, want)
	}
	if got.Args[0] != int64(2) || got.Args[1] != 9 {
		t.Fatalf("args=%v", got.Args)
	}

	db = &fakeDB{execRes: fakeResult{affected: 1}}
	if _, err := fastUsers().Where("id", 9).Decrement(context.Background(), db, "age"); err != nil {
		t.Fatal(err)
	}
	got = db.statements()[0]
	want = `UPDATE "users" SET "age" = "age" + $1 WHERE "id" = $2`
	if got.SQL != want {
		t.Fatalf("decrement: %s", got.SQL)
	}
	if got.Args[0] != int64(-1) {
		t.Fatalf("delta=%v", got.Args[0])
	}
}

func TestUpsertSQL(t *testing.T) {
	db := &fakeDB{execRes: fakeResult{affected: 1}}
	_, err := fastUsers().Upsert(context.Background(), db,
		[]map[string]any{{"email": "a@x.io", "age": 20}},
		[]string{"email"}, []string{"age"})
	if err != nil {
		t.Fatal(err)
	}
	got := db.statements()[0].SQL
	if !strings.Contains(got, `INSERT INTO "users"`) || strings.Contains(got, "*") {
		t.Fatalf("insert: %s", got)
	}
	if !strings.Contains(got, `ON CONFLICT ("email") DO UPDATE SET "age" = EXCLUDED."age"`) {
		t.Fatalf("conflict: %s", got)
	}

	db = &fakeDB{execRes: fakeResult{affected: 1}}
	_, err = fastUsers().New().Dialect(DialectMySQL).Upsert(context.Background(), db,
		[]map[string]any{{"email": "a@x.io", "age": 20}},
		[]string{"email"}, []string{"age"})
	if err != nil {
		t.Fatal(err)
	}
	got = db.statements()[0].SQL
	if !strings.Contains(got, "ON DUPLICATE KEY UPDATE") {
		t.Fatalf("mysql upsert: %s", got)
	}
}

func TestChunkByIDKeysetSQL(t *testing.T) {
	db := (&fakeDB{}).
		on(`FROM "users"`, []string{"id", "email", "age"},
			[]any{int64(1), "a@x.io", 20},
		)
	var pages int
	err := fastUsers().Where("age", ">", 10).ChunkByID(context.Background(), db, 2, func(rows []fastUser) error {
		pages++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if pages != 1 {
		t.Fatalf("pages=%d", pages)
	}
	got := db.statements()[0].SQL
	want := `SELECT "id", "email", "age" FROM "users" WHERE "age" > $1 ORDER BY "id" ASC LIMIT 2`
	if got != want {
		t.Fatalf("chunk SQL:\n got: %s\nwant: %s", got, want)
	}
	if strings.Contains(got, "OFFSET") {
		t.Fatal("ChunkByID must not OFFSET")
	}
}

func TestOnlyTrashedSQL(t *testing.T) {
	Users := Model[fastUser](Meta{
		Table: "users", Columns: []string{"id", "email", "age", "deleted_at"}, SoftDeletes: true,
	})
	sql, _, err := Users.OnlyTrashed().CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if sql != `SELECT "id", "email", "age", "deleted_at" FROM "users" WHERE "deleted_at" IS NOT NULL` {
		t.Fatalf("onlyTrashed: %s", sql)
	}
}

func TestWhereHasAndWithCountSQL(t *testing.T) {
	type u struct {
		ID         int64 `db:"id"`
		PostsCount int64 `db:"posts_count"`
		PostsExist bool  `db:"posts_exists"`
	}
	Users := Model[u](Meta{Table: "users", Columns: []string{"id"}, PrimaryKey: "id"})
	RegisterRelation(Relation{
		Name: "posts", Kind: RelationHasMany, Table: "posts",
		LocalKey: "id", ForeignKey: "user_id",
	}, func(ctx context.Context, db DB, parents []*u) error { return nil })

	sql, args, err := Users.WhereHas("posts").WhereRelation("posts", "state", "published").CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, `EXISTS (SELECT 1 FROM "posts" WHERE "posts"."user_id" = "users"."id"`) {
		t.Fatalf("whereHas: %s", sql)
	}
	if !strings.Contains(sql, `"posts"."state" = $1`) {
		t.Fatalf("whereRelation: %s", sql)
	}
	if len(args) != 1 || args[0] != "published" {
		t.Fatalf("args=%v", args)
	}

	sql, _, err = Users.WithCount("posts").CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, `(SELECT COUNT(*) FROM "posts" WHERE "posts"."user_id" = "users"."id") AS "posts_count"`) {
		t.Fatalf("withCount: %s", sql)
	}
	if strings.Contains(sql, "*") && !strings.Contains(sql, "COUNT(*)") {
		t.Fatalf("SELECT *: %s", sql)
	}

	sql, _, err = Users.WithExists("posts").CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, `AS "posts_exists"`) || !strings.Contains(sql, "SELECT EXISTS") {
		t.Fatalf("withExists: %s", sql)
	}

	sql, _, err = Users.WhereDoesntHave("posts").CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "NOT EXISTS") {
		t.Fatalf("doesntHave: %s", sql)
	}
}

func TestFirstOrCreateSelectThenInsert(t *testing.T) {
	db := (&fakeDB{}).
		on("RETURNING", []string{"id"}, []any{int64(7)}).
		on(`FROM "users"`, []string{"id", "email", "age"})
	_, err := fastUsers().FirstOrCreate(context.Background(), db, map[string]any{"email": "a@x.io"})
	if err != nil {
		t.Fatal(err)
	}
	st := db.statements()
	if len(st) < 2 {
		t.Fatalf("want SELECT then INSERT, got %d", len(st))
	}
	if !strings.Contains(st[0].SQL, `SELECT "id", "email", "age" FROM "users" WHERE "email" = $1`) || !strings.Contains(st[0].SQL, "LIMIT 1") {
		t.Fatalf("select: %s", st[0].SQL)
	}
	if !strings.Contains(st[1].SQL, `INSERT INTO "users"`) {
		t.Fatalf("insert: %s", st[1].SQL)
	}
}
