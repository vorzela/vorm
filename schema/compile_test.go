package schema_test

import (
	"strings"
	"testing"

	"github.com/vorzela/vorm/schema"
)

const createPostsSrc = `//go:build ignore

package migrations

import "github.com/vorzela/vorm/schema"

func Up(s *schema.Facade) {
	s.Create("posts", func(t *schema.Blueprint) {
		t.ID()
		t.String("title", 120).Unique()
		t.Text("body")
		t.Boolean("active").Default(true)
		t.Integer("age").Nullable()
		t.ForeignId("user_id").Constrained("users").CascadeOnDelete()
		t.Enum("status", "draft", "published")
		t.Morphs("commentable")
		t.Timestamps()
		t.SoftDeletes()
	})
}

func Down(s *schema.Facade) {
	s.DropIfExists("posts")
}
`

func TestCompileSourceCreateTable(t *testing.T) {
	got, err := schema.CompileSource("1_create_posts_table.go", createPostsSrc, "postgres")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS posts",
		"title VARCHAR(120) NOT NULL UNIQUE",
		"body TEXT NULL",
		"active BOOLEAN NOT NULL DEFAULT TRUE",
		"age INTEGER NULL",
		"user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE",
		"CREATE TYPE posts_status AS ENUM ('draft', 'published')",
		"commentable_type VARCHAR(255) NOT NULL",
		"commentable_id BIGINT NOT NULL",
		"created_at TIMESTAMPTZ",
		"deleted_at TIMESTAMPTZ NULL",
	} {
		if !strings.Contains(got.UpSQL, want) {
			t.Errorf("Up missing %q\n%s", want, got.UpSQL)
		}
	}
	if !strings.Contains(got.DownSQL, "DROP TABLE IF EXISTS posts CASCADE") {
		t.Errorf("Down missing drop:\n%s", got.DownSQL)
	}
}

func TestCompileSourceBelongsToMany(t *testing.T) {
	src := `package migrations
import "github.com/vorzela/vorm/schema"
func Up(s *schema.Facade) { s.BelongsToMany("posts", "tags") }
func Down(s *schema.Facade) { s.DropIfExists("post_tag") }
`
	got, err := schema.CompileSource("2_post_tag.go", src, "postgres")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS post_tag",
		"post_id BIGINT NOT NULL REFERENCES posts(id) ON DELETE CASCADE",
		"tag_id BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE",
	} {
		if !strings.Contains(got.UpSQL, want) {
			t.Errorf("Up missing %q\n%s", want, got.UpSQL)
		}
	}
}

func TestCompileSourceAlterTable(t *testing.T) {
	src := `package migrations
import "github.com/vorzela/vorm/schema"
func Up(s *schema.Facade) {
	s.Table("posts", func(t *schema.Blueprint) {
		t.String("slug").Unique()
	})
}
func Down(s *schema.Facade) {
	s.Table("posts", func(t *schema.Blueprint) {
		t.DropColumn("slug")
	})
}
`
	got, err := schema.CompileSource("3_add_slug.go", src, "postgres")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.UpSQL, "ALTER TABLE posts ADD COLUMN slug VARCHAR(255) NOT NULL UNIQUE") {
		t.Errorf("Up:\n%s", got.UpSQL)
	}
	if !strings.Contains(got.DownSQL, "ALTER TABLE posts DROP COLUMN IF EXISTS slug") {
		t.Errorf("Down:\n%s", got.DownSQL)
	}
}

func TestCompileSourceUnknownMethod(t *testing.T) {
	src := `package migrations
import "github.com/vorzela/vorm/schema"
func Up(s *schema.Facade) {
	s.Create("x", func(t *schema.Blueprint) { t.Nope("x") })
}
`
	_, err := schema.CompileSource("bad.go", src, "postgres")
	if err == nil || !strings.Contains(err.Error(), "unknown Blueprint method Nope") {
		t.Fatalf("want unknown method error, got %v", err)
	}
}

func TestCompileSourceMissingUp(t *testing.T) {
	src := `package migrations
func Down(s *schema.Facade) {}
`
	_, err := schema.CompileSource("no_up.go", src, "postgres")
	if err == nil || !strings.Contains(err.Error(), "missing func Up") {
		t.Fatalf("want missing Up, got %v", err)
	}
}

func TestCompileSourceDoesNotExecuteFacade(t *testing.T) {
	src := `package migrations
import "github.com/vorzela/vorm/schema"
func Up(s *schema.Facade) {
	s.Create("only_in_memory", func(t *schema.Blueprint) { t.ID() })
}
`
	if _, err := schema.CompileSource("mem.go", src, "postgres"); err != nil {
		t.Fatal(err)
	}
}

func TestPivotName(t *testing.T) {
	if got := schema.PivotName("posts", "tags"); got != "post_tag" {
		t.Fatalf("PivotName = %q", got)
	}
	if got := schema.PivotName("tags", "posts"); got != "post_tag" {
		t.Fatalf("order-independent PivotName = %q", got)
	}
}
