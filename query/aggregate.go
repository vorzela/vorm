package query

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Pluck selects a single column into a slice. Never SELECT *.
func (b *Builder[T]) Pluck(ctx context.Context, db DB, col string) ([]any, error) {
	if err := b.meta.RequireColumn(col); err != nil {
		return nil, err
	}
	cp := b.clone()
	cp.selects = []string{col}
	cp.extraSelects = nil
	sqlText, args, err := cp.CompileSelect()
	if err != nil {
		return nil, err
	}
	obs := observe(ctx, "select", b.meta.Table, sqlText, args)
	rows, err := db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, obs.done(ctx, 0, wrapErr("select", b.meta.Table, sqlText, len(args), err))
	}
	defer rows.Close()
	var out []any
	for rows.Next() {
		var v any
		if err := rows.Scan(&v); err != nil {
			return nil, obs.done(ctx, len(out), wrapErr("scan", b.meta.Table, sqlText, len(args), err))
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, obs.done(ctx, len(out), wrapErr("select", b.meta.Table, sqlText, len(args), err))
	}
	_ = obs.done(ctx, len(out), nil)
	return out, nil
}

// Value returns the first row's column, or (nil, nil) when empty.
func (b *Builder[T]) Value(ctx context.Context, db DB, col string) (any, error) {
	cp := b.clone()
	cp.limit = 1
	vals, err := cp.Pluck(ctx, db, col)
	if err != nil || len(vals) == 0 {
		return nil, err
	}
	return vals[0], nil
}

// Sum returns SUM(col) as float64.
func (b *Builder[T]) Sum(ctx context.Context, db DB, col string) (float64, error) {
	return b.aggregate(ctx, db, "SUM", col)
}

// Avg returns AVG(col) as float64.
func (b *Builder[T]) Avg(ctx context.Context, db DB, col string) (float64, error) {
	return b.aggregate(ctx, db, "AVG", col)
}

// Min returns MIN(col) as float64.
func (b *Builder[T]) Min(ctx context.Context, db DB, col string) (float64, error) {
	return b.aggregate(ctx, db, "MIN", col)
}

// Max returns MAX(col) as float64.
func (b *Builder[T]) Max(ctx context.Context, db DB, col string) (float64, error) {
	return b.aggregate(ctx, db, "MAX", col)
}

func (b *Builder[T]) aggregate(ctx context.Context, db DB, fn, col string) (float64, error) {
	if err := b.meta.RequireColumn(col); err != nil {
		return 0, err
	}
	fn = strings.ToUpper(fn)
	switch fn {
	case "SUM", "AVG", "MIN", "MAX":
	default:
		return 0, fmt.Errorf("vorm/query: unknown aggregate %q", fn)
	}
	tableQ, err := QuoteIdent(b.dialect, b.meta.Table)
	if err != nil {
		return 0, err
	}
	colQ, err := QuoteIdent(b.dialect, b.qualify(col))
	if err != nil {
		return 0, err
	}
	whereSQL, args, err := b.compileWhere(1)
	if err != nil {
		return 0, err
	}
	var sb strings.Builder
	sb.WriteString("SELECT ")
	sb.WriteString(fn)
	sb.WriteString("(")
	sb.WriteString(colQ)
	sb.WriteString(") FROM ")
	sb.WriteString(tableQ)
	sb.WriteString(compileJoinsQuoted(b.joins, b.dialect))
	if whereSQL != "" {
		sb.WriteString(" WHERE ")
		sb.WriteString(whereSQL)
	}
	sqlText := sb.String()
	obs := observe(ctx, "select", b.meta.Table, sqlText, args)
	var n sql.NullFloat64
	if err := db.QueryRowContext(ctx, sqlText, args...).Scan(&n); err != nil {
		return 0, obs.done(ctx, 0, wrapErr("select", b.meta.Table, sqlText, len(args), err))
	}
	_ = obs.done(ctx, 1, nil)
	if !n.Valid {
		return 0, nil
	}
	return n.Float64, nil
}
