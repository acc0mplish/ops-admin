package store

import (
	"ops-admin/backend/model"

	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.Dept{},
		&model.Post{},
		&model.Role{},
		&model.Menu{},
		&model.Admin{},
		&model.AdminRole{},
		&model.RoleMenu{},
		&model.LoginLog{},
		&model.OperationLog{},
		&model.SystemConfig{},
		&model.AssetHostGroup{},
		&model.AssetHostGroupRelation{},
		&model.AssetCredential{},
		&model.AssetCloudAccount{},
		&model.AssetHost{},
		&model.AssetDatabase{},
		&model.DatabaseSQLHistory{},
		&model.DatabaseTransferTask{},
		&model.K8sCluster{},
		&model.OpsScript{},
		&model.OpsExecTask{},
		&model.OpsExecTargetResult{},
		&model.OpsScheduleTemplate{},
		&model.OpsScheduleTask{},
		&model.OpsScheduleTaskLog{},
		&model.OpsJobTemplate{},
		&model.OpsJob{},
		&model.OpsJobHistory{},
		&model.OpsJobHistoryStep{},
		&model.OpsApplication{},
		&model.OpsAppBuildTask{},
		&model.OpsAppRelease{},
		&model.NotifyTemplate{},
		&model.NotifyChannel{},
		&model.NotifyRule{},
		&model.NotifySendLog{},
		&model.MonitorDatasource{},
		&model.MonitorAlertRule{},
		&model.MonitorAlertEvent{},
		&model.MonitorSilenceRule{},
		&model.MonitorAggregationRule{},
		&model.MonitorQueryHistory{},
		&model.MonitorDashboard{},
		&model.MonitorDashboardPanel{},
	)
}
