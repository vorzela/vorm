package query

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

// HasManyThrough loads a distant relation through an intermediate table in one JOIN.
type HasManyThrough[P any, C any] struct {
	Related      *Entity[C]
	ThroughTable string
	ThroughLocal string // through column matching the parent key
	ThroughFar   string // through PK joined to the far table
	FarKey       string // far table FK to the through table
	ParentKey    func(*P) any
	Assign       func(*P, []C)
}

// LoadHasManyThrough runs one batched JOIN for the whole parent set.
func LoadHasManyThrough[P any, C any](ctx context.Context, db DB, parents []*P, opts HasManyThrough[P, C]) error {
	if opts.Related == nil || opts.ParentKey == nil || opts.Assign == nil {
		return validationErr("with", "", "LoadHasManyThrough requires Related, ParentKey and Assign")
	}
	if opts.ThroughTable == "" || opts.ThroughLocal == "" || opts.FarKey == "" {
		return validationErr("with", "", "LoadHasManyThrough requires through keys")
	}
	farPK := opts.ThroughFar
	if farPK == "" {
		farPK = "id"
	}
	keys, _ := distinctKeys(parents, opts.ParentKey)
	if len(keys) == 0 {
		return nil
	}
	d := opts.Related.New().dialect
	cols := opts.Related.meta.Columns
	sqlText, args, err := throughSQL(d, opts.Related.meta.Table, cols, opts.ThroughTable, opts.ThroughLocal, farPK, opts.FarKey, keys)
	if err != nil {
		return err
	}
	obs := observe(ctx, "select", opts.Related.meta.Table, sqlText, args)
	rows, err := db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return obs.done(ctx, 0, wrapErr("select", opts.Related.meta.Table, sqlText, len(args), err))
	}
	defer rows.Close()

	grouped := map[any][]C{}
	n := 0
	for rows.Next() {
		child, parentKey, err := scanWithParentKey[C](rows, cols)
		if err != nil {
			return obs.done(ctx, n, wrapErr("scan", opts.Related.meta.Table, sqlText, len(args), err))
		}
		grouped[normalizeKey(parentKey)] = append(grouped[normalizeKey(parentKey)], child)
		n++
	}
	if err := rows.Err(); err != nil {
		return obs.done(ctx, n, wrapErr("select", opts.Related.meta.Table, sqlText, len(args), err))
	}
	_ = obs.done(ctx, n, nil)
	for _, p := range parents {
		opts.Assign(p, grouped[normalizeKey(opts.ParentKey(p))])
	}
	return nil
}

func throughSQL(d Dialect, farTable string, cols []string, through, throughLocal, throughFar, farKey string, keys []any) (string, []any, error) {
	quoted := make([]string, len(cols))
	for i, c := range cols {
		q, err := QuoteIdent(d, farTable+"."+c)
		if err != nil {
			return "", nil, err
		}
		quoted[i] = q
	}
	throughQ, err := QuoteIdent(d, through)
	if err != nil {
		return "", nil, err
	}
	farQ, err := QuoteIdent(d, farTable)
	if err != nil {
		return "", nil, err
	}
	localQ, err := QuoteIdent(d, through+"."+throughLocal)
	if err != nil {
		return "", nil, err
	}
	throughFarQ, err := QuoteIdent(d, through+"."+throughFar)
	if err != nil {
		return "", nil, err
	}
	farKeyQ, err := QuoteIdent(d, farTable+"."+farKey)
	if err != nil {
		return "", nil, err
	}
	holders := make([]string, len(keys))
	for i := range keys {
		if d == DialectMySQL {
			holders[i] = "?"
		} else {
			holders[i] = fmt.Sprintf("$%d", i+1)
		}
	}
	sqlText := fmt.Sprintf("SELECT %s, %s FROM %s INNER JOIN %s ON %s = %s WHERE %s IN (%s)",
		strings.Join(quoted, ", "), localQ, farQ, throughQ, throughFarQ, farKeyQ, localQ, joinComma(holders))
	return sqlText, keys, nil
}

func scanWithParentKey[C any](rows Rows, cols []string) (C, any, error) {
	var zero C
	rt := reflect.TypeFor[C]()
	if rt.Kind() == reflect.Pointer {
		return zero, nil, fmt.Errorf("vorm/query: through scan needs a struct value")
	}
	plan := planFor(rt, cols)
	ptr := reflect.New(rt)
	dest := make([]any, len(cols)+1)
	sinks := make([]any, len(cols)+1)
	bindDest(ptr.Elem(), plan, dest[:len(cols)], sinks[:len(cols)])
	var parentKey any
	dest[len(cols)] = &parentKey
	if err := rows.Scan(dest...); err != nil {
		return zero, nil, err
	}
	return ptr.Elem().Interface().(C), parentKey, nil
}

// HasOneOfMany loads the latest/oldest child per parent.
type HasOneOfMany[P any, C any] struct {
	Related    *Entity[C]
	ForeignKey string
	OrderCol   string
	Desc       bool
	ParentKey  func(*P) any
	ChildKey   func(*C) any
	Assign     func(*P, *C)
}

// LoadHasOneOfMany uses DISTINCT ON (Postgres) or a MAX-join (MySQL).
func LoadHasOneOfMany[P any, C any](ctx context.Context, db DB, parents []*P, opts HasOneOfMany[P, C]) error {
	if opts.Related == nil || opts.ParentKey == nil || opts.ChildKey == nil || opts.Assign == nil {
		return validationErr("with", "", "LoadHasOneOfMany requires Related, ParentKey, ChildKey and Assign")
	}
	keys, _ := distinctKeys(parents, opts.ParentKey)
	if len(keys) == 0 {
		return nil
	}
	fk := opts.ForeignKey
	orderCol := opts.OrderCol
	if orderCol == "" {
		orderCol = "created_at"
		if err := opts.Related.meta.RequireColumn(orderCol); err != nil {
			orderCol = opts.Related.meta.PrimaryKey
		}
	}
	d := opts.Related.New().dialect
	var (
		rows []C
		err  error
	)
	if d == DialectMySQL {
		rows, err = loadOfManyMySQL(ctx, db, opts.Related, fk, orderCol, opts.Desc, keys)
	} else {
		b := opts.Related.New().WhereIn(fk, keys...).DistinctOn(fk)
		dir := "ASC"
		if opts.Desc {
			dir = "DESC"
		}
		b.OrderBy(fk, "ASC").OrderBy(orderCol, dir)
		rows, err = b.Get(ctx, db)
	}
	if err != nil {
		return err
	}
	byKey := map[any]*C{}
	for i := range rows {
		byKey[normalizeKey(opts.ChildKey(&rows[i]))] = &rows[i]
	}
	for _, p := range parents {
		if c, ok := byKey[normalizeKey(opts.ParentKey(p))]; ok {
			opts.Assign(p, c)
		}
	}
	return nil
}

func loadOfManyMySQL[C any](ctx context.Context, db DB, related *Entity[C], fk, orderCol string, desc bool, keys []any) ([]C, error) {
	d := DialectMySQL
	tableQ, err := QuoteIdent(d, related.meta.Table)
	if err != nil {
		return nil, err
	}
	proj := make([]string, len(related.meta.Columns))
	for i, c := range related.meta.Columns {
		proj[i], err = QuoteIdent(d, "t."+c)
		if err != nil {
			return nil, err
		}
	}
	fkQ, err := QuoteIdent(d, fk)
	if err != nil {
		return nil, err
	}
	ordQ, err := QuoteIdent(d, orderCol)
	if err != nil {
		return nil, err
	}
	tFK, err := QuoteIdent(d, "t."+fk)
	if err != nil {
		return nil, err
	}
	gFK, err := QuoteIdent(d, "g."+fk)
	if err != nil {
		return nil, err
	}
	tOrd, err := QuoteIdent(d, "t."+orderCol)
	if err != nil {
		return nil, err
	}
	agg := "MAX"
	if !desc {
		agg = "MIN"
	}
	holders := make([]string, len(keys))
	for i := range keys {
		holders[i] = "?"
	}
	in := joinComma(holders)
	sqlText := fmt.Sprintf(
		"SELECT %s FROM %s AS `t` INNER JOIN (SELECT %s, %s(%s) AS `_ord` FROM %s WHERE %s IN (%s) GROUP BY %s) AS `g` ON %s = %s AND %s = `g`.`_ord` WHERE %s IN (%s)",
		strings.Join(proj, ", "), tableQ, fkQ, agg, ordQ, tableQ, fkQ, in, fkQ, tFK, gFK, tOrd, tFK, in,
	)
	args := append(append([]any{}, keys...), keys...)
	obs := observe(ctx, "select", related.meta.Table, sqlText, args)
	rs, err := db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, obs.done(ctx, 0, wrapErr("select", related.meta.Table, sqlText, len(args), err))
	}
	defer rs.Close()
	mapper, err := structScanner[C](related.meta.Columns)
	if err != nil {
		return nil, err
	}
	var out []C
	for rs.Next() {
		row, err := mapper(rs)
		if err != nil {
			return nil, obs.done(ctx, len(out), wrapErr("scan", related.meta.Table, sqlText, len(args), err))
		}
		out = append(out, row)
	}
	if err := rs.Err(); err != nil {
		return nil, obs.done(ctx, len(out), wrapErr("select", related.meta.Table, sqlText, len(args), err))
	}
	_ = obs.done(ctx, len(out), nil)
	return out, nil
}
