package query

import (
	"context"
	"strings"
	"testing"
)

func TestBelongsToManyAssocAttachSQLMatchesSchema(t *testing.T) {
	db := &fakeDB{}
	a := BelongsToManyAssoc{
		PivotTable:      "post_tags",
		PivotParentKey:  "post_id",
		PivotRelatedKey: "tag_id",
		ParentID:        int64(1),
		CreatedAt:       true,
		UpdatedAt:       false,
		UniquePair:      true,
	}
	if err := a.Attach(context.Background(), db, int64(2), int64(3)); err != nil {
		t.Fatal(err)
	}
	got := db.statements()[0]
	want := `INSERT INTO "post_tags" ("post_id", "tag_id", "created_at") VALUES ($1, $2, CURRENT_TIMESTAMP), ($3, $4, CURRENT_TIMESTAMP) ON CONFLICT ("post_id", "tag_id") DO NOTHING`
	if got.SQL != want {
		t.Fatalf("attach SQL:\n got: %s\nwant: %s", got.SQL, want)
	}
	if len(got.Args) != 4 || got.Args[0] != int64(1) || got.Args[1] != int64(2) || got.Args[3] != int64(3) {
		t.Fatalf("args=%v", got.Args)
	}
	if strings.Contains(got.SQL, "*") || strings.Contains(got.SQL, "updated_at") {
		t.Fatalf("must not invent columns: %s", got.SQL)
	}
}

func TestBelongsToManyAssocAttachWithoutUniqueIsPlainInsert(t *testing.T) {
	db := &fakeDB{}
	a := BelongsToManyAssoc{
		PivotTable:      "post_tags",
		PivotParentKey:  "post_id",
		PivotRelatedKey: "tag_id",
		ParentID:        int64(1),
	}
	if err := a.Attach(context.Background(), db, int64(2)); err != nil {
		t.Fatal(err)
	}
	got := db.statements()[0].SQL
	if got != `INSERT INTO "post_tags" ("post_id", "tag_id") VALUES ($1, $2)` {
		t.Fatalf("plain insert: %s", got)
	}
	if strings.Contains(got, "ON CONFLICT") {
		t.Fatal("ON CONFLICT without a unique pair is invalid SQL")
	}
}

func TestBelongsToManyAssocAttachWithExtraColumn(t *testing.T) {
	db := &fakeDB{}
	a := BelongsToManyAssoc{
		PivotTable:      "post_tags",
		PivotParentKey:  "post_id",
		PivotRelatedKey: "tag_id",
		ParentID:        int64(1),
		CreatedAt:       true,
		UniquePair:      true,
	}
	if err := a.AttachWith(context.Background(), db, int64(2), map[string]any{"pinned": true}); err != nil {
		t.Fatal(err)
	}
	got := db.statements()[0]
	want := `INSERT INTO "post_tags" ("post_id", "tag_id", "pinned", "created_at") VALUES ($1, $2, $3, CURRENT_TIMESTAMP) ON CONFLICT ("post_id", "tag_id") DO NOTHING`
	if got.SQL != want {
		t.Fatalf("attach with extra:\n got: %s\nwant: %s", got.SQL, want)
	}
	if got.Args[2] != true {
		t.Fatalf("pinned not bound: %v", got.Args)
	}
}

func TestBelongsToManyAssocDetachSQL(t *testing.T) {
	db := &fakeDB{}
	a := BelongsToManyAssoc{
		PivotTable:      "post_tags",
		PivotParentKey:  "post_id",
		PivotRelatedKey: "tag_id",
		ParentID:        int64(1),
	}
	if err := a.Detach(context.Background(), db, int64(2), int64(3)); err != nil {
		t.Fatal(err)
	}
	got := db.statements()[0]
	want := `DELETE FROM "post_tags" WHERE "post_id" = $1 AND "tag_id" IN ($2, $3)`
	if got.SQL != want {
		t.Fatalf("detach SQL:\n got: %s\nwant: %s", got.SQL, want)
	}
}

func TestBelongsToManyAssocDetachAll(t *testing.T) {
	db := &fakeDB{}
	a := BelongsToManyAssoc{
		PivotTable:      "post_tags",
		PivotParentKey:  "post_id",
		PivotRelatedKey: "tag_id",
		ParentID:        int64(1),
	}
	if err := a.Detach(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	got := db.statements()[0].SQL
	if got != `DELETE FROM "post_tags" WHERE "post_id" = $1` {
		t.Fatalf("detach all: %s", got)
	}
}

func TestBelongsToManyAssocSync(t *testing.T) {
	db := (&fakeDB{}).on("post_tags", []string{"tag_id"}, []any{int64(2)}, []any{int64(9)})
	a := BelongsToManyAssoc{
		PivotTable:      "post_tags",
		PivotParentKey:  "post_id",
		PivotRelatedKey: "tag_id",
		ParentID:        int64(1),
		UniquePair:      true,
	}
	if err := a.Sync(context.Background(), db, int64(2), int64(3)); err != nil {
		t.Fatal(err)
	}
	var sawSelect, sawDelete, sawInsert bool
	for _, s := range db.statements() {
		if strings.HasPrefix(s.SQL, "SELECT ") {
			sawSelect = true
			if s.SQL != `SELECT "tag_id" FROM "post_tags" WHERE "post_id" = $1` {
				t.Fatalf("sync lookup must list the related key only: %s", s.SQL)
			}
		}
		if strings.HasPrefix(s.SQL, "DELETE ") {
			sawDelete = true
		}
		if strings.HasPrefix(s.SQL, "INSERT ") {
			sawInsert = true
		}
		if strings.Contains(s.SQL, "*") {
			t.Fatalf("relation SQL must never use *: %s", s.SQL)
		}
	}
	if !sawSelect || !sawDelete || !sawInsert {
		t.Fatalf("sync should select, detach extras and attach missing: %+v", db.statements())
	}
}

func TestBelongsToManyAssocAttachMySQLUsesInsertIgnore(t *testing.T) {
	db := &fakeDB{}
	a := BelongsToManyAssoc{
		PivotTable:      "post_tags",
		PivotParentKey:  "post_id",
		PivotRelatedKey: "tag_id",
		ParentID:        int64(1),
		UniquePair:      true,
		Dialect:         DialectMySQL,
	}
	if err := a.Attach(context.Background(), db, int64(2)); err != nil {
		t.Fatal(err)
	}
	got := db.statements()[0].SQL
	want := "INSERT IGNORE INTO `post_tags` (`post_id`, `tag_id`) VALUES (?, ?)"
	if got != want {
		t.Fatalf("mysql attach:\n got: %s\nwant: %s", got, want)
	}
	if strings.Contains(got, "ON CONFLICT") {
		t.Fatalf("MySQL must not emit ON CONFLICT: %s", got)
	}
}

func TestMorphToManyAttachIncludesType(t *testing.T) {
	db := &fakeDB{}
	a := BelongsToManyAssoc{
		PivotTable:      "taggables",
		PivotParentKey:  "taggable_id",
		PivotRelatedKey: "tag_id",
		ParentID:        int64(1),
		MorphType:       "posts",
		MorphTypeColumn: "taggable_type",
		UniquePair:      true,
	}
	if err := a.Attach(context.Background(), db, int64(2)); err != nil {
		t.Fatal(err)
	}
	got := db.statements()[0]
	want := `INSERT INTO "taggables" ("taggable_id", "tag_id", "taggable_type") VALUES ($1, $2, $3) ON CONFLICT ("taggable_id", "tag_id") DO NOTHING`
	if got.SQL != want {
		t.Fatalf("morph attach:\n got: %s\nwant: %s", got.SQL, want)
	}
	if got.Args[2] != "posts" {
		t.Fatalf("type arg=%v", got.Args)
	}
}
