package net

import (
	"crypto/sha256"
	"fmt"
)

// AuthMethod defines an authentication method.
type AuthMethod string

const (
	AuthNone  AuthMethod = "none"
	AuthToken AuthMethod = "token"
)

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	Method AuthMethod
	Tokens map[string]string // username → SHA-256(token)
}

// VerifyToken checks if a token is valid for a user.
func (a *AuthConfig) VerifyToken(user, token string) bool {
	if a.Method == AuthNone {
		return true
	}
	expectedHash, ok := a.Tokens[user]
	if !ok {
		return false
	}
	tokenHash := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(token)))
	return tokenHash == expectedHash
}

// TokenHash hashes a token for storage.
func TokenHash(token string) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(token)))
}
