package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

var credentialSeed struct {
	sync.RWMutex
	value string
}

// ConfigureCredentialKey sets the application-wide encryption seed loaded
// from config.yaml. It must be called before encrypted credentials are read.
func ConfigureCredentialKey(value string) {
	credentialSeed.Lock()
	credentialSeed.value = strings.TrimSpace(value)
	credentialSeed.Unlock()
}

func credentialKey() []byte {
	credentialSeed.RLock()
	seed := credentialSeed.value
	credentialSeed.RUnlock()
	// Environment variables remain a compatibility fallback for existing
	// deployments; config.yaml takes precedence when credential-key is set.
	if seed == "" {
		seed = strings.TrimSpace(os.Getenv("OPS_ADMIN_CREDENTIAL_KEY"))
	}
	if seed == "" {
		seed = strings.TrimSpace(os.Getenv("OPS_ADMIN_JWT_SECRET"))
	}
	if seed == "" {
		seed = "ops-admin-development-credential-key"
	}
	digest := sha256.Sum256([]byte(seed))
	return digest[:]
}

func EncryptSecret(value string) (string, error) {
	block, err := aes.NewCipher(credentialKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(value), nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func DecryptSecret(value string) (string, error) {
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return "", fmt.Errorf("decode encrypted credential: %w", err)
	}
	block, err := aes.NewCipher(credentialKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", fmt.Errorf("encrypted credential is truncated")
	}
	plain, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt credential: %w", err)
	}
	return string(plain), nil
}
