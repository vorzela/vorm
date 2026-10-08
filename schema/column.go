package schema

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Column is a fluent column definition (Laravel Blueprint column).
type Column struct {
	name          string
	dataType      string
	nullable      bool
	isPrimary     bool
	autoInc       bool
	unique        bool
	defaultExpr   string
	isTimestamp   bool
	foreignTable  string
	foreignColumn string
	onDelete      string // CASCADE|RESTRICT|SET NULL|NO ACTION
	onUpdate      string
	enumName      string
	enumValues    []string
	custom        bool // set by CustomType; name arrives via Column
	uuidV4        bool // t.UUID: dialect picks the v4 default
}

func newColumn(name string) *Column {
	return &Column{name: name, dataType: "TEXT", nullable: true}
}

func (c *Column) typ(t string) *Column {
	c.dataType = t
	c.isTimestamp = false
	return c
}

func (c *Column) timestamp() *Column {
	c.isTimestamp = true
	return c
}

func (c *Column) primary() *Column {
	c.isPrimary = true
	c.nullable = false
	return c
}

// Primary marks the column as the table primary key.
//
//	t.UUID("id").Primary()
func (c *Column) Primary() *Column {
	return c.primary()
}

func (c *Column) autoIncrement() *Column {
	c.autoInc = true
	c.nullable = false
	return c
}

// Nullable marks the column nullable.
func (c *Column) Nullable() *Column {
	c.nullable = true
	return c
}

// NotNull marks the column NOT NULL.
func (c *Column) NotNull() *Column {
	c.nullable = false
	return c
}

// Unique adds a UNIQUE constraint on this column.
func (c *Column) Unique() *Column {
	c.unique = true
	return c
}

// Default sets a column default from a Go value. Strings and JSON are quoted
// as SQL literals; bool/int/float stay unquoted.
//
//	t.Boolean("active").Default(true)          // DEFAULT TRUE
//	t.Integer("age").Default(0)                // DEFAULT 0
//	t.String("currency_code").Default("KES")   // DEFAULT 'KES'
//	t.Json("meta").Default("{}")               // DEFAULT '{}'
//	t.Json("meta").Default(map[string]any{})   // DEFAULT '{}'
//
// Already-quoted SQL ('KES') and known SQL expressions (CURRENT_TIMESTAMP,
// gen_random_uuid(), NULL, …) are left as-is. Prefer DefaultRaw for other SQL.
func (c *Column) Default(v any) *Column {
	c.defaultExpr = formatDefault(v)
	return c
}

// DefaultRaw sets a DEFAULT expression written through unchanged.
//
//	t.UUID("id").DefaultRaw("gen_random_uuid()")
//	t.Timestamp("expires_at").DefaultRaw("NOW() + INTERVAL '7 days'")
func (c *Column) DefaultRaw(expr string) *Column {
	c.defaultExpr = strings.TrimSpace(expr)
	return c
}

// DefaultCurrent sets DEFAULT CURRENT_TIMESTAMP.
func (c *Column) DefaultCurrent() *Column {
	c.defaultExpr = "CURRENT_TIMESTAMP"
	c.nullable = false
	return c
}

func formatDefault(v any) string {
	switch x := v.(type) {
	case bool:
		if x {
			return "TRUE"
		}
		return "FALSE"
	case int:
		return strconv.Itoa(x)
	case int8:
		return strconv.FormatInt(int64(x), 10)
	case int16:
		return strconv.FormatInt(int64(x), 10)
	case int32:
		return strconv.FormatInt(int64(x), 10)
	case int64:
		return strconv.FormatInt(x, 10)
	case uint:
		return strconv.FormatUint(uint64(x), 10)
	case uint8:
		return strconv.FormatUint(uint64(x), 10)
	case uint16:
		return strconv.FormatUint(uint64(x), 10)
	case uint32:
		return strconv.FormatUint(uint64(x), 10)
	case uint64:
		return strconv.FormatUint(x, 10)
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case string:
		return formatDefaultString(x)
	case json.RawMessage:
		if len(x) == 0 {
			return quoteSQLString("null")
		}
		return quoteSQLString(string(x))
	case []byte:
		return quoteSQLString(string(x))
	case nil:
		return "NULL"
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return quoteSQLString(fmt.Sprint(v))
		}
		return quoteSQLString(string(b))
	}
}

func formatDefaultString(s string) string {
	if isSQLStringLiteral(s) || isSQLDefaultExpr(s) {
		return s
	}
	return quoteSQLString(s)
}

func quoteSQLString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// isSQLStringLiteral reports whether s is already a single-quoted SQL literal
// (including escaped '' inside), e.g. 'KES' or 'it''s'.
func isSQLStringLiteral(s string) bool {
	if len(s) < 2 || s[0] != '\'' || s[len(s)-1] != '\'' {
		return false
	}
	inner := s[1 : len(s)-1]
	for i := 0; i < len(inner); i++ {
		if inner[i] != '\'' {
			continue
		}
		if i+1 >= len(inner) || inner[i+1] != '\'' {
			return false
		}
		i++
	}
	return true
}

// isSQLDefaultExpr keeps common raw SQL defaults working without DefaultRaw.
func isSQLDefaultExpr(s string) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return false
	}
	upper := strings.ToUpper(trimmed)
	switch upper {
	case "NULL", "TRUE", "FALSE",
		"CURRENT_TIMESTAMP", "CURRENT_DATE", "CURRENT_TIME",
		"LOCALTIMESTAMP", "LOCALTIME":
		return true
	}
	// Function / cast expression: gen_random_uuid(), NOW(), '{}'::jsonb
	return strings.ContainsAny(trimmed, "():")
}

// Constrained sets REFERENCES table(id). Optional second arg is the parent column.
// Default ON DELETE CASCADE.
//
//	t.ForeignID("user_id").Constrained("users")
//	t.ForeignID("user_id").Constrained("users", "uuid")
func (c *Column) Constrained(table string, column ...string) *Column {
	if table == "" {
		panic("vorm/schema: Constrained(table) requires a non-empty table name")
	}
	col := "id"
	if len(column) > 0 && strings.TrimSpace(column[0]) != "" {
		col = strings.TrimSpace(column[0])
	}
	return c.References(table, col)
}

// References sets REFERENCES table(column).
func (c *Column) References(table, column string) *Column {
	c.foreignTable = table
	c.foreignColumn = column
	c.nullable = false
	if c.onDelete == "" {
		c.onDelete = "CASCADE"
	}
	return c
}

// CascadeOnDelete sets ON DELETE CASCADE.
func (c *Column) CascadeOnDelete() *Column {
	c.onDelete = "CASCADE"
	return c
}

// RestrictOnDelete sets ON DELETE RESTRICT.
func (c *Column) RestrictOnDelete() *Column {
	c.onDelete = "RESTRICT"
	return c
}

// NullOnDelete sets ON DELETE SET NULL and marks the column nullable.
func (c *Column) NullOnDelete() *Column {
	c.onDelete = "SET NULL"
	c.nullable = true
	return c
}

// CascadeOnUpdate sets ON UPDATE CASCADE.
func (c *Column) CascadeOnUpdate() *Column {
	c.onUpdate = "CASCADE"
	return c
}

func (c *Column) enumType(typeName string, values []string) *Column {
	c.enumName = typeName
	c.enumValues = values
	c.dataType = typeName
	return c
}

// Column names the field created by CustomType.
//
//	t.CustomType("GEOGRAPHY(POINT, 4326)").Column("location")
func (c *Column) Column(name string) *Column {
	if err := c.setName(name); err != nil {
		panic(err)
	}
	return c
}

func (c *Column) setName(name string) error {
	if !c.custom {
		return fmt.Errorf("vorm/schema: Column() is only valid after CustomType")
	}
	name = strings.TrimSpace(name)
	if !colIdentRe.MatchString(name) {
		return fmt.Errorf("vorm/schema: invalid column name %q", name)
	}
	c.name = name
	return nil
}

func (c *Column) sql(dialect string) string {
	if c.name == "" {
		panic("vorm/schema: CustomType requires .Column(\"name\")")
	}
	mysql := dialect == "mysql" || dialect == "mariadb"
	var b strings.Builder
	b.WriteString(c.name)
	b.WriteString(" ")

	switch {
	case c.uuidV4 && dialect == "mysql":
		// MySQL has UUID() (version 1) and no UUID column type.
		b.WriteString("CHAR(36)")
	case c.autoInc && c.isPrimary:
		if mysql {
			b.WriteString("BIGINT AUTO_INCREMENT PRIMARY KEY")
		} else {
			b.WriteString("BIGSERIAL PRIMARY KEY")
		}
	case c.isTimestamp:
		if mysql {
			b.WriteString("TIMESTAMP")
		} else {
			b.WriteString("TIMESTAMPTZ")
		}
	case len(c.enumValues) > 0 && mysql:
		b.WriteString("ENUM(")
		b.WriteString(quoteEnumValues(c.enumValues))
		b.WriteString(")")
	default:
		b.WriteString(c.sqlDataType(dialect))
	}

	if !c.autoInc || !c.isPrimary {
		if c.nullable {
			b.WriteString(" NULL")
		} else {
			b.WriteString(" NOT NULL")
		}
	}
	if expr := c.defaultSQL(dialect); expr != "" && !(c.autoInc && c.isPrimary) {
		b.WriteString(" DEFAULT ")
		b.WriteString(expr)
	}
	if c.unique && !c.isPrimary {
		b.WriteString(" UNIQUE")
	}
	if c.isPrimary && !c.autoInc {
		b.WriteString(" PRIMARY KEY")
	}
	if c.foreignTable != "" {
		b.WriteString(" REFERENCES ")
		b.WriteString(c.foreignTable)
		b.WriteString("(")
		b.WriteString(c.foreignColumn)
		b.WriteString(")")
		if c.onDelete != "" {
			b.WriteString(" ON DELETE ")
			b.WriteString(c.onDelete)
		}
		if c.onUpdate != "" {
			b.WriteString(" ON UPDATE ")
			b.WriteString(c.onUpdate)
		}
	}
	return b.String()
}

// sqlDataType maps the stored (Postgres-oriented) type onto the target dialect.
// CustomType values are never remapped.
func (c *Column) sqlDataType(dialect string) string {
	if c.custom {
		return c.dataType
	}
	mysql := dialect == "mysql" || dialect == "mariadb"
	upper := strings.ToUpper(c.dataType)
	if !mysql {
		if upper == "DATETIME" {
			return "TIMESTAMP"
		}
		return c.dataType
	}
	switch {
	case upper == "TIMESTAMPTZ":
		return "TIMESTAMP"
	case upper == "DATETIME":
		return "DATETIME"
	case upper == "JSONB", upper == "JSON":
		return "JSON"
	case upper == "REAL":
		return "FLOAT"
	case upper == "DOUBLE PRECISION":
		return "DOUBLE"
	case strings.HasPrefix(upper, "NUMERIC"):
		return "DECIMAL" + c.dataType[len("NUMERIC"):]
	case upper == "BYTEA":
		return "BLOB"
	case upper == "INET":
		return "VARCHAR(45)"
	case upper == "INTEGER":
		return "INT"
	case upper == "BOOLEAN":
		return "TINYINT(1)"
	default:
		return c.dataType
	}
}

// defaultSQL is an explicit Default(), or the v4 generator for t.UUID().
// MariaDB 11.7+ has UUID_v4(). MySQL has no v4 function.
func (c *Column) defaultSQL(dialect string) string {
	if c.defaultExpr != "" {
		return c.defaultExpr
	}
	if !c.uuidV4 {
		return ""
	}
	switch dialect {
	case "mariadb":
		return "UUID_v4()"
	case "mysql":
		return ""
	default:
		return "gen_random_uuid()"
	}
}
