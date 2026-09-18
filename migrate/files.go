package migrate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Migration is a migration file on disk.
type Migration struct {
	// Name is the bare file name, e.g. 1712345678_create_users_table.sql. It is
	// also the value stored in the migration column of the tracking table.
	Name string
	// Path is Name joined with the migrations directory.
	Path string
	// Timestamp is the numeric prefix of Name.
	Timestamp int64
	// Checksum is the SHA-256 of the whole file, lowercase hex.
	Checksum string
}

// Discover returns the migration files in dir ordered by timestamp, then by
// name so that files sharing a timestamp keep a stable order.
//
// Numbered *.sql files (legacy) and numbered *.go Blueprint files both count.
// extensions.sql, functions.sql, enums.sql and *_test.go stay out of the
// sequence. Subdirectories are ignored, and a missing directory yields no
// migrations rather than an error.
func Discover(dir string) ([]Migration, error) {
	if dir == "" {
		dir = DefaultDir
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("vorm/migrate: read directory %s: %w", dir, err)
	}

	out := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isMigrationName(name) {
			continue
		}
		ts, ok := parseTimestamp(name)
		if !ok {
			continue
		}
		path := filepath.Join(dir, name)
		sum, err := Checksum(path)
		if err != nil {
			return nil, err
		}
		out = append(out, Migration{Name: name, Path: path, Timestamp: ts, Checksum: sum})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Timestamp != out[j].Timestamp {
			return out[i].Timestamp < out[j].Timestamp
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func isMigrationName(name string) bool {
	if strings.HasSuffix(name, "_test.go") {
		return false
	}
	ext := filepath.Ext(name)
	if ext != ".sql" && ext != ".go" {
		return false
	}
	_, ok := parseTimestamp(name)
	return ok
}

// parseTimestamp reads the leading run of digits from a migration file name.
func parseTimestamp(name string) (int64, bool) {
	end := 0
	for end < len(name) && name[end] >= '0' && name[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, false
	}
	ts, err := strconv.ParseInt(name[:end], 10, 64)
	if err != nil {
		return 0, false
	}
	return ts, true
}
