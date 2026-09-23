package generate

import (
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Options controls code generation (vorm/gen only — no sqlc).
type Options struct {
	QueryDir     string // default ./queries
	OutDir       string // default ./vorm/gen
	ModelDir     string // default ./models — for column checks + Scan
	ModelImport  string // import path for models package (e.g. myapp/models)
	Package      string // generated Go package name (default gen)
	ModelPackage string // models package name used in types (default models)
	Dialect      string // postgres|mysql|mariadb
	Driver       string // pgx (default) | pq — documented in generated header
}

// Result summarizes generation.
type Result struct {
	Queries   int
	GoFiles   []string
	StubsSeen []string
	Dialect   string
	Driver    string

	// Pending lists stubs that stayed on the runtime builder, with the reason.
	Pending []PendingStub
}

// PendingStub is one // vorm:query function that could not be lowered to SQL.
type PendingStub struct {
	Name   string
	File   string
	Reason string
}

// Run scans // vorm:query stubs, lowers fluent chains to parameterized SQL,
// and emits typed Go under vorm/gen (return models, never SELECT *).
func Run(opts *Options) (*Result, error) {
	if opts == nil {
		opts = &Options{}
	}
	if opts.QueryDir == "" {
		opts.QueryDir = DefaultQueryDir
	}
	if opts.OutDir == "" {
		opts.OutDir = DefaultOutDir
	}
	if opts.ModelDir == "" {
		opts.ModelDir = DefaultModelDir
	}
	if opts.Dialect == "" {
		opts.Dialect = "postgres"
	}
	if opts.Driver == "" {
		opts.Driver = "pgx"
	}
	if opts.Package == "" {
		opts.Package = "gen"
	}
	if opts.ModelPackage == "" {
		opts.ModelPackage = "models"
	}
	if opts.ModelImport == "" {
		opts.ModelImport = detectModelImport(opts.ModelDir)
	}
	dialect := strings.ToLower(opts.Dialect)
	driver := strings.ToLower(opts.Driver)

	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return nil, err
	}

	models, err := parseModelsDir(opts.ModelDir)
	if err != nil {
		return nil, err
	}

	stubs, err := findAnnotatedStubs(opts.QueryDir, models)
	if err != nil {
		return nil, err
	}

	res := &Result{
		Queries:   len(stubs),
		StubsSeen: stubNames(stubs),
		Dialect:   dialect,
		Driver:    driver,
		Pending:   pendingStubs(stubs),
	}

	keep := map[string]bool{}
	if len(stubs) > 0 {
		dbName := "db.go"
		dbPath := filepath.Join(opts.OutDir, dbName)
		if err := writeQueryGo(dbPath, emitDBFile(opts)); err != nil {
			return nil, err
		}
		keep[dbName] = true
		res.GoFiles = append(res.GoFiles, dbPath)

		for _, g := range groupStubsBySource(opts.QueryDir, stubs) {
			body, err := emitQueryFile(opts, g.Stubs, models)
			if err != nil {
				return nil, err
			}
			path := filepath.Join(opts.OutDir, g.Name)
			if err := writeQueryGo(path, body); err != nil {
				return nil, err
			}
			keep[g.Name] = true
			res.GoFiles = append(res.GoFiles, path)
		}
	}
	if err := pruneGoFiles(opts.OutDir, keep); err != nil {
		return nil, err
	}
	return res, nil
}

func writeQueryGo(path, body string) error {
	src, err := format.Source([]byte(body))
	if err != nil {
		// Keep the unformatted source on disk: the compiler error points at the
		// generator bug far better than a format error does.
		src = []byte(body)
	}
	return os.WriteFile(path, src, 0o644)
}

type stubGroup struct {
	Name  string
	Stubs []StubFunc
}

// groupStubsBySource buckets stubs into one output file per source, sqlc-style.
func groupStubsBySource(queryDir string, stubs []StubFunc) []stubGroup {
	order := make([]string, 0)
	byName := map[string][]StubFunc{}
	for _, s := range stubs {
		name := queryGoFileName(queryDir, s.File)
		if _, ok := byName[name]; !ok {
			order = append(order, name)
		}
		byName[name] = append(byName[name], s)
	}
	sort.Strings(order)
	out := make([]stubGroup, 0, len(order))
	for _, name := range order {
		out = append(out, stubGroup{Name: name, Stubs: byName[name]})
	}
	return out
}

// queryGoFileName maps a stub source path to an output name:
//
//	queries/users.go       → users.sql.go
//	queries/admin/users.go → admin_users.sql.go
func queryGoFileName(queryDir, stubFile string) string {
	rel := stubFile
	if queryDir != "" {
		if r, err := filepath.Rel(queryDir, stubFile); err == nil && r != "." && !strings.HasPrefix(r, "..") {
			rel = r
		} else {
			rel = filepath.Base(stubFile)
		}
	}
	rel = strings.TrimSuffix(filepath.ToSlash(rel), ".go")
	rel = strings.ReplaceAll(rel, "/", "_")
	if rel == "" || rel == "." {
		rel = "queries"
	}
	return rel + ".sql.go"
}

func pruneGoFiles(dir string, keep map[string]bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		if keep[e.Name()] {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func pendingStubs(stubs []StubFunc) []PendingStub {
	var out []PendingStub
	for _, s := range stubs {
		if !s.Pending && s.Action != "" {
			continue
		}
		out = append(out, PendingStub{Name: s.Name, File: s.File, Reason: pendingReason(s)})
	}
	return out
}

func stubNames(stubs []StubFunc) []string {
	out := make([]string, len(stubs))
	for i, s := range stubs {
		out[i] = s.Name
	}
	return out
}

func parseQueryNameLine(line string) string {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "vorm:query"))
	for _, part := range strings.Fields(rest) {
		if after, ok := strings.CutPrefix(part, "name="); ok {
			return after
		}
	}
	return ""
}
