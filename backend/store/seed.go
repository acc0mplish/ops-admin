package store

import (
	"errors"
	"time"

	"ops-admin/backend/model"
	"ops-admin/backend/util"

	"gorm.io/gorm"
)

func Seed(db *gorm.DB) error {
	steps := []func(*gorm.DB) error{
		seedSystemConfig,
		seedDept,
		seedPost,
		seedRole,
		seedMenus,
		seedAdmin,
		seedSuperRolePermissions,
	}

	for _, step := range steps {
		if err := step(db); err != nil {
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
		var cfg model.SystemConfig
		if err := db.First(&cfg).Error; err != nil {
			return err
		}
		updates := map[string]any{}
		if cfg.SiteSlogan == "Personal operations platform" {
			updates["site_slogan"] = "个人运维管理平台"
		}
		if cfg.LoginSubtitle == "System management and operation console" {
			updates["login_subtitle"] = "系统管理与运维控制台"
		}
		if len(updates) == 0 {
			return nil
		}
		return db.Model(&cfg).Updates(updates).Error
	}

	return db.Create(&model.SystemConfig{
		SiteName:           "Ops Admin",
		SiteSlogan:         "个人运维管理平台",
		LogoType:           "text",
		LogoValue:          "OA",
		LoginTitle:         "Ops Admin",
		LoginSubtitle:      "系统管理与运维控制台",
		UseLoginBackground: false,
		PrimaryColor:       "#5b6cf9",
		SidebarTheme:       "dark",
		CreatedAt:          time.Now(),
	}).Error
}

func seedDept(db *gorm.DB) error {
	var count int64
	db.Model(&model.Dept{}).Count(&count)
	if count > 0 {
		return nil
	}
	return db.Create(&model.Dept{
		ParentID:   0,
		DeptType:   1,
		DeptName:   "总部",
		DeptStatus: 1,
		CreatedAt:  time.Now(),
	}).Error
}

func seedPost(db *gorm.DB) error {
	var count int64
	db.Model(&model.Post{}).Count(&count)
	if count > 0 {
		return nil
	}
	return db.Create(&model.Post{
		PostCode:   "super-admin",
		PostName:   "超级管理员",
		PostStatus: 1,
		Remark:     "系统初始化岗位",
		CreatedAt:  time.Now(),
	}).Error
}

func seedRole(db *gorm.DB) error {
	var count int64
	db.Model(&model.Role{}).Count(&count)
	if count > 0 {
		return nil
	}
	return db.Create(&model.Role{
		RoleName:    "超级管理员",
		RoleKey:     "super-admin",
		Status:      1,
		Description: "系统初始化角色",
		CreatedAt:   time.Now(),
	}).Error
}

func seedMenus(db *gorm.DB) error {
	systemRoot, err := ensureMenu(db, model.Menu{
		ParentID:   0,
		MenuName:   "系统管理",
		MenuType:   1,
		URL:        "/system",
		Value:      "system",
		MenuStatus: 1,
		Sort:       1,
		Icon:       "Setting",
	})
	if err != nil {
		return err
	}
	businessTopologyMenu, err := ensureMenu(db, model.Menu{
		ParentID:   0,
		MenuName:   "业务拓扑图",
		MenuType:   2,
		URL:        "/business-topology",
		Value:      "console:business-topology",
		MenuStatus: 1,
		Sort:       0,
		Icon:       "Share",
	})
	if err != nil {
		return err
	}

	logRoot, err := ensureMenu(db, model.Menu{
		ParentID:   0,
		MenuName:   "操作审计",
		MenuType:   1,
		URL:        "/logs",
		Value:      "logs",
		MenuStatus: 1,
		Sort:       2,
		Icon:       "Document",
	})
	if err != nil {
		return err
	}

	adminMenu, err := ensureMenu(db, model.Menu{ParentID: systemRoot.ID, MenuName: "用户信息", MenuType: 2, URL: "/system/admin", Value: "system:admin:list", MenuStatus: 1, Sort: 1, Icon: "User"})
	if err != nil {
		return err
	}
	roleMenu, err := ensureMenu(db, model.Menu{ParentID: systemRoot.ID, MenuName: "角色信息", MenuType: 2, URL: "/system/role", Value: "system:role:list", MenuStatus: 1, Sort: 2, Icon: "Avatar"})
	if err != nil {
		return err
	}
	menuMenu, err := ensureMenu(db, model.Menu{ParentID: systemRoot.ID, MenuName: "菜单信息", MenuType: 2, URL: "/system/menu", Value: "system:menu:list", MenuStatus: 1, Sort: 3, Icon: "Grid"})
	if err != nil {
		return err
	}
	deptMenu, err := ensureMenu(db, model.Menu{ParentID: systemRoot.ID, MenuName: "部门信息", MenuType: 2, URL: "/system/dept", Value: "system:dept:list", MenuStatus: 1, Sort: 4, Icon: "OfficeBuilding"})
	if err != nil {
		return err
	}
	postMenu, err := ensureMenu(db, model.Menu{ParentID: systemRoot.ID, MenuName: "岗位信息", MenuType: 2, URL: "/system/post", Value: "system:post:list", MenuStatus: 1, Sort: 5, Icon: "Suitcase"})
	if err != nil {
		return err
	}
	configMenu, err := ensureMenu(db, model.Menu{ParentID: systemRoot.ID, MenuName: "系统设置", MenuType: 2, URL: "/system/settings", Value: "system:config:view", MenuStatus: 1, Sort: 6, Icon: "Tools"})
	if err != nil {
		return err
	}
	loginLogMenu, err := ensureMenu(db, model.Menu{ParentID: logRoot.ID, MenuName: "登录日志", MenuType: 2, URL: "/logs/login", Value: "system:loginlog:list", MenuStatus: 1, Sort: 1, Icon: "Tickets"})
	if err != nil {
		return err
	}
	operationLogMenu, err := ensureMenu(db, model.Menu{ParentID: logRoot.ID, MenuName: "操作日志", MenuType: 2, URL: "/logs/operation", Value: "system:operationlog:list", MenuStatus: 1, Sort: 2, Icon: "Memo"})
	if err != nil {
		return err
	}

	buttons := []model.Menu{
		{ParentID: businessTopologyMenu.ID, MenuName: "查看业务拓扑", MenuType: 3, Value: "console:business-topology:view", MenuStatus: 1, Sort: 1},
		{ParentID: adminMenu.ID, MenuName: "新增用户", MenuType: 3, Value: "system:admin:add", MenuStatus: 1, Sort: 1},
		{ParentID: adminMenu.ID, MenuName: "编辑用户", MenuType: 3, Value: "system:admin:edit", MenuStatus: 1, Sort: 2},
		{ParentID: adminMenu.ID, MenuName: "删除用户", MenuType: 3, Value: "system:admin:delete", MenuStatus: 1, Sort: 3},
		{ParentID: adminMenu.ID, MenuName: "切换状态", MenuType: 3, Value: "system:admin:status", MenuStatus: 1, Sort: 4},
		{ParentID: adminMenu.ID, MenuName: "重置密码", MenuType: 3, Value: "system:admin:resetpwd", MenuStatus: 1, Sort: 5},
		{ParentID: adminMenu.ID, MenuName: "同步 LDAP 用户", MenuType: 3, Value: "system:admin:ldapSync", MenuStatus: 1, Sort: 6},

		{ParentID: roleMenu.ID, MenuName: "新增角色", MenuType: 3, Value: "system:role:add", MenuStatus: 1, Sort: 1},
		{ParentID: roleMenu.ID, MenuName: "编辑角色", MenuType: 3, Value: "system:role:edit", MenuStatus: 1, Sort: 2},
		{ParentID: roleMenu.ID, MenuName: "删除角色", MenuType: 3, Value: "system:role:delete", MenuStatus: 1, Sort: 3},
		{ParentID: roleMenu.ID, MenuName: "切换状态", MenuType: 3, Value: "system:role:status", MenuStatus: 1, Sort: 4},
		{ParentID: roleMenu.ID, MenuName: "分配权限", MenuType: 3, Value: "system:role:assign", MenuStatus: 1, Sort: 5},

		{ParentID: menuMenu.ID, MenuName: "新增菜单", MenuType: 3, Value: "system:menu:add", MenuStatus: 1, Sort: 1},
		{ParentID: menuMenu.ID, MenuName: "编辑菜单", MenuType: 3, Value: "system:menu:edit", MenuStatus: 1, Sort: 2},
		{ParentID: menuMenu.ID, MenuName: "删除菜单", MenuType: 3, Value: "system:menu:delete", MenuStatus: 1, Sort: 3},

		{ParentID: deptMenu.ID, MenuName: "新增部门", MenuType: 3, Value: "system:dept:add", MenuStatus: 1, Sort: 1},
		{ParentID: deptMenu.ID, MenuName: "编辑部门", MenuType: 3, Value: "system:dept:edit", MenuStatus: 1, Sort: 2},
		{ParentID: deptMenu.ID, MenuName: "删除部门", MenuType: 3, Value: "system:dept:delete", MenuStatus: 1, Sort: 3},

		{ParentID: postMenu.ID, MenuName: "新增岗位", MenuType: 3, Value: "system:post:add", MenuStatus: 1, Sort: 1},
		{ParentID: postMenu.ID, MenuName: "编辑岗位", MenuType: 3, Value: "system:post:edit", MenuStatus: 1, Sort: 2},
		{ParentID: postMenu.ID, MenuName: "删除岗位", MenuType: 3, Value: "system:post:delete", MenuStatus: 1, Sort: 3},
		{ParentID: postMenu.ID, MenuName: "切换状态", MenuType: 3, Value: "system:post:status", MenuStatus: 1, Sort: 4},

		{ParentID: configMenu.ID, MenuName: "保存配置", MenuType: 3, Value: "system:config:save", MenuStatus: 1, Sort: 1},
		{ParentID: configMenu.ID, MenuName: "配置 LDAP 集成", MenuType: 3, Value: "system:config:ldap", MenuStatus: 1, Sort: 2},

		{ParentID: loginLogMenu.ID, MenuName: "删除登录日志", MenuType: 3, Value: "system:loginlog:delete", MenuStatus: 1, Sort: 1},
		{ParentID: loginLogMenu.ID, MenuName: "清空登录日志", MenuType: 3, Value: "system:loginlog:clean", MenuStatus: 1, Sort: 2},

		{ParentID: operationLogMenu.ID, MenuName: "删除操作日志", MenuType: 3, Value: "system:operationlog:delete", MenuStatus: 1, Sort: 1},
		{ParentID: operationLogMenu.ID, MenuName: "清空操作日志", MenuType: 3, Value: "system:operationlog:clean", MenuStatus: 1, Sort: 2},
	}

	for _, button := range buttons {
		if _, err := ensureMenu(db, button); err != nil {
			return err
		}
	}

	return seedApplicationMenus(db)
}

// seedApplicationMenus mirrors the application switcher navigation in sys_menu.
// This keeps menu administration and role permission assignment aware of every
// functional area, even though the five application sidebars are rendered by the
// frontend's application layout.
func seedApplicationMenus(db *gorm.DB) error {
	type menuSeed struct {
		name  string
		url   string
		value string
		icon  string
	}
	type appSeed struct {
		name     string
		url      string
		value    string
		icon     string
		children []menuSeed
	}
	type buttonSeed struct {
		parentValue string
		name        string
		value       string
	}

	applications := []appSeed{
		{
			name: "资产管理", url: "/assets", value: "assets", icon: "Box",
			children: []menuSeed{
				{"资产概览", "/assets/overview", "assets:overview", "DataBoard"},
				{"环境模型", "/assets/environments", "ops:environment:list", "SetUp"},
				{"终端登录", "/assets/terminal", "assets:terminal", "Platform"},
				{"主机管理", "/assets/server/hosts", "assets:host:list", "Monitor"},
				{"主机组管理", "/assets/server/groups", "assets:hostgroup:list", "FolderOpened"},
				{"凭据管理", "/assets/server/credentials", "assets:credential:list", "Key"},
				{"云账号管理", "/assets/server/cloud-accounts", "assets:cloudaccount:list", "Cloudy"},
				{"数据库列表", "/assets/databases", "assets:database:list", "List"},
				{"DBMS 工作台", "/assets/databases/workbench", "assets:database:workbench", "EditPen"},
				{"数据导入", "/assets/databases/import", "assets:database:import", "Upload"},
				{"备份管理", "/assets/databases/backups", "assets:database:backup", "FolderOpened"},
				{"网关管理", "/assets/gateways", "assets:gateway:list", "Switch"},
				{"集群管理", "/assets/k8s/clusters", "assets:k8s:cluster", "FolderOpened"},
				{"集群概览", "/assets/k8s/overview", "assets:k8s:overview", "DataAnalysis"},
				{"节点管理", "/assets/k8s/nodes", "assets:k8s:node", "Monitor"},
				{"命名空间", "/assets/k8s/namespaces", "assets:k8s:namespace", "Grid"},
				{"工作负载", "/assets/k8s/workloads", "assets:k8s:workload", "SetUp"},
				{"Pod 管理", "/assets/k8s/pods", "assets:k8s:pod", "Box"},
				{"服务", "/assets/k8s/services", "assets:k8s:service", "Share"},
				{"Ingress", "/assets/k8s/ingresses", "assets:k8s:ingress", "Connection"},
				{"高级网络", "/assets/k8s/advanced-network", "assets:k8s:advancednetwork", "Connection"},
				{"配置与存储", "/assets/k8s/config-storage", "assets:k8s:configstorage", "Files"},
			},
		},
		{
			name: "标准运维", url: "/ops", value: "ops", icon: "Operation",
			children: []menuSeed{
				{"脚本库", "/ops/scripts/library", "ops:script:list", "Document"},
				{"命令执行", "/ops/quick-exec/command", "ops:quickexec:command", "CaretRight"},
				{"脚本执行", "/ops/quick-exec/script", "ops:quickexec:script", "EditPen"},
				{"文件分发", "/ops/quick-exec/file-dispatch", "ops:quickexec:file", "Files"},
				{"快速执行历史", "/ops/quick-exec/history", "ops:quickexec:history", "DocumentCopy"},
				{"定时任务", "/ops/schedule/tasks", "ops:schedule:task", "Clock"},
				{"任务日志", "/ops/schedule/logs", "ops:schedule:log", "Document"},
				{"任务模板", "/ops/schedule/templates", "ops:schedule:template", "Tickets"},
				{"作业编排", "/ops/jobs/designer", "ops:job:designer", "Grid"},
				{"作业列表", "/ops/jobs/list", "ops:job:list", "List"},
				{"人工确认", "/ops/jobs/approvals", "ops:job:approval", "Bell"},
				{"作业历史", "/ops/jobs/history", "ops:job:history", "Document"},
				{"作业模板", "/ops/jobs/templates", "ops:job:template", "Tickets"},
			},
		},
		{
			name: "应用中心", url: "/applications", value: "applications", icon: "Box",
			children: []menuSeed{
				{"项目列表", "/applications/projects", "applications:project:list", "Tickets"},
				{"应用拓扑", "/applications/topology", "applications:topology", "Connection"},
				{"构建任务", "/applications/build-tasks", "applications:buildtask:list", "Operation"},
				{"构建历史", "/applications/build-history", "applications:buildhistory:list", "Document"},
				{"CI/CD 流水线", "/applications/pipelines", "applications:pipeline:list", "Share"},
			},
		},
		{
			name: "消息通知", url: "/notify", value: "notify", icon: "Bell",
			children: []menuSeed{
				{"通知规则", "/notify/rules", "notify:rule:list", "Operation"},
				{"消息模板", "/notify/templates", "notify:template:list", "Document"},
				{"通知媒介", "/notify/channels", "notify:channel:list", "Connection"},
				{"发送日志", "/notify/send-logs", "notify:sendlog:list", "Tickets"},
			},
		},
		{
			name: "监控中心", url: "/monitor", value: "monitor", icon: "Histogram",
			children: []menuSeed{
				{"监控概览", "/monitor/overview", "monitor:overview", "TrendCharts"},
				{"智能大屏", "/monitor/command-center", "monitor:commandcenter", "DataBoard"},
				{"数据源管理", "/monitor/datasources", "monitor:datasource:list", "Connection"},
				{"即时查询", "/monitor/query", "monitor:query", "Search"},
				{"日志查询", "/monitor/logs", "monitor:logs", "Document"},
				{"链路追踪", "/monitor/traces", "monitor:traces", "Share"},
				{"告警规则", "/monitor/alert-rules", "monitor:alertrule:list", "Bell"},
				{"告警事件", "/monitor/alert-events", "monitor:alertevent:list", "Warning"},
				{"告警屏蔽", "/monitor/silences", "monitor:silence:list", "MuteNotification"},
				{"聚合收敛", "/monitor/aggregations", "monitor:aggregation:list", "Filter"},
				{"监控大屏", "/monitor/dashboards", "monitor:dashboard:list", "PieChart"},
				{"巡检大屏", "/monitor/inspections", "monitor:inspection:list", "Tickets"},
			},
		},
	}

	menuByValue := make(map[string]model.Menu)
	for appIndex, application := range applications {
		root, err := ensureMenu(db, model.Menu{
			ParentID:   0,
			MenuName:   application.name,
			MenuType:   1,
			URL:        application.url,
			Value:      application.value,
			MenuStatus: 1,
			Sort:       10 + appIndex,
			Icon:       application.icon,
		})
		if err != nil {
			return err
		}
		for childIndex, child := range application.children {
			menu, err := ensureMenu(db, model.Menu{
				ParentID:   root.ID,
				MenuName:   child.name,
				MenuType:   2,
				URL:        child.url,
				Value:      child.value,
				MenuStatus: 1,
				Sort:       childIndex + 1,
				Icon:       child.icon,
			})
			if err != nil {
				return err
			}
			menuByValue[child.value] = menu
		}
	}

	buttons := []buttonSeed{
		// Asset management
		{"assets:host:list", "新增主机", "assets:host:add"}, {"assets:host:list", "编辑主机", "assets:host:edit"}, {"assets:host:list", "删除主机", "assets:host:delete"}, {"assets:host:list", "复制主机", "assets:host:copy"}, {"assets:host:list", "终端登录", "assets:host:terminal"},
		{"assets:hostgroup:list", "新增主机组", "assets:hostgroup:add"}, {"assets:hostgroup:list", "编辑主机组", "assets:hostgroup:edit"}, {"assets:hostgroup:list", "删除主机组", "assets:hostgroup:delete"},
		{"assets:credential:list", "新增凭据", "assets:credential:add"}, {"assets:credential:list", "编辑凭据", "assets:credential:edit"}, {"assets:credential:list", "删除凭据", "assets:credential:delete"},
		{"assets:cloudaccount:list", "新增云账号", "assets:cloudaccount:add"}, {"assets:cloudaccount:list", "编辑云账号", "assets:cloudaccount:edit"}, {"assets:cloudaccount:list", "删除云账号", "assets:cloudaccount:delete"},
		{"assets:database:list", "新增数据库", "assets:database:add"}, {"assets:database:list", "编辑数据库", "assets:database:edit"}, {"assets:database:list", "删除数据库", "assets:database:delete"}, {"assets:database:list", "测试连接", "assets:database:test"},
		{"assets:database:workbench", "执行 SQL", "assets:database:sql:execute"}, {"assets:database:workbench", "编辑数据", "assets:database:data:edit"}, {"assets:database:workbench", "导出数据", "assets:database:export"},
		{"assets:database:import", "执行数据导入", "assets:database:import:execute"}, {"assets:database:backup", "手动备份", "assets:database:backup:create"}, {"assets:database:backup", "恢复备份", "assets:database:backup:restore"}, {"assets:database:backup", "删除备份", "assets:database:backup:delete"},
		{"assets:gateway:list", "新增网关", "assets:gateway:add"}, {"assets:gateway:list", "编辑网关", "assets:gateway:edit"}, {"assets:gateway:list", "删除网关", "assets:gateway:delete"}, {"assets:gateway:list", "测试网关", "assets:gateway:test"},
		{"assets:k8s:cluster", "新增集群", "assets:k8s:cluster:add"}, {"assets:k8s:cluster", "编辑集群", "assets:k8s:cluster:edit"}, {"assets:k8s:cluster", "删除集群", "assets:k8s:cluster:delete"},
		{"assets:k8s:workload", "伸缩工作负载", "assets:k8s:workload:scale"}, {"assets:k8s:workload", "重启工作负载", "assets:k8s:workload:restart"}, {"assets:k8s:workload", "更新镜像", "assets:k8s:workload:image"}, {"assets:k8s:workload", "编辑 YAML", "assets:k8s:workload:yaml"},
		{"assets:k8s:pod", "进入 Pod 终端", "assets:k8s:pod:terminal"}, {"assets:k8s:pod", "删除 Pod", "assets:k8s:pod:delete"}, {"assets:k8s:pod", "编辑 YAML", "assets:k8s:pod:yaml"},

		// Standard operations
		{"ops:script:list", "新增脚本", "ops:script:add"}, {"ops:script:list", "编辑脚本", "ops:script:edit"}, {"ops:script:list", "删除脚本", "ops:script:delete"}, {"ops:script:list", "启用或禁用脚本", "ops:script:status"},
		{"ops:quickexec:command", "执行命令", "ops:quickexec:command:execute"}, {"ops:quickexec:script", "执行脚本", "ops:quickexec:script:execute"}, {"ops:quickexec:file", "分发文件", "ops:quickexec:file:execute"},
		{"ops:schedule:task", "新增定时任务", "ops:schedule:task:add"}, {"ops:schedule:task", "编辑定时任务", "ops:schedule:task:edit"}, {"ops:schedule:task", "删除定时任务", "ops:schedule:task:delete"}, {"ops:schedule:task", "启用或禁用定时任务", "ops:schedule:task:status"},
		{"ops:job:designer", "保存作业模板", "ops:job:template:save"}, {"ops:job:list", "执行作业", "ops:job:execute"}, {"ops:job:list", "删除作业", "ops:job:delete"}, {"ops:job:approval", "确认作业步骤", "ops:job:approve"},

		// Application center
		{"applications:project:list", "新增项目", "applications:project:add"}, {"applications:project:list", "编辑项目", "applications:project:edit"}, {"applications:project:list", "删除项目", "applications:project:delete"},
		{"applications:buildtask:list", "新增构建任务", "applications:buildtask:add"}, {"applications:buildtask:list", "编辑构建任务", "applications:buildtask:edit"}, {"applications:buildtask:list", "删除构建任务", "applications:buildtask:delete"}, {"applications:buildtask:list", "立即构建", "applications:buildtask:run"},
		{"applications:pipeline:list", "新增流水线", "applications:pipeline:add"}, {"applications:pipeline:list", "编辑流水线", "applications:pipeline:edit"}, {"applications:pipeline:list", "删除流水线", "applications:pipeline:delete"}, {"applications:pipeline:list", "执行流水线", "applications:pipeline:run"},

		// Notification center
		{"notify:rule:list", "新增通知规则", "notify:rule:add"}, {"notify:rule:list", "编辑通知规则", "notify:rule:edit"}, {"notify:rule:list", "删除通知规则", "notify:rule:delete"},
		{"notify:template:list", "新增消息模板", "notify:template:add"}, {"notify:template:list", "编辑消息模板", "notify:template:edit"}, {"notify:template:list", "删除消息模板", "notify:template:delete"},
		{"notify:channel:list", "新增通知媒介", "notify:channel:add"}, {"notify:channel:list", "编辑通知媒介", "notify:channel:edit"}, {"notify:channel:list", "删除通知媒介", "notify:channel:delete"}, {"notify:channel:list", "测试通知媒介", "notify:channel:test"},

		// Monitoring center
		{"monitor:datasource:list", "新增数据源", "monitor:datasource:add"}, {"monitor:datasource:list", "编辑数据源", "monitor:datasource:edit"}, {"monitor:datasource:list", "删除数据源", "monitor:datasource:delete"}, {"monitor:datasource:list", "测试数据源", "monitor:datasource:test"},
		{"monitor:alertrule:list", "新增告警规则", "monitor:alertrule:add"}, {"monitor:alertrule:list", "编辑告警规则", "monitor:alertrule:edit"}, {"monitor:alertrule:list", "删除告警规则", "monitor:alertrule:delete"}, {"monitor:alertrule:list", "批量更新告警规则", "monitor:alertrule:batch"},
		{"monitor:alertevent:list", "认领告警", "monitor:alertevent:claim"}, {"monitor:alertevent:list", "关闭告警", "monitor:alertevent:close"}, {"monitor:alertevent:list", "删除告警事件", "monitor:alertevent:delete"},
		{"monitor:silence:list", "新增屏蔽规则", "monitor:silence:add"}, {"monitor:silence:list", "编辑屏蔽规则", "monitor:silence:edit"}, {"monitor:silence:list", "删除屏蔽规则", "monitor:silence:delete"},
		{"monitor:aggregation:list", "新增收敛规则", "monitor:aggregation:add"}, {"monitor:aggregation:list", "编辑收敛规则", "monitor:aggregation:edit"}, {"monitor:aggregation:list", "删除收敛规则", "monitor:aggregation:delete"},
		{"monitor:dashboard:list", "创建监控大屏", "monitor:dashboard:add"}, {"monitor:dashboard:list", "编辑监控大屏", "monitor:dashboard:edit"}, {"monitor:dashboard:list", "删除监控大屏", "monitor:dashboard:delete"},
	}

	for sort, button := range buttons {
		parent, found := menuByValue[button.parentValue]
		if !found {
			continue
		}
		if _, err := ensureMenu(db, model.Menu{
			ParentID:   parent.ID,
			MenuName:   button.name,
			MenuType:   3,
			Value:      button.value,
			MenuStatus: 1,
			Sort:       sort + 1,
		}); err != nil {
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

	admin := model.Admin{
		PostID:    post.ID,
		DeptID:    dept.ID,
		Username:  "admin",
		Password:  util.HashPassword("123456"),
		Nickname:  "系统管理员",
		Status:    1,
		Email:     "admin@example.com",
		Phone:     "13800000000",
		Note:      "初始化管理员",
		CreatedAt: time.Now(),
	}
	if err := db.Create(&admin).Error; err != nil {
		return err
	}

	return db.Create(&model.AdminRole{
		AdminID: admin.ID,
		RoleID:  role.ID,
	}).Error
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

func ensureMenu(db *gorm.DB, menu model.Menu) (model.Menu, error) {
	var existing model.Menu
	err := db.Where("value = ? AND value <> ''", menu.Value).First(&existing).Error
	if err == nil {
		updates := map[string]any{
			"parent_id":   menu.ParentID,
			"menu_name":   menu.MenuName,
			"menu_type":   menu.MenuType,
			"url":         menu.URL,
			"menu_status": menu.MenuStatus,
			"sort":        menu.Sort,
			"icon":        menu.Icon,
		}
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
