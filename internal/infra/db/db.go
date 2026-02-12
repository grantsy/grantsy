package db

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
	driver    string
	namespace string
}

func New(driver, dsn, namespace string) (*DB, error) {
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

	if driver == "postgres" && namespace != "" {
		if _, err := db.Exec(fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", namespace)); err != nil {
			return nil, fmt.Errorf("db: failed to create schema: %w", err)
		}
		if _, err := db.Exec(fmt.Sprintf("SET search_path TO %s,public", namespace)); err != nil {
			return nil, fmt.Errorf("db: failed to set search_path: %w", err)
		}
	}

	return &DB{DB: db, driver: driver, namespace: namespace}, nil
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
