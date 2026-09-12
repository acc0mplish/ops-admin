package service

import (
	"encoding/json"
	"errors"
	"strings"

	"gorm.io/gorm"
	infraModel "ops-admin/backend/internal/infra/model"
	"ops-admin/backend/model"
)

type AssetCloudSyncPayload struct {
	GroupID            uint     `json:"groupId"`
	Provider           string   `json:"provider"`
	CredentialID       uint     `json:"credentialId"`
	ConnectionMode     string   `json:"connectionMode"`
	GatewayID          uint     `json:"gatewayId"`
	Environment        string   `json:"environment"`
	Regions            []string `json:"regions"`
	Region             string   `json:"region"`
	CloudAccountID     uint     `json:"cloudAccountId"`
	UseExistingAccount bool     `json:"useExistingAccount"`
	AccessKey          string   `json:"accessKey"`
	SecretKey          string   `json:"secretKey"`
	AccountName        string   `json:"accountName"`
	SaveAccount        bool     `json:"saveAccount"`
}

type AssetCredentialPayload struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	AuthType    string `json:"authType"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	PrivateKey  string `json:"privateKey"`
	Passphrase  string `json:"passphrase"`
	Status      int    `json:"status"`
	Description string `json:"description"`
}

type AssetCloudAccountPayload struct {
	ID          uint     `json:"id"`
	Name        string   `json:"name"`
	Provider    string   `json:"provider"`
	AccessKey   string   `json:"accessKey"`
	SecretKey   string   `json:"secretKey"`
	Regions     []string `json:"regions"`
	Region      string   `json:"region"`
	Status      int      `json:"status"`
	Description string   `json:"description"`
}

type AssetDatabasePayload struct {
	ID             uint     `json:"id"`
	Name           string   `json:"name"`
	DBType         string   `json:"dbType"`
	Host           string   `json:"host"`
	Port           int      `json:"port"`
	Username       string   `json:"username"`
	Password       string   `json:"password"`
	ConnectionMode string   `json:"connectionMode"`
	GatewayID      uint     `json:"gatewayId"`
	DBName         string   `json:"dbName"`
	Charset        string   `json:"charset"`
	Env            string   `json:"env"`
	Tags           []string `json:"tags"`
	AccessMode     string   `json:"accessMode"`
	MonitorEnabled bool     `json:"monitorEnabled"`
	Status         int      `json:"status"`
	Description    string   `json:"description"`
	Operator       string   `json:"-"`
}

func (s *Service) ListAssetCredentials(pageNum, pageSize int, keyword string, authType string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.AssetCredential{})
	if keyword != "" {
		query = query.Where("name like ? or username like ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if authType != "" {
		query = query.Where("auth_type = ?", authType)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.AssetCredential
	if err := query.Order("id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	for i := range list {
		maskCredential(&list[i])
	}
	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func (s *Service) ListAssetCredentialOptions() ([]model.AssetCredential, error) {
	var list []model.AssetCredential
	err := s.db.Where("status = ?", 1).Order("id desc").Find(&list).Error
	for i := range list {
		maskCredential(&list[i])
	}
	return list, err
}

func (s *Service) GetAssetCredential(id uint) (*model.AssetCredential, error) {
	var item model.AssetCredential
	return &item, s.db.First(&item, id).Error
}

func (s *Service) CreateAssetCredential(payload AssetCredentialPayload) error {
	item := model.AssetCredential{
		Name:        Trimmed(payload.Name),
		AuthType:    normalizedAuthType(payload.AuthType),
		Username:    Trimmed(payload.Username),
		Password:    payload.Password,
		PrivateKey:  payload.PrivateKey,
		Passphrase:  payload.Passphrase,
		Status:      payload.Status,
		Description: Trimmed(payload.Description),
	}
	if item.Status == 0 {
		item.Status = 1
	}
	return s.db.Create(&item).Error
}

func (s *Service) UpdateAssetCredential(payload AssetCredentialPayload) error {
	updates := map[string]any{
		"name":        Trimmed(payload.Name),
		"auth_type":   normalizedAuthType(payload.AuthType),
		"username":    Trimmed(payload.Username),
		"status":      payload.Status,
		"description": Trimmed(payload.Description),
	}
	if payload.Password != "" {
		updates["password"] = payload.Password
	}
	if payload.PrivateKey != "" {
		updates["private_key"] = payload.PrivateKey
	}
	if payload.Passphrase != "" {
		updates["passphrase"] = payload.Passphrase
	}
	return s.db.Model(&model.AssetCredential{}).Where("id = ?", payload.ID).Updates(updates).Error
}

func (s *Service) DeleteAssetCredential(id uint) error {
	var count int64
	s.db.Model(&model.AssetHost{}).Where("credential_id = ?", id).Count(&count)
	if count > 0 {
		return errors.New("credential is referenced by hosts and cannot be deleted")
	}
	return s.db.Delete(&model.AssetCredential{}, id).Error
}

func (s *Service) ListAssetCloudAccounts(pageNum, pageSize int, keyword string, provider string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.AssetCloudAccount{})
	if keyword != "" {
		query = query.Where("name like ? or access_key like ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if provider != "" {
		query = query.Where("provider = ?", provider)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.AssetCloudAccount
	if err := query.Order("id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	for i := range list {
		hydrateCloudAccountRegions(&list[i])
		list[i].SecretKey = maskedSecret(list[i].SecretKey)
	}
	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func (s *Service) ListAssetCloudAccountOptions() ([]model.AssetCloudAccount, error) {
	var list []model.AssetCloudAccount
	err := s.db.Where("status = ?", 1).Order("id desc").Find(&list).Error
	for i := range list {
		hydrateCloudAccountRegions(&list[i])
		list[i].SecretKey = maskedSecret(list[i].SecretKey)
	}
	return list, err
}

func (s *Service) GetAssetCloudAccount(id uint) (*model.AssetCloudAccount, error) {
	var item model.AssetCloudAccount
	if err := s.db.First(&item, id).Error; err != nil {
		return &item, err
	}
	hydrateCloudAccountRegions(&item)
	return &item, nil
}

func normalizeCloudRegions(regions []string, legacy string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(regions)+1)
	appendRegion := func(value string) {
		for _, region := range strings.FieldsFunc(value, func(r rune) bool {
			return r == ',' || r == '，' || r == ';' || r == '；' || r == '\n' || r == '\r' || r == '\t' || r == ' '
		}) {
			region = strings.TrimSpace(region)
			if region == "" {
				continue
			}
			if _, exists := seen[region]; exists {
				continue
			}
			seen[region] = struct{}{}
			result = append(result, region)
		}
	}
	for _, region := range regions {
		appendRegion(region)
	}
	appendRegion(legacy)
	return result
}

func cloudRegionsJSON(regions []string) string {
	encoded, err := json.Marshal(regions)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

func hydrateCloudAccountRegions(account *model.AssetCloudAccount) {
	account.Regions = normalizeCloudRegions(account.Regions, account.Region)
	if account.Region == "" {
		account.Region = strings.Join(account.Regions, ",")
	}
}

func (s *Service) CreateAssetCloudAccount(payload AssetCloudAccountPayload) error {
	regions := normalizeCloudRegions(payload.Regions, payload.Region)
	if len(regions) == 0 {
		return errors.New("at least one sync region is required")
	}
	item := model.AssetCloudAccount{
		Name:        Trimmed(payload.Name),
		Provider:    Trimmed(payload.Provider),
		AccessKey:   Trimmed(payload.AccessKey),
		SecretKey:   payload.SecretKey,
		Regions:     regions,
		Region:      strings.Join(regions, ","),
		Status:      payload.Status,
		Description: Trimmed(payload.Description),
	}
	if item.Status == 0 {
		item.Status = 1
	}
	return s.db.Create(&item).Error
}

func (s *Service) UpdateAssetCloudAccount(payload AssetCloudAccountPayload) error {
	regions := normalizeCloudRegions(payload.Regions, payload.Region)
	if len(regions) == 0 {
		return errors.New("at least one sync region is required")
	}
	var existing model.AssetCloudAccount
	if err := s.db.Select("id", "access_key", "secret_key").First(&existing, payload.ID).Error; err != nil {
		return err
	}
	// J5-3 (plan phase4 r2): a rotated v1 credential makes the backfilled V2
	// inventory binding a lie — the broker would serve the OLD material.
	// Detect the rotation and unlink the binding in the same transaction as
	// the save; `sync-inventory` re-creates the chain from the new material.
	credentialRotated := Trimmed(payload.AccessKey) != existing.AccessKey ||
		(payload.SecretKey != "" && payload.SecretKey != existing.SecretKey)
	updates := map[string]any{
		"name":        Trimmed(payload.Name),
		"provider":    Trimmed(payload.Provider),
		"access_key":  Trimmed(payload.AccessKey),
		"regions":     cloudRegionsJSON(regions),
		"region":      strings.Join(regions, ","),
		"status":      payload.Status,
		"description": Trimmed(payload.Description),
	}
	if payload.SecretKey != "" {
		updates["secret_key"] = payload.SecretKey
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.AssetCloudAccount{}).Where("id = ?", payload.ID).Updates(updates).Error; err != nil {
			return err
		}
		if credentialRotated {
			if err := tx.Where("purpose = ?", "inventory").
				Where("provider_connection_id IN (?)",
					tx.Table("provider_connection").Select("id").
						Where("source_model = ? AND source_id = ?", "asset_cloud_account", payload.ID),
				).Delete(&infraModel.ProviderCredentialBinding{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) DeleteAssetCloudAccount(id uint) error {
	var count int64
	s.db.Model(&model.AssetHost{}).Where("cloud_account_id = ?", id).Count(&count)
	if count > 0 {
		return errors.New("cloud account is referenced by hosts and cannot be deleted")
	}
	return s.db.Delete(&model.AssetCloudAccount{}, id).Error
}

func normalizedAuthType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "key", "private_key", "\u5bc6\u94a5\u8ba4\u8bc1":
		return "key"
	default:
		return "password"
	}
}

func maskCredential(item *model.AssetCredential) {
	item.Password = maskedSecret(item.Password)
	item.PrivateKey = maskedSecret(item.PrivateKey)
	item.Passphrase = maskedSecret(item.Passphrase)
}

func maskedSecret(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return "******"
}
