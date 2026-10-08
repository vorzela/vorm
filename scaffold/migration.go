package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MigrationDirs holds output locations for make migration.
type MigrationDirs struct {
	Dir     string // default ./migrations
	Dialect string
}

func (d *MigrationDirs) defaults() {
	if d.Dir == "" {
		d.Dir = "./migrations"
	}
	if d.Dialect == "" {
		d.Dialect = "postgres"
	}
}

// MakeResult lists files created by MakeMigration.
type MakeResult struct {
	Table         string
	Column        string // set for add_<col>_to_<table> / drop_<col>_from_<table>
	MigrationFile string
	Kind          string // create | pivot | alter
}

// MakeMigration scaffolds a numbered Blueprint migration:
//
//	vorm make migration posts
//	vorm make migration post_tag
//	vorm make migration add_slug_to_posts
//	vorm make migration drop_slug_from_posts
func MakeMigration(rawName string, dirs MigrationDirs) (*MakeResult, error) {
	dirs.defaults()
	table, kind, pivot, column, action := classifyName(rawName)
	if err := os.MkdirAll(dirs.Dir, 0o755); err != nil {
		return nil, err
	}

	var (
		filename string
		body     string
	)
	switch kind {
	case "pivot":
		filename = "create_" + table + "_table.go"
		body = pivotSource(pivot)
	case "alter":
		filename = snakeName(rawName) + ".go"
		switch {
		case action == "add" && column != "":
			body = alterAddSource(table, column)
		case action == "drop" && column != "":
			body = alterDropSource(table, column)
		default:
			body = alterSource(table)
		}
	default:
		filename = "create_" + table + "_table.go"
		body = createSource(table)
	}

	path, err := uniquePath(dirs.Dir, filename)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return nil, err
	}
	return &MakeResult{Table: table, Column: column, MigrationFile: path, Kind: kind}, nil
}

func uniquePath(dir, filename string) (string, error) {
	ts := time.Now().UnixMilli()
	for i := 0; i < 1000; i++ {
		full := filepath.Join(dir, fmt.Sprintf("%d_%s", ts+int64(i), filename))
		if _, err := os.Stat(full); os.IsNotExist(err) {
			return full, nil
		}
	}
	return "", fmt.Errorf("scaffold: could not pick a unique name for %s", filename)
}

func classifyName(raw string) (table, kind string, pivot *pivotHint, column, action string) {
	name := strings.TrimSpace(raw)
	name = strings.TrimSuffix(name, ".go")
	name = strings.TrimPrefix(name, "create_")
	name = strings.TrimSuffix(name, "_table")
	snake := snakeName(name)

	if strings.HasPrefix(snake, "add_") || strings.HasPrefix(snake, "alter_") || strings.HasPrefix(snake, "drop_") {
		table, column, action = parseAlter(snake)
		return table, "alter", nil, column, action
	}

	parts := strings.Split(snake, "_")
	if len(parts) == 2 && !strings.HasSuffix(parts[0], "s") && !strings.HasSuffix(parts[1], "s") {
		left, right := parts[0], parts[1]
		return snake, "pivot", &pivotHint{
			LeftTable:  pluralize(left),
			RightTable: pluralize(right),
		}, "", ""
	}
	return snake, "create", nil, "", ""
}

// parseAlter extracts table/column/action from alter-style names.
//
//	add_slug_to_posts          → add, slug, posts
//	add_display_name_to_users  → add, display_name, users
//	drop_slug_from_posts       → drop, slug, posts
//	drop_slug_to_posts         → drop, slug, posts (symmetry)
//	alter_posts / add_posts    → empty column (generic alter template)
func parseAlter(name string) (table, column, action string) {
	switch {
	case strings.HasPrefix(name, "add_"):
		action = "add"
		rest := strings.TrimPrefix(name, "add_")
		if i := strings.LastIndex(rest, "_to_"); i >= 0 {
			column = rest[:i]
			table = rest[i+4:]
			return table, column, action
		}
		table = strings.TrimSuffix(rest, "_table")
		return table, "", action
	case strings.HasPrefix(name, "drop_"):
		action = "drop"
		rest := strings.TrimPrefix(name, "drop_")
		if i := strings.LastIndex(rest, "_from_"); i >= 0 {
			column = rest[:i]
			table = rest[i+6:]
			return table, column, action
		}
		if i := strings.LastIndex(rest, "_to_"); i >= 0 {
			column = rest[:i]
			table = rest[i+4:]
			return table, column, action
		}
		table = strings.TrimSuffix(rest, "_table")
		return table, "", action
	default: // alter_
		action = "alter"
		rest := strings.TrimPrefix(name, "alter_")
		if i := strings.LastIndex(rest, "_to_"); i >= 0 {
			// alter_slug_to_posts treated like add
			column = rest[:i]
			table = rest[i+4:]
			action = "add"
			return table, column, action
		}
		table = strings.TrimSuffix(rest, "_table")
		return table, "", action
	}
}

type pivotHint struct {
	LeftTable, RightTable string
	LeftRef, RightRef     string // optional referenced columns (default id)
}

func createSource(table string) string {
	return fmt.Sprintf(`//go:build ignore

package main

import "github.com/vorzela/vorm/schema"

func Up(s *schema.Facade) {
	s.Create(%q, func(t *schema.Blueprint) {
		t.ID()
		// t.String("title")
		// t.Text("body")
		// t.Enum("status", "draft", "published")
		// t.ForeignId("user_id").Constrained("users").CascadeOnDelete()
		// t.Boolean("active").Default(true)
		t.Timestamps()
		t.SoftDeletes()
	})
}

func Down(s *schema.Facade) {
	s.DropIfExists(%q)
}
`, table, table)
}

func pivotSource(p *pivotHint) string {
	call := fmt.Sprintf("%q, %q", p.LeftTable, p.RightTable)
	if p.LeftRef != "" || p.RightRef != "" {
		leftRef, rightRef := p.LeftRef, p.RightRef
		if leftRef == "" {
			leftRef = "id"
		}
		if rightRef == "" {
			rightRef = "id"
		}
		call = fmt.Sprintf("%q, %q, %q, %q", p.LeftTable, p.RightTable, leftRef, rightRef)
	}
	return fmt.Sprintf(`//go:build ignore

package main

import "github.com/vorzela/vorm/schema"

func Up(s *schema.Facade) {
	s.BelongsToMany(%s)
}

func Down(s *schema.Facade) {
	s.DropIfExists(%q)
}
`, call, schemaPivotName(p.LeftTable, p.RightTable))
}

func alterSource(table string) string {
	if table == "" {
		table = "table_name"
	}
	return fmt.Sprintf(`//go:build ignore

package main

import "github.com/vorzela/vorm/schema"

func Up(s *schema.Facade) {
	s.Table(%q, func(t *schema.Blueprint) {
		// t.String("column")
	})
}

func Down(s *schema.Facade) {
	s.Table(%q, func(t *schema.Blueprint) {
		// t.DropColumn("column")
	})
}
`, table, table)
}

func alterAddSource(table, column string) string {
	if table == "" {
		table = "table_name"
	}
	return fmt.Sprintf(`//go:build ignore

package main

import "github.com/vorzela/vorm/schema"

func Up(s *schema.Facade) {
	s.Table(%q, func(t *schema.Blueprint) {
		t.String(%q) // change to Text / Integer / … as needed
	})
}

func Down(s *schema.Facade) {
	s.Table(%q, func(t *schema.Blueprint) {
		t.DropColumn(%q)
	})
}
`, table, column, table, column)
}

func alterDropSource(table, column string) string {
	if table == "" {
		table = "table_name"
	}
	return fmt.Sprintf(`//go:build ignore

package main

import "github.com/vorzela/vorm/schema"

func Up(s *schema.Facade) {
	s.Table(%q, func(t *schema.Blueprint) {
		t.DropColumn(%q)
	})
}

func Down(s *schema.Facade) {
	s.Table(%q, func(t *schema.Blueprint) {
		t.String(%q) // restore type to match what was dropped
	})
}
`, table, column, table, column)
}

func schemaPivotName(left, right string) string {
	a, b := singular(left), singular(right)
	if a > b {
		a, b = b, a
	}
	return a + "_" + b
}

func snakeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, " ", "_")
	return strings.ToLower(name)
}

func singular(table string) string {
	if strings.HasSuffix(table, "ies") && len(table) > 3 {
		return table[:len(table)-3] + "y"
	}
	if strings.HasSuffix(table, "ses") || strings.HasSuffix(table, "xes") || strings.HasSuffix(table, "zes") {
		return table[:len(table)-2]
	}
	if strings.HasSuffix(table, "s") && len(table) > 1 {
		return table[:len(table)-1]
	}
	return table
}

func pluralize(word string) string {
	if strings.HasSuffix(word, "y") && len(word) > 1 {
		return word[:len(word)-1] + "ies"
	}
	if strings.HasSuffix(word, "s") {
		return word
	}
	return word + "s"
}

// GoMigration is kept for callers that only want the schema file.
func GoMigration(dir, name string) (string, error) {
	res, err := MakeMigration(name, MigrationDirs{Dir: dir})
	if err != nil {
		return "", err
	}
	return res.MigrationFile, nil
}
