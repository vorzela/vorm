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
			return relSpec{}, fmt.Errorf("usage: vorm make belongs-to <child> <parent> [column] [references]")
		}
		child, parent := tableIdent(args[0]), tableIdent(args[1])
		col, references := relationColumnArgs(parent, args[2:])
		return relSpec{
			filename: "add_" + col + "_to_" + child + "_table.go",
			table:    child,
			body:     belongsToSource(child, parent, col, references, false),
		}, nil
	case "has-many":
		if len(args) < 2 {
			return relSpec{}, fmt.Errorf("usage: vorm make has-many <parent> <child> [column] [references]")
		}
		parent, child := tableIdent(args[0]), tableIdent(args[1])
		col, references := relationColumnArgs(parent, args[2:])
		return relSpec{
			filename: "add_" + col + "_to_" + child + "_table.go",
			table:    child,
			body:     belongsToSource(child, parent, col, references, false),
		}, nil
	case "has-one":
		if len(args) < 2 {
			return relSpec{}, fmt.Errorf("usage: vorm make has-one <parent> <child> [column] [references]")
		}
		parent, child := tableIdent(args[0]), tableIdent(args[1])
		col, references := relationColumnArgs(parent, args[2:])
		return relSpec{
			filename: "add_" + col + "_to_" + child + "_table.go",
			table:    child,
			body:     belongsToSource(child, parent, col, references, true),
		}, nil
	case "belongs-to-many":
		if len(args) < 2 {
			return relSpec{}, fmt.Errorf("usage: vorm make belongs-to-many <left> <right> [left_ref] [right_ref]")
		}
		left, right := tableIdent(args[0]), tableIdent(args[1])
		pivot := schema.PivotName(left, right)
		leftRef, rightRef := optionalRef(args, 2), optionalRef(args, 3)
		return relSpec{
			filename: "create_" + pivot + "_table.go",
			table:    pivot,
			body:     pivotSource(&pivotHint{LeftTable: left, RightTable: right, LeftRef: leftRef, RightRef: rightRef}),
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
			return relSpec{}, fmt.Errorf("usage: vorm make morph-to-many <related> <morph> [references]")
		}
		related := tableIdent(args[0])
		morph := snakeName(args[1])
		if morph == "" {
			return relSpec{}, fmt.Errorf("usage: vorm make morph-to-many <related> <morph> [references]")
		}
		pivot := schema.Pluralize(morph)
		return relSpec{
			filename: "create_" + pivot + "_table.go",
			table:    pivot,
			body:     morphToManySource(related, morph, pivot, optionalRef(args, 2)),
		}, nil
	default:
		return relSpec{}, fmt.Errorf("unknown relation %q — try belongs-to, has-one, has-many, belongs-to-many, morphs, morph-to-many", kind)
	}
}

// relationColumnArgs parses optional [column] [references] after the two tables.
func relationColumnArgs(parentTable string, rest []string) (col, references string) {
	col = fkColumn(parentTable)
	if len(rest) >= 1 && strings.TrimSpace(rest[0]) != "" {
		col = snakeName(rest[0])
	}
	if len(rest) >= 2 && strings.TrimSpace(rest[1]) != "" {
		references = snakeName(rest[1])
	}
	return col, references
}

func optionalRef(args []string, i int) string {
	if len(args) <= i {
		return ""
	}
	return snakeName(args[i])
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

func belongsToSource(child, parent, col, references string, unique bool) string {
	upExtra := ""
	downIndex := ""
	parentCol := "id"
	if references != "" {
		parentCol = references
	}
	comment := fmt.Sprintf("%s belongsTo %s via %s.%s → %s.%s", child, parent, child, col, parent, parentCol)
	if unique {
		upExtra = fmt.Sprintf("\n\t\tt.Unique(%q)", col)
		downIndex = fmt.Sprintf("\t\tt.DropIndex(%q)\n", uniqueIndexName(child, col))
		comment = fmt.Sprintf("%s hasOne %s via unique %s.%s → %s.%s", parent, child, child, col, parent, parentCol)
	}
	belongsCall := fmt.Sprintf("t.BelongsTo(%q, %q)", col, parent)
	if references != "" && references != "id" {
		belongsCall = fmt.Sprintf("t.BelongsTo(%q, %q, %q)", col, references, parent)
	}
	return fmt.Sprintf(`//go:build ignore

package main

import "github.com/vorzela/vorm/schema"

// %s
func Up(s *schema.Facade) {
	s.Table(%q, func(t *schema.Blueprint) {
		%s%s
	})
}

func Down(s *schema.Facade) {
	s.Table(%q, func(t *schema.Blueprint) {
%s		t.DropColumn(%q)
	})
}
`, comment, child, belongsCall, upExtra, child, downIndex, col)
}

func morphsSource(child, name string) string {
	idx := indexName(child, name+"_type", name+"_id")
	return fmt.Sprintf(`//go:build ignore

package main

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

func morphToManySource(related, morph, pivot, references string) string {
	call := fmt.Sprintf("%q, %q", related, morph)
	if references != "" && references != "id" {
		call = fmt.Sprintf("%q, %q, %q", related, morph, references)
	}
	return fmt.Sprintf(`//go:build ignore

package main

import "github.com/vorzela/vorm/schema"

func Up(s *schema.Facade) {
	s.MorphToMany(%s)
}

func Down(s *schema.Facade) {
	s.DropIfExists(%q)
}
`, call, pivot)
}
