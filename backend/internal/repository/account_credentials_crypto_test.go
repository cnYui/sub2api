//go:build unit

package repository

import (
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCredentialCodecEncryptsOnlySensitiveValues(t *testing.T) {
	codec, err := NewCredentialCodecHex(aesHexKey(32, 0x42), nil)
	require.NoError(t, err)
	input := map[string]any{
		"api_key":       "sk-test",
		"refresh_token": "refresh-test",
		"base_url":      "https://example.test",
		"model_mapping": map[string]any{
			"m": "m",
		},
	}

	stored, err := codec.EncryptMap(input)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(stored["api_key"].(string), credentialCiphertextPrefix))
	require.True(t, strings.HasPrefix(stored["refresh_token"].(string), credentialCiphertextPrefix))
	require.Equal(t, input["base_url"], stored["base_url"])
	require.NotEqual(t, "sk-test", stored["api_key"])
	storedAgain, err := codec.EncryptMap(input)
	require.NoError(t, err)
	require.NotEqual(t, stored["api_key"], storedAgain["api_key"])
	require.Equal(t, stored[credentialFingerprintKey], storedAgain[credentialFingerprintKey])
	require.NotEmpty(t, stored[credentialAPIKeyFingerprintKey])
	require.Equal(t, stored[credentialAPIKeyFingerprintKey], storedAgain[credentialAPIKeyFingerprintKey])
	require.NotEqual(t, "sk-test", stored[credentialAPIKeyFingerprintKey])

	got, err := codec.DecryptMap(stored)
	require.NoError(t, err)
	require.Equal(t, input, got)
}

func TestCredentialCodecEncryptsEveryServiceSensitiveCredentialKey(t *testing.T) {
	codec, err := NewCredentialCodecHex(aesHexKey(32, 0x42), nil)
	require.NoError(t, err)
	for _, key := range service.SensitiveCredentialKeys {
		t.Run(key, func(t *testing.T) {
			stored, err := codec.EncryptMap(map[string]any{key: "secret-value"})
			require.NoError(t, err)
			require.True(t, strings.HasPrefix(stored[key].(string), credentialCiphertextPrefix))
			require.NotContains(t, stored[key], "secret-value")
		})
	}
}

func TestCredentialCodecEncryptsEmptyCredentialMap(t *testing.T) {
	codec, err := NewCredentialCodecHex(aesHexKey(32, 0x42), nil)
	require.NoError(t, err)

	stored, err := codec.EncryptMap(nil)
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.NotEmpty(t, stored[credentialFingerprintKey])

	got, err := codec.DecryptMap(stored)
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestCredentialCodecRejectsTamperedCiphertext(t *testing.T) {
	codec, err := NewCredentialCodecHex(aesHexKey(32, 0x42), nil)
	require.NoError(t, err)
	stored, err := codec.EncryptMap(map[string]any{"api_key": "sk-test"})
	require.NoError(t, err)
	stored["api_key"] = stored["api_key"].(string) + "tampered"

	_, err = codec.DecryptMap(stored)
	require.Error(t, err)
}

func TestCredentialMapNeedsMigrationWhenAPIKeyFingerprintIsMissing(t *testing.T) {
	codec, err := NewCredentialCodecHex(aesHexKey(32, 0x42), nil)
	require.NoError(t, err)

	stored, err := codec.EncryptMap(map[string]any{"api_key": "sk-test"})
	require.NoError(t, err)
	delete(stored, credentialAPIKeyFingerprintKey)

	require.True(t, CredentialMapNeedsMigration(stored))
}

func TestCredentialCodecMapNeedsMigrationRejectsInvalidCiphertext(t *testing.T) {
	codec, err := NewCredentialCodecHex(aesHexKey(32, 0x42), nil)
	require.NoError(t, err)
	credentials := map[string]any{
		"api_key":                      credentialCiphertextPrefix + "not-base64",
		credentialFingerprintKey:       "fingerprint",
		credentialAPIKeyFingerprintKey: "api-key-fingerprint",
	}

	_, err = codec.MapNeedsMigration(credentials)

	require.Error(t, err)
}

func TestCredentialCodecMapNeedsMigrationRejectsInvalidCiphertextWithoutFingerprint(t *testing.T) {
	codec, err := NewCredentialCodecHex(aesHexKey(32, 0x42), nil)
	require.NoError(t, err)

	_, err = codec.MapNeedsMigration(map[string]any{"api_key": credentialCiphertextPrefix + "not-base64"})

	require.Error(t, err)
}

func TestAccountRepositoryCredentialBoundaryReturnsPlaintextOnlyAfterRead(t *testing.T) {
	codec, err := NewCredentialCodecHex(aesHexKey(32, 0x42), nil)
	require.NoError(t, err)
	repo := &accountRepository{credentialCodec: codec}

	stored, err := repo.encryptCredentials(map[string]any{"api_key": "sk-test", "base_url": "https://example.test"})
	require.NoError(t, err)
	require.NotContains(t, stored["api_key"], "sk-test")

	plain, err := repo.decryptCredentials(stored)
	require.NoError(t, err)
	require.Equal(t, "sk-test", plain["api_key"])
}

func TestAccountRepositoryStorageCopyDoesNotReplaceCallerCredentials(t *testing.T) {
	codec, err := NewCredentialCodecHex(aesHexKey(32, 0x42), nil)
	require.NoError(t, err)
	repo := &accountRepository{credentialCodec: codec}
	account := service.Account{Credentials: map[string]any{"api_key": "sk-test"}}

	stored, err := repo.accountForStorage(&account)
	require.NoError(t, err)
	require.Equal(t, "sk-test", account.Credentials["api_key"])
	require.NotEqual(t, "sk-test", stored.Credentials["api_key"])
}

func TestAccountRepositoryCredentialCASUsesStableFingerprint(t *testing.T) {
	codec, err := NewCredentialCodecHex(aesHexKey(32, 0x42), nil)
	require.NoError(t, err)
	repo := &accountRepository{credentialCodec: codec}
	first, err := repo.encryptCredentials(map[string]any{"api_key": "sk-test"})
	require.NoError(t, err)
	second, err := repo.encryptCredentials(map[string]any{"api_key": "sk-test"})
	require.NoError(t, err)

	firstArg, err := repo.credentialCASArg(first)
	require.NoError(t, err)
	secondArg, err := repo.credentialCASArg(second)
	require.NoError(t, err)
	require.Equal(t, firstArg, secondArg)
	require.Equal(t, "a.credentials ->> '_credential_fingerprint' = $7", repo.credentialCASCondition("a.credentials", "$7"))
}

func TestAccountRepositoryCredentialChangeUsesStableFingerprint(t *testing.T) {
	codec, err := NewCredentialCodecHex(aesHexKey(32, 0x42), nil)
	require.NoError(t, err)
	repo := &accountRepository{credentialCodec: codec}

	require.Equal(t,
		"credentials ->> '_credential_fingerprint' IS DISTINCT FROM $1::jsonb ->> '_credential_fingerprint'",
		repo.credentialChangedCondition("credentials", "$1::jsonb"),
	)
}

func TestMergeAccountCredentialsKeepsStoredValuesAndAppliesPatch(t *testing.T) {
	stored := map[string]any{"api_key": "old", "base_url": "https://old.example", "plan_type": "pro"}
	patch := map[string]any{"api_key": "new"}

	got := mergeAccountCredentials(stored, patch)

	require.Equal(t, map[string]any{"api_key": "new", "base_url": "https://old.example", "plan_type": "pro"}, got)
	require.Equal(t, "old", stored["api_key"])
}
