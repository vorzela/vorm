package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vorzela/vorm/config"
)

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	body := "# comment\nDATABASE_URL=postgres://from-dotenv/db\nexport DRIVER=pgx\nQUOTED=\"hello world\"\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("DRIVER", "already-set")
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("QUOTED")

	if err := config.LoadDotEnv(dir); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("DATABASE_URL"); got != "postgres://from-dotenv/db" {
		t.Fatalf("DATABASE_URL=%q", got)
	}
	if got := os.Getenv("DRIVER"); got != "already-set" {
		t.Fatalf("DRIVER should not be overwritten, got %q", got)
	}
	if got := os.Getenv("QUOTED"); got != "hello world" {
		t.Fatalf("QUOTED=%q", got)
	}
}

func TestLoadDotEnvMissingOK(t *testing.T) {
	if err := config.LoadDotEnv(t.TempDir()); err != nil {
		t.Fatal(err)
	}
}
