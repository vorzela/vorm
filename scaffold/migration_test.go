package scaffold_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vorzela/vorm/scaffold"
	"github.com/vorzela/vorm/schema"
)

func TestMakeMigrationPosts(t *testing.T) {
	dir := t.TempDir()
	res, err := scaffold.MakeMigration("posts", scaffold.MigrationDirs{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != "create" {
		t.Fatalf("kind = %q", res.Kind)
	}
	if filepath.Dir(res.MigrationFile) != dir {
		t.Fatalf("wrote %s", res.MigrationFile)
	}
	if !strings.HasSuffix(res.MigrationFile, "_create_posts_table.go") {
		t.Fatalf("filename %s", res.MigrationFile)
	}
	body, _ := os.ReadFile(res.MigrationFile)
	for _, want := range []string{
		"//go:build ignore",
		"func Up(s *schema.Facade)",
		`s.Create("posts"`,
		"t.ID()",
		"t.Timestamps()",
		"t.SoftDeletes()",
		`s.DropIfExists("posts")`,
	} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
	got, err := schema.CompileFile(res.MigrationFile, "postgres")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.UpSQL, "CREATE TABLE IF NOT EXISTS posts") {
		t.Fatalf("compiled Up:\n%s", got.UpSQL)
	}
}

func TestMakeMigrationPivot(t *testing.T) {
	dir := t.TempDir()
	res, err := scaffold.MakeMigration("post_tag", scaffold.MigrationDirs{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != "pivot" {
		t.Fatalf("kind = %q", res.Kind)
	}
	body, _ := os.ReadFile(res.MigrationFile)
	if !strings.Contains(string(body), `s.BelongsToMany("posts", "tags")`) {
		t.Fatal(string(body))
	}
	got, err := schema.CompileFile(res.MigrationFile, "postgres")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.UpSQL, "CREATE TABLE IF NOT EXISTS post_tag") {
		t.Fatalf("compiled Up:\n%s", got.UpSQL)
	}
}

func TestMakeMigrationAlter(t *testing.T) {
	dir := t.TempDir()
	res, err := scaffold.MakeMigration("add_slug_to_posts", scaffold.MigrationDirs{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != "alter" {
		t.Fatalf("kind = %q", res.Kind)
	}
	if res.Column != "slug" || res.Table != "posts" {
		t.Fatalf("table=%q column=%q", res.Table, res.Column)
	}
	body, _ := os.ReadFile(res.MigrationFile)
	for _, want := range []string{
		`s.Table("posts"`,
		`t.String("slug")`,
		`t.DropColumn("slug")`,
	} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
	got, err := schema.CompileFile(res.MigrationFile, "postgres")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.UpSQL, "ADD COLUMN") || !strings.Contains(got.UpSQL, "slug") {
		t.Fatalf("compiled Up:\n%s", got.UpSQL)
	}
}

func TestMakeMigrationAddDisplayName(t *testing.T) {
	dir := t.TempDir()
	res, err := scaffold.MakeMigration("add_display_name_to_users", scaffold.MigrationDirs{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Table != "users" || res.Column != "display_name" {
		t.Fatalf("table=%q column=%q", res.Table, res.Column)
	}
	body, _ := os.ReadFile(res.MigrationFile)
	if !strings.Contains(string(body), `t.String("display_name")`) {
		t.Fatal(string(body))
	}
}

func TestMakeMigrationDropColumn(t *testing.T) {
	dir := t.TempDir()
	res, err := scaffold.MakeMigration("drop_slug_from_posts", scaffold.MigrationDirs{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Column != "slug" || res.Table != "posts" {
		t.Fatalf("table=%q column=%q", res.Table, res.Column)
	}
	body, _ := os.ReadFile(res.MigrationFile)
	// Up drops; Down re-adds
	upIdx := strings.Index(string(body), "func Up")
	downIdx := strings.Index(string(body), "func Down")
	if upIdx < 0 || downIdx < 0 || downIdx < upIdx {
		t.Fatal(string(body))
	}
	up := string(body)[upIdx:downIdx]
	down := string(body)[downIdx:]
	if !strings.Contains(up, `t.DropColumn("slug")`) {
		t.Fatalf("Up should drop:\n%s", up)
	}
	if !strings.Contains(down, `t.String("slug")`) {
		t.Fatalf("Down should restore:\n%s", down)
	}
}

func TestMakeMigrationAlterTableNoColumn(t *testing.T) {
	dir := t.TempDir()
	res, err := scaffold.MakeMigration("alter_posts", scaffold.MigrationDirs{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Column != "" || res.Table != "posts" {
		t.Fatalf("table=%q column=%q", res.Table, res.Column)
	}
	body, _ := os.ReadFile(res.MigrationFile)
	if !strings.Contains(string(body), `// t.String("column")`) {
		t.Fatal(string(body))
	}
}

