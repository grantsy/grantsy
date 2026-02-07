package db

import (
	"embed"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed all:migrations/*
var migrationsFS embed.FS

func Migrate(driver, dsn string) error {
	var dbURL string
	switch driver {
	case "sqlite":
		dbURL = "sqlite3://" + dsn
	case "postgres":
		dbURL = "pgx5" + strings.TrimPrefix(dsn, "postgres")
	default:
		return fmt.Errorf("migrations: unsupported driver: %s", driver)
	}

	source, err := iofs.New(migrationsFS, path.Join("migrations", driver))
	if err != nil {
		return fmt.Errorf("migrations: failed to create source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, dbURL)
	if err != nil {
		return fmt.Errorf("migrations: failed to create instance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrations: failed to run: %w", err)
	}

	return nil
}
