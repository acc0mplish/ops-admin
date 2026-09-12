package service

import (
	"database/sql"
	"errors"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
	"ops-admin/backend/auth"
	"ops-admin/backend/model"
	"ops-admin/backend/util"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
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

func (s *Service) Login(req LoginRequest, ip string, browser string, osName string) (map[string]any, string, error) {
	var admin model.Admin
	if err := s.db.Where("username = ?", req.Username).First(&admin).Error; err != nil {
		s.createLoginLog(req.Username, ip, browser, osName, 2, "invalid username or password")
		return nil, "", errors.New("invalid username or password")
	}
	if admin.Status != 1 {
		s.createLoginLog(req.Username, ip, browser, osName, 2, "account is disabled")
		return nil, "", errors.New("account is disabled")
	}
	if !util.CheckPassword(admin.Password, req.Password) {
		s.createLoginLog(req.Username, ip, browser, osName, 2, "invalid username or password")
		return nil, "", errors.New("invalid username or password")
	}

	sessionID, err := auth.NewOpaqueToken()
	if err != nil {
		return nil, "", err
	}
	refreshToken, err := auth.NewOpaqueToken()
	if err != nil {
		return nil, "", err
	}
	now := time.Now()
	session := model.AuthSession{
		ID:               sessionID,
		AdminID:          admin.ID,
		RefreshTokenHash: auth.HashOpaqueToken(refreshToken),
		LastActivityAt:   now,
		ExpiresAt:        now.Add(auth.SessionMaxTTL),
	}
	if err := s.db.Create(&session).Error; err != nil {
		return nil, "", err
	}
	token, accessExpiresAt, err := auth.GenerateToken(admin.ID, admin.Username, session.ID)
	if err != nil {
		_ = s.db.Delete(&session).Error
		return nil, "", err
	}
	s.createLoginLog(req.Username, ip, browser, osName, 1, "login successful")

	roleID := s.getRoleID(admin.ID)
	config, _ := s.GetSystemConfig()

	return map[string]any{
		"token":                token,
		"accessTokenExpiresAt": accessExpiresAt.UnixMilli(),
		"sessionExpiresAt":     session.ExpiresAt.UnixMilli(),
		"sysAdmin":             admin,
		"leftMenuList":         s.CurrentMenus(roleID),
		"permissionList":       s.CurrentPermissions(roleID),
		"systemConfig":         config,
	}, refreshToken, nil
}

func (s *Service) RefreshSession(refreshToken string, reportedActivityAt time.Time) (map[string]any, string, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return nil, "", errors.New("refresh token is missing")
	}
	var session model.AuthSession
	if err := s.db.Where("refresh_token_hash = ?", auth.HashOpaqueToken(refreshToken)).First(&session).Error; err != nil {
		return nil, "", errors.New("login session is invalid")
	}
	now := time.Now()
	lastActivityAt := session.LastActivityAt
	if reportedActivityAt.After(lastActivityAt) && !reportedActivityAt.After(now.Add(time.Minute)) {
		lastActivityAt = reportedActivityAt
	}
	if session.RevokedAt != nil || !now.Before(session.ExpiresAt) || now.Sub(lastActivityAt) >= auth.SessionIdleTTL {
		return nil, "", errors.New("login session has expired")
	}

	var admin model.Admin
	if err := s.db.First(&admin, session.AdminID).Error; err != nil || admin.Status != 1 {
		return nil, "", errors.New("account is unavailable")
	}
	if err := s.db.Model(&model.AuthSession{}).Where("id = ? AND revoked_at IS NULL", session.ID).Updates(map[string]any{
		"last_activity_at": lastActivityAt,
	}).Error; err != nil {
		return nil, "", err
	}
	accessToken, accessExpiresAt, err := auth.GenerateToken(admin.ID, admin.Username, session.ID)
	if err != nil {
		return nil, "", err
	}
	return map[string]any{
		"token":                accessToken,
		"accessTokenExpiresAt": accessExpiresAt.UnixMilli(),
		"sessionExpiresAt":     session.ExpiresAt.UnixMilli(),
	}, refreshToken, nil
}

func (s *Service) RevokeSession(refreshToken string) {
	if strings.TrimSpace(refreshToken) == "" {
		return
	}
	now := time.Now()
	_ = s.db.Model(&model.AuthSession{}).
		Where("refresh_token_hash = ? AND revoked_at IS NULL", auth.HashOpaqueToken(refreshToken)).
		Update("revoked_at", &now).Error
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
		SiteSlogan:         "Personal Operations Management Platform",
		LogoType:           "text",
		LogoValue:          "OA",
		LoginTitle:         "Ops Admin",
		LoginSubtitle:      "System Administration and Operations Console",
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

func (s *Service) ListOperationLogs(pageNum, pageSize int, username string, keyword string, riskLevel string, success string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.operationLogQuery(username, keyword, riskLevel, success)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	stats, err := s.operationLogStats(username, keyword, riskLevel, success)
	if err != nil {
		return nil, err
	}
	var list []model.OperationLog
	if err := query.Order("id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize, "stats": stats}, nil
}

func (s *Service) operationLogQuery(username string, keyword string, riskLevel string, success string) *gorm.DB {
	query := s.db.Model(&model.OperationLog{})
	if username = strings.TrimSpace(username); username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("description LIKE ? OR url LIKE ? OR request_summary LIKE ? OR ip LIKE ?", like, like, like, like)
	}
	if riskLevel = strings.TrimSpace(riskLevel); riskLevel != "" {
		query = query.Where("risk_level = ?", riskLevel)
	}
	if success = strings.TrimSpace(success); success != "" {
		query = query.Where("success = ?", success == "true" || success == "1")
	}
	return query
}

func (s *Service) operationLogStats(username string, keyword string, riskLevel string, success string) (map[string]any, error) {
	base := s.operationLogQuery(username, keyword, riskLevel, success)
	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, err
	}
	var highRisk int64
	if err := base.Session(&gorm.Session{}).Where("risk_level = ?", "high").Count(&highRisk).Error; err != nil {
		return nil, err
	}
	var failed int64
	if err := base.Session(&gorm.Session{}).Where("success = ?", false).Count(&failed).Error; err != nil {
		return nil, err
	}
	var avgDuration sql.NullFloat64
	if err := base.Session(&gorm.Session{}).Select("AVG(duration_ms)").Scan(&avgDuration).Error; err != nil {
		return nil, err
	}
	avg := 0
	if avgDuration.Valid {
		avg = int(avgDuration.Float64)
	}
	return map[string]any{
		"total":       total,
		"highRisk":    highRisk,
		"failed":      failed,
		"avgDuration": avg,
	}, nil
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
