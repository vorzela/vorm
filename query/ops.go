package query

// Operator for Where / Find maps (TypeORM-style helpers).
type Operator struct {
	Op    string
	Value any
}

func Eq(v any) Operator                 { return Operator{Op: "=", Value: v} }
func Not(v any) Operator                { return Operator{Op: "<>", Value: v} }
func MoreThan(v any) Operator           { return Operator{Op: ">", Value: v} }
func MoreThanOrEqual(v any) Operator    { return Operator{Op: ">=", Value: v} }
func LessThan(v any) Operator           { return Operator{Op: "<", Value: v} }
func LessThanOrEqual(v any) Operator    { return Operator{Op: "<=", Value: v} }
func Like(v any) Operator               { return Operator{Op: "LIKE", Value: v} }
func ILike(v any) Operator              { return Operator{Op: "ILIKE", Value: v} }
func In(vals ...any) Operator           { return Operator{Op: "IN", Value: flattenInValues(vals)} }
func NotIn(vals ...any) Operator        { return Operator{Op: "NOT IN", Value: flattenInValues(vals)} }
func Between(low, high any) Operator    { return Operator{Op: "BETWEEN", Value: []any{low, high}} }
func NotBetween(low, high any) Operator { return Operator{Op: "NOT BETWEEN", Value: []any{low, high}} }
func IsNull() Operator                  { return Operator{Op: "IS NULL", Value: nil} }
func IsNotNull() Operator               { return Operator{Op: "IS NOT NULL", Value: nil} }

// flattenInValues accepts both In(1, 2) and In([]int{1, 2}) without forcing
// callers to remember the variadic expansion.
func flattenInValues(vals []any) []any {
	if len(vals) == 1 {
		if flat, ok := toAnySlice(vals[0]); ok {
			return flat
		}
	}
	return vals
}

// Sort direction for FindOptions.Order.
type Sort string

const (
	Asc  Sort = "ASC"
	Desc Sort = "DESC"
)

// FindOptions mirrors TypeORM-style find:
//
//	Users.Find(ctx, db, query.FindOptions{
//	    Where: query.WhereMap{"active": true, "age": query.MoreThan(18)},
//	    Order: query.OrderMap{"name": query.Asc},
//	    Take: 10,
//	})
type FindOptions struct {
	Where  WhereMap
	Order  OrderMap
	Take   int
	Skip   int
	Select []string // subset of Meta.Columns; empty = all registered columns
}

// WhereMap is column → value or Operator.
type WhereMap map[string]any

// OrderMap is column → Asc|Desc|"ASC"|"DESC".
type OrderMap map[string]any
