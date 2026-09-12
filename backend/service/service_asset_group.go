package service

import (
	"errors"
	"sort"
	"time"

	"gorm.io/gorm"
	"ops-admin/backend/model"
)

type AssetHostGroupMemberPayload struct {
	HostID  uint   `json:"hostId"`
	HostIDs []uint `json:"hostIds"`
	GroupID uint   `json:"groupId"`
}

type AssetHostGroupPayload struct {
	ID          uint   `json:"id"`
	ParentID    uint   `json:"parentId"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Sort        int    `json:"sort"`
	Status      int    `json:"status"`
	Description string `json:"description"`
}

type AssetHostGroupNode struct {
	ID          uint                  `json:"id"`
	ParentID    uint                  `json:"parentId"`
	Name        string                `json:"name"`
	Code        string                `json:"code"`
	Sort        int                   `json:"sort"`
	Status      int                   `json:"status"`
	Description string                `json:"description"`
	HostCount   int64                 `json:"hostCount"`
	CreatedAt   time.Time             `json:"createTime"`
	UpdatedAt   time.Time             `json:"updateTime"`
	Children    []*AssetHostGroupNode `json:"children,omitempty"`
}

type AssetHostGroupListResult struct {
	List  []AssetHostGroupNode  `json:"list"`
	Tree  []*AssetHostGroupNode `json:"tree"`
	Total int                   `json:"total"`
}

func (s *Service) ListAssetHostGroups(keyword string) (*AssetHostGroupListResult, error) {
	var groups []model.AssetHostGroup
	query := s.db.Model(&model.AssetHostGroup{})
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name like ? or code like ?", like, like)
	}
	if err := query.Order("sort asc, id asc").Find(&groups).Error; err != nil {
		return nil, err
	}
	countMap, err := s.assetHostGroupHostCountMap()
	if err != nil {
		return nil, err
	}
	list := make([]AssetHostGroupNode, 0, len(groups))
	for _, item := range groups {
		list = append(list, s.newAssetHostGroupNode(item, countMap[item.ID]))
	}
	return &AssetHostGroupListResult{
		List:  list,
		Tree:  buildAssetHostGroupTree(list),
		Total: len(list),
	}, nil
}

func (s *Service) GetAssetHostGroup(id uint) (*AssetHostGroupNode, error) {
	var item model.AssetHostGroup
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	countMap, err := s.assetHostGroupHostCountMap()
	if err != nil {
		return nil, err
	}
	result := s.newAssetHostGroupNode(item, countMap[item.ID])
	return &result, nil
}

func (s *Service) CreateAssetHostGroup(payload AssetHostGroupPayload) error {
	name := Trimmed(payload.Name)
	if name == "" {
		return errors.New("host group name is required")
	}
	if err := s.validateAssetHostGroupParent(0, payload.ParentID); err != nil {
		return err
	}
	item := model.AssetHostGroup{
		ParentID:    payload.ParentID,
		Name:        name,
		Code:        Trimmed(payload.Code),
		Sort:        payload.Sort,
		Status:      payload.Status,
		Description: Trimmed(payload.Description),
	}
	if item.Status == 0 {
		item.Status = 1
	}
	return s.db.Create(&item).Error
}

func (s *Service) UpdateAssetHostGroup(payload AssetHostGroupPayload) error {
	name := Trimmed(payload.Name)
	if payload.ID == 0 {
		return errors.New("host group does not exist")
	}
	if name == "" {
		return errors.New("host group name is required")
	}
	if err := s.validateAssetHostGroupParent(payload.ID, payload.ParentID); err != nil {
		return err
	}
	return s.db.Model(&model.AssetHostGroup{}).Where("id = ?", payload.ID).Updates(map[string]any{
		"parent_id":   payload.ParentID,
		"name":        name,
		"code":        Trimmed(payload.Code),
		"sort":        payload.Sort,
		"status":      payload.Status,
		"description": Trimmed(payload.Description),
	}).Error
}

func (s *Service) DeleteAssetHostGroup(id uint) error {
	if id == 0 {
		return errors.New("host group does not exist")
	}
	hostCount, err := s.assetHostGroupHostCount(id)
	if err != nil {
		return err
	}
	if hostCount > 0 {
		return errors.New("host group still contains hosts and cannot be deleted")
	}
	var childCount int64
	if err := s.db.Model(&model.AssetHostGroup{}).Where("parent_id = ?", id).Count(&childCount).Error; err != nil {
		return err
	}
	if childCount > 0 {
		return errors.New("host group still contains child groups and cannot be deleted")
	}
	return s.db.Delete(&model.AssetHostGroup{}, id).Error
}

func (s *Service) validateAssetHostGroupParent(id uint, parentID uint) error {
	if parentID == 0 {
		return nil
	}
	if id != 0 && id == parentID {
		return errors.New("a host group cannot be its own parent")
	}
	var parent model.AssetHostGroup
	if err := s.db.First(&parent, parentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("parent host group does not exist")
		}
		return err
	}
	if id == 0 {
		return nil
	}
	var groups []model.AssetHostGroup
	if err := s.db.Select("id", "parent_id").Find(&groups).Error; err != nil {
		return err
	}
	parentMap := make(map[uint]uint, len(groups))
	for _, item := range groups {
		parentMap[item.ID] = item.ParentID
	}
	for current := parentID; current != 0; {
		if current == id {
			return errors.New("parent host group cannot be the current group or one of its descendants")
		}
		next, ok := parentMap[current]
		if !ok || next == current {
			break
		}
		current = next
	}
	return nil
}

func (s *Service) assetHostGroupHostCount(id uint) (int64, error) {
	countMap, err := s.assetHostGroupHostCountMap()
	if err != nil {
		return 0, err
	}
	return countMap[id], nil
}

func (s *Service) assetHostGroupHostCountMap() (map[uint]int64, error) {
	type countRow struct {
		GroupID uint  `gorm:"column:group_id"`
		Count   int64 `gorm:"column:count"`
	}

	countMap := map[uint]int64{}

	var relRows []countRow
	if err := s.db.Model(&model.AssetHostGroupRelation{}).
		Select("group_id, count(distinct host_id) as count").
		Group("group_id").
		Scan(&relRows).Error; err != nil {
		return nil, err
	}
	for _, item := range relRows {
		countMap[item.GroupID] += item.Count
	}

	var fallbackRows []countRow
	if err := s.db.Model(&model.AssetHost{}).
		Select("asset_host.group_id as group_id, count(distinct asset_host.id) as count").
		Joins("LEFT JOIN asset_host_group_rel rel ON rel.host_id = asset_host.id AND rel.group_id = asset_host.group_id").
		Where("asset_host.group_id <> 0 AND rel.host_id IS NULL").
		Group("asset_host.group_id").
		Scan(&fallbackRows).Error; err != nil {
		return nil, err
	}
	for _, item := range fallbackRows {
		countMap[item.GroupID] += item.Count
	}

	return countMap, nil
}

func (s *Service) newAssetHostGroupNode(item model.AssetHostGroup, hostCount int64) AssetHostGroupNode {
	return AssetHostGroupNode{
		ID:          item.ID,
		ParentID:    item.ParentID,
		Name:        item.Name,
		Code:        item.Code,
		Sort:        item.Sort,
		Status:      item.Status,
		Description: item.Description,
		HostCount:   hostCount,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func buildAssetHostGroupTree(list []AssetHostGroupNode) []*AssetHostGroupNode {
	nodes := make(map[uint]*AssetHostGroupNode, len(list))
	roots := make([]*AssetHostGroupNode, 0)
	for _, item := range list {
		node := item
		node.Children = nil
		nodes[node.ID] = &node
	}
	for _, item := range list {
		node := nodes[item.ID]
		if node.ParentID != 0 {
			if parent, ok := nodes[node.ParentID]; ok {
				parent.Children = append(parent.Children, node)
				continue
			}
		}
		roots = append(roots, node)
	}
	sortAssetHostGroupNodes(roots)
	return roots
}

func sortAssetHostGroupNodes(nodes []*AssetHostGroupNode) {
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Sort == nodes[j].Sort {
			return nodes[i].ID < nodes[j].ID
		}
		return nodes[i].Sort < nodes[j].Sort
	})
	for _, node := range nodes {
		if len(node.Children) > 0 {
			sortAssetHostGroupNodes(node.Children)
		}
	}
}

func normalizeGroupIDs(groupIDs []uint, fallback uint) []uint {
	result := make([]uint, 0, len(groupIDs)+1)
	seen := map[uint]struct{}{}
	for _, id := range groupIDs {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	if len(result) == 0 && fallback > 0 {
		result = append(result, fallback)
	}
	return result
}

func firstGroupID(groupIDs []uint) uint {
	if len(groupIDs) > 0 {
		return groupIDs[0]
	}
	return 0
}

func syncAssetHostGroups(db *gorm.DB, hostID uint, groupIDs []uint, fallback uint) error {
	normalized := normalizeGroupIDs(groupIDs, fallback)
	if len(normalized) == 0 {
		return nil
	}
	if err := db.Where("host_id = ?", hostID).Delete(&model.AssetHostGroupRelation{}).Error; err != nil {
		return err
	}
	relations := make([]model.AssetHostGroupRelation, 0, len(normalized))
	for _, groupID := range normalized {
		relations = append(relations, model.AssetHostGroupRelation{
			HostID:  hostID,
			GroupID: groupID,
		})
	}
	return db.Create(&relations).Error
}

func ensureHostGroupFallback(host *model.AssetHost) {
	if host == nil {
		return
	}
	if len(host.HostGroups) == 0 && host.GroupID > 0 && host.Group.ID > 0 {
		host.HostGroups = []model.AssetHostGroup{host.Group}
	}
}

func ensureHostGroupFallbackList(list []model.AssetHost) {
	for index := range list {
		ensureHostGroupFallback(&list[index])
	}
}
