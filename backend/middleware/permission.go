package middleware

import (
	"ops-admin/backend/httpx"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RequirePermission enforces sensitive permissions server-side. The frontend
// directive is only a usability aid and must never be the security boundary.
func RequirePermission(db *gorm.DB, permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var count int64
		err := db.Table("sys_admin_role ar").
			Joins("JOIN sys_role_menu rm ON rm.role_id=ar.role_id").
			Joins("JOIN sys_menu m ON m.id=rm.menu_id").
			Where("ar.admin_id = ? AND m.value = ? AND m.menu_status = ?", c.GetUint("userID"), permission, 1).
			Count(&count).Error
		if err != nil || count == 0 {
			httpx.Failed(c, 403, "没有执行该证书操作的权限")
			c.Abort()
			return
		}
		c.Next()
	}
}
