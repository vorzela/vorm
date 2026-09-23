package query

import (
	"context"
	"strings"
	"testing"
)

func TestPageCount(t *testing.T) {
	cases := []struct {
		total, perPage int64
		want           int
	}{
		{0, 15, 0},
		{1, 15, 1},
		{15, 15, 1},
		{16, 15, 2},
		{100, 10, 10},
		{101, 10, 11},
	}
	for _, tc := range cases {
		if got := pageCount(tc.total, int(tc.perPage)); got != tc.want {
			t.Fatalf("pageCount(%d,%d)=%d want %d", tc.total, tc.perPage, got, tc.want)
		}
	}
}

type pageUser struct {
	ID    int64  `db:"id"`
	Email string `db:"email"`
}

func TestOffsetPaginateRunsCount(t *testing.T) {
	Users := Model[pageUser](Meta{Table: "users", Columns: []string{"id", "email"}, PrimaryKey: "id"})
	db := (&fakeDB{}).
		on("COUNT(*)", []string{"count"}, []any{int64(40)}).
		on(`FROM "users"`, []string{"id", "email"},
			[]any{int64(16), "a@x.io"},
			[]any{int64(17), "b@x.io"},
		)
	page, err := Users.Paginate(context.Background(), db, PageRequest{Page: 2, PerPage: 15})
	if err != nil {
		t.Fatal(err)
	}
	st := db.statements()
	if len(st) != 2 {
		t.Fatalf("offset paginate wants 2 round-trips, got %d", len(st))
	}
	if !strings.Contains(st[0].SQL, "LIMIT 15") || !strings.Contains(st[0].SQL, "OFFSET 15") {
		t.Fatalf("page SQL: %s", st[0].SQL)
	}
	if strings.Contains(st[0].SQL, "*") && !strings.Contains(st[0].SQL, `"id"`) {
		t.Fatalf("SELECT *: %s", st[0].SQL)
	}
	if !strings.Contains(st[1].SQL, "COUNT(*)") {
		t.Fatalf("count SQL: %s", st[1].SQL)
	}
	if page.Style != string(PageOffset) || page.Pages != 3 || !page.HasMore || page.TotalCount() != 40 {
		t.Fatalf("page=%+v", page)
	}
}

func TestSimplePaginatePeeksWithoutCount(t *testing.T) {
	Users := Model[pageUser](Meta{Table: "users", Columns: []string{"id", "email"}, PrimaryKey: "id"})
	db := (&fakeDB{}).on(`FROM "users"`, []string{"id", "email"},
		[]any{int64(1), "a@x.io"},
		[]any{int64(2), "b@x.io"},
		[]any{int64(3), "c@x.io"},
	)
	page, err := Users.SimplePaginate(context.Background(), db, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if db.count() != 1 {
		t.Fatalf("simplePaginate must not COUNT, got %d queries", db.count())
	}
	got := db.statements()[0].SQL
	if !strings.Contains(got, "LIMIT 3") {
		t.Fatalf("simple SQL: %s", got)
	}
	if strings.Contains(got, "COUNT(") {
		t.Fatalf("COUNT leaked: %s", got)
	}
	if !page.HasMore || len(page.Data) != 2 || page.Style != string(PageSimple) || page.TotalCount() != -1 {
		t.Fatalf("page=%+v", page)
	}
}

func TestCursorPaginateKeysetSQL(t *testing.T) {
	Users := Model[pageUser](Meta{Table: "users", Columns: []string{"id", "email"}, PrimaryKey: "id"})
	db := (&fakeDB{}).on(`FROM "users"`, []string{"id", "email"},
		[]any{int64(10), "a@x.io"},
		[]any{int64(11), "b@x.io"},
		[]any{int64(12), "c@x.io"},
	)
	page, err := Users.OrderBy("id").CursorPaginate(context.Background(), db, "", 2)
	if err != nil {
		t.Fatal(err)
	}
	got := db.statements()[0].SQL
	want := `SELECT "id", "email" FROM "users" ORDER BY "id" ASC LIMIT 3`
	if got != want {
		t.Fatalf("first page:\n got: %s\nwant: %s", got, want)
	}
	if !page.HasMore || page.NextCursor == "" || len(page.Data) != 2 {
		t.Fatalf("page=%+v", page)
	}

	db2 := (&fakeDB{}).on(`FROM "users"`, []string{"id", "email"},
		[]any{int64(12), "c@x.io"},
	)
	page2, err := Users.OrderBy("id").CursorPaginate(context.Background(), db2, page.NextCursor, 2)
	if err != nil {
		t.Fatal(err)
	}
	got = db2.statements()[0].SQL
	want = `SELECT "id", "email" FROM "users" WHERE "id" > $1 ORDER BY "id" ASC LIMIT 3`
	if got != want {
		t.Fatalf("next page:\n got: %s\nwant: %s", got, want)
	}
	if len(db2.statements()[0].Args) != 1 || db2.statements()[0].Args[0] != int64(11) {
		t.Fatalf("cursor arg=%v", db2.statements()[0].Args)
	}
	if page2.HasMore {
		t.Fatal("last page must not peek past the end")
	}
}

func TestEncodeDecodeCursor(t *testing.T) {
	tok := EncodeCursor(int64(11))
	v, err := DecodeCursor(tok)
	if err != nil {
		t.Fatal(err)
	}
	if v != int64(11) {
		t.Fatalf("got %#v", v)
	}
	row := pageUser{ID: 42, Email: "a@x.io"}
	if CursorValue(row, "id") != int64(42) {
		t.Fatalf("default extractor: %v", CursorValue(row, "id"))
	}
}
