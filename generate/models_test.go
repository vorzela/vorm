package generate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vorzela/vorm/generate"
)

func TestGenerateModelsFromBlueprint(t *testing.T) {
	root := t.TempDir()
	schemaDir := filepath.Join(root, "schema")
	modelDir := filepath.Join(root, "models")
	os.MkdirAll(schemaDir, 0o755)
	src := `package main
import "github.com/vorzela/vorm/schema"
func CreatePostsTable(s *schema.Facade) error {
	return s.Create("posts", func(t *schema.Blueprint) {
		t.ID()
		t.String("title")
		t.Enum("status", "draft", "published")
		t.Integer("age").Nullable()
		t.ForeignId("user_id").Constrained("users").CascadeOnDelete()
		t.Timestamps()
		t.SoftDeletes()
	})
}
`
	os.WriteFile(filepath.Join(schemaDir, "create_posts.go"), []byte(src), 0o644)
	res, err := generate.Models(generate.ModelOptions{SchemaDir: schemaDir, ModelDir: modelDir})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Files) != 1 {
		t.Fatalf("%+v", res)
	}
	body, _ := os.ReadFile(res.Files[0])
	s := string(body)
	for _, want := range []string{
		"type Post struct",
		"Title string",
		"type PostsStatus string",
		"PostsStatusDraft",
		"UserId int64",
		`SoftDeletes: true`,
		`"title", "status", "age", "user_id"`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in:\n%s", want, s)
		}
	}
}

func TestGenerateModelsBlueprintJSONImport(t *testing.T) {
	root := t.TempDir()
	schemaDir := filepath.Join(root, "schema")
	modelDir := filepath.Join(root, "models")
	os.MkdirAll(schemaDir, 0o755)
	src := `package main
import "github.com/vorzela/vorm/schema"
func CreateItemsTable(s *schema.Facade) error {
	return s.Create("items", func(t *schema.Blueprint) {
		t.ID()
		t.CustomType("jsonb").Column("payload")
		t.Timestamps()
	})
}
`
	os.WriteFile(filepath.Join(schemaDir, "create_items.go"), []byte(src), 0o644)
	res, err := generate.Models(generate.ModelOptions{SchemaDir: schemaDir, ModelDir: modelDir})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(res.Files[0])
	s := string(body)
	if !strings.Contains(s, `"encoding/json"`) {
		t.Fatalf("missing encoding/json import:\n%s", s)
	}
	if !strings.Contains(s, "json.RawMessage") {
		t.Fatalf("missing json.RawMessage field:\n%s", s)
	}
}

func TestGenerateModelsBlueprintColumnHelpers(t *testing.T) {
	root := t.TempDir()
	schemaDir := filepath.Join(root, "schema")
	modelDir := filepath.Join(root, "models")
	os.MkdirAll(schemaDir, 0o755)
	src := `package main
import "github.com/vorzela/vorm/schema"
func CreateWidgetsTable(s *schema.Facade) error {
	return s.Create("widgets", func(t *schema.Blueprint) {
		t.ID()
		t.Jsonb("payload")
		t.TimestampTz("event_at")
		t.Decimal("price", 10, 2)
		t.Float("score")
		t.SmallInteger("rank")
		t.Binary("raw")
		t.Inet("ip")
		t.TimestampsTz()
	})
}
`
	os.WriteFile(filepath.Join(schemaDir, "create_widgets.go"), []byte(src), 0o644)
	res, err := generate.Models(generate.ModelOptions{SchemaDir: schemaDir, ModelDir: modelDir})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(res.Files[0])
	s := string(body)
	for _, want := range []string{
		`"encoding/json"`,
		`"time"`,
		"Payload json.RawMessage",
		"EventAt time.Time",
		"Price string",
		"Score float32",
		"Rank int16",
		"Raw []byte",
		"Ip string",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in:\n%s", want, s)
		}
	}
}
