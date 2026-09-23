package query

import (
	"context"
	"fmt"
	"strings"
)

// Increment adds amount (default 1) to col: SET "age" = "age" + $1.
func (b *Builder[T]) Increment(ctx context.Context, db DB, col string, amount ...int64) (int64, error) {
	n := int64(1)
	if len(amount) > 0 {
		n = amount[0]
	}
	return b.addToColumn(ctx, db, col, n)
}

// Decrement subtracts amount (default 1) from col.
func (b *Builder[T]) Decrement(ctx context.Context, db DB, col string, amount ...int64) (int64, error) {
	n := int64(1)
	if len(amount) > 0 {
		n = amount[0]
	}
	return b.addToColumn(ctx, db, col, -n)
}

func (b *Builder[T]) addToColumn(ctx context.Context, db DB, col string, delta int64) (int64, error) {
	if err := b.meta.RequireColumn(col); err != nil {
		return 0, err
	}
	tableQ, err := QuoteIdent(b.dialect, b.meta.Table)
	if err != nil {
		return 0, err
	}
	colQ, err := QuoteIdent(b.dialect, col)
	if err != nil {
		return 0, err
	}
	ph := "$1"
	if b.dialect == DialectMySQL {
		ph = "?"
	}
	whereSQL, whereArgs, err := b.compileWhere(2)
	if err != nil {
		return 0, err
	}
	sqlText := fmt.Sprintf("UPDATE %s SET %s = %s + %s", tableQ, colQ, colQ, ph)
	args := []any{delta}
	if whereSQL != "" {
		sqlText += " WHERE " + whereSQL
		args = append(args, whereArgs...)
	}
	return b.execAffected(ctx, db, "update", sqlText, args)
}

// Upsert inserts rows, updating updateCols on unique conflict.
//
//	Users.Upsert(ctx, db, []map[string]any{{"email": e, "name": n}}, []string{"email"}, []string{"name"})
func (b *Builder[T]) Upsert(ctx context.Context, db DB, rows []map[string]any, uniqueCols, updateCols []string) (int64, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	keys := sortedKeys(rows[0])
	if len(keys) == 0 {
		return 0, validationErr("upsert", b.meta.Table, "Upsert requires values")
	}
	for _, c := range append(append([]string{}, uniqueCols...), updateCols...) {
		if err := b.meta.RequireColumn(c); err != nil {
			return 0, err
		}
	}
	tableQ, err := QuoteIdent(b.dialect, b.meta.Table)
	if err != nil {
		return 0, err
	}
	cols := make([]string, len(keys))
	for i, k := range keys {
		qc, err := QuoteIdent(b.dialect, k)
		if err != nil {
			return 0, err
		}
		cols[i] = qc
	}

	args := make([]any, 0, len(rows)*len(keys))
	tuples := make([]string, 0, len(rows))
	n := 1
	for _, row := range rows {
		holders := make([]string, len(keys))
		for i, k := range keys {
			v, ok := row[k]
			if !ok {
				return 0, validationErr("upsert", b.meta.Table, "row is missing column %q", k)
			}
			if err := b.meta.CheckColumnValue(k, v); err != nil {
				return 0, err
			}
			if b.dialect == DialectMySQL {
				holders[i] = "?"
			} else {
				holders[i] = fmt.Sprintf("$%d", n)
				n++
			}
			args = append(args, v)
		}
		tuples = append(tuples, "("+strings.Join(holders, ", ")+")")
	}
	sqlText := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s", tableQ, strings.Join(cols, ", "), strings.Join(tuples, ", "))
	if len(updateCols) == 0 {
		if b.dialect == DialectMySQL {
			sqlText = "INSERT IGNORE " + strings.TrimPrefix(sqlText, "INSERT ")
		} else if len(uniqueCols) > 0 {
			uq, err := quoteIdentList(b.dialect, uniqueCols)
			if err != nil {
				return 0, err
			}
			sqlText += " ON CONFLICT (" + strings.Join(uq, ", ") + ") DO NOTHING"
		}
	} else if b.dialect == DialectMySQL {
		sets := make([]string, len(updateCols))
		for i, c := range updateCols {
			qc, err := QuoteIdent(b.dialect, c)
			if err != nil {
				return 0, err
			}
			sets[i] = fmt.Sprintf("%s = VALUES(%s)", qc, qc)
		}
		sqlText += " ON DUPLICATE KEY UPDATE " + strings.Join(sets, ", ")
	} else {
		if len(uniqueCols) == 0 {
			return 0, validationErr("upsert", b.meta.Table, "ON CONFLICT needs unique columns")
		}
		uq, err := quoteIdentList(b.dialect, uniqueCols)
		if err != nil {
			return 0, err
		}
		sets := make([]string, len(updateCols))
		for i, c := range updateCols {
			qc, err := QuoteIdent(b.dialect, c)
			if err != nil {
				return 0, err
			}
			sets[i] = fmt.Sprintf("%s = EXCLUDED.%s", qc, qc)
		}
		sqlText += " ON CONFLICT (" + strings.Join(uq, ", ") + ") DO UPDATE SET " + strings.Join(sets, ", ")
	}

	obs := observe(ctx, "upsert", b.meta.Table, sqlText, args)
	res, err := db.ExecContext(ctx, sqlText, args...)
	if err != nil {
		return 0, obs.done(ctx, 0, wrapErr("upsert", b.meta.Table, sqlText, len(args), err))
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, obs.done(ctx, 0, wrapErr("upsert", b.meta.Table, sqlText, len(args), err))
	}
	_ = obs.done(ctx, int(affected), nil)
	return affected, nil
}

func quoteIdentList(d Dialect, cols []string) ([]string, error) {
	out := make([]string, len(cols))
	for i, c := range cols {
		q, err := QuoteIdent(d, c)
		if err != nil {
			return nil, err
		}
		out[i] = q
	}
	return out, nil
}

// FirstOrCreate finds by attrs or inserts attrs merged with values.
func (b *Builder[T]) FirstOrCreate(ctx context.Context, db DB, attrs map[string]any, values ...map[string]any) (*T, error) {
	cp := b.clone()
	for _, k := range sortedKeys(attrs) {
		cp.Where(k, attrs[k])
	}
	row, err := cp.First(ctx, db)
	if err != nil || row != nil {
		return row, err
	}
	merged := map[string]any{}
	for k, v := range attrs {
		merged[k] = v
	}
	if len(values) > 0 {
		for k, v := range values[0] {
			merged[k] = v
		}
	}
	id, err := b.Create(ctx, db, merged)
	if err != nil {
		return nil, err
	}
	return b.clone().Where(b.meta.PrimaryKey, id).First(ctx, db)
}

// UpdateOrCreate finds by attrs and updates, or inserts attrs merged with values.
func (b *Builder[T]) UpdateOrCreate(ctx context.Context, db DB, attrs, values map[string]any) (*T, error) {
	cp := b.clone()
	for _, k := range sortedKeys(attrs) {
		cp.Where(k, attrs[k])
	}
	row, err := cp.First(ctx, db)
	if err != nil {
		return nil, err
	}
	if row != nil {
		key := CursorValue(*row, b.meta.PrimaryKey)
		if key == nil {
			return nil, validationErr("update", b.meta.Table, "could not read primary key")
		}
		if len(values) > 0 {
			if _, err := b.clone().Where(b.meta.PrimaryKey, key).Update(ctx, db, values); err != nil {
				return nil, err
			}
		}
		return b.clone().Where(b.meta.PrimaryKey, key).First(ctx, db)
	}
	merged := map[string]any{}
	for k, v := range attrs {
		merged[k] = v
	}
	for k, v := range values {
		merged[k] = v
	}
	id, err := b.Create(ctx, db, merged)
	if err != nil {
		return nil, err
	}
	return b.clone().Where(b.meta.PrimaryKey, id).First(ctx, db)
}
