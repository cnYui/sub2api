//go:build unit

package repository

import (
	"strings"
	"testing"

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

	got, err := codec.DecryptMap(stored)
	require.NoError(t, err)
	require.Equal(t, input, got)
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
