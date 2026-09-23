package query

import (
	"fmt"
	"reflect"
	"strings"
)

type existsArg struct {
	RelatedTable string
	RelatedCol   string // column on the related/pivot table
	ParentTable  string
	ParentCol    string // column on the parent
	JoinTable    string // optional: join related for m2m extra filters
	JoinOnLeft   string
	JoinOnRight  string
	MorphTypeCol string
	MorphType    string
	Extra        []pred
}

func (b *Builder[T]) compileExistsPred(p pred, not bool, placeholder func() string) (string, []any, error) {
	if ea, ok := p.arg.(existsArg); ok {
		inner, args, err := b.compileExistsSQL(ea, placeholder)
		if err != nil {
			return "", nil, err
		}
		if not {
			return "NOT EXISTS (" + inner + ")", args, nil
		}
		return "EXISTS (" + inner + ")", args, nil
	}
	clause := "EXISTS (" + p.op + ")"
	if not {
		clause = "NOT EXISTS (" + p.op + ")"
	}
	var args []any
	if raw, ok := p.arg.([]any); ok {
		args = raw
	}
	return clause, args, nil
}

func (b *Builder[T]) compileExistsSQL(ea existsArg, placeholder func() string) (string, []any, error) {
	relQ, err := QuoteIdent(b.dialect, ea.RelatedTable)
	if err != nil {
		return "", nil, err
	}
	relCol, err := QuoteIdent(b.dialect, ea.RelatedTable+"."+ea.RelatedCol)
	if err != nil {
		return "", nil, err
	}
	parentCol, err := QuoteIdent(b.dialect, ea.ParentTable+"."+ea.ParentCol)
	if err != nil {
		return "", nil, err
	}
	var sb strings.Builder
	sb.WriteString("SELECT 1 FROM ")
	sb.WriteString(relQ)
	if ea.JoinTable != "" {
		joinQ, err := QuoteIdent(b.dialect, ea.JoinTable)
		if err != nil {
			return "", nil, err
		}
		left, err := QuoteIdent(b.dialect, ea.JoinOnLeft)
		if err != nil {
			return "", nil, err
		}
		right, err := QuoteIdent(b.dialect, ea.JoinOnRight)
		if err != nil {
			return "", nil, err
		}
		sb.WriteString(" INNER JOIN ")
		sb.WriteString(joinQ)
		sb.WriteString(" ON ")
		sb.WriteString(left)
		sb.WriteString(" = ")
		sb.WriteString(right)
	}
	sb.WriteString(" WHERE ")
	sb.WriteString(relCol)
	sb.WriteString(" = ")
	sb.WriteString(parentCol)
	var args []any
	if ea.MorphTypeCol != "" && ea.MorphType != "" {
		typeQ, err := QuoteIdent(b.dialect, ea.RelatedTable+"."+ea.MorphTypeCol)
		if err != nil {
			typeQ, err = QuoteIdent(b.dialect, ea.MorphTypeCol)
			if err != nil {
				return "", nil, err
			}
		}
		sb.WriteString(" AND ")
		sb.WriteString(typeQ)
		sb.WriteString(" = ")
		if placeholder != nil {
			sb.WriteString(placeholder())
			args = append(args, ea.MorphType)
		} else {
			if err := SafeIdent(ea.MorphType); err != nil {
				return "", nil, err
			}
			sb.WriteString("'" + strings.ReplaceAll(ea.MorphType, "'", "''") + "'")
		}
	}
	for _, extra := range ea.Extra {
		op := strings.ToUpper(extra.op)
		if op == "" {
			op = "="
		}
		col := extra.col
		if !strings.Contains(col, ".") {
			if ea.JoinTable != "" {
				col = ea.JoinTable + "." + col
			} else {
				col = ea.RelatedTable + "." + col
			}
		}
		qc, err := QuoteIdent(b.dialect, col)
		if err != nil {
			return "", nil, err
		}
		switch op {
		case "IS NULL", "IS NOT NULL":
			sb.WriteString(" AND ")
			sb.WriteString(qc)
			sb.WriteString(" ")
			sb.WriteString(op)
		default:
			if err := SafeOp(op); err != nil {
				return "", nil, err
			}
			sb.WriteString(" AND ")
			sb.WriteString(qc)
			sb.WriteString(" ")
			sb.WriteString(op)
			sb.WriteString(" ")
			sb.WriteString(placeholder())
			args = append(args, extra.arg)
		}
	}
	return sb.String(), args, nil
}

// WhereHas keeps rows that have at least one matching related row (correlated EXISTS).
func (b *Builder[T]) WhereHas(name string) *Builder[T] {
	return b.whereHasRel(name, false, pred{})
}

// WhereDoesntHave is the inverse of WhereHas.
func (b *Builder[T]) WhereDoesntHave(name string) *Builder[T] {
	return b.whereHasRel(name, true, pred{})
}

// WhereRelation is WhereHas plus a predicate on the related table.
func (b *Builder[T]) WhereRelation(name, col string, args ...any) *Builder[T] {
	p, err := parseWhere(append([]any{col}, args...)...)
	if err != nil {
		b.wheres = append(b.wheres, pred{col: "__error__", op: "=", arg: err.Error()})
		return b
	}
	return b.whereHasRel(name, false, p)
}

func (b *Builder[T]) whereHasRel(name string, not bool, extra pred) *Builder[T] {
	rel, ok := lookupRegistered(reflect.TypeFor[T](), name)
	if !ok {
		b.wheres = append(b.wheres, pred{col: "__error__", op: "=", arg: fmt.Sprintf("vorm/query: unknown relation %q", name)})
		return b
	}
	ea, err := existsFromRelation(b.meta.Table, b.meta.PrimaryKey, rel.rel)
	if err != nil {
		b.wheres = append(b.wheres, pred{col: "__error__", op: "=", arg: err.Error()})
		return b
	}
	attachExistsJoin(&ea, rel.rel, extra)
	if extra.col != "" {
		ea.Extra = append(ea.Extra, extra)
	}
	col := "__exists__"
	if not {
		col = "__not_exists__"
	}
	b.wheres = append(b.wheres, pred{col: col, arg: ea})
	return b
}

func existsFromRelation(parentTable, parentPK string, rel Relation) (existsArg, error) {
	parentCol := rel.LocalKey
	if parentCol == "" {
		parentCol = parentPK
	}
	relatedCol := rel.ForeignKey
	if relatedCol == "" {
		relatedCol = "id"
	}
	ea := existsArg{
		ParentTable:  parentTable,
		ParentCol:    parentCol,
		MorphType:    rel.MorphType,
		MorphTypeCol: rel.MorphTypeColumn,
	}
	switch rel.Kind {
	case RelationBelongsTo:
		ea.RelatedTable = rel.Table
		ea.RelatedCol = relatedCol
		if rel.ForeignKey == "" {
			ea.RelatedCol = "id"
		}
		ea.ParentCol = rel.LocalKey
	case RelationHasMany, RelationHasOne, RelationMorphMany:
		ea.RelatedTable = rel.Table
		ea.RelatedCol = rel.ForeignKey
		ea.ParentCol = rel.LocalKey
		if rel.Kind == RelationMorphMany && ea.MorphTypeCol == "" {
			ea.MorphTypeCol = morphTypeColumn(rel)
		}
	case RelationHasManyThrough:
		if rel.PivotTable == "" || rel.PivotLocalKey == "" {
			return ea, fmt.Errorf("vorm/query: hasManyThrough %q needs through keys", rel.Name)
		}
		ea.RelatedTable = rel.PivotTable
		ea.RelatedCol = rel.PivotLocalKey
		ea.JoinTable = rel.Table
		farPK := rel.PivotForeignKey
		if farPK == "" {
			farPK = "id"
		}
		ea.JoinOnLeft = rel.PivotTable + "." + farPK
		ea.JoinOnRight = rel.Table + "." + rel.ForeignKey
	case RelationBelongsToMany, RelationMorphToMany:
		if rel.PivotTable == "" {
			return ea, fmt.Errorf("vorm/query: belongsToMany %q needs a pivot", rel.Name)
		}
		ea.RelatedTable = rel.PivotTable
		ea.RelatedCol = rel.PivotLocalKey
		ea.ParentCol = rel.LocalKey
		if rel.Kind == RelationMorphToMany && ea.MorphTypeCol == "" {
			ea.MorphTypeCol = morphTypeColumn(rel)
		}
	default:
		return ea, fmt.Errorf("vorm/query: WhereHas does not support %s", rel.Kind)
	}
	return ea, nil
}

func attachExistsJoin(ea *existsArg, rel Relation, extra pred) {
	if extra.col == "" {
		return
	}
	switch rel.Kind {
	case RelationBelongsToMany, RelationMorphToMany:
		pk := rel.ForeignKey
		if pk == "" {
			pk = "id"
		}
		ea.JoinTable = rel.Table
		ea.JoinOnLeft = rel.PivotTable + "." + rel.PivotForeignKey
		ea.JoinOnRight = rel.Table + "." + pk
	}
}

func morphTypeColumn(rel Relation) string {
	if rel.PivotTable != "" && strings.HasSuffix(strings.ToLower(rel.PivotLocalKey), "_id") {
		return strings.TrimSuffix(rel.PivotLocalKey, "_id") + "_type"
	}
	if strings.HasSuffix(strings.ToLower(rel.ForeignKey), "_id") {
		return strings.TrimSuffix(rel.ForeignKey, "_id") + "_type"
	}
	if rel.MorphType != "" {
		return "taggable_type"
	}
	return ""
}

func throughFarKey(rel Relation) string {
	if rel.PivotForeignKey != "" {
		return rel.PivotForeignKey
	}
	return "id"
}

// WithCount adds `(SELECT COUNT(*) FROM related WHERE …) AS {name}_count`.
func (b *Builder[T]) WithCount(names ...string) *Builder[T] {
	for _, name := range names {
		if err := b.addRelationSubselect(name, true); err != nil {
			b.wheres = append(b.wheres, pred{col: "__error__", op: "=", arg: err.Error()})
			return b
		}
	}
	return b
}

// WithExists adds `(SELECT EXISTS(SELECT 1 FROM related WHERE …)) AS {name}_exists`.
func (b *Builder[T]) WithExists(names ...string) *Builder[T] {
	for _, name := range names {
		if err := b.addRelationSubselect(name, false); err != nil {
			b.wheres = append(b.wheres, pred{col: "__error__", op: "=", arg: err.Error()})
			return b
		}
	}
	return b
}

func (b *Builder[T]) addRelationSubselect(name string, count bool) error {
	rel, ok := lookupRegistered(reflect.TypeFor[T](), name)
	if !ok {
		return fmt.Errorf("vorm/query: unknown relation %q", name)
	}
	ea, err := existsFromRelation(b.meta.Table, b.meta.PrimaryKey, rel.rel)
	if err != nil {
		return err
	}
	inner, _, err := b.compileExistsSQL(ea, nil)
	if err != nil {
		return err
	}
	alias := strings.ReplaceAll(name, ".", "_") + "_count"
	sqlText := "(" + strings.Replace(inner, "SELECT 1 FROM", "SELECT COUNT(*) FROM", 1) + ")"
	if !count {
		alias = strings.ReplaceAll(name, ".", "_") + "_exists"
		sqlText = "(SELECT EXISTS (" + inner + "))"
	}
	b.extraSelects = append(b.extraSelects, extraSelect{sql: sqlText, alias: alias})
	return nil
}
