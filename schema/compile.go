package schema

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"

	"github.com/vorzela/vorm/migrate"
)

func init() {
	migrate.CompileGo = func(filename, src, dialect string) (up, down string, err error) {
		c, err := CompileSource(filename, src, dialect)
		if err != nil {
			return "", "", err
		}
		return c.UpSQL, c.DownSQL, nil
	}
}

// Compiled is the SQL produced from a Go Blueprint migration's Up/Down.
type Compiled struct {
	UpSQL   string
	DownSQL string
}

// CompileFile parses a Go migration and compiles Up/Down to SQL.
func CompileFile(path, dialect string) (Compiled, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return Compiled{}, fmt.Errorf("schema: read %s: %w", path, err)
	}
	return CompileSource(path, string(src), dialect)
}

// CompileSource compiles a Go migration already in memory. Up/Down are never
// executed: Facade.Create would write files and AutoMigrate would hit the
// database. The AST is replayed onto a live Blueprint instead.
func CompileSource(filename, src, dialect string) (Compiled, error) {
	if dialect == "" {
		dialect = "postgres"
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, parser.SkipObjectResolution)
	if err != nil {
		return Compiled{}, fmt.Errorf("schema: parse %s: %w", filename, err)
	}
	upFn := findFunc(f, "Up")
	if upFn == nil {
		return Compiled{}, fmt.Errorf("schema: %s: missing func Up", filename)
	}
	up, reverse, err := compileFunc(filename, upFn, dialect)
	if err != nil {
		return Compiled{}, err
	}
	var down string
	if downFn := findFunc(f, "Down"); downFn != nil {
		down, _, err = compileFunc(filename, downFn, dialect)
		if err != nil {
			return Compiled{}, err
		}
	}
	if strings.TrimSpace(down) == "" {
		down = reverse
	}
	return Compiled{UpSQL: up, DownSQL: down}, nil
}

func findFunc(f *ast.File, name string) *ast.FuncDecl {
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if ok && fn.Recv == nil && fn.Name != nil && fn.Name.Name == name {
			return fn
		}
	}
	return nil
}

func compileFunc(filename string, fn *ast.FuncDecl, dialect string) (forward, reverse string, err error) {
	calls, err := facadeCalls(filename, fn)
	if err != nil {
		return "", "", err
	}
	var ups, downs []string
	for _, call := range calls {
		u, d, err := compileFacadeCall(filename, call, dialect)
		if err != nil {
			return "", "", err
		}
		if strings.TrimSpace(u) != "" {
			ups = append(ups, u)
		}
		if strings.TrimSpace(d) != "" {
			downs = append(downs, d)
		}
	}
	rev := make([]string, 0, len(downs))
	for i := len(downs) - 1; i >= 0; i-- {
		rev = append(rev, downs[i])
	}
	return joinSQL(ups), joinSQL(rev), nil
}

func facadeCalls(filename string, fn *ast.FuncDecl) ([]*ast.CallExpr, error) {
	if fn.Body == nil {
		return nil, nil
	}
	var out []*ast.CallExpr
	for _, stmt := range fn.Body.List {
		calls, err := facadeCallsStmt(filename, stmt)
		if err != nil {
			return nil, err
		}
		out = append(out, calls...)
	}
	return out, nil
}

func facadeCallsStmt(filename string, stmt ast.Stmt) ([]*ast.CallExpr, error) {
	switch s := stmt.(type) {
	case *ast.ExprStmt:
		return facadeCallFromExpr(s.X), nil
	case *ast.ReturnStmt:
		var out []*ast.CallExpr
		for _, e := range s.Results {
			out = append(out, facadeCallFromExpr(e)...)
		}
		return out, nil
	case *ast.AssignStmt:
		var out []*ast.CallExpr
		for _, e := range s.Rhs {
			out = append(out, facadeCallFromExpr(e)...)
		}
		return out, nil
	case *ast.DeclStmt, *ast.EmptyStmt:
		return nil, nil
	case *ast.BlockStmt:
		var out []*ast.CallExpr
		for _, inner := range s.List {
			calls, err := facadeCallsStmt(filename, inner)
			if err != nil {
				return nil, err
			}
			out = append(out, calls...)
		}
		return out, nil
	case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt, *ast.GoStmt, *ast.DeferStmt:
		return nil, fmt.Errorf("schema: %s: control flow in Up/Down is not compiled — keep the function linear", filename)
	default:
		return nil, nil
	}
}

func facadeCallFromExpr(e ast.Expr) []*ast.CallExpr {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return nil
	}
	if _, ok := facadeMethod(call); ok {
		return []*ast.CallExpr{call}
	}
	return nil
}

func facadeMethod(call *ast.CallExpr) (string, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel == nil {
		return "", false
	}
	switch sel.Sel.Name {
	case "Create", "Table", "DropIfExists", "Drop", "BelongsToMany", "MorphToMany", "CreateExtension", "CreateEnum", "CreateFunction":
		return sel.Sel.Name, true
	}
	return "", false
}

func compileFacadeCall(filename string, call *ast.CallExpr, dialect string) (up, down string, err error) {
	name, _ := facadeMethod(call)
	switch name {
	case "Create":
		table, build, err := createArgs(filename, call)
		if err != nil {
			return "", "", err
		}
		bp := NewBlueprint(table)
		if err := applyBlueprintBody(filename, bp, build); err != nil {
			return "", "", err
		}
		if err := ValidateBlueprint(bp); err != nil {
			return "", "", err
		}
		up, down = bp.Compile(dialect)
		return up, down, nil
	case "Table":
		table, build, err := createArgs(filename, call)
		if err != nil {
			return "", "", err
		}
		bp := NewAlterBlueprint(table)
		if err := applyBlueprintBody(filename, bp, build); err != nil {
			return "", "", err
		}
		if err := ValidateBlueprint(bp); err != nil {
			return "", "", err
		}
		up, down = bp.Compile(dialect)
		return up, down, nil
	case "DropIfExists", "Drop":
		table, err := stringLiteral(call.Args, 0)
		if err != nil {
			return "", "", fmt.Errorf("schema: %s: %s needs a table name: %w", filename, name, err)
		}
		return CompileDropIfExists(table, dialect), fmt.Sprintf("-- restore %s manually", table), nil
	case "BelongsToMany":
		left, err := stringLiteral(call.Args, 0)
		if err != nil {
			return "", "", fmt.Errorf("schema: %s: BelongsToMany needs left table: %w", filename, err)
		}
		right, err := stringLiteral(call.Args, 1)
		if err != nil {
			return "", "", fmt.Errorf("schema: %s: BelongsToMany needs right table: %w", filename, err)
		}
		bp := NewPivotBlueprint(left, right)
		if err := ValidateBlueprint(bp); err != nil {
			return "", "", err
		}
		up, down = bp.Compile(dialect)
		return up, down, nil
	case "MorphToMany":
		related, err := stringLiteral(call.Args, 0)
		if err != nil {
			return "", "", fmt.Errorf("schema: %s: MorphToMany needs related table: %w", filename, err)
		}
		morph, err := stringLiteral(call.Args, 1)
		if err != nil {
			return "", "", fmt.Errorf("schema: %s: MorphToMany needs morph name: %w", filename, err)
		}
		bp := NewMorphPivotBlueprint(related, morph)
		if err := ValidateBlueprint(bp); err != nil {
			return "", "", err
		}
		up, down = bp.Compile(dialect)
		return up, down, nil
	case "CreateExtension":
		ext, err := stringLiteral(call.Args, 0)
		if err != nil {
			return "", "", fmt.Errorf("schema: %s: CreateExtension needs a name: %w", filename, err)
		}
		if !strings.EqualFold(dialect, "postgres") {
			return "", "", fmt.Errorf("schema: extensions require postgres (got %s)", dialect)
		}
		up = fmt.Sprintf("CREATE EXTENSION IF NOT EXISTS %q;", ext)
		down = fmt.Sprintf("DROP EXTENSION IF EXISTS %q CASCADE;", ext)
		return up, down, nil
	case "CreateEnum":
		typeName, err := stringLiteral(call.Args, 0)
		if err != nil {
			return "", "", fmt.Errorf("schema: %s: CreateEnum needs a type name: %w", filename, err)
		}
		vals := make([]string, 0, len(call.Args)-1)
		for i := 1; i < len(call.Args); i++ {
			v, err := stringLiteral(call.Args, i)
			if err != nil {
				return "", "", fmt.Errorf("schema: %s: CreateEnum values must be string literals", filename)
			}
			vals = append(vals, v)
		}
		if len(vals) == 0 {
			return "", "", fmt.Errorf("schema: %s: enum %q needs at least one value", filename, typeName)
		}
		if !strings.EqualFold(dialect, "postgres") {
			return "", "", fmt.Errorf("schema: enums require postgres (got %s)", dialect)
		}
		quoted := quoteEnumValues(vals)
		up = fmt.Sprintf("CREATE TYPE %s AS ENUM (%s);", typeName, quoted)
		down = fmt.Sprintf("DROP TYPE IF EXISTS %s CASCADE;", typeName)
		return up, down, nil
	case "CreateFunction":
		fnName, err := stringLiteral(call.Args, 0)
		if err != nil {
			return "", "", fmt.Errorf("schema: %s: CreateFunction needs a name: %w", filename, err)
		}
		body, err := stringLiteral(call.Args, 1)
		if err != nil {
			return "", "", fmt.Errorf("schema: %s: CreateFunction needs an up SQL string: %w", filename, err)
		}
		up = strings.TrimSpace(body)
		down = fmt.Sprintf("DROP FUNCTION IF EXISTS %s CASCADE;", fnName)
		return up, down, nil
	default:
		return "", "", fmt.Errorf("schema: %s: unsupported Schema method %s", filename, name)
	}
}

func createArgs(filename string, call *ast.CallExpr) (table string, body *ast.BlockStmt, err error) {
	if len(call.Args) < 2 {
		return "", nil, fmt.Errorf("schema: %s: Create/Table needs a table name and Blueprint callback", filename)
	}
	table, err = stringLiteral(call.Args, 0)
	if err != nil {
		return "", nil, fmt.Errorf("schema: %s: table name must be a string literal: %w", filename, err)
	}
	lit, ok := call.Args[1].(*ast.FuncLit)
	if !ok || lit.Body == nil {
		return "", nil, fmt.Errorf("schema: %s: Blueprint callback must be an inline func(*Blueprint)", filename)
	}
	return table, lit.Body, nil
}

func applyBlueprintBody(filename string, bp *Blueprint, body *ast.BlockStmt) error {
	if body == nil {
		return nil
	}
	for _, stmt := range body.List {
		if err := applyBlueprintStmt(filename, bp, stmt); err != nil {
			return err
		}
	}
	return nil
}

func applyBlueprintStmt(filename string, bp *Blueprint, stmt ast.Stmt) error {
	switch s := stmt.(type) {
	case *ast.ExprStmt:
		call, ok := s.X.(*ast.CallExpr)
		if !ok {
			return nil
		}
		return applyBlueprintCall(filename, bp, call)
	case *ast.AssignStmt:
		for _, e := range s.Rhs {
			call, ok := e.(*ast.CallExpr)
			if !ok {
				continue
			}
			if err := applyBlueprintCall(filename, bp, call); err != nil {
				return err
			}
		}
		return nil
	case *ast.BlockStmt:
		return applyBlueprintBody(filename, bp, s)
	case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt:
		return fmt.Errorf("schema: %s: control flow in a Blueprint callback is not compiled", filename)
	default:
		return nil
	}
}

func applyBlueprintCall(filename string, bp *Blueprint, call *ast.CallExpr) error {
	steps := flattenCall(call)
	if len(steps) == 0 {
		return fmt.Errorf("schema: %s: expected a Blueprint method call", filename)
	}
	col, err := applyRoot(filename, bp, steps[0])
	if err != nil {
		return err
	}
	for _, step := range steps[1:] {
		if col == nil {
			return fmt.Errorf("schema: %s: .%s() chained after a Blueprint method that does not return a column", filename, step.name)
		}
		if err := applyColumnMethod(filename, col, step); err != nil {
			return err
		}
	}
	if col != nil && col.custom && col.name == "" {
		return fmt.Errorf("schema: %s: CustomType requires .Column(\"name\")", filename)
	}
	return nil
}

type callStep struct {
	name string
	args []ast.Expr
}

func flattenCall(call *ast.CallExpr) []callStep {
	var steps []callStep
	cur := ast.Expr(call)
	for {
		c, ok := cur.(*ast.CallExpr)
		if !ok {
			break
		}
		sel, ok := c.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel == nil {
			break
		}
		steps = append(steps, callStep{name: sel.Sel.Name, args: c.Args})
		cur = sel.X
	}
	for i, j := 0, len(steps)-1; i < j; i, j = i+1, j-1 {
		steps[i], steps[j] = steps[j], steps[i]
	}
	return steps
}

func applyRoot(filename string, bp *Blueprint, root callStep) (*Column, error) {
	switch root.name {
	case "ID", "Id":
		return bp.ID(), nil
	case "BigIncrements":
		name, err := stringLiteral(root.args, 0)
		if err != nil {
			return nil, fmt.Errorf("schema: %s: BigIncrements needs a column name", filename)
		}
		return bp.BigIncrements(name), nil
	case "UUID":
		name, err := stringLiteral(root.args, 0)
		if err != nil {
			return nil, fmt.Errorf("schema: %s: UUID needs a column name", filename)
		}
		return bp.UUID(name), nil
	case "String":
		name, err := stringLiteral(root.args, 0)
		if err != nil {
			return nil, fmt.Errorf("schema: %s: String needs a column name", filename)
		}
		if len(root.args) > 1 {
			n, err := intLiteral(root.args[1])
			if err != nil {
				return nil, fmt.Errorf("schema: %s: String length must be an int literal: %w", filename, err)
			}
			return bp.String(name, n), nil
		}
		return bp.String(name), nil
	case "Text":
		name, err := stringLiteral(root.args, 0)
		if err != nil {
			return nil, fmt.Errorf("schema: %s: Text needs a column name", filename)
		}
		return bp.Text(name), nil
	case "Boolean":
		name, err := stringLiteral(root.args, 0)
		if err != nil {
			return nil, fmt.Errorf("schema: %s: Boolean needs a column name", filename)
		}
		return bp.Boolean(name), nil
	case "Integer":
		name, err := stringLiteral(root.args, 0)
		if err != nil {
			return nil, fmt.Errorf("schema: %s: Integer needs a column name", filename)
		}
		return bp.Integer(name), nil
	case "BigInteger":
		name, err := stringLiteral(root.args, 0)
		if err != nil {
			return nil, fmt.Errorf("schema: %s: BigInteger needs a column name", filename)
		}
		return bp.BigInteger(name), nil
	case "ForeignID", "ForeignId":
		name, err := stringLiteral(root.args, 0)
		if err != nil {
			return nil, fmt.Errorf("schema: %s: ForeignId needs a column name", filename)
		}
		return bp.ForeignID(name), nil
	case "ForeignIDNullable":
		name, err := stringLiteral(root.args, 0)
		if err != nil {
			return nil, fmt.Errorf("schema: %s: ForeignIDNullable needs a column name", filename)
		}
		return bp.ForeignIDNullable(name), nil
	case "BelongsTo":
		col, err := stringLiteral(root.args, 0)
		if err != nil {
			return nil, fmt.Errorf("schema: %s: BelongsTo needs a column name", filename)
		}
		table, err := stringLiteral(root.args, 1)
		if err != nil {
			return nil, fmt.Errorf("schema: %s: BelongsTo needs a table name", filename)
		}
		return bp.BelongsTo(col, table), nil
	case "Enum":
		col, err := stringLiteral(root.args, 0)
		if err != nil {
			return nil, fmt.Errorf("schema: %s: Enum needs a column name", filename)
		}
		vals := make([]string, 0, len(root.args)-1)
		for i := 1; i < len(root.args); i++ {
			v, err := stringLiteral(root.args, i)
			if err != nil {
				return nil, fmt.Errorf("schema: %s: Enum values must be string literals", filename)
			}
			vals = append(vals, v)
		}
		if len(vals) == 0 {
			return nil, fmt.Errorf("schema: %s: Enum %q needs at least one value", filename, col)
		}
		return bp.Enum(col, vals...), nil
	case "Timestamps":
		bp.Timestamps()
		return nil, nil
	case "SoftDeletes":
		bp.SoftDeletes()
		return nil, nil
	case "Morphs":
		name, err := stringLiteral(root.args, 0)
		if err != nil {
			return nil, fmt.Errorf("schema: %s: Morphs needs a name", filename)
		}
		bp.Morphs(name)
		return nil, nil
	case "CustomType":
		sqlType, err := stringLiteral(root.args, 0)
		if err != nil {
			return nil, fmt.Errorf("schema: %s: CustomType needs a SQL type string", filename)
		}
		if !sqlTypeRe.MatchString(strings.TrimSpace(sqlType)) {
			return nil, fmt.Errorf("schema: %s: invalid SQL type %q", filename, sqlType)
		}
		return bp.CustomType(sqlType), nil
	case "Index":
		cols, err := stringLiterals(root.args)
		if err != nil || len(cols) == 0 {
			return nil, fmt.Errorf("schema: %s: Index needs column names", filename)
		}
		bp.Index(cols...)
		return nil, nil
	case "Unique":
		if len(root.args) == 0 {
			return nil, fmt.Errorf("schema: %s: Blueprint.Unique needs column names (column.Unique() is chained)", filename)
		}
		cols, err := stringLiterals(root.args)
		if err != nil {
			return nil, fmt.Errorf("schema: %s: Unique needs column names", filename)
		}
		bp.Unique(cols...)
		return nil, nil
	case "DropColumn":
		name, err := stringLiteral(root.args, 0)
		if err != nil {
			return nil, fmt.Errorf("schema: %s: DropColumn needs a name", filename)
		}
		bp.DropColumn(name)
		return nil, nil
	case "DropIndex":
		name, err := stringLiteral(root.args, 0)
		if err != nil {
			return nil, fmt.Errorf("schema: %s: DropIndex needs a name", filename)
		}
		bp.DropIndex(name)
		return nil, nil
	case "Raw":
		up, err := stringLiteral(root.args, 0)
		if err != nil {
			return nil, fmt.Errorf("schema: %s: Raw needs an up SQL string", filename)
		}
		down := ""
		if len(root.args) > 1 {
			down, _ = stringLiteral(root.args, 1)
		}
		bp.Raw(up, down)
		return nil, nil
	default:
		return nil, fmt.Errorf("schema: %s: unknown Blueprint method %s", filename, root.name)
	}
}

func applyColumnMethod(filename string, col *Column, step callStep) error {
	switch step.name {
	case "Nullable":
		col.Nullable()
		return nil
	case "NotNull":
		col.NotNull()
		return nil
	case "Unique":
		col.Unique()
		return nil
	case "Default":
		if len(step.args) == 0 {
			return fmt.Errorf("schema: %s: Default needs a value", filename)
		}
		v, err := astValue(step.args[0])
		if err != nil {
			return fmt.Errorf("schema: %s: Default: %w", filename, err)
		}
		col.Default(v)
		return nil
	case "DefaultCurrent":
		col.DefaultCurrent()
		return nil
	case "Constrained":
		table := ""
		if len(step.args) > 0 {
			t, err := stringLiteral(step.args, 0)
			if err != nil {
				return fmt.Errorf("schema: %s: Constrained table must be a string literal", filename)
			}
			table = t
		}
		if table == "" {
			if !strings.HasSuffix(col.name, "_id") {
				return fmt.Errorf("schema: %s: Constrained() could not infer a table from %q — pass Constrained(\"table\")", filename, col.name)
			}
			table = Pluralize(strings.TrimSuffix(col.name, "_id"))
		}
		col.Constrained(table)
		return nil
	case "References":
		if len(step.args) < 2 {
			return fmt.Errorf("schema: %s: References needs table and column", filename)
		}
		table, err := stringLiteral(step.args, 0)
		if err != nil {
			return fmt.Errorf("schema: %s: References table must be a string literal", filename)
		}
		column, err := stringLiteral(step.args, 1)
		if err != nil {
			return fmt.Errorf("schema: %s: References column must be a string literal", filename)
		}
		col.References(table, column)
		return nil
	case "CascadeOnDelete":
		col.CascadeOnDelete()
		return nil
	case "RestrictOnDelete":
		col.RestrictOnDelete()
		return nil
	case "NullOnDelete":
		col.NullOnDelete()
		return nil
	case "CascadeOnUpdate":
		col.CascadeOnUpdate()
		return nil
	case "Column":
		name, err := stringLiteral(step.args, 0)
		if err != nil {
			return fmt.Errorf("schema: %s: Column needs a column name", filename)
		}
		if err := col.setName(name); err != nil {
			return fmt.Errorf("schema: %s: %w", filename, err)
		}
		return nil
	default:
		return fmt.Errorf("schema: %s: unknown column method %s", filename, step.name)
	}
}

func stringLiteral(args []ast.Expr, i int) (string, error) {
	if i >= len(args) {
		return "", fmt.Errorf("missing argument %d", i)
	}
	return exprString(args[i])
}

func stringLiterals(args []ast.Expr) ([]string, error) {
	out := make([]string, 0, len(args))
	for i := range args {
		s, err := stringLiteral(args, i)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func exprString(e ast.Expr) (string, error) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", fmt.Errorf("expected string literal")
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", err
	}
	return s, nil
}

func intLiteral(e ast.Expr) (int, error) {
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.SUB {
		n, err := intLiteral(u.X)
		return -n, err
	}
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return 0, fmt.Errorf("expected int literal")
	}
	n, err := strconv.Atoi(lit.Value)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func astValue(e ast.Expr) (any, error) {
	switch v := e.(type) {
	case *ast.Ident:
		switch v.Name {
		case "true":
			return true, nil
		case "false":
			return false, nil
		case "nil":
			return nil, nil
		default:
			return nil, fmt.Errorf("unsupported identifier %s", v.Name)
		}
	case *ast.BasicLit:
		switch v.Kind {
		case token.STRING:
			return exprString(v)
		case token.INT:
			n, err := strconv.Atoi(v.Value)
			return n, err
		case token.FLOAT:
			return strconv.ParseFloat(v.Value, 64)
		default:
			return nil, fmt.Errorf("unsupported literal")
		}
	case *ast.UnaryExpr:
		inner, err := astValue(v.X)
		if err != nil {
			return nil, err
		}
		if v.Op == token.SUB {
			switch n := inner.(type) {
			case int:
				return -n, nil
			case float64:
				return -n, nil
			}
		}
		return nil, fmt.Errorf("unsupported unary expression")
	default:
		return nil, fmt.Errorf("Default value must be a bool, number, or string literal")
	}
}

func joinSQL(parts []string) string {
	var b strings.Builder
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(p)
	}
	return b.String()
}

// CompileDropIfExists returns dialect-correct DROP TABLE SQL.
func CompileDropIfExists(table, dialect string) string {
	if dialect == "mysql" || dialect == "mariadb" {
		return fmt.Sprintf("DROP TABLE IF EXISTS %s;", table)
	}
	return fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE;", table)
}
