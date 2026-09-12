package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"ops-admin/backend/internal/domain/provider"
	"ops-admin/backend/model"
	"ops-admin/backend/util"
)

type DNSAuditActor struct {
	AdminID      uint
	Username, IP string
}

// registeredSecretField resolves a §4.1 secret inventory entry for the
// credential paths below. A lookup miss fails closed so registry drift can
// never turn a secret column into a plaintext passthrough.
func registeredSecretField(table, column string) (util.SecretField, error) {
	if field, ok := util.LookupSecretField(table, column); ok {
		return field, nil
	}
	return util.SecretField{}, fmt.Errorf("secret field %s.%s is not registered in the secret field inventory", table, column)
}
type PublicDNSAccountPayload struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Provider  string `json:"provider"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
	Status    int    `json:"status"`
}
type InternalDNSSettingsPayload struct {
	Enabled        bool     `json:"enabled"`
	ListenAddress  string   `json:"listenAddress"`
	ListenPort     int      `json:"listenPort"`
	Upstreams      []string `json:"upstreams"`
	TimeoutSeconds int      `json:"timeoutSeconds"`
}
type InternalZonePayload struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      int    `json:"status"`
}
type InternalRecordPayload struct {
	ID     uint   `json:"id"`
	ZoneID uint   `json:"zoneId"`
	Host   string `json:"host"`
	Type   string `json:"type"`
	Value  string `json:"value"`
	TTL    uint32 `json:"ttl"`
	Status int    `json:"status"`
}
type InternalRecordBatchPayload struct {
	ZoneID  uint                    `json:"zoneId"`
	Action  string                  `json:"action"`
	IDs     []uint                  `json:"ids"`
	Records []InternalRecordPayload `json:"records"`
}
type PublicBatchPayload struct {
	AccountID uint                     `json:"accountId"`
	Domain    string                   `json:"domain"`
	Action    string                   `json:"action"`
	TTL       int                      `json:"ttl"`
	Value     string                   `json:"value"`
	Records   []provider.RecordRequest `json:"records"`
}

func (s *Service) Shutdown(ctx context.Context) error {
	if s.dnsManager != nil {
		return s.dnsManager.Stop(ctx)
	}
	return nil
}

func (s *Service) ListPublicDNSAccounts(pageNum, pageSize int, keyword, providerName string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	query := s.db.Model(&model.PublicDNSAccount{})
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}
	if providerName = strings.TrimSpace(providerName); providerName != "" {
		query = query.Where("provider = ?", providerName)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var items []model.PublicDNSAccount
	if err := query.Order("id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, err
	}
	list := make([]map[string]any, 0, len(items))
	for _, item := range items {
		hint := "Configured"
		// The plaintext fallback for unknown formats stays until Step 4;
		// empty values keep the generic hint, matching the previous behavior.
		if accessField, fieldErr := registeredSecretField("domain_public_dns_account", "access_key_cipher"); fieldErr == nil {
			if value, err := util.ReadSecretField(item.AccessKeyCipher, accessField, false); err == nil && value != "" {
				hint = maskDNSKey(value)
			}
		}
		list = append(list, map[string]any{"id": item.ID, "name": item.Name, "provider": item.Provider, "accessKeyHint": hint, "status": item.Status, "lastConnectionStatus": item.LastConnectionStatus, "lastConnectionError": item.LastConnectionError, "lastConnectionAt": item.LastConnectionAt, "createdAt": item.CreatedAt, "updatedAt": item.UpdatedAt})
	}
	return map[string]any{"list": list, "total": total}, nil
}
func (s *Service) PublicDNSAccountOptions() ([]map[string]any, error) {
	var accounts []model.PublicDNSAccount
	if err := s.db.Where("status = ?", 1).Order("name asc,id asc").Find(&accounts).Error; err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(accounts))
	for _, item := range accounts {
		result = append(result, map[string]any{"id": item.ID, "name": item.Name, "provider": item.Provider, "status": item.Status})
	}
	return result, nil
}
func (s *Service) GetPublicDNSAccount(id uint) (map[string]any, error) {
	var item model.PublicDNSAccount
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return map[string]any{"id": item.ID, "name": item.Name, "provider": item.Provider, "status": item.Status, "hasAccessKey": item.AccessKeyCipher != "", "hasSecretKey": item.SecretKeyCipher != ""}, nil
}
func (s *Service) SavePublicDNSAccount(payload PublicDNSAccountPayload, actor DNSAuditActor) error {
	payload.Name = strings.TrimSpace(payload.Name)
	payload.Provider = strings.ToLower(strings.TrimSpace(payload.Provider))
	if payload.Name == "" {
		return errors.New("account name is required")
	}
	if payload.Provider != "aliyun" && payload.Provider != "tencent" {
		return errors.New("only Aliyun DNS and Tencent Cloud DNSPod are supported")
	}
	if payload.Status == 0 {
		payload.Status = 1
	}
	var old model.PublicDNSAccount
	if payload.ID > 0 {
		if err := s.db.First(&old, payload.ID).Error; err != nil {
			return err
		}
	}
	accessCipher := old.AccessKeyCipher
	secretCipher := old.SecretKeyCipher
	var err error
	if strings.TrimSpace(payload.AccessKey) != "" {
		accessCipher, err = util.EncryptSecretV2(strings.TrimSpace(payload.AccessKey))
		if err != nil {
			return err
		}
	}
	if strings.TrimSpace(payload.SecretKey) != "" {
		secretCipher, err = util.EncryptSecretV2(strings.TrimSpace(payload.SecretKey))
		if err != nil {
			return err
		}
	}
	if accessCipher == "" || secretCipher == "" {
		return errors.New("AccessKey or SecretId and SecretKey are required")
	}
	item := model.PublicDNSAccount{ID: payload.ID, Name: payload.Name, Provider: payload.Provider, AccessKeyCipher: accessCipher, SecretKeyCipher: secretCipher, Status: payload.Status}
	if payload.ID == 0 {
		err = s.db.Create(&item).Error
	} else {
		err = s.db.Model(&model.PublicDNSAccount{}).Where("id = ?", payload.ID).Updates(map[string]any{"name": item.Name, "provider": item.Provider, "access_key_cipher": accessCipher, "secret_key_cipher": secretCipher, "status": item.Status}).Error
	}
	action := "Update Public DNS Account"
	if payload.ID == 0 {
		action = "Create Public DNS Account"
	}
	s.writeDNSAudit(actor, action, payload.Provider, "", payload.Name, "", old.Name, payload.Name, err)
	return err
}
func (s *Service) DeletePublicDNSAccount(id uint, actor DNSAuditActor) error {
	var item model.PublicDNSAccount
	if err := s.db.First(&item, id).Error; err != nil {
		return err
	}
	var certificateCount int64
	if err := s.db.Model(&model.SSLCertificate{}).Where("dns_account_id = ?", id).Count(&certificateCount).Error; err != nil {
		return err
	}
	if certificateCount > 0 {
		return fmt.Errorf("the DNS account is still referenced by %d SSL certificate(s); migrate or delete the certificates first", certificateCount)
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("account_id = ?", id).Delete(&model.PublicDomainSnapshot{}).Error; err != nil {
			return err
		}
		return tx.Delete(&item).Error
	})
	s.writeDNSAudit(actor, "Delete Public DNS Account", item.Provider, "", item.Name, "", item.Name, "", err)
	return err
}
func (s *Service) TestPublicDNSAccount(id uint) error {
	p, item, err := s.publicProvider(id)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, testErr := p.ListDomains(ctx)
	now := time.Now()
	updates := map[string]any{"last_connection_at": &now, "last_connection_status": "success", "last_connection_error": ""}
	if testErr != nil {
		updates["last_connection_status"] = "failed"
		updates["last_connection_error"] = testErr.Error()
	}
	_ = s.db.Model(&model.PublicDNSAccount{}).Where("id = ?", item.ID).Updates(updates).Error
	return testErr
}
func (s *Service) publicProvider(accountID uint) (provider.PublicDNSProvider, *model.PublicDNSAccount, error) {
	var account model.PublicDNSAccount
	if err := s.db.First(&account, accountID).Error; err != nil {
		return nil, nil, err
	}
	if account.Status != 1 {
		return nil, nil, errors.New("DNS account is disabled")
	}
	accessField, fieldErr := registeredSecretField("domain_public_dns_account", "access_key_cipher")
	if fieldErr != nil {
		return nil, nil, fieldErr
	}
	secretField, fieldErr := registeredSecretField("domain_public_dns_account", "secret_key_cipher")
	if fieldErr != nil {
		return nil, nil, fieldErr
	}
	access, err := util.ReadSecretField(account.AccessKeyCipher, accessField, false)
	if err != nil {
		return nil, nil, err
	}
	if access == "" {
		return nil, nil, errors.New("DNS account access key is empty")
	}
	secret, err := util.ReadSecretField(account.SecretKeyCipher, secretField, false)
	if err != nil {
		return nil, nil, err
	}
	if secret == "" {
		return nil, nil, errors.New("DNS account secret key is empty")
	}
	p, err := provider.New(account.Provider, access, secret)
	return p, &account, err
}

func (s *Service) SyncPublicDomains(accountID uint) (int, error) {
	accounts := []model.PublicDNSAccount{}
	query := s.db.Where("status = ?", 1)
	if accountID > 0 {
		query = query.Where("id = ?", accountID)
	}
	if err := query.Find(&accounts).Error; err != nil {
		return 0, err
	}
	count := 0
	for _, account := range accounts {
		p, _, err := s.publicProvider(account.ID)
		if err != nil {
			return count, err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		domains, err := p.ListDomains(ctx)
		cancel()
		if err != nil {
			return count, fmt.Errorf("%s: %w", account.Name, err)
		}
		now := time.Now()
		err = s.db.Transaction(func(tx *gorm.DB) error {
			names := make([]string, 0, len(domains))
			for _, domain := range domains {
				names = append(names, domain.Name)
				item := model.PublicDomainSnapshot{AccountID: account.ID, Provider: account.Provider, Domain: domain.Name, RecordCount: domain.RecordCount, Status: domain.Status, SyncedAt: now}
				if err := tx.Where("account_id = ? AND domain = ?", account.ID, domain.Name).Assign(item).FirstOrCreate(&item).Error; err != nil {
					return err
				}
			}
			deleteQuery := tx.Where("account_id = ?", account.ID)
			if len(names) > 0 {
				deleteQuery = deleteQuery.Where("domain NOT IN ?", names)
			}
			return deleteQuery.Delete(&model.PublicDomainSnapshot{}).Error
		})
		if err != nil {
			return count, err
		}
		count += len(domains)
	}
	return count, nil
}
func (s *Service) ListPublicDomains(pageNum, pageSize int, keyword, providerName string, accountID uint) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	query := s.db.Table("domain_public_snapshot s").Select("s.*, a.name AS account_name").Joins("JOIN domain_public_dns_account a ON a.id=s.account_id")
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		query = query.Where("s.domain LIKE ?", "%"+keyword+"%")
	}
	if providerName != "" {
		query = query.Where("s.provider = ?", providerName)
	}
	if accountID > 0 {
		query = query.Where("s.account_id = ?", accountID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []struct {
		model.PublicDomainSnapshot
		AccountName string `json:"accountName"`
	}
	if err := query.Order("s.domain asc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Scan(&list).Error; err != nil {
		return nil, err
	}
	return map[string]any{"list": list, "total": total}, nil
}
func (s *Service) ListPublicRecords(accountID uint, domain string) ([]provider.DNSRecord, error) {
	p, _, err := s.publicProvider(accountID)
	if err != nil {
		return nil, err
	}
	domain, err = s.requirePublicDomainSnapshot(accountID, domain)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return p.ListRecords(ctx, domain)
}
func (s *Service) MutatePublicRecord(action string, accountID uint, req provider.RecordRequest, actor DNSAuditActor) error {
	p, account, err := s.publicProvider(accountID)
	if err != nil {
		return err
	}
	action = strings.ToLower(strings.TrimSpace(action))
	req.Domain, err = s.requirePublicDomainSnapshot(accountID, req.Domain)
	if err != nil {
		return err
	}
	req.Type = strings.ToUpper(strings.TrimSpace(req.Type))
	if req.Domain == "" || req.RecordID == "" && action != "create" {
		return errors.New("domain or record ID is missing")
	}
	var oldValue string
	if action != "create" {
		current, lookupErr := findProviderRecord(context.Background(), p, req.Domain, req.RecordID)
		if lookupErr != nil {
			return lookupErr
		}
		oldValue = current.Value
	}
	if req.TTL == 0 {
		req.TTL = 600
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	err = executeProviderAction(ctx, p, action, req)
	s.writeDNSAudit(actor, "Public DNS Record "+action, account.Provider, "", req.Domain, req.Type, oldValue, req.Value, err)
	return err
}
func (s *Service) BatchPublicRecords(payload PublicBatchPayload, actor DNSAuditActor) map[string]any {
	p, account, err := s.publicProvider(payload.AccountID)
	if err != nil {
		return map[string]any{"successCount": 0, "failureCount": len(payload.Records), "results": []map[string]any{}, "error": err.Error()}
	}
	payload.Action = strings.ToLower(strings.TrimSpace(payload.Action))
	payload.Domain, err = s.requirePublicDomainSnapshot(payload.AccountID, payload.Domain)
	if err != nil {
		return map[string]any{"successCount": 0, "failureCount": len(payload.Records), "results": []map[string]any{}, "error": err.Error()}
	}
	allowedActions := map[string]bool{"create": true, "update": true, "delete": true, "enable": true, "disable": true, "ttl": true, "value": true}
	if !allowedActions[payload.Action] {
		return map[string]any{"successCount": 0, "failureCount": len(payload.Records), "results": []map[string]any{}, "error": "unsupported batch operation"}
	}
	currentByID := map[string]provider.DNSRecord{}
	if payload.Action != "create" {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		current, listErr := p.ListRecords(ctx, payload.Domain)
		cancel()
		if listErr != nil {
			return map[string]any{"successCount": 0, "failureCount": len(payload.Records), "results": []map[string]any{}, "error": listErr.Error()}
		}
		for _, record := range current {
			currentByID[record.ID] = record
		}
	}
	types := map[string]struct{}{}
	for _, item := range payload.Records {
		if current, ok := currentByID[item.RecordID]; ok {
			types[strings.ToUpper(current.Type)] = struct{}{}
		} else {
			types[strings.ToUpper(item.Type)] = struct{}{}
		}
	}
	if payload.Action == "value" && len(types) > 1 {
		return map[string]any{"successCount": 0, "failureCount": len(payload.Records), "results": []map[string]any{}, "error": "batch value updates require all selected records to have the same type"}
	}
	results := make([]map[string]any, 0, len(payload.Records))
	success := 0
	for _, item := range payload.Records {
		item.Domain = payload.Domain
		current, exists := currentByID[item.RecordID]
		if payload.Action != "create" && !exists {
			results = append(results, map[string]any{"recordId": item.RecordID, "host": item.Host, "success": false, "error": "record does not exist or does not belong to the current domain"})
			continue
		}
		if payload.Action == "delete" || payload.Action == "enable" || payload.Action == "disable" || payload.Action == "ttl" || payload.Action == "value" {
			item = provider.RecordRequest{Domain: payload.Domain, RecordID: current.ID, Host: current.Host, Type: current.Type, Value: current.Value, TTL: current.TTL, Line: current.Line}
		}
		if payload.Action == "ttl" {
			item.TTL = payload.TTL
		}
		if payload.Action == "value" {
			item.Value = payload.Value
		}
		action := payload.Action
		if action == "ttl" || action == "value" {
			action = "update"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err := executeProviderAction(ctx, p, action, item)
		cancel()
		s.writeDNSAudit(actor, "Public DNS Record "+action, account.Provider, "", payload.Domain, item.Type, "", item.Value, err)
		entry := map[string]any{"recordId": item.RecordID, "host": item.Host, "success": err == nil}
		if err != nil {
			entry["error"] = err.Error()
		} else {
			success++
		}
		results = append(results, entry)
	}
	return map[string]any{"successCount": success, "failureCount": len(payload.Records) - success, "results": results}
}

func (s *Service) requirePublicDomainSnapshot(accountID uint, domain string) (string, error) {
	domain = normalizePublicName(domain)
	if accountID == 0 || domain == "" {
		return "", errors.New("DNS account and domain are required")
	}
	var count int64
	if err := s.db.Model(&model.PublicDomainSnapshot{}).Where("account_id = ? AND domain = ?", accountID, domain).Count(&count).Error; err != nil {
		return "", err
	}
	if count == 0 {
		return "", errors.New("domain does not exist or is not owned by the current DNS account; refresh the domain list first")
	}
	return domain, nil
}

func findProviderRecord(parent context.Context, p provider.PublicDNSProvider, domain, recordID string) (*provider.DNSRecord, error) {
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	records, err := p.ListRecords(ctx, domain)
	if err != nil {
		return nil, err
	}
	for index := range records {
		if records[index].ID == recordID {
			return &records[index], nil
		}
	}
	return nil, errors.New("record does not exist or does not belong to the current domain")
}

func executeProviderAction(ctx context.Context, p provider.PublicDNSProvider, action string, req provider.RecordRequest) error {
	switch action {
	case "create":
		return p.CreateRecord(ctx, req)
	case "update":
		return p.UpdateRecord(ctx, req)
	case "delete":
		return p.DeleteRecord(ctx, req.Domain, req.RecordID)
	case "enable":
		return p.EnableRecord(ctx, req.Domain, req.RecordID)
	case "disable":
		return p.DisableRecord(ctx, req.Domain, req.RecordID)
	default:
		return errors.New("unsupported record operation")
	}
}
