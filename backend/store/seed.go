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
	configMenu, err := ensureMenu(db, model.Menu{ParentID: systemRoot.ID, MenuName: "基础配置", MenuType: 2, URL: "/system/basic-config", Value: "system:config:view", MenuStatus: 1, Sort: 6, Icon: "Tools"})
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
		{ParentID: adminMenu.ID, MenuName: "新增用户", MenuType: 3, Value: "system:admin:add", MenuStatus: 1, Sort: 1},
		{ParentID: adminMenu.ID, MenuName: "编辑用户", MenuType: 3, Value: "system:admin:edit", MenuStatus: 1, Sort: 2},
		{ParentID: adminMenu.ID, MenuName: "删除用户", MenuType: 3, Value: "system:admin:delete", MenuStatus: 1, Sort: 3},
		{ParentID: adminMenu.ID, MenuName: "切换状态", MenuType: 3, Value: "system:admin:status", MenuStatus: 1, Sort: 4},
		{ParentID: adminMenu.ID, MenuName: "重置密码", MenuType: 3, Value: "system:admin:resetpwd", MenuStatus: 1, Sort: 5},

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
