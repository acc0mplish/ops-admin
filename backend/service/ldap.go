package service

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"ops-admin/backend/model"
	"ops-admin/backend/util"

	"github.com/go-ldap/ldap/v3"
	"gorm.io/gorm"
)

type LDAPConfigPayload struct {
	Enabled            bool   `json:"enabled"`
	ServerURL          string `json:"serverUrl"`
	TLSMode            string `json:"tlsMode"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify"`
	BindDN             string `json:"bindDn"`
	BindPassword       string `json:"bindPassword"`
	BaseDN             string `json:"baseDn"`
	UserFilter         string `json:"userFilter"`
	UsernameAttribute  string `json:"usernameAttribute"`
	DisplayAttribute   string `json:"displayAttribute"`
	EmailAttribute     string `json:"emailAttribute"`
	PhoneAttribute     string `json:"phoneAttribute"`
	DefaultRoleID      uint   `json:"defaultRoleId"`
	DefaultDeptID      uint   `json:"defaultDeptId"`
	DefaultPostID      uint   `json:"defaultPostId"`
}

type LDAPUser struct {
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	DN       string `json:"dn"`
}

type LDAPSyncPayload struct {
	Usernames []string `json:"usernames"`
}

func (s *Service) ensureLDAPConfig() (model.LDAPConfig, error) {
	var cfg model.LDAPConfig
	err := s.db.First(&cfg).Error
	if err == nil { return cfg, nil }
	if !errors.Is(err, gorm.ErrRecordNotFound) { return cfg, err }
	cfg = model.LDAPConfig{TLSMode: "starttls", UserFilter: "(&(objectClass=person)(uid={{username}}))", UsernameAttribute: "uid", DisplayAttribute: "displayName", EmailAttribute: "mail", PhoneAttribute: "mobile"}
	return cfg, s.db.Create(&cfg).Error
}

func (s *Service) GetLDAPConfig() (map[string]any, error) {
	cfg, err := s.ensureLDAPConfig()
	if err != nil { return nil, err }
	return ldapConfigView(cfg), nil
}

func (s *Service) SaveLDAPConfig(payload LDAPConfigPayload) (map[string]any, error) {
	cfg, err := s.ensureLDAPConfig()
	if err != nil { return nil, err }
	payload = normalizeLDAPPayload(payload)
	if payload.Enabled && (payload.ServerURL == "" || payload.BaseDN == "" || payload.UsernameAttribute == "") { return nil, errors.New("server URL, Base DN, and username attribute are required before enabling LDAP") }
	if payload.Enabled && payload.DefaultRoleID == 0 { return nil, errors.New("a default role for newly synchronized users is required before enabling LDAP") }
	updates := map[string]any{
		"enabled": payload.Enabled, "server_url": payload.ServerURL, "tls_mode": payload.TLSMode,
		"insecure_skip_verify": payload.InsecureSkipVerify, "bind_dn": payload.BindDN, "base_dn": payload.BaseDN,
		"user_filter": payload.UserFilter, "username_attribute": payload.UsernameAttribute,
		"display_attribute": payload.DisplayAttribute, "email_attribute": payload.EmailAttribute,
		"phone_attribute": payload.PhoneAttribute, "default_role_id": payload.DefaultRoleID,
		"default_dept_id": payload.DefaultDeptID, "default_post_id": payload.DefaultPostID,
	}
	if payload.BindPassword != "" { updates["bind_password"] = payload.BindPassword }
	if err := s.db.Model(&cfg).Updates(updates).Error; err != nil { return nil, err }
	cfg, err = s.ensureLDAPConfig()
	if err != nil { return nil, err }
	return ldapConfigView(cfg), nil
}

func (s *Service) TestLDAPConfig(payload LDAPConfigPayload) error {
	cfg, err := s.ldapConfigFromPayload(payload)
	if err != nil { return err }
	conn, err := openLDAPConnection(cfg)
	if err != nil { return err }
	defer conn.Close()
	return nil
}

func (s *Service) PreviewLDAPUsers(keyword string) ([]LDAPUser, error) {
	cfg, err := s.ensureLDAPConfig()
	if err != nil { return nil, err }
	if !cfg.Enabled { return nil, errors.New("enable LDAP integration in system settings first") }
	return queryLDAPUsers(cfg, keyword, 200)
}

func (s *Service) SyncLDAPUsers(payload LDAPSyncPayload) (map[string]int, error) {
	cfg, err := s.ensureLDAPConfig()
	if err != nil { return nil, err }
	if !cfg.Enabled { return nil, errors.New("enable LDAP integration in system settings first") }
	if cfg.DefaultRoleID == 0 { return nil, errors.New("configure a default role for LDAP users in system settings first") }
	var role model.Role
	if err := s.db.Where("id = ? AND status = ?", cfg.DefaultRoleID, 1).First(&role).Error; err != nil { return nil, errors.New("the configured LDAP default role does not exist or is disabled") }
	users, err := queryLDAPUsers(cfg, "", 500)
	if err != nil { return nil, err }
	selected := make(map[string]struct{}, len(payload.Usernames))
	for _, username := range payload.Usernames { if value := strings.TrimSpace(username); value != "" { selected[value] = struct{}{} } }
	result := map[string]int{"created": 0, "updated": 0, "skipped": 0}
	for _, user := range users {
		if len(selected) > 0 { if _, ok := selected[user.Username]; !ok { continue } }
		err := s.db.Transaction(func(tx *gorm.DB) error {
			var admin model.Admin
			err := tx.Where("username = ?", user.Username).First(&admin).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				admin = model.Admin{Username: user.Username, Nickname: firstNonEmpty(user.Nickname, user.Username), Email: user.Email, Phone: user.Phone, DeptID: cfg.DefaultDeptID, PostID: cfg.DefaultPostID, Status: 1, Password: util.HashPassword(fmt.Sprintf("ldap-sync:%s:%d", user.Username, time.Now().UnixNano()))}
				if createErr := tx.Create(&admin).Error; createErr != nil { return createErr }
				if createErr := tx.Create(&model.AdminRole{AdminID: admin.ID, RoleID: cfg.DefaultRoleID}).Error; createErr != nil { return createErr }
				result["created"]++
				return nil
			}
			if err != nil { return err }
			if admin.Username == "admin" { result["skipped"]++; return nil }
			result["updated"]++
			return tx.Model(&admin).Updates(map[string]any{"nickname": firstNonEmpty(user.Nickname, admin.Nickname), "email": user.Email, "phone": user.Phone}).Error
		})
		if err != nil { return nil, err }
	}
	return result, nil
}

func (s *Service) ldapConfigFromPayload(payload LDAPConfigPayload) (model.LDAPConfig, error) {
	cfg, err := s.ensureLDAPConfig()
	if err != nil { return cfg, err }
	payload = normalizeLDAPPayload(payload)
	if payload.ServerURL != "" { cfg.ServerURL = payload.ServerURL }
	if payload.TLSMode != "" { cfg.TLSMode = payload.TLSMode }
	if payload.BindDN != "" { cfg.BindDN = payload.BindDN }
	if payload.BindPassword != "" { cfg.BindPassword = payload.BindPassword }
	if payload.BaseDN != "" { cfg.BaseDN = payload.BaseDN }
	if payload.UserFilter != "" { cfg.UserFilter = payload.UserFilter }
	cfg.InsecureSkipVerify = payload.InsecureSkipVerify
	if cfg.ServerURL == "" { return cfg, errors.New("LDAP server URL is required") }
	return cfg, nil
}

func normalizeLDAPPayload(payload LDAPConfigPayload) LDAPConfigPayload {
	payload.ServerURL = strings.TrimSpace(payload.ServerURL)
	payload.TLSMode = strings.ToLower(strings.TrimSpace(payload.TLSMode))
	if payload.TLSMode == "" { payload.TLSMode = "starttls" }
	if payload.TLSMode != "plain" && payload.TLSMode != "starttls" && payload.TLSMode != "ldaps" { payload.TLSMode = "starttls" }
	payload.BindDN = strings.TrimSpace(payload.BindDN)
	payload.BaseDN = strings.TrimSpace(payload.BaseDN)
	payload.UserFilter = strings.TrimSpace(payload.UserFilter)
	if payload.UserFilter == "" { payload.UserFilter = "(&(objectClass=person)(uid={{username}}))" }
	payload.UsernameAttribute = firstNonEmpty(strings.TrimSpace(payload.UsernameAttribute), "uid")
	payload.DisplayAttribute = firstNonEmpty(strings.TrimSpace(payload.DisplayAttribute), "displayName")
	payload.EmailAttribute = firstNonEmpty(strings.TrimSpace(payload.EmailAttribute), "mail")
	payload.PhoneAttribute = firstNonEmpty(strings.TrimSpace(payload.PhoneAttribute), "mobile")
	return payload
}

func ldapConfigView(cfg model.LDAPConfig) map[string]any {
	return map[string]any{
		"id": cfg.ID, "enabled": cfg.Enabled, "serverUrl": cfg.ServerURL, "tlsMode": cfg.TLSMode,
		"insecureSkipVerify": cfg.InsecureSkipVerify, "bindDn": cfg.BindDN, "bindPasswordSet": cfg.BindPassword != "",
		"baseDn": cfg.BaseDN, "userFilter": cfg.UserFilter, "usernameAttribute": cfg.UsernameAttribute,
		"displayAttribute": cfg.DisplayAttribute, "emailAttribute": cfg.EmailAttribute, "phoneAttribute": cfg.PhoneAttribute,
		"defaultRoleId": cfg.DefaultRoleID, "defaultDeptId": cfg.DefaultDeptID, "defaultPostId": cfg.DefaultPostID,
	}
}

func openLDAPConnection(cfg model.LDAPConfig) (*ldap.Conn, error) {
	serverURL, err := url.Parse(cfg.ServerURL)
	if err != nil || serverURL.Host == "" { return nil, errors.New("LDAP server URL must use ldap:// or ldaps://") }
	tlsConfig := &tls.Config{ServerName: serverURL.Hostname(), InsecureSkipVerify: cfg.InsecureSkipVerify} // #nosec G402 - explicitly configured by system administrator.
	conn, err := ldap.DialURL(cfg.ServerURL, ldap.DialWithTLSConfig(tlsConfig))
	if err != nil { return nil, fmt.Errorf("failed to connect to LDAP: %w", err) }
	conn.SetTimeout(10 * time.Second)
	if cfg.TLSMode == "starttls" && strings.EqualFold(serverURL.Scheme, "ldap") {
		if err := conn.StartTLS(tlsConfig); err != nil { conn.Close(); return nil, fmt.Errorf("LDAP StartTLS failed: %w", err) }
	}
	if cfg.BindDN != "" {
		if err := conn.Bind(cfg.BindDN, cfg.BindPassword); err != nil { conn.Close(); return nil, fmt.Errorf("LDAP bind failed: %w", err) }
	}
	return conn, nil
}

func queryLDAPUsers(cfg model.LDAPConfig, keyword string, limit int) ([]LDAPUser, error) {
	if cfg.BaseDN == "" { return nil, errors.New("LDAP Base DN is required") }
	conn, err := openLDAPConnection(cfg)
	if err != nil { return nil, err }
	defer conn.Close()
	filter := cfg.UserFilter
	if strings.Contains(filter, "{{username}}") {
		filterValue := strings.TrimSpace(keyword)
		if filterValue == "" { filterValue = "*" }
		filter = strings.ReplaceAll(filter, "{{username}}", ldap.EscapeFilter(filterValue))
	}
	request := ldap.NewSearchRequest(cfg.BaseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, limit, 10, false, filter, []string{cfg.UsernameAttribute, cfg.DisplayAttribute, cfg.EmailAttribute, cfg.PhoneAttribute}, nil)
	result, err := conn.Search(request)
	if err != nil { return nil, fmt.Errorf("failed to query LDAP users: %w", err) }
	users := make([]LDAPUser, 0, len(result.Entries))
	for _, entry := range result.Entries {
		username := strings.TrimSpace(entry.GetAttributeValue(cfg.UsernameAttribute))
		if username == "" { continue }
		if keyword != "" && !strings.Contains(strings.ToLower(username), strings.ToLower(keyword)) { continue }
		users = append(users, LDAPUser{Username: username, Nickname: strings.TrimSpace(entry.GetAttributeValue(cfg.DisplayAttribute)), Email: strings.TrimSpace(entry.GetAttributeValue(cfg.EmailAttribute)), Phone: strings.TrimSpace(entry.GetAttributeValue(cfg.PhoneAttribute)), DN: entry.DN})
	}
	return users, nil
}
