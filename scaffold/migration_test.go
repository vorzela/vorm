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
	body, _ := os.ReadFile(res.MigrationFile)
	if !strings.Contains(string(body), `s.Table("posts"`) {
		t.Fatal(string(body))
	}
}
