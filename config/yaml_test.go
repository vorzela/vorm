package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vorzela/vorm/config"
)

func TestEmitSQLAsCommentFromVorm(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".vorm"), []byte("EMIT_SQL_AS_COMMENT=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !c.EmitSQLAsComment {
		t.Fatal("expected EMIT_SQL_AS_COMMENT=true")
	}
}

func TestEmitSQLAsCommentFromYAML(t *testing.T) {
	dir := t.TempDir()
	body := "version: \"1\"\ngen:\n  go:\n    emit_sql_as_comment: true\n"
	if err := os.WriteFile(filepath.Join(dir, "vorm.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !c.EmitSQLAsComment {
		t.Fatal("expected gen.go.emit_sql_as_comment from vorm.yaml")
	}
}

func TestYAMLPreferredOverLegacy(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".vorm"), []byte("PACKAGE=fromvorm\nEMIT_SQL_AS_COMMENT=false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "vorm.yml"), []byte("package: fromyaml\nemit_sql_as_comment: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.Package != "fromyaml" {
		t.Fatalf("package=%q, want fromyaml (YAML preferred)", c.Package)
	}
	if !c.EmitSQLAsComment {
		t.Fatal("vorm.yml should win over .vorm")
	}
}

func TestYAMLWriteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vorm.yaml")
	c := config.Default()
	c.Package = "vormgen"
	c.EmitSQLAsComment = true
	if err := c.Write(path); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Package != "vormgen" || !got.EmitSQLAsComment {
		t.Fatalf("%+v", got)
	}
}
