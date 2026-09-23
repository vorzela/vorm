package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vorzela/vorm/schema"
)

// Severity of a finding.
type Severity string

const (
	Error   Severity = "error"
	Warning Severity = "warning"
	Hint    Severity = "hint"
)

// Finding is one lint result.
type Finding struct {
	File       string
	Severity   Severity
	Message    string
	Suggestion string
}

// Result aggregates findings.
type Result struct {
	Findings []Finding
}

func (r *Result) HasErrors() bool {
	for _, f := range r.Findings {
		if f.Severity == Error {
			return true
		}
	}
	return false
}

// Dir lints numbered *.sql and *.go migrations under path.
func Dir(path string) (*Result, error) {
	res := &Result{}
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			res.Findings = append(res.Findings, Finding{
				Severity:   Warning,
				Message:    "migrations directory missing: " + path,
				Suggestion: "run: vorm make migration <name>",
			})
			return res, nil
		}
		return nil, err
	}
	if len(entries) == 0 {
		res.Findings = append(res.Findings, Finding{
			Severity:   Hint,
			Message:    "no migration files yet",
			Suggestion: "vorm make migration users",
		})
		return res, nil
	}
	migrations := 0
	for _, e := range entries {
		if e.IsDir() || !isMigrationFile(e.Name()) {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext != ".sql" && ext != ".go" {
			continue
		}
		if strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		migrations++
		full := filepath.Join(path, e.Name())
		body, err := os.ReadFile(full)
		if err != nil {
			return nil, err
		}
		if ext == ".go" {
			res.Findings = append(res.Findings, GoFile(full, string(body))...)
			continue
		}
		res.Findings = append(res.Findings, File(full, string(body))...)
	}
	if migrations == 0 {
		res.Findings = append(res.Findings, Finding{
			Severity:   Hint,
			Message:    "no migration files yet",
			Suggestion: "vorm make migration users",
		})
	}
	return res, nil
}

// isMigrationFile reports whether name belongs to the migration sequence. The
// declarative helpers — extensions.sql, enums.sql, functions.sql — share the
// directory but have no Up/Down sections, so linting them as migrations would
// report errors for files that are correct as they are.
func isMigrationFile(name string) bool {
	return len(name) > 0 && name[0] >= '0' && name[0] <= '9'
}

// File lints a single migration body.
func File(path, body string) []Finding {
	var out []Finding
	lower := strings.ToLower(body)
	name := filepath.Base(path)

	if !strings.Contains(body, "⬆ Up") && !strings.Contains(lower, "-- up") {
		out = append(out, Finding{path, Error, "missing Up section marker", "use vorm markers: -- ⬆ Up … and -- ⬇ Down …"})
	}
	if !strings.Contains(body, "⬇ Down") && !strings.Contains(lower, "-- down") {
		out = append(out, Finding{path, Error, "missing Down section marker", "add a reversible -- ⬇ Down section (DROP TABLE / DROP TYPE / DROP INDEX)"})
	}
	if strings.Contains(lower, "select *") {
		out = append(out, Finding{path, Warning, "SELECT * found", "list columns explicitly for performant, stable queries"})
	}
	if strings.Contains(lower, "create table") && !strings.Contains(lower, "drop table") {
		out = append(out, Finding{path, Warning, "CREATE TABLE without DROP TABLE in file", "Down should DROP TABLE IF EXISTS … (CASCADE on postgres)"})
	}
	if strings.Contains(lower, "create type") && strings.Contains(lower, " as enum") && !strings.Contains(lower, "drop type") {
		out = append(out, Finding{path, Error, "CREATE TYPE ENUM without DROP TYPE", "Down must DROP TYPE IF EXISTS <name> CASCADE"})
	}
	if strings.Contains(lower, "create extension") && !strings.Contains(lower, "drop extension") {
		out = append(out, Finding{path, Warning, "CREATE EXTENSION without DROP EXTENSION", "Down: DROP EXTENSION IF EXISTS … CASCADE"})
	}
	if strings.Contains(lower, "create index") && !strings.Contains(lower, "drop index") && strings.Contains(lower, "create table") {
		out = append(out, Finding{path, Hint, "indexes created; ensure Down drops them (or rely on DROP TABLE CASCADE)", "explicit DROP INDEX IF EXISTS is clearer on rollback"})
	}
	if strings.Contains(name, "create_") && strings.Contains(name, "_table") && !strings.Contains(lower, "create table") {
		out = append(out, Finding{path, Error, "filename looks like create_*_table but no CREATE TABLE", "re-run: vorm make migration <name>"})
	}
	if strings.Contains(lower, "references ") && !strings.Contains(lower, "on delete") {
		out = append(out, Finding{path, Hint, "FK without ON DELETE clause", "consider ON DELETE CASCADE or RESTRICT explicitly"})
	}
	if strings.Contains(lower, "deleted_at") && !strings.Contains(lower, "index") {
		out = append(out, Finding{path, Hint, "soft-delete column without index mention", "add INDEX on deleted_at for query performance"})
	}

	// Empty up/down bodies
	up, down := splitUpDown(body)
	if strings.TrimSpace(stripSQLComments(up)) == "" {
		out = append(out, Finding{path, Error, "Up section is empty", "add CREATE TABLE / ALTER / TYPE SQL, or delete the file"})
	}
	if strings.TrimSpace(stripSQLComments(down)) == "" {
		out = append(out, Finding{path, Warning, "Down section is empty", "add DROP statements so vorm rollback works"})
	}

	return out
}

// GoFile lints a Blueprint migration by compiling Up/Down to SQL.
func GoFile(path, body string) []Finding {
	var out []Finding
	if !strings.Contains(body, "func Up") {
		out = append(out, Finding{path, Error, "missing func Up", "add func Up(s *schema.Facade) { s.Create(...) }"})
		return out
	}
	if !strings.Contains(body, "func Down") {
		out = append(out, Finding{path, Warning, "missing func Down", "add func Down(s *schema.Facade) { s.DropIfExists(...) }"})
	}
	if _, err := schema.CompileSource(path, body, "postgres"); err != nil {
		out = append(out, Finding{
			File:       path,
			Severity:   Error,
			Message:    err.Error(),
			Suggestion: "use Blueprint methods vorm compiles (ID, String, ForeignId, BelongsTo, BelongsToMany, Morphs, MorphToMany, …)",
		})
	}
	return out
}

func splitUpDown(body string) (up, down string) {
	// Prefer vorzela markers
	if i := strings.Index(body, "⬆ Up"); i >= 0 {
		rest := body[i:]
		if j := strings.Index(rest, "⬇ Down"); j >= 0 {
			return rest[:j], rest[j:]
		}
		return rest, ""
	}
	lower := body
	// fallback: split on -- Down
	idx := strings.Index(strings.ToLower(lower), "-- down")
	if idx < 0 {
		return body, ""
	}
	return body[:idx], body[idx:]
}

func stripSQLComments(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "--") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// Format prints findings for CLI.
func Format(r *Result) string {
	if len(r.Findings) == 0 {
		return "vorm lint: ok\n"
	}
	var b strings.Builder
	for _, f := range r.Findings {
		loc := f.File
		if loc == "" {
			loc = "."
		}
		fmt.Fprintf(&b, "%s: %s: %s\n", loc, f.Severity, f.Message)
		if f.Suggestion != "" {
			fmt.Fprintf(&b, "  suggestion: %s\n", f.Suggestion)
		}
	}
	return b.String()
}
