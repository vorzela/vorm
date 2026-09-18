package query

import (
	"context"
	"fmt"
	"strings"
)

// MorphTo loads a polymorphic parent. TypeColumn values are table names (or
// keys in Targets); each matching group is fetched in one query.
type MorphTo[P any] struct {
	ParentType func(*P) string
	ParentID   func(*P) any
	Assign     func(*P, any)
	Targets    map[string]MorphLoader
}

// MorphLoader fetches related rows for a batch of ids and returns them keyed
// by primary key (any comparable form; LoadMorphTo normalizes).
type MorphLoader func(ctx context.Context, db DB, ids []any) (map[any]any, error)

// LoadMorphTo resolves morphTo for a parent batch.
func LoadMorphTo[P any](ctx context.Context, db DB, parents []*P, opts MorphTo[P]) error {
	if opts.ParentType == nil || opts.ParentID == nil || opts.Assign == nil {
		return validationErr("with", "", "LoadMorphTo requires ParentType, ParentID and Assign")
	}
	groups := map[string][]*P{}
	for _, p := range parents {
		if p == nil {
			continue
		}
		typ := strings.TrimSpace(opts.ParentType(p))
		if typ == "" {
			continue
		}
		groups[typ] = append(groups[typ], p)
	}
	for typ, group := range groups {
		load, ok := opts.Targets[typ]
		if !ok || load == nil {
			continue
		}
		keys, _ := distinctKeys(group, opts.ParentID)
		if len(keys) == 0 {
			continue
		}
		found, err := load(ctx, db, keys)
		if err != nil {
			return fmt.Errorf("vorm/query: morphTo %q: %w", typ, err)
		}
		norm := make(map[any]any, len(found))
		for k, v := range found {
			norm[normalizeKey(k)] = v
		}
		for _, p := range group {
			if v, ok := norm[normalizeKey(opts.ParentID(p))]; ok {
				opts.Assign(p, v)
			}
		}
	}
	return nil
}

// MorphLoad is a helper that builds a MorphLoader from an Entity.
func MorphLoad[C any](related *Entity[C], id func(*C) any) MorphLoader {
	return func(ctx context.Context, db DB, ids []any) (map[any]any, error) {
		if related == nil || id == nil {
			return nil, validationErr("with", "", "MorphLoad requires Related and id")
		}
		pk := related.meta.PrimaryKey
		if pk == "" {
			pk = "id"
		}
		rows, err := related.New().WhereIn(pk, ids...).Get(ctx, db)
		if err != nil {
			return nil, err
		}
		out := make(map[any]any, len(rows))
		for i := range rows {
			out[normalizeKey(id(&rows[i]))] = &rows[i]
		}
		return out, nil
	}
}
