package generate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/vorzela/vorm/introspect"
	"github.com/vorzela/vorm/query"
)

// SchemaDrift is one models/ vs live-schema mismatch.
type SchemaDrift struct {
	Table   string
	Column  string // empty for table-level findings
	Message string
}

func (d SchemaDrift) String() string {
	if d.Column != "" {
		return fmt.Sprintf("%s.%s: %s", d.Table, d.Column, d.Message)
	}
	return fmt.Sprintf("%s: %s", d.Table, d.Message)
}

// ParseModelsDir reads generated models/*.go into ModelSpec keyed by entity name.
func ParseModelsDir(dir string) (map[string]ModelSpec, error) {
	return parseModelsDir(dir)
}

// DiffModelsSchema compares on-disk models against a live schema snapshot.
// Only tables present in models are checked; extra schema tables are ignored.
func DiffModelsSchema(models map[string]ModelSpec, schema *introspect.Schema, dialect query.Dialect) []SchemaDrift {
	if schema == nil {
		return nil
	}
	byTable := uniqueModelsByTable(models)
	schemaTables := map[string]introspect.Table{}
	for _, t := range schema.Tables {
		schemaTables[t.Name] = t
	}
	mapper := NewTypeMapper(dialect, schema, nil)

	var drifts []SchemaDrift
	tables := make([]string, 0, len(byTable))
	for name := range byTable {
		tables = append(tables, name)
	}
	sort.Strings(tables)

	for _, table := range tables {
		ms := byTable[table]
		st, ok := schemaTables[table]
		if !ok {
			drifts = append(drifts, SchemaDrift{
				Table:   table,
				Message: "table missing in database schema (run migrations or vorm generate models after fixing)",
			})
			continue
		}
		schemaCols := map[string]introspect.Column{}
		for _, c := range st.Columns {
			schemaCols[c.Name] = c
		}
		modelCols := map[string]bool{}
		for _, c := range ms.Columns {
			modelCols[c] = true
			if _, ok := schemaCols[c]; !ok {
				drifts = append(drifts, SchemaDrift{
					Table:   table,
					Column:  c,
					Message: "column in models but not in database schema",
				})
			}
		}
		fieldByCol := map[string]FieldSpec{}
		for _, f := range ms.Fields {
			if f.Column == "" || f.Column == "-" {
				continue
			}
			fieldByCol[f.Column] = f
		}
		colNames := make([]string, 0, len(st.Columns))
		for _, c := range st.Columns {
			colNames = append(colNames, c.Name)
		}
		sort.Strings(colNames)
		for _, name := range colNames {
			c := schemaCols[name]
			if !modelCols[name] {
				drifts = append(drifts, SchemaDrift{
					Table:   table,
					Column:  name,
					Message: "column in database schema but missing from models (run: vorm generate models)",
				})
				continue
			}
			f, ok := fieldByCol[name]
			if !ok {
				continue
			}
			want := mapper.Resolve(c)
			if !goTypesCompatible(f.Type, want.Name) {
				drifts = append(drifts, SchemaDrift{
					Table:   table,
					Column:  name,
					Message: fmt.Sprintf("Go type %q does not match schema mapping %q", f.Type, want.Name),
				})
			}
		}
	}
	return drifts
}

// FormatSchemaDrift renders drift findings for CLI output.
func FormatSchemaDrift(drifts []SchemaDrift) string {
	if len(drifts) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("vorm: models do not match database schema:\n")
	for _, d := range drifts {
		fmt.Fprintf(&b, "  - %s\n", d.String())
	}
	b.WriteString("run: vorm generate models\n")
	return b.String()
}

// CheckModelsDir parses models/ and diffs against schema. Returns a non-nil
// error when any drift is found.
func CheckModelsDir(modelDir string, schema *introspect.Schema, dialect query.Dialect) error {
	models, err := ParseModelsDir(modelDir)
	if err != nil {
		return err
	}
	if len(models) == 0 {
		return nil
	}
	drifts := DiffModelsSchema(models, schema, dialect)
	if len(drifts) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.TrimSuffix(FormatSchemaDrift(drifts), "\n"))
}

func uniqueModelsByTable(models map[string]ModelSpec) map[string]ModelSpec {
	out := map[string]ModelSpec{}
	for key, ms := range models {
		if ms.Table == "" {
			continue
		}
		// Prefer bare entity keys (Users) over pkg-qualified (models.Users).
		if strings.Contains(key, ".") {
			if _, exists := out[ms.Table]; exists {
				continue
			}
		}
		out[ms.Table] = ms
	}
	return out
}

// goTypesCompatible compares a parsed struct field type to a TypeMapper name.
func goTypesCompatible(got, want string) bool {
	return normalizeGoType(got) == normalizeGoType(want)
}

func normalizeGoType(t string) string {
	t = strings.TrimSpace(t)
	t = strings.ReplaceAll(t, " ", "")
	// models.UserStatus → UserStatus; *models.UserStatus → *UserStatus
	ptr := strings.HasPrefix(t, "*")
	if ptr {
		t = t[1:]
	}
	if i := strings.LastIndex(t, "."); i >= 0 {
		t = t[i+1:]
	}
	if ptr {
		t = "*" + t
	}
	return t
}
