package query

import (
	"context"
	"fmt"
	"strings"
)

// BelongsToManyAssoc is the Eloquent-style pivot association: Attach, Detach,
// Sync and Toggle. Generated models expose it as TagsRelation() so the method
// does not collide with the Tags []Tag eager-load field.
type BelongsToManyAssoc struct {
	PivotTable      string
	PivotParentKey  string
	PivotRelatedKey string
	ParentID        any
	Timestamps      bool
	Dialect         Dialect
}

func (a BelongsToManyAssoc) dialect() Dialect {
	if a.Dialect != "" {
		return a.Dialect
	}
	return DefaultDialect()
}

// Attach inserts pivot rows for related IDs (existing pairs are ignored).
func (a BelongsToManyAssoc) Attach(ctx context.Context, db DB, ids ...any) error {
	return a.attachRows(ctx, db, ids, nil)
}

// AttachWith inserts one pivot row and optional extra columns.
func (a BelongsToManyAssoc) AttachWith(ctx context.Context, db DB, id any, extra map[string]any) error {
	return a.attachRows(ctx, db, []any{id}, extra)
}

func (a BelongsToManyAssoc) attachRows(ctx context.Context, db DB, ids []any, extra map[string]any) error {
	if err := a.validate(); err != nil {
		return err
	}
	if a.ParentID == nil {
		return validationErr("attach", a.PivotTable, "parent id is required")
	}
	clean := make([]any, 0, len(ids))
	seen := map[any]bool{}
	for _, id := range ids {
		if id == nil {
			continue
		}
		k := normalizeKey(id)
		if k == nil || seen[k] {
			continue
		}
		seen[k] = true
		clean = append(clean, id)
	}
	if len(clean) == 0 {
		return nil
	}

	d := a.dialect()
	table, err := QuoteIdent(d, a.PivotTable)
	if err != nil {
		return err
	}
	parentCol, err := QuoteIdent(d, a.PivotParentKey)
	if err != nil {
		return err
	}
	relatedCol, err := QuoteIdent(d, a.PivotRelatedKey)
	if err != nil {
		return err
	}

	cols := []string{parentCol, relatedCol}
	extraKeys := sortedExtraKeys(extra)
	for _, k := range extraKeys {
		q, err := QuoteIdent(d, k)
		if err != nil {
			return err
		}
		cols = append(cols, q)
	}
	if a.Timestamps {
		created, err := QuoteIdent(d, "created_at")
		if err != nil {
			return err
		}
		updated, err := QuoteIdent(d, "updated_at")
		if err != nil {
			return err
		}
		cols = append(cols, created, updated)
	}

	var (
		placeholders []string
		args         []any
	)
	for _, id := range clean {
		row := make([]string, 0, len(cols))
		args = append(args, a.ParentID, id)
		row = append(row, a.nextPlaceholder(d, len(args)-1), a.nextPlaceholder(d, len(args)))
		for _, k := range extraKeys {
			args = append(args, extra[k])
			row = append(row, a.nextPlaceholder(d, len(args)))
		}
		if a.Timestamps {
			row = append(row, "CURRENT_TIMESTAMP", "CURRENT_TIMESTAMP")
		}
		placeholders = append(placeholders, "("+strings.Join(row, ", ")+")")
	}

	sqlText := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s", table, strings.Join(cols, ", "), strings.Join(placeholders, ", "))
	if d == DialectMySQL {
		sqlText = "INSERT IGNORE " + strings.TrimPrefix(sqlText, "INSERT ")
	} else {
		sqlText += fmt.Sprintf(" ON CONFLICT (%s, %s) DO NOTHING", parentCol, relatedCol)
	}

	obs := observe(ctx, "insert", a.PivotTable, sqlText, args)
	_, err = db.ExecContext(ctx, sqlText, args...)
	return obs.done(ctx, len(clean), wrapErr("insert", a.PivotTable, sqlText, len(args), err))
}

func (a BelongsToManyAssoc) nextPlaceholder(d Dialect, n int) string {
	if d == DialectMySQL {
		return "?"
	}
	return fmt.Sprintf("$%d", n)
}

// Detach removes pivot rows. With no IDs, every related row for the parent is removed.
func (a BelongsToManyAssoc) Detach(ctx context.Context, db DB, ids ...any) error {
	if err := a.validate(); err != nil {
		return err
	}
	if a.ParentID == nil {
		return validationErr("detach", a.PivotTable, "parent id is required")
	}
	d := a.dialect()
	table, err := QuoteIdent(d, a.PivotTable)
	if err != nil {
		return err
	}
	parentCol, err := QuoteIdent(d, a.PivotParentKey)
	if err != nil {
		return err
	}
	relatedCol, err := QuoteIdent(d, a.PivotRelatedKey)
	if err != nil {
		return err
	}

	args := []any{a.ParentID}
	sqlText := fmt.Sprintf("DELETE FROM %s WHERE %s = %s", table, parentCol, a.nextPlaceholder(d, 1))
	if len(ids) > 0 {
		holders := make([]string, 0, len(ids))
		for _, id := range ids {
			args = append(args, id)
			holders = append(holders, a.nextPlaceholder(d, len(args)))
		}
		sqlText += fmt.Sprintf(" AND %s IN (%s)", relatedCol, strings.Join(holders, ", "))
	}
	obs := observe(ctx, "delete", a.PivotTable, sqlText, args)
	_, err = db.ExecContext(ctx, sqlText, args...)
	return obs.done(ctx, 0, wrapErr("delete", a.PivotTable, sqlText, len(args), err))
}

// Sync makes the pivot match ids: missing rows are attached, extras detached.
func (a BelongsToManyAssoc) Sync(ctx context.Context, db DB, ids ...any) error {
	current, err := a.relatedIDs(ctx, db)
	if err != nil {
		return err
	}
	want := map[any]any{}
	for _, id := range ids {
		if id == nil {
			continue
		}
		want[normalizeKey(id)] = id
	}
	var detach, attach []any
	seen := map[any]bool{}
	for _, cur := range current {
		k := normalizeKey(cur)
		if _, ok := want[k]; !ok {
			detach = append(detach, cur)
		}
		seen[k] = true
	}
	for k, id := range want {
		if !seen[k] {
			attach = append(attach, id)
		}
	}
	if len(detach) > 0 {
		if err := a.Detach(ctx, db, detach...); err != nil {
			return err
		}
	}
	if len(attach) > 0 {
		if err := a.Attach(ctx, db, attach...); err != nil {
			return err
		}
	}
	return nil
}

// Toggle attaches IDs that are missing and detaches IDs that are present.
func (a BelongsToManyAssoc) Toggle(ctx context.Context, db DB, ids ...any) error {
	current, err := a.relatedIDs(ctx, db)
	if err != nil {
		return err
	}
	have := map[any]bool{}
	for _, cur := range current {
		have[normalizeKey(cur)] = true
	}
	var detach, attach []any
	for _, id := range ids {
		if id == nil {
			continue
		}
		if have[normalizeKey(id)] {
			detach = append(detach, id)
		} else {
			attach = append(attach, id)
		}
	}
	if len(detach) > 0 {
		if err := a.Detach(ctx, db, detach...); err != nil {
			return err
		}
	}
	if len(attach) > 0 {
		if err := a.Attach(ctx, db, attach...); err != nil {
			return err
		}
	}
	return nil
}

func (a BelongsToManyAssoc) relatedIDs(ctx context.Context, db DB) ([]any, error) {
	d := a.dialect()
	table, err := QuoteIdent(d, a.PivotTable)
	if err != nil {
		return nil, err
	}
	parentCol, err := QuoteIdent(d, a.PivotParentKey)
	if err != nil {
		return nil, err
	}
	relatedCol, err := QuoteIdent(d, a.PivotRelatedKey)
	if err != nil {
		return nil, err
	}
	sqlText := fmt.Sprintf("SELECT %s FROM %s WHERE %s = %s", relatedCol, table, parentCol, a.nextPlaceholder(d, 1))
	args := []any{a.ParentID}
	obs := observe(ctx, "select", a.PivotTable, sqlText, args)
	rows, err := db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, obs.done(ctx, 0, wrapErr("select", a.PivotTable, sqlText, 1, err))
	}
	defer rows.Close()
	var out []any
	for rows.Next() {
		var id any
		if err := rows.Scan(&id); err != nil {
			return nil, obs.done(ctx, len(out), wrapErr("scan", a.PivotTable, sqlText, 1, err))
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, obs.done(ctx, len(out), wrapErr("select", a.PivotTable, sqlText, 1, err))
	}
	_ = obs.done(ctx, len(out), nil)
	return out, nil
}

func (a BelongsToManyAssoc) validate() error {
	if a.PivotTable == "" || a.PivotParentKey == "" || a.PivotRelatedKey == "" {
		return validationErr("attach", a.PivotTable, "pivot table and key columns are required")
	}
	return nil
}

func sortedExtraKeys(extra map[string]any) []string {
	if len(extra) == 0 {
		return nil
	}
	keys := make([]string, 0, len(extra))
	for k := range extra {
		keys = append(keys, k)
	}
	// small n; insertion order is enough if we sort for stable SQL
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}
