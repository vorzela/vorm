package query

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
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

func TestFromDoesNotAssumeSoftDeletes(t *testing.T) {
	sql, _, err := From[fastUser]("items", "id", "name").CompileSelect()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sql, "deleted_at") {
		t.Fatalf("From must not filter deleted_at by default: %s", sql)
	}
	want := `SELECT "id", "name" FROM "items"`
	if sql != want {
		t.Fatalf("got %q want %q", sql, want)
	}
}

func TestRestoreTargetsOnlyTrashed(t *testing.T) {
	Users := Model[fastUser](Meta{
		Table: "users", Columns: []string{"id", "email", "age", "deleted_at"}, SoftDeletes: true,
	})
	db := &fakeDB{execRes: fakeResult{affected: 2}}
	n, err := Users.New().Restore(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("affected=%d", n)
	}
	got := db.statements()[0].SQL
	want := `UPDATE "users" SET "deleted_at" = NULL WHERE "deleted_at" IS NOT NULL`
	if got != want {
		t.Fatalf("restore:\n got: %s\nwant: %s", got, want)
	}

	db2 := &fakeDB{execRes: fakeResult{affected: 1}}
	_, err = Users.Where("id", 3).Restore(context.Background(), db2)
	if err != nil {
		t.Fatal(err)
	}
	got = db2.statements()[0].SQL
	want = `UPDATE "users" SET "deleted_at" = NULL WHERE "id" = $1 AND "deleted_at" IS NOT NULL`
	if got != want {
		t.Fatalf("restore with where:\n got: %s\nwant: %s", got, want)
	}
}

// TestRestoreNoWhereOnlyFlipsTrashed is the mass-update regression: Restore() with
// no other WHERE on a mixed table must clear deleted_at only on trashed rows.
func TestRestoreNoWhereOnlyFlipsTrashed(t *testing.T) {
	type softUser struct {
		ID        int64  `db:"id"`
		Email     string `db:"email"`
		DeletedAt *int   `db:"deleted_at"`
	}
	Users := Model[softUser](Meta{
		Table: "users", Columns: []string{"id", "email", "deleted_at"}, SoftDeletes: true,
	})
	db := &softRestoreDB{rows: []softRestoreRow{
		{id: 1, trashed: false},
		{id: 2, trashed: true},
		{id: 3, trashed: false},
		{id: 4, trashed: true},
		{id: 5, trashed: false},
	}}
	beforeLive := map[int64]bool{1: true, 3: true, 5: true}

	n, err := Users.New().Restore(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("affected=%d want 2 (only trashed); SQL=%s", n, db.sql)
	}
	if db.sql != `UPDATE "users" SET "deleted_at" = NULL WHERE "deleted_at" IS NOT NULL` {
		t.Fatalf("restore SQL:\n got: %s", db.sql)
	}
	for _, r := range db.rows {
		if beforeLive[r.id] {
			if r.trashed {
				t.Fatalf("live row %d became trashed", r.id)
			}
			continue
		}
		if r.trashed {
			t.Fatalf("trashed row %d was not restored", r.id)
		}
	}
}

type softRestoreRow struct {
	id      int64
	trashed bool
}

// softRestoreDB applies Restore-style UPDATEs against an in-memory soft-delete table.
// Without "deleted_at IS NOT NULL", a bare UPDATE matches every row (the pre-fix bug).
type softRestoreDB struct {
	rows []softRestoreRow
	sql  string
}

func (d *softRestoreDB) QueryContext(context.Context, string, ...any) (Rows, error) {
	return nil, fmt.Errorf("softRestoreDB: QueryContext unused")
}
func (d *softRestoreDB) QueryRowContext(context.Context, string, ...any) Row {
	return errRow{fmt.Errorf("softRestoreDB: QueryRowContext unused")}
}

func (d *softRestoreDB) ExecContext(_ context.Context, sqlText string, args ...any) (Result, error) {
	d.sql = sqlText
	if !strings.Contains(sqlText, `SET "deleted_at" = NULL`) {
		return nil, fmt.Errorf("unexpected SQL: %s", sqlText)
	}
	onlyTrashed := strings.Contains(sqlText, `"deleted_at" IS NOT NULL`)
	var idFilter *int64
	if strings.Contains(sqlText, `"id" =`) && len(args) > 0 {
		if v, ok := args[0].(int64); ok {
			idFilter = &v
		}
	}
	var n int64
	for i := range d.rows {
		r := &d.rows[i]
		if idFilter != nil && r.id != *idFilter {
			continue
		}
		if onlyTrashed && !r.trashed {
			continue
		}
		// No soft filter and no WHERE ⇒ match all rows (bug shape).
		if !onlyTrashed && !strings.Contains(sqlText, "WHERE") {
			r.trashed = false
			n++
			continue
		}
		if !onlyTrashed && idFilter != nil {
			r.trashed = false
			n++
			continue
		}
		if onlyTrashed {
			r.trashed = false
			n++
		}
	}
	return fakeResult{affected: n}, nil
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

func TestFirstOrCreateReturnsExisting(t *testing.T) {
	db := (&fakeDB{}).on(`FROM "users"`, []string{"id", "email", "age"}, []any{int64(3), "a@x.io", 21})
	row, err := fastUsers().FirstOrCreate(context.Background(), db, map[string]any{"email": "a@x.io"})
	if err != nil {
		t.Fatal(err)
	}
	if row == nil || row.ID != 3 {
		t.Fatalf("row=%+v", row)
	}
	for _, st := range db.statements() {
		if strings.Contains(st.SQL, "INSERT") {
			t.Fatalf("must not insert when row exists: %s", st.SQL)
		}
	}
}

func TestFirstOrCreateRecoversOnUniqueViolation(t *testing.T) {
	db := &orCreateRaceDB{email: "a@x.io"}
	row, err := fastUsers().FirstOrCreate(context.Background(), db, map[string]any{"email": "a@x.io"})
	if err != nil {
		t.Fatal(err)
	}
	if row == nil || row.ID != 7 || row.Email != "a@x.io" {
		t.Fatalf("row=%+v", row)
	}
	if db.selects < 2 {
		t.Fatalf("want re-select after conflict, selects=%d", db.selects)
	}
	if !db.inserted {
		t.Fatal("expected insert attempt")
	}
}

func TestFirstOrCreateSurfacesNonUniqueInsertError(t *testing.T) {
	db := &orCreateRaceDB{email: "a@x.io", insertErr: &pgconn.PgError{Code: "23503", ConstraintName: "users_team_fkey"}}
	_, err := fastUsers().FirstOrCreate(context.Background(), db, map[string]any{"email": "a@x.io"})
	if err == nil {
		t.Fatal("expected foreign-key error")
	}
	if IsUniqueViolation(err) || !IsForeignKeyViolation(err) {
		t.Fatalf("want FK violation, got %v (kind=%s)", err, Classify(err))
	}
}

func TestUpdateOrCreateRecoversOnUniqueViolation(t *testing.T) {
	db := &orCreateRaceDB{email: "a@x.io"}
	row, err := fastUsers().UpdateOrCreate(context.Background(), db,
		map[string]any{"email": "a@x.io"},
		map[string]any{"age": 30},
	)
	if err != nil {
		t.Fatal(err)
	}
	if row == nil || row.ID != 7 {
		t.Fatalf("row=%+v", row)
	}
	if !db.updated {
		t.Fatal("expected update after conflict re-select")
	}
}

func TestUpdateOrCreateUpdatesExisting(t *testing.T) {
	db := (&fakeDB{}).
		on(`FROM "users"`, []string{"id", "email", "age"}, []any{int64(3), "a@x.io", 21}).
		on("UPDATE", nil)
	db.execRes = fakeResult{affected: 1}
	row, err := fastUsers().UpdateOrCreate(context.Background(), db,
		map[string]any{"email": "a@x.io"},
		map[string]any{"age": 40},
	)
	if err != nil {
		t.Fatal(err)
	}
	if row == nil || row.ID != 3 {
		t.Fatalf("row=%+v", row)
	}
	var sawUpdate bool
	for _, st := range db.statements() {
		if strings.Contains(st.SQL, "UPDATE") {
			sawUpdate = true
			if !strings.Contains(st.SQL, `"age" = $1`) {
				t.Fatalf("update: %s", st.SQL)
			}
		}
		if strings.Contains(st.SQL, "INSERT") {
			t.Fatalf("must not insert when row exists: %s", st.SQL)
		}
	}
	if !sawUpdate {
		t.Fatal("expected UPDATE")
	}
}

// orCreateRaceDB: first SELECT misses, INSERT hits unique violation (or insertErr), later SELECTs hit.
type orCreateRaceDB struct {
	email     string
	selects   int
	inserted  bool
	updated   bool
	insertErr error
}

func (d *orCreateRaceDB) QueryContext(_ context.Context, sqlText string, _ ...any) (Rows, error) {
	d.selects++
	if d.selects == 1 {
		return &fakeRows{cols: []string{"id", "email", "age"}}, nil
	}
	return &fakeRows{
		cols:   []string{"id", "email", "age"},
		values: [][]any{{int64(7), d.email, 21}},
	}, nil
}

func (d *orCreateRaceDB) QueryRowContext(_ context.Context, sqlText string, _ ...any) Row {
	if strings.Contains(sqlText, "INSERT") {
		d.inserted = true
		if d.insertErr != nil {
			return errRow{d.insertErr}
		}
		return errRow{&pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"}}
	}
	return errRow{fmt.Errorf("orCreateRaceDB: unexpected QueryRow: %s", sqlText)}
}

func (d *orCreateRaceDB) ExecContext(_ context.Context, sqlText string, _ ...any) (Result, error) {
	if strings.Contains(sqlText, "UPDATE") {
		d.updated = true
		return fakeResult{affected: 1}, nil
	}
	return nil, fmt.Errorf("orCreateRaceDB: unexpected Exec: %s", sqlText)
}
