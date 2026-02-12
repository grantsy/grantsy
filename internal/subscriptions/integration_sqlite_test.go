package subscriptions_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grantsy/grantsy/internal/infra/db"
	"github.com/grantsy/grantsy/internal/subscriptions"
)

func newSQLiteRepo(t *testing.T) *subscriptions.Repo {
	t.Helper()

	dsn := filepath.Join(t.TempDir(), "test.db")

	err := db.Migrate("sqlite", dsn, "")
	require.NoError(t, err, "sqlite migration failed")

	database, err := db.New("sqlite", dsn, "")
	require.NoError(t, err, "sqlite connection failed")

	t.Cleanup(func() { database.Close() })

	return subscriptions.NewRepo(database)
}
