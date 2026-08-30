package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	APIKeyPrefix      = "pl_live_"
	apiKeySecretBytes = 32
	APIKeyPrefixLen   = 16
)

func GenerateAPIKey() (raw, prefix, hash string, err error) {
	secret := make([]byte, apiKeySecretBytes)
	if _, err = rand.Read(secret); err != nil {
		return "", "", "", fmt.Errorf("api key entropy: %w", err)
	}
	raw = APIKeyPrefix + hex.EncodeToString(secret)
	prefix = DisplayPrefix(raw)
	hash = HashAPIKey(raw)
	return raw, prefix, hash, nil
}

func HashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func DisplayPrefix(raw string) string {
	if len(raw) < APIKeyPrefixLen {
		return raw
	}
	return raw[:APIKeyPrefixLen]
}

func LooksLikeAPIKey(raw string) bool {
	return strings.HasPrefix(strings.TrimSpace(raw), APIKeyPrefix) && len(raw) > len(APIKeyPrefix)+8
}
