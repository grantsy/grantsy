package db

import (
	"bytes"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"path"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed all:migrations/*
var migrationsFS embed.FS

func Migrate(driver, dsn, namespace string) error {
	var dbURL string
	var fsys fs.FS = migrationsFS

	switch driver {
	case "sqlite":
		dbURL = "sqlite://" + dsn
		if namespace != "" {
			dbURL += "?x-migrations-table=" + namespace + "_schema_migrations"
			fsys = &templateFS{inner: migrationsFS, replacements: map[string]string{
				"{ns}": namespace + "_",
			}}
		} else {
			fsys = &templateFS{inner: migrationsFS, replacements: map[string]string{
				"{ns}": "",
			}}
		}
	case "postgres":
		dbURL = "pgx5" + strings.TrimPrefix(dsn, "postgres")
		if namespace != "" {
			if err := ensureSchema(dsn, namespace); err != nil {
				return err
			}

			u, err := url.Parse(dbURL)
			if err != nil {
				return fmt.Errorf("migrations: failed to parse DSN: %w", err)
			}
			q := u.Query()
			q.Set("search_path", namespace)
			q.Set("x-migrations-table", fmt.Sprintf(`"%s"."schema_migrations"`, namespace))
			q.Set("x-migrations-table-quoted", "true")
			u.RawQuery = q.Encode()
			dbURL = u.String()
		}
	default:
		return fmt.Errorf("migrations: unsupported driver: %s", driver)
	}

	source, err := iofs.New(fsys, path.Join("migrations", driver))
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

// ensureSchema creates the PostgreSQL schema if it doesn't exist.
func ensureSchema(dsn, namespace string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("migrations: failed to open db for schema creation: %w", err)
	}
	defer db.Close()

	if _, err := db.Exec(fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", namespace)); err != nil {
		return fmt.Errorf("migrations: failed to create schema: %w", err)
	}
	return nil
}

// templateFS wraps an fs.FS and replaces placeholders in .sql files.
type templateFS struct {
	inner        fs.FS
	replacements map[string]string
}

func (t *templateFS) Open(name string) (fs.File, error) {
	f, err := t.inner.Open(name)
	if err != nil {
		return nil, err
	}

	if !strings.HasSuffix(name, ".sql") {
		return f, nil
	}

	data, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		return nil, err
	}

	for old, new := range t.replacements {
		data = bytes.ReplaceAll(data, []byte(old), []byte(new))
	}

	info, err := fs.Stat(t.inner, name)
	if err != nil {
		return nil, err
	}

	return &templateFile{
		Reader:   bytes.NewReader(data),
		fileInfo: info,
	}, nil
}

func (t *templateFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(t.inner, name)
}

type templateFile struct {
	*bytes.Reader
	fileInfo fs.FileInfo
}

func (f *templateFile) Stat() (fs.FileInfo, error) {
	return f.fileInfo, nil
}

func (f *templateFile) Close() error {
	return nil
}
