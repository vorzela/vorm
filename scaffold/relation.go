package scaffold

import (
	"fmt"
	"os"
	"strings"

	"github.com/vorzela/vorm/schema"
)

// MakeRelation scaffolds a numbered Blueprint for one Eloquent-style association.
//
//	vorm make belongs-to posts users
//	vorm make has-one users profiles
//	vorm make has-many users posts
//	vorm make belongs-to-many posts tags
//	vorm make morphs comments commentable
//	vorm make morph-to-many tags taggable
func MakeRelation(kind string, args []string, dirs MigrationDirs) (*MakeResult, error) {
	dirs.defaults()
	kind = NormalizeRelationKind(kind)
	spec, err := relationSpec(kind, args)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dirs.Dir, 0o755); err != nil {
		return nil, err
	}
	path, err := uniquePath(dirs.Dir, spec.filename)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(spec.body), 0o644); err != nil {
		return nil, err
	}
	return &MakeResult{Table: spec.table, MigrationFile: path, Kind: kind}, nil
}

// NormalizeRelationKind maps CLI aliases onto the canonical kind names.
func NormalizeRelationKind(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	kind = strings.ReplaceAll(kind, "_", "-")
	switch kind {
	case "belongsto":
		return "belongs-to"
	case "hasone":
		return "has-one"
	case "hasmany":
		return "has-many"
	case "belongstomany", "btm", "pivot":
		return "belongs-to-many"
	case "morph-to", "morph-many", "morphto", "morphmany":
		return "morphs"
	case "morphtomany":
		return "morph-to-many"
	default:
		return kind
	}
}

type relSpec struct {
	filename, table, body string
}

func relationSpec(kind string, args []string) (relSpec, error) {
	switch kind {
	case "belongs-to":
		if len(args) < 2 {
			return relSpec{}, fmt.Errorf("usage: vorm make belongs-to <child> <parent> [column]")
		}
		child, parent := tableIdent(args[0]), tableIdent(args[1])
		col := fkColumn(parent)
		if len(args) >= 3 && strings.TrimSpace(args[2]) != "" {
			col = snakeName(args[2])
		}
		return relSpec{
			filename: "add_" + col + "_to_" + child + "_table.go",
			table:    child,
			body:     belongsToSource(child, parent, col, false),
		}, nil
	case "has-many":
		if len(args) < 2 {
			return relSpec{}, fmt.Errorf("usage: vorm make has-many <parent> <child> [column]")
		}
		parent, child := tableIdent(args[0]), tableIdent(args[1])
		col := fkColumn(parent)
		if len(args) >= 3 && strings.TrimSpace(args[2]) != "" {
			col = snakeName(args[2])
		}
		return relSpec{
			filename: "add_" + col + "_to_" + child + "_table.go",
			table:    child,
			body:     belongsToSource(child, parent, col, false),
		}, nil
	case "has-one":
		if len(args) < 2 {
			return relSpec{}, fmt.Errorf("usage: vorm make has-one <parent> <child> [column]")
		}
		parent, child := tableIdent(args[0]), tableIdent(args[1])
		col := fkColumn(parent)
		if len(args) >= 3 && strings.TrimSpace(args[2]) != "" {
			col = snakeName(args[2])
		}
		return relSpec{
			filename: "add_" + col + "_to_" + child + "_table.go",
			table:    child,
			body:     belongsToSource(child, parent, col, true),
		}, nil
	case "belongs-to-many":
		if len(args) < 2 {
			return relSpec{}, fmt.Errorf("usage: vorm make belongs-to-many <left> <right>")
		}
		left, right := tableIdent(args[0]), tableIdent(args[1])
		pivot := schema.PivotName(left, right)
		return relSpec{
			filename: "create_" + pivot + "_table.go",
			table:    pivot,
			body:     pivotSource(&pivotHint{LeftTable: left, RightTable: right}),
		}, nil
	case "morphs":
		if len(args) < 2 {
			return relSpec{}, fmt.Errorf("usage: vorm make morphs <child> <name>")
		}
		child := tableIdent(args[0])
		name := snakeName(args[1])
		if name == "" {
			return relSpec{}, fmt.Errorf("usage: vorm make morphs <child> <name>")
		}
		return relSpec{
			filename: "add_" + name + "_to_" + child + "_table.go",
			table:    child,
			body:     morphsSource(child, name),
		}, nil
	case "morph-to-many":
		if len(args) < 2 {
			return relSpec{}, fmt.Errorf("usage: vorm make morph-to-many <related> <morph>")
		}
		related := tableIdent(args[0])
		morph := snakeName(args[1])
		if morph == "" {
			return relSpec{}, fmt.Errorf("usage: vorm make morph-to-many <related> <morph>")
		}
		pivot := schema.Pluralize(morph)
		return relSpec{
			filename: "create_" + pivot + "_table.go",
			table:    pivot,
			body:     morphToManySource(related, morph, pivot),
		}, nil
	default:
		return relSpec{}, fmt.Errorf("unknown relation %q — try belongs-to, has-one, has-many, belongs-to-many, morphs, morph-to-many", kind)
	}
}

func tableIdent(s string) string {
	s = snakeName(s)
	s = strings.Trim(s, "_")
	if s == "" {
		return s
	}
	return schema.Pluralize(schema.Singularize(s))
}

func fkColumn(parentTable string) string {
	return schema.Singularize(parentTable) + "_id"
}

func uniqueIndexName(table string, cols ...string) string {
	return "uq_" + table + "_" + strings.Join(cols, "_")
}

func indexName(table string, cols ...string) string {
	return "idx_" + table + "_" + strings.Join(cols, "_")
}

func belongsToSource(child, parent, col string, unique bool) string {
	upExtra := ""
	downIndex := ""
	comment := fmt.Sprintf("%s belongsTo %s via %s.%s", child, parent, child, col)
	if unique {
		upExtra = fmt.Sprintf("\n\t\tt.Unique(%q)", col)
		downIndex = fmt.Sprintf("\t\tt.DropIndex(%q)\n", uniqueIndexName(child, col))
		comment = fmt.Sprintf("%s hasOne %s via unique %s.%s", parent, child, child, col)
	}
	return fmt.Sprintf(`//go:build ignore

package migrations

import "github.com/vorzela/vorm/schema"

// %s
func Up(s *schema.Facade) {
	s.Table(%q, func(t *schema.Blueprint) {
		t.BelongsTo(%q, %q)%s
	})
}

func Down(s *schema.Facade) {
	s.Table(%q, func(t *schema.Blueprint) {
%s		t.DropColumn(%q)
	})
}
`, comment, child, col, parent, upExtra, child, downIndex, col)
}

func morphsSource(child, name string) string {
	idx := indexName(child, name+"_type", name+"_id")
	return fmt.Sprintf(`//go:build ignore

package migrations

import "github.com/vorzela/vorm/schema"

// %s morphTo %s; other tables morphMany %s
func Up(s *schema.Facade) {
	s.Table(%q, func(t *schema.Blueprint) {
		t.Morphs(%q)
	})
}

func Down(s *schema.Facade) {
	s.Table(%q, func(t *schema.Blueprint) {
		t.DropIndex(%q)
		t.DropColumn(%q)
		t.DropColumn(%q)
	})
}
`, child, name, child, child, name, child, idx, name+"_type", name+"_id")
}

func morphToManySource(related, morph, pivot string) string {
	return fmt.Sprintf(`//go:build ignore

package migrations

import "github.com/vorzela/vorm/schema"

func Up(s *schema.Facade) {
	s.MorphToMany(%q, %q)
}

func Down(s *schema.Facade) {
	s.DropIfExists(%q)
}
`, related, morph, pivot)
}
