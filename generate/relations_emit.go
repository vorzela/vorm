package generate

import (
	"fmt"
	"strings"

	"github.com/vorzela/vorm/introspect"
	"github.com/vorzela/vorm/query"
)

// resolveFields assigns final Go field names for every column and relation,
// resolving collisions once so models and loaders agree on the identifiers.
func resolveFields(tables []introspect.Table, rels map[string][]relPlan) map[string]map[string]string {
	out := make(map[string]map[string]string, len(tables))
	for _, t := range tables {
		fields, taken := tableFields(t)
		plans := rels[t.Name]
		for i := range plans {
			plans[i].Field = uniqueName(plans[i].Field, taken)
		}
		rels[t.Name] = plans
		out[t.Name] = fields
	}
	return out
}

func renderRelations(opts SchemaOptions, tables []introspect.Table, rels map[string][]relPlan, fields map[string]map[string]string) string {
	var b strings.Builder
	b.WriteString(doNotEdit)
	fmt.Fprintf(&b, "\npackage %s\n\n", opts.Package)
	b.WriteString("import (\n\t\"context\"\n\n\t\"github.com/vorzela/vorm/query\"\n)\n\n")
	b.WriteString("// Relations are registered on package init so .With(\"name\") resolves without\n")
	b.WriteString("// extra wiring. Every loader batches the whole result set into a bounded\n")
	b.WriteString("// number of queries — there is no per-row lookup.\n")
	b.WriteString("func init() {\n")

	for _, t := range tables {
		plans := rels[t.Name]
		if len(plans) == 0 {
			continue
		}
		owner := ModelName(t.Name)
		for _, p := range plans {
			related := ModelName(p.RelatedTable)
			ownerField, ok := fields[t.Name][p.LocalKey]
			if !ok {
				continue
			}
			relatedFields := fields[p.RelatedTable]
			if relatedFields == nil {
				continue
			}

			fmt.Fprintf(&b, "\tquery.RegisterRelation(query.Relation{\n")
			fmt.Fprintf(&b, "\t\tName: %q, Kind: query.Relation%s, Table: %q, Field: %q,\n", p.Name, relationKindGo(p.Kind), p.RelatedTable, p.Field)
			fmt.Fprintf(&b, "\t\tLocalKey: %q, ForeignKey: %q,\n", p.LocalKey, p.ForeignKey)
			if p.Pivot != "" {
				fmt.Fprintf(&b, "\t\tPivotTable: %q, PivotLocalKey: %q, PivotForeignKey: %q,\n", p.Pivot, p.PivotOwnerKey, p.PivotRelatedKey)
			}
			if p.MorphType != "" {
				fmt.Fprintf(&b, "\t\tMorphType: %q,\n", p.MorphType)
			}
			fmt.Fprintf(&b, "\t}, func(ctx context.Context, db query.DB, rows []*%s) error {\n", owner)

			switch p.Kind {
			case query.RelationBelongsTo:
				relatedField, ok := relatedFields[p.ForeignKey]
				if !ok {
					b.WriteString("\t\treturn nil\n\t})\n")
					continue
				}
				fmt.Fprintf(&b, "\t\treturn query.LoadBelongsTo(ctx, db, rows, query.BelongsTo[%s, %s]{\n", owner, related)
				fmt.Fprintf(&b, "\t\t\tRelated:   %s,\n", EntityName(p.RelatedTable))
				fmt.Fprintf(&b, "\t\t\tOwnerKey:  %q,\n", p.ForeignKey)
				fmt.Fprintf(&b, "\t\t\tParentKey: func(m *%s) any { return m.%s },\n", owner, ownerField)
				fmt.Fprintf(&b, "\t\t\tChildKey:  func(r *%s) any { return r.%s },\n", related, relatedField)
				fmt.Fprintf(&b, "\t\t\tAssign:    func(m *%s, r *%s) { m.%s = r },\n", owner, related, p.Field)
				b.WriteString("\t\t})\n")

			case query.RelationHasOne:
				relatedField, ok := relatedFields[p.ForeignKey]
				if !ok {
					b.WriteString("\t\treturn nil\n\t})\n")
					continue
				}
				fmt.Fprintf(&b, "\t\treturn query.LoadHasMany(ctx, db, rows, query.HasMany[%s, %s]{\n", owner, related)
				fmt.Fprintf(&b, "\t\t\tRelated:    %s,\n", EntityName(p.RelatedTable))
				fmt.Fprintf(&b, "\t\t\tForeignKey: %q,\n", p.ForeignKey)
				fmt.Fprintf(&b, "\t\t\tParentKey:  func(m *%s) any { return m.%s },\n", owner, ownerField)
				fmt.Fprintf(&b, "\t\t\tChildKey:   func(r *%s) any { return r.%s },\n", related, relatedField)
				fmt.Fprintf(&b, "\t\t\tAssign: func(m *%s, rows []%s) {\n", owner, related)
				b.WriteString("\t\t\t\tif len(rows) > 0 {\n")
				fmt.Fprintf(&b, "\t\t\t\t\tm.%s = &rows[0]\n", p.Field)
				b.WriteString("\t\t\t\t}\n\t\t\t},\n")
				b.WriteString("\t\t})\n")

			case query.RelationHasMany:
				relatedField, ok := relatedFields[p.ForeignKey]
				if !ok {
					b.WriteString("\t\treturn nil\n\t})\n")
					continue
				}
				fmt.Fprintf(&b, "\t\treturn query.LoadHasMany(ctx, db, rows, query.HasMany[%s, %s]{\n", owner, related)
				fmt.Fprintf(&b, "\t\t\tRelated:    %s,\n", EntityName(p.RelatedTable))
				fmt.Fprintf(&b, "\t\t\tForeignKey: %q,\n", p.ForeignKey)
				fmt.Fprintf(&b, "\t\t\tParentKey:  func(m *%s) any { return m.%s },\n", owner, ownerField)
				fmt.Fprintf(&b, "\t\t\tChildKey:   func(r *%s) any { return r.%s },\n", related, relatedField)
				fmt.Fprintf(&b, "\t\t\tAssign:     func(m *%s, rows []%s) { m.%s = rows },\n", owner, related, p.Field)
				b.WriteString("\t\t})\n")

			case query.RelationBelongsToMany:
				relatedField, ok := relatedFields[p.RelatedKey]
				if !ok {
					b.WriteString("\t\treturn nil\n\t})\n")
					continue
				}
				fmt.Fprintf(&b, "\t\treturn query.LoadBelongsToMany(ctx, db, rows, query.BelongsToMany[%s, %s]{\n", owner, related)
				fmt.Fprintf(&b, "\t\t\tRelated:         %s,\n", EntityName(p.RelatedTable))
				fmt.Fprintf(&b, "\t\t\tPivotTable:      %q,\n", p.Pivot)
				fmt.Fprintf(&b, "\t\t\tPivotParentKey:  %q,\n", p.PivotOwnerKey)
				fmt.Fprintf(&b, "\t\t\tPivotRelatedKey: %q,\n", p.PivotRelatedKey)
				fmt.Fprintf(&b, "\t\t\tRelatedKey:      %q,\n", p.RelatedKey)
				fmt.Fprintf(&b, "\t\t\tParentKey:       func(m *%s) any { return m.%s },\n", owner, ownerField)
				fmt.Fprintf(&b, "\t\t\tChildKey:        func(r *%s) any { return r.%s },\n", related, relatedField)
				fmt.Fprintf(&b, "\t\t\tAssign:          func(m *%s, rows []%s) { m.%s = rows },\n", owner, related, p.Field)
				b.WriteString("\t\t})\n")

			case query.RelationMorphTo:
				typeField, okType := fields[t.Name][p.MorphTypeColumn]
				idField, okID := fields[t.Name][p.MorphIDColumn]
				if !okType || !okID {
					b.WriteString("\t\treturn nil\n\t})\n")
					continue
				}
				fmt.Fprintf(&b, "\t\treturn query.LoadMorphTo(ctx, db, rows, query.MorphTo[%s]{\n", owner)
				fmt.Fprintf(&b, "\t\t\tParentType: func(m *%s) string { return m.%s },\n", owner, typeField)
				fmt.Fprintf(&b, "\t\t\tParentID:   func(m *%s) any { return m.%s },\n", owner, idField)
				fmt.Fprintf(&b, "\t\t\tAssign:     func(m *%s, v any) { m.%s = v },\n", owner, p.Field)
				b.WriteString("\t\t\tTargets: map[string]query.MorphLoader{\n")
				for _, other := range tables {
					if strings.EqualFold(other.Name, t.Name) {
						continue
					}
					pk := other.SinglePrimaryKey()
					if pk == "" {
						pk = "id"
					}
					pkField, ok := fields[other.Name][pk]
					if !ok {
						continue
					}
					fmt.Fprintf(&b, "\t\t\t\t%q: query.MorphLoad(%s, func(row *%s) any { return row.%s }),\n",
						other.Name, EntityName(other.Name), ModelName(other.Name), pkField)
				}
				b.WriteString("\t\t\t},\n\t\t})\n")

			case query.RelationMorphMany:
				relatedField, ok := relatedFields[p.ForeignKey]
				if !ok {
					b.WriteString("\t\treturn nil\n\t})\n")
					continue
				}
				fmt.Fprintf(&b, "\t\treturn query.LoadHasMany(ctx, db, rows, query.HasMany[%s, %s]{\n", owner, related)
				fmt.Fprintf(&b, "\t\t\tRelated:    %s,\n", EntityName(p.RelatedTable))
				fmt.Fprintf(&b, "\t\t\tForeignKey: %q,\n", p.ForeignKey)
				fmt.Fprintf(&b, "\t\t\tParentKey:  func(m *%s) any { return m.%s },\n", owner, ownerField)
				fmt.Fprintf(&b, "\t\t\tChildKey:   func(r *%s) any { return r.%s },\n", related, relatedField)
				fmt.Fprintf(&b, "\t\t\tAssign:     func(m *%s, rows []%s) { m.%s = rows },\n", owner, related, p.Field)
				fmt.Fprintf(&b, "\t\t\tModify:     func(b *query.Builder[%s]) *query.Builder[%s] {\n", related, related)
				fmt.Fprintf(&b, "\t\t\t\treturn b.Where(%q, %q)\n", p.MorphTypeColumn, p.MorphType)
				b.WriteString("\t\t\t},\n")
				b.WriteString("\t\t})\n")
			}
			b.WriteString("\t})\n\n")
		}
	}

	b.WriteString("}\n")
	b.WriteString(renderAssocMethods(tables, rels, fields))
	return b.String()
}

func renderAssocMethods(tables []introspect.Table, rels map[string][]relPlan, fields map[string]map[string]string) string {
	var b strings.Builder
	for _, t := range tables {
		owner := ModelName(t.Name)
		for _, p := range rels[t.Name] {
			if p.Kind != query.RelationBelongsToMany {
				continue
			}
			idField, ok := fields[t.Name][p.LocalKey]
			if !ok {
				continue
			}
			fmt.Fprintf(&b, "\n// %sRelation is the belongs-to-many association (Attach/Detach/Sync/Toggle).\n", p.Field)
			fmt.Fprintf(&b, "// The eager-load field is named %s; Go cannot share that identifier with a method.\n", p.Field)
			fmt.Fprintf(&b, "func (m *%s) %sRelation() query.BelongsToManyAssoc {\n", owner, p.Field)
			fmt.Fprintf(&b, "\treturn query.BelongsToManyAssoc{\n")
			fmt.Fprintf(&b, "\t\tPivotTable: %q, PivotParentKey: %q, PivotRelatedKey: %q,\n", p.Pivot, p.PivotOwnerKey, p.PivotRelatedKey)
			fmt.Fprintf(&b, "\t\tParentID: m.%s, Timestamps: %t,\n", idField, p.PivotTimestamps)
			b.WriteString("\t}\n}\n")
			fmt.Fprintf(&b, "\nfunc (m *%s) Attach%s(ctx context.Context, db query.DB, ids ...any) error {\n", owner, p.Field)
			fmt.Fprintf(&b, "\treturn m.%sRelation().Attach(ctx, db, ids...)\n}\n", p.Field)
			fmt.Fprintf(&b, "\nfunc (m *%s) Detach%s(ctx context.Context, db query.DB, ids ...any) error {\n", owner, p.Field)
			fmt.Fprintf(&b, "\treturn m.%sRelation().Detach(ctx, db, ids...)\n}\n", p.Field)
			fmt.Fprintf(&b, "\nfunc (m *%s) Sync%s(ctx context.Context, db query.DB, ids ...any) error {\n", owner, p.Field)
			fmt.Fprintf(&b, "\treturn m.%sRelation().Sync(ctx, db, ids...)\n}\n", p.Field)
			fmt.Fprintf(&b, "\nfunc (m *%s) Toggle%s(ctx context.Context, db query.DB, ids ...any) error {\n", owner, p.Field)
			fmt.Fprintf(&b, "\treturn m.%sRelation().Toggle(ctx, db, ids...)\n}\n", p.Field)
		}
	}
	return b.String()
}

func relationKindGo(k query.RelationKind) string {
	switch k {
	case query.RelationBelongsTo:
		return "BelongsTo"
	case query.RelationHasOne:
		return "HasOne"
	case query.RelationBelongsToMany:
		return "BelongsToMany"
	case query.RelationMorphTo:
		return "MorphTo"
	case query.RelationMorphMany:
		return "MorphMany"
	default:
		return "HasMany"
	}
}
