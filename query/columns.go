package query

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

// columnTypeMap is built from model struct fields (db tags) at Model[T] registration.
type columnTypeMap map[string]reflect.Type

func buildColumnTypes[T any]() columnTypeMap {
	out := columnTypeMap{}
	var zero T
	t := reflect.TypeOf(zero)
	if t == nil {
		return out
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return out
	}
	collectFields(t, out)
	return out
}

func collectFields(t reflect.Type, out columnTypeMap) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			collectFields(f.Type, out)
			continue
		}
		col := f.Tag.Get("db")
		if col == "" || col == "-" {
			col = strings.ToLower(f.Name)
			// vorm.Model uses CreatedAt → created_at via common snake; prefer explicit db tags.
			if f.Name == "ID" {
				col = "id"
			} else if f.Name == "CreatedAt" {
				col = "created_at"
			} else if f.Name == "UpdatedAt" {
				col = "updated_at"
			} else if f.Name == "DeletedAt" {
				col = "deleted_at"
			}
		}
		out[col] = f.Type
	}
}

func bareColumn(col string) string {
	if i := strings.LastIndex(col, "."); i >= 0 {
		return col[i+1:]
	}
	return col
}

// SoftDeleteColumn is the conventional soft-delete timestamp column.
const SoftDeleteColumn = "deleted_at"

// ProjectColumns returns the SELECT list for a soft-delete-aware query.
// When the default soft scope filters deleted_at IS NULL, that column is
// omitted from the projection — returned rows would always have NULL there.
// WithTrashed and OnlyTrashed keep deleted_at so callers can see it.
func ProjectColumns(cols []string, softDeletes, withTrashed, onlyTrashed bool) []string {
	if len(cols) == 0 || !softDeletes || withTrashed || onlyTrashed {
		return cols
	}
	return omitNamedColumn(cols, SoftDeleteColumn)
}

func omitNamedColumn(cols []string, name string) []string {
	n := 0
	for _, c := range cols {
		if bareColumn(c) != name {
			n++
		}
	}
	if n == len(cols) {
		return cols
	}
	out := make([]string, 0, n)
	for _, c := range cols {
		if bareColumn(c) == name {
			continue
		}
		out = append(out, c)
	}
	return out
}

// HasColumn reports whether col is in Meta.Columns (table.col or bare).
func (m Meta) HasColumn(col string) bool {
	bare := bareColumn(col)
	for _, c := range m.Columns {
		if c == col || c == bare || bareColumn(c) == bare {
			return true
		}
	}
	return false
}

// RequireColumn errors if the column is not listed on Meta (prevents typos / injection via unknown names).
// Qualified columns on other tables (joins) are allowed after SafeIdent only.
func (m Meta) RequireColumn(col string) error {
	if col == "" || strings.HasPrefix(col, "__") {
		return nil
	}
	if err := SafeIdent(col); err != nil {
		return err
	}
	if i := strings.LastIndex(col, "."); i >= 0 {
		table, bare := col[:i], col[i+1:]
		if table != m.Table {
			return nil // join / other relation
		}
		col = bare
	}
	if !m.HasColumn(col) {
		return fmt.Errorf("vorm/query: unknown column %q on table %q (known: %s)",
			col, m.Table, strings.Join(m.Columns, ", "))
	}
	return nil
}

// ValidateGroupBy checks that every projected column is covered by GROUP BY.
// A grouped primary key also covers the rest of that table's columns, matching
// the functional dependency PostgreSQL and MySQL recognise. Joined-table
// columns are only covered when that same column is grouped explicitly.
func (m Meta) ValidateGroupBy(groupBy, selects []string) error {
	if len(groupBy) == 0 || len(selects) == 0 {
		return nil
	}
	pk := m.PrimaryKey
	if pk == "" {
		pk = "id"
	}

	grouped := make(map[string]struct{}, len(groupBy))
	pkGrouped := false
	for _, g := range groupBy {
		table, name := splitColumnRef(g)
		if m.refersToBase(table) {
			table = ""
			if strings.EqualFold(name, pk) {
				pkGrouped = true
			}
		}
		grouped[columnRefKey(table, name)] = struct{}{}
	}

	for _, sel := range selects {
		if IsSafeExpr(sel) {
			continue // aggregate expressions do not need to be grouped
		}
		table, name := splitColumnRef(sel)
		if m.refersToBase(table) {
			if _, ok := grouped[columnRefKey("", name)]; ok {
				continue
			}
			if pkGrouped {
				continue
			}
		} else if _, ok := grouped[columnRefKey(table, name)]; ok {
			continue
		}
		return fmt.Errorf("vorm/query: GROUP BY %s does not cover projected column %q on table %q; add it to GroupBy or narrow the projection with Select(...)",
			strings.Join(groupBy, ", "), sel, m.Table)
	}
	return nil
}

// refersToBase reports whether a qualifier belongs to this Meta's table.
// An unqualified column always refers to the base table in a vorm query.
func (m Meta) refersToBase(table string) bool {
	if table == "" || strings.EqualFold(table, m.Table) {
		return true
	}
	if i := strings.LastIndex(m.Table, "."); i >= 0 {
		return strings.EqualFold(table, m.Table[i+1:])
	}
	return false
}

func splitColumnRef(col string) (table, name string) {
	if i := strings.LastIndex(col, "."); i >= 0 {
		return col[:i], col[i+1:]
	}
	return "", col
}

func columnRefKey(table, name string) string {
	return strings.ToLower(table) + "\x00" + strings.ToLower(name)
}

// CheckColumnValue ensures col exists and value is assignable to the model field type (when known).
func (m Meta) CheckColumnValue(col string, value any) error {
	if err := m.RequireColumn(col); err != nil {
		return err
	}
	if value == nil || m.columnTypes == nil {
		return nil
	}
	want, ok := m.columnTypes[bareColumn(col)]
	if !ok {
		return nil
	}
	return checkAssignable(want, value, bareColumn(col))
}

func checkAssignable(want reflect.Type, value any, col string) error {
	if op, ok := value.(Operator); ok {
		if op.Value == nil && (op.Op == "IS NULL" || op.Op == "IS NOT NULL") {
			return nil
		}
		if op.Op == "IN" || op.Op == "NOT IN" {
			return checkInValues(want, op.Value, col)
		}
		return checkAssignable(want, op.Value, col)
	}
	got := reflect.TypeOf(value)
	if got == nil {
		return nil
	}
	if assignableTo(want, got) {
		return nil
	}
	return fmt.Errorf("vorm/query: column %q expects %s, got %s", col, typeName(want), typeName(got))
}

func checkInValues(want reflect.Type, v any, col string) error {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return nil
	}
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return checkAssignable(want, v, col)
	}
	for i := 0; i < rv.Len(); i++ {
		if err := checkAssignable(want, rv.Index(i).Interface(), col); err != nil {
			return err
		}
	}
	return nil
}

func assignableTo(want, got reflect.Type) bool {
	if want == nil || got == nil {
		return true
	}
	if got.AssignableTo(want) || got.ConvertibleTo(want) {
		return true
	}
	// unwrap pointers
	w, g := want, got
	for w.Kind() == reflect.Pointer {
		w = w.Elem()
	}
	for g.Kind() == reflect.Pointer {
		g = g.Elem()
	}
	if g.AssignableTo(w) || g.ConvertibleTo(w) {
		return true
	}
	// common numeric widenings: int literals vs int64 fields
	if isIntLike(w) && isIntLike(g) {
		return true
	}
	if isFloatLike(w) && isFloatLike(g) {
		return true
	}
	if w == reflect.TypeOf(time.Time{}) && g == reflect.TypeOf(time.Time{}) {
		return true
	}
	// string-backed enums
	if w.Kind() == reflect.String && g.Kind() == reflect.String {
		return true
	}
	return false
}

func isIntLike(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	default:
		return false
	}
}

func isFloatLike(t reflect.Type) bool {
	k := t.Kind()
	return k == reflect.Float32 || k == reflect.Float64
}

func typeName(t reflect.Type) string {
	if t == nil {
		return "nil"
	}
	return t.String()
}
