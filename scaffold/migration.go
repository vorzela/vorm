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
	MigrationFile string
	Kind          string // create | pivot | alter
}

// MakeMigration scaffolds a numbered Blueprint migration:
//
//	vorm make migration posts
//	vorm make migration post_tag
//	vorm make migration add_slug_to_posts
func MakeMigration(rawName string, dirs MigrationDirs) (*MakeResult, error) {
	dirs.defaults()
	table, kind, pivot := classifyName(rawName)
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
		body = alterSource(table)
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
	return &MakeResult{Table: table, MigrationFile: path, Kind: kind}, nil
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

func classifyName(raw string) (table, kind string, pivot *pivotHint) {
	name := strings.TrimSpace(raw)
	name = strings.TrimSuffix(name, ".go")
	name = strings.TrimPrefix(name, "create_")
	name = strings.TrimSuffix(name, "_table")
	snake := snakeName(name)

	if strings.HasPrefix(snake, "add_") || strings.HasPrefix(snake, "alter_") || strings.HasPrefix(snake, "drop_") {
		table = tableFromAlter(snake)
		return table, "alter", nil
	}

	parts := strings.Split(snake, "_")
	if len(parts) == 2 && !strings.HasSuffix(parts[0], "s") && !strings.HasSuffix(parts[1], "s") {
		left, right := parts[0], parts[1]
		return snake, "pivot", &pivotHint{
			LeftTable:  pluralize(left),
			RightTable: pluralize(right),
		}
	}
	return snake, "create", nil
}

func tableFromAlter(name string) string {
	if i := strings.LastIndex(name, "_to_"); i >= 0 {
		return name[i+4:]
	}
	name = strings.TrimPrefix(name, "add_")
	name = strings.TrimPrefix(name, "alter_")
	name = strings.TrimPrefix(name, "drop_")
	name = strings.TrimSuffix(name, "_table")
	return name
}

type pivotHint struct {
	LeftTable, RightTable string
}

func createSource(table string) string {
	return fmt.Sprintf(`//go:build ignore

package migrations

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
	return fmt.Sprintf(`//go:build ignore

package migrations

import "github.com/vorzela/vorm/schema"

func Up(s *schema.Facade) {
	s.BelongsToMany(%q, %q)
}

func Down(s *schema.Facade) {
	s.DropIfExists(%q)
}
`, p.LeftTable, p.RightTable, schemaPivotName(p.LeftTable, p.RightTable))
}

func alterSource(table string) string {
	if table == "" {
		table = "table_name"
	}
	return fmt.Sprintf(`//go:build ignore

package migrations

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
