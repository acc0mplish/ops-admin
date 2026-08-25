package auth

import (
	"testing"
	"time"
)

func TestGenerateTokenUsesConfiguredAccessTTL(t *testing.T) {
	t.Setenv("OPS_ADMIN_JWT_SECRET", "test-only-secret-with-sufficient-length")
	before := time.Now()
	token, expiresAt, err := GenerateToken(7, "admin", "session-123")
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	claims, err := ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken() error = %v", err)
	}
	if claims.UserID != 7 || claims.Username != "admin" || claims.SessionID != "session-123" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
	if expiresAt.Before(before.Add(AccessTokenTTL-time.Second)) || expiresAt.After(before.Add(AccessTokenTTL+time.Second)) {
		t.Fatalf("expiry %v does not match access TTL %v", expiresAt, AccessTokenTTL)
	}
}

func TestOpaqueTokenIsRandomAndOnlyHashIsPersistable(t *testing.T) {
	first, err := NewOpaqueToken()
	if err != nil {
		t.Fatalf("NewOpaqueToken() error = %v", err)
	}
	second, err := NewOpaqueToken()
	if err != nil {
		t.Fatalf("NewOpaqueToken() error = %v", err)
	}
	if first == second || len(first) != 64 {
		t.Fatalf("opaque tokens are not sufficiently random")
	}
	if hash := HashOpaqueToken(first); hash == first || len(hash) != 64 {
		t.Fatalf("unexpected token hash %q", hash)
	}
}
