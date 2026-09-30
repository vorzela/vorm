package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// YAMLFileNames are optional overlays next to .vorm (sqlc-style keys).
var YAMLFileNames = []string{"vorm.yaml", "vorm.yml"}

// yamlFile mirrors the sqlc.yaml gen.go shape for emit options.
type yamlFile struct {
	Version          string `yaml:"version"`
	EmitSQLAsComment *bool  `yaml:"emit_sql_as_comment"`
	Gen              *struct {
		Go *struct {
			EmitSQLAsComment *bool `yaml:"emit_sql_as_comment"`
		} `yaml:"go"`
	} `yaml:"gen"`
}

// applyYAML overlays vorm.yaml / vorm.yml onto cfg when present.
// Nested gen.go.emit_sql_as_comment wins over a top-level key, matching sqlc.
func applyYAML(cfg *Config, dir string) error {
	path, body, err := readYAML(dir)
	if err != nil || body == nil {
		return err
	}
	var y yamlFile
	if err := yaml.Unmarshal(body, &y); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if y.EmitSQLAsComment != nil {
		cfg.EmitSQLAsComment = *y.EmitSQLAsComment
	}
	if y.Gen != nil && y.Gen.Go != nil && y.Gen.Go.EmitSQLAsComment != nil {
		cfg.EmitSQLAsComment = *y.Gen.Go.EmitSQLAsComment
	}
	return nil
}

func readYAML(dir string) (string, []byte, error) {
	for _, name := range YAMLFileNames {
		path := filepath.Join(dir, name)
		body, err := os.ReadFile(path)
		if err == nil {
			return path, body, nil
		}
		if !os.IsNotExist(err) {
			return path, nil, err
		}
	}
	return "", nil, nil
}
