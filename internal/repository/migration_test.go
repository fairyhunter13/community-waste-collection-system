//go:build integration

// I1: migration reversibility — runs all migrations Up, then Down, then Up
// again against a dedicated Postgres testcontainer and asserts no error at
// any step. Confirms that every migration file has a working down-migration.
package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
	testcontainers "github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestMigrations_DownThenUp_Reversible(t *testing.T) {
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("test_migrate"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	newMigrator := func() *migrate.Migrate {
		m, err := migrate.New("file://../../migrations", connStr)
		require.NoError(t, err)
		return m
	}

	m := newMigrator()
	require.NoError(t, m.Up(), "first Up must succeed")
	v1, _, err := m.Version()
	require.NoError(t, err)
	m.Close()

	m = newMigrator()
	require.NoError(t, m.Down(), "Down must succeed")
	m.Close()

	m = newMigrator()
	require.NoError(t, m.Up(), "second Up after Down must succeed")
	v2, _, err := m.Version()
	require.NoError(t, err)
	m.Close()

	require.Equal(t, v1, v2, "migration version after Up → Down → Up must match first Up")
}
