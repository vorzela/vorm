package query

import (
	"strings"
	"testing"
)

func TestWhereSearchCompile(t *testing.T) {
	Users := Model[struct{}](Meta{Table: "users", Columns: []string{"id", "name", "email"}})
	sql, args, err := Users.WhereSearch([]string{"name", "email"}, "ada").CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, `("name" ILIKE $1 OR "email" ILIKE $2)`) {
		t.Fatalf("sql=%s", sql)
	}
	if len(args) != 2 || args[0] != "%ada%" {
		t.Fatalf("args=%v", args)
	}
}

func TestOffsetLimitCompile(t *testing.T) {
	Users := Model[struct{}](Meta{Table: "users", Columns: []string{"id"}})
	sql, _, err := Users.New().OrderBy("id").Limit(15).Offset(30).CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "LIMIT 15") || !strings.Contains(sql, "OFFSET 30") {
		t.Fatal(sql)
	}
}

func TestWhereRawSubstitutesPlaceholders(t *testing.T) {
	Places := Model[struct{}](Meta{Table: "places", Columns: []string{"id", "location"}})
	sql, args, err := Places.WhereRaw(
		"ST_DWithin(location, ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography, ?)",
		-122.4, 37.8, 500,
	).CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "ST_MakePoint($1, $2)") || !strings.Contains(sql, ", $3)") {
		t.Fatalf("placeholders not substituted:\n%s", sql)
	}
	if strings.Contains(sql, "?") {
		t.Fatalf("raw ? leaked into SQL:\n%s", sql)
	}
	if len(args) != 3 || args[2] != 500 {
		t.Fatalf("args=%v", args)
	}

	sql, _, err = Places.WhereRaw("name = '?'").CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "name = '?'") {
		t.Fatalf("quoted ? must stay literal:\n%s", sql)
	}

	_, _, err = Places.WhereRaw("location = ?", 1, 2).CompileSelect()
	if err == nil || !strings.Contains(err.Error(), "1 ?") {
		t.Fatalf("want placeholder count error, got %v", err)
	}
}

func TestWhereFullTextAndJsonContainsSQL(t *testing.T) {
	Users := Model[struct{}](Meta{
		Table:   "users",
		Columns: []string{"id", "search_vector", "metadata"},
	})
	sql, args, err := Users.WhereFullText("search_vector", "ada & lovelace").CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if sql != `SELECT "id", "search_vector", "metadata" FROM "users" WHERE "search_vector" @@ to_tsquery('english', $1)` {
		t.Fatalf("fts: %s", sql)
	}
	if len(args) != 1 || args[0] != "ada & lovelace" {
		t.Fatalf("args=%v", args)
	}

	sql, args, err = Users.WhereJsonContains("metadata", `{"role":"admin"}`).CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if sql != `SELECT "id", "search_vector", "metadata" FROM "users" WHERE "metadata" @> $1::jsonb` {
		t.Fatalf("json: %s", sql)
	}

	Users = Model[struct{}](Meta{
		Table:   "users",
		Columns: []string{"id", "body"},
		Indexes: []IndexInfo{{Name: "users_body_fulltext", Columns: []string{"body"}, Method: "fulltext"}},
	})
	sql, _, err = Users.New().Dialect(DialectMySQL).WhereFullText("body", "ada").CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "MATCH (`body`) AGAINST (?)") {
		t.Fatalf("mysql fts: %s", sql)
	}

	sql, _, err = Users.New().Dialect(DialectMySQL).WhereJsonContains("body", `{"a":1}`).CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "JSON_CONTAINS(`body`, ?)") {
		t.Fatalf("mysql json: %s", sql)
	}
}
