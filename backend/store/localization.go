package store

import (
	"fmt"
	"strings"

	"ops-admin/backend/model"

	"gorm.io/gorm"
)

// CanonicalizeSeedLocalization keeps persisted platform-owned metadata in English.
// The frontend is responsible for rendering localized Korean labels from stable
// paths, permission values, and translation keys.
func CanonicalizeSeedLocalization(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := canonicalizeSystemDefaults(tx); err != nil {
			return err
		}
		if err := canonicalizeBuiltInMenus(tx); err != nil {
			return err
		}
		if err := canonicalizeAlertTemplates(tx); err != nil {
			return err
		}
		return nil
	})
}

func canonicalizeSystemDefaults(db *gorm.DB) error {
	legacySiteSlogan := "\u4e2a\u4eba\u8fd0\u7ef4\u7ba1\u7406\u5e73\u53f0"
	legacyLoginSubtitle := "\u7cfb\u7edf\u7ba1\u7406\u4e0e\u8fd0\u7ef4\u63a7\u5236\u53f0"
	if err := db.Model(&model.SystemConfig{}).
		Where("site_slogan = ?", legacySiteSlogan).
		Update("site_slogan", "Personal operations platform").Error; err != nil {
		return err
	}
	if err := db.Model(&model.SystemConfig{}).
		Where("login_subtitle = ?", legacyLoginSubtitle).
		Update("login_subtitle", "System management and operations console").Error; err != nil {
		return err
	}

	if err := db.Model(&model.Dept{}).
		Where("dept_name = ?", "\u603b\u90e8").
		Update("dept_name", "Headquarters").Error; err != nil {
		return err
	}
	if err := db.Model(&model.Post{}).
		Where("post_code = ?", "super-admin").
		Updates(map[string]any{"post_name": "Super Administrator", "remark": "System bootstrap position"}).Error; err != nil {
		return err
	}
	if err := db.Model(&model.Role{}).
		Where("role_key = ?", "super-admin").
		Updates(map[string]any{"role_name": "Super Administrator", "description": "System bootstrap role"}).Error; err != nil {
		return err
	}
	if err := db.Model(&model.Admin{}).
		Where("nickname = ?", "\u7cfb\u7edf\u7ba1\u7406\u5458").
		Update("nickname", "System Administrator").Error; err != nil {
		return err
	}
	if err := db.Model(&model.Admin{}).
		Where("note = ?", "\u521d\u59cb\u5316\u7ba1\u7406\u5458").
		Update("note", "Bootstrap administrator").Error; err != nil {
		return err
	}
	return nil
}

func canonicalizeBuiltInMenus(db *gorm.DB) error {
	menuNames := map[string]string{
		"system": "System Management",
		"console:business-topology": "Business Topology",
		"logs": "Operation Audit",
		"system:admin:list": "Users",
		"system:role:list": "Roles",
		"system:menu:list": "Menus",
		"system:dept:list": "Departments",
		"system:post:list": "Positions",
		"system:config:view": "System Settings",
		"system:loginlog:list": "Login Log",
		"system:operationlog:list": "Operation Log",

		"assets": "Asset Management",
		"assets:overview": "Asset Overview",
		"ops:environment:list": "Environment Model",
		"assets:terminal": "Terminal Login",
		"assets:host:list": "Host Management",
		"assets:hostgroup:list": "Host Group Management",
		"assets:credential:list": "Credential Management",
		"assets:cloudaccount:list": "Cloud Account Management",
		"assets:database:list": "Database Management",
		"assets:database:workbench": "DBMS Workbench",
		"assets:database:import": "Data Import",
		"assets:database:backup": "Backup Management",
		"assets:gateway:list": "Gateway Management",

		"containers": "Container Management",
		"assets:k8s:cluster": "Cluster Management",
		"assets:k8s:overview": "Cluster Overview",
		"assets:k8s:node": "Node Management",
		"assets:k8s:namespace": "Namespaces",
		"assets:k8s:workload": "Workloads",
		"assets:k8s:pod": "Pods",
		"assets:k8s:service": "Services",
		"assets:k8s:ingress": "Ingress",
		"assets:k8s:advancednetwork": "Advanced Network",
		"assets:k8s:configstorage": "Config & Storage",

		"ops": "Standard Operations",
		"ops:script:list": "Script Library",
		"ops:quickexec:command": "Command Execution",
		"ops:quickexec:script": "Script Execution",
		"ops:quickexec:file": "File Distribution",
		"ops:quickexec:history": "Quick Execution History",
		"ops:schedule:task": "Scheduled Tasks",
		"ops:schedule:log": "Task Logs",
		"ops:schedule:template": "Task Templates",
		"ops:job:designer": "Job Designer",
		"ops:job:list": "Job List",
		"ops:job:approval": "Manual Approvals",
		"ops:job:history": "Job History",
		"ops:job:template": "Job Templates",

		"applications": "Application Center",
		"applications:project:list": "Application Management",
		"applications:buildtask:list": "Build Tasks",
		"applications:buildhistory:list": "Build History",
		"applications:pipeline:list": "CI/CD Pipelines",

		"notify": "Notifications",
		"notify:rule:list": "Notification Rules",
		"notify:template:list": "Message Templates",
		"notify:channel:list": "Notification Channels",
		"notify:sendlog:list": "Send Logs",

		"monitor": "Monitoring Center",
		"monitor:overview": "Monitoring Overview",
		"monitor:commandcenter": "Operations Command Center",
		"monitor:datasource:list": "Datasource Management",
		"monitor:query": "Instant Query",
		"monitor:logs": "Log Explorer",
		"monitor:traces": "Trace Explorer",
		"monitor:alerttemplate:list": "Alert Templates",
		"monitor:alertrule:list": "Alert Rules",
		"monitor:alertevent:list": "Alert Events",
		"monitor:silence:list": "Alert Silences",
		"monitor:aggregation:list": "Alert Aggregation",
		"monitor:dashboard:list": "Monitoring Dashboards",
		"monitor:inspection:list": "Inspection Dashboards",

		"domains": "Domain Management",
		"domains:public:list": "Public Domains",
		"domains:account:list": "Public DNS Accounts",
		"domains:ssl:view": "SSL Certificates",
		"domains:internal:list": "Internal Domains",
		"domains:settings:view": "DNS Settings",
		"domains:query:test": "DNS Query Test",
		"domains:audit:list": "Operation Audit",
	}

	for value, name := range menuNames {
		if err := db.Model(&model.Menu{}).Where("value = ?", value).Update("menu_name", name).Error; err != nil {
			return err
		}
	}

	var permissions []model.Menu
	if err := db.Where("menu_type = ?", 3).Find(&permissions).Error; err != nil {
		return err
	}
	for _, item := range permissions {
		if name := canonicalPermissionName(item.Value); name != "" && item.MenuName != name {
			if err := db.Model(&item).Update("menu_name", name).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func canonicalPermissionName(value string) string {
	full := map[string]string{
		"console:business-topology:view": "View Business Topology",
		"system:admin:resetpwd": "Reset Password",
		"system:admin:ldapSync": "Sync LDAP Users",
		"system:role:assign": "Assign Permissions",
		"system:config:ldap": "Configure LDAP Integration",
		"assets:host:terminal": "Open Terminal",
		"assets:database:sql:execute": "Execute SQL",
		"assets:database:data:edit": "Edit Data",
		"assets:database:export": "Export Data",
		"assets:database:import:execute": "Execute Data Import",
		"assets:database:backup:create": "Create Manual Backup",
		"assets:database:backup:restore": "Restore Backup",
		"assets:database:backup:delete": "Delete Backup",
		"assets:k8s:workload:scale": "Scale Workload",
		"assets:k8s:workload:restart": "Restart Workload",
		"assets:k8s:workload:image": "Update Image",
		"assets:k8s:workload:yaml": "Edit YAML",
		"assets:k8s:pod:terminal": "Open Pod Terminal",
		"assets:k8s:pod:yaml": "Edit YAML",
		"ops:job:template:save": "Save Job Template",
		"ops:job:approve": "Approve Job Step",
		"domains:public:sync": "Sync Public Domains",
		"domains:public:record": "Manage Public DNS Records",
		"domains:public:batch": "Batch Public DNS Records",
		"domains:ssl:sync": "Sync SSL Certificates",
		"domains:ssl:apply": "Request SSL Certificate",
		"domains:ssl:upload": "Upload SSL Certificate",
		"domains:ssl:renew": "Renew SSL Certificate",
		"domains:ssl:download": "Download SSL Certificate",
		"domains:ssl:download-key": "Download Certificate Private Key",
		"domains:ssl:settings": "Change Auto Renewal",
		"domains:internal:record": "Manage Internal DNS Records",
	}
	if name := full[value]; name != "" {
		return name
	}
	parts := strings.Split(value, ":")
	if len(parts) < 2 {
		return ""
	}
	action := parts[len(parts)-1]
	actions := map[string]string{
		"add": "Add",
		"edit": "Edit",
		"delete": "Delete",
		"status": "Change Status",
		"test": "Test",
		"copy": "Copy",
		"run": "Run",
		"execute": "Execute",
		"clean": "Clear",
		"save": "Save",
		"claim": "Claim",
		"close": "Close",
		"batch": "Batch Update",
	}
	return actions[action]
}

type canonicalAlertTemplate struct {
	query       string
	name        string
	objectType  string
	description string
}

func canonicalizeAlertTemplates(db *gorm.DB) error {
	templates := []canonicalAlertTemplate{
		{`up{job=~"node.*|node_exporter"}`, "Host Exporter Down", "host", "The exporter target is continuously unreachable; verify network connectivity, the exporter process, and scrape configuration."},
		{`100 - (avg by(instance) (irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)`, "Host CPU Usage High", "host", "CPU utilization remains high; correlate with system load and high-consumption processes."},
		{`node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes * 100`, "Host Available Memory Ratio Low", "host", "Available memory remains low; check memory pressure, leaks, cache growth, and capacity."},
		{`(100 - ((node_filesystem_avail_bytes{fstype!~"tmpfs|overlay"} * 100) / node_filesystem_size_bytes{fstype!~"tmpfs|overlay"}))`, "Host Filesystem Usage High", "host", "Filesystem capacity is near its limit; clean logs and temporary files or expand storage."},
		{`node_netstat_Tcp_CurrEstab`, "Host TCP Connections High", "host", "Established TCP connections remain high; inspect connection leaks, traffic spikes, and short-connection behavior."},
		{`node_sockstat_UDP_inuse`, "Host UDP Sockets High", "host", "UDP sockets in use remain high; inspect abnormal traffic and service behavior."},
		{`avg by(instance) (rate(node_cpu_seconds_total{mode="user"}[5m])) * 100`, "Host User CPU Usage High", "host", "User-space CPU utilization remains high; identify high-consumption application processes."},
		{`avg by(instance) (rate(node_cpu_seconds_total{mode="system"}[5m])) * 100`, "Host System CPU Usage High", "host", "Kernel-space CPU utilization remains high; inspect system calls, networking, and disk I/O."},
		{`100 - (node_filesystem_files_free{mountpoint="/",fstype!~"tmpfs|overlay"} / node_filesystem_files{mountpoint="/",fstype!~"tmpfs|overlay"} * 100)`, "Root Filesystem Inode Usage High", "host", "Root filesystem inode usage is near its limit; inspect excessive small-file creation."},
		{`sum by(instance) (rate(node_disk_read_bytes_total[5m]))`, "Host Disk Read Throughput High", "host", "Disk read throughput remains above 100 MiB/s."},
		{`sum by(instance) (rate(node_disk_written_bytes_total[5m]))`, "Host Disk Write Throughput High", "host", "Disk write throughput remains above 100 MiB/s."},
		{`sum by(instance) (rate(node_disk_reads_completed_total[5m]))`, "Host Disk Read IOPS High", "host", "Disk read IOPS remains high; inspect hot requests and storage performance."},
		{`sum by(instance) (rate(node_disk_writes_completed_total[5m]))`, "Host Disk Write IOPS High", "host", "Disk write IOPS remains high; inspect write amplification and batch jobs."},
		{`node_load1`, "Host 1m Load Average High", "host", "The one-minute load average remains high."},
		{`node_load5`, "Host 5m Load Average High", "host", "The five-minute load average remains high."},
		{`node_load15`, "Host 15m Load Average High", "host", "The fifteen-minute load average remains high."},
		{`node_load1 / count by(instance) (node_cpu_seconds_total{mode="idle"})`, "Host 1m Load per CPU High", "host", "The one-minute load-to-CPU ratio remains above one."},
		{`node_load5 / count by(instance) (node_cpu_seconds_total{mode="idle"})`, "Host 5m Load per CPU High", "host", "The five-minute load-to-CPU ratio remains above one."},
		{`node_load15 / count by(instance) (node_cpu_seconds_total{mode="idle"})`, "Host 15m Load per CPU High", "host", "The fifteen-minute load-to-CPU ratio remains above one."},
		{`(1 - (node_memory_SwapFree_bytes / node_memory_SwapTotal_bytes)) * 100`, "Host Swap Usage High", "host", "Swap utilization remains high; inspect memory pressure."},
		{`sum by(instance) (rate(node_network_receive_bytes_total{device!~"lo"}[5m]))`, "Host Network Receive Throughput High", "host", "Network receive throughput remains above 100 MiB/s."},
		{`sum by(instance) (rate(node_network_transmit_bytes_total{device!~"lo"}[5m]))`, "Host Network Transmit Throughput High", "host", "Network transmit throughput remains above 100 MiB/s."},
		{`time() - node_boot_time_seconds`, "Host Recently Rebooted", "host", "Host uptime is less than one day; verify whether the reboot was planned."},
		{`node_filefd_allocated`, "Host Open File Descriptors High", "host", "Allocated file descriptors remain high; inspect descriptor leaks and system limits."},

		{`kube_node_status_condition{condition="Ready",status="true"}`, "Kubernetes Node Not Ready", "node", "A node has remained NotReady for five minutes; inspect kubelet, network, and node resources."},
		{`sum by (namespace, pod, phase) (kube_pod_status_phase{phase=~"Failed|Unknown|Pending"})`, "Kubernetes Pod Abnormal State", "pod", "Aggregates Failed, Unknown, and long-running Pending Pods."},
		{`sum by (namespace, pod) (increase(kube_pod_container_status_restarts_total[10m]))`, "Kubernetes Pod Restart Rate High", "pod", "More than three restarts in ten minutes; inspect OOM events, probe failures, and application errors."},
		{`kube_deployment_status_replicas_unavailable`, "Kubernetes Deployment Unavailable Replicas", "deployment", "Deployment replicas are unavailable; inspect images, resources, scheduling, and events."},

		{`mysql_up`, "MySQL Service Down", "database", "The MySQL target is continuously unreachable; verify the process, network, credentials, and exporter."},
		{`mysql_global_status_uptime`, "MySQL Recently Restarted", "database", "MySQL uptime is below five minutes; verify whether the restart was planned or caused by a failure."},
		{`(mysql_global_status_threads_connected / clamp_min(mysql_global_variables_max_connections, 1)) * 100`, "MySQL Connection Usage High", "database", "Connection usage remains above 85%; inspect leaks, slow queries, and connection pool settings."},
		{`mysql_global_status_threads_running`, "MySQL Running Threads High", "database", "Running threads remain high; inspect slow SQL, lock waits, and traffic spikes."},
		{`increase(mysql_global_status_slow_queries[5m])`, "MySQL Slow Queries Spike", "database", "More than ten slow queries were added in five minutes; inspect execution plans, indexes, and hot SQL."},
		{`increase(mysql_global_status_aborted_clients[5m])`, "MySQL Aborted Clients Spike", "database", "Aborted clients continue to increase; inspect networking, connection timeouts, and application pools."},
		{`increase(mysql_global_status_aborted_connects[5m])`, "MySQL Failed Connections Spike", "database", "Connection failures continue to increase; inspect authentication, network, firewalls, and connection limits."},
		{`(mysql_global_status_buffer_pool_bytes_dirty / clamp_min(mysql_global_status_buffer_pool_bytes_total, 1)) * 100`, "MySQL InnoDB Dirty Page Ratio High", "database", "InnoDB dirty-page ratio remains high; inspect disk I/O, flushing, and write pressure."},
		{`(1 - rate(mysql_global_status_innodb_buffer_pool_reads[5m]) / clamp_min(rate(mysql_global_status_innodb_buffer_pool_read_requests[5m]), 1)) * 100`, "MySQL InnoDB Buffer Pool Hit Ratio Low", "database", "Buffer pool hit ratio remains below 99%; inspect memory sizing, working set, and full table scans."},
		{`increase(mysql_global_status_innodb_row_lock_waits[5m])`, "MySQL Row Lock Waits High", "database", "Row-lock waits are high within five minutes; identify long transactions and conflicting SQL."},
		{`increase(mysql_global_status_innodb_row_lock_time[5m]) / 1000`, "MySQL Row Lock Wait Time High", "database", "Accumulated row-lock wait time exceeds ten seconds in five minutes; inspect blocking transactions first."},
		{`(rate(mysql_global_status_created_tmp_disk_tables[5m]) / clamp_min(rate(mysql_global_status_created_tmp_tables[5m]), 1)) * 100`, "MySQL Disk Temporary Table Ratio High", "database", "Disk temporary tables remain above 25%; inspect sort/group queries and temporary-table settings."},
		{`(mysql_global_status_open_files / clamp_min(mysql_global_variables_open_files_limit, 1)) * 100`, "MySQL Open Files Usage High", "database", "Open-file usage remains above 85%; inspect table count, connections, and file descriptor limits."},
		{`(mysql_global_status_prepared_stmt_count / clamp_min(mysql_global_variables_max_prepared_stmt_count, 1)) * 100`, "MySQL Prepared Statement Usage High", "database", "Prepared-statement usage remains above 85%; inspect statement cleanup and configured limits."},
		{`mysql_scrape_use_seconds`, "MySQL Exporter Scrape Duration High", "database", "Exporter scrape duration remains above five seconds; inspect database load, network, and scrape permissions."},

		{`redis_up`, "Redis Service Down", "database", "The Redis target is continuously unreachable; verify the process, network, authentication, and exporter."},
		{`redis_uptime_in_seconds`, "Redis Recently Restarted", "database", "Redis uptime is below five minutes; verify whether the restart was planned or caused by a failure."},
		{`100 * redis_memory_used_bytes / clamp_min((redis_memory_max_bytes > 0) or on(instance) redis_memory_used_peak_bytes, 1)`, "Redis Memory Usage High", "database", "Memory usage remains above 85%; inspect maxmemory, eviction policy, and large keys."},
		{`100 * rate(redis_keyspace_hits_total[5m]) / clamp_min(rate(redis_keyspace_hits_total[5m]) + rate(redis_keyspace_misses_total[5m]), 1)`, "Redis Cache Hit Ratio Low", "database", "Cache hit ratio remains below 80%; inspect cache warm-up, expiration policy, and access patterns."},
		{`histogram_quantile(0.95, sum by (le, instance) (rate(redis_commands_latencies_usec_bucket[5m]))) / 1000`, "Redis P95 Response Latency High", "database", "P95 command latency remains above 50 ms; inspect slow commands, CPU, network, and persistence pressure."},
		{`rate(redis_total_error_replies[5m])`, "Redis Error Reply Rate High", "database", "Error reply rate remains above one per second; inspect invalid commands, permissions, and client behavior."},
		{`increase(redis_rejected_connections_total[5m])`, "Redis Rejected Connections", "database", "Rejected connections occurred; inspect maxclients, file descriptors, and connection leaks."},
		{`redis_blocked_clients`, "Redis Blocked Clients High", "database", "Blocked clients remain above 50; inspect blocking commands, slow Lua scripts, and downstream dependencies."},
		{`increase(redis_evicted_keys_total[5m])`, "Redis Key Evictions", "database", "Key evictions occurred, indicating memory pressure or an eviction policy affecting the cache."},
		{`redis_slowlog_length`, "Redis Slowlog Backlog High", "database", "Slowlog length remains above ten; inspect slow commands, large keys, and blocking operations."},
	}

	for _, item := range templates {
		updates := map[string]any{
			"name":             item.name,
			"object_type":      item.objectType,
			"annotations_json": fmt.Sprintf(`{"summary":%q}`, item.name),
			"description":      item.description,
		}
		if err := db.Model(&model.MonitorAlertTemplate{}).
			Where("source = ? AND query_text = ?", "platform", item.query).
			Updates(updates).Error; err != nil {
			return err
		}
	}
	return nil
}
