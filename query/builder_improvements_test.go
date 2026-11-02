package query

import (
	"context"
	"strings"
	"testing"
)

func TestWhereNullFromNilValue(t *testing.T) {
	Users := Model[userRow](Meta{Table: "users", Columns: []string{"id", "deleted_at"}})
	sql, _, err := Users.Where("deleted_at", nil).CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, `"deleted_at" IS NULL`) {
		t.Fatalf("nil should compile to IS NULL: %s", sql)
	}
	sql, _, err = Users.Where("deleted_at", "!=", nil).CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, `"deleted_at" IS NOT NULL`) {
		t.Fatalf("!= nil should compile to IS NOT NULL: %s", sql)
	}
}

func TestWhereGroupParenthesizesOr(t *testing.T) {
	Users := Model[userRow](Meta{Table: "users", Columns: []string{"id", "active", "name"}})
	sql, args, err := Users.
		Where("active", true).
		WhereGroup(func(q *Builder[userRow]) {
			q.Where("name", "ada").OrWhere("name", "grace")
		}).
		CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	want := `SELECT "id", "active", "name" FROM "users" WHERE "active" = $1 AND ("name" = $2 OR "name" = $3)`
	if sql != want {
		t.Fatalf("group SQL:\n got: %s\nwant: %s", sql, want)
	}
	if len(args) != 3 {
		t.Fatalf("args=%v", args)
	}
}

func TestWhereBetweenAndInSlice(t *testing.T) {
	Users := Model[userRow](Meta{Table: "users", Columns: []string{"age", "id"}})
	sql, args, err := Users.
		WhereBetween("age", 18, 65).
		WhereIn("id", []int64{1, 2}).
		CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, `"age" BETWEEN $1 AND $2`) || !strings.Contains(sql, `"id" IN ($3, $4)`) {
		t.Fatalf("sql=%s", sql)
	}
	if len(args) != 4 {
		t.Fatalf("args=%v", args)
	}
}

func TestLimitZeroAndNegative(t *testing.T) {
	Users := Model[userRow](Meta{Table: "users", Columns: []string{"id"}})
	sql, _, err := Users.Limit(0).CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(sql, " LIMIT 0") {
		t.Fatalf("Limit(0) must emit LIMIT 0: %s", sql)
	}
	if _, _, err := Users.Limit(-1).CompileSelect(); err == nil {
		t.Fatal("negative Limit must be rejected")
	}
}

func TestDeleteWithTrashedStillSoftDeletes(t *testing.T) {
	Users := Model[userRow](Meta{
		Table: "users", Columns: []string{"id", "deleted_at"}, SoftDeletes: true, PrimaryKey: "id",
	})
	db := &fakeDB{execRes: fakeResult{affected: 1}}
	if _, err := Users.WithTrashed().Where("id", 1).Delete(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	got := db.statements()[0].SQL
	if !strings.HasPrefix(got, `UPDATE "users" SET "deleted_at"`) {
		t.Fatalf("WithTrashed.Delete must remain soft: %s", got)
	}
}

func TestSoftDeleteRequiresSoftMeta(t *testing.T) {
	Users := Model[userRow](Meta{Table: "users", Columns: []string{"id"}})
	db := &fakeDB{execRes: fakeResult{affected: 1}}
	if _, err := Users.New().SoftDelete(context.Background(), db); err == nil {
		t.Fatal("SoftDelete on a non-soft table must fail")
	}
}

func TestWhereExistsBindsQuestionMarks(t *testing.T) {
	Users := Model[userRow](Meta{Table: "users", Columns: []string{"id"}})
	sql, args, err := Users.
		WhereExists("SELECT 1 FROM posts WHERE posts.user_id = users.id AND posts.status = ?", "published").
		CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "posts.status = $1") || len(args) != 1 || args[0] != "published" {
		t.Fatalf("sql=%s args=%v", sql, args)
	}
}

func TestCreateIDSupportsStringPrimaryKey(t *testing.T) {
	type row struct {
		ID   string `db:"id"`
		Name string `db:"name"`
	}
	Rows := Model[row](Meta{Table: "things", Columns: []string{"id", "name"}, PrimaryKey: "id"})
	db := (&fakeDB{}).on("RETURNING", []string{"id"}, []any{"uuid-1"})
	id, err := Rows.New().CreateID(context.Background(), db, map[Col]any{"name": "x"})
	if err != nil {
		t.Fatal(err)
	}
	if id != "uuid-1" {
		t.Fatalf("id=%#v", id)
	}
	if _, err := Rows.New().Create(context.Background(), db, map[Col]any{"name": "x"}); err == nil {
		t.Fatal("Create should reject a non-integer PK and point at CreateID")
	}
}

func TestApplyFindDeterministicOrder(t *testing.T) {
	Users := Model[userRow](Meta{Table: "users", Columns: []string{"id", "name", "active", "age"}})
	opts := FindOptions{
		Where: WhereMap{"active": true, "age": MoreThan(18)},
		Order: OrderMap{"name": Asc, "id": Desc},
	}
	first, _, err := Users.New().ApplyFind(opts).CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		again, _, err := Users.New().ApplyFind(opts).CompileSelect()
		if err != nil {
			t.Fatal(err)
		}
		if again != first {
			t.Fatalf("nondeterministic SQL:\n%s\n%s", first, again)
		}
	}
}

func TestWhereSearchQualifiesJoinedColumns(t *testing.T) {
	Users := Model[userRow](Meta{Table: "users", Columns: []string{"id", "name"}})
	sql, _, err := Users.Join("posts", "posts.user_id = users.id").
		WhereSearch([]string{"name"}, "ada").CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, `"users"."name" ILIKE $1`) {
		t.Fatalf("search columns must be qualified: %s", sql)
	}
}

func TestCountGroupedRejected(t *testing.T) {
	Users := Model[userRow](Meta{Table: "users", Columns: []string{"id", "active"}})
	db := &fakeDB{}
	if _, err := Users.New().GroupBy("active").Count(context.Background(), db); err == nil {
		t.Fatal("Count+GroupBy must return a validation error")
	}
}

func TestCreateIDMySQLStillInserts(t *testing.T) {
	type row struct {
		ID   string `db:"id"`
		Name string `db:"name"`
	}
	Rows := Model[row](Meta{Table: "things", Columns: []string{"id", "name"}, PrimaryKey: "id"})
	db := &fakeDB{execRes: fakeResult{affected: 1}}
	id, err := Rows.New().Dialect(DialectMySQL).CreateID(context.Background(), db, map[Col]any{"id": "uuid-1", "name": "x"})
	if err != nil {
		t.Fatal(err)
	}
	if id != "uuid-1" {
		t.Fatalf("id=%v", id)
	}
	if db.count() != 1 || !strings.HasPrefix(db.statements()[0].SQL, "INSERT INTO `things`") {
		t.Fatalf("expected one insert, got %+v", db.statements())
	}
}
