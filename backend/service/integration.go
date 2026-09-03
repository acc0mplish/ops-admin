package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"

	"ops-admin/backend/model"

	"gorm.io/gorm"
)

type IntegrationNavigationGroupPayload struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPublic    bool   `json:"isPublic"`
	Status      int    `json:"status"`
	Sort        int    `json:"sort"`
}

type IntegrationNavigationPayload struct {
	ID          uint   `json:"id"`
	GroupID     uint   `json:"groupId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
	IconURL     string `json:"iconUrl"`
	OpenMode    string `json:"openMode"`
	Status      int    `json:"status"`
	Sort        int    `json:"sort"`
}

func (s *Service) ListIntegrationNavigationGroups(keyword string) ([]map[string]any, error) {
	query := s.db.Model(&model.IntegrationNavigationGroup{})
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		query = query.Where("name LIKE ? OR description LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	var groups []model.IntegrationNavigationGroup
	if err := query.Order("sort ASC, id ASC").Find(&groups).Error; err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(groups))
	for _, group := range groups {
		var count int64
		if err := s.db.Model(&model.IntegrationNavigation{}).Where("group_id = ?", group.ID).Count(&count).Error; err != nil {
			return nil, err
		}
		result = append(result, map[string]any{
			"id": group.ID, "name": group.Name, "description": group.Description,
			"isPublic": group.IsPublic, "publicToken": group.PublicToken,
			"status": group.Status, "sort": group.Sort, "itemCount": count,
			"createTime": group.CreatedAt, "updateTime": group.UpdatedAt,
		})
	}
	return result, nil
}

func (s *Service) SaveIntegrationNavigationGroup(payload IntegrationNavigationGroupPayload) (model.IntegrationNavigationGroup, error) {
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		return model.IntegrationNavigationGroup{}, errors.New("navigation group name is required")
	}
	status := payload.Status
	if status != 2 {
		status = 1
	}
	var group model.IntegrationNavigationGroup
	if payload.ID > 0 {
		if err := s.db.First(&group, payload.ID).Error; err != nil {
			return group, err
		}
	}
	group.Name = name
	group.Description = strings.TrimSpace(payload.Description)
	group.IsPublic = payload.IsPublic
	group.Status = status
	group.Sort = payload.Sort
	if group.IsPublic && group.PublicToken == "" {
		token, err := newPublicNavigationToken()
		if err != nil {
			return group, err
		}
		group.PublicToken = token
	}
	if err := s.db.Save(&group).Error; err != nil {
		return group, err
	}
	return group, nil
}

func (s *Service) DeleteIntegrationNavigationGroup(id uint) error {
	if id == 0 {
		return errors.New("navigation group ID is required")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", id).Delete(&model.IntegrationNavigation{}).Error; err != nil {
			return err
		}
		result := tx.Delete(&model.IntegrationNavigationGroup{}, id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (s *Service) RegenerateIntegrationPublicToken(id uint) (model.IntegrationNavigationGroup, error) {
	var group model.IntegrationNavigationGroup
	if err := s.db.First(&group, id).Error; err != nil {
		return group, err
	}
	token, err := newPublicNavigationToken()
	if err != nil {
		return group, err
	}
	group.PublicToken = token
	group.IsPublic = true
	if err := s.db.Save(&group).Error; err != nil {
		return group, err
	}
	return group, nil
}

func (s *Service) ListIntegrationNavigations(groupID uint, keyword string) ([]model.IntegrationNavigation, error) {
	query := s.db.Model(&model.IntegrationNavigation{})
	if groupID > 0 {
		query = query.Where("group_id = ?", groupID)
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		query = query.Where("name LIKE ? OR description LIKE ? OR url LIKE ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	var items []model.IntegrationNavigation
	err := query.Order("sort ASC, id ASC").Find(&items).Error
	return items, err
}

func (s *Service) SaveIntegrationNavigation(payload IntegrationNavigationPayload) (model.IntegrationNavigation, error) {
	name := strings.TrimSpace(payload.Name)
	targetURL := strings.TrimSpace(payload.URL)
	if name == "" || payload.GroupID == 0 || targetURL == "" {
		return model.IntegrationNavigation{}, errors.New("navigation name, group, and target URL are required")
	}
	if err := validatePublicNavigationURL(targetURL); err != nil {
		return model.IntegrationNavigation{}, err
	}
	iconURL := strings.TrimSpace(payload.IconURL)
	if iconURL != "" {
		if err := validatePublicNavigationURL(iconURL); err != nil {
			return model.IntegrationNavigation{}, errors.New("icon URL must be a valid HTTP/HTTPS address")
		}
	}
	var group model.IntegrationNavigationGroup
	if err := s.db.First(&group, payload.GroupID).Error; err != nil {
		return model.IntegrationNavigation{}, errors.New("navigation group does not exist")
	}
	status := payload.Status
	if status != 2 {
		status = 1
	}
	openMode := payload.OpenMode
	if openMode != "current" {
		openMode = "new"
	}
	var item model.IntegrationNavigation
	if payload.ID > 0 {
		if err := s.db.First(&item, payload.ID).Error; err != nil {
			return item, err
		}
	}
	item.GroupID = payload.GroupID
	item.Name = name
	item.Description = strings.TrimSpace(payload.Description)
	item.URL = targetURL
	item.IconURL = iconURL
	item.OpenMode = openMode
	item.Status = status
	item.Sort = payload.Sort
	if err := s.db.Save(&item).Error; err != nil {
		return item, err
	}
	return item, nil
}

func (s *Service) DeleteIntegrationNavigation(id uint) error {
	if id == 0 {
		return errors.New("navigation ID is required")
	}
	result := s.db.Delete(&model.IntegrationNavigation{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Service) GetPublicIntegrationNavigation(token string) (map[string]any, error) {
	var group model.IntegrationNavigationGroup
	if err := s.db.Where("public_token = ? AND is_public = ? AND status = ?", strings.TrimSpace(token), true, 1).First(&group).Error; err != nil {
		return nil, err
	}
	var items []model.IntegrationNavigation
	if err := s.db.Where("group_id = ? AND status = ?", group.ID, 1).Order("sort ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	publicItems := make([]map[string]any, 0, len(items))
	for _, item := range items {
		publicItems = append(publicItems, map[string]any{
			"name": item.Name, "description": item.Description, "url": item.URL,
			"iconUrl": item.IconURL, "openMode": item.OpenMode,
		})
	}
	return map[string]any{
		"name": group.Name, "description": group.Description, "items": publicItems,
	}, nil
}

func newPublicNavigationToken() (string, error) {
	buffer := make([]byte, 24)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func validatePublicNavigationURL(value string) error {
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("target URL must be a valid HTTP/HTTPS address")
	}
	return nil
}
