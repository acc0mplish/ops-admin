package opdef

import "net/http"

// systemDefs covers the systemConfig/ldap/admin/role/menu/dept/post/log
// section of the authGroup. Normal GET reads (menu/dept/post/log listings and
// the LDAP config probe, which only returns a bindPasswordSet bool) stay
// auth-only; every mutation and the admin account reads are defined here.
// Permission strings reuse the seeded system:* button vocabulary wherever it
// exists.
var systemDefs = []Def{
	{Method: http.MethodPut, Path: "/systemConfig", Permission: "system:config:save", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/systemConfig/upload", Permission: "system:config:upload", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/system/ldap/config", Permission: "system:config:ldap", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPost, Path: "/system/ldap/test", Permission: "system:config:ldap-test", Mutating: true, Risk: RiskMedium},

	{Method: http.MethodPost, Path: "/admin/ldap/sync", Permission: "system:admin:ldap-sync", Mutating: true, Risk: RiskMedium},

	{Method: http.MethodGet, Path: "/admin/list", Permission: "system:admin:list", Risk: RiskLow},
	{Method: http.MethodGet, Path: "/admin/info", Permission: "system:admin:list", Risk: RiskLow},
	{Method: http.MethodPost, Path: "/admin/add", Permission: "system:admin:add", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/admin/update", Permission: "system:admin:edit", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/admin/delete", Permission: "system:admin:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPut, Path: "/admin/updateStatus", Permission: "system:admin:status", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/admin/updatePassword", Permission: "system:admin:resetpwd", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPut, Path: "/admin/updatePersonal", Permission: "system:admin:personal", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/admin/updatePersonalPassword", Permission: "system:admin:personal", Mutating: true, Risk: RiskHigh},

	{Method: http.MethodPost, Path: "/role/add", Permission: "system:role:add", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/role/update", Permission: "system:role:edit", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/role/delete", Permission: "system:role:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodPut, Path: "/role/updateStatus", Permission: "system:role:status", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/role/assignPermissions", Permission: "system:role:assign", Mutating: true, Risk: RiskHigh},

	{Method: http.MethodPost, Path: "/menu/add", Permission: "system:menu:add", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/menu/update", Permission: "system:menu:edit", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/menu/delete", Permission: "system:menu:delete", Mutating: true, Risk: RiskHigh},

	{Method: http.MethodPost, Path: "/dept/add", Permission: "system:dept:add", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/dept/update", Permission: "system:dept:edit", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/dept/delete", Permission: "system:dept:delete", Mutating: true, Risk: RiskHigh},

	{Method: http.MethodPost, Path: "/post/add", Permission: "system:post:add", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/post/update", Permission: "system:post:edit", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodPut, Path: "/post/updateStatus", Permission: "system:post:status", Mutating: true, Risk: RiskMedium},
	{Method: http.MethodDelete, Path: "/post/delete", Permission: "system:post:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodDelete, Path: "/post/batch/delete", Permission: "system:post:delete", Mutating: true, Risk: RiskHigh},

	{Method: http.MethodDelete, Path: "/sysLoginInfo/delete", Permission: "system:loginlog:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodDelete, Path: "/sysLoginInfo/batch/delete", Permission: "system:loginlog:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodDelete, Path: "/sysLoginInfo/clean", Permission: "system:loginlog:clean", Mutating: true, Risk: RiskHigh},

	{Method: http.MethodDelete, Path: "/sysOperationLog/delete", Permission: "system:operationlog:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodDelete, Path: "/sysOperationLog/batch/delete", Permission: "system:operationlog:delete", Mutating: true, Risk: RiskHigh},
	{Method: http.MethodDelete, Path: "/sysOperationLog/clean", Permission: "system:operationlog:clean", Mutating: true, Risk: RiskHigh},
}
