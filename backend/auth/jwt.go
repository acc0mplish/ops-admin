package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var secret = []byte("ops-admin-secret")

type Claims struct {
	UserID   uint   `json:"userId"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func GenerateToken(userID uint, username string) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

func ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return claims, nil
}

// TokenErrorMessage returns a user-facing message without exposing JWT details.
func TokenErrorMessage(err error) string {
	if errors.Is(err, jwt.ErrTokenExpired) {
		return "登录已过期，请重新登录"
	}
	return "登录凭证无效，请重新登录"
}
