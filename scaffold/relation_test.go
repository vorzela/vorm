package scaffold_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vorzela/vorm/scaffold"
	"github.com/vorzela/vorm/schema"
)

func TestMakeBelongsTo(t *testing.T) {
	dir := t.TempDir()
	res, err := scaffold.MakeRelation("belongs-to", []string{"posts", "users"}, scaffold.MigrationDirs{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != "belongs-to" {
		t.Fatalf("kind = %q", res.Kind)
	}
	if !strings.HasSuffix(res.MigrationFile, "_add_user_id_to_posts_table.go") {
		t.Fatalf("filename %s", res.MigrationFile)
	}
	body := readFile(t, res.MigrationFile)
	for _, want := range []string{
		`s.Table("posts"`,
		`t.BelongsTo("user_id", "users")`,
		`t.DropColumn("user_id")`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
	got, err := schema.CompileFile(res.MigrationFile, "postgres")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.UpSQL, "ALTER TABLE posts ADD COLUMN user_id BIGINT NOT NULL REFERENCES users(id)") {
		t.Fatalf("compiled Up:\n%s", got.UpSQL)
	}
}

func TestMakeBelongsToCustomColumn(t *testing.T) {
	dir := t.TempDir()
	res, err := scaffold.MakeRelation("belongs-to", []string{"posts", "users", "author_id"}, scaffold.MigrationDirs{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	body := readFile(t, res.MigrationFile)
	if !strings.Contains(body, `t.BelongsTo("author_id", "users")`) {
		t.Fatal(body)
	}
}

func TestMakeHasOneAddsUnique(t *testing.T) {
	dir := t.TempDir()
	res, err := scaffold.MakeRelation("has-one", []string{"users", "profiles"}, scaffold.MigrationDirs{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	body := readFile(t, res.MigrationFile)
	if !strings.Contains(body, `t.BelongsTo("user_id", "users")`) || !strings.Contains(body, `t.Unique("user_id")`) {
		t.Fatal(body)
	}
	if !strings.Contains(body, `t.DropIndex("uq_profiles_user_id")`) {
		t.Fatal(body)
	}
	got, err := schema.CompileFile(res.MigrationFile, "postgres")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.UpSQL, "ALTER TABLE profiles ADD COLUMN user_id") {
		t.Fatalf("Up:\n%s", got.UpSQL)
	}
	if !strings.Contains(got.UpSQL, "UNIQUE INDEX") && !strings.Contains(got.UpSQL, "CREATE UNIQUE INDEX") {
		t.Fatalf("unique missing:\n%s", got.UpSQL)
	}
}

func TestMakeHasManyPutsFKOnChild(t *testing.T) {
	dir := t.TempDir()
	res, err := scaffold.MakeRelation("has-many", []string{"users", "posts"}, scaffold.MigrationDirs{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readFile(t, res.MigrationFile), `t.BelongsTo("user_id", "users")`) {
		t.Fatal(readFile(t, res.MigrationFile))
	}
}

func TestMakeBelongsToMany(t *testing.T) {
	dir := t.TempDir()
	res, err := scaffold.MakeRelation("belongs-to-many", []string{"posts", "tags"}, scaffold.MigrationDirs{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != "belongs-to-many" {
		t.Fatalf("kind = %q", res.Kind)
	}
	body := readFile(t, res.MigrationFile)
	if !strings.Contains(body, `s.BelongsToMany("posts", "tags")`) {
		t.Fatal(body)
	}
	got, err := schema.CompileFile(res.MigrationFile, "postgres")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.UpSQL, "CREATE TABLE IF NOT EXISTS post_tag") {
		t.Fatalf("Up:\n%s", got.UpSQL)
	}
}

func TestMakeMorphs(t *testing.T) {
	dir := t.TempDir()
	res, err := scaffold.MakeRelation("morphs", []string{"comments", "commentable"}, scaffold.MigrationDirs{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	body := readFile(t, res.MigrationFile)
	if !strings.Contains(body, `t.Morphs("commentable")`) {
		t.Fatal(body)
	}
	got, err := schema.CompileFile(res.MigrationFile, "postgres")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.UpSQL, "commentable_type") || !strings.Contains(got.UpSQL, "commentable_id") {
		t.Fatalf("Up:\n%s", got.UpSQL)
	}
}

func TestMakeMorphToMany(t *testing.T) {
	dir := t.TempDir()
	res, err := scaffold.MakeRelation("morph-to-many", []string{"tags", "taggable"}, scaffold.MigrationDirs{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(res.MigrationFile, "_create_taggables_table.go") {
		t.Fatalf("filename %s", res.MigrationFile)
	}
	body := readFile(t, res.MigrationFile)
	if !strings.Contains(body, `s.MorphToMany("tags", "taggable")`) {
		t.Fatal(body)
	}
	got, err := schema.CompileFile(res.MigrationFile, "postgres")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS taggables",
		"tag_id BIGINT NOT NULL REFERENCES tags(id)",
		"taggable_type VARCHAR(255) NOT NULL",
		"taggable_id BIGINT NOT NULL",
	} {
		if !strings.Contains(got.UpSQL, want) {
			t.Errorf("Up missing %q\n%s", want, got.UpSQL)
		}
	}
}

func TestMakeRelationNormalizesSingularNames(t *testing.T) {
	dir := t.TempDir()
	res, err := scaffold.MakeRelation("belongs-to", []string{"post", "user"}, scaffold.MigrationDirs{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	body := readFile(t, res.MigrationFile)
	if !strings.Contains(body, `s.Table("posts"`) || !strings.Contains(body, `t.BelongsTo("user_id", "users")`) {
		t.Fatal(body)
	}
}

func TestMakeRelationRequiresArgs(t *testing.T) {
	_, err := scaffold.MakeRelation("belongs-to", []string{"posts"}, scaffold.MigrationDirs{Dir: t.TempDir()})
	if err == nil {
		t.Fatal("expected usage error")
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = filepath.Base(path)
	return string(b)
}
