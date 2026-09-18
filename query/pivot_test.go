package query

import (
	"context"
	"strings"
	"testing"
)

func TestBelongsToManyAssocAttach(t *testing.T) {
	db := &fakeDB{}
	a := BelongsToManyAssoc{
		PivotTable:      "post_tag",
		PivotParentKey:  "post_id",
		PivotRelatedKey: "tag_id",
		ParentID:        int64(1),
	}
	if err := a.Attach(context.Background(), db, int64(2), int64(3)); err != nil {
		t.Fatal(err)
	}
	if db.count() != 1 {
		t.Fatalf("want 1 insert, got %d: %v", db.count(), db.statements())
	}
	sqlText := db.statements()[0].SQL
	if !strings.Contains(sqlText, `INSERT INTO "post_tag"`) || !strings.Contains(sqlText, "ON CONFLICT") {
		t.Fatalf("unexpected SQL: %s", sqlText)
	}
}

func TestBelongsToManyAssocDetachAll(t *testing.T) {
	db := &fakeDB{}
	a := BelongsToManyAssoc{
		PivotTable:      "post_tag",
		PivotParentKey:  "post_id",
		PivotRelatedKey: "tag_id",
		ParentID:        int64(1),
	}
	if err := a.Detach(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	sqlText := db.statements()[0].SQL
	if !strings.Contains(sqlText, "DELETE FROM") || strings.Contains(sqlText, "IN (") {
		t.Fatalf("detach all should not filter related ids: %s", sqlText)
	}
}

func TestBelongsToManyAssocSync(t *testing.T) {
	db := (&fakeDB{}).on("post_tag", []string{"tag_id"}, []any{int64(2)}, []any{int64(9)})
	a := BelongsToManyAssoc{
		PivotTable:      "post_tag",
		PivotParentKey:  "post_id",
		PivotRelatedKey: "tag_id",
		ParentID:        int64(1),
	}
	if err := a.Sync(context.Background(), db, int64(2), int64(3)); err != nil {
		t.Fatal(err)
	}
	var sawDelete, sawInsert bool
	for _, s := range db.statements() {
		if strings.Contains(s.SQL, "DELETE FROM") {
			sawDelete = true
		}
		if strings.Contains(s.SQL, "INSERT INTO") {
			sawInsert = true
		}
	}
	if !sawDelete || !sawInsert {
		t.Fatalf("sync should detach extras and attach missing: %+v", db.statements())
	}
}
