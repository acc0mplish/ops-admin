package service

import (
	"strings"

	"ops-admin/backend/model"
)

func normalizeAssetTags(tags []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, tag)
	}
	return result
}

func (s *Service) recordAssetChange(resourceType string, resourceID uint, resourceName, action, summary, operator string) {
	if resourceID == 0 {
		return
	}
	if strings.TrimSpace(operator) == "" {
		operator = "system"
	}
	_ = s.db.Create(&model.AssetChangeLog{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		ResourceName: strings.TrimSpace(resourceName),
		Action:       action,
		Summary:      strings.TrimSpace(summary),
		Operator:     operator,
	}).Error
}

func (s *Service) ListAssetChangeLogs(resourceType string, resourceID uint, limit int) ([]model.AssetChangeLog, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	query := s.db.Model(&model.AssetChangeLog{})
	if strings.TrimSpace(resourceType) != "" {
		query = query.Where("resource_type = ?", strings.TrimSpace(resourceType))
	}
	if resourceID > 0 {
		query = query.Where("resource_id = ?", resourceID)
	}
	var list []model.AssetChangeLog
	if err := query.Order("id desc").Limit(limit).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}
