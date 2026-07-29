package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration180EvolvesReservationsWithoutDroppingHistory(t *testing.T) {
	content, err := FS.ReadFile("180_openai_billing_authorizations.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "ALTER TABLE traffic_credit_reservations RENAME TO billing_authorizations")
	require.Contains(t, sql, "ALTER TABLE traffic_credit_reservation_items RENAME TO billing_authorization_traffic_credit_items")
	require.Contains(t, sql, "billing_source")
	require.Contains(t, sql, "estimate_breakdown")
	require.Contains(t, sql, "estimator_version")
	require.Contains(t, sql, "suspense_usd")
	require.Contains(t, sql, "authorization_id")
	require.Contains(t, sql, "idx_billing_authorizations_subscription_active")
	require.Contains(t, sql, "idx_billing_authorizations_unknown_reconcile")
	require.Contains(t, sql, "'suspense'")
	require.NotContains(t, sql, "DROP TABLE traffic_credit_reservations")
	require.NotContains(t, sql, "DROP TABLE traffic_credit_reservation_items")
}
