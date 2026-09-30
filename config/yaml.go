package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// YAMLFileNames are the preferred project config files (sqlc-style).
// Checked in order; the first that exists wins.
var YAMLFileNames = []string{"vorm.yaml", "vorm.yml"}

// LegacyFile is the older KEY=value config; still loaded when no YAML is present.
const LegacyFile = ".vorm"

// DefaultFile is what `vorm init` and `vorm config set` write by default.
const DefaultFile = "vorm.yaml"

// yamlDoc is the on-disk shape for vorm.yaml / vorm.yml.
type yamlDoc struct {
	Version          string `yaml:"version,omitempty"`
	Package          string `yaml:"package,omitempty"`
	OutDir           string `yaml:"out_dir,omitempty"`
	QueryDir         string `yaml:"query_dir,omitempty"`
	ModelDir         string `yaml:"model_dir,omitempty"`
	SchemaDir        string `yaml:"schema_dir,omitempty"`
	ModelPackage     string `yaml:"model_package,omitempty"`
	ModelImport      string `yaml:"model_import,omitempty"`
	Driver           string `yaml:"driver,omitempty"`
	Dialect          string `yaml:"dialect,omitempty"`
	DatabaseURL      string `yaml:"database_url,omitempty"`
	MigrationPath    string `yaml:"migration_path,omitempty"`
	ModelSource      string `yaml:"model_source,omitempty"`
	SchemaName       string `yaml:"schema_name,omitempty"`
	EmitRelations    *bool  `yaml:"emit_relations,omitempty"`
	EmitFunctions    *bool  `yaml:"emit_functions,omitempty"`
	EmitSQLAsComment *bool  `yaml:"emit_sql_as_comment,omitempty"`
	IncludeViews     *bool  `yaml:"include_views,omitempty"`
	Gen              *struct {
		Go *struct {
			EmitSQLAsComment *bool `yaml:"emit_sql_as_comment"`
		} `yaml:"go"`
	} `yaml:"gen"`
}

func applyYAMLDoc(cfg *Config, y yamlDoc) {
	if y.Package != "" {
		cfg.Package = y.Package
	}
	if y.OutDir != "" {
		cfg.OutDir = y.OutDir
		cfg.outDirSet = true
	}
	if y.QueryDir != "" {
		cfg.QueryDir = y.QueryDir
	}
	if y.ModelDir != "" {
		cfg.ModelDir = y.ModelDir
	}
	if y.SchemaDir != "" {
		cfg.SchemaDir = y.SchemaDir
	}
	if y.ModelPackage != "" {
		cfg.ModelPackage = y.ModelPackage
	}
	if y.ModelImport != "" {
		cfg.ModelImport = y.ModelImport
	}
	if y.Driver != "" {
		cfg.Driver = strings.ToLower(y.Driver)
	}
	if y.Dialect != "" {
		cfg.Dialect = strings.ToLower(y.Dialect)
	}
	if y.DatabaseURL != "" {
		cfg.DatabaseURL = y.DatabaseURL
	}
	if y.MigrationPath != "" {
		cfg.MigrationPath = y.MigrationPath
	}
	if y.ModelSource != "" {
		cfg.ModelSource = strings.ToLower(y.ModelSource)
		cfg.modelSourceSet = true
	}
	if y.SchemaName != "" {
		cfg.SchemaName = y.SchemaName
	}
	if y.EmitRelations != nil {
		cfg.EmitRelations = *y.EmitRelations
	}
	if y.EmitFunctions != nil {
		cfg.EmitFunctions = *y.EmitFunctions
	}
	if y.IncludeViews != nil {
		cfg.IncludeViews = *y.IncludeViews
	}
	if y.EmitSQLAsComment != nil {
		cfg.EmitSQLAsComment = *y.EmitSQLAsComment
	}
	if y.Gen != nil && y.Gen.Go != nil && y.Gen.Go.EmitSQLAsComment != nil {
		cfg.EmitSQLAsComment = *y.Gen.Go.EmitSQLAsComment
	}
}

// loadYAMLFile reads vorm.yaml or vorm.yml into cfg. ok is false when neither exists.
func loadYAMLFile(cfg *Config, dir string) (path string, ok bool, err error) {
	path, body, err := readYAML(dir)
	if err != nil || body == nil {
		return path, false, err
	}
	var y yamlDoc
	if err := yaml.Unmarshal(body, &y); err != nil {
		return path, false, fmt.Errorf("%s: %w", path, err)
	}
	applyYAMLDoc(cfg, y)
	return path, true, nil
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

// FormatYAML renders config as vorm.yaml.
func FormatYAML(c *Config) string {
	c = clone(c)
	c.applyDerived()
	var b strings.Builder
	b.WriteString("# vorm project config — edit or: vorm config set KEY=value\n")
	b.WriteString("# Prefer vorm.yaml / vorm.yml (sqlc-style). Legacy .vorm KEY=value still loads.\n")
	b.WriteString("version: \"1\"\n\n")
	fmt.Fprintf(&b, "package: %s\n", yamlQuote(c.Package))
	fmt.Fprintf(&b, "out_dir: %s\n", yamlQuote(c.OutDir))
	fmt.Fprintf(&b, "query_dir: %s\n", yamlQuote(c.QueryDir))
	fmt.Fprintf(&b, "model_dir: %s\n", yamlQuote(c.ModelDir))
	fmt.Fprintf(&b, "schema_dir: %s\n", yamlQuote(c.SchemaDir))
	fmt.Fprintf(&b, "model_package: %s\n", yamlQuote(c.ModelPackage))
	if c.ModelImport != "" {
		fmt.Fprintf(&b, "model_import: %s\n", yamlQuote(c.ModelImport))
	}
	fmt.Fprintf(&b, "driver: %s\n", yamlQuote(c.Driver))
	fmt.Fprintf(&b, "dialect: %s\n", yamlQuote(c.Dialect))
	fmt.Fprintf(&b, "migration_path: %s\n", yamlQuote(c.MigrationPath))
	fmt.Fprintf(&b, "model_source: %s\n", yamlQuote(c.ModelSource))
	fmt.Fprintf(&b, "schema_name: %s\n", yamlQuote(c.SchemaName))
	fmt.Fprintf(&b, "emit_relations: %v\n", c.EmitRelations)
	fmt.Fprintf(&b, "emit_functions: %v\n", c.EmitFunctions)
	fmt.Fprintf(&b, "include_views: %v\n", c.IncludeViews)
	b.WriteString("\n# Prefer DATABASE_URL in the environment over database_url here.\n")
	if c.DatabaseURL != "" {
		fmt.Fprintf(&b, "database_url: %s\n", yamlQuote(c.DatabaseURL))
	}
	b.WriteString("\ngen:\n  go:\n")
	fmt.Fprintf(&b, "    emit_sql_as_comment: %v\n", c.EmitSQLAsComment)
	return b.String()
}

func yamlQuote(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, ":#{}[],&*?|>!%@`'\"\n") || strings.Contains(s, " ") {
		return fmt.Sprintf("%q", s)
	}
	return s
}

// ConfigExists reports whether a vorm.yaml, vorm.yml, or legacy .vorm is present.
func ConfigExists(dir string) (string, bool) {
	if dir == "" {
		dir = "."
	}
	for _, name := range append(append([]string{}, YAMLFileNames...), LegacyFile) {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}
	return "", false
}
