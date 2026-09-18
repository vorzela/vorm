package migrate

import (
	"fmt"
	"strings"

	"github.com/vorzela/vorm/query"
)

// CompileGo turns a Go Blueprint migration into Up/Down SQL. The schema
// package registers the implementation in init so this package does not import
// schema (that import would cycle).
var CompileGo func(filename, src, dialect string) (up, down string, err error)

func statementsFor(path string, content []byte, down bool, dialect query.Dialect) ([]string, error) {
	if strings.HasSuffix(path, ".go") {
		if CompileGo == nil {
			return nil, fmt.Errorf("vorm/migrate: Go migrations require github.com/vorzela/vorm/schema")
		}
		up, downSQL, err := CompileGo(path, string(content), string(dialect))
		if err != nil {
			return nil, err
		}
		sql := up
		if down {
			sql = downSQL
		}
		return SplitStatements(sql), nil
	}
	if down {
		return SplitStatements(ExtractDown(string(content))), nil
	}
	return SplitStatements(ExtractUp(string(content))), nil
}
