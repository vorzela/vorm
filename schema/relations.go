package schema

import "fmt"

// NewPivotBlueprint builds the default many-to-many join table (id, two FKs,
// unique pair, timestamps). Extra pivot columns can be added by using Create
// instead of BelongsToMany.
func NewPivotBlueprint(leftTable, rightTable string) *Blueprint {
	leftCol := Singularize(leftTable) + "_id"
	rightCol := Singularize(rightTable) + "_id"
	bp := NewBlueprint(PivotName(leftTable, rightTable))
	bp.ID()
	bp.BelongsTo(leftCol, leftTable)
	bp.BelongsTo(rightCol, rightTable)
	bp.Unique(leftCol, rightCol)
	bp.Timestamps()
	return bp
}

// BelongsToMany writes a classic pivot table migration (many-to-many).
//
//	schema.BelongsToMany(facade, "post_tag", "post_id", "posts", "tag_id", "tags")
func BelongsToMany(f *Facade, pivot, leftCol, leftTable, rightCol, rightTable string) error {
	if f == nil {
		f = Default
	}
	return f.Create(pivot, func(t *Blueprint) {
		t.ID()
		t.BelongsTo(leftCol, leftTable)
		t.BelongsTo(rightCol, rightTable)
		t.Unique(leftCol, rightCol)
		t.Timestamps()
	})
}

// BelongsToMany writes a pivot for leftTable ↔ rightTable using Laravel's
// alphabetical singular join name (posts + tags → post_tag).
func (f *Facade) BelongsToMany(leftTable, rightTable string) error {
	bp := NewPivotBlueprint(leftTable, rightTable)
	return BelongsToMany(f, bp.table, Singularize(leftTable)+"_id", leftTable, Singularize(rightTable)+"_id", rightTable)
}

// HasManyComment documents the inverse of BelongsTo (FK lives on the many side).
func HasManyComment(parentTable, childTable, fk string) string {
	return fmt.Sprintf("// hasMany: %s has many %s via %s.%s → %s.id",
		parentTable, childTable, childTable, fk, parentTable)
}
