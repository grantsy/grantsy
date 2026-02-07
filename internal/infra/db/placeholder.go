package db

import "regexp"

var placeholderRegex = regexp.MustCompile(`\$\d+`)

// Rebind converts $1, $2, ... placeholders to ? for SQLite.
// Queries are written in PostgreSQL notation as canonical form.
func (d *DB) Rebind(query string) string {
	if d.driver == "sqlite" {
		return placeholderRegex.ReplaceAllString(query, "?")
	}
	return query
}

// Driver returns the current database driver name.
func (d *DB) Driver() string {
	return d.driver
}
