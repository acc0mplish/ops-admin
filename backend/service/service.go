package service

import (
	"errors"
	"log"
	"strings"
	"time"

	"ops-admin/backend/auth"
	"ops-admin/backend/model"
	"ops-admin/backend/util"

	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Service {
	return &Service{db: db}
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AdminPayload struct {
	ID       uint   `json:"id"`
	PostID   uint   `json:"postId"`
	RoleID   uint   `json:"roleId"`
	DeptID   uint   `json:"deptId"`
	Username string `json:"username"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
	Status   int    `json:"status"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Note     string `json:"note"`
}

type RolePayload struct {
	ID          uint   `json:"id"`
	RoleName    string `json:"roleName"`
	RoleKey     string `json:"roleKey"`
	Status      int    `json:"status"`
	Description string `json:"description"`
}

type MenuPayload struct {
	ID         uint   `json:"id"`
	ParentID   uint   `json:"parentId"`
	MenuName   string `json:"menuName"`
	Icon       string `json:"icon"`
	Value      string `json:"value"`
	MenuType   int    `json:"menuType"`
	URL        string `json:"url"`
	MenuStatus int    `json:"menuStatus"`
	Sort       int    `json:"sort"`
}

type DeptPayload struct {
	ID         uint   `json:"id"`
	ParentID   uint   `json:"parentId"`
	DeptType   int    `json:"deptType"`
	DeptName   string `json:"deptName"`
	DeptStatus int    `json:"deptStatus"`
}

type PostPayload struct {
	ID         uint   `json:"id"`
	PostCode   string `json:"postCode"`
	PostName   string `json:"postName"`
	PostStatus int    `json:"postStatus"`
	Remark     string `json:"remark"`
}

type RoleMenuPayload struct {
	ID      uint   `json:"id"`
	MenuIDs []uint `json:"menuIds"`
}

type IDPayload struct {
	ID uint `json:"id"`
}

type BatchIDPayload struct {
	IDs []uint `json:"ids"`
}

type AdminStatusPayload struct {
	ID     uint `json:"id"`
	Status int  `json:"status"`
}

type PostStatusPayload struct {
	ID         uint `json:"id"`
	PostStatus int  `json:"postStatus"`
}

type RoleStatusPayload struct {
	ID     uint `json:"id"`
	Status int  `json:"status"`
}

type SystemConfigPayload struct {
	SiteName           string `json:"siteName"`
	SiteSlogan         string `json:"siteSlogan"`
	LogoType           string `json:"logoType"`
	LogoValue          string `json:"logoValue"`
	LoginTitle         string `json:"loginTitle"`
	LoginSubtitle      string `json:"loginSubtitle"`
	UseLoginBackground bool   `json:"useLoginBackground"`
	LoginBackground    string `json:"loginBackground"`
	PrimaryColor       string `json:"primaryColor"`
	SidebarTheme       string `json:"sidebarTheme"`
}

type profileRow struct {
	model.Admin
	RoleName string `gorm:"column:role_name"`
	DeptName string `gorm:"column:dept_name"`
	PostName string `gorm:"column:post_name"`
}

func (s *Service) Login(req LoginRequest, ip string, browser string, osName string) (map[string]any, error) {
	var admin model.Admin
	if err := s.db.Where("username = ?", req.Username).First(&admin).Error; err != nil {
		s.createLoginLog(req.Username, ip, browser, osName, 2, "用户名或密码错误")
		return nil, errors.New("用户名或密码错误")
	}
	if admin.Status != 1 {
		s.createLoginLog(req.Username, ip, browser, osName, 2, "账号已停用")
		return nil, errors.New("账号已停用")
	}
	if !util.CheckPassword(admin.Password, req.Password) {
		s.createLoginLog(req.Username, ip, browser, osName, 2, "用户名或密码错误")
		return nil, errors.New("用户名或密码错误")
	}

	token, err := auth.GenerateToken(admin.ID, admin.Username)
	if err != nil {
		return nil, err
	}
	s.createLoginLog(req.Username, ip, browser, osName, 1, "登录成功")

	roleID := s.getRoleID(admin.ID)
	config, _ := s.GetSystemConfig()

	return map[string]any{
		"token":          token,
		"sysAdmin":       admin,
		"leftMenuList":   s.CurrentMenus(roleID),
		"permissionList": s.CurrentPermissions(roleID),
		"systemConfig":   config,
	}, nil
}

func (s *Service) GetProfile(userID uint) (map[string]any, error) {
	user, err := s.profileUser(userID)
	if err != nil {
		return nil, err
	}
	roleID := s.getRoleID(userID)
	config, _ := s.GetSystemConfig()
	return map[string]any{
		"user":           user,
		"leftMenuList":   s.CurrentMenus(roleID),
		"permissionList": s.CurrentPermissions(roleID),
		"systemConfig":   config,
	}, nil
}

func (s *Service) GetSystemConfig() (model.SystemConfig, error) {
	return s.ensureSystemConfig()
}

func (s *Service) UpdateSystemConfig(payload SystemConfigPayload) (model.SystemConfig, error) {
	cfg, err := s.ensureSystemConfig()
	if err != nil {
		return cfg, err
	}

	updates := map[string]any{
		"site_name":            Trimmed(payload.SiteName),
		"site_slogan":          Trimmed(payload.SiteSlogan),
		"logo_type":            Trimmed(payload.LogoType),
		"logo_value":           Trimmed(payload.LogoValue),
		"login_title":          Trimmed(payload.LoginTitle),
		"login_subtitle":       Trimmed(payload.LoginSubtitle),
		"use_login_background": payload.UseLoginBackground,
		"login_background":     Trimmed(payload.LoginBackground),
		"primary_color":        Trimmed(payload.PrimaryColor),
		"sidebar_theme":        Trimmed(payload.SidebarTheme),
	}

	if updates["site_name"] == "" {
		updates["site_name"] = "Ops Admin"
	}
	if updates["logo_type"] == "" {
		updates["logo_type"] = "text"
	}
	if updates["primary_color"] == "" {
		updates["primary_color"] = "#5b6cf9"
	}
	if updates["sidebar_theme"] == "" {
		updates["sidebar_theme"] = "dark"
	}

	if err := s.db.Model(&cfg).Updates(updates).Error; err != nil {
		return cfg, err
	}
	return s.ensureSystemConfig()
}

func (s *Service) createLoginLog(username, ip, browser, osName string, status int, message string) {
	entry := model.LoginLog{
		Username:      username,
		IPAddress:     ip,
		LoginLocation: "local",
		Browser:       normalizeBrowser(browser),
		OS:            normalizeOS(browser, osName),
		LoginStatus:   status,
		Message:       message,
		LoginTime:     time.Now(),
	}
	if err := s.db.Create(&entry).Error; err != nil {
		log.Printf("create login log failed: %v", err)
	}
}

func (s *Service) ensureSystemConfig() (model.SystemConfig, error) {
	var cfg model.SystemConfig
	err := s.db.First(&cfg).Error
	if err == nil {
		return cfg, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return cfg, err
	}

	cfg = model.SystemConfig{
		SiteName:           "Ops Admin",
		SiteSlogan:         "个人运维管理平台",
		LogoType:           "text",
		LogoValue:          "OA",
		LoginTitle:         "Ops Admin",
		LoginSubtitle:      "系统管理与运维控制台",
		UseLoginBackground: false,
		PrimaryColor:       "#5b6cf9",
		SidebarTheme:       "dark",
	}
	return cfg, s.db.Create(&cfg).Error
}

func (s *Service) getRoleID(adminID uint) uint {
	var adminRole model.AdminRole
	s.db.Where("admin_id = ?", adminID).First(&adminRole)
	return adminRole.RoleID
}

func (s *Service) CurrentPermissions(roleID uint) []string {
	var values []string
	s.db.Table("sys_menu").
		Select("sys_menu.value").
		Joins("join sys_role_menu on sys_role_menu.menu_id = sys_menu.id").
		Where("sys_role_menu.role_id = ? and sys_menu.value <> ''", roleID).
		Order("sys_menu.sort asc").
		Scan(&values)
	return values
}

func (s *Service) CurrentMenus(roleID uint) []map[string]any {
	var menus []model.Menu
	s.db.Table("sys_menu").
		Joins("join sys_role_menu on sys_role_menu.menu_id = sys_menu.id").
		Where("sys_role_menu.role_id = ? and sys_menu.menu_status = ? and sys_menu.menu_type in ?", roleID, 1, []int{1, 2}).
		Order("sys_menu.sort asc, sys_menu.id asc").
		Find(&menus)

	parentMap := map[uint][]model.Menu{}
	roots := make([]model.Menu, 0)
	for _, menu := range menus {
		if menu.ParentID == 0 {
			roots = append(roots, menu)
			continue
		}
		parentMap[menu.ParentID] = append(parentMap[menu.ParentID], menu)
	}

	result := make([]map[string]any, 0, len(roots))
	for _, root := range roots {
		item := map[string]any{
			"id":       root.ID,
			"menuName": root.MenuName,
			"icon":     root.Icon,
			"url":      root.URL,
		}
		children := make([]map[string]any, 0)
		for _, child := range parentMap[root.ID] {
			children = append(children, map[string]any{
				"id":       child.ID,
				"menuName": child.MenuName,
				"icon":     child.Icon,
				"url":      child.URL,
			})
		}
		item["menuSvoList"] = children
		result = append(result, item)
	}
	return result
}

func (s *Service) profileUser(userID uint) (map[string]any, error) {
	var row profileRow
	err := s.db.Table("sys_admin").
		Select("sys_admin.*, sys_role.role_name, sys_dept.dept_name, sys_post.post_name").
		Joins("left join sys_admin_role on sys_admin_role.admin_id = sys_admin.id").
		Joins("left join sys_role on sys_role.id = sys_admin_role.role_id").
		Joins("left join sys_dept on sys_dept.id = sys_admin.dept_id").
		Joins("left join sys_post on sys_post.id = sys_admin.post_id").
		Where("sys_admin.id = ?", userID).
		Scan(&row).Error
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id":         row.ID,
		"username":   row.Username,
		"nickname":   row.Nickname,
		"status":     row.Status,
		"email":      row.Email,
		"phone":      row.Phone,
		"note":       row.Note,
		"deptId":     row.DeptID,
		"postId":     row.PostID,
		"deptName":   row.DeptName,
		"postName":   row.PostName,
		"roleName":   row.RoleName,
		"createTime": row.CreatedAt,
		"updateTime": row.UpdatedAt,
	}, nil
}

func (s *Service) ListAdmins(pageNum, pageSize int, username string, status string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	query := s.db.Table("sys_admin").
		Select("sys_admin.id, sys_admin.username, sys_admin.nickname, sys_admin.status, sys_admin.post_id, sys_admin.dept_id, sys_admin.email, sys_admin.phone, sys_admin.note, sys_admin.created_at, sys_post.post_name, sys_dept.dept_name, sys_role.role_name, sys_admin_role.role_id").
		Joins("left join sys_post on sys_post.id = sys_admin.post_id").
		Joins("left join sys_dept on sys_dept.id = sys_admin.dept_id").
		Joins("left join sys_admin_role on sys_admin_role.admin_id = sys_admin.id").
		Joins("left join sys_role on sys_role.id = sys_admin_role.role_id")

	if username != "" {
		query = query.Where("sys_admin.username like ?", "%"+username+"%")
	}
	if status != "" {
		query = query.Where("sys_admin.status = ?", status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var list []model.AdminListItem
	if err := query.Order("sys_admin.id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Scan(&list).Error; err != nil {
		return nil, err
	}

	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func (s *Service) GetAdmin(id uint) (map[string]any, error) {
	var admin model.Admin
	if err := s.db.First(&admin, id).Error; err != nil {
		return nil, err
	}
	return map[string]any{
		"id":       admin.ID,
		"postId":   admin.PostID,
		"deptId":   admin.DeptID,
		"username": admin.Username,
		"nickname": admin.Nickname,
		"status":   admin.Status,
		"email":    admin.Email,
		"phone":    admin.Phone,
		"note":     admin.Note,
		"roleId":   s.getRoleID(admin.ID),
	}, nil
}

func (s *Service) CreateAdmin(payload AdminPayload) error {
	var count int64
	s.db.Model(&model.Admin{}).Where("username = ?", payload.Username).Count(&count)
	if count > 0 {
		return errors.New("用户名已存在")
	}
	admin := model.Admin{
		PostID:   payload.PostID,
		DeptID:   payload.DeptID,
		Username: payload.Username,
		Password: util.HashPassword(payload.Password),
		Nickname: payload.Nickname,
		Status:   payload.Status,
		Email:    payload.Email,
		Phone:    payload.Phone,
		Note:     payload.Note,
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&admin).Error; err != nil {
			return err
		}
		return tx.Create(&model.AdminRole{AdminID: admin.ID, RoleID: payload.RoleID}).Error
	})
}

func (s *Service) UpdateAdmin(payload AdminPayload) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var admin model.Admin
		if err := tx.First(&admin, payload.ID).Error; err != nil {
			return err
		}
		admin.PostID = payload.PostID
		admin.DeptID = payload.DeptID
		admin.Username = payload.Username
		admin.Nickname = payload.Nickname
		admin.Status = payload.Status
		admin.Email = payload.Email
		admin.Phone = payload.Phone
		admin.Note = payload.Note
		if payload.Password != "" {
			admin.Password = util.HashPassword(payload.Password)
		}
		if err := tx.Save(&admin).Error; err != nil {
			return err
		}
		if err := tx.Where("admin_id = ?", admin.ID).Delete(&model.AdminRole{}).Error; err != nil {
			return err
		}
		return tx.Create(&model.AdminRole{AdminID: admin.ID, RoleID: payload.RoleID}).Error
	})
}

func (s *Service) DeleteAdmin(id uint) error {
	var admin model.Admin
	if err := s.db.First(&admin, id).Error; err != nil {
		return err
	}
	if admin.Username == "admin" {
		return errors.New("默认管理员不允许删除")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.Admin{}, id).Error; err != nil {
			return err
		}
		return tx.Where("admin_id = ?", id).Delete(&model.AdminRole{}).Error
	})
}

func (s *Service) UpdateAdminStatus(payload AdminStatusPayload) error {
	return s.db.Model(&model.Admin{}).Where("id = ?", payload.ID).Update("status", payload.Status).Error
}

func (s *Service) ResetAdminPassword(id uint, password string) error {
	return s.db.Model(&model.Admin{}).Where("id = ?", id).Update("password", util.HashPassword(password)).Error
}

func (s *Service) UpdateProfile(userID uint, payload AdminPayload) error {
	updates := map[string]any{
		"nickname": Trimmed(payload.Nickname),
		"email":    Trimmed(payload.Email),
		"phone":    Trimmed(payload.Phone),
		"note":     Trimmed(payload.Note),
	}
	return s.db.Model(&model.Admin{}).Where("id = ?", userID).Updates(updates).Error
}

func (s *Service) UpdateProfilePassword(userID uint, oldPassword string, newPassword string) error {
	var admin model.Admin
	if err := s.db.First(&admin, userID).Error; err != nil {
		return err
	}
	if !util.CheckPassword(admin.Password, oldPassword) {
		return errors.New("原密码不正确")
	}
	return s.db.Model(&admin).Update("password", util.HashPassword(newPassword)).Error
}

func (s *Service) ListRoles() ([]model.Role, error) {
	var list []model.Role
	return list, s.db.Order("id asc").Find(&list).Error
}

func (s *Service) CreateRole(payload RolePayload) error {
	return s.db.Create(&model.Role{
		RoleName:    payload.RoleName,
		RoleKey:     payload.RoleKey,
		Status:      payload.Status,
		Description: payload.Description,
	}).Error
}

func (s *Service) UpdateRole(payload RolePayload) error {
	return s.db.Model(&model.Role{}).Where("id = ?", payload.ID).Updates(map[string]any{
		"role_name":   payload.RoleName,
		"role_key":    payload.RoleKey,
		"status":      payload.Status,
		"description": payload.Description,
	}).Error
}

func (s *Service) DeleteRole(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.Role{}, id).Error; err != nil {
			return err
		}
		if err := tx.Where("role_id = ?", id).Delete(&model.RoleMenu{}).Error; err != nil {
			return err
		}
		return tx.Where("role_id = ?", id).Delete(&model.AdminRole{}).Error
	})
}

func (s *Service) GetRole(id uint) (*model.Role, error) {
	var role model.Role
	return &role, s.db.First(&role, id).Error
}

func (s *Service) UpdateRoleStatus(payload RoleStatusPayload) error {
	return s.db.Model(&model.Role{}).Where("id = ?", payload.ID).Update("status", payload.Status).Error
}

func (s *Service) AssignRoleMenus(payload RoleMenuPayload) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", payload.ID).Delete(&model.RoleMenu{}).Error; err != nil {
			return err
		}
		if len(payload.MenuIDs) == 0 {
			return nil
		}
		items := make([]model.RoleMenu, 0, len(payload.MenuIDs))
		for _, menuID := range payload.MenuIDs {
			items = append(items, model.RoleMenu{RoleID: payload.ID, MenuID: menuID})
		}
		return tx.Create(&items).Error
	})
}

func (s *Service) RoleMenuIDs(roleID uint) ([]uint, error) {
	var ids []uint
	err := s.db.Model(&model.RoleMenu{}).Where("role_id = ?", roleID).Pluck("menu_id", &ids).Error
	return ids, err
}

func (s *Service) ListMenus() ([]model.Menu, error) {
	var list []model.Menu
	return list, s.db.Order("sort asc, id asc").Find(&list).Error
}

func (s *Service) CreateMenu(payload MenuPayload) error {
	return s.db.Create(&model.Menu{
		ParentID:   payload.ParentID,
		MenuName:   payload.MenuName,
		Icon:       payload.Icon,
		Value:      payload.Value,
		MenuType:   payload.MenuType,
		URL:        payload.URL,
		MenuStatus: payload.MenuStatus,
		Sort:       payload.Sort,
	}).Error
}

func (s *Service) UpdateMenu(payload MenuPayload) error {
	return s.db.Model(&model.Menu{}).Where("id = ?", payload.ID).Updates(map[string]any{
		"parent_id":   payload.ParentID,
		"menu_name":   payload.MenuName,
		"icon":        payload.Icon,
		"value":       payload.Value,
		"menu_type":   payload.MenuType,
		"url":         payload.URL,
		"menu_status": payload.MenuStatus,
		"sort":        payload.Sort,
	}).Error
}

func (s *Service) DeleteMenu(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.Menu{}, id).Error; err != nil {
			return err
		}
		if err := tx.Where("parent_id = ?", id).Delete(&model.Menu{}).Error; err != nil {
			return err
		}
		return tx.Where("menu_id = ?", id).Delete(&model.RoleMenu{}).Error
	})
}

func (s *Service) GetMenu(id uint) (*model.Menu, error) {
	var menu model.Menu
	return &menu, s.db.First(&menu, id).Error
}

func (s *Service) ListDepts() ([]model.Dept, error) {
	var list []model.Dept
	return list, s.db.Order("id asc").Find(&list).Error
}

func (s *Service) CreateDept(payload DeptPayload) error {
	return s.db.Create(&model.Dept{
		ParentID:   payload.ParentID,
		DeptType:   payload.DeptType,
		DeptName:   payload.DeptName,
		DeptStatus: payload.DeptStatus,
	}).Error
}

func (s *Service) UpdateDept(payload DeptPayload) error {
	return s.db.Model(&model.Dept{}).Where("id = ?", payload.ID).Updates(map[string]any{
		"parent_id":   payload.ParentID,
		"dept_type":   payload.DeptType,
		"dept_name":   payload.DeptName,
		"dept_status": payload.DeptStatus,
	}).Error
}

func (s *Service) DeleteDept(id uint) error {
	return s.db.Delete(&model.Dept{}, id).Error
}

func (s *Service) GetDept(id uint) (*model.Dept, error) {
	var dept model.Dept
	return &dept, s.db.First(&dept, id).Error
}

func (s *Service) DeptUsers(id uint) ([]model.Admin, error) {
	var list []model.Admin
	return list, s.db.Where("dept_id = ?", id).Find(&list).Error
}

func (s *Service) ListPosts() ([]model.Post, error) {
	var list []model.Post
	return list, s.db.Order("id asc").Find(&list).Error
}

func (s *Service) CreatePost(payload PostPayload) error {
	return s.db.Create(&model.Post{
		PostCode:   payload.PostCode,
		PostName:   payload.PostName,
		PostStatus: payload.PostStatus,
		Remark:     payload.Remark,
	}).Error
}

func (s *Service) UpdatePost(payload PostPayload) error {
	return s.db.Model(&model.Post{}).Where("id = ?", payload.ID).Updates(map[string]any{
		"post_code":   payload.PostCode,
		"post_name":   payload.PostName,
		"post_status": payload.PostStatus,
		"remark":      payload.Remark,
	}).Error
}

func (s *Service) DeletePost(id uint) error {
	return s.db.Delete(&model.Post{}, id).Error
}

func (s *Service) BatchDeletePosts(ids []uint) error {
	return s.db.Delete(&model.Post{}, ids).Error
}

func (s *Service) UpdatePostStatus(payload PostStatusPayload) error {
	return s.db.Model(&model.Post{}).Where("id = ?", payload.ID).Update("post_status", payload.PostStatus).Error
}

func (s *Service) GetPost(id uint) (*model.Post, error) {
	var post model.Post
	return &post, s.db.First(&post, id).Error
}

func (s *Service) ListLoginLogs(pageNum, pageSize int, username string) (map[string]any, error) {
	return s.paginateLogs(&model.LoginLog{}, pageNum, pageSize, "username", username)
}

func (s *Service) DeleteLoginLog(id uint) error {
	return s.db.Delete(&model.LoginLog{}, id).Error
}

func (s *Service) BatchDeleteLoginLogs(ids []uint) error {
	return s.db.Delete(&model.LoginLog{}, ids).Error
}

func (s *Service) CleanLoginLogs() error {
	return s.db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.LoginLog{}).Error
}

func (s *Service) ListOperationLogs(pageNum, pageSize int, username string) (map[string]any, error) {
	return s.paginateLogs(&model.OperationLog{}, pageNum, pageSize, "username", username)
}

func (s *Service) DeleteOperationLog(id uint) error {
	return s.db.Delete(&model.OperationLog{}, id).Error
}

func (s *Service) BatchDeleteOperationLogs(ids []uint) error {
	return s.db.Delete(&model.OperationLog{}, ids).Error
}

func (s *Service) CleanOperationLogs() error {
	return s.db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.OperationLog{}).Error
}

func (s *Service) paginateLogs(entity any, pageNum, pageSize int, field string, keyword string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(entity)
	if keyword != "" {
		query = query.Where(field+" like ?", "%"+keyword+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	switch entity.(type) {
	case *model.LoginLog:
		var list []model.LoginLog
		if err := query.Order("id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
			return nil, err
		}
		return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
	case *model.OperationLog:
		var list []model.OperationLog
		if err := query.Order("id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
			return nil, err
		}
		return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
	default:
		return nil, errors.New("unsupported entity")
	}
}

func Trimmed(s string) string {
	return strings.TrimSpace(s)
}

func normalizeBrowser(userAgent string) string {
	ua := strings.ToLower(userAgent)
	switch {
	case strings.Contains(ua, "edg/"):
		return "Edge"
	case strings.Contains(ua, "chrome/"):
		return "Chrome"
	case strings.Contains(ua, "firefox/"):
		return "Firefox"
	case strings.Contains(ua, "safari/") && !strings.Contains(ua, "chrome/"):
		return "Safari"
	case strings.Contains(ua, "micromessenger"):
		return "WeChat"
	default:
		return shortenText(strings.TrimSpace(userAgent), 120)
	}
}

func normalizeOS(userAgent string, fallback string) string {
	ua := strings.ToLower(userAgent)
	switch {
	case strings.Contains(ua, "windows"):
		return "Windows"
	case strings.Contains(ua, "mac os x"), strings.Contains(ua, "macintosh"):
		return "macOS"
	case strings.Contains(ua, "android"):
		return "Android"
	case strings.Contains(ua, "iphone"), strings.Contains(ua, "ipad"), strings.Contains(ua, "ios"):
		return "iOS"
	case strings.Contains(ua, "linux"):
		return "Linux"
	default:
		return shortenText(strings.TrimSpace(fallback), 120)
	}
}

func shortenText(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
