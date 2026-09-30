package generate

import (
	"strings"
	"testing"

	"github.com/vorzela/vorm/introspect"
	"github.com/vorzela/vorm/query"
)

func TestDiffModelsSchemaMatch(t *testing.T) {
	schema := &introspect.Schema{
		Dialect: query.DialectPostgres,
		Tables: []introspect.Table{{
			Name: "users",
			Columns: []introspect.Column{
				{Name: "id", DBType: "bigint", FullType: "bigint"},
				{Name: "email", DBType: "text", FullType: "text"},
				{Name: "name", DBType: "text", FullType: "text", Nullable: true},
			},
		}},
	}
	models := map[string]ModelSpec{
		"Users": {
			Entity: "Users",
			Table:  "users",
			Columns: []string{"id", "email", "name"},
			Fields: []FieldSpec{
				{Name: "ID", Column: "id", Type: "int64"},
				{Name: "Email", Column: "email", Type: "string"},
				{Name: "Name", Column: "name", Type: "*string"},
			},
		},
	}
	if drifts := DiffModelsSchema(models, schema, query.DialectPostgres); len(drifts) != 0 {
		t.Fatalf("unexpected drifts: %v", drifts)
	}
}

func TestDiffModelsSchemaMissingAndExtra(t *testing.T) {
	schema := &introspect.Schema{
		Dialect: query.DialectPostgres,
		Tables: []introspect.Table{{
			Name: "users",
			Columns: []introspect.Column{
				{Name: "id", DBType: "bigint", FullType: "bigint"},
				{Name: "email", DBType: "text", FullType: "text"},
				{Name: "phone", DBType: "text", FullType: "text"},
			},
		}},
	}
	models := map[string]ModelSpec{
		"Users": {
			Entity:  "Users",
			Table:   "users",
			Columns: []string{"id", "email", "legacy"},
			Fields: []FieldSpec{
				{Name: "ID", Column: "id", Type: "int64"},
				{Name: "Email", Column: "email", Type: "string"},
				{Name: "Legacy", Column: "legacy", Type: "string"},
			},
		},
	}
	drifts := DiffModelsSchema(models, schema, query.DialectPostgres)
	joined := FormatSchemaDrift(drifts)
	if !strings.Contains(joined, "legacy") || !strings.Contains(joined, "phone") {
		t.Fatalf("want legacy+phone findings, got:\n%s", joined)
	}
}

func TestDiffModelsSchemaWrongType(t *testing.T) {
	schema := &introspect.Schema{
		Dialect: query.DialectPostgres,
		Tables: []introspect.Table{{
			Name: "users",
			Columns: []introspect.Column{
				{Name: "id", DBType: "bigint", FullType: "bigint"},
				{Name: "active", DBType: "boolean", FullType: "boolean"},
			},
		}},
	}
	models := map[string]ModelSpec{
		"Users": {
			Entity:  "Users",
			Table:   "users",
			Columns: []string{"id", "active"},
			Fields: []FieldSpec{
				{Name: "ID", Column: "id", Type: "int64"},
				{Name: "Active", Column: "active", Type: "string"},
			},
		},
	}
	drifts := DiffModelsSchema(models, schema, query.DialectPostgres)
	if len(drifts) != 1 || drifts[0].Column != "active" {
		t.Fatalf("got %#v", drifts)
	}
	if !strings.Contains(drifts[0].Message, "bool") {
		t.Fatalf("message = %q", drifts[0].Message)
	}
}

func TestDiffModelsSchemaMissingTable(t *testing.T) {
	schema := &introspect.Schema{Dialect: query.DialectPostgres}
	models := map[string]ModelSpec{
		"Users": {Entity: "Users", Table: "users", Columns: []string{"id"}},
	}
	drifts := DiffModelsSchema(models, schema, query.DialectPostgres)
	if len(drifts) != 1 || drifts[0].Table != "users" || drifts[0].Column != "" {
		t.Fatalf("got %#v", drifts)
	}
}

func TestNormalizeGoType(t *testing.T) {
	cases := map[string]string{
		"*string":            "*string",
		"*models.UserStatus": "*UserStatus",
		"models.UserStatus":  "UserStatus",
		"int64":              "int64",
	}
	for in, want := range cases {
		if got := normalizeGoType(in); got != want {
			t.Errorf("normalizeGoType(%q)=%q want %q", in, got, want)
		}
	}
}
