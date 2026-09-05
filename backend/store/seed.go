package store

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"ops-admin/backend/model"
	"ops-admin/backend/opdef"
	"ops-admin/backend/util"

	"gorm.io/gorm"
)

const (
	// routePermissionsRootValue is the hidden root menu holding route
	// permissions that have no seeded page menu. It carries menu_status 0, so
	// the sidebar export (menu_status = 1 AND menu_type IN (1,2)) never shows
	// it, while the grant lookup only reads the leaf's status.
	routePermissionsRootValue = "route-permissions"
	// routePermissionsMarkerValue is the one-shot migration marker: it exists
	// in sys_menu once every existing role has been granted the route
	// permissions, and its presence makes the migration a complete no-op.
	routePermissionsMarkerValue = "route-permissions:granted:v1"
)

func Seed(db *gorm.DB) error {
	steps := []func(*gorm.DB) error{
		seedSystemConfig,
		seedMonitorAlertTemplates,
		seedDept,
		seedPost,
		seedRole,
		seedMenus,
		seedAdmin,
		seedSuperRolePermissions,
		seedRoutePermissionMenus,
		seedSuperAdminRoutePermissions,
		migrateRoleRoutePermissionsOnce,
	}
	for _, step := range steps {
		if err := step(db); err != nil {
			return err
		}
	}
	return nil
}

type alertSeedSpec struct {
	category   string
	collector  string
	objectType string
	query      string
	comparator string
	threshold  float64
	duration   int
	severity   string
	domain     string
}

func seedMonitorAlertTemplates(db *gorm.DB) error {
	return db.Transaction(seedMonitorAlertTemplatesTx)
}

func seedMonitorAlertTemplatesTx(db *gorm.DB) error {
	groupIDs := map[string]uint{}
	for _, path := range [][]string{{"Linux", "node_exporter"}, {"Kubernetes", "kube-state-metrics"}, {"MySQL", "mysqld_exporter"}, {"Redis", "redis_exporter"}} {
		parentID := uint(0)
		for _, name := range path {
			key := fmt.Sprintf("%d/%s", parentID, name)
			var group model.MonitorAlertTemplateGroup
			err := db.Where("parent_id = ? AND name = ?", parentID, name).First(&group).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				group = model.MonitorAlertTemplateGroup{ParentID: parentID, Name: name}
				if err = db.Create(&group).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
			groupIDs[key] = group.ID
			parentID = group.ID
		}
	}

	specs := []alertSeedSpec{
		{"Linux", "node_exporter", "host", `up{job=~"node.*|node_exporter"}`, "==", 0, 120, "P0", "availability"},
		{"Linux", "node_exporter", "host", `100 - (avg by(instance) (irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)`, ">", 90, 300, "P1", "cpu"},
		{"Linux", "node_exporter", "host", `node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes * 100`, "<", 10, 300, "P1", "memory"},
		{"Linux", "node_exporter", "host", `(100 - ((node_filesystem_avail_bytes{fstype!~"tmpfs|overlay"} * 100) / node_filesystem_size_bytes{fstype!~"tmpfs|overlay"}))`, ">", 85, 600, "P1", "disk"},
		{"Linux", "node_exporter", "host", `node_netstat_Tcp_CurrEstab`, ">", 20000, 300, "P1", "connection"},
		{"Linux", "node_exporter", "host", `node_sockstat_UDP_inuse`, ">", 10000, 300, "P1", "connection"},
		{"Linux", "node_exporter", "host", `avg by(instance) (rate(node_cpu_seconds_total{mode="user"}[5m])) * 100`, ">", 70, 300, "P1", "cpu"},
		{"Linux", "node_exporter", "host", `avg by(instance) (rate(node_cpu_seconds_total{mode="system"}[5m])) * 100`, ">", 50, 300, "P1", "cpu"},
		{"Linux", "node_exporter", "host", `100 - (node_filesystem_files_free{mountpoint="/",fstype!~"tmpfs|overlay"} / node_filesystem_files{mountpoint="/",fstype!~"tmpfs|overlay"} * 100)`, ">", 85, 600, "P1", "disk"},
		{"Linux", "node_exporter", "host", `sum by(instance) (rate(node_disk_read_bytes_total[5m]))`, ">", 104857600, 300, "P2", "disk_io"},
		{"Linux", "node_exporter", "host", `sum by(instance) (rate(node_disk_written_bytes_total[5m]))`, ">", 104857600, 300, "P2", "disk_io"},
		{"Linux", "node_exporter", "host", `sum by(instance) (rate(node_disk_reads_completed_total[5m]))`, ">", 5000, 300, "P2", "disk_io"},
		{"Linux", "node_exporter", "host", `sum by(instance) (rate(node_disk_writes_completed_total[5m]))`, ">", 5000, 300, "P2", "disk_io"},
		{"Linux", "node_exporter", "host", `node_load1`, ">", 8, 300, "P2", "load"},
		{"Linux", "node_exporter", "host", `node_load5`, ">", 8, 300, "P2", "load"},
		{"Linux", "node_exporter", "host", `node_load15`, ">", 8, 600, "P2", "load"},
		{"Linux", "node_exporter", "host", `node_load1 / count by(instance) (node_cpu_seconds_total{mode="idle"})`, ">", 1, 300, "P2", "load"},
		{"Linux", "node_exporter", "host", `node_load5 / count by(instance) (node_cpu_seconds_total{mode="idle"})`, ">", 1, 300, "P2", "load"},
		{"Linux", "node_exporter", "host", `node_load15 / count by(instance) (node_cpu_seconds_total{mode="idle"})`, ">", 1, 600, "P2", "load"},
		{"Linux", "node_exporter", "host", `(1 - (node_memory_SwapFree_bytes / node_memory_SwapTotal_bytes)) * 100`, ">", 50, 300, "P2", "memory"},
		{"Linux", "node_exporter", "host", `sum by(instance) (rate(node_network_receive_bytes_total{device!~"lo"}[5m]))`, ">", 104857600, 300, "P2", "network"},
		{"Linux", "node_exporter", "host", `sum by(instance) (rate(node_network_transmit_bytes_total{device!~"lo"}[5m]))`, ">", 104857600, 300, "P2", "network"},
		{"Linux", "node_exporter", "host", `time() - node_boot_time_seconds`, "<", 86400, 0, "P2", "service"},
		{"Linux", "node_exporter", "host", `node_filefd_allocated`, ">", 100000, 300, "P2", "service"},

		{"Kubernetes", "kube-state-metrics", "node", `kube_node_status_condition{condition="Ready",status="true"}`, "==", 0, 300, "P1", "kubernetes"},
		{"Kubernetes", "kube-state-metrics", "pod", `sum by (namespace, pod, phase) (kube_pod_status_phase{phase=~"Failed|Unknown|Pending"})`, ">", 0, 300, "P1", "kubernetes"},
		{"Kubernetes", "kube-state-metrics", "pod", `sum by (namespace, pod) (increase(kube_pod_container_status_restarts_total[10m]))`, ">", 3, 0, "P2", "kubernetes"},
		{"Kubernetes", "kube-state-metrics", "deployment", `kube_deployment_status_replicas_unavailable`, ">", 0, 300, "P1", "kubernetes"},

		{"MySQL", "mysqld_exporter", "database", `mysql_up`, "==", 0, 120, "P1", "mysql_availability"},
		{"MySQL", "mysqld_exporter", "database", `mysql_global_status_uptime`, "<", 300, 0, "P2", "mysql_availability"},
		{"MySQL", "mysqld_exporter", "database", `(mysql_global_status_threads_connected / clamp_min(mysql_global_variables_max_connections, 1)) * 100`, ">", 85, 300, "P1", "mysql_connection"},
		{"MySQL", "mysqld_exporter", "database", `mysql_global_status_threads_running`, ">", 50, 300, "P2", "mysql_connection"},
		{"MySQL", "mysqld_exporter", "database", `increase(mysql_global_status_slow_queries[5m])`, ">", 10, 300, "P2", "mysql_query"},
		{"MySQL", "mysqld_exporter", "database", `increase(mysql_global_status_aborted_clients[5m])`, ">", 10, 300, "P2", "mysql_connection"},
		{"MySQL", "mysqld_exporter", "database", `increase(mysql_global_status_aborted_connects[5m])`, ">", 10, 300, "P2", "mysql_connection"},
		{"MySQL", "mysqld_exporter", "database", `(mysql_global_status_buffer_pool_bytes_dirty / clamp_min(mysql_global_status_buffer_pool_bytes_total, 1)) * 100`, ">", 20, 300, "P2", "mysql_innodb"},
		{"MySQL", "mysqld_exporter", "database", `(1 - rate(mysql_global_status_innodb_buffer_pool_reads[5m]) / clamp_min(rate(mysql_global_status_innodb_buffer_pool_read_requests[5m]), 1)) * 100`, "<", 99, 600, "P2", "mysql_innodb"},
		{"MySQL", "mysqld_exporter", "database", `increase(mysql_global_status_innodb_row_lock_waits[5m])`, ">", 10, 300, "P2", "mysql_lock"},
		{"MySQL", "mysqld_exporter", "database", `increase(mysql_global_status_innodb_row_lock_time[5m]) / 1000`, ">", 10, 300, "P1", "mysql_lock"},
		{"MySQL", "mysqld_exporter", "database", `(rate(mysql_global_status_created_tmp_disk_tables[5m]) / clamp_min(rate(mysql_global_status_created_tmp_tables[5m]), 1)) * 100`, ">", 25, 600, "P2", "mysql_query"},
		{"MySQL", "mysqld_exporter", "database", `(mysql_global_status_open_files / clamp_min(mysql_global_variables_open_files_limit, 1)) * 100`, ">", 85, 300, "P1", "mysql_capacity"},
		{"MySQL", "mysqld_exporter", "database", `(mysql_global_status_prepared_stmt_count / clamp_min(mysql_global_variables_max_prepared_stmt_count, 1)) * 100`, ">", 85, 300, "P2", "mysql_capacity"},
		{"MySQL", "mysqld_exporter", "database", `mysql_scrape_use_seconds`, ">", 5, 300, "P2", "mysql_exporter"},

		{"Redis", "redis_exporter", "database", `redis_up`, "==", 0, 120, "P1", "redis_availability"},
		{"Redis", "redis_exporter", "database", `redis_uptime_in_seconds`, "<", 300, 0, "P2", "redis_availability"},
		{"Redis", "redis_exporter", "database", `100 * redis_memory_used_bytes / clamp_min((redis_memory_max_bytes > 0) or on(instance) redis_memory_used_peak_bytes, 1)`, ">", 85, 300, "P1", "redis_memory"},
		{"Redis", "redis_exporter", "database", `100 * rate(redis_keyspace_hits_total[5m]) / clamp_min(rate(redis_keyspace_hits_total[5m]) + rate(redis_keyspace_misses_total[5m]), 1)`, "<", 80, 600, "P2", "redis_cache"},
		{"Redis", "redis_exporter", "database", `histogram_quantile(0.95, sum by (le, instance) (rate(redis_commands_latencies_usec_bucket[5m]))) / 1000`, ">", 50, 300, "P2", "redis_latency"},
		{"Redis", "redis_exporter", "database", `rate(redis_total_error_replies[5m])`, ">", 1, 300, "P2", "redis_error"},
		{"Redis", "redis_exporter", "database", `increase(redis_rejected_connections_total[5m])`, ">", 0, 0, "P1", "redis_connection"},
		{"Redis", "redis_exporter", "database", `redis_blocked_clients`, ">", 50, 300, "P2", "redis_connection"},
		{"Redis", "redis_exporter", "database", `increase(redis_evicted_keys_total[5m])`, ">", 0, 0, "P1", "redis_memory"},
		{"Redis", "redis_exporter", "database", `redis_slowlog_length`, ">", 10, 300, "P2", "redis_latency"},
	}

	if err := db.Where("source = ? AND collector = ?", "platform", "agent").Delete(&model.MonitorAlertTemplate{}).Error; err != nil {
		return err
	}

	for index, spec := range specs {
		rootID := groupIDs[fmt.Sprintf("%d/%s", uint(0), spec.category)]
		groupID := groupIDs[fmt.Sprintf("%d/%s", rootID, spec.collector)]
		if rootID == 0 || groupID == 0 {
			return fmt.Errorf("resolve built-in alert template group failed: %s/%s", spec.category, spec.collector)
		}
		name := fmt.Sprintf("%s %s Alert %02d", spec.category, strings.ReplaceAll(spec.domain, "_", " "), index+1)
		item := model.MonitorAlertTemplate{
			GroupID: groupID, Name: name, Category: spec.category, Collector: spec.collector,
			ObjectType: spec.objectType, DatasourceType: "prometheus", QueryText: spec.query,
			Comparator: spec.comparator, Threshold: spec.threshold, ForSeconds: spec.duration,
			EvalIntervalSeconds: 60, Severity: spec.severity,
			LabelsJSON:      fmt.Sprintf(`{"domain":%q}`, spec.domain),
			AnnotationsJSON: fmt.Sprintf(`{"summary":%q}`, name),
			Description:     "Built-in platform alert definition. Operational guidance is applied from the canonical alert catalog.",
			Source:          "platform", Status: 1,
		}
		var existing model.MonitorAlertTemplate
		err := db.Where("query_text = ? AND source = ?", spec.query, "platform").First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := db.Create(&item).Error; err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		updates := map[string]any{
			"group_id": item.GroupID, "category": item.Category, "collector": item.Collector,
			"object_type": item.ObjectType, "datasource_type": item.DatasourceType,
			"query_text": item.QueryText, "comparator": item.Comparator, "threshold": item.Threshold,
			"for_seconds": item.ForSeconds, "eval_interval_seconds": item.EvalIntervalSeconds,
			"severity": item.Severity, "labels_json": item.LabelsJSON, "status": item.Status,
		}
		if err := db.Model(&existing).Updates(updates).Error; err != nil {
			return err
		}
	}
	return nil
}

func seedSystemConfig(db *gorm.DB) error {
	var count int64
	if err := db.Model(&model.SystemConfig{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return db.Create(&model.SystemConfig{
		SiteName: "Ops Admin", SiteSlogan: "Personal operations platform", LogoType: "text", LogoValue: "OA",
		LoginTitle: "Ops Admin", LoginSubtitle: "System management and operations console",
		UseLoginBackground: false, PrimaryColor: "#5b6cf9", SidebarTheme: "dark", CreatedAt: time.Now(),
	}).Error
}

func seedDept(db *gorm.DB) error {
	var count int64
	db.Model(&model.Dept{}).Count(&count)
	if count > 0 {
		return nil
	}
	return db.Create(&model.Dept{ParentID: 0, DeptType: 1, DeptName: "Headquarters", DeptStatus: 1, CreatedAt: time.Now()}).Error
}

func seedPost(db *gorm.DB) error {
	var count int64
	db.Model(&model.Post{}).Count(&count)
	if count > 0 {
		return nil
	}
	return db.Create(&model.Post{PostCode: "super-admin", PostName: "Super Administrator", PostStatus: 1, Remark: "System bootstrap position", CreatedAt: time.Now()}).Error
}

func seedRole(db *gorm.DB) error {
	var count int64
	db.Model(&model.Role{}).Count(&count)
	if count > 0 {
		return nil
	}
	return db.Create(&model.Role{RoleName: "Super Administrator", RoleKey: "super-admin", Status: 1, Description: "System bootstrap role", CreatedAt: time.Now()}).Error
}

func seedMenuName(value string) string {
	names := map[string]string{
		"system": "System Management", "console:business-topology": "Business Topology", "logs": "Operation Audit",
		"system:admin:list": "Users", "system:role:list": "Roles", "system:menu:list": "Menus", "system:dept:list": "Departments", "system:post:list": "Positions", "system:config:view": "System Settings", "system:loginlog:list": "Login Log", "system:operationlog:list": "Operation Log",
		"assets": "Asset Management", "assets:overview": "Asset Overview", "ops:environment:list": "Environment Model", "assets:terminal": "Terminal Login", "assets:host:list": "Host Management", "assets:hostgroup:list": "Host Group Management", "assets:credential:list": "Credential Management", "assets:cloudaccount:list": "Cloud Account Management", "assets:database:list": "Database Management", "assets:database:workbench": "DBMS Workbench", "assets:database:import": "Data Import", "assets:database:backup": "Backup Management", "assets:gateway:list": "Gateway Management",
		"containers": "Container Management", "assets:k8s:cluster": "Cluster Management", "assets:k8s:overview": "Cluster Overview", "assets:k8s:node": "Node Management", "assets:k8s:namespace": "Namespaces", "assets:k8s:workload": "Workloads", "assets:k8s:pod": "Pods", "assets:k8s:service": "Services", "assets:k8s:ingress": "Ingress", "assets:k8s:advancednetwork": "Advanced Network", "assets:k8s:configstorage": "Config & Storage",
		"ops": "Standard Operations", "ops:script:list": "Script Library", "ops:quickexec:command": "Command Execution", "ops:quickexec:script": "Script Execution", "ops:quickexec:file": "File Distribution", "ops:quickexec:history": "Quick Execution History", "ops:schedule:task": "Scheduled Tasks", "ops:schedule:log": "Task Logs", "ops:schedule:template": "Task Templates", "ops:job:designer": "Job Designer", "ops:job:list": "Job List", "ops:job:approval": "Manual Approvals", "ops:job:history": "Job History", "ops:job:template": "Job Templates",
		"applications": "Application Center", "applications:project:list": "Application Management", "applications:buildtask:list": "Build Tasks", "applications:buildhistory:list": "Build History", "applications:pipeline:list": "CI/CD Pipelines",
		"notify": "Notifications", "notify:rule:list": "Notification Rules", "notify:template:list": "Message Templates", "notify:channel:list": "Notification Channels", "notify:sendlog:list": "Send Logs",
		"monitor": "Monitoring Center", "monitor:overview": "Monitoring Overview", "monitor:commandcenter": "Operations Command Center", "monitor:datasource:list": "Datasource Management", "monitor:query": "Instant Query", "monitor:logs": "Log Explorer", "monitor:traces": "Trace Explorer", "monitor:alerttemplate:list": "Alert Templates", "monitor:alertrule:list": "Alert Rules", "monitor:alertevent:list": "Alert Events", "monitor:silence:list": "Alert Silences", "monitor:aggregation:list": "Alert Aggregation", "monitor:dashboard:list": "Monitoring Dashboards", "monitor:inspection:list": "Inspection Dashboards",
		"domains": "Domain Management", "domains:public:list": "Public Domains", "domains:account:list": "Public DNS Accounts", "domains:ssl:view": "SSL Certificates", "domains:internal:list": "Internal Domains", "domains:settings:view": "DNS Settings", "domains:query:test": "DNS Query Test", "domains:audit:list": "Operation Audit",
	}
	return names[value]
}

func seedMenus(db *gorm.DB) error {
	systemRoot, err := ensureMenu(db, model.Menu{ParentID: 0, MenuName: seedMenuName("system"), MenuType: 1, URL: "/system", Value: "system", MenuStatus: 1, Sort: 1, Icon: "Setting"})
	if err != nil {
		return err
	}
	businessTopologyMenu, err := ensureMenu(db, model.Menu{ParentID: 0, MenuName: seedMenuName("console:business-topology"), MenuType: 2, URL: "/business-topology", Value: "console:business-topology", MenuStatus: 1, Sort: 0, Icon: "Share"})
	if err != nil {
		return err
	}
	logRoot, err := ensureMenu(db, model.Menu{ParentID: 0, MenuName: seedMenuName("logs"), MenuType: 1, URL: "/logs", Value: "logs", MenuStatus: 1, Sort: 2, Icon: "Document"})
	if err != nil {
		return err
	}

	items := []model.Menu{
		{ParentID: systemRoot.ID, MenuName: seedMenuName("system:admin:list"), MenuType: 2, URL: "/system/admin", Value: "system:admin:list", MenuStatus: 1, Sort: 1, Icon: "User"},
		{ParentID: systemRoot.ID, MenuName: seedMenuName("system:role:list"), MenuType: 2, URL: "/system/role", Value: "system:role:list", MenuStatus: 1, Sort: 2, Icon: "Avatar"},
		{ParentID: systemRoot.ID, MenuName: seedMenuName("system:menu:list"), MenuType: 2, URL: "/system/menu", Value: "system:menu:list", MenuStatus: 1, Sort: 3, Icon: "Grid"},
		{ParentID: systemRoot.ID, MenuName: seedMenuName("system:dept:list"), MenuType: 2, URL: "/system/dept", Value: "system:dept:list", MenuStatus: 1, Sort: 4, Icon: "OfficeBuilding"},
		{ParentID: systemRoot.ID, MenuName: seedMenuName("system:post:list"), MenuType: 2, URL: "/system/post", Value: "system:post:list", MenuStatus: 1, Sort: 5, Icon: "Suitcase"},
		{ParentID: systemRoot.ID, MenuName: seedMenuName("system:config:view"), MenuType: 2, URL: "/system/settings", Value: "system:config:view", MenuStatus: 1, Sort: 6, Icon: "Tools"},
		{ParentID: logRoot.ID, MenuName: seedMenuName("system:loginlog:list"), MenuType: 2, URL: "/logs/login", Value: "system:loginlog:list", MenuStatus: 1, Sort: 1, Icon: "Tickets"},
		{ParentID: logRoot.ID, MenuName: seedMenuName("system:operationlog:list"), MenuType: 2, URL: "/logs/operation", Value: "system:operationlog:list", MenuStatus: 1, Sort: 2, Icon: "Memo"},
	}
	menuByValue := map[string]model.Menu{}
	for _, item := range items {
		menu, err := ensureMenu(db, item)
		if err != nil {
			return err
		}
		menuByValue[item.Value] = menu
	}

	buttons := []struct{ parentValue, value string }{
		{"console:business-topology", "console:business-topology:view"},
		{"system:admin:list", "system:admin:add"}, {"system:admin:list", "system:admin:edit"}, {"system:admin:list", "system:admin:delete"}, {"system:admin:list", "system:admin:status"}, {"system:admin:list", "system:admin:resetpwd"}, {"system:admin:list", "system:admin:ldapSync"},
		{"system:role:list", "system:role:add"}, {"system:role:list", "system:role:edit"}, {"system:role:list", "system:role:delete"}, {"system:role:list", "system:role:status"}, {"system:role:list", "system:role:assign"},
		{"system:menu:list", "system:menu:add"}, {"system:menu:list", "system:menu:edit"}, {"system:menu:list", "system:menu:delete"},
		{"system:dept:list", "system:dept:add"}, {"system:dept:list", "system:dept:edit"}, {"system:dept:list", "system:dept:delete"},
		{"system:post:list", "system:post:add"}, {"system:post:list", "system:post:edit"}, {"system:post:list", "system:post:delete"}, {"system:post:list", "system:post:status"},
		{"system:config:view", "system:config:save"}, {"system:config:view", "system:config:ldap"},
		{"system:loginlog:list", "system:loginlog:delete"}, {"system:loginlog:list", "system:loginlog:clean"},
		{"system:operationlog:list", "system:operationlog:delete"}, {"system:operationlog:list", "system:operationlog:clean"},
	}
	menuByValue["console:business-topology"] = businessTopologyMenu
	for sort, button := range buttons {
		parent, ok := menuByValue[button.parentValue]
		if !ok {
			continue
		}
		name := canonicalPermissionName(button.value)
		if name == "" {
			name = "Permission"
		}
		if _, err := ensureMenu(db, model.Menu{ParentID: parent.ID, MenuName: name, MenuType: 3, Value: button.value, MenuStatus: 1, Sort: sort + 1}); err != nil {
			return err
		}
	}
	return seedApplicationMenus(db)
}

func seedApplicationMenus(db *gorm.DB) error {
	type menuSeed struct{ url, value, icon string }
	type appSeed struct {
		url, value, icon string
		children         []menuSeed
	}
	type buttonSeed struct{ parentValue, value string }

	var retiredMenus []model.Menu
	if err := db.Where("value = ? OR url = ?", "applications:topology", "/applications/topology").Find(&retiredMenus).Error; err != nil {
		return err
	}
	if len(retiredMenus) > 0 {
		ids := make([]uint, 0, len(retiredMenus))
		for _, menu := range retiredMenus {
			ids = append(ids, menu.ID)
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("menu_id IN ?", ids).Delete(&model.RoleMenu{}).Error; err != nil {
				return err
			}
			return tx.Where("id IN ?", ids).Delete(&model.Menu{}).Error
		}); err != nil {
			return err
		}
	}

	applications := []appSeed{
		{url: "/assets", value: "assets", icon: "Box", children: []menuSeed{
			{"/assets/overview", "assets:overview", "DataBoard"}, {"/assets/environments", "ops:environment:list", "SetUp"}, {"/assets/terminal", "assets:terminal", "Platform"}, {"/assets/server/hosts", "assets:host:list", "Monitor"}, {"/assets/server/groups", "assets:hostgroup:list", "FolderOpened"}, {"/assets/server/credentials", "assets:credential:list", "Key"}, {"/assets/server/cloud-accounts", "assets:cloudaccount:list", "Cloudy"}, {"/assets/databases", "assets:database:list", "List"}, {"/assets/databases/workbench", "assets:database:workbench", "EditPen"}, {"/assets/databases/import", "assets:database:import", "Upload"}, {"/assets/databases/backups", "assets:database:backup", "FolderOpened"}, {"/assets/gateways", "assets:gateway:list", "Switch"},
		}},
		{url: "/containers", value: "containers", icon: "Box", children: []menuSeed{
			{"/containers/k8s/clusters", "assets:k8s:cluster", "FolderOpened"}, {"/containers/k8s/overview", "assets:k8s:overview", "DataAnalysis"}, {"/containers/k8s/nodes", "assets:k8s:node", "Monitor"}, {"/containers/k8s/namespaces", "assets:k8s:namespace", "Grid"}, {"/containers/k8s/workloads", "assets:k8s:workload", "SetUp"}, {"/containers/k8s/pods", "assets:k8s:pod", "Box"}, {"/containers/k8s/services", "assets:k8s:service", "Share"}, {"/containers/k8s/ingresses", "assets:k8s:ingress", "Connection"}, {"/containers/k8s/advanced-network", "assets:k8s:advancednetwork", "Connection"}, {"/containers/k8s/config-storage", "assets:k8s:configstorage", "Files"},
		}},
		{url: "/ops", value: "ops", icon: "Operation", children: []menuSeed{
			{"/ops/scripts/library", "ops:script:list", "Document"}, {"/ops/quick-exec/command", "ops:quickexec:command", "CaretRight"}, {"/ops/quick-exec/script", "ops:quickexec:script", "EditPen"}, {"/ops/quick-exec/file-dispatch", "ops:quickexec:file", "Files"}, {"/ops/quick-exec/history", "ops:quickexec:history", "DocumentCopy"}, {"/ops/schedule/tasks", "ops:schedule:task", "Clock"}, {"/ops/schedule/logs", "ops:schedule:log", "Document"}, {"/ops/schedule/templates", "ops:schedule:template", "Tickets"}, {"/ops/jobs/designer", "ops:job:designer", "Grid"}, {"/ops/jobs/list", "ops:job:list", "List"}, {"/ops/jobs/approvals", "ops:job:approval", "Bell"}, {"/ops/jobs/history", "ops:job:history", "Document"}, {"/ops/jobs/templates", "ops:job:template", "Tickets"},
		}},
		{url: "/applications", value: "applications", icon: "Box", children: []menuSeed{{"/applications/projects", "applications:project:list", "Tickets"}, {"/applications/build-tasks", "applications:buildtask:list", "Operation"}, {"/applications/build-history", "applications:buildhistory:list", "Document"}, {"/applications/pipelines", "applications:pipeline:list", "Share"}}},
		{url: "/notify", value: "notify", icon: "Bell", children: []menuSeed{{"/notify/rules", "notify:rule:list", "Operation"}, {"/notify/templates", "notify:template:list", "Document"}, {"/notify/channels", "notify:channel:list", "Connection"}, {"/notify/send-logs", "notify:sendlog:list", "Tickets"}}},
		{url: "/monitor", value: "monitor", icon: "Histogram", children: []menuSeed{{"/monitor/overview", "monitor:overview", "TrendCharts"}, {"/monitor/command-center", "monitor:commandcenter", "DataBoard"}, {"/monitor/datasources", "monitor:datasource:list", "Connection"}, {"/monitor/query", "monitor:query", "Search"}, {"/monitor/logs", "monitor:logs", "Document"}, {"/monitor/traces", "monitor:traces", "Share"}, {"/monitor/alert-templates", "monitor:alerttemplate:list", "CollectionTag"}, {"/monitor/alert-rules", "monitor:alertrule:list", "Bell"}, {"/monitor/alert-events", "monitor:alertevent:list", "Warning"}, {"/monitor/silences", "monitor:silence:list", "MuteNotification"}, {"/monitor/aggregations", "monitor:aggregation:list", "Filter"}, {"/monitor/dashboards", "monitor:dashboard:list", "PieChart"}, {"/monitor/inspections", "monitor:inspection:list", "Tickets"}}},
		{url: "/domains", value: "domains", icon: "Connection", children: []menuSeed{{"/domains/public", "domains:public:list", "Position"}, {"/domains/public/accounts", "domains:account:list", "Key"}, {"/domains/public/certificates", "domains:ssl:view", "Lock"}, {"/domains/internal", "domains:internal:list", "OfficeBuilding"}, {"/domains/internal/settings", "domains:settings:view", "Setting"}, {"/domains/query-test", "domains:query:test", "Search"}, {"/domains/audit", "domains:audit:list", "Memo"}}},
	}

	menuByValue := make(map[string]model.Menu)
	for appIndex, application := range applications {
		root, err := ensureMenu(db, model.Menu{ParentID: 0, MenuName: seedMenuName(application.value), MenuType: 1, URL: application.url, Value: application.value, MenuStatus: 1, Sort: 10 + appIndex, Icon: application.icon})
		if err != nil {
			return err
		}
		for childIndex, child := range application.children {
			menu, err := ensureMenu(db, model.Menu{ParentID: root.ID, MenuName: seedMenuName(child.value), MenuType: 2, URL: child.url, Value: child.value, MenuStatus: 1, Sort: childIndex + 1, Icon: child.icon})
			if err != nil {
				return err
			}
			menuByValue[child.value] = menu
		}
	}

	buttons := []buttonSeed{
		{"assets:host:list", "assets:host:add"}, {"assets:host:list", "assets:host:edit"}, {"assets:host:list", "assets:host:delete"}, {"assets:host:list", "assets:host:copy"}, {"assets:host:list", "assets:host:terminal"},
		{"assets:hostgroup:list", "assets:hostgroup:add"}, {"assets:hostgroup:list", "assets:hostgroup:edit"}, {"assets:hostgroup:list", "assets:hostgroup:delete"},
		{"assets:credential:list", "assets:credential:add"}, {"assets:credential:list", "assets:credential:edit"}, {"assets:credential:list", "assets:credential:delete"},
		{"assets:cloudaccount:list", "assets:cloudaccount:add"}, {"assets:cloudaccount:list", "assets:cloudaccount:edit"}, {"assets:cloudaccount:list", "assets:cloudaccount:delete"},
		{"assets:database:list", "assets:database:add"}, {"assets:database:list", "assets:database:edit"}, {"assets:database:list", "assets:database:delete"}, {"assets:database:list", "assets:database:test"},
		{"assets:database:workbench", "assets:database:sql:execute"}, {"assets:database:workbench", "assets:database:data:edit"}, {"assets:database:workbench", "assets:database:export"},
		{"assets:database:import", "assets:database:import:execute"}, {"assets:database:backup", "assets:database:backup:create"}, {"assets:database:backup", "assets:database:backup:restore"}, {"assets:database:backup", "assets:database:backup:delete"},
		{"assets:gateway:list", "assets:gateway:add"}, {"assets:gateway:list", "assets:gateway:edit"}, {"assets:gateway:list", "assets:gateway:delete"}, {"assets:gateway:list", "assets:gateway:test"},
		{"assets:k8s:cluster", "assets:k8s:cluster:add"}, {"assets:k8s:cluster", "assets:k8s:cluster:edit"}, {"assets:k8s:cluster", "assets:k8s:cluster:delete"},
		{"assets:k8s:workload", "assets:k8s:workload:scale"}, {"assets:k8s:workload", "assets:k8s:workload:restart"}, {"assets:k8s:workload", "assets:k8s:workload:image"}, {"assets:k8s:workload", "assets:k8s:workload:yaml"},
		{"assets:k8s:pod", "assets:k8s:pod:terminal"}, {"assets:k8s:pod", "assets:k8s:pod:delete"}, {"assets:k8s:pod", "assets:k8s:pod:yaml"},
		{"ops:script:list", "ops:script:add"}, {"ops:script:list", "ops:script:edit"}, {"ops:script:list", "ops:script:delete"}, {"ops:script:list", "ops:script:status"},
		{"ops:quickexec:command", "ops:quickexec:command:execute"}, {"ops:quickexec:script", "ops:quickexec:script:execute"}, {"ops:quickexec:file", "ops:quickexec:file:execute"},
		{"ops:schedule:task", "ops:schedule:task:add"}, {"ops:schedule:task", "ops:schedule:task:edit"}, {"ops:schedule:task", "ops:schedule:task:delete"}, {"ops:schedule:task", "ops:schedule:task:status"},
		{"ops:job:designer", "ops:job:template:save"}, {"ops:job:list", "ops:job:execute"}, {"ops:job:list", "ops:job:delete"}, {"ops:job:approval", "ops:job:approve"},
		{"applications:project:list", "applications:project:add"}, {"applications:project:list", "applications:project:edit"}, {"applications:project:list", "applications:project:delete"},
		{"applications:buildtask:list", "applications:buildtask:add"}, {"applications:buildtask:list", "applications:buildtask:edit"}, {"applications:buildtask:list", "applications:buildtask:delete"}, {"applications:buildtask:list", "applications:buildtask:run"},
		{"applications:pipeline:list", "applications:pipeline:add"}, {"applications:pipeline:list", "applications:pipeline:edit"}, {"applications:pipeline:list", "applications:pipeline:delete"}, {"applications:pipeline:list", "applications:pipeline:run"},
		{"notify:rule:list", "notify:rule:add"}, {"notify:rule:list", "notify:rule:edit"}, {"notify:rule:list", "notify:rule:delete"},
		{"notify:template:list", "notify:template:add"}, {"notify:template:list", "notify:template:edit"}, {"notify:template:list", "notify:template:delete"},
		{"notify:channel:list", "notify:channel:add"}, {"notify:channel:list", "notify:channel:edit"}, {"notify:channel:list", "notify:channel:delete"}, {"notify:channel:list", "notify:channel:test"},
		{"monitor:datasource:list", "monitor:datasource:add"}, {"monitor:datasource:list", "monitor:datasource:edit"}, {"monitor:datasource:list", "monitor:datasource:delete"}, {"monitor:datasource:list", "monitor:datasource:test"},
		{"monitor:alerttemplate:list", "monitor:alerttemplate:add"}, {"monitor:alerttemplate:list", "monitor:alerttemplate:edit"}, {"monitor:alerttemplate:list", "monitor:alerttemplate:delete"},
		{"monitor:alertrule:list", "monitor:alertrule:add"}, {"monitor:alertrule:list", "monitor:alertrule:edit"}, {"monitor:alertrule:list", "monitor:alertrule:delete"}, {"monitor:alertrule:list", "monitor:alertrule:batch"},
		{"monitor:alertevent:list", "monitor:alertevent:claim"}, {"monitor:alertevent:list", "monitor:alertevent:close"}, {"monitor:alertevent:list", "monitor:alertevent:delete"},
		{"monitor:silence:list", "monitor:silence:add"}, {"monitor:silence:list", "monitor:silence:edit"}, {"monitor:silence:list", "monitor:silence:delete"},
		{"monitor:aggregation:list", "monitor:aggregation:add"}, {"monitor:aggregation:list", "monitor:aggregation:edit"}, {"monitor:aggregation:list", "monitor:aggregation:delete"},
		{"monitor:dashboard:list", "monitor:dashboard:add"}, {"monitor:dashboard:list", "monitor:dashboard:edit"}, {"monitor:dashboard:list", "monitor:dashboard:delete"},
		{"domains:account:list", "domains:account:add"}, {"domains:account:list", "domains:account:edit"}, {"domains:account:list", "domains:account:delete"}, {"domains:account:list", "domains:account:test"},
		{"domains:public:list", "domains:public:sync"}, {"domains:public:list", "domains:public:record"}, {"domains:public:list", "domains:public:batch"},
		{"domains:ssl:view", "domains:ssl:sync"}, {"domains:ssl:view", "domains:ssl:apply"}, {"domains:ssl:view", "domains:ssl:upload"}, {"domains:ssl:view", "domains:ssl:renew"}, {"domains:ssl:view", "domains:ssl:delete"}, {"domains:ssl:view", "domains:ssl:download"}, {"domains:ssl:view", "domains:ssl:download-key"}, {"domains:ssl:view", "domains:ssl:settings"},
		{"domains:internal:list", "domains:internal:zone:add"}, {"domains:internal:list", "domains:internal:zone:edit"}, {"domains:internal:list", "domains:internal:zone:delete"}, {"domains:internal:list", "domains:internal:record"},
		{"domains:settings:view", "domains:settings:save"},
	}
	for sort, button := range buttons {
		parent, found := menuByValue[button.parentValue]
		if !found {
			continue
		}
		name := canonicalPermissionName(button.value)
		if name == "" {
			name = "Permission"
		}
		if _, err := ensureMenu(db, model.Menu{ParentID: parent.ID, MenuName: name, MenuType: 3, Value: button.value, MenuStatus: 1, Sort: sort + 1}); err != nil {
			return err
		}
	}
	return nil
}

func seedAdmin(db *gorm.DB) error {
	var count int64
	db.Model(&model.Admin{}).Count(&count)
	if count > 0 {
		return nil
	}
	initialPassword := strings.TrimSpace(os.Getenv("OPS_ADMIN_INITIAL_PASSWORD"))
	if initialPassword == "" {
		return errors.New("OPS_ADMIN_INITIAL_PASSWORD must be set before initializing the first administrator")
	}
	username := strings.TrimSpace(os.Getenv("OPS_ADMIN_INITIAL_USERNAME"))
	if username == "" {
		username = "admin"
	}
	var dept model.Dept
	var post model.Post
	var role model.Role
	if err := db.First(&dept).Error; err != nil {
		return err
	}
	if err := db.First(&post).Error; err != nil {
		return err
	}
	if err := db.First(&role).Error; err != nil {
		return err
	}
	admin := model.Admin{PostID: post.ID, DeptID: dept.ID, Username: username, Password: util.HashPassword(initialPassword), Nickname: "System Administrator", Status: 1, Email: "admin@example.com", Phone: "13800000000", Note: "Bootstrap administrator", CreatedAt: time.Now()}
	if err := db.Create(&admin).Error; err != nil {
		return err
	}
	return db.Create(&model.AdminRole{AdminID: admin.ID, RoleID: role.ID}).Error
}

func seedSuperRolePermissions(db *gorm.DB) error {
	var role model.Role
	if err := db.Where("role_key = ?", "super-admin").First(&role).Error; err != nil {
		return err
	}
	var menus []model.Menu
	if err := db.Find(&menus).Error; err != nil {
		return err
	}
	for _, menu := range menus {
		var count int64
		if err := db.Model(&model.RoleMenu{}).Where("role_id = ? and menu_id = ?", role.ID, menu.ID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if err := db.Create(&model.RoleMenu{RoleID: role.ID, MenuID: menu.ID}).Error; err != nil {
			return err
		}
	}
	return nil
}

// seedRoutePermissionMenus (always-on) ensures every route permission string
// in the opdef table has a status-1 sys_menu row the grant tables can point
// at. Values that already exist (seeded page menus and buttons) are reused
// untouched — their row is never modified; values with no seeded row are
// created as type-3 leaves under the hidden status-0 route-permissions root.
func seedRoutePermissionMenus(db *gorm.DB) error {
	root, err := ensureMenu(db, model.Menu{ParentID: 0, MenuName: "Route Permissions", MenuType: 1, URL: "", Value: routePermissionsRootValue, MenuStatus: 0, Sort: 99})
	if err != nil {
		return err
	}
	// The menu model declares gorm default:1, so a create with the zero value
	// would be stored as status 1; the hidden root must stay status 0.
	if root.MenuStatus != 0 {
		if err := db.Model(&model.Menu{}).Where("id = ?", root.ID).Update("menu_status", 0).Error; err != nil {
			return err
		}
	}
	for i, permission := range opdef.PermissionStrings() {
		var count int64
		if err := db.Model(&model.Menu{}).Where("value = ?", permission).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		name := canonicalPermissionName(permission)
		if name == "" {
			name = "Permission"
		}
		menu := model.Menu{ParentID: root.ID, MenuName: name, MenuType: 3, Value: permission, MenuStatus: 1, Sort: i + 1, CreatedAt: time.Now()}
		if err := db.Create(&menu).Error; err != nil {
			return err
		}
	}
	return nil
}

// grantRoutePermissionMenus grants every status-1 route permission menu to one
// role, skipping grants that already exist.
func grantRoutePermissionMenus(db *gorm.DB, roleID uint) error {
	var menus []model.Menu
	if err := db.Where("value IN ? AND menu_status = 1", opdef.PermissionStrings()).Find(&menus).Error; err != nil {
		return err
	}
	for _, menu := range menus {
		var count int64
		if err := db.Model(&model.RoleMenu{}).Where("role_id = ? AND menu_id = ?", roleID, menu.ID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if err := db.Create(&model.RoleMenu{RoleID: roleID, MenuID: menu.ID}).Error; err != nil {
			return err
		}
	}
	return nil
}

// seedSuperAdminRoutePermissions (always-on) re-grants the full route
// vocabulary to super-admin on every boot, keeping the existing
// "super-admin holds everything" invariant when new definitions appear later.
func seedSuperAdminRoutePermissions(db *gorm.DB) error {
	var role model.Role
	if err := db.Where("role_key = ?", "super-admin").First(&role).Error; err != nil {
		return err
	}
	return grantRoutePermissionMenus(db, role.ID)
}

// migrateRoleRoutePermissionsOnce (one-shot, marker-gated) grants the full
// route vocabulary to every role that exists at migration time and then
// creates the marker row. With the marker present it is a complete no-op, so
// roles created afterwards start with zero route permissions and operator
// revocations survive reboots. The grant pass and the marker creation run in
// one transaction so a failure cannot leave the router enforcement without
// the corresponding grants.
func migrateRoleRoutePermissionsOnce(db *gorm.DB) error {
	var marker model.Menu
	err := db.Where("value = ?", routePermissionsMarkerValue).First(&marker).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var roles []model.Role
		if err := tx.Find(&roles).Error; err != nil {
			return err
		}
		for _, role := range roles {
			if err := grantRoutePermissionMenus(tx, role.ID); err != nil {
				return err
			}
		}
		var root model.Menu
		if err := tx.Where("value = ?", routePermissionsRootValue).First(&root).Error; err != nil {
			return err
		}
		return tx.Create(&model.Menu{ParentID: root.ID, MenuName: "Route Permissions Granted (v1)", MenuType: 3, Value: routePermissionsMarkerValue, MenuStatus: 1, CreatedAt: time.Now()}).Error
	})
}

func ensureMenu(db *gorm.DB, menu model.Menu) (model.Menu, error) {
	var existing model.Menu
	err := db.Where("value = ? AND value <> ''", menu.Value).First(&existing).Error
	if err == nil {
		updates := map[string]any{"parent_id": menu.ParentID, "menu_name": menu.MenuName, "menu_type": menu.MenuType, "url": menu.URL, "menu_status": menu.MenuStatus, "sort": menu.Sort, "icon": menu.Icon}
		if updateErr := db.Model(&existing).Updates(updates).Error; updateErr != nil {
			return existing, updateErr
		}
		return existing, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return existing, err
	}
	menu.CreatedAt = time.Now()
	if err := db.Create(&menu).Error; err != nil {
		return existing, err
	}
	return menu, nil
}
