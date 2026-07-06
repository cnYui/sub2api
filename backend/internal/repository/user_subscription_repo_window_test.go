//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestRefreshExpiredUsageWindows_UsesConditionalWindowUpdate(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(drv))
	t.Cleanup(func() {
		_ = client.Close()
	})

	repo := &userSubscriptionRepository{client: client}
	dailyStart := time.Date(2026, 7, 6, 0, 0, 0, 0, time.FixedZone("CST", 8*3600))
	weeklyStart := time.Date(2026, 7, 6, 0, 0, 0, 0, time.FixedZone("CST", 8*3600))
	monthlyStart := time.Date(2026, 7, 6, 10, 0, 0, 0, time.FixedZone("CST", 8*3600))
	now := monthlyStart

	mock.ExpectExec(`(?s)UPDATE user_subscriptions.*daily_window_start < \$2.*weekly_window_start < \$3.*monthly_window_start \+ INTERVAL '30 days' <= \$5.*WHERE id = \$1.*deleted_at IS NULL`).
		WithArgs(int64(101), dailyStart, weeklyStart, monthlyStart, now).
		WillReturnResult(sqlmock.NewResult(0, 1))

	updated, err := repo.RefreshExpiredUsageWindows(ctx, 101, dailyStart, weeklyStart, monthlyStart, now)

	require.NoError(t, err)
	require.True(t, updated)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshExpiredUsageWindows_ReturnsFalseWhenNoWindowUpdated(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(drv))
	t.Cleanup(func() {
		_ = client.Close()
	})

	repo := &userSubscriptionRepository{client: client}
	now := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)

	mock.ExpectExec(`(?s)UPDATE user_subscriptions`).
		WithArgs(int64(101), now, now, now, now).
		WillReturnResult(sqlmock.NewResult(0, 0))

	updated, err := repo.RefreshExpiredUsageWindows(ctx, 101, now, now, now, now)

	require.NoError(t, err)
	require.False(t, updated)
	require.NoError(t, mock.ExpectationsWereMet())
}
