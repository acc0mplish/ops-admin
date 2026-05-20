package middleware

import (
	"strings"
	"time"

	"ops-admin/backend/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func OperationLog(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if !strings.HasPrefix(c.Request.URL.Path, "/api/v1") {
			return
		}
		if c.Request.Method == "GET" || c.Request.URL.Path == "/api/v1/login" {
			return
		}

		userID, _ := c.Get("userID")
		username, _ := c.Get("username")
		logItem := model.OperationLog{
			Method:      c.Request.Method,
			IP:          c.ClientIP(),
			URL:         c.Request.URL.Path,
			Description: actionDescription(c.Request.URL.Path, c.Request.Method),
			CreatedAt:   time.Now(),
		}
		if id, ok := userID.(uint); ok {
			logItem.AdminID = id
		}
		if name, ok := username.(string); ok {
			logItem.Username = name
		}
		db.Create(&logItem)
	}
}

func actionDescription(path string, method string) string {
	switch {
	case strings.Contains(path, "/admin"):
		return "用户管理"
	case strings.Contains(path, "/role"):
		return "角色管理"
	case strings.Contains(path, "/menu"):
		return "菜单管理"
	case strings.Contains(path, "/dept"):
		return "部门管理"
	case strings.Contains(path, "/post"):
		return "岗位管理"
	case strings.Contains(path, "/sysLoginInfo"):
		return "登录日志管理"
	case strings.Contains(path, "/sysOperationLog"):
		return "操作日志管理"
	default:
		return method + " " + path
	}
}
