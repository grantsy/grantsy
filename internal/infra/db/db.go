package db

import (
	"database/sql"
	"fmt"
	"net/url"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
	driver    string
	namespace string
}

func New(driver, dsn, namespace string) (*DB, error) {
	// For PostgreSQL with a namespace, ensure the schema exists and embed
	// search_path in the DSN so every pooled connection uses the right schema.
	if driver == "postgres" && namespace != "" {
		if err := ensureSchema(dsn, namespace); err != nil {
			return nil, fmt.Errorf("db: %w", err)
		}
		var err error
		dsn, err = appendSearchPath(dsn, namespace)
		if err != nil {
			return nil, fmt.Errorf("db: %w", err)
		}
	}

	var db *sql.DB
	var err error

	switch driver {
	case "sqlite":
		db, err = sql.Open("sqlite", dsn)
	case "postgres":
		db, err = sql.Open("pgx", dsn)
	default:
		return nil, fmt.Errorf("db: unsupported driver: %s", driver)
	}

	if err != nil {
		return nil, fmt.Errorf("db: failed to open: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("db: failed to ping: %w", err)
	}

	return &DB{DB: db, driver: driver, namespace: namespace}, nil
}

func appendSearchPath(dsn, namespace string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("failed to parse dsn: %w", err)
	}
	q := u.Query()
	q.Set("search_path", namespace+",public")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// TableName returns the qualified table name for the current driver and namespace.
// For PostgreSQL, search_path handles schema resolution, so the name is returned as-is.
// For SQLite with a namespace, the table name is prefixed with "namespace_".
func (d *DB) TableName(name string) string {
	if d.driver == "sqlite" && d.namespace != "" {
		return d.namespace + "_" + name
	}
	return name
}

func (d *DB) Close() error {
	return d.DB.Close()
}
