package db

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
	driver string
}

func New(driver, dsn string) (*DB, error) {
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

	return &DB{DB: db, driver: driver}, nil
}

func (d *DB) Close() error {
	return d.DB.Close()
}
