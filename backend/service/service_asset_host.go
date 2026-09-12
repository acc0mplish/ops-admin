package service

import (
	"errors"
	"sync"

	"gorm.io/gorm"
	"ops-admin/backend/model"
)

type AssetHostBatchCredentialPayload struct {
	IDs          []uint `json:"ids"`
	CredentialID uint   `json:"credentialId"`
}

type AssetHostPayload struct {
	ID             uint   `json:"id"`
	HostName       string `json:"hostName"`
	Alias          string `json:"alias"`
	GroupID        uint   `json:"groupId"`
	GroupIDs       []uint `json:"groupIds"`
	CredentialID   uint   `json:"credentialId"`
	ConnectionMode string `json:"connectionMode"`
	GatewayID      uint   `json:"gatewayId"`
	CloudAccountID uint   `json:"cloudAccountId"`
	PrivateIP      string `json:"privateIp"`
	PublicIP       string `json:"publicIp"`
	SSHUser        string `json:"sshUser"`
	SSHIP          string `json:"sshIp"`
	SSHPort        int    `json:"sshPort"`
	OS             string `json:"os"`
	Arch           string `json:"arch"`
	CPU            string `json:"cpu"`
	Memory         string `json:"memory"`
	Disk           string `json:"disk"`
	Environment    string `json:"environment"`
	Provider       string `json:"provider"`
	Region         string `json:"region"`
	Status         int    `json:"status"`
	Description    string `json:"description"`
	Operator       string `json:"-"`
}

type AssetHostImportRow struct {
	HostName       string `json:"hostName"`
	SSHIP          string `json:"sshIp"`
	SSHPort        int    `json:"sshPort"`
	SSHUser        string `json:"sshUser"`
	CredentialName string `json:"credentialName"`
	ConnectionMode string `json:"connectionMode"`
	GatewayName    string `json:"gatewayName"`
	Environment    string `json:"environment"`
	PrivateIP      string `json:"privateIp"`
	PublicIP       string `json:"publicIp"`
	Provider       string `json:"provider"`
	Region         string `json:"region"`
	Description    string `json:"description"`
}

// The optional legacyTag argument keeps older callers compatible. Host tags are no
// longer part of host management and the value is intentionally ignored.
func (s *Service) ListAssetHosts(pageNum, pageSize int, keyword string, groupID uint, status, environment string, legacyTag ...string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.AssetHost{}).Preload("Group").Preload("HostGroups").Preload("Credential").Preload("Gateway").Preload("Gateway.Credential").Preload("CloudAccount")
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("host_name like ? or alias like ? or private_ip like ? or public_ip like ? or ssh_ip like ?", like, like, like, like, like)
	}
	if groupID > 0 {
		query = query.Joins("LEFT JOIN asset_host_group_rel rel ON rel.host_id = asset_host.id").
			Where("asset_host.group_id = ? OR rel.group_id = ?", groupID, groupID)
	}
	if status != "" {
		query = query.Where("alive_status = ?", status)
	}
	if environment != "" {
		query = query.Where("environment = ?", normalizeEnvCode(environment))
	}
	var total int64
	if err := query.Session(&gorm.Session{}).Distinct("asset_host.id").Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.AssetHost
	if err := query.Select("asset_host.*").Distinct().Order("asset_host.id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	ensureHostGroupFallbackList(list)
	s.enrichAssetHostUsageMetrics(list)
	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func (s *Service) GetAssetHost(id uint) (*model.AssetHost, error) {
	var item model.AssetHost
	if err := s.db.Preload("Group").Preload("HostGroups").Preload("Credential").Preload("Gateway").Preload("Gateway.Credential").Preload("CloudAccount").First(&item, id).Error; err != nil {
		return &item, err
	}
	ensureHostGroupFallback(&item)
	return &item, nil
}

func (s *Service) CreateAssetHost(payload AssetHostPayload) error {
	host := assetHostFromPayload(payload)
	if host.Environment == "" {
		return errors.New("select an environment")
	}
	if err := validateGatewaySelection(host.ConnectionMode, host.GatewayID); err != nil {
		return err
	}
	if host.SSHPort == 0 {
		host.SSHPort = 22
	}
	if host.Status == 0 {
		host.Status = 1
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&host).Error; err != nil {
			return err
		}
		return syncAssetHostGroups(tx, host.ID, payload.GroupIDs, host.GroupID)
	})
	if err == nil {
		s.recordAssetChange("host", host.ID, host.HostName, "create", "Create Host Asset", payload.Operator)
	}
	return err
}

func (s *Service) UpdateAssetHost(payload AssetHostPayload) error {
	updates := assetHostUpdates(payload)
	if normalizeEnvCode(payload.Environment) == "" {
		return errors.New("select an environment")
	}
	if err := validateGatewaySelection(normalizeConnectionMode(payload.ConnectionMode), optionalGatewayID(payload.ConnectionMode, payload.GatewayID)); err != nil {
		return err
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.AssetHost{}).Where("id = ?", payload.ID).Updates(updates).Error; err != nil {
			return err
		}
		return syncAssetHostGroups(tx, payload.ID, payload.GroupIDs, payload.GroupID)
	})
	if err == nil {
		s.recordAssetChange("host", payload.ID, payload.HostName, "update", "Update Host Details", payload.Operator)
	}
	return err
}

func (s *Service) DeleteAssetHost(id uint) error {
	var host model.AssetHost
	_ = s.db.First(&host, id).Error
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("host_id = ?", id).Delete(&model.AssetHostGroupRelation{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.AssetHost{}, id).Error
	})
	if err == nil {
		s.recordAssetChange("host", id, host.HostName, "delete", "Delete Host Asset", "system")
	}
	return err
}

func (s *Service) BatchDeleteAssetHosts(ids []uint) error {
	if len(ids) == 0 {
		return errors.New("no host selected")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("host_id IN ?", ids).Delete(&model.AssetHostGroupRelation{}).Error; err != nil {
			return err
		}
		return tx.Where("id IN ?", ids).Delete(&model.AssetHost{}).Error
	})
}

func (s *Service) RemoveAssetHostsFromGroup(groupID uint, hostIDs []uint) error {
	if groupID == 0 {
		return errors.New("host group is required")
	}
	if len(hostIDs) == 0 {
		return errors.New("no host selected")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ? AND host_id IN ?", groupID, hostIDs).Delete(&model.AssetHostGroupRelation{}).Error; err != nil {
			return err
		}
		for _, hostID := range hostIDs {
			var host model.AssetHost
			if err := tx.Select("id", "group_id").First(&host, hostID).Error; err != nil {
				return err
			}
			if host.GroupID != groupID {
				continue
			}
			var nextGroupID uint
			if err := tx.Model(&model.AssetHostGroupRelation{}).
				Where("host_id = ?", hostID).
				Order("group_id asc").
				Limit(1).
				Pluck("group_id", &nextGroupID).Error; err != nil {
				return err
			}
			nextGroupValue := any(nil)
			if nextGroupID > 0 {
				nextGroupValue = nextGroupID
			}
			if err := tx.Model(&model.AssetHost{}).Where("id = ?", hostID).Update("group_id", nextGroupValue).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) BatchSyncAssetHosts(ids []uint) (map[string]any, error) {
	if len(ids) == 0 {
		return nil, errors.New("no host selected")
	}
	type result struct {
		id  uint
		err error
	}
	workerCount := len(ids)
	if workerCount > 4 {
		workerCount = 4
	}
	jobs := make(chan uint)
	results := make(chan result, len(ids))
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				_, err := s.SyncAssetHost(id)
				results <- result{id: id, err: err}
			}
		}()
	}
	go func() {
		for _, id := range ids {
			jobs <- id
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	successCount := 0
	failed := make([]uint, 0)
	for item := range results {
		if item.err != nil {
			failed = append(failed, item.id)
			continue
		}
		successCount++
	}
	return map[string]any{
		"success":   successCount,
		"fail":      len(failed),
		"failedIds": failed,
	}, nil
}

func (s *Service) BatchReplaceAssetHostCredential(ids []uint, credentialID uint) error {
	if len(ids) == 0 {
		return errors.New("no host selected")
	}
	if credentialID == 0 {
		return errors.New("credentialId is required")
	}
	var credential model.AssetCredential
	if err := s.db.First(&credential, credentialID).Error; err != nil {
		return errors.New("credential not found")
	}
	return s.db.Model(&model.AssetHost{}).Where("id IN ?", ids).Updates(map[string]any{
		"credential_id": credentialID,
		"auth_status":   2,
	}).Error
}

func assetHostFromPayload(payload AssetHostPayload) model.AssetHost {
	groupIDs := normalizeGroupIDs(payload.GroupIDs, payload.GroupID)
	return model.AssetHost{
		HostName:       Trimmed(payload.HostName),
		Alias:          Trimmed(payload.Alias),
		GroupID:        firstGroupID(groupIDs),
		CredentialID:   optionalUint(payload.CredentialID),
		ConnectionMode: normalizeConnectionMode(payload.ConnectionMode),
		GatewayID:      optionalGatewayID(payload.ConnectionMode, payload.GatewayID),
		CloudAccountID: optionalUint(payload.CloudAccountID),
		PrivateIP:      Trimmed(payload.PrivateIP),
		PublicIP:       Trimmed(payload.PublicIP),
		SSHUser:        Trimmed(payload.SSHUser),
		SSHIP:          Trimmed(payload.SSHIP),
		SSHPort:        payload.SSHPort,
		OS:             Trimmed(payload.OS),
		Arch:           Trimmed(payload.Arch),
		CPU:            Trimmed(payload.CPU),
		Memory:         Trimmed(payload.Memory),
		Disk:           Trimmed(payload.Disk),
		Environment:    normalizeEnvCode(payload.Environment),
		Provider:       Trimmed(payload.Provider),
		Region:         Trimmed(payload.Region),
		Status:         payload.Status,
		Description:    Trimmed(payload.Description),
	}
}

func assetHostUpdates(payload AssetHostPayload) map[string]any {
	host := assetHostFromPayload(payload)
	if host.SSHPort == 0 {
		host.SSHPort = 22
	}
	if host.Status == 0 {
		host.Status = 1
	}
	return map[string]any{
		"host_name":        host.HostName,
		"alias":            host.Alias,
		"group_id":         host.GroupID,
		"credential_id":    host.CredentialID,
		"connection_mode":  host.ConnectionMode,
		"gateway_id":       host.GatewayID,
		"cloud_account_id": host.CloudAccountID,
		"private_ip":       host.PrivateIP,
		"public_ip":        host.PublicIP,
		"ssh_user":         host.SSHUser,
		"ssh_ip":           host.SSHIP,
		"ssh_port":         host.SSHPort,
		"os":               host.OS,
		"arch":             host.Arch,
		"cpu":              host.CPU,
		"memory":           host.Memory,
		"disk":             host.Disk,
		"environment":      host.Environment,
		"provider":         host.Provider,
		"region":           host.Region,
		"status":           host.Status,
		"description":      host.Description,
	}
}
