//go:build integration

package repository

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestApplyMigrationsFS_DashboardQuotaUsageFactsIndexMigration_RepairsActualInvalidIndex(t *testing.T) {
	ctx := context.Background()
	const invalidIndexTable = "dashboard_quota_invalid_index_retry_test"

	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DROP TABLE IF EXISTS "+invalidIndexTable)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM schema_migrations WHERE filename = $1", dashboardQuotaUsageFactsIndexMigration)
		require.NoError(t, ApplyMigrations(ctx, integrationDB))
	})

	_, err := integrationDB.ExecContext(ctx, "DROP INDEX CONCURRENTLY IF EXISTS "+dashboardQuotaUsageFactsIndex)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, "DROP TABLE IF EXISTS "+invalidIndexTable)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, "CREATE TABLE "+invalidIndexTable+" (value INTEGER NOT NULL)")
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, "INSERT INTO "+invalidIndexTable+" (value) VALUES (1), (1)")
	require.NoError(t, err)

	_, err = integrationDB.ExecContext(ctx, "CREATE UNIQUE INDEX CONCURRENTLY "+dashboardQuotaUsageFactsIndex+" ON "+invalidIndexTable+" (value)")
	require.Error(t, err)

	invalid, err := indexIsInvalid(ctx, integrationDB, dashboardQuotaUsageFactsIndex)
	require.NoError(t, err)
	require.True(t, invalid)

	_, err = integrationDB.ExecContext(ctx, "DELETE FROM schema_migrations WHERE filename = $1", dashboardQuotaUsageFactsIndexMigration)
	require.NoError(t, err)

	content, err := migrations.FS.ReadFile(dashboardQuotaUsageFactsIndexMigration)
	require.NoError(t, err)
	fSys := fstest.MapFS{
		dashboardQuotaUsageFactsIndexMigration: &fstest.MapFile{Data: content},
	}
	require.NoError(t, applyMigrationsFS(ctx, integrationDB, fSys))

	invalid, err = indexIsInvalid(ctx, integrationDB, dashboardQuotaUsageFactsIndex)
	require.NoError(t, err)
	require.False(t, invalid)

	var indexedTable string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
SELECT tbl.relname
FROM pg_class idx
JOIN pg_index i ON i.indexrelid = idx.oid
JOIN pg_class tbl ON tbl.oid = i.indrelid
JOIN pg_namespace ns ON ns.oid = tbl.relnamespace
WHERE ns.nspname = 'public'
  AND idx.relname = $1
`, dashboardQuotaUsageFactsIndex).Scan(&indexedTable))
	require.Equal(t, "usage_facts", indexedTable)
}
