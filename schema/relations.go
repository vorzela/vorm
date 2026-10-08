package schema

import "fmt"

// NewPivotBlueprint builds the default many-to-many join table (id, two FKs,
// unique pair, timestamps). Extra pivot columns can be added by using Create
// instead of BelongsToMany.
//
// Optional parentCols are the referenced columns on left/right (default "id"):
//
//	NewPivotBlueprint("posts", "tags")
//	NewPivotBlueprint("posts", "tags", "uuid", "uuid")
func NewPivotBlueprint(leftTable, rightTable string, parentCols ...string) *Blueprint {
	leftCol := Singularize(leftTable) + "_id"
	rightCol := Singularize(rightTable) + "_id"
	leftRef, rightRef := pivotParentCols(parentCols)
	bp := NewBlueprint(PivotName(leftTable, rightTable))
	bp.ID()
	bp.BelongsTo(leftCol, leftRef, leftTable)
	bp.BelongsTo(rightCol, rightRef, rightTable)
	bp.Unique(leftCol, rightCol)
	bp.Timestamps()
	return bp
}

func pivotParentCols(parentCols []string) (leftRef, rightRef string) {
	leftRef, rightRef = "id", "id"
	if len(parentCols) >= 1 && parentCols[0] != "" {
		leftRef = parentCols[0]
	}
	if len(parentCols) >= 2 && parentCols[1] != "" {
		rightRef = parentCols[1]
	}
	return leftRef, rightRef
}

// BelongsToMany writes a classic pivot table migration (many-to-many).
//
//	schema.BelongsToMany(facade, "post_tag", "post_id", "posts", "tag_id", "tags")
//	schema.BelongsToMany(facade, "post_tag", "post_id", "posts", "tag_id", "tags", "uuid", "uuid")
func BelongsToMany(f *Facade, pivot, leftCol, leftTable, rightCol, rightTable string, parentCols ...string) error {
	if f == nil {
		f = Default
	}
	leftRef, rightRef := pivotParentCols(parentCols)
	return f.Create(pivot, func(t *Blueprint) {
		t.ID()
		t.BelongsTo(leftCol, leftRef, leftTable)
		t.BelongsTo(rightCol, rightRef, rightTable)
		t.Unique(leftCol, rightCol)
		t.Timestamps()
	})
}

// BelongsToMany writes a pivot for leftTable ↔ rightTable using Laravel's
// alphabetical singular join name (posts + tags → post_tag).
// Optional parentCols override the referenced columns (default "id").
func (f *Facade) BelongsToMany(leftTable, rightTable string, parentCols ...string) error {
	bp := NewPivotBlueprint(leftTable, rightTable, parentCols...)
	return BelongsToMany(f, bp.table, Singularize(leftTable)+"_id", leftTable, Singularize(rightTable)+"_id", rightTable, parentCols...)
}

// NewMorphPivotBlueprint builds a morphToMany join table: one real FK plus a
// morph pair (taggables: tag_id + taggable_id + taggable_type).
// Optional relatedRef is the referenced column on relatedTable (default "id").
func NewMorphPivotBlueprint(relatedTable, morph string, relatedRef ...string) *Blueprint {
	relatedCol := Singularize(relatedTable) + "_id"
	ref := "id"
	if len(relatedRef) > 0 && relatedRef[0] != "" {
		ref = relatedRef[0]
	}
	bp := NewBlueprint(Pluralize(morph))
	bp.ID()
	bp.BelongsTo(relatedCol, ref, relatedTable)
	bp.Morphs(morph)
	bp.Unique(relatedCol, morph+"_id", morph+"_type")
	bp.Timestamps()
	return bp
}

// MorphToMany writes a polymorphic many-to-many pivot
// (tags + taggable → taggables with tag_id and taggable morphs).
// Optional relatedRef is the referenced column on relatedTable (default "id").
func (f *Facade) MorphToMany(relatedTable, morph string, relatedRef ...string) error {
	if f == nil {
		f = Default
	}
	bp := NewMorphPivotBlueprint(relatedTable, morph, relatedRef...)
	relatedCol := Singularize(relatedTable) + "_id"
	ref := "id"
	if len(relatedRef) > 0 && relatedRef[0] != "" {
		ref = relatedRef[0]
	}
	return f.Create(bp.table, func(t *Blueprint) {
		t.ID()
		t.BelongsTo(relatedCol, ref, relatedTable)
		t.Morphs(morph)
		t.Unique(relatedCol, morph+"_id", morph+"_type")
		t.Timestamps()
	})
}

// HasManyComment documents the inverse of BelongsTo (FK lives on the many side).
// Optional references is the parent column (default "id").
func HasManyComment(parentTable, childTable, fk string, references ...string) string {
	parentCol := "id"
	if len(references) > 0 && references[0] != "" {
		parentCol = references[0]
	}
	return fmt.Sprintf("// hasMany: %s has many %s via %s.%s → %s.%s",
		parentTable, childTable, childTable, fk, parentTable, parentCol)
}
