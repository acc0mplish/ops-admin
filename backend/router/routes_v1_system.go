package router

import (
	"ops-admin/backend/controller"
	"ops-admin/backend/opdef"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// registerSystem mounts the system-domain v1 routes on the authenticated api
// group: profile, systemConfig, system/ldap, console-sessions, admin
// (incl. admin/ldap), role, menu, dept, post, sysLoginInfo, sysOperationLog
// and notify. Phase A pure move from router.go — route statements are
// verbatim; only the group receiver name and one indentation level changed.
// Group construction and the group-level middlewares stay in router.go
// (plan §J1).
func registerSystem(g *gin.RouterGroup, db *gorm.DB, ctl *controller.Controller) {
	g.GET("/profile", ctl.Profile)

	g.GET("/systemConfig", ctl.GetSystemConfig)
	g.PUT("/systemConfig", opdef.Middleware(db, opdef.Must("PUT", "/systemConfig")), ctl.UpdateSystemConfig)
	g.POST("/systemConfig/upload", opdef.Middleware(db, opdef.Must("POST", "/systemConfig/upload")), ctl.UploadSystemAsset)
	g.GET("/system/ldap/config", ctl.GetLDAPConfig)
	g.PUT("/system/ldap/config", opdef.Middleware(db, opdef.Must("PUT", "/system/ldap/config")), ctl.SaveLDAPConfig)
	g.POST("/system/ldap/test", opdef.Middleware(db, opdef.Must("POST", "/system/ldap/test")), ctl.TestLDAPConfig)
	g.POST("/console-sessions", opdef.Middleware(db, opdef.Must("POST", "/console-sessions")), ctl.CreateConsoleSession)

	g.GET("/admin/ldap/users", ctl.PreviewLDAPUsers)
	g.POST("/admin/ldap/sync", opdef.Middleware(db, opdef.Must("POST", "/admin/ldap/sync")), ctl.SyncLDAPUsers)

	g.GET("/admin/list", opdef.Middleware(db, opdef.Must("GET", "/admin/list")), ctl.GetSysAdminList)
	g.GET("/admin/info", opdef.Middleware(db, opdef.Must("GET", "/admin/info")), ctl.GetSysAdminInfo)
	g.POST("/admin/add", opdef.Middleware(db, opdef.Must("POST", "/admin/add")), ctl.CreateSysAdmin)
	g.PUT("/admin/update", opdef.Middleware(db, opdef.Must("PUT", "/admin/update")), ctl.UpdateSysAdmin)
	g.DELETE("/admin/delete", opdef.Middleware(db, opdef.Must("DELETE", "/admin/delete")), ctl.DeleteSysAdmin)
	g.PUT("/admin/updateStatus", opdef.Middleware(db, opdef.Must("PUT", "/admin/updateStatus")), ctl.UpdateSysAdminStatus)
	g.PUT("/admin/updatePassword", opdef.Middleware(db, opdef.Must("PUT", "/admin/updatePassword")), ctl.ResetSysAdminPassword)
	g.PUT("/admin/updatePersonal", opdef.Middleware(db, opdef.Must("PUT", "/admin/updatePersonal")), ctl.UpdatePersonal)
	g.PUT("/admin/updatePersonalPassword", opdef.Middleware(db, opdef.Must("PUT", "/admin/updatePersonalPassword")), ctl.UpdatePersonalPassword)

	g.GET("/role/list", ctl.GetRoleList)
	g.GET("/role/vo/list", ctl.QuerySysRoleVoList)
	g.GET("/role/info", ctl.GetRoleInfo)
	g.POST("/role/add", opdef.Middleware(db, opdef.Must("POST", "/role/add")), ctl.CreateRole)
	g.PUT("/role/update", opdef.Middleware(db, opdef.Must("PUT", "/role/update")), ctl.UpdateRole)
	g.DELETE("/role/delete", opdef.Middleware(db, opdef.Must("DELETE", "/role/delete")), ctl.DeleteRole)
	g.PUT("/role/updateStatus", opdef.Middleware(db, opdef.Must("PUT", "/role/updateStatus")), ctl.UpdateRoleStatus)
	g.GET("/role/vo/idList", ctl.QueryRoleMenuIDList)
	g.PUT("/role/assignPermissions", opdef.Middleware(db, opdef.Must("PUT", "/role/assignPermissions")), ctl.AssignPermissions)

	g.GET("/menu/list", ctl.GetMenuList)
	g.GET("/menu/vo/list", ctl.QuerySysMenuVoList)
	g.GET("/menu/info", ctl.GetMenuInfo)
	g.POST("/menu/add", opdef.Middleware(db, opdef.Must("POST", "/menu/add")), ctl.CreateMenu)
	g.PUT("/menu/update", opdef.Middleware(db, opdef.Must("PUT", "/menu/update")), ctl.UpdateMenu)
	g.DELETE("/menu/delete", opdef.Middleware(db, opdef.Must("DELETE", "/menu/delete")), ctl.DeleteMenu)

	g.GET("/dept/list", ctl.GetDeptList)
	g.GET("/dept/vo/list", ctl.QuerySysDeptVoList)
	g.GET("/dept/info", ctl.GetDeptInfo)
	g.GET("/dept/users", ctl.GetDeptUsers)
	g.POST("/dept/add", opdef.Middleware(db, opdef.Must("POST", "/dept/add")), ctl.CreateDept)
	g.PUT("/dept/update", opdef.Middleware(db, opdef.Must("PUT", "/dept/update")), ctl.UpdateDept)
	g.DELETE("/dept/delete", opdef.Middleware(db, opdef.Must("DELETE", "/dept/delete")), ctl.DeleteDept)

	g.GET("/post/list", ctl.GetPostList)
	g.GET("/post/vo/list", ctl.QuerySysPostVoList)
	g.GET("/post/info", ctl.GetPostInfo)
	g.POST("/post/add", opdef.Middleware(db, opdef.Must("POST", "/post/add")), ctl.CreatePost)
	g.PUT("/post/update", opdef.Middleware(db, opdef.Must("PUT", "/post/update")), ctl.UpdatePost)
	g.PUT("/post/updateStatus", opdef.Middleware(db, opdef.Must("PUT", "/post/updateStatus")), ctl.UpdatePostStatus)
	g.DELETE("/post/delete", opdef.Middleware(db, opdef.Must("DELETE", "/post/delete")), ctl.DeletePost)
	g.DELETE("/post/batch/delete", opdef.Middleware(db, opdef.Must("DELETE", "/post/batch/delete")), ctl.BatchDeletePost)

	g.GET("/sysLoginInfo/list", ctl.GetLoginLogList)
	g.DELETE("/sysLoginInfo/delete", opdef.Middleware(db, opdef.Must("DELETE", "/sysLoginInfo/delete")), ctl.DeleteLoginLog)
	g.DELETE("/sysLoginInfo/batch/delete", opdef.Middleware(db, opdef.Must("DELETE", "/sysLoginInfo/batch/delete")), ctl.BatchDeleteLoginLog)
	g.DELETE("/sysLoginInfo/clean", opdef.Middleware(db, opdef.Must("DELETE", "/sysLoginInfo/clean")), ctl.CleanLoginLog)

	g.GET("/sysOperationLog/list", ctl.GetOperationLogList)
	g.DELETE("/sysOperationLog/delete", opdef.Middleware(db, opdef.Must("DELETE", "/sysOperationLog/delete")), ctl.DeleteOperationLog)
	g.DELETE("/sysOperationLog/batch/delete", opdef.Middleware(db, opdef.Must("DELETE", "/sysOperationLog/batch/delete")), ctl.BatchDeleteOperationLog)
	g.DELETE("/sysOperationLog/clean", opdef.Middleware(db, opdef.Must("DELETE", "/sysOperationLog/clean")), ctl.CleanOperationLog)

	g.GET("/notify/template/list", ctl.GetNotifyTemplateList)
	g.GET("/notify/template/options", ctl.GetNotifyTemplateOptions)
	g.GET("/notify/template/info", ctl.GetNotifyTemplateInfo)
	g.POST("/notify/template/save", opdef.Middleware(db, opdef.Must("POST", "/notify/template/save")), ctl.SaveNotifyTemplate)
	g.DELETE("/notify/template/delete", opdef.Middleware(db, opdef.Must("DELETE", "/notify/template/delete")), ctl.DeleteNotifyTemplate)
	g.GET("/notify/channel/list", opdef.Middleware(db, opdef.Must("GET", "/notify/channel/list")), ctl.GetNotifyChannelList)
	g.GET("/notify/channel/options", opdef.Middleware(db, opdef.Must("GET", "/notify/channel/options")), ctl.GetNotifyChannelOptions)
	g.GET("/notify/channel/info", opdef.Middleware(db, opdef.Must("GET", "/notify/channel/info")), ctl.GetNotifyChannelInfo)
	g.POST("/notify/channel/save", opdef.Middleware(db, opdef.Must("POST", "/notify/channel/save")), ctl.SaveNotifyChannel)
	g.DELETE("/notify/channel/delete", opdef.Middleware(db, opdef.Must("DELETE", "/notify/channel/delete")), ctl.DeleteNotifyChannel)
	g.GET("/notify/rule/list", ctl.GetNotifyRuleList)
	g.GET("/notify/rule/options", ctl.GetNotifyRuleOptions)
	g.GET("/notify/rule/info", ctl.GetNotifyRuleInfo)
	g.POST("/notify/rule/save", opdef.Middleware(db, opdef.Must("POST", "/notify/rule/save")), ctl.SaveNotifyRule)
	g.POST("/notify/rule/test", opdef.Middleware(db, opdef.Must("POST", "/notify/rule/test")), ctl.TestNotifyRule)
	g.DELETE("/notify/rule/delete", opdef.Middleware(db, opdef.Must("DELETE", "/notify/rule/delete")), ctl.DeleteNotifyRule)
	g.GET("/notify/send-log/list", ctl.GetNotifySendLogList)
	g.POST("/notify/send-log/retry", opdef.Middleware(db, opdef.Must("POST", "/notify/send-log/retry")), ctl.RetryNotifySendLog)
}
