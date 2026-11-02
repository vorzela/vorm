package query

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

// OpenMySQL opens database/sql with the MySQL driver (MySQL / MariaDB DSNs).
// DSN example: user:pass@tcp(localhost:3306)/dbname?parseTime=true
func OpenMySQL(dsn string) (*SQLDB, error) {
	normalized, err := mysqlDSN(dsn)
	if err != nil {
		return nil, err
	}
	dsn = ensureMySQLParseTime(normalized)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &SQLDB{DB: db}, nil
}

// OpenMariaDB is an alias of OpenMySQL (same protocol driver).
func OpenMariaDB(dsn string) (*SQLDB, error) {
	return OpenMySQL(dsn)
}

// mysqlDSN converts mysql://host/db URLs to go-sql-driver's DSN form while
// leaving classic `user:pass@tcp(host:port)/db` DSNs alone.
func mysqlDSN(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "mysql://") && !strings.HasPrefix(raw, "mariadb://") {
		return raw, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("vorm/query: invalid MySQL URL: %w", err)
	}
	if u.User == nil && u.Host == "" {
		return "", fmt.Errorf("vorm/query: invalid MySQL URL: missing user or host")
	}
	user := ""
	pass := ""
	if u.User != nil {
		user = u.User.Username()
		pass, _ = u.User.Password()
	}
	out := fmt.Sprintf("%s:%s@tcp(%s)/%s", user, pass, u.Host, strings.TrimPrefix(u.Path, "/"))
	if u.RawQuery != "" {
		out += "?" + u.RawQuery
	}
	return out, nil
}

// ensureMySQLParseTime makes database/sql return time.Time for DATE/DATETIME
// columns, which is what generated structs expect.
func ensureMySQLParseTime(dsn string) string {
	if strings.Contains(dsn, "parseTime=") {
		return dsn
	}
	if strings.Contains(dsn, "?") {
		return dsn + "&parseTime=true"
	}
	return dsn + "?parseTime=true"
}
