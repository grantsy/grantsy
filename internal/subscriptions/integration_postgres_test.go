package subscriptions_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/grantsy/grantsy/internal/infra/db"
	"github.com/grantsy/grantsy/internal/subscriptions"
)

var pgDBCounter atomic.Int64

func startPostgresContainer(ctx context.Context) (string, testcontainers.Container, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("integration"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		return "", nil, fmt.Errorf("start postgres container: %w", err)
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		container.Terminate(ctx)
		return "", nil, fmt.Errorf("get connection string: %w", err)
	}

	return connStr, container, nil
}

func newPostgresRepo(baseURL string) func(t *testing.T) *subscriptions.Repo {
	return func(t *testing.T) *subscriptions.Repo {
		t.Helper()

		n := pgDBCounter.Add(1)
		dbName := fmt.Sprintf("test_%d", n)

		// Connect to the base database to create a fresh test database.
		adminDB, err := sql.Open("pgx", baseURL)
		require.NoError(t, err, "failed to connect to admin PG database")
		defer adminDB.Close()

		_, err = adminDB.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName))
		require.NoError(t, err, "failed to create test database %s", dbName)

		t.Cleanup(func() {
			conn, err := sql.Open("pgx", baseURL)
			if err == nil {
				conn.Exec(fmt.Sprintf("DROP DATABASE %s WITH (FORCE)", dbName))
				conn.Close()
			}
		})

		testDSN := replaceDatabaseInURL(baseURL, dbName)

		err = db.Migrate("postgres", testDSN)
		require.NoError(t, err, "postgres migration failed for %s", dbName)

		database, err := db.New("postgres", testDSN)
		require.NoError(t, err, "postgres connection failed for %s", dbName)

		t.Cleanup(func() { database.Close() })

		return subscriptions.NewRepo(database)
	}
}

// replaceDatabaseInURL replaces the database name in a PostgreSQL URL.
// "postgres://test:test@localhost:55432/integration?sslmode=disable"
// becomes "postgres://test:test@localhost:55432/newdb?sslmode=disable".
func replaceDatabaseInURL(connStr, newDB string) string {
	parts := strings.SplitN(connStr, "?", 2)
	base := parts[0]

	lastSlash := strings.LastIndex(base, "/")
	if lastSlash == -1 {
		return connStr
	}

	result := base[:lastSlash+1] + newDB
	if len(parts) > 1 {
		result += "?" + parts[1]
	}
	return result
}
