package query

import "context"

// Find runs FindOptions then Get.
func (e *Entity[T]) Find(ctx context.Context, db DB, opts FindOptions) ([]T, error) {
	return e.New().ApplyFind(opts).Get(ctx, db)
}

// FindOne returns the first match for FindOptions.
func (e *Entity[T]) FindOne(ctx context.Context, db DB, opts FindOptions) (*T, error) {
	opts.Take = 1
	return e.New().ApplyFind(opts).First(ctx, db)
}

// FindByID loads by primary key.
func (e *Entity[T]) FindByID(ctx context.Context, db DB, id any) (*T, error) {
	return e.New().Where(e.meta.PrimaryKey, id).First(ctx, db)
}

// Create inserts values.
func (e *Entity[T]) Create(ctx context.Context, db DB, values map[string]any) (int64, error) {
	return e.New().Create(ctx, db, values)
}

// Update applies values with no WHERE — prefer Users.Where(...).Update(...).
func (e *Entity[T]) Update(ctx context.Context, db DB, values map[string]any) (int64, error) {
	return e.New().Update(ctx, db, values)
}

// SoftDelete soft-deletes by primary key (sets deleted_at).
func (e *Entity[T]) SoftDelete(ctx context.Context, db DB, id any) (int64, error) {
	return e.New().Where(e.meta.PrimaryKey, id).Delete(ctx, db)
}

// ForceDelete permanently deletes by primary key (ignores soft deletes).
func (e *Entity[T]) ForceDelete(ctx context.Context, db DB, id any) (int64, error) {
	return e.New().Where(e.meta.PrimaryKey, id).ForceDelete(ctx, db)
}

// Paginate runs offset/cursor pagination from the entity root.
func (e *Entity[T]) Paginate(ctx context.Context, db DB, req PageRequest) (*PageResult[T], error) {
	return e.New().Paginate(ctx, db, req)
}

// SimplePaginate is Laravel simplePaginate from the entity root.
func (e *Entity[T]) SimplePaginate(ctx context.Context, db DB, page, perPage int) (*PageResult[T], error) {
	return e.New().SimplePaginate(ctx, db, page, perPage)
}

// CursorPaginate is Laravel cursorPaginate from the entity root.
func (e *Entity[T]) CursorPaginate(ctx context.Context, db DB, cursor string, perPage int) (*PageResult[T], error) {
	return e.New().CursorPaginate(ctx, db, cursor, perPage)
}

// ChunkByID walks matching rows by primary key, one page at a time.
func (e *Entity[T]) ChunkByID(ctx context.Context, db DB, size int, fn func([]T) error) error {
	return e.New().ChunkByID(ctx, db, size, fn)
}

// Chunk is ChunkByID when the model has a primary key.
func (e *Entity[T]) Chunk(ctx context.Context, db DB, size int, fn func([]T) error) error {
	return e.New().Chunk(ctx, db, size, fn)
}

// LazyByID streams rows one at a time using ChunkByID pages.
func (e *Entity[T]) LazyByID(ctx context.Context, db DB, size int, fn func(T) error) error {
	return e.New().LazyByID(ctx, db, size, fn)
}

// Pluck selects a single column into a slice.
func (e *Entity[T]) Pluck(ctx context.Context, db DB, col string) ([]any, error) {
	return e.New().Pluck(ctx, db, col)
}

// Value returns the first row's column.
func (e *Entity[T]) Value(ctx context.Context, db DB, col string) (any, error) {
	return e.New().Value(ctx, db, col)
}

// Sum returns SUM(col).
func (e *Entity[T]) Sum(ctx context.Context, db DB, col string) (float64, error) {
	return e.New().Sum(ctx, db, col)
}

// Avg returns AVG(col).
func (e *Entity[T]) Avg(ctx context.Context, db DB, col string) (float64, error) {
	return e.New().Avg(ctx, db, col)
}

// Min returns MIN(col).
func (e *Entity[T]) Min(ctx context.Context, db DB, col string) (float64, error) {
	return e.New().Min(ctx, db, col)
}

// Max returns MAX(col).
func (e *Entity[T]) Max(ctx context.Context, db DB, col string) (float64, error) {
	return e.New().Max(ctx, db, col)
}

// Increment adds amount (default 1) to col.
func (e *Entity[T]) Increment(ctx context.Context, db DB, col string, amount ...int64) (int64, error) {
	return e.New().Increment(ctx, db, col, amount...)
}

// Decrement subtracts amount (default 1) from col.
func (e *Entity[T]) Decrement(ctx context.Context, db DB, col string, amount ...int64) (int64, error) {
	return e.New().Decrement(ctx, db, col, amount...)
}

// Upsert inserts rows, updating updateCols on unique conflict.
func (e *Entity[T]) Upsert(ctx context.Context, db DB, rows []map[string]any, uniqueCols, updateCols []string) (int64, error) {
	return e.New().Upsert(ctx, db, rows, uniqueCols, updateCols)
}

// FirstOrCreate finds by attrs or inserts attrs merged with values.
func (e *Entity[T]) FirstOrCreate(ctx context.Context, db DB, attrs map[string]any, values ...map[string]any) (*T, error) {
	return e.New().FirstOrCreate(ctx, db, attrs, values...)
}

// UpdateOrCreate finds by attrs and updates, or inserts attrs merged with values.
func (e *Entity[T]) UpdateOrCreate(ctx context.Context, db DB, attrs, values map[string]any) (*T, error) {
	return e.New().UpdateOrCreate(ctx, db, attrs, values)
}
