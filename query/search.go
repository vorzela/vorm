package query

import (
	"fmt"
	"strings"
)

// WhereSearch adds a case-insensitive OR search across columns (ILIKE on Postgres, LIKE on MySQL).
// Empty term is a no-op.
//
//	Users.WhereSearch([]string{"name", "email"}, "ada").Limit(20)
func (b *Builder[T]) WhereSearch(columns []string, term string) *Builder[T] {
	term = strings.TrimSpace(term)
	if term == "" || len(columns) == 0 {
		return b
	}
	for _, c := range columns {
		if err := b.meta.RequireColumn(c); err != nil {
			b.wheres = append(b.wheres, pred{col: "__error__", op: "=", arg: err.Error()})
			return b
		}
	}
	op := "ILIKE"
	if b.dialect == DialectMySQL {
		op = "LIKE"
	}
	pattern := "%" + escapeLike(term) + "%"
	// Encode as a special pred consumed by compileWhere
	b.wheres = append(b.wheres, pred{
		col: "__search__",
		op:  op,
		arg: searchArg{Columns: columns, Pattern: pattern},
	})
	return b
}

// WhereRaw adds a raw SQL fragment with bound args only.
// SECURITY: fragment must not embed user input — pass values via args.
// Rejects semicolons and comment markers.
func (b *Builder[T]) WhereRaw(fragment string, args ...any) *Builder[T] {
	if strings.ContainsAny(fragment, ";") || strings.Contains(fragment, "--") || strings.Contains(fragment, "/*") {
		b.wheres = append(b.wheres, pred{col: "__error__", op: "=", arg: "vorm/query: WhereRaw rejects ; or SQL comments (injection risk)"})
		return b
	}
	b.wheres = append(b.wheres, pred{
		col: "__raw__",
		op:  fragment,
		arg: args,
	})
	return b
}

type searchArg struct {
	Columns []string
	Pattern string
}

func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// compileWhere is extended in exec.go — patch search/raw there via helpers used from compileWhere.
func appendSearchOrRaw(dialect Dialect, p pred, parts *[]string, args *[]any, placeholder func() string) error {
	switch p.col {
	case "__search__":
		sa, ok := p.arg.(searchArg)
		if !ok {
			return fmt.Errorf("invalid search arg")
		}
		ors := make([]string, len(sa.Columns))
		for i, c := range sa.Columns {
			qc, err := QuoteIdent(dialect, c)
			if err != nil {
				return err
			}
			// Values bound via placeholder — never concat the search term into SQL.
			ors[i] = fmt.Sprintf("%s %s %s", qc, p.op, placeholder())
			*args = append(*args, sa.Pattern)
		}
		*parts = append(*parts, "("+strings.Join(ors, " OR ")+")")
		return nil
	case "__raw__":
		rawArgs, _ := p.arg.([]any)
		clause, err := expandRawBinds(p.op, rawArgs, placeholder, args)
		if err != nil {
			return err
		}
		*parts = append(*parts, clause)
		return nil
	case "__fts__":
		qc, err := QuoteIdent(dialect, p.op)
		if err != nil {
			return err
		}
		if dialect == DialectMySQL {
			*parts = append(*parts, fmt.Sprintf("MATCH (%s) AGAINST (%s)", qc, placeholder()))
		} else {
			*parts = append(*parts, fmt.Sprintf("%s @@ to_tsquery('english', %s)", qc, placeholder()))
		}
		*args = append(*args, p.arg)
		return nil
	case "__json__":
		qc, err := QuoteIdent(dialect, p.op)
		if err != nil {
			return err
		}
		if dialect == DialectMySQL {
			*parts = append(*parts, fmt.Sprintf("JSON_CONTAINS(%s, %s)", qc, placeholder()))
		} else {
			*parts = append(*parts, fmt.Sprintf("%s @> %s::jsonb", qc, placeholder()))
		}
		*args = append(*args, p.arg)
		return nil
	default:
		return fmt.Errorf("not special")
	}
}

// WhereFullText filters a tsvector column (Postgres @@ to_tsquery) or a
// FULLTEXT-indexed column (MySQL MATCH AGAINST). The query string is bound.
func (b *Builder[T]) WhereFullText(col, q string) *Builder[T] {
	if err := b.meta.RequireColumn(col); err != nil {
		b.wheres = append(b.wheres, pred{col: "__error__", op: "=", arg: err.Error()})
		return b
	}
	if b.dialect == DialectMySQL && !metaHasFullText(b.meta, col) {
		b.wheres = append(b.wheres, pred{col: "__error__", op: "=", arg: "vorm/query: WhereFullText on MySQL needs a FULLTEXT index"})
		return b
	}
	b.wheres = append(b.wheres, pred{col: "__fts__", op: col, arg: q})
	return b
}

// WhereJsonContains is Postgres `col @> $1::jsonb` / MySQL JSON_CONTAINS.
func (b *Builder[T]) WhereJsonContains(col string, value any) *Builder[T] {
	if err := b.meta.RequireColumn(col); err != nil {
		b.wheres = append(b.wheres, pred{col: "__error__", op: "=", arg: err.Error()})
		return b
	}
	b.wheres = append(b.wheres, pred{col: "__json__", op: col, arg: value})
	return b
}

// RawPiece is one slice of a WhereRaw fragment. Bind is a ? placeholder.
type RawPiece struct {
	Text string
	Bind bool
}

// SplitRawBinds splits fragment on ? bind markers. ? inside a single-quoted
// SQL string is left alone. An unclosed quote is an error.
func SplitRawBinds(fragment string) ([]RawPiece, error) {
	var out []RawPiece
	var b strings.Builder
	inStr := false
	for i := 0; i < len(fragment); i++ {
		c := fragment[i]
		if inStr {
			b.WriteByte(c)
			if c == '\'' {
				if i+1 < len(fragment) && fragment[i+1] == '\'' {
					b.WriteByte(fragment[i+1])
					i++
					continue
				}
				inStr = false
			}
			continue
		}
		if c == '\'' {
			inStr = true
			b.WriteByte(c)
			continue
		}
		if c == '?' {
			if b.Len() > 0 {
				out = append(out, RawPiece{Text: b.String()})
				b.Reset()
			}
			out = append(out, RawPiece{Bind: true})
			continue
		}
		b.WriteByte(c)
	}
	if inStr {
		return nil, fmt.Errorf("unclosed string literal")
	}
	if b.Len() > 0 {
		out = append(out, RawPiece{Text: b.String()})
	}
	return out, nil
}

func expandRawBinds(fragment string, rawArgs []any, placeholder func() string, args *[]any) (string, error) {
	pieces, err := SplitRawBinds(fragment)
	if err != nil {
		return "", fmt.Errorf("vorm/query: WhereRaw: %w", err)
	}
	n := 0
	for _, piece := range pieces {
		if piece.Bind {
			n++
		}
	}
	if n != len(rawArgs) {
		return "", fmt.Errorf("vorm/query: WhereRaw has %d ? placeholders and %d arguments", n, len(rawArgs))
	}
	var sb strings.Builder
	sb.WriteByte('(')
	ai := 0
	for _, piece := range pieces {
		if piece.Bind {
			sb.WriteString(placeholder())
			*args = append(*args, rawArgs[ai])
			ai++
			continue
		}
		sb.WriteString(piece.Text)
	}
	sb.WriteByte(')')
	return sb.String(), nil
}

func metaHasFullText(m Meta, col string) bool {
	for _, idx := range m.Indexes {
		method := strings.ToLower(idx.Method)
		name := strings.ToLower(idx.Name)
		if method != "fulltext" && !strings.Contains(name, "fulltext") && !strings.Contains(name, "ft_") {
			continue
		}
		for _, c := range idx.Columns {
			if strings.EqualFold(c, col) {
				return true
			}
		}
	}
	return false
}
