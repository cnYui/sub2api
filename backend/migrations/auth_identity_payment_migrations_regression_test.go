package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration112UsesIdempotentAddColumn(t *testing.T) {
	content, err := FS.ReadFile("112_add_payment_order_provider_key_snapshot.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS provider_key VARCHAR(30)")
	require.NotContains(t, sql, "ADD COLUMN provider_key VARCHAR(30);")
}

func TestMigration118DoesNotForceOverwriteAuthSourceGrantDefaults(t *testing.T) {
	content, err := FS.ReadFile("118_wechat_dual_mode_and_auth_source_defaults.sql")
	require.NoError(t, err)

	sql := string(content)
	require.NotContains(t, sql, "UPDATE settings")
	require.NotContains(t, sql, "SET value = 'false'")
	require.True(t, strings.Contains(sql, "ON CONFLICT (key) DO NOTHING"))
	require.Contains(t, sql, "THEN ''")
}

func TestAuthIdentityReportTypeWideningRunsBeforeLongReportWritersAndStillReconcilesAt121(t *testing.T) {
	preflightContent, err := FS.ReadFile("108a_widen_auth_identity_migration_report_type.sql")
	require.NoError(t, err)

	preflightSQL := string(preflightContent)
	require.Contains(t, preflightSQL, "ALTER TABLE auth_identity_migration_reports")
	require.Contains(t, preflightSQL, "ALTER COLUMN report_type TYPE VARCHAR(80)")

	content, err := FS.ReadFile("109_auth_identity_compat_backfill.sql")
	require.NoError(t, err)

	sql := string(content)
	require.NotContains(t, sql, "ALTER TABLE auth_identity_migration_reports")

	followupContent, err := FS.ReadFile("121_auth_identity_migration_report_type_widen.sql")
	require.NoError(t, err)

	followupSQL := string(followupContent)
	require.Contains(t, followupSQL, "ALTER TABLE auth_identity_migration_reports")
	require.Contains(t, followupSQL, "ALTER COLUMN report_type TYPE VARCHAR(80)")
}

func TestMigration119DefersPaymentIndexRolloutToOnlineFollowup(t *testing.T) {
	content, err := FS.ReadFile("119_enforce_payment_orders_out_trade_no_unique.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "120_enforce_payment_orders_out_trade_no_unique_notx.sql")
	require.Contains(t, sql, "NULL;")
	require.NotContains(t, sql, "CREATE UNIQUE INDEX")
	require.NotContains(t, sql, "DROP INDEX")

	followupContent, err := FS.ReadFile("120_enforce_payment_orders_out_trade_no_unique_notx.sql")
	require.NoError(t, err)

	followupSQL := string(followupContent)
	require.Contains(t, followupSQL, "explicit duplicate out_trade_no precheck")
	require.Contains(t, followupSQL, "stale invalid paymentorder_out_trade_no_unique index")
	require.Contains(t, followupSQL, "CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS paymentorder_out_trade_no_unique")
	require.NotContains(t, followupSQL, "DROP INDEX CONCURRENTLY IF EXISTS paymentorder_out_trade_no_unique")
	require.Contains(t, followupSQL, "DROP INDEX CONCURRENTLY IF EXISTS paymentorder_out_trade_no")
	require.Contains(t, followupSQL, "WHERE out_trade_no <> ''")

	alignmentContent, err := FS.ReadFile("120a_align_payment_orders_out_trade_no_index_name.sql")
	require.NoError(t, err)

	alignmentSQL := string(alignmentContent)
	require.Contains(t, alignmentSQL, "paymentorder_out_trade_no_unique")
	require.Contains(t, alignmentSQL, "RENAME TO paymentorder_out_trade_no")
}

func TestMigration110SeedsAuthSourceSignupGrantsDisabledByDefault(t *testing.T) {
	content, err := FS.ReadFile("110_pending_auth_and_provider_default_grants.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "('auth_source_default_email_grant_on_signup', 'false')")
	require.Contains(t, sql, "('auth_source_default_linuxdo_grant_on_signup', 'false')")
	require.Contains(t, sql, "('auth_source_default_oidc_grant_on_signup', 'false')")
	require.Contains(t, sql, "('auth_source_default_wechat_grant_on_signup', 'false')")
	require.NotContains(t, sql, "('auth_source_default_email_grant_on_signup', 'true')")
}

func TestMigration158EnablesAffiliateFeature(t *testing.T) {
	content, err := FS.ReadFile("158_enable_affiliate_default.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "affiliate_enabled")
	require.Contains(t, sql, "value = 'true'")
	require.Contains(t, sql, "ON CONFLICT (key) DO UPDATE")
}

func TestMigration122ScrubsPendingOAuthCompletionTokensAtRest(t *testing.T) {
	content, err := FS.ReadFile("122_pending_auth_completion_token_cleanup.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "UPDATE pending_auth_sessions")
	require.Contains(t, sql, "completion_response")
	require.Contains(t, sql, "access_token")
	require.Contains(t, sql, "refresh_token")
	require.Contains(t, sql, "expires_in")
	require.Contains(t, sql, "token_type")
}

func TestMigration123BackfillsLegacyAuthSourceGrantDefaultsSafely(t *testing.T) {
	content, err := FS.ReadFile("123_fix_legacy_auth_source_grant_on_signup_defaults.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "110_pending_auth_and_provider_default_grants.sql")
	require.Contains(t, sql, "schema_migrations")
	require.Contains(t, sql, "updated_at")
	require.Contains(t, sql, "'_grant_on_signup'")
	require.Contains(t, sql, "value = 'false'")
	require.Contains(t, sql, "auth_identity_migration_reports")
}

func TestMigration124BackfillsLegacyOIDCSecurityFlagsSafely(t *testing.T) {
	content, err := FS.ReadFile("124_backfill_legacy_oidc_security_flags.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "oidc_connect_use_pkce")
	require.Contains(t, sql, "oidc_connect_validate_id_token")
	require.Contains(t, sql, "ON CONFLICT (key) DO NOTHING")
	require.Contains(t, sql, "oidc_connect_enabled")
	require.Contains(t, sql, "'false'")
}

func TestMigration134AddsAffiliateLedgerAuditFieldsWithoutJSONCast(t *testing.T) {
	content, err := FS.ReadFile("134_affiliate_ledger_audit_snapshots.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS source_order_id BIGINT")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS balance_after DECIMAL(20,8)")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS aff_quota_after DECIMAL(20,8)")
	require.Contains(t, sql, "substring(")
	require.Contains(t, sql, `"rebateAmount"`)
	require.Contains(t, sql, "COUNT(*) OVER (PARTITION BY ra.order_id) AS order_match_count")
	require.Contains(t, sql, "COUNT(*) OVER (PARTITION BY ual.id) AS ledger_match_count")
	require.NotContains(t, sql, "detail::jsonb")
}

func TestMigration135AllowsGitHubAndGoogleAuthProviders(t *testing.T) {
	content, err := FS.ReadFile("135_allow_email_oauth_provider_types.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "users_signup_source_check")
	require.Contains(t, sql, "auth_identities_provider_type_check")
	require.Contains(t, sql, "auth_identity_channels_provider_type_check")
	require.Contains(t, sql, "pending_auth_sessions_provider_type_check")
	require.Contains(t, sql, "'github'")
	require.Contains(t, sql, "'google'")
}

func TestMigration151AddsAccountAutoPauseExpiryPartialIndex(t *testing.T) {
	content, err := FS.ReadFile("151_account_autopause_expiry_index_notx.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_accounts_autopause_expiry_due")
	require.Contains(t, sql, "ON accounts (expires_at)")
	require.Contains(t, sql, "WHERE deleted_at IS NULL")
	require.Contains(t, sql, "schedulable = TRUE")
	require.Contains(t, sql, "auto_pause_on_expired = TRUE")
	require.Contains(t, sql, "expires_at IS NOT NULL")
}

func TestMigration154SeedsCodex99SubscriptionPlanWithoutAccountBinding(t *testing.T) {
	content, err := FS.ReadFile("154_seed_codex_99_subscription_plan.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "codex-pool-89-usd")
	require.Contains(t, sql, "99 元订阅池")
	require.Contains(t, sql, "daily_limit_usd = 89")
	require.Contains(t, sql, "price = 99.00")
	require.Contains(t, sql, "image_price_1k = 0.10")
	require.Contains(t, sql, "image_price_2k = 0.20")
	require.Contains(t, sql, "image_price_4k = 0.40")
	require.NotContains(t, sql, "INSERT INTO account_groups")
	require.NotContains(t, sql, "UPDATE account_groups")
}

func TestMigration155SeedsCodexSubscriptionPlanBaselineWithoutAccountBinding(t *testing.T) {
	content, err := FS.ReadFile("155_seed_codex_subscription_plans_baseline.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "codex-pool-19-usd")
	require.Contains(t, sql, "codex-pool-29-usd")
	require.Contains(t, sql, "codex-pool-49-usd")
	require.Contains(t, sql, "codex-pool-89-usd")
	require.Contains(t, sql, "'29 元订阅池'::TEXT, 29.00::NUMERIC, 19::NUMERIC")
	require.Contains(t, sql, "'39 元订阅池'::TEXT, 39.00::NUMERIC, 29::NUMERIC")
	require.Contains(t, sql, "'59 元订阅池'::TEXT, 59.00::NUMERIC, 49::NUMERIC")
	require.Contains(t, sql, "'99 元订阅池'::TEXT, 99.00::NUMERIC, 89::NUMERIC")
	require.Contains(t, sql, "WHERE NOT EXISTS")
	require.Contains(t, sql, "for_sale = TRUE")
	require.NotContains(t, sql, "INSERT INTO account_groups")
	require.NotContains(t, sql, "UPDATE account_groups")
}

func TestMigration156SeedsCodex79SubscriptionPlanWithoutAccountBinding(t *testing.T) {
	content, err := FS.ReadFile("156_seed_codex_79_subscription_plan.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "codex-pool-69-usd")
	require.Contains(t, sql, "79 元订阅池")
	require.Contains(t, sql, "daily_limit_usd = 69")
	require.Contains(t, sql, "price = 79.00")
	require.Contains(t, sql, "image_price_1k = 0.10")
	require.Contains(t, sql, "image_price_2k = 0.20")
	require.Contains(t, sql, "image_price_4k = 0.40")
	require.NotContains(t, sql, "INSERT INTO account_groups")
	require.NotContains(t, sql, "UPDATE account_groups")
}

func TestMigration157FixesCodex79SubscriptionPlanBasePriceWithoutAccountBinding(t *testing.T) {
	content, err := FS.ReadFile("157_fix_codex_79_subscription_plan_base_price.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "codex-pool-69-usd")
	require.Contains(t, sql, "79 元订阅池")
	require.Contains(t, sql, "price = 79.00")
	require.Contains(t, sql, "price = 79.79")
	require.NotContains(t, sql, "INSERT INTO account_groups")
	require.NotContains(t, sql, "UPDATE account_groups")
}
