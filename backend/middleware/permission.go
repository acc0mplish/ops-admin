package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"ops-admin/backend/httpx"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RequirePermission enforces sensitive permissions server-side. The frontend
// directive is only a usability aid and must never be the security boundary.
func RequirePermission(db *gorm.DB, permission string) gin.HandlerFunc {
	return RequireAnyPermission(db, permission)
}

// RequireCreateOrEditPermission selects the permission from the payload ID so
// a role that may create objects cannot use the shared save endpoint to edit
// existing ones (and vice versa).
func RequireCreateOrEditPermission(db *gorm.DB, createPermission, editPermission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
		if err != nil {
			httpx.Failed(c, 400, "invalid request payload")
			c.Abort()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(data))
		var payload struct {
			ID uint `json:"id"`
		}
		permission := createPermission
		if json.Unmarshal(data, &payload) == nil && payload.ID > 0 {
			permission = editPermission
		}
		RequirePermission(db, permission)(c)
	}
}

// AdminHasPermission reports whether an admin holds a granted, enabled route
// permission menu. The authorization SQL exists exactly once (M-1): the
// permission middleware and the console-ticket mint path share this helper.
func AdminHasPermission(db *gorm.DB, adminID uint, permission string) bool {
	permission = strings.TrimSpace(permission)
	if permission == "" {
		return false
	}
	return adminHasAnyPermission(db, adminID, []string{permission})
}

// adminHasAnyPermission is the single authorization query: at least one of
// the given permission strings must be attached, through sys_admin_role and
// sys_role_menu, to a status-1 sys_menu row.
func adminHasAnyPermission(db *gorm.DB, adminID uint, values []string) bool {
	var count int64
	err := db.Table("sys_admin_role ar").
		Joins("JOIN sys_role_menu rm ON rm.role_id=ar.role_id").
		Joins("JOIN sys_menu m ON m.id=rm.menu_id").
		Where("ar.admin_id = ? AND m.value IN ? AND m.menu_status = ?", adminID, values, 1).
		Count(&count).Error
	return err == nil && count > 0
}

// RequireAnyPermission accepts any one of the supplied permissions. This is
// useful for shared read endpoints (for example DNS account options) that are
// consumed by more than one independently-authorized page.
func RequireAnyPermission(db *gorm.DB, permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		values := make([]string, 0, len(permissions))
		for _, permission := range permissions {
			if permission = strings.TrimSpace(permission); permission != "" {
				values = append(values, permission)
			}
		}
		if len(values) == 0 {
			httpx.Failed(c, 403, "Permission denied")
			c.Abort()
			return
		}
		if !adminHasAnyPermission(db, c.GetUint("userID"), values) {
			httpx.Failed(c, 403, "Permission denied")
			c.Abort()
			return
		}
		c.Next()
	}
}
