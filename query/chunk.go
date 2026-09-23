package query

import (
	"context"
	"fmt"
)

// ChunkByID walks matching rows in primary-key order, one page at a time.
// Each callback receives at most size rows. Memory stays bounded; the SQL is
// keyset (`WHERE id > $1 ORDER BY id LIMIT $2`), never OFFSET.
func (b *Builder[T]) ChunkByID(ctx context.Context, db DB, size int, fn func([]T) error) error {
	if size <= 0 {
		size = 1000
	}
	pk := b.meta.PrimaryKey
	if pk == "" {
		return validationErr("select", b.meta.Table, "ChunkByID requires a primary key")
	}
	var last any
	for {
		cp := b.clone()
		cp.orderBy = []order{{col: pk, dir: "ASC"}}
		cp.limit = size
		if last != nil {
			cp.wheres = append(cp.wheres, pred{col: pk, op: ">", arg: last})
		}
		rows, err := cp.Get(ctx, db)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		if err := fn(rows); err != nil {
			return err
		}
		last = CursorValue(rows[len(rows)-1], pk)
		if last == nil {
			return fmt.Errorf("vorm/query: ChunkByID could not read %s from the last row", pk)
		}
		if len(rows) < size {
			return nil
		}
	}
}

// Chunk is ChunkByID when the model has a primary key (OFFSET chunking is not generated).
func (b *Builder[T]) Chunk(ctx context.Context, db DB, size int, fn func([]T) error) error {
	return b.ChunkByID(ctx, db, size, fn)
}

// LazyByID streams rows one at a time using ChunkByID pages internally.
func (b *Builder[T]) LazyByID(ctx context.Context, db DB, size int, fn func(T) error) error {
	return b.ChunkByID(ctx, db, size, func(rows []T) error {
		for _, row := range rows {
			if err := fn(row); err != nil {
				return err
			}
		}
		return nil
	})
}
