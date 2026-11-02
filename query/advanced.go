package query

import (
	"fmt"
	"strings"
)

type joinClause struct {
	typ   string // INNER JOIN | LEFT JOIN | RIGHT JOIN
	table string
	on    string
}

// Distinct adds SELECT DISTINCT (never SELECT *).
func (b *Builder[T]) Distinct() *Builder[T] {
	b.distinct = true
	b.distinctOn = nil
	return b
}

// DistinctOn is PostgreSQL DISTINCT ON (cols...) — ignored on MySQL (falls back to Distinct).
func (b *Builder[T]) DistinctOn(cols ...string) *Builder[T] {
	for _, c := range cols {
		if err := b.meta.RequireColumn(c); err != nil {
			b.wheres = append(b.wheres, pred{col: "__error__", op: "=", arg: err.Error()})
			return b
		}
	}
	b.distinct = true
	b.distinctOn = append([]string(nil), cols...)
	return b
}

// OrderByDesc is OrderBy(col, "DESC").
func (b *Builder[T]) OrderByDesc(col any) *Builder[T] {
	return b.OrderBy(col, "DESC")
}

// Join adds INNER JOIN table ON on (raw SQL escape hatch).
// Prefer Filter / Has / Missing for relation filters — those use EXISTS and
// never duplicate parent rows (no Distinct needed).
//
//	Users.Join("posts", "posts.user_id = users.id").Select("users.id", "users.email")
func (b *Builder[T]) Join(table, on string) *Builder[T] {
	b.joins = append(b.joins, joinClause{typ: "INNER JOIN", table: table, on: on})
	return b
}

// LeftJoin adds LEFT JOIN.
func (b *Builder[T]) LeftJoin(table, on string) *Builder[T] {
	b.joins = append(b.joins, joinClause{typ: "LEFT JOIN", table: table, on: on})
	return b
}

// RightJoin adds RIGHT JOIN.
func (b *Builder[T]) RightJoin(table, on string) *Builder[T] {
	b.joins = append(b.joins, joinClause{typ: "RIGHT JOIN", table: table, on: on})
	return b
}

// GroupByCols is GroupBy using generated typed column keys.
func (b *Builder[T]) GroupByCols(cols ...Col) *Builder[T] {
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = string(c)
	}
	return b.GroupBy(names...)
}

// GroupBy adds GROUP BY cols (use with Select aggregates / Distinct).
func (b *Builder[T]) GroupBy(cols ...string) *Builder[T] {
	for _, c := range cols {
		if err := b.meta.RequireColumn(c); err != nil {
			b.wheres = append(b.wheres, pred{col: "__error__", op: "=", arg: err.Error()})
			return b
		}
	}
	b.groupBy = append(b.groupBy, cols...)
	return b
}

// Having adds a HAVING predicate. Accepts the same forms as Where, plus safe
// aggregate expressions (COUNT(*), SUM(amount), …) which are not in Meta.Columns.
func (b *Builder[T]) Having(args ...any) *Builder[T] {
	p, err := parseWhere(args...)
	if err != nil {
		b.havings = append(b.havings, pred{col: "__error__", op: "=", arg: err.Error()})
		return b
	}
	if err := b.validateHavingPred(p); err != nil {
		b.havings = append(b.havings, pred{col: "__error__", op: "=", arg: err.Error()})
		return b
	}
	b.havings = append(b.havings, p)
	return b
}

// HavingRaw adds a HAVING fragment with bound args only (mirrors WhereRaw).
// SECURITY: fragment must not embed user input — pass values via args.
// Rejects semicolons and comment markers.
func (b *Builder[T]) HavingRaw(fragment string, args ...any) *Builder[T] {
	if strings.ContainsAny(fragment, ";") || strings.Contains(fragment, "--") || strings.Contains(fragment, "/*") {
		b.havings = append(b.havings, pred{col: "__error__", op: "=", arg: "vorm/query: HavingRaw rejects ; or SQL comments (injection risk)"})
		return b
	}
	b.havings = append(b.havings, pred{
		col: "__raw__",
		op:  fragment,
		arg: args,
	})
	return b
}

// WhereGroup adds a parenthesized predicate group. Use it when mixing AND and
// OR so SQL precedence is explicit:
//
//	Users.Where("active", true).
//		WhereGroup(func(q *Builder[User]) {
//			q.Where("role", "admin").OrWhere("role", "owner")
//		})
//	// (active = $1) AND (role = $2 OR role = $3)
func (b *Builder[T]) WhereGroup(fn func(*Builder[T])) *Builder[T] {
	sub := newBuilder[T](b.meta)
	sub.dialect = b.dialect
	if fn != nil {
		fn(sub)
	}
	if len(sub.wheres) == 0 {
		return b
	}
	b.wheres = append(b.wheres, pred{col: "__group__", group: sub.wheres})
	return b
}

// OrWhereGroup is WhereGroup joined with OR.
func (b *Builder[T]) OrWhereGroup(fn func(*Builder[T])) *Builder[T] {
	b.WhereGroup(fn)
	if len(b.wheres) > 0 {
		b.wheres[len(b.wheres)-1].or = true
	}
	return b
}

// WhereBetween is col BETWEEN low AND high.
func (b *Builder[T]) WhereBetween(col string, low, high any) *Builder[T] {
	return b.Where(col, Between(low, high))
}

// WhereNotBetween is col NOT BETWEEN low AND high.
func (b *Builder[T]) WhereNotBetween(col string, low, high any) *Builder[T] {
	return b.Where(col, NotBetween(low, high))
}

// WhereLike is col LIKE pattern. Use query.LikePattern for contains search.
func (b *Builder[T]) WhereLike(col string, pattern string) *Builder[T] {
	return b.Where(col, "LIKE", pattern)
}

// WhereNotLike is col NOT LIKE pattern.
func (b *Builder[T]) WhereNotLike(col string, pattern string) *Builder[T] {
	return b.Where(col, "NOT LIKE", pattern)
}

// OrWhere adds a predicate joined with OR to the previous WHERE group.
func (b *Builder[T]) OrWhere(args ...any) *Builder[T] {
	p, err := parseWhere(args...)
	if err != nil {
		b.wheres = append(b.wheres, pred{col: "__error__", op: "=", arg: err.Error(), or: true})
		return b
	}
	if err := b.validatePred(p); err != nil {
		b.wheres = append(b.wheres, pred{col: "__error__", op: "=", arg: err.Error(), or: true})
		return b
	}
	p.or = true
	b.wheres = append(b.wheres, p)
	return b
}

// WhereIn is Where(col, In(vals...)). An empty set matches no rows.
func (b *Builder[T]) WhereIn(col string, vals ...any) *Builder[T] {
	return b.Where(col, In(vals...))
}

// WhereNotIn is Where(col, NotIn(vals...)). An empty set excludes no rows.
func (b *Builder[T]) WhereNotIn(col string, vals ...any) *Builder[T] {
	return b.Where(col, NotIn(vals...))
}

// WhereNull is Where(col, IsNull()).
func (b *Builder[T]) WhereNull(col string) *Builder[T] {
	return b.Where(col, IsNull())
}

// WhereNotNull is Where(col, IsNotNull()).
func (b *Builder[T]) WhereNotNull(col string) *Builder[T] {
	return b.Where(col, IsNotNull())
}

// WhereExists adds AND EXISTS (subquery). Prefer parameterized subquerySQL with args.
// SECURITY: subquerySQL must not embed user input; bind via args. Rejects ; and comments.
func (b *Builder[T]) WhereExists(subquerySQL string, args ...any) *Builder[T] {
	if err := safeSubquery(subquerySQL); err != nil {
		b.wheres = append(b.wheres, pred{col: "__error__", op: "=", arg: err.Error()})
		return b
	}
	b.wheres = append(b.wheres, pred{col: "__exists__", op: subquerySQL, arg: args})
	return b
}

// WhereNotExists adds AND NOT EXISTS (subquery).
func (b *Builder[T]) WhereNotExists(subquerySQL string, args ...any) *Builder[T] {
	if err := safeSubquery(subquerySQL); err != nil {
		b.wheres = append(b.wheres, pred{col: "__error__", op: "=", arg: err.Error()})
		return b
	}
	b.wheres = append(b.wheres, pred{col: "__not_exists__", op: subquerySQL, arg: args})
	return b
}

func safeSubquery(sql string) error {
	if strings.ContainsAny(sql, ";") || strings.Contains(sql, "--") || strings.Contains(sql, "/*") {
		return fmt.Errorf("vorm/query: subquery rejects ; or SQL comments (injection risk)")
	}
	return nil
}

// LockForUpdate appends FOR UPDATE (row locks — use inside Transaction).
func (b *Builder[T]) LockForUpdate() *Builder[T] {
	b.lock = "FOR UPDATE"
	return b
}

// LockForShare appends FOR SHARE (Postgres) / LOCK IN SHARE MODE (MySQL).
func (b *Builder[T]) LockForShare() *Builder[T] {
	b.lock = "FOR SHARE"
	return b
}

// SkipLocked appends FOR UPDATE SKIP LOCKED (queue workers; Postgres/MySQL 8+).
func (b *Builder[T]) SkipLocked() *Builder[T] {
	b.lock = "FOR UPDATE SKIP LOCKED"
	return b
}

// ForUpdate is an alias of LockForUpdate.
func (b *Builder[T]) ForUpdate() *Builder[T] { return b.LockForUpdate() }
