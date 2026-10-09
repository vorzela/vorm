package query

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// IndexInfo mirrors a database index. Generated model code carries it so tools
// and tests can reason about which filters are backed by an index.
type IndexInfo struct {
	Name      string
	Columns   []string
	Unique    bool
	Primary   bool
	Method    string
	Partial   bool
	Predicate string
}

// Meta describes a model's table and columns for codegen + runtime SQL.
// Always list Columns explicitly — vorm never emits SELECT *.
type Meta struct {
	Table       string
	Columns     []string // required for reads
	PrimaryKey  string   // default "id"
	SoftDeletes bool
	Indexes     []IndexInfo
	Generated   []string // columns the database computes; excluded from writes

	// columnTypes is filled by Model[T] from struct db tags for type checks.
	columnTypes columnTypeMap
}

// Indexed reports whether col is the leading column of any index, i.e. whether
// filtering on it can use an index.
func (m Meta) Indexed(col string) bool {
	bare := bareColumn(col)
	for _, idx := range m.Indexes {
		if len(idx.Columns) > 0 && idx.Columns[0] == bare {
			return true
		}
	}
	return false
}

// UniqueOn reports whether cols are covered by a unique/primary constraint
// (exact column set, order-independent). Partial indexes do not qualify.
// A lone PrimaryKey also counts when Indexes omit an explicit PK entry.
func (m Meta) UniqueOn(cols ...string) bool {
	if len(cols) == 0 {
		return false
	}
	if len(cols) == 1 && m.PrimaryKey != "" && strings.EqualFold(bareColumn(cols[0]), m.PrimaryKey) {
		return true
	}
	want := make(map[string]struct{}, len(cols))
	for _, c := range cols {
		want[strings.ToLower(bareColumn(c))] = struct{}{}
	}
	same := func(have []string) bool {
		if len(have) != len(want) {
			return false
		}
		for _, c := range have {
			if _, ok := want[strings.ToLower(c)]; !ok {
				return false
			}
		}
		return true
	}
	for _, idx := range m.Indexes {
		if !idx.Unique || idx.Partial || !same(idx.Columns) {
			continue
		}
		return true
	}
	return false
}

// RequireUniqueLookup errors unless cols match a unique index (or the primary key).
// FirstOrCreate / UpdateOrCreate require this so concurrent inserts cannot duplicate.
func (m Meta) RequireUniqueLookup(cols []string) error {
	if m.UniqueOn(cols...) {
		return nil
	}
	return fmt.Errorf("FirstOrCreate/UpdateOrCreate on %q requires a unique index exactly on %v", m.Table, cols)
}

// IsGenerated reports whether the database computes col (never written by vorm).
func (m Meta) IsGenerated(col string) bool {
	bare := bareColumn(col)
	for _, g := range m.Generated {
		if g == bare {
			return true
		}
	}
	return false
}

// Entity is a registered model handle: Users.Where(...).Get(...)
type Entity[T any] struct {
	meta Meta
}

var (
	registryMu sync.RWMutex
	byType     = map[reflect.Type]Meta{}
)

// Model registers table metadata and returns a typed entity handle.
//
//	var Users = query.Model[User](query.Meta{
//	    Table: "users",
//	    Columns: []string{"id", "email", "name", "active", "age", "created_at", "updated_at", "deleted_at"},
//	    SoftDeletes: true,
//	})
func Model[T any](meta Meta) *Entity[T] {
	if meta.PrimaryKey == "" {
		meta.PrimaryKey = "id"
	}
	if meta.Table == "" {
		panic("vorm/query: Meta.Table is required")
	}
	if len(meta.Columns) == 0 {
		panic("vorm/query: Meta.Columns must be non-empty (explicit columns; no SELECT *)")
	}
	meta.columnTypes = buildColumnTypes[T]()
	registryMu.Lock()
	byType[reflect.TypeFor[T]()] = meta
	registryMu.Unlock()
	return &Entity[T]{meta: meta}
}

// Meta returns a copy of the entity metadata.
func (e *Entity[T]) Meta() Meta { return e.meta }

// New starts a fluent query.
func (e *Entity[T]) New() *Builder[T] {
	return newBuilder[T](e.meta)
}

// Where starts a fluent query with a predicate.
func (e *Entity[T]) Where(args ...any) *Builder[T] {
	return e.New().Where(args...)
}

// OrderBy starts with ordering.
func (e *Entity[T]) OrderBy(col string, dir ...string) *Builder[T] {
	return e.New().OrderBy(col, dir...)
}

// Limit starts with a limit.
func (e *Entity[T]) Limit(n int) *Builder[T] {
	return e.New().Limit(n)
}

// WhereSearch starts a search query.
func (e *Entity[T]) WhereSearch(columns []string, term string) *Builder[T] {
	return e.New().WhereSearch(columns, term)
}

// WhereIn starts with WHERE col IN (...).
func (e *Entity[T]) WhereIn(col string, vals ...any) *Builder[T] {
	return e.New().WhereIn(col, vals...)
}

// WhereNotIn starts with WHERE col NOT IN (...).
func (e *Entity[T]) WhereNotIn(col string, vals ...any) *Builder[T] {
	return e.New().WhereNotIn(col, vals...)
}

// WhereNull starts with WHERE col IS NULL.
func (e *Entity[T]) WhereNull(col string) *Builder[T] {
	return e.New().WhereNull(col)
}

// WhereNotNull starts with WHERE col IS NOT NULL.
func (e *Entity[T]) WhereNotNull(col string) *Builder[T] {
	return e.New().WhereNotNull(col)
}

// Select starts with a narrowed projection.
func (e *Entity[T]) Select(cols ...string) *Builder[T] {
	return e.New().Select(cols...)
}

// OrderByDesc starts with descending ordering.
func (e *Entity[T]) OrderByDesc(col string) *Builder[T] {
	return e.New().OrderByDesc(col)
}

// LeftJoin starts with a LEFT JOIN.
func (e *Entity[T]) LeftJoin(table, on string) *Builder[T] {
	return e.New().LeftJoin(table, on)
}

// WithTrashed starts a query that includes soft-deleted rows.
func (e *Entity[T]) WithTrashed() *Builder[T] {
	return e.New().WithTrashed()
}

// OnlyTrashed starts a query of only soft-deleted rows.
func (e *Entity[T]) OnlyTrashed() *Builder[T] {
	return e.New().OnlyTrashed()
}

// Distinct starts a DISTINCT query.
func (e *Entity[T]) Distinct() *Builder[T] {
	return e.New().Distinct()
}

// Join starts with an INNER JOIN (raw SQL). Prefer Filter/Has for relations.
func (e *Entity[T]) Join(table, on string) *Builder[T] {
	return e.New().Join(table, on)
}

// With starts a query with eager-loaded relations.
func (e *Entity[T]) With(relations ...string) *Builder[T] {
	return e.New().With(relations...)
}

// WhereHas keeps rows that have at least one matching related row.
func (e *Entity[T]) WhereHas(name string) *Builder[T] {
	return e.New().WhereHas(name)
}

// Has is WhereHas.
func (e *Entity[T]) Has(name string) *Builder[T] {
	return e.New().Has(name)
}

// WhereDoesntHave is the inverse of WhereHas.
func (e *Entity[T]) WhereDoesntHave(name string) *Builder[T] {
	return e.New().WhereDoesntHave(name)
}

// Missing is WhereDoesntHave.
func (e *Entity[T]) Missing(name string) *Builder[T] {
	return e.New().Missing(name)
}

// WhereRelation is WhereHas plus a predicate on the related table.
func (e *Entity[T]) WhereRelation(name, col string, args ...any) *Builder[T] {
	return e.New().WhereRelation(name, col, args...)
}

// Filter is WhereRelation — filter parents by a related column.
func (e *Entity[T]) Filter(name, col string, args ...any) *Builder[T] {
	return e.New().Filter(name, col, args...)
}

// WithCount adds a `{name}_count` subquery column.
func (e *Entity[T]) WithCount(names ...string) *Builder[T] {
	return e.New().WithCount(names...)
}

// WithExists adds a `{name}_exists` subquery column.
func (e *Entity[T]) WithExists(names ...string) *Builder[T] {
	return e.New().WithExists(names...)
}

// WhereRaw adds a fragment whose ? markers are bound arguments.
func (e *Entity[T]) WhereRaw(fragment string, args ...any) *Builder[T] {
	return e.New().WhereRaw(fragment, args...)
}

// HavingRaw adds a HAVING fragment whose ? markers are bound arguments.
func (e *Entity[T]) HavingRaw(fragment string, args ...any) *Builder[T] {
	return e.New().HavingRaw(fragment, args...)
}

// WhereFullText filters a tsvector / FULLTEXT column.
func (e *Entity[T]) WhereFullText(col, q string) *Builder[T] {
	return e.New().WhereFullText(col, q)
}

// WhereJsonContains is Postgres `@>` / MySQL JSON_CONTAINS.
func (e *Entity[T]) WhereJsonContains(col string, value any) *Builder[T] {
	return e.New().WhereJsonContains(col, value)
}

// LookupMeta returns registered meta for T, if any.
func LookupMeta[T any]() (Meta, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	m, ok := byType[reflect.TypeFor[T]()]
	return m, ok
}

// From starts a builder with ad-hoc meta (prefer Model[T] for apps).
// Soft deletes are off by default; set SoftDeletes on a Model[T] Meta when needed.
func From[T any](table string, columns ...string) *Builder[T] {
	if len(columns) == 0 {
		panic("vorm/query: From requires explicit columns (no SELECT *)")
	}
	return newBuilder[T](Meta{Table: table, Columns: columns, PrimaryKey: "id", SoftDeletes: false})
}
