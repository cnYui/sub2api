// Package repository 提供账号凭证的加密存储边界。
package repository

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const credentialCiphertextPrefix = "enc:v1:"

var sensitiveCredentialKeys = map[string]struct{}{
	"api_key":       {},
	"access_token":  {},
	"refresh_token": {},
	"id_token":      {},
	"client_secret": {},
	"private_key":   {},
	"cookie":        {},
	"cookies":       {},
	"setup_token":   {},
	"session_token": {},
	"token":         {},
}

type credentialCodec struct {
	activeKey  []byte
	legacyKeys [][]byte
}

func NewCredentialCodecHex(activeKeyHex string, legacyKeyHex []string) (*credentialCodec, error) {
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
	return &credentialCodec{activeKey: active, legacyKeys: legacy}, nil
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

func (c *credentialCodec) EncryptMap(input map[string]any) (map[string]any, error) {
	if c == nil || len(c.activeKey) == 0 {
		return nil, fmt.Errorf("account credentials codec is not configured")
	}
	output := cloneCredentialMap(input)
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
	return output, nil
}

func (c *credentialCodec) DecryptMap(input map[string]any) (map[string]any, error) {
	if c == nil {
		return nil, fmt.Errorf("account credentials codec is not configured")
	}
	output := cloneCredentialMap(input)
	for key, value := range output {
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
	return output, nil
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
