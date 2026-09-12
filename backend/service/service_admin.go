package service

import (
	"errors"

	"gorm.io/gorm"
	"ops-admin/backend/model"
	"ops-admin/backend/store"
	"ops-admin/backend/util"
)

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

func (s *Service) CurrentMenus(roleID uint) []map[string]any {
	var menus []model.Menu
	s.db.Table("sys_menu").
		Joins("join sys_role_menu on sys_role_menu.menu_id = sys_menu.id").
		Where("sys_role_menu.role_id = ? and sys_menu.menu_status = ? and sys_menu.menu_type in ?", roleID, 1, []int{1, 2}).
		Order("sys_menu.sort asc, sys_menu.id asc").
		Find(&menus)

	parentMap := map[uint][]model.Menu{}
	roots := make([]model.Menu, 0)
	applicationRoots := map[string]bool{
		"/assets":       true,
		"/ops":          true,
		"/applications": true,
		"/notify":       true,
		"/monitor":      true,
	}
	for _, menu := range menus {
		if menu.ParentID == 0 {
			// Application navigation is rendered by the frontend app switcher.
			// Keep these menus in sys_menu for role assignment, but never expose
			// them as children of the console sidebar.
			if applicationRoots[menu.URL] {
				continue
			}
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
		return errors.New("username already exists")
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
		return errors.New("default administrator cannot be deleted")
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
		return errors.New("current password is incorrect")
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

// AssignRoleMenus replaces the role's visible menu grants from the payload
// while preserving the hidden route-permission grants. The role tree payload
// (Role.vue) only carries what ListMenus showed, so a plain replace would
// silently erase the hidden grants seeded under the protected root.
func (s *Service) AssignRoleMenus(payload RoleMenuPayload) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var list []model.Menu
		if err := tx.Order("sort asc, id asc").Find(&list).Error; err != nil {
			return err
		}
		hidden := collectHiddenMenuIDs(list)
		hiddenIDs := make([]uint, 0, len(hidden))
		for menuID, isHidden := range hidden {
			if isHidden {
				hiddenIDs = append(hiddenIDs, menuID)
			}
		}
		// Delete only the visible grants; hidden grants survive the replace.
		deletions := tx.Where("role_id = ?", payload.ID)
		if len(hiddenIDs) > 0 {
			deletions = deletions.Where("menu_id NOT IN ?", hiddenIDs)
		}
		if err := deletions.Delete(&model.RoleMenu{}).Error; err != nil {
			return err
		}
		if len(payload.MenuIDs) == 0 {
			return nil
		}
		// Insert every payload id, but skip an id whose preserved hidden grant
		// already covers it and dedupe the payload itself.
		preserved := map[uint]bool{}
		if len(hiddenIDs) > 0 {
			var existing []uint
			if err := tx.Model(&model.RoleMenu{}).Where("role_id = ? AND menu_id IN ?", payload.ID, hiddenIDs).Pluck("menu_id", &existing).Error; err != nil {
				return err
			}
			for _, menuID := range existing {
				preserved[menuID] = true
			}
		}
		items := make([]model.RoleMenu, 0, len(payload.MenuIDs))
		seen := map[uint]bool{}
		for _, menuID := range payload.MenuIDs {
			if seen[menuID] || preserved[menuID] {
				continue
			}
			seen[menuID] = true
			items = append(items, model.RoleMenu{RoleID: payload.ID, MenuID: menuID})
		}
		if len(items) == 0 {
			return nil
		}
		return tx.Create(&items).Error
	})
}

func (s *Service) RoleMenuIDs(roleID uint) ([]uint, error) {
	var ids []uint
	err := s.db.Model(&model.RoleMenu{}).Where("role_id = ?", roleID).Pluck("menu_id", &ids).Error
	return ids, err
}

// ErrProtectedSystemMenu marks an operator request that would modify or
// delete a seeded system menu row: the hidden route-permissions root or the
// one-shot migration marker. Losing either row makes
// migrateRoleRoutePermissionsOnce re-run on the next boot and re-grant every
// existing role its full route vocabulary.
var ErrProtectedSystemMenu = errors.New("protected system menu: this row is managed by the system and cannot be modified or deleted")

// isProtectedSystemMenu reports whether a sys_menu row belongs to the seeded
// route-permission infrastructure and must be kept away from operator
// mutations (value-based; the rows themselves are created by store.Seed).
func isProtectedSystemMenu(menu model.Menu) bool {
	return menu.Value == store.RoutePermissionsRootValue || menu.Value == store.RoutePermissionsMarkerValue
}

// collectHiddenMenuIDs resolves the set of menu IDs that must not reach the
// menu-management UI: the hidden route-permissions root, the migration marker
// and every descendant of the root (the route permission leaves). The subtree
// closure runs to a fixed point so arbitrarily deep nesting under the root is
// hidden too.
func collectHiddenMenuIDs(list []model.Menu) map[uint]bool {
	hidden := make(map[uint]bool, len(list))
	for _, menu := range list {
		if isProtectedSystemMenu(menu) {
			hidden[menu.ID] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, menu := range list {
			if !hidden[menu.ID] && hidden[menu.ParentID] {
				hidden[menu.ID] = true
				changed = true
			}
		}
	}
	return hidden
}

// hiddenMenuIDs resolves the protected set against the live table so a
// mutation observes the subtree as it exists right now.
func hiddenMenuIDs(db *gorm.DB) (map[uint]bool, error) {
	var list []model.Menu
	if err := db.Order("sort asc, id asc").Find(&list).Error; err != nil {
		return nil, err
	}
	return collectHiddenMenuIDs(list), nil
}

func (s *Service) ListMenus() ([]model.Menu, error) {
	var list []model.Menu
	if err := s.db.Order("sort asc, id asc").Find(&list).Error; err != nil {
		return nil, err
	}
	hidden := collectHiddenMenuIDs(list)
	filtered := make([]model.Menu, 0, len(list))
	for _, menu := range list {
		if hidden[menu.ID] {
			continue
		}
		filtered = append(filtered, menu)
	}
	return filtered, nil
}

func (s *Service) CreateMenu(payload MenuPayload) error {
	// A created row must not impersonate the hidden infrastructure values,
	// or the boot migration would misread the protected rows.
	if isProtectedSystemMenu(model.Menu{Value: payload.Value}) {
		return ErrProtectedSystemMenu
	}
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
	var existing model.Menu
	if err := s.db.First(&existing, payload.ID).Error; err != nil {
		return err
	}
	hidden, err := hiddenMenuIDs(s.db)
	if err != nil {
		return err
	}
	// The whole hidden subtree is protected: rewriting a leaf's route value,
	// flipping its menu_status or re-parenting it to visibility would corrupt
	// the seeded route-permission infrastructure just like deleting it.
	if hidden[existing.ID] {
		return ErrProtectedSystemMenu
	}
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
	var existing model.Menu
	if err := s.db.First(&existing, id).Error; err != nil {
		return err
	}
	hidden, err := hiddenMenuIDs(s.db)
	if err != nil {
		return err
	}
	if hidden[existing.ID] {
		return ErrProtectedSystemMenu
	}
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
	if err := s.db.First(&menu, id).Error; err != nil {
		return &menu, err
	}
	hidden, err := hiddenMenuIDs(s.db)
	if err != nil {
		return &menu, err
	}
	if hidden[menu.ID] {
		return &menu, ErrProtectedSystemMenu
	}
	return &menu, nil
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
