package query

import (
	"strings"
	"testing"
)

func TestCompileDistinctJoinLock(t *testing.T) {
	Users := Model[struct{}](Meta{Table: "users", Columns: []string{"id", "email", "active", "role"}})
	sql, _, err := Users.New().
		Distinct().
		Select("users.id", "users.email").
		Join("posts", "posts.user_id = users.id").
		Where("users.active", true).
		OrWhere("users.role", "admin").
		OrderByDesc("users.id").
		LockForUpdate().
		CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"SELECT DISTINCT",
		`FROM "users"`,
		`INNER JOIN "posts" ON posts.user_id = users.id`,
		`"users"."active" = $1`,
		`OR "users"."role" = $2`,
		`ORDER BY "users"."id" DESC`,
		"FOR UPDATE",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("missing %q in:\n%s", want, sql)
		}
	}
}

func TestCompileDistinctOnGroupBy(t *testing.T) {
	Users := Model[struct{}](Meta{Table: "users", Columns: []string{"id", "email", "org_id"}})
	sql, _, err := Users.New().
		DistinctOn("org_id").
		Select("org_id", "id", "email").
		OrderBy("org_id").OrderByDesc("id").
		CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, `DISTINCT ON ("org_id")`) {
		t.Fatal(sql)
	}
}

func TestWhereAcceptsTypedColumn(t *testing.T) {
	Users := Model[struct{}](Meta{Table: "users", Columns: []string{"id", "email"}})
	sql, args, err := Users.New().
		Where(Col("email"), "a@b.c").
		OrWhere(Col("id"), 7).
		CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, `"email" = $1`) || !strings.Contains(sql, `OR "id" = $2`) {
		t.Fatalf("typed Col where:\n%s", sql)
	}
	if len(args) != 2 || args[0] != "a@b.c" || args[1] != 7 {
		t.Fatalf("args=%v", args)
	}
}

func TestGroupByRequiresCoveredProjection(t *testing.T) {
	Users := Model[struct{}](Meta{
		Table:      "users",
		Columns:    []string{"id", "team_id", "age"},
		PrimaryKey: "id",
	})

	_, _, err := Users.New().GroupBy("team_id").CompileSelect()
	if err == nil || !strings.Contains(err.Error(), "does not cover") {
		t.Fatalf("expected uncovered GROUP BY error, got %v", err)
	}

	sql, _, err := Users.New().Select("team_id").GroupBy("team_id").CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, `SELECT "team_id" FROM "users" GROUP BY "team_id"`) {
		t.Fatalf("narrowed group by:\n%s", sql)
	}

	// Grouping by the primary key covers the rest of that table's columns.
	sql, _, err = Users.New().GroupBy("id").CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, `SELECT "id", "team_id", "age"`) ||
		!strings.Contains(sql, `GROUP BY "id"`) {
		t.Fatalf("primary-key group by:\n%s", sql)
	}
}

func TestHavingAllowsAggregateExpressions(t *testing.T) {
	Users := Model[struct{}](Meta{Table: "users", Columns: []string{"id", "team_id", "age"}})
	sql, args, err := Users.New().
		Select("team_id").
		GroupBy("team_id").
		Having("COUNT(*)", ">", 5).
		CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, `GROUP BY "team_id"`) || !strings.Contains(sql, `HAVING COUNT(*) > $1`) {
		t.Fatalf("aggregate having:\n%s", sql)
	}
	if len(args) != 1 || args[0] != 5 {
		t.Fatalf("args=%v", args)
	}

	sql, args, err = Users.New().
		Select("team_id").
		GroupBy("team_id").
		Having("SUM(age)", ">", 100).
		CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, `HAVING SUM(age) > $1`) {
		t.Fatalf("sum having:\n%s", sql)
	}
	if len(args) != 1 || args[0] != 100 {
		t.Fatalf("args=%v", args)
	}

	_, _, err = Users.New().Having("COUNT(DROP TABLE)", ">", 1).CompileSelect()
	if err == nil {
		t.Fatal("unsafe aggregate must fail")
	}
}

func TestHavingRaw(t *testing.T) {
	Users := Model[struct{}](Meta{Table: "users", Columns: []string{"id", "team_id"}})
	sql, args, err := Users.New().
		Select("team_id").
		GroupBy("team_id").
		HavingRaw("COUNT(*) >= ? AND COUNT(*) < ?", 5, 10).
		CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, `HAVING (COUNT(*) >= $1 AND COUNT(*) < $2)`) {
		t.Fatalf("having raw:\n%s", sql)
	}
	if len(args) != 2 || args[0] != 5 || args[1] != 10 {
		t.Fatalf("args=%v", args)
	}
	_, _, err = Users.New().HavingRaw("COUNT(*); DROP TABLE users").CompileSelect()
	if err == nil {
		t.Fatal("HavingRaw must reject ;")
	}
}

func TestCompileWhereExists(t *testing.T) {
	Users := Model[struct{}](Meta{Table: "users", Columns: []string{"id"}})
	sql, args, err := Users.New().
		WhereExists("SELECT 1 FROM posts WHERE posts.user_id = users.id AND posts.status = $1", "published").
		CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "EXISTS (SELECT 1 FROM posts") {
		t.Fatal(sql)
	}
	if len(args) != 1 || args[0] != "published" {
		t.Fatalf("%v", args)
	}
}

func TestCompileWhereInSlice(t *testing.T) {
	Users := Model[struct{}](Meta{Table: "users", Columns: []string{"id"}})
	sql, args, err := Users.WhereIn("id", 1, 2, 3).CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, `"id" IN ($1, $2, $3)`) {
		t.Fatal(sql)
	}
	if len(args) != 3 {
		t.Fatal(args)
	}
}
