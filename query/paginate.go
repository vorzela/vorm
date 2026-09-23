package query

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// PageStyle is the caller's preferred pagination strategy.
type PageStyle string

const (
	PageOffset PageStyle = "offset" // COUNT + LIMIT/OFFSET (Laravel paginate)
	PageSimple PageStyle = "simple" // LIMIT perPage+1 + OFFSET, no COUNT (simplePaginate)
	PageCursor PageStyle = "cursor" // keyset, no COUNT (cursorPaginate)
)

// PageRequest is a unified pagination input (offset, simple, or cursor).
type PageRequest struct {
	Style   PageStyle // default offset
	Page    int       // 1-based page number (offset/simple)
	PerPage int       // page size; default 15
	// Cursor style:
	Cursor  string // opaque cursor from previous PageResult.NextCursor
	OrderBy string // cursor column (default primary key)
	Desc    bool
}

// PageResult is a page of rows plus navigation metadata.
type PageResult[T any] struct {
	Data       []T    `json:"data"`
	Style      string `json:"style"`
	PerPage    int    `json:"per_page"`
	Page       int    `json:"page,omitempty"`        // offset/simple — current 1-based page
	Pages      int    `json:"pages,omitempty"`       // offset only — total number of pages
	LastPage   int    `json:"last_page,omitempty"`   // same as Pages (Laravel-style alias)
	Total      *int64 `json:"total,omitempty"`       // offset only — total matching rows
	NextCursor string `json:"next_cursor,omitempty"` // cursor only
	HasMore    bool   `json:"has_more"`
}

// TotalCount returns the total row count, or -1 when no count was run
// (simple and cursor pages).
func (p *PageResult[T]) TotalCount() int64 {
	if p == nil || p.Total == nil {
		return -1
	}
	return *p.Total
}

type cursorPayload struct {
	V any    `json:"v"`
	C string `json:"c"`
}

// Paginate runs offset, simple, or cursor pagination based on req.Style.
func (b *Builder[T]) Paginate(ctx context.Context, db DB, req PageRequest) (*PageResult[T], error) {
	if req.PerPage <= 0 {
		req.PerPage = 15
	}
	switch req.Style {
	case PageSimple:
		return b.paginateSimple(ctx, db, req)
	case PageCursor:
		return b.paginateCursor(ctx, db, req)
	default:
		return b.paginateOffset(ctx, db, req)
	}
}

func (b *Builder[T]) paginateOffset(ctx context.Context, db DB, req PageRequest) (*PageResult[T], error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	cp := b.clone()
	cp.limit = req.PerPage
	cp.offset = (req.Page - 1) * req.PerPage

	rows, err := cp.Get(ctx, db)
	if err != nil {
		return nil, err
	}
	total, err := b.Count(ctx, db)
	if err != nil {
		return nil, err
	}
	pages := pageCount(total, req.PerPage)
	hasMore := req.Page < pages
	return &PageResult[T]{
		Data:     rows,
		Style:    string(PageOffset),
		PerPage:  req.PerPage,
		Page:     req.Page,
		Pages:    pages,
		LastPage: pages,
		Total:    &total,
		HasMore:  hasMore,
	}, nil
}

func (b *Builder[T]) paginateSimple(ctx context.Context, db DB, req PageRequest) (*PageResult[T], error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	cp := b.clone()
	cp.limit = req.PerPage + 1
	cp.offset = (req.Page - 1) * req.PerPage
	rows, err := cp.Get(ctx, db)
	if err != nil {
		return nil, err
	}
	hasMore := len(rows) > req.PerPage
	if hasMore {
		rows = rows[:req.PerPage]
	}
	return &PageResult[T]{
		Data:    rows,
		Style:   string(PageSimple),
		PerPage: req.PerPage,
		Page:    req.Page,
		HasMore: hasMore,
	}, nil
}

// pageCount returns how many pages are needed for total rows at perPage size.
func pageCount(total int64, perPage int) int {
	if perPage <= 0 {
		return 0
	}
	if total <= 0 {
		return 0
	}
	return int((total + int64(perPage) - 1) / int64(perPage))
}

func (b *Builder[T]) paginateCursor(ctx context.Context, db DB, req PageRequest) (*PageResult[T], error) {
	col := req.OrderBy
	if col == "" {
		if len(b.orderBy) > 0 {
			col = b.orderBy[0].col
			if !req.Desc && strings.EqualFold(b.orderBy[0].dir, "DESC") {
				req.Desc = true
			}
		} else {
			col = b.meta.PrimaryKey
		}
	}
	cp := b.clone()
	dir := "ASC"
	op := ">"
	if req.Desc {
		dir = "DESC"
		op = "<"
	}
	cp.orderBy = []order{{col: col, dir: dir}}
	if req.Cursor != "" {
		val, err := DecodeCursor(req.Cursor)
		if err != nil {
			return nil, fmt.Errorf("vorm/query: invalid cursor: %w", err)
		}
		cp.wheres = append(cp.wheres, pred{col: col, op: op, arg: val})
	}
	cp.limit = req.PerPage + 1
	rows, err := cp.Get(ctx, db)
	if err != nil {
		return nil, err
	}
	hasMore := len(rows) > req.PerPage
	if hasMore {
		rows = rows[:req.PerPage]
	}
	var next string
	if hasMore && len(rows) > 0 {
		if val := cursorValueFromContext(ctx, rows[len(rows)-1], col); val != nil {
			next = EncodeCursor(val)
		}
	}
	return &PageResult[T]{
		Data:       rows,
		Style:      string(PageCursor),
		PerPage:    req.PerPage,
		NextCursor: next,
		HasMore:    hasMore,
	}, nil
}

// OffsetPage is sugar for offset pagination (COUNT + LIMIT/OFFSET).
func (b *Builder[T]) OffsetPage(ctx context.Context, db DB, page, perPage int) (*PageResult[T], error) {
	return b.Paginate(ctx, db, PageRequest{Style: PageOffset, Page: page, PerPage: perPage})
}

// SimplePaginate is Laravel simplePaginate: one query, no COUNT, HasMore from a peek row.
func (b *Builder[T]) SimplePaginate(ctx context.Context, db DB, page, perPage int) (*PageResult[T], error) {
	return b.Paginate(ctx, db, PageRequest{Style: PageSimple, Page: page, PerPage: perPage})
}

// CursorPage is sugar for keyset/cursor pagination.
func (b *Builder[T]) CursorPage(ctx context.Context, db DB, cursor string, perPage int, orderBy string, desc bool) (*PageResult[T], error) {
	return b.Paginate(ctx, db, PageRequest{
		Style: PageCursor, Cursor: cursor, PerPage: perPage, OrderBy: orderBy, Desc: desc,
	})
}

// CursorPaginate is Laravel cursorPaginate: keyset on the existing ORDER BY (or the PK).
func (b *Builder[T]) CursorPaginate(ctx context.Context, db DB, cursor string, perPage int) (*PageResult[T], error) {
	return b.Paginate(ctx, db, PageRequest{Style: PageCursor, Cursor: cursor, PerPage: perPage})
}

// EncodeCursor packs a comparable keyset value into an opaque URL-safe token.
func EncodeCursor(v any) string {
	raw, _ := json.Marshal(cursorPayload{V: v, C: "vorm1"})
	return base64.RawURLEncoding.EncodeToString(raw)
}

// DecodeCursor unpacks a token produced by EncodeCursor.
func DecodeCursor(s string) (any, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	var p cursorPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	switch n := p.V.(type) {
	case float64:
		if n == float64(int64(n)) {
			return int64(n), nil
		}
		return n, nil
	case string:
		if i, err := strconv.ParseInt(n, 10, 64); err == nil {
			return i, nil
		}
		return n, nil
	default:
		return p.V, nil
	}
}

type cursorValKey struct{}

// CursorValueFunc extracts the cursor column value from a scanned row.
type CursorValueFunc[T any] func(row T, column string) any

// WithCursorValue attaches a cursor extractor for CursorPage / Paginate(cursor).
func WithCursorValue[T any](ctx context.Context, fn CursorValueFunc[T]) context.Context {
	return context.WithValue(ctx, cursorValKey{}, fn)
}

// CursorValue reads column from row via the db tag (or a WithCursorValue hook).
func CursorValue[T any](row T, col string) any {
	return cursorValueFromRow(row, col)
}

func cursorValueFromContext[T any](ctx context.Context, row T, col string) any {
	if v := ctx.Value(cursorValKey{}); v != nil {
		if fn, ok := v.(CursorValueFunc[T]); ok {
			return fn(row, col)
		}
	}
	return cursorValueFromRow(row, col)
}

func cursorValueFromRow[T any](row T, col string) any {
	rv := reflect.ValueOf(row)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() || rv.Kind() != reflect.Struct {
		return nil
	}
	path, ok := structFields(rv.Type())[strings.ToLower(col)]
	if !ok {
		return nil
	}
	f := rv.FieldByIndex(path)
	if !f.IsValid() {
		return nil
	}
	if f.Kind() == reflect.Pointer {
		if f.IsNil() {
			return nil
		}
		return f.Elem().Interface()
	}
	return f.Interface()
}
