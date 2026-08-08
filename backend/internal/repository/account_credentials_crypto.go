// Package repository 提供账号凭证的加密存储边界。
package repository

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	credentialCiphertextPrefix     = "enc:v1:"
	credentialFingerprintKey       = "_credential_fingerprint"
	credentialAPIKeyFingerprintKey = "_api_key_fingerprint"
)

var sensitiveCredentialKeys = func() map[string]struct{} {
	keys := map[string]struct{}{
		"client_secret": {},
		"cookies":       {},
		"setup_token":   {},
		"session_token": {},
		"token":         {},
	}
	for _, key := range service.SensitiveCredentialKeys {
		keys[key] = struct{}{}
	}
	return keys
}()

type CredentialCodec struct {
	activeKey  []byte
	legacyKeys [][]byte
}

func NewCredentialCodecFromConfig(cfg *config.Config) (*CredentialCodec, error) {
	if cfg == nil {
		return nil, fmt.Errorf("account credentials encryption configuration is not configured")
	}
	return NewCredentialCodecHex(cfg.AccountCredentials.EncryptionKey, nil)
}

func NewCredentialCodecHex(activeKeyHex string, legacyKeyHex []string) (*CredentialCodec, error) {
	active, err := decodeCredentialKey(activeKeyHex)
	if err != nil {
		return nil, err
	}
	legacy := make([][]byte, 0, len(legacyKeyHex))
	for _, raw := range legacyKeyHex {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		key, err := decodeCredentialKey(raw)
		if err != nil {
			return nil, err
		}
		legacy = append(legacy, key)
	}
	return &CredentialCodec{activeKey: active, legacyKeys: legacy}, nil
}

func decodeCredentialKey(raw string) ([]byte, error) {
	key, err := hex.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid account credentials encryption key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("account credentials encryption key must be 32 bytes (64 hex chars), got %d bytes", len(key))
	}
	return key, nil
}

func (c *CredentialCodec) EncryptMap(input map[string]any) (map[string]any, error) {
	if c == nil || len(c.activeKey) == 0 {
		return nil, fmt.Errorf("account credentials codec is not configured")
	}
	output := cloneCredentialMap(input)
	if output == nil {
		output = make(map[string]any, 1)
	}
	delete(output, credentialFingerprintKey)
	delete(output, credentialAPIKeyFingerprintKey)
	fingerprint, err := c.FingerprintMap(input)
	if err != nil {
		return nil, err
	}
	for key, value := range output {
		if _, sensitive := sensitiveCredentialKeys[key]; !sensitive || value == nil {
			continue
		}
		if text, ok := value.(string); ok && strings.HasPrefix(text, credentialCiphertextPrefix) {
			continue
		}
		plain, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("marshal credential %q: %w", key, err)
		}
		ciphertext, err := encryptCredentialBytes(c.activeKey, plain)
		if err != nil {
			return nil, fmt.Errorf("encrypt credential %q: %w", key, err)
		}
		output[key] = credentialCiphertextPrefix + base64.StdEncoding.EncodeToString(ciphertext)
	}
	output[credentialFingerprintKey] = fingerprint
	if apiKey, ok := input["api_key"].(string); ok && apiKey != "" {
		output[credentialAPIKeyFingerprintKey] = c.APIKeyFingerprint(apiKey)
	}
	return output, nil
}

func (c *CredentialCodec) DecryptMap(input map[string]any) (map[string]any, error) {
	if c == nil {
		return nil, fmt.Errorf("account credentials codec is not configured")
	}
	output := cloneCredentialMap(input)
	for key, value := range output {
		if key == credentialFingerprintKey || key == credentialAPIKeyFingerprintKey {
			continue
		}
		if _, sensitive := sensitiveCredentialKeys[key]; !sensitive || value == nil {
			continue
		}
		text, ok := value.(string)
		if !ok || !strings.HasPrefix(text, credentialCiphertextPrefix) {
			continue
		}
		encoded := strings.TrimPrefix(text, credentialCiphertextPrefix)
		ciphertext, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("decode credential %q: %w", key, err)
		}
		plain, err := decryptCredentialBytes(c.activeKey, ciphertext)
		if err != nil {
			for _, legacyKey := range c.legacyKeys {
				plain, err = decryptCredentialBytes(legacyKey, ciphertext)
				if err == nil {
					break
				}
			}
		}
		if err != nil {
			return nil, fmt.Errorf("decrypt credential %q: %w", key, err)
		}
		var decoded any
		if err := json.Unmarshal(plain, &decoded); err != nil {
			return nil, fmt.Errorf("unmarshal credential %q: %w", key, err)
		}
		output[key] = decoded
	}
	delete(output, credentialFingerprintKey)
	delete(output, credentialAPIKeyFingerprintKey)
	return output, nil
}

func (c *CredentialCodec) APIKeyFingerprint(apiKey string) string {
	mac := hmac.New(sha256.New, c.activeKey)
	_, _ = mac.Write([]byte("api_key\x00"))
	_, _ = mac.Write([]byte(apiKey))
	return hex.EncodeToString(mac.Sum(nil))
}

func (c *CredentialCodec) FingerprintMap(input map[string]any) (string, error) {
	if c == nil || len(c.activeKey) == 0 {
		return "", fmt.Errorf("account credentials codec is not configured")
	}
	canonical := cloneCredentialMap(input)
	delete(canonical, credentialFingerprintKey)
	delete(canonical, credentialAPIKeyFingerprintKey)
	payload, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("marshal account credentials fingerprint: %w", err)
	}
	mac := hmac.New(sha256.New, c.activeKey)
	_, _ = mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func CredentialMapNeedsMigration(credentials map[string]any) bool {
	if fingerprint, ok := credentials[credentialFingerprintKey].(string); !ok || fingerprint == "" {
		return true
	}
	for key := range sensitiveCredentialKeys {
		value, exists := credentials[key]
		if !exists || value == nil {
			continue
		}
		text, ok := value.(string)
		if !ok || !strings.HasPrefix(text, credentialCiphertextPrefix) {
			return true
		}
	}
	if apiKey, exists := credentials["api_key"]; exists && apiKey != nil {
		if fingerprint, ok := credentials[credentialAPIKeyFingerprintKey].(string); !ok || fingerprint == "" {
			return true
		}
	}
	return false
}

func (c *CredentialCodec) MapNeedsMigration(credentials map[string]any) (bool, error) {
	hasCiphertext := false
	for key := range sensitiveCredentialKeys {
		if value, ok := credentials[key].(string); ok && strings.HasPrefix(value, credentialCiphertextPrefix) {
			hasCiphertext = true
			break
		}
	}
	var decrypted map[string]any
	if hasCiphertext {
		var err error
		decrypted, err = c.DecryptMap(credentials)
		if err != nil {
			return false, err
		}
	}
	if CredentialMapNeedsMigration(credentials) {
		return true, nil
	}
	if !hasCiphertext {
		return false, nil
	}
	fingerprint, err := c.FingerprintMap(decrypted)
	if err != nil {
		return false, err
	}
	if credentials[credentialFingerprintKey] != fingerprint {
		return true, nil
	}
	if apiKey, ok := decrypted["api_key"].(string); ok && apiKey != "" &&
		credentials[credentialAPIKeyFingerprintKey] != c.APIKeyFingerprint(apiKey) {
		return true, nil
	}
	return false, nil
}

func cloneCredentialMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func encryptCredentialBytes(key, plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plain, nil), nil
}

func decryptCredentialBytes(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}
	return gcm.Open(nil, ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():], nil)
}
