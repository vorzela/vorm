package generate

import (
	"fmt"
	"strings"

	"github.com/vorzela/vorm/query"
)

// A plan is SQL split into literal text and the runtime-sized IN groups that
// cannot be known at generate time. When Dynamic is false the whole statement
// collapses to one const string, which is the fast path: no per-call building.
type plan struct {
	segs    []segment
	binds   []bind
	locals  []local
	dynamic bool
}

// local is a variable computed once before the statement runs, so a value bound
// to several placeholders is not recomputed per placeholder.
type local struct {
	Name string
	Expr string
}

func (p *plan) addLocal(name, expr string) string {
	p.locals = append(p.locals, local{Name: name, Expr: expr})
	return name
}

type segKind int

const (
	segText segKind = iota
	// segPlaceholder is a bind marker whose number is only known at run time,
	// because a preceding IN group shifted every later argument.
	segPlaceholder
	// segIn is an IN group sized by len(SliceExpr) at run time.
	segIn
)

type segment struct {
	Kind segKind
	Text string

	// Inline marks a placeholder whose marker is already part of the literal
	// text; only its argument still has to be appended at run time.
	Inline bool

	SliceExpr string // Go expression evaluating to a slice
	Column    string // pre-quoted column for the IN group
	Negated   bool
}

// bind is one argument in textual order. Slice binds append every element.
type bind struct {
	Expr  string
	Slice bool
}

func (p *plan) text(s string) {
	if s == "" {
		return
	}
	if n := len(p.segs); n > 0 && p.segs[n-1].Kind == segText {
		p.segs[n-1].Text += s
		return
	}
	p.segs = append(p.segs, segment{Kind: segText, Text: s})
}

func (p *plan) arg(expr string) {
	p.binds = append(p.binds, bind{Expr: expr})
}

// inGroup appends a runtime-sized IN group and the slice that fills it.
func (p *plan) inGroup(col, sliceExpr string, negated bool) {
	p.segs = append(p.segs, segment{Kind: segIn, SliceExpr: sliceExpr, Column: col, Negated: negated})
	p.binds = append(p.binds, bind{Expr: sliceExpr, Slice: true})
	p.dynamic = true
}

// placeholder writes the marker for the next argument. Before any dynamic
// segment the number is known at generate time and goes straight into the
// literal text; afterwards it depends on runtime lengths.
func (p *plan) placeholder(d query.Dialect) {
	if !p.dynamic {
		p.text(query.Placeholder(d, p.staticArgCount()+1))
		p.segs = append(p.segs, segment{Kind: segPlaceholder, Inline: true})
		return
	}
	p.segs = append(p.segs, segment{Kind: segPlaceholder})
}

// staticArgCount counts binds already emitted; only valid while !dynamic.
func (p *plan) staticArgCount() int { return len(p.binds) }

// constSQL returns the whole statement as one string. Only valid when !dynamic.
func (p *plan) constSQL() string {
	var sb strings.Builder
	for _, s := range p.segs {
		sb.WriteString(s.Text)
	}
	return sb.String()
}

// argExprs returns the Go expressions for a static plan, in bind order.
func (p *plan) argExprs() []string {
	out := make([]string, len(p.binds))
	for i, b := range p.binds {
		out[i] = b.Expr
	}
	return out
}

// planner compiles a lowered stub into a plan, mirroring query.Builder's SQL.
type planner struct {
	st      StubFunc
	ms      ModelSpec
	dialect query.Dialect
	bindFn  func(string) string
}

func newPlanner(st StubFunc, ms ModelSpec, d query.Dialect, bindFn func(string) string) *planner {
	if bindFn == nil {
		bindFn = func(s string) string { return s }
	}
	return &planner{st: st, ms: ms, dialect: d, bindFn: bindFn}
}

func (pl *planner) quote(name string) (string, error) {
	return query.QuoteIdent(pl.dialect, name)
}

// qualify mirrors Builder.qualify: once a join is present a bare column is
// prefixed with the base table so `id` cannot become ambiguous.
func (pl *planner) qualify(col string) string {
	if len(pl.st.Joins) == 0 || col == "" {
		return col
	}
	if strings.ContainsAny(col, ".( ") {
		return col
	}
	return pl.ms.Table + "." + col
}

func (pl *planner) quoteCol(col string) (string, error) {
	if err := query.SafeIdent(col); err != nil {
		return "", err
	}
	return pl.quote(pl.qualify(col))
}

// selectMode picks the projection: full rows, COUNT(*), or an existence probe.
type selectMode int

const (
	selectRows selectMode = iota
	selectCount
	selectExists
)

// selectPlan compiles the SELECT statement for Get/First/Count/Exists/Paginate.
func (pl *planner) selectPlan(cols []string, mode selectMode) (*plan, error) {
	if pl.ms.Table == "" {
		return nil, fmt.Errorf("cannot resolve model/entity %q — run vorm generate models", pl.st.Entity)
	}
	p := &plan{}
	p.text("SELECT ")
	switch {
	case mode == selectExists:
		p.text("1")
	case mode == selectCount:
		if pl.st.Distinct && len(pl.st.Selects) == 1 {
			q, err := pl.quoteCol(pl.st.Selects[0])
			if err != nil {
				return nil, err
			}
			p.text("COUNT(DISTINCT " + q + ")")
		} else {
			p.text("COUNT(*)")
		}
	default:
		if pl.st.Distinct {
			if len(pl.st.DistinctOn) > 0 && pl.dialect != query.DialectMySQL {
				parts := make([]string, len(pl.st.DistinctOn))
				for i, c := range pl.st.DistinctOn {
					q, err := pl.quoteCol(c)
					if err != nil {
						return nil, err
					}
					parts[i] = q
				}
				p.text("DISTINCT ON (" + strings.Join(parts, ", ") + ") ")
			} else {
				p.text("DISTINCT ")
			}
		}
		if len(cols) == 0 {
			return nil, fmt.Errorf("no columns selected (refusing SELECT *)")
		}
		if err := query.RejectStarInList(cols); err != nil {
			return nil, err
		}
		quoted := make([]string, len(cols))
		for i, c := range cols {
			q, err := pl.quoteCol(c)
			if err != nil {
				return nil, err
			}
			quoted[i] = q
		}
		p.text(strings.Join(quoted, ", "))
		if err := pl.writeRelSubselects(p); err != nil {
			return nil, err
		}
	}

	tableQ, err := pl.quote(pl.ms.Table)
	if err != nil {
		return nil, err
	}
	p.text(" FROM " + tableQ)
	if err := pl.writeJoins(p); err != nil {
		return nil, err
	}
	if err := pl.writeWhere(p); err != nil {
		return nil, err
	}
	if mode == selectExists {
		p.text(" LIMIT 1")
		if pl.st.Lock != "" {
			p.text(" " + pl.lockSQL())
		}
		return p, nil
	}
	if mode == selectCount {
		return p, nil
	}

	if len(pl.st.GroupBy) > 0 {
		parts := make([]string, len(pl.st.GroupBy))
		for i, g := range pl.st.GroupBy {
			q, err := pl.quoteCol(g)
			if err != nil {
				return nil, err
			}
			parts[i] = q
		}
		p.text(" GROUP BY " + strings.Join(parts, ", "))
	}
	if len(pl.st.Havings) > 0 {
		p.text(" HAVING ")
		if err := pl.writePreds(p, pl.st.Havings, false); err != nil {
			return nil, err
		}
	}
	if len(pl.st.Orders) > 0 {
		parts := make([]string, len(pl.st.Orders))
		for i, o := range pl.st.Orders {
			q, err := pl.quoteCol(o.Col)
			if err != nil {
				return nil, err
			}
			dir, err := query.SafeOrderDir(o.Dir)
			if err != nil {
				return nil, err
			}
			parts[i] = q + " " + dir
		}
		p.text(" ORDER BY " + strings.Join(parts, ", "))
	}
	if err := pl.writeLimitOffset(p); err != nil {
		return nil, err
	}
	if pl.st.Lock != "" {
		p.text(" " + pl.lockSQL())
	}
	return p, nil
}

func (pl *planner) lockSQL() string {
	if pl.st.Lock == "FOR SHARE" && pl.dialect == query.DialectMySQL {
		return "LOCK IN SHARE MODE"
	}
	return pl.st.Lock
}

func (pl *planner) writeLimitOffset(p *plan) error {
	switch {
	case pl.st.LimitExpr != "":
		p.text(" LIMIT ")
		p.placeholder(pl.dialect)
		p.arg(pl.bindFn(pl.st.LimitExpr))
	case pl.st.Limit > 0:
		p.text(fmt.Sprintf(" LIMIT %d", pl.st.Limit))
	}
	switch {
	case pl.st.OffsetExpr != "":
		p.text(" OFFSET ")
		p.placeholder(pl.dialect)
		p.arg(pl.bindFn(pl.st.OffsetExpr))
	case pl.st.Offset > 0:
		p.text(fmt.Sprintf(" OFFSET %d", pl.st.Offset))
	}
	return nil
}

func (pl *planner) writeJoins(p *plan) error {
	for _, j := range pl.st.Joins {
		if err := query.SafeIdent(j.Table); err != nil {
			return err
		}
		if err := query.SafeOnClause(j.On); err != nil {
			return err
		}
		tq, err := pl.quote(j.Table)
		if err != nil {
			return err
		}
		p.text(" " + j.Type + " " + tq + " ON " + j.On)
	}
	return nil
}

// writeWhere emits the WHERE clause including the soft-delete filter, matching
// Builder.compileWhere.
func (pl *planner) writeWhere(p *plan) error {
	soft := pl.ms.SoftDeletes && !pl.st.WithTrashed
	if pl.st.OnlyTrashed {
		soft = true
	}
	if len(pl.st.Wheres) == 0 && !soft {
		return nil
	}
	p.text(" WHERE ")
	if len(pl.st.Wheres) == 0 {
		return pl.writeSoftFilter(p)
	}
	group := soft && wheresContainOr(pl.st.Wheres)
	if group {
		p.text("(")
	}
	if err := pl.writePreds(p, pl.st.Wheres, true); err != nil {
		return err
	}
	if group {
		p.text(")")
	}
	if !soft {
		return nil
	}
	p.text(" AND ")
	return pl.writeSoftFilter(p)
}

func (pl *planner) writeSoftFilter(p *plan) error {
	col, err := pl.quoteCol("deleted_at")
	if err != nil {
		return err
	}
	if pl.st.OnlyTrashed {
		p.text(col + " IS NOT NULL")
		return nil
	}
	p.text(col + " IS NULL")
	return nil
}

func wheresContainOr(preds []WhereSpec) bool {
	for i, w := range preds {
		if i > 0 && w.Or {
			return true
		}
	}
	return false
}

func (pl *planner) writePreds(p *plan, preds []WhereSpec, allowOr bool) error {
	for i, w := range preds {
		if i > 0 {
			if w.Or && allowOr {
				p.text(" OR ")
			} else {
				p.text(" AND ")
			}
		}
		if err := pl.writePred(p, w); err != nil {
			return err
		}
	}
	return nil
}

func (pl *planner) writeRaw(p *plan, w WhereSpec) error {
	pieces, err := query.SplitRawBinds(w.Raw)
	if err != nil {
		return fmt.Errorf("WhereRaw: %w", err)
	}
	n := 0
	for _, piece := range pieces {
		if piece.Bind {
			n++
		}
	}
	if n != len(w.Args) {
		return fmt.Errorf("WhereRaw has %d ? placeholders and %d arguments", n, len(w.Args))
	}
	p.text("(")
	ai := 0
	for _, piece := range pieces {
		if piece.Bind {
			p.placeholder(pl.dialect)
			p.arg(pl.bindFn(w.Args[ai]))
			ai++
			continue
		}
		p.text(piece.Text)
	}
	p.text(")")
	return nil
}

func (pl *planner) writePred(p *plan, w WhereSpec) error {
	switch w.Kind {
	case WhereRaw:
		return pl.writeRaw(p, w)

	case WhereSearch:
		op := "ILIKE"
		if pl.dialect == query.DialectMySQL {
			op = "LIKE"
		}
		// Build the escaped pattern once, then bind it to every column.
		pattern := fmt.Sprintf("pattern%d", len(p.locals)+1)
		p.addLocal(pattern, "query.LikePattern("+pl.bindFn(w.ArgExpr)+")")
		p.text("(")
		for i, c := range w.Cols {
			if i > 0 {
				p.text(" OR ")
			}
			q, err := pl.quoteCol(c)
			if err != nil {
				return err
			}
			p.text(q + " " + op + " ")
			p.placeholder(pl.dialect)
			p.arg(pattern)
		}
		p.text(")")
		return nil

	case WhereExists:
		return pl.writeExists(p, w)

	case WhereFTS:
		q, err := pl.quoteCol(w.Col)
		if err != nil {
			return err
		}
		if pl.dialect == query.DialectMySQL {
			p.text("MATCH (" + q + ") AGAINST (")
			p.placeholder(pl.dialect)
			p.arg(pl.bindFn(w.ArgExpr))
			p.text(")")
			return nil
		}
		p.text(q + " @@ to_tsquery('english', ")
		p.placeholder(pl.dialect)
		p.arg(pl.bindFn(w.ArgExpr))
		p.text(")")
		return nil

	case WhereJSON:
		q, err := pl.quoteCol(w.Col)
		if err != nil {
			return err
		}
		if pl.dialect == query.DialectMySQL {
			p.text("JSON_CONTAINS(" + q + ", ")
			p.placeholder(pl.dialect)
			p.arg(pl.bindFn(w.ArgExpr))
			p.text(")")
			return nil
		}
		p.text(q + " @> ")
		p.placeholder(pl.dialect)
		p.arg(pl.bindFn(w.ArgExpr))
		p.text("::jsonb")
		return nil
	}

	col, err := pl.quoteCol(w.Col)
	if err != nil {
		return err
	}

	switch w.Kind {
	case WhereNull:
		p.text(col + " IS NULL")
	case WhereNotNull:
		p.text(col + " IS NOT NULL")
	case WhereInList:
		if len(w.Args) == 0 {
			p.text("1 = 0")
			return nil
		}
		p.text(col + " IN (")
		for i, a := range w.Args {
			if i > 0 {
				p.text(", ")
			}
			p.placeholder(pl.dialect)
			p.arg(pl.bindFn(a))
		}
		p.text(")")
	case WhereInSlice:
		p.inGroup(col, pl.bindFn(w.ArgExpr), w.Negated)
	default:
		op := strings.ToUpper(strings.TrimSpace(w.Op))
		if op == "" {
			op = "="
		}
		if err := query.SafeOp(op); err != nil {
			return err
		}
		if (op == "ILIKE" || op == "NOT ILIKE") && pl.dialect == query.DialectMySQL {
			return fmt.Errorf("%s is PostgreSQL-only; use LIKE on MySQL/MariaDB", op)
		}
		p.text(col + " " + op + " ")
		p.placeholder(pl.dialect)
		p.arg(pl.bindFn(w.ArgExpr))
	}
	return nil
}

// deletePlan compiles DELETE FROM … WHERE …. A hard delete targets rows by
// predicate alone, so the soft-delete filter is never added.
func (pl *planner) deletePlan() (*plan, error) {
	tableQ, err := pl.quote(pl.ms.Table)
	if err != nil {
		return nil, err
	}
	p := &plan{}
	p.text("DELETE FROM " + tableQ)
	if err := pl.writeExplicitWhere(p); err != nil {
		return nil, err
	}
	return p, nil
}

// updatePlan compiles UPDATE … SET … WHERE … for Update/SoftDelete/Restore.
// applySoft keeps already soft-deleted rows out of the update, matching
// Builder.Update and Builder.SoftDelete.
func (pl *planner) updatePlan(sets []KVSpec, rawSets []string, applySoft bool) (*plan, error) {
	tableQ, err := pl.quote(pl.ms.Table)
	if err != nil {
		return nil, err
	}
	p := &plan{}
	p.text("UPDATE " + tableQ + " SET ")
	first := true
	for _, kv := range sets {
		if !first {
			p.text(", ")
		}
		first = false
		col, err := pl.quote(kv.Col)
		if err != nil {
			return nil, err
		}
		p.text(col + " = ")
		p.placeholder(pl.dialect)
		p.arg(pl.bindFn(kv.Expr))
	}
	for _, raw := range rawSets {
		if !first {
			p.text(", ")
		}
		first = false
		p.text(raw)
	}
	if applySoft {
		if err := pl.writeWhere(p); err != nil {
			return nil, err
		}
		return p, nil
	}
	if err := pl.writeExplicitWhere(p); err != nil {
		return nil, err
	}
	return p, nil
}

// writeExplicitWhere emits only the predicates the caller wrote.
func (pl *planner) writeExplicitWhere(p *plan) error {
	if len(pl.st.Wheres) == 0 {
		return nil
	}
	p.text(" WHERE ")
	return pl.writePreds(p, pl.st.Wheres, true)
}

func (pl *planner) aggregatePlan(fn, col string) (*plan, error) {
	if pl.ms.Table == "" {
		return nil, fmt.Errorf("cannot resolve model/entity %q — run vorm generate models", pl.st.Entity)
	}
	fn = strings.ToUpper(fn)
	switch fn {
	case "SUM", "AVG", "MIN", "MAX":
	default:
		return nil, fmt.Errorf("unknown aggregate %q", fn)
	}
	tableQ, err := pl.quote(pl.ms.Table)
	if err != nil {
		return nil, err
	}
	colQ, err := pl.quoteCol(col)
	if err != nil {
		return nil, err
	}
	p := &plan{}
	p.text("SELECT " + fn + "(" + colQ + ") FROM " + tableQ)
	if err := pl.writeJoins(p); err != nil {
		return nil, err
	}
	if err := pl.writeWhere(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (pl *planner) incrementPlan(col, amountExpr string, decrement bool) (*plan, error) {
	tableQ, err := pl.quote(pl.ms.Table)
	if err != nil {
		return nil, err
	}
	colQ, err := pl.quote(col)
	if err != nil {
		return nil, err
	}
	op := " + "
	if decrement {
		op = " - "
	}
	p := &plan{}
	p.text("UPDATE " + tableQ + " SET " + colQ + " = " + colQ + op)
	p.placeholder(pl.dialect)
	p.arg(pl.bindFn(amountExpr))
	if err := pl.writeWhere(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (pl *planner) writeRelSubselects(p *plan) error {
	for _, sub := range pl.st.RelSubselects {
		rel, ok := pl.ms.relation(sub.Name)
		if !ok {
			return fmt.Errorf("unknown relation %q", sub.Name)
		}
		inner, err := pl.existsInnerSQL(rel, WhereSpec{})
		if err != nil {
			return err
		}
		alias := strings.ReplaceAll(sub.Name, ".", "_")
		if sub.Count {
			alias += "_count"
			inner = strings.Replace(inner, "SELECT 1 FROM", "SELECT COUNT(*) FROM", 1)
			sql := "(" + inner + ")"
			aq, err := pl.quote(alias)
			if err != nil {
				return err
			}
			p.text(", " + sql + " AS " + aq)
			continue
		}
		alias += "_exists"
		aq, err := pl.quote(alias)
		if err != nil {
			return err
		}
		p.text(", (SELECT EXISTS (" + inner + ")) AS " + aq)
	}
	return nil
}

func (pl *planner) writeExists(p *plan, w WhereSpec) error {
	rel, ok := pl.ms.relation(w.RelName)
	if !ok {
		return fmt.Errorf("unknown relation %q", w.RelName)
	}
	if w.Not {
		p.text("NOT EXISTS (")
	} else {
		p.text("EXISTS (")
	}
	_, extra := splitExistsExtra(w)
	sql, err := pl.existsInnerSQL(rel, extra)
	if err != nil {
		return err
	}
	if extra.Col != "" && extra.Kind != WhereNull && extra.Kind != WhereNotNull {
		if err := pl.writeExistsWithBind(p, sql, extra); err != nil {
			return err
		}
		p.text(")")
		return nil
	}
	p.text(sql)
	p.text(")")
	return nil
}

func splitExistsExtra(w WhereSpec) (WhereSpec, WhereSpec) {
	if w.Col == "" && w.Kind == WhereExists {
		return w, WhereSpec{}
	}
	extra := w
	extra.RelName = ""
	if extra.Kind == WhereExists {
		extra.Kind = ""
	}
	return w, extra
}

func (pl *planner) writeExistsWithBind(p *plan, sql string, extra WhereSpec) error {
	const mark = "\x00ph\x00"
	before, after, ok := strings.Cut(sql, mark)
	if !ok {
		p.text(sql)
		return nil
	}
	p.text(before)
	p.placeholder(pl.dialect)
	p.arg(pl.bindFn(extra.ArgExpr))
	p.text(after)
	return nil
}

func (pl *planner) existsInnerSQL(rel RelSpec, extra WhereSpec) (string, error) {
	ea, err := existsFromRel(pl.ms.Table, pl.ms.PrimaryKey, rel, extra.Col != "")
	if err != nil {
		return "", err
	}
	relQ, err := pl.quote(ea.relatedTable)
	if err != nil {
		return "", err
	}
	relCol, err := query.QuoteIdent(pl.dialect, ea.relatedTable+"."+ea.relatedCol)
	if err != nil {
		return "", err
	}
	parentCol, err := query.QuoteIdent(pl.dialect, ea.parentTable+"."+ea.parentCol)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	sb.WriteString("SELECT 1 FROM ")
	sb.WriteString(relQ)
	if ea.joinTable != "" {
		joinQ, err := pl.quote(ea.joinTable)
		if err != nil {
			return "", err
		}
		left, err := query.QuoteIdent(pl.dialect, ea.joinOnLeft)
		if err != nil {
			return "", err
		}
		right, err := query.QuoteIdent(pl.dialect, ea.joinOnRight)
		if err != nil {
			return "", err
		}
		sb.WriteString(" INNER JOIN ")
		sb.WriteString(joinQ)
		sb.WriteString(" ON ")
		sb.WriteString(left)
		sb.WriteString(" = ")
		sb.WriteString(right)
	}
	sb.WriteString(" WHERE ")
	sb.WriteString(relCol)
	sb.WriteString(" = ")
	sb.WriteString(parentCol)
	if ea.morphTypeCol != "" && ea.morphType != "" {
		if err := query.SafeIdent(ea.morphType); err != nil {
			return "", err
		}
		typeQ, err := query.QuoteIdent(pl.dialect, ea.relatedTable+"."+ea.morphTypeCol)
		if err != nil {
			return "", err
		}
		sb.WriteString(" AND ")
		sb.WriteString(typeQ)
		sb.WriteString(" = '")
		sb.WriteString(strings.ReplaceAll(ea.morphType, "'", "''"))
		sb.WriteString("'")
	}
	if extra.Col == "" {
		return sb.String(), nil
	}
	col := extra.Col
	if !strings.Contains(col, ".") {
		if ea.joinTable != "" {
			col = ea.joinTable + "." + col
		} else {
			col = ea.relatedTable + "." + col
		}
	}
	qc, err := query.QuoteIdent(pl.dialect, col)
	if err != nil {
		return "", err
	}
	switch extra.Kind {
	case WhereNull:
		sb.WriteString(" AND ")
		sb.WriteString(qc)
		sb.WriteString(" IS NULL")
	case WhereNotNull:
		sb.WriteString(" AND ")
		sb.WriteString(qc)
		sb.WriteString(" IS NOT NULL")
	default:
		op := strings.ToUpper(strings.TrimSpace(extra.Op))
		if op == "" {
			op = "="
		}
		if err := query.SafeOp(op); err != nil {
			return "", err
		}
		sb.WriteString(" AND ")
		sb.WriteString(qc)
		sb.WriteString(" ")
		sb.WriteString(op)
		sb.WriteString(" ")
		sb.WriteString("\x00ph\x00")
	}
	return sb.String(), nil
}

type existsPlan struct {
	relatedTable, relatedCol           string
	parentTable, parentCol             string
	joinTable, joinOnLeft, joinOnRight string
	morphType, morphTypeCol            string
}

func existsFromRel(parentTable, parentPK string, rel RelSpec, extra bool) (existsPlan, error) {
	parentCol := rel.LocalKey
	if parentCol == "" {
		parentCol = parentPK
	}
	ea := existsPlan{
		parentTable:  parentTable,
		parentCol:    parentCol,
		morphType:    rel.MorphType,
		morphTypeCol: rel.MorphTypeColumn,
	}
	switch rel.Kind {
	case query.RelationBelongsTo:
		ea.relatedTable = rel.Table
		ea.relatedCol = rel.ForeignKey
		if ea.relatedCol == "" {
			ea.relatedCol = "id"
		}
		ea.parentCol = rel.LocalKey
	case query.RelationHasMany, query.RelationHasOne, query.RelationMorphMany, query.RelationHasOneOfMany:
		ea.relatedTable = rel.Table
		ea.relatedCol = rel.ForeignKey
		ea.parentCol = rel.LocalKey
	case query.RelationHasManyThrough:
		if rel.PivotTable == "" || rel.PivotLocalKey == "" {
			return ea, fmt.Errorf("hasManyThrough %q needs through keys", rel.Name)
		}
		ea.relatedTable = rel.PivotTable
		ea.relatedCol = rel.PivotLocalKey
		ea.joinTable = rel.Table
		farPK := rel.PivotForeignKey
		if farPK == "" {
			farPK = "id"
		}
		ea.joinOnLeft = rel.PivotTable + "." + farPK
		ea.joinOnRight = rel.Table + "." + rel.ForeignKey
	case query.RelationBelongsToMany, query.RelationMorphToMany:
		if rel.PivotTable == "" {
			return ea, fmt.Errorf("belongsToMany %q needs a pivot", rel.Name)
		}
		ea.relatedTable = rel.PivotTable
		ea.relatedCol = rel.PivotLocalKey
		ea.parentCol = rel.LocalKey
		if extra {
			pk := rel.ForeignKey
			if pk == "" {
				pk = "id"
			}
			ea.joinTable = rel.Table
			ea.joinOnLeft = rel.PivotTable + "." + rel.PivotForeignKey
			ea.joinOnRight = rel.Table + "." + pk
		}
	default:
		return ea, fmt.Errorf("WhereHas does not support %s", rel.Kind)
	}
	return ea, nil
}
