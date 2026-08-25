package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	AccessTokenTTL = 60 * time.Minute
	SessionIdleTTL = 6 * time.Hour
	SessionMaxTTL  = 7 * 24 * time.Hour
)

func signingSecret() []byte {
	if value := strings.TrimSpace(os.Getenv("OPS_ADMIN_JWT_SECRET")); value != "" {
		return []byte(value)
	}
	return []byte("ops-admin-secret")
}

type Claims struct {
	UserID    uint   `json:"userId"`
	Username  string `json:"username"`
	SessionID string `json:"sessionId"`
	jwt.RegisteredClaims
}

func GenerateToken(userID uint, username, sessionID string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(AccessTokenTTL)
	claims := Claims{
		UserID:    userID,
		Username:  username,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(signingSecret())
	return signed, expiresAt, err
}

func ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, jwt.ErrSignatureInvalid
		}
		return signingSecret(), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return claims, nil
}

func NewOpaqueToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func HashOpaqueToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

// TokenErrorMessage returns a user-facing message without exposing JWT details.
func TokenErrorMessage(err error) string {
	if errors.Is(err, jwt.ErrTokenExpired) {
		return "登录已过期，请重新登录"
	}
	return "登录凭证无效，请重新登录"
}
