package generate

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/vorzela/vorm/query"
)

// writeSQLAsComment emits the sqlc-style block comment above a generated
// function: the name, a blank line, then each SQL line indented with two spaces.
//
//	// ListActiveAdults
//	//
//	//   SELECT "id", "email" FROM "users" WHERE ...
func writeSQLAsComment(b *strings.Builder, name string, sqls ...string) {
	fmt.Fprintf(b, "// %s\n", name)
	for _, sql := range sqls {
		sql = strings.TrimSpace(sql)
		if sql == "" {
			continue
		}
		b.WriteString("//\n")
		sc := bufio.NewScanner(strings.NewReader(sql))
		for sc.Scan() {
			fmt.Fprintf(b, "//\t%s\n", sc.Text())
		}
	}
}

// previewSQL returns the SQL text(s) that will run for a stub, for comments.
// Dynamic plans (IN slices) render with an IN (/* slice */) marker.
func previewSQL(st StubFunc, ms ModelSpec, d query.Dialect, hasParams bool) []string {
	bind := binder(st, hasParams)
	switch st.Action {
	case "Get", "First", "FirstOrFail":
		cp := st
		if cp.Action != "Get" && cp.Limit == 0 && cp.LimitExpr == "" {
			cp.Limit = 1
		}
		p, err := newPlanner(cp, ms, d, bind).selectPlan(resultCols(cp, ms), selectRows)
		if err != nil {
			return nil
		}
		return []string{p.commentSQL(d)}
	case "Count":
		p, err := newPlanner(st, ms, d, bind).selectPlan(nil, selectCount)
		if err != nil {
			return nil
		}
		return []string{p.commentSQL(d)}
	case "Exists":
		p, err := newPlanner(st, ms, d, bind).selectPlan(nil, selectExists)
		if err != nil {
			return nil
		}
		return []string{p.commentSQL(d)}
	case "Paginate":
		return previewPaginateSQL(st, ms, d, hasParams)
	case "Pluck":
		if len(st.Selects) == 0 {
			return nil
		}
		p, err := newPlanner(st, ms, d, bind).selectPlan(st.Selects, selectRows)
		if err != nil {
			return nil
		}
		return []string{p.commentSQL(d)}
	case "Sum", "Avg", "Min", "Max":
		if len(st.Selects) == 0 {
			return nil
		}
		p, err := newPlanner(st, ms, d, bind).aggregatePlan(st.Action, st.Selects[0])
		if err != nil {
			return nil
		}
		return []string{p.commentSQL(d)}
	case "Increment", "Decrement":
		if len(st.Selects) == 0 {
			return nil
		}
		amount := st.AmountExpr
		if amount == "" {
			amount = "int64(1)"
		}
		p, err := newPlanner(st, ms, d, bind).incrementPlan(st.Selects[0], amount, st.Action == "Decrement")
		if err != nil {
			return nil
		}
		return []string{p.commentSQL(d)}
	case "Create":
		if sql := previewCreateSQL(st, ms, d); sql != "" {
			return []string{sql}
		}
		return nil
	case "Update":
		p, err := newPlanner(st, ms, d, bind).updatePlan(st.UpdateVals, nil, true)
		if err != nil {
			return nil
		}
		return []string{p.commentSQL(d)}
	case "SoftDelete":
		p, err := newPlanner(primaryKeyStub(st, ms), ms, d, bind).updatePlan(nil,
			[]string{quoteIdent(d, "deleted_at") + " = CURRENT_TIMESTAMP"}, true)
		if err != nil {
			return nil
		}
		return []string{p.commentSQL(d)}
	case "Delete":
		if ms.SoftDeletes && !st.WithTrashed {
			p, err := newPlanner(primaryKeyStub(st, ms), ms, d, bind).updatePlan(nil,
				[]string{quoteIdent(d, "deleted_at") + " = CURRENT_TIMESTAMP"}, true)
			if err != nil {
				return nil
			}
			return []string{p.commentSQL(d)}
		}
		fallthrough
	case "ForceDelete":
		p, err := newPlanner(primaryKeyStub(st, ms), ms, d, bind).deletePlan()
		if err != nil {
			return nil
		}
		return []string{p.commentSQL(d)}
	case "Restore":
		st = primaryKeyStub(st, ms)
		st.OnlyTrashed = true
		p, err := newPlanner(st, ms, d, bind).updatePlan(nil,
			[]string{quoteIdent(d, "deleted_at") + " = NULL"}, true)
		if err != nil {
			return nil
		}
		return []string{p.commentSQL(d)}
	default:
		return nil
	}
}

func previewCreateSQL(st StubFunc, ms ModelSpec, d query.Dialect) string {
	if ms.Table == "" || len(st.CreateVals) == 0 {
		return ""
	}
	cols := make([]string, len(st.CreateVals))
	for i, kv := range st.CreateVals {
		cols[i] = kv.Col
	}
	holders := placeholders(d, len(cols))
	sql := fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s)`,
		quoteIdent(d, ms.Table), quoteList(d, cols), strings.Join(holders, ", "))
	if d != query.DialectMySQL {
		sql += " RETURNING " + quoteIdent(d, orPK(ms))
	}
	return sql
}

func previewPaginateSQL(st StubFunc, ms ModelSpec, d query.Dialect, hasParams bool) []string {
	bind := binder(st, hasParams)
	localBind := func(expr string) string { return bind(expr) }
	pageStub := st
	pageStub.LimitExpr = "perPage"
	pageStub.OffsetExpr = "(page-1)*perPage"
	switch st.PageStyle {
	case "simple":
		pageStub.OffsetExpr = ""
		pageStub.LimitExpr = "limitN"
		p, err := newPlanner(pageStub, ms, d, localBind).selectPlan(resultCols(st, ms), selectRows)
		if err != nil {
			return nil
		}
		return []string{p.commentSQL(d)}
	case "cursor":
		pageStub.OffsetExpr = ""
		pageStub.LimitExpr = "limitN"
		p, err := newPlanner(pageStub, ms, d, localBind).selectPlan(resultCols(st, ms), selectRows)
		if err != nil {
			return nil
		}
		return []string{p.commentSQL(d)}
	default:
		p, err := newPlanner(pageStub, ms, d, localBind).selectPlan(resultCols(st, ms), selectRows)
		if err != nil {
			return nil
		}
		out := []string{p.commentSQL(d)}
		countStub := st
		countStub.Limit = 0
		countStub.LimitExpr = ""
		countStub.Offset = 0
		countStub.OffsetExpr = ""
		countStub.Orders = nil
		cp, err := newPlanner(countStub, ms, d, bind).selectPlan(nil, selectCount)
		if err == nil {
			out = append(out, cp.commentSQL(d))
		}
		return out
	}
}

// commentSQL renders the statement for a Go comment. Static plans use the same
// text as the generated const; dynamic IN groups show IN (/* slice */).
func (p *plan) commentSQL(d query.Dialect) string {
	if !p.dynamic {
		return p.constSQL()
	}
	var sb strings.Builder
	n := 1
	for _, s := range p.segs {
		switch s.Kind {
		case segText:
			sb.WriteString(s.Text)
		case segPlaceholder:
			if !s.Inline {
				sb.WriteString(query.Placeholder(d, n))
			}
			n++
		case segIn:
			op := " IN (/* slice */)"
			if s.Negated {
				op = " NOT IN (/* slice */)"
			}
			sb.WriteString(s.Column)
			sb.WriteString(op)
		}
	}
	return sb.String()
}
