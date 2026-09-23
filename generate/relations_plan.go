package generate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/vorzela/vorm/introspect"
	"github.com/vorzela/vorm/query"
)

// relPlan is one association to generate on a single owning table.
type relPlan struct {
	Owner        string
	Name         string
	Field        string
	Kind         query.RelationKind
	RelatedTable string

	// LocalKey is the owner column carrying the join value; ForeignKey is the
	// related column it is matched against. This holds for belongs-to and
	// has-many alike, only the direction of the FK differs.
	LocalKey   string
	ForeignKey string

	Pivot           string
	PivotOwnerKey   string
	PivotRelatedKey string
	RelatedKey      string
	PivotCreatedAt  bool
	PivotUpdatedAt  bool
	PivotUnique     bool
	OrderCol        string
	OfManyDesc      bool

	MorphType       string
	MorphTypeColumn string
	MorphIDColumn   string
}

// tableFields resolves the Go field name for every column, applying the same
// collision handling the model emitter uses so relation loaders reference the
// identifiers that actually exist.
func tableFields(t introspect.Table) (map[string]string, map[string]bool) {
	fields := make(map[string]string, len(t.Columns))
	taken := map[string]bool{}
	for _, c := range t.Columns {
		fields[c.Name] = uniqueName(GoFieldName(c.Name), taken)
	}
	return fields, taken
}

// planRelations derives associations from foreign keys, keyed by owning table.
func planRelations(tables []introspect.Table) map[string][]relPlan {
	known := make(map[string]introspect.Table, len(tables))
	for _, t := range tables {
		known[strings.ToLower(t.Name)] = t
	}

	out := map[string][]relPlan{}
	add := func(p relPlan) { out[p.Owner] = append(out[p.Owner], p) }

	pivots := map[string]bool{}
	for _, t := range tables {
		if a, b, ok := pivotSides(t, known); ok {
			pivots[strings.ToLower(t.Name)] = true
			add(belongsToManyPlan(t, a, b))
			add(belongsToManyPlan(t, b, a))
		}
		if fk, morph, ok := morphPivotSides(t, known); ok {
			pivots[strings.ToLower(t.Name)] = true
			addMorphToMany(add, t, fk, morph, tables)
		}
	}

	for _, t := range tables {
		fkCounts := map[string]int{}
		for _, fk := range t.ForeignKeys {
			if len(fk.Columns) == 1 && len(fk.RefColumns) == 1 {
				fkCounts[strings.ToLower(fk.RefTable)]++
			}
		}

		for _, fk := range t.ForeignKeys {
			if len(fk.Columns) != 1 || len(fk.RefColumns) != 1 {
				continue // composite keys need explicit modelling
			}
			ref, ok := known[strings.ToLower(fk.RefTable)]
			if !ok {
				continue
			}
			fkCol := fk.Columns[0]
			refCol := fk.RefColumns[0]
			base := relationBase(fkCol, ref.Name)

			add(relPlan{
				Owner:        t.Name,
				Name:         base,
				Field:        GoName(base),
				Kind:         query.RelationBelongsTo,
				RelatedTable: ref.Name,
				LocalKey:     fkCol,
				ForeignKey:   refCol,
			})

			if pivots[strings.ToLower(t.Name)] {
				continue // the pivot's own rows are exposed as belongs-to-many
			}

			kind := query.RelationHasMany
			inverse := Plural(Singular(t.Name))
			if uniqueSingleColumn(t, fkCol) {
				kind = query.RelationHasOne
				inverse = Singular(t.Name)
			}
			// Two FKs to the same table (author_id, editor_id) would collide, so
			// qualify the inverse side with the column's base name.
			if fkCounts[strings.ToLower(ref.Name)] > 1 {
				inverse = base + "_" + inverse
			}
			add(relPlan{
				Owner:        ref.Name,
				Name:         inverse,
				Field:        GoName(inverse),
				Kind:         kind,
				RelatedTable: t.Name,
				LocalKey:     refCol,
				ForeignKey:   fkCol,
			})
		}
	}

	for _, t := range tables {
		for _, m := range morphColumns(t) {
			add(relPlan{
				Owner:           t.Name,
				Name:            m.prefix,
				Field:           GoName(m.prefix),
				Kind:            query.RelationMorphTo,
				RelatedTable:    t.Name,
				LocalKey:        m.idCol,
				ForeignKey:      "id",
				MorphTypeColumn: m.typeCol,
				MorphIDColumn:   m.idCol,
			})
			childName := Plural(Singular(t.Name))
			for _, other := range tables {
				if strings.EqualFold(other.Name, t.Name) || pivots[strings.ToLower(other.Name)] {
					continue
				}
				if relationNameTaken(out[other.Name], childName) {
					continue
				}
				pk := other.SinglePrimaryKey()
				if pk == "" {
					pk = "id"
				}
				add(relPlan{
					Owner:           other.Name,
					Name:            childName,
					Field:           GoName(childName),
					Kind:            query.RelationMorphMany,
					RelatedTable:    t.Name,
					LocalKey:        pk,
					ForeignKey:      m.idCol,
					MorphType:       other.Name,
					MorphTypeColumn: m.typeCol,
					MorphIDColumn:   m.idCol,
				})
			}
		}
	}

	addThroughAndOfMany(add, out, known)

	for owner := range out {
		out[owner] = dedupeRelations(out[owner])
	}
	return out
}

// relationBase names a belongs-to after the column (author_id → author) so
// multiple FKs to one table stay distinguishable.
func relationBase(fkCol, refTable string) string {
	if trimmed := strings.TrimSuffix(fkCol, "_id"); trimmed != fkCol && trimmed != "" {
		return trimmed
	}
	if trimmed := strings.TrimSuffix(fkCol, "id"); trimmed != fkCol && trimmed != "" {
		return strings.TrimSuffix(trimmed, "_")
	}
	return Singular(refTable)
}

func uniqueSingleColumn(t introspect.Table, col string) bool {
	for _, idx := range t.Indexes {
		if idx.Unique && len(idx.Columns) == 1 && strings.EqualFold(idx.Columns[0], col) {
			return true
		}
	}
	return false
}

// pivotSides reports the two foreign keys of a join table. Extra columns
// (pinned, timestamps, …) still count: two FKs to two different known tables
// is enough to treat it as many-to-many.
func pivotSides(t introspect.Table, known map[string]introspect.Table) (introspect.ForeignKey, introspect.ForeignKey, bool) {
	var fks []introspect.ForeignKey
	for _, fk := range t.ForeignKeys {
		if len(fk.Columns) == 1 && len(fk.RefColumns) == 1 {
			if _, ok := known[strings.ToLower(fk.RefTable)]; ok {
				fks = append(fks, fk)
			}
		}
	}
	if len(fks) != 2 || strings.EqualFold(fks[0].RefTable, fks[1].RefTable) {
		return introspect.ForeignKey{}, introspect.ForeignKey{}, false
	}
	return fks[0], fks[1], true
}

func belongsToManyPlan(pivot introspect.Table, own, other introspect.ForeignKey) relPlan {
	name := Plural(Singular(other.RefTable))
	return relPlan{
		Owner:           own.RefTable,
		Name:            name,
		Field:           GoName(name),
		Kind:            query.RelationBelongsToMany,
		RelatedTable:    other.RefTable,
		LocalKey:        own.RefColumns[0],
		RelatedKey:      other.RefColumns[0],
		Pivot:           pivot.Name,
		PivotOwnerKey:   own.Columns[0],
		PivotRelatedKey: other.Columns[0],
		PivotCreatedAt:  tableHasColumn(pivot, "created_at"),
		PivotUpdatedAt:  tableHasColumn(pivot, "updated_at"),
		PivotUnique:     uniquePair(pivot, own.Columns[0], other.Columns[0]),
	}
}

func uniquePair(t introspect.Table, a, b string) bool {
	la, lb := strings.ToLower(a), strings.ToLower(b)
	pair := func(cols []string) bool {
		if len(cols) != 2 {
			return false
		}
		c0, c1 := strings.ToLower(cols[0]), strings.ToLower(cols[1])
		return (c0 == la && c1 == lb) || (c0 == lb && c1 == la)
	}
	// ON CONFLICT (a, b) is valid only for a unique constraint on exactly those
	// two columns. Partial and expression indexes do not qualify.
	if pair(t.PrimaryKey) {
		return true
	}
	for _, idx := range t.Indexes {
		if !idx.Unique || idx.Partial || idx.Expression || !pair(idx.Columns) {
			continue
		}
		return true
	}
	return false
}

func tableHasColumn(t introspect.Table, name string) bool {
	for _, c := range t.Columns {
		if strings.EqualFold(c.Name, name) {
			return true
		}
	}
	return false
}

func relationNameTaken(plans []relPlan, name string) bool {
	for _, p := range plans {
		if p.Name == name {
			return true
		}
	}
	return false
}

type morphPair struct {
	prefix, typeCol, idCol string
}

func morphColumns(t introspect.Table) []morphPair {
	have := map[string]string{}
	for _, c := range t.Columns {
		have[strings.ToLower(c.Name)] = c.Name
	}
	var out []morphPair
	for _, c := range t.Columns {
		lower := strings.ToLower(c.Name)
		if !strings.HasSuffix(lower, "_type") {
			continue
		}
		prefix := strings.TrimSuffix(lower, "_type")
		if prefix == "" {
			continue
		}
		idCol, ok := have[prefix+"_id"]
		if !ok || columnIsFK(t, idCol) {
			continue
		}
		out = append(out, morphPair{prefix: prefix, typeCol: c.Name, idCol: idCol})
	}
	return out
}

func columnIsFK(t introspect.Table, col string) bool {
	for _, fk := range t.ForeignKeys {
		for _, c := range fk.Columns {
			if strings.EqualFold(c, col) {
				return true
			}
		}
	}
	return false
}

func morphPivotSides(t introspect.Table, known map[string]introspect.Table) (introspect.ForeignKey, morphPair, bool) {
	morphs := morphColumns(t)
	if len(morphs) != 1 {
		return introspect.ForeignKey{}, morphPair{}, false
	}
	var fks []introspect.ForeignKey
	for _, fk := range t.ForeignKeys {
		if len(fk.Columns) == 1 && len(fk.RefColumns) == 1 {
			if _, ok := known[strings.ToLower(fk.RefTable)]; ok {
				fks = append(fks, fk)
			}
		}
	}
	if len(fks) != 1 {
		return introspect.ForeignKey{}, morphPair{}, false
	}
	if columnIsFK(t, morphs[0].idCol) {
		return introspect.ForeignKey{}, morphPair{}, false
	}
	return fks[0], morphs[0], true
}

func addMorphToMany(add func(relPlan), pivot introspect.Table, fk introspect.ForeignKey, morph morphPair, tables []introspect.Table) {
	related := fk.RefTable
	relatedPK := fk.RefColumns[0]
	for _, other := range tables {
		if strings.EqualFold(other.Name, pivot.Name) || strings.EqualFold(other.Name, related) {
			continue
		}
		pk := other.SinglePrimaryKey()
		if pk == "" {
			pk = "id"
		}
		name := Plural(Singular(related))
		add(relPlan{
			Owner:           other.Name,
			Name:            name,
			Field:           GoName(name),
			Kind:            query.RelationMorphToMany,
			RelatedTable:    related,
			LocalKey:        pk,
			RelatedKey:      relatedPK,
			Pivot:           pivot.Name,
			PivotOwnerKey:   morph.idCol,
			PivotRelatedKey: fk.Columns[0],
			PivotCreatedAt:  tableHasColumn(pivot, "created_at"),
			PivotUpdatedAt:  tableHasColumn(pivot, "updated_at"),
			PivotUnique:     uniquePair(pivot, morph.idCol, fk.Columns[0]),
			MorphType:       other.Name,
			MorphTypeColumn: morph.typeCol,
			MorphIDColumn:   morph.idCol,
		})
		inv := Plural(Singular(other.Name))
		add(relPlan{
			Owner:           related,
			Name:            inv,
			Field:           GoName(inv),
			Kind:            query.RelationMorphToMany,
			RelatedTable:    other.Name,
			LocalKey:        relatedPK,
			RelatedKey:      pk,
			Pivot:           pivot.Name,
			PivotOwnerKey:   fk.Columns[0],
			PivotRelatedKey: morph.idCol,
			PivotCreatedAt:  tableHasColumn(pivot, "created_at"),
			PivotUpdatedAt:  tableHasColumn(pivot, "updated_at"),
			PivotUnique:     uniquePair(pivot, morph.idCol, fk.Columns[0]),
			MorphType:       other.Name,
			MorphTypeColumn: morph.typeCol,
			MorphIDColumn:   morph.idCol,
		})
	}
}

func addThroughAndOfMany(add func(relPlan), out map[string][]relPlan, known map[string]introspect.Table) {
	type hop struct {
		owner, name, related, local, foreign string
	}
	var hops []hop
	for owner, plans := range out {
		for _, p := range plans {
			if p.Kind == query.RelationHasMany {
				if strings.EqualFold(owner, p.RelatedTable) {
					continue
				}
				hops = append(hops, hop{owner, p.Name, p.RelatedTable, p.LocalKey, p.ForeignKey})
			}
		}
	}
	sort.Slice(hops, func(i, j int) bool {
		if hops[i].owner != hops[j].owner {
			return hops[i].owner < hops[j].owner
		}
		if hops[i].name != hops[j].name {
			return hops[i].name < hops[j].name
		}
		return hops[i].related < hops[j].related
	})
	for _, a := range hops {
		for _, b := range hops {
			if !strings.EqualFold(a.related, b.owner) || strings.EqualFold(b.related, a.owner) {
				continue
			}
			name := b.name
			if relationNameTaken(out[a.owner], name) {
				name = Singular(a.related) + "_" + b.name
				if relationNameTaken(out[a.owner], name) {
					continue
				}
			}
			throughPK := "id"
			if t, ok := known[strings.ToLower(a.related)]; ok {
				if pk := t.SinglePrimaryKey(); pk != "" {
					throughPK = pk
				}
			}
			add(relPlan{
				Owner:           a.owner,
				Name:            name,
				Field:           GoName(name),
				Kind:            query.RelationHasManyThrough,
				RelatedTable:    b.related,
				LocalKey:        a.local,
				ForeignKey:      b.foreign,
				Pivot:           a.related,
				PivotOwnerKey:   a.foreign,
				PivotRelatedKey: throughPK,
			})
		}
	}
	for _, a := range hops {
		rel, ok := known[strings.ToLower(a.related)]
		if !ok {
			continue
		}
		orderCol := ""
		if tableHasColumn(rel, "created_at") {
			orderCol = "created_at"
		} else if pk := rel.SinglePrimaryKey(); pk != "" {
			orderCol = pk
		} else {
			continue
		}
		base := Singular(a.name)
		latest := "latest_" + base
		if !relationNameTaken(out[a.owner], latest) {
			add(relPlan{
				Owner: a.owner, Name: latest, Field: GoName(latest),
				Kind: query.RelationHasOneOfMany, RelatedTable: a.related,
				LocalKey: a.local, ForeignKey: a.foreign,
				OrderCol: orderCol, OfManyDesc: true,
			})
		}
		oldest := "oldest_" + base
		if !relationNameTaken(out[a.owner], oldest) {
			add(relPlan{
				Owner: a.owner, Name: oldest, Field: GoName(oldest),
				Kind: query.RelationHasOneOfMany, RelatedTable: a.related,
				LocalKey: a.local, ForeignKey: a.foreign,
				OrderCol: orderCol, OfManyDesc: false,
			})
		}
	}
}

func dedupeRelations(plans []relPlan) []relPlan {
	sort.SliceStable(plans, func(i, j int) bool { return plans[i].Name < plans[j].Name })
	seen := map[string]bool{}
	out := plans[:0]
	for _, p := range plans {
		key := p.Name
		for i := 2; seen[key]; i++ {
			key = fmt.Sprintf("%s_%d", p.Name, i)
		}
		seen[key] = true
		p.Name = key
		p.Field = GoName(key)
		out = append(out, p)
	}
	return out
}
