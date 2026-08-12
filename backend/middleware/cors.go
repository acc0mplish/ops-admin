package middleware

import (
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

func allowedOrigins() map[string]struct{} {
	origins := make(map[string]struct{})
	for _, origin := range strings.Split(os.Getenv("OPS_ADMIN_CORS_ORIGINS"), ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			origins[origin] = struct{}{}
		}
	}
	return origins
}

func CORS() gin.HandlerFunc {
	origins := allowedOrigins()
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Header("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")

		if origin := c.GetHeader("Origin"); origin != "" {
			if _, allowed := origins[origin]; allowed {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
				c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				c.Header("Vary", "Origin")
			}
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
