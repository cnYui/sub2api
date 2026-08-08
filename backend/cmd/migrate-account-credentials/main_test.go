package main

import (
	"context"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/securitysecret"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func TestCredentialMigrationInitializesEntRuntimeDefaults(t *testing.T) {
	require.NotNil(t, securitysecret.DefaultCreatedAt)
	require.NotNil(t, securitysecret.DefaultUpdatedAt)
}

func TestCredentialMapNeedsMigration(t *testing.T) {
	tests := []struct {
		name        string
		credentials map[string]any
		want        bool
	}{
		{name: "plain api key", credentials: map[string]any{"api_key": "sk-test"}, want: true},
		{name: "missing fingerprint", credentials: map[string]any{"base_url": "https://example.test"}, want: true},
		{name: "encrypted", credentials: map[string]any{"api_key": "enc:v1:abc", "_credential_fingerprint": "fingerprint", "_api_key_fingerprint": "api-key-fingerprint"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := credentialMapNeedsMigration(tt.credentials); got != tt.want {
				t.Fatalf("credentialMapNeedsMigration() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestPrepareAccountCredentialsMigrationReencryptsDecryptedDocument(t *testing.T) {
	codec, err := repository.NewCredentialCodecHex(strings.Repeat("42", 32), nil)
	require.NoError(t, err)
	stored, err := codec.EncryptMap(map[string]any{"api_key": "sk-test", "base_url": "https://example.test"})
	require.NoError(t, err)

	migrated, err := prepareAccountCredentialsMigration(codec, stored)

	require.NoError(t, err)
	needsMigration, err := codec.MapNeedsMigration(migrated)
	require.NoError(t, err)
	require.False(t, needsMigration)
}

func TestLoadLockedCredentialMigrationBatchUsesTransactionLock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	mock.ExpectQuery(`(?s)SELECT id, credentials FROM accounts.*FOR UPDATE`).
		WithArgs(int64(10), 100).
		WillReturnRows(sqlmock.NewRows([]string{"id", "credentials"}).AddRow(int64(11), []byte(`{"api_key":"sk-test"}`)))

	rows, err := loadLockedCredentialMigrationBatch(context.Background(), client, 10, 100)

	require.NoError(t, err)
	require.Equal(t, []credentialMigrationAccount{{id: 11, credentials: map[string]any{"api_key": "sk-test"}}}, rows)
	require.NoError(t, mock.ExpectationsWereMet())
}
