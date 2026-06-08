package router

import (
	"ops-admin/backend/config"
	"ops-admin/backend/controller"
	"ops-admin/backend/middleware"
	"ops-admin/backend/service"
	"os"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func New(cfg *config.Config, db *gorm.DB) *gin.Engine {
	if cfg.App.Mode == gin.ReleaseMode {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.Use(gin.Logger(), gin.Recovery(), middleware.CORS())
	_ = os.MkdirAll("uploads", 0o755)
	engine.Static("/uploads", "./uploads")

	svc := service.New(db)
	ctl := controller.New(svc)

	engine.GET("/ping", ctl.Ping)

	api := engine.Group("/api/v1")
	{
		api.POST("/login", ctl.Login)
		api.GET("/systemConfig/public", ctl.GetSystemConfig)
		api.GET("/asset/terminal/ws", ctl.AssetTerminalWS)
		api.GET("/k8s/pod/terminal/ws", ctl.K8sPodTerminalWS)
	}

	authGroup := api.Group("")
	authGroup.Use(middleware.Auth(), middleware.OperationLog(db))
	{
		authGroup.GET("/profile", ctl.Profile)
		authGroup.GET("/systemConfig", ctl.GetSystemConfig)
		authGroup.PUT("/systemConfig", ctl.UpdateSystemConfig)
		authGroup.POST("/systemConfig/upload", ctl.UploadSystemAsset)

		authGroup.GET("/admin/list", ctl.GetSysAdminList)
		authGroup.GET("/admin/info", ctl.GetSysAdminInfo)
		authGroup.POST("/admin/add", ctl.CreateSysAdmin)
		authGroup.PUT("/admin/update", ctl.UpdateSysAdmin)
		authGroup.DELETE("/admin/delete", ctl.DeleteSysAdmin)
		authGroup.PUT("/admin/updateStatus", ctl.UpdateSysAdminStatus)
		authGroup.PUT("/admin/updatePassword", ctl.ResetSysAdminPassword)
		authGroup.PUT("/admin/updatePersonal", ctl.UpdatePersonal)
		authGroup.PUT("/admin/updatePersonalPassword", ctl.UpdatePersonalPassword)

		authGroup.GET("/role/list", ctl.GetRoleList)
		authGroup.GET("/role/vo/list", ctl.QuerySysRoleVoList)
		authGroup.GET("/role/info", ctl.GetRoleInfo)
		authGroup.POST("/role/add", ctl.CreateRole)
		authGroup.PUT("/role/update", ctl.UpdateRole)
		authGroup.DELETE("/role/delete", ctl.DeleteRole)
		authGroup.PUT("/role/updateStatus", ctl.UpdateRoleStatus)
		authGroup.GET("/role/vo/idList", ctl.QueryRoleMenuIDList)
		authGroup.PUT("/role/assignPermissions", ctl.AssignPermissions)

		authGroup.GET("/menu/list", ctl.GetMenuList)
		authGroup.GET("/menu/vo/list", ctl.QuerySysMenuVoList)
		authGroup.GET("/menu/info", ctl.GetMenuInfo)
		authGroup.POST("/menu/add", ctl.CreateMenu)
		authGroup.PUT("/menu/update", ctl.UpdateMenu)
		authGroup.DELETE("/menu/delete", ctl.DeleteMenu)

		authGroup.GET("/dept/list", ctl.GetDeptList)
		authGroup.GET("/dept/vo/list", ctl.QuerySysDeptVoList)
		authGroup.GET("/dept/info", ctl.GetDeptInfo)
		authGroup.GET("/dept/users", ctl.GetDeptUsers)
		authGroup.POST("/dept/add", ctl.CreateDept)
		authGroup.PUT("/dept/update", ctl.UpdateDept)
		authGroup.DELETE("/dept/delete", ctl.DeleteDept)

		authGroup.GET("/post/list", ctl.GetPostList)
		authGroup.GET("/post/vo/list", ctl.QuerySysPostVoList)
		authGroup.GET("/post/info", ctl.GetPostInfo)
		authGroup.POST("/post/add", ctl.CreatePost)
		authGroup.PUT("/post/update", ctl.UpdatePost)
		authGroup.PUT("/post/updateStatus", ctl.UpdatePostStatus)
		authGroup.DELETE("/post/delete", ctl.DeletePost)
		authGroup.DELETE("/post/batch/delete", ctl.BatchDeletePost)

		authGroup.GET("/sysLoginInfo/list", ctl.GetLoginLogList)
		authGroup.DELETE("/sysLoginInfo/delete", ctl.DeleteLoginLog)
		authGroup.DELETE("/sysLoginInfo/batch/delete", ctl.BatchDeleteLoginLog)
		authGroup.DELETE("/sysLoginInfo/clean", ctl.CleanLoginLog)

		authGroup.GET("/sysOperationLog/list", ctl.GetOperationLogList)
		authGroup.DELETE("/sysOperationLog/delete", ctl.DeleteOperationLog)
		authGroup.DELETE("/sysOperationLog/batch/delete", ctl.BatchDeleteOperationLog)
		authGroup.DELETE("/sysOperationLog/clean", ctl.CleanOperationLog)

		authGroup.GET("/asset/host/list", ctl.GetAssetHostList)
		authGroup.GET("/asset/overview", ctl.GetAssetOverview)
		authGroup.GET("/asset/host/info", ctl.GetAssetHostInfo)
		authGroup.GET("/asset/host/template", ctl.DownloadAssetHostTemplate)
		authGroup.POST("/asset/host/add", ctl.CreateAssetHost)
		authGroup.POST("/asset/host/import", ctl.ImportAssetHosts)
		authGroup.POST("/asset/host/cloudSync", ctl.SyncAssetHostsFromCloud)
		authGroup.PUT("/asset/host/update", ctl.UpdateAssetHost)
		authGroup.POST("/asset/host/sync", ctl.SyncAssetHost)
		authGroup.POST("/asset/host/batch/sync", ctl.BatchSyncAssetHosts)
		authGroup.PUT("/asset/host/batch/credential", ctl.BatchReplaceAssetHostCredential)
		authGroup.DELETE("/asset/host/delete", ctl.DeleteAssetHost)
		authGroup.DELETE("/asset/host/batch/delete", ctl.BatchDeleteAssetHosts)

		authGroup.GET("/asset/hostGroup/list", ctl.GetAssetHostGroupList)
		authGroup.GET("/asset/hostGroup/info", ctl.GetAssetHostGroupInfo)
		authGroup.POST("/asset/hostGroup/add", ctl.CreateAssetHostGroup)
		authGroup.PUT("/asset/hostGroup/update", ctl.UpdateAssetHostGroup)
		authGroup.DELETE("/asset/hostGroup/delete", ctl.DeleteAssetHostGroup)

		authGroup.GET("/asset/credential/list", ctl.GetAssetCredentialList)
		authGroup.GET("/asset/credential/options", ctl.GetAssetCredentialOptions)
		authGroup.GET("/asset/credential/info", ctl.GetAssetCredentialInfo)
		authGroup.POST("/asset/credential/add", ctl.CreateAssetCredential)
		authGroup.PUT("/asset/credential/update", ctl.UpdateAssetCredential)
		authGroup.DELETE("/asset/credential/delete", ctl.DeleteAssetCredential)

		authGroup.GET("/asset/cloudAccount/list", ctl.GetAssetCloudAccountList)
		authGroup.GET("/asset/cloudAccount/options", ctl.GetAssetCloudAccountOptions)
		authGroup.GET("/asset/cloudAccount/info", ctl.GetAssetCloudAccountInfo)
		authGroup.POST("/asset/cloudAccount/add", ctl.CreateAssetCloudAccount)
		authGroup.PUT("/asset/cloudAccount/update", ctl.UpdateAssetCloudAccount)
		authGroup.DELETE("/asset/cloudAccount/delete", ctl.DeleteAssetCloudAccount)

		authGroup.GET("/asset/database/list", ctl.GetAssetDatabaseList)
		authGroup.GET("/asset/database/info", ctl.GetAssetDatabaseInfo)
		authGroup.POST("/asset/database/add", ctl.CreateAssetDatabase)
		authGroup.PUT("/asset/database/update", ctl.UpdateAssetDatabase)
		authGroup.DELETE("/asset/database/delete", ctl.DeleteAssetDatabase)
		authGroup.POST("/asset/database/test", ctl.TestAssetDatabaseConnection)

		authGroup.GET("/dbms/workbench", ctl.GetDatabaseWorkbench)
		authGroup.GET("/dbms/schema/tree", ctl.GetDatabaseSchemaTree)
		authGroup.GET("/dbms/table/data", ctl.GetDatabaseTableData)
		authGroup.POST("/dbms/sql/execute", ctl.ExecuteDatabaseSQL)
		authGroup.GET("/dbms/sql/history", ctl.GetDatabaseSQLHistory)
		authGroup.POST("/dbms/table/row/insert", ctl.InsertDatabaseTableRow)
		authGroup.PUT("/dbms/table/row/update", ctl.UpdateDatabaseTableRow)
		authGroup.DELETE("/dbms/table/row/delete", ctl.DeleteDatabaseTableRow)
		authGroup.POST("/dbms/task/export", ctl.CreateExportTask)
		authGroup.POST("/dbms/task/import", ctl.CreateImportTask)
		authGroup.GET("/dbms/task/list", ctl.GetTransferTaskList)
		authGroup.GET("/dbms/task/download", ctl.DownloadTransferTask)
		
		authGroup.GET("/k8s/cluster/list", ctl.GetK8sClusterList)
		authGroup.GET("/k8s/cluster/info", ctl.GetK8sClusterInfo)
		authGroup.POST("/k8s/cluster/add", ctl.CreateK8sCluster)
		authGroup.PUT("/k8s/cluster/update", ctl.UpdateK8sCluster)
		authGroup.DELETE("/k8s/cluster/delete", ctl.DeleteK8sCluster)
		authGroup.GET("/k8s/cluster/detail", ctl.GetK8sClusterDetail)
		authGroup.GET("/k8s/node/detail", ctl.GetK8sNodeDetail)
		authGroup.GET("/k8s/node/pods", ctl.GetK8sNodePods)
		authGroup.GET("/k8s/namespace/detail", ctl.GetK8sNamespaceDetail)
		authGroup.GET("/k8s/namespace/events", ctl.GetK8sNamespaceEvents)
		authGroup.GET("/k8s/service/detail", ctl.GetK8sServiceDetail)
		authGroup.GET("/k8s/ingress/detail", ctl.GetK8sIngressDetail)
		authGroup.GET("/k8s/istio/detail", ctl.GetK8sIstioResourceDetail)
		authGroup.GET("/k8s/configmap/detail", ctl.GetK8sConfigMapDetail)
		authGroup.GET("/k8s/secret/detail", ctl.GetK8sSecretDetail)
		authGroup.GET("/k8s/storage/detail", ctl.GetK8sStorageDetail)
		authGroup.GET("/k8s/pod/detail", ctl.GetK8sPodDetail)
		authGroup.GET("/k8s/pod/containers", ctl.GetK8sPodContainers)
		authGroup.GET("/k8s/pod/logs", ctl.GetK8sPodLogs)
		authGroup.GET("/k8s/pod/events", ctl.GetK8sPodEvents)
		authGroup.GET("/k8s/workload/detail", ctl.GetK8sWorkloadDetail)
		authGroup.POST("/k8s/workload/scale", ctl.ScaleK8sWorkload)
		authGroup.POST("/k8s/workload/restart", ctl.RestartK8sWorkload)
		authGroup.POST("/k8s/workload/images", ctl.UpdateK8sWorkloadImages)
		authGroup.POST("/k8s/istio/traffic", ctl.UpdateK8sIstioTraffic)
		authGroup.POST("/k8s/httproute/traffic", ctl.UpdateK8sHTTPRouteTraffic)
		authGroup.POST("/k8s/resource/yaml/create", ctl.CreateK8sResourceYAML)
		authGroup.PUT("/k8s/resource/yaml", ctl.UpdateK8sResourceYAML)
		authGroup.DELETE("/k8s/resource/delete", ctl.DeleteK8sResource)
	}

	return engine
}
