package schema

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vorzela/vorm/migrate"
	"github.com/vorzela/vorm/query"
)

// Facade mirrors Laravel's Schema::…
//
//	err := vorm.Schema.Create("users", func(t *Blueprint) {
//	    t.ID()
//	    t.String("email").Unique()
//	    t.Timestamps()
//	})
//
// By default AutoMigrate is true: the SQL is written, then applied in-process.
type Facade struct {
	MigrationPath string
	Dialect       string
	AutoMigrate   bool

	// DatabaseURL overrides the DATABASE_URL environment variable.
	DatabaseURL string
}

// Default is the package-level Schema facade (Laravel Schema::).
var Default = &Facade{
	MigrationPath: "./migrations",
	AutoMigrate:   true,
}

// Result describes a written migration.
type Result struct {
	Path  string
	Table string
	Name  string
}

// Create builds a Laravel-style create-table migration and applies it (unless AutoMigrate is false).
func (f *Facade) Create(table string, build func(*Blueprint)) error {
	_, err := f.CreateResult(table, build)
	return err
}

// CreateResult is Create with the written file path returned.
func (f *Facade) CreateResult(table string, build func(*Blueprint)) (*Result, error) {
	if table == "" {
		return nil, &Error{Op: "create", Hint: "table name is required"}
	}
	if build == nil {
		return nil, &Error{Op: "create", Table: table, Hint: "Blueprint callback is required"}
	}
	bp := NewBlueprint(table)
	build(bp)
	if err := ValidateBlueprint(bp); err != nil {
		return nil, err
	}
	sqlUp, sqlDown := bp.Compile(f.resolveDialect())
	name := "create_" + table + "_table"
	path, err := f.writeMigration(name, sqlUp, sqlDown)
	if err != nil {
		return nil, wrap("create", table, "", err, "could not write migration file")
	}
	fmt.Fprintf(os.Stderr, "vorm: wrote %s\n", path)
	if err := f.maybeMigrate(); err != nil {
		return &Result{Path: path, Table: table, Name: name}, wrap("migrate", table, path, err,
			"SQL was written; fix the DB/lock issue then run: vorm migrate")
	}
	return &Result{Path: path, Table: table, Name: name}, nil
}

// Table alters an existing table.
func (f *Facade) Table(table string, build func(*Blueprint)) error {
	if table == "" || build == nil {
		return &Error{Op: "table", Table: table, Hint: "table name and Blueprint callback required"}
	}
	bp := NewBlueprint(table)
	bp.alter = true
	build(bp)
	if err := ValidateBlueprint(bp); err != nil {
		return err
	}
	sqlUp, sqlDown := bp.Compile(f.resolveDialect())
	name := "alter_" + table + "_table"
	path, err := f.writeMigration(name, sqlUp, sqlDown)
	if err != nil {
		return wrap("table", table, "", err, "could not write migration file")
	}
	fmt.Fprintf(os.Stderr, "vorm: wrote %s\n", path)
	return wrap("migrate", table, path, f.maybeMigrate(), "SQL written; run vorm migrate if needed")
}

// DropIfExists writes a drop-table migration.
func (f *Facade) DropIfExists(table string) error {
	d := f.resolveDialect()
	var up, down string
	if d == "mysql" || d == "mariadb" {
		up = fmt.Sprintf("DROP TABLE IF EXISTS %s;", table)
		down = fmt.Sprintf("-- restore %s manually", table)
	} else {
		up = fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE;", table)
		down = fmt.Sprintf("-- restore %s manually", table)
	}
	name := "drop_" + table + "_table"
	path, err := f.writeMigration(name, up, down)
	if err != nil {
		return wrap("drop", table, "", err, "")
	}
	fmt.Fprintf(os.Stderr, "vorm: wrote %s\n", path)
	return wrap("migrate", table, path, f.maybeMigrate(), "")
}

// Migrate applies pending migrations once (use after several Creates with
// AutoMigrate=false).
func (f *Facade) Migrate() error {
	return wrap("migrate", "", "", f.applyMigrations(context.Background()),
		"check DATABASE_URL, locks, and vorm lint")
}

func (f *Facade) applyMigrations(ctx context.Context) error {
	url := f.databaseURL()
	if url == "" {
		return fmt.Errorf("DATABASE_URL is not set (or set Facade.DatabaseURL)")
	}
	conn, err := query.Open(ctx, url)
	if err != nil {
		return err
	}
	defer conn.Close()

	dir := f.MigrationPath
	if dir == "" {
		dir = migrate.DefaultDir
	}
	opts := migrate.DefaultOptions()
	opts.Dir = dir
	opts.Dialect = migrate.DetectDialect(url)
	opts.RunPrereq = opts.Dialect == query.DialectPostgres

	report, err := migrate.New(conn, opts).Up(ctx)
	if report != nil {
		for _, s := range report.Steps {
			if s.Applied {
				fmt.Fprintf(os.Stderr, "vorm: migrated %s\n", s.Name)
			}
		}
	}
	return err
}

// Batch disables AutoMigrate for fn, then runs a single Migrate.
//
//	err := vorm.Schema.Batch(func(s *schema.Facade) error {
//	    if err := s.Create("users", ...); err != nil { return err }
//	    return s.Create("posts", ...)
//	})
func (f *Facade) Batch(fn func(*Facade) error) error {
	prev := f.AutoMigrate
	f.AutoMigrate = false
	defer func() { f.AutoMigrate = prev }()
	if err := fn(f); err != nil {
		return err
	}
	return f.Migrate()
}

func (f *Facade) maybeMigrate() error {
	if !f.AutoMigrate {
		return nil
	}
	return f.Migrate()
}

func (f *Facade) resolveDialect() string {
	if f.Dialect != "" {
		return strings.ToLower(f.Dialect)
	}
	if url := f.databaseURL(); url != "" {
		return string(query.DetectDialect(url))
	}
	return "postgres"
}

func (f *Facade) databaseURL() string {
	if f.DatabaseURL != "" {
		return f.DatabaseURL
	}
	return os.Getenv("DATABASE_URL")
}

func (f *Facade) writeMigration(name, up, down string) (string, error) {
	dir := f.MigrationPath
	if dir == "" {
		dir = "./migrations"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// Second-resolution timestamps keep files in chronological order. Bump on
	// collision because a Batch can write several migrations within the same second.
	ts := time.Now().Unix()
	var full string
	for {
		full = filepath.Join(dir, fmt.Sprintf("%d_%s.sql", ts, name))
		if _, err := os.Stat(full); os.IsNotExist(err) {
			break
		}
		ts++
	}
	body := fmt.Sprintf(`-- Migration: %s
-- Generated by vorm Schema.Create / Blueprint

-- ⬆ Up (Run when migrating forward)
%s

-- ⬇ Down (Run when rolling back)
%s
`, strings.ToUpper(name), strings.TrimSpace(up), strings.TrimSpace(down))
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		return "", err
	}
	return full, nil
}
