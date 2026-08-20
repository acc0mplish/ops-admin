package middleware

import (
	"strings"

	"ops-admin/backend/auth"
	"ops-admin/backend/httpx"

	"github.com/gin-gonic/gin"
)

func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			httpx.Failed(c, 401, "请先登录")
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(header, "Bearer ")
		claims, err := auth.ParseToken(tokenString)
		if err != nil {
			httpx.Failed(c, 401, auth.TokenErrorMessage(err))
			c.Abort()
			return
		}

		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Next()
	}
}
