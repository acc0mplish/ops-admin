package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/miekg/dns"
	"gorm.io/gorm"
	"ops-admin/backend/internal/domain/dnsserver"
	"ops-admin/backend/internal/domain/provider"
	"ops-admin/backend/model"
	"ops-admin/backend/util"
)

type DNSAuditActor struct {
	AdminID      uint
	Username, IP string
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
		if value, err := util.DecryptSecret(item.AccessKeyCipher); err == nil {
			hint = maskDNSKey(value)
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
		accessCipher, err = util.EncryptSecret(strings.TrimSpace(payload.AccessKey))
		if err != nil {
			return err
		}
	}
	if strings.TrimSpace(payload.SecretKey) != "" {
		secretCipher, err = util.EncryptSecret(strings.TrimSpace(payload.SecretKey))
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
	access, err := util.DecryptSecret(account.AccessKeyCipher)
	if err != nil {
		return nil, nil, err
	}
	secret, err := util.DecryptSecret(account.SecretKeyCipher)
	if err != nil {
		return nil, nil, err
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

func (s *Service) GetInternalDNSSettings() (map[string]any, error) {
	settings, err := dnsserver.LoadSettings(s.db)
	if err != nil {
		return nil, err
	}
	return map[string]any{"settings": settings, "status": s.dnsManager.Status()}, nil
}
func (s *Service) SaveInternalDNSSettings(payload InternalDNSSettingsPayload, actor DNSAuditActor) error {
	// DNS uses the standard port by product definition. Never trust a client
	// supplied port, including direct API calls that bypass the UI.
	payload.ListenPort = 53
	oldSettings, loadErr := dnsserver.LoadSettings(s.db)
	if loadErr != nil {
		return loadErr
	}
	settings := dnsserver.Settings{Enabled: payload.Enabled, ListenAddress: payload.ListenAddress, ListenPort: 53, Upstreams: payload.Upstreams, TimeoutSeconds: payload.TimeoutSeconds}
	data, _ := json.Marshal(payload.Upstreams)
	item := model.InternalDNSSetting{ID: 1, Enabled: payload.Enabled, ListenAddress: payload.ListenAddress, ListenPort: 53, UpstreamsJSON: string(data), TimeoutSeconds: payload.TimeoutSeconds}
	if err := s.dnsManager.Apply(settings); err != nil {
		_ = s.db.Model(&model.InternalDNSSetting{}).Where("id = ?", 1).Updates(map[string]any{"enabled": false, "last_error": err.Error()}).Error
		s.writeDNSAudit(actor, "Enable Internal DNS", "internal", "", "", "", "", "", err)
		return err
	}
	now := time.Now()
	updates := map[string]any{"enabled": item.Enabled, "listen_address": item.ListenAddress, "listen_port": item.ListenPort, "upstreams_json": item.UpstreamsJSON, "timeout_seconds": item.TimeoutSeconds, "last_error": ""}
	if item.Enabled {
		updates["last_started_at"] = &now
	}
	err := s.db.Model(&model.InternalDNSSetting{}).Where("id = ?", 1).Updates(updates).Error
	if err != nil {
		_ = s.dnsManager.Apply(oldSettings)
	}
	s.writeDNSAudit(actor, "Save Internal DNS Settings", "internal", "", "", "", "", "", err)
	return err
}

func (s *Service) ListInternalZones(keyword string) ([]model.InternalDNSZone, error) {
	query := s.db.Model(&model.InternalDNSZone{}).Select("domain_internal_dns_zone.*, COUNT(domain_internal_dns_record.id) AS record_count").Joins("LEFT JOIN domain_internal_dns_record ON domain_internal_dns_record.zone_id=domain_internal_dns_zone.id").Group("domain_internal_dns_zone.id")
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		query = query.Where("domain_internal_dns_zone.name LIKE ?", "%"+keyword+"%")
	}
	list := make([]model.InternalDNSZone, 0)
	err := query.Order("domain_internal_dns_zone.name asc").Scan(&list).Error
	return list, err
}
func (s *Service) SaveInternalZone(payload InternalZonePayload, actor DNSAuditActor) error {
	name, err := normalizeZone(payload.Name)
	if err != nil {
		return err
	}
	if payload.Status == 0 {
		payload.Status = 1
	}
	var snapshot *dnsserver.Snapshot
	var old model.InternalDNSZone
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if payload.ID == 0 {
			old = model.InternalDNSZone{}
			if err := tx.Create(&model.InternalDNSZone{Name: name, Description: strings.TrimSpace(payload.Description), Status: payload.Status}).Error; err != nil {
				return err
			}
		} else {
			if err := tx.First(&old, payload.ID).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.InternalDNSZone{}).Where("id = ?", payload.ID).Updates(map[string]any{"name": name, "description": strings.TrimSpace(payload.Description), "status": payload.Status}).Error; err != nil {
				return err
			}
		}
		snapshot, err = dnsserver.BuildSnapshot(tx)
		return err
	})
	if err == nil {
		s.dnsManager.ReplaceSnapshot(snapshot)
	}
	s.writeDNSAudit(actor, map[bool]string{true: "Create Internal DNS Zone", false: "Update Internal DNS Zone"}[payload.ID == 0], "internal", name, name, "", old.Name, name, err)
	return err
}
func (s *Service) DeleteInternalZone(id uint, actor DNSAuditActor) error {
	var zone model.InternalDNSZone
	if err := s.db.First(&zone, id).Error; err != nil {
		return err
	}
	var snapshot *dnsserver.Snapshot
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("zone_id = ?", id).Delete(&model.InternalDNSRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&zone).Error; err != nil {
			return err
		}
		var buildErr error
		snapshot, buildErr = dnsserver.BuildSnapshot(tx)
		return buildErr
	})
	if err == nil {
		s.dnsManager.ReplaceSnapshot(snapshot)
	}
	s.writeDNSAudit(actor, "Delete Internal DNS Zone", "internal", zone.Name, zone.Name, "", zone.Name, "", err)
	return err
}
func (s *Service) ListInternalRecords(zoneID uint, keyword string) ([]model.InternalDNSRecord, error) {
	query := s.db.Where("zone_id = ?", zoneID)
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		query = query.Where("host LIKE ? OR value LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	list := make([]model.InternalDNSRecord, 0)
	err := query.Order("host asc,type asc").Find(&list).Error
	return list, err
}
func (s *Service) SaveInternalRecord(payload InternalRecordPayload, actor DNSAuditActor) error {
	var zone model.InternalDNSZone
	if err := s.db.First(&zone, payload.ZoneID).Error; err != nil {
		return errors.New("DNS zone does not exist")
	}
	payload.Host = strings.ToLower(strings.TrimSpace(payload.Host))
	payload.Type = strings.ToUpper(strings.TrimSpace(payload.Type))
	payload.Value = strings.TrimSpace(payload.Value)
	if payload.TTL == 0 {
		payload.TTL = 300
	}
	if payload.Status == 0 {
		payload.Status = 1
	}
	if err := validateDNSHost(payload.Host); err != nil {
		return err
	}
	if payload.Type == "A" {
		ip := net.ParseIP(payload.Value)
		if ip == nil || ip.To4() == nil {
			return errors.New("A record value must be a valid IPv4 address")
		}
	} else if payload.Type == "CNAME" {
		normalized, err := normalizeFQDN(payload.Value)
		if err != nil {
			return err
		}
		payload.Value = normalized
	} else {
		return errors.New("the current internal DNS phase supports only A and CNAME records")
	}
	var old model.InternalDNSRecord
	var snapshot *dnsserver.Snapshot
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := validateRecordConflict(tx, payload); err != nil {
			return err
		}
		if payload.Type == "CNAME" {
			if err := validateCNAMELoop(tx, payload, zone.Name); err != nil {
				return err
			}
		}
		item := model.InternalDNSRecord{ID: payload.ID, ZoneID: payload.ZoneID, Host: payload.Host, Type: payload.Type, Value: payload.Value, TTL: payload.TTL, Status: payload.Status}
		if payload.ID == 0 {
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Where("id = ? AND zone_id = ?", payload.ID, payload.ZoneID).First(&old).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errors.New("DNS record does not exist or does not belong to the current zone")
				}
				return err
			}
			if err := tx.Model(&model.InternalDNSRecord{}).Where("id = ? AND zone_id = ?", payload.ID, payload.ZoneID).Updates(map[string]any{"host": item.Host, "type": item.Type, "value": item.Value, "ttl": item.TTL, "status": item.Status}).Error; err != nil {
				return err
			}
		}
		var buildErr error
		snapshot, buildErr = dnsserver.BuildSnapshot(tx)
		return buildErr
	})
	if err == nil {
		s.dnsManager.ReplaceSnapshot(snapshot)
	}
	domain := internalFQDN(payload.Host, zone.Name)
	s.writeDNSAudit(actor, map[bool]string{true: "Create Internal DNS Record", false: "Update Internal DNS Record"}[payload.ID == 0], "internal", zone.Name, domain, payload.Type, old.Value, payload.Value, err)
	return err
}
func (s *Service) DeleteInternalRecord(id uint, actor DNSAuditActor) error {
	var item model.InternalDNSRecord
	if err := s.db.First(&item, id).Error; err != nil {
		return err
	}
	var zone model.InternalDNSZone
	_ = s.db.First(&zone, item.ZoneID).Error
	var snapshot *dnsserver.Snapshot
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&item).Error; err != nil {
			return err
		}
		var buildErr error
		snapshot, buildErr = dnsserver.BuildSnapshot(tx)
		return buildErr
	})
	if err == nil {
		s.dnsManager.ReplaceSnapshot(snapshot)
	}
	s.writeDNSAudit(actor, "Delete Internal DNS Record", "internal", zone.Name, internalFQDN(item.Host, zone.Name), item.Type, item.Value, "", err)
	return err
}
func (s *Service) BatchInternalRecords(payload InternalRecordBatchPayload, actor DNSAuditActor) (int, error) {
	var zone model.InternalDNSZone
	if err := s.db.First(&zone, payload.ZoneID).Error; err != nil {
		return 0, errors.New("DNS zone does not exist")
	}
	action := strings.ToLower(strings.TrimSpace(payload.Action))
	actionNames := map[string]string{"create": "Batch Create Internal DNS Records", "update": "Batch Update Internal DNS Records", "delete": "Batch Delete Internal DNS Records", "enable": "Batch Enable Internal DNS Records", "disable": "Batch Disable Internal DNS Records"}
	actionName, ok := actionNames[action]
	if !ok {
		return 0, errors.New("unsupported batch operation")
	}
	var snapshot *dnsserver.Snapshot
	affected := 0
	err := s.db.Transaction(func(tx *gorm.DB) error {
		switch action {
		case "create", "update":
			if len(payload.Records) == 0 {
				return errors.New("provide at least one DNS record")
			}
			for index, record := range payload.Records {
				record.ZoneID = payload.ZoneID
				normalized, normalizeErr := normalizeInternalRecordPayload(record)
				if normalizeErr != nil {
					return fmt.Errorf("record %d: %w", index+1, normalizeErr)
				}
				if action == "create" {
					normalized.ID = 0
				} else if normalized.ID == 0 {
					return fmt.Errorf("record %d is missing an ID", index+1)
				} else {
					var existing model.InternalDNSRecord
					if findErr := tx.Where("id = ? AND zone_id = ?", normalized.ID, payload.ZoneID).First(&existing).Error; findErr != nil {
						return fmt.Errorf("record %d does not exist", index+1)
					}
				}
				if validateErr := validateRecordConflict(tx, normalized); validateErr != nil {
					return fmt.Errorf("record %d: %w", index+1, validateErr)
				}
				if normalized.Type == "CNAME" {
					if loopErr := validateCNAMELoop(tx, normalized, zone.Name); loopErr != nil {
						return fmt.Errorf("record %d: %w", index+1, loopErr)
					}
				}
				item := model.InternalDNSRecord{ID: normalized.ID, ZoneID: normalized.ZoneID, Host: normalized.Host, Type: normalized.Type, Value: normalized.Value, TTL: normalized.TTL, Status: normalized.Status}
				if action == "create" {
					if createErr := tx.Create(&item).Error; createErr != nil {
						return fmt.Errorf("failed to save record %d: %w", index+1, createErr)
					}
				} else if updateErr := tx.Model(&model.InternalDNSRecord{}).Where("id = ? AND zone_id = ?", item.ID, payload.ZoneID).Updates(map[string]any{"host": item.Host, "type": item.Type, "value": item.Value, "ttl": item.TTL, "status": item.Status}).Error; updateErr != nil {
					return fmt.Errorf("failed to save record %d: %w", index+1, updateErr)
				}
				affected++
			}
		case "delete", "enable", "disable":
			ids := uniqueUintIDs(payload.IDs)
			if len(ids) == 0 {
				return errors.New("select at least one DNS record")
			}
			var existing []model.InternalDNSRecord
			if findErr := tx.Where("zone_id = ? AND id IN ?", payload.ZoneID, ids).Find(&existing).Error; findErr != nil {
				return findErr
			}
			if len(existing) != len(ids) {
				return errors.New("some DNS records do not exist or do not belong to the current zone")
			}
			query := tx.Where("zone_id = ? AND id IN ?", payload.ZoneID, ids)
			if action == "delete" {
				if deleteErr := query.Delete(&model.InternalDNSRecord{}).Error; deleteErr != nil {
					return deleteErr
				}
			} else {
				status := 1
				if action == "disable" {
					status = 2
				}
				if updateErr := query.Model(&model.InternalDNSRecord{}).Update("status", status).Error; updateErr != nil {
					return updateErr
				}
			}
			affected = len(ids)
		}
		var buildErr error
		snapshot, buildErr = dnsserver.BuildSnapshot(tx)
		return buildErr
	})
	if err == nil {
		s.dnsManager.ReplaceSnapshot(snapshot)
	}
	s.writeDNSAudit(actor, actionName, "internal", zone.Name, zone.Name, "", fmt.Sprintf("%d records", affected), fmt.Sprintf("%d records", affected), err)
	return affected, err
}

func normalizeInternalRecordPayload(payload InternalRecordPayload) (InternalRecordPayload, error) {
	payload.Host = strings.ToLower(strings.TrimSpace(payload.Host))
	payload.Type = strings.ToUpper(strings.TrimSpace(payload.Type))
	payload.Value = strings.TrimSpace(payload.Value)
	if payload.TTL == 0 {
		payload.TTL = 300
	}
	if payload.Status == 0 {
		payload.Status = 1
	}
	if err := validateDNSHost(payload.Host); err != nil {
		return payload, err
	}
	if payload.Type == "A" {
		ip := net.ParseIP(payload.Value)
		if ip == nil || ip.To4() == nil {
			return payload, errors.New("A record value must be a valid IPv4 address")
		}
	} else if payload.Type == "CNAME" {
		normalized, err := normalizeFQDN(payload.Value)
		if err != nil {
			return payload, err
		}
		payload.Value = normalized
	} else {
		return payload, errors.New("the current internal DNS phase supports only A and CNAME records")
	}
	return payload, nil
}

func uniqueUintIDs(values []uint) []uint {
	seen := make(map[uint]struct{}, len(values))
	result := make([]uint, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
func (s *Service) TestDNSResolution(domainName, recordType string) (map[string]any, error) {
	recordType = strings.ToUpper(strings.TrimSpace(recordType))
	qtype, ok := map[string]uint16{"A": dns.TypeA, "CNAME": dns.TypeCNAME}[recordType]
	if !ok {
		return nil, errors.New("test type supports only A or CNAME")
	}
	request := new(dns.Msg)
	request.SetQuestion(dns.Fqdn(domainName), qtype)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	response := s.dnsManager.Resolve(ctx, request)
	elapsed := float64(time.Since(start).Microseconds()) / 1000
	if response.Rcode != dns.RcodeSuccess {
		return map[string]any{"status": "failed", "rcode": dns.RcodeToString[response.Rcode], "responseTimeMs": elapsed}, nil
	}
	answers := []map[string]any{}
	for _, rr := range response.Answer {
		value := ""
		switch typed := rr.(type) {
		case *dns.A:
			value = typed.A.String()
		case *dns.CNAME:
			value = typed.Target
		default:
			value = rr.String()
		}
		answers = append(answers, map[string]any{"value": value, "ttl": rr.Header().Ttl, "type": dns.TypeToString[rr.Header().Rrtype]})
	}
	status := s.dnsManager.Status()
	return map[string]any{"status": "success", "dnsServer": status["listenAddress"], "answers": answers, "responseTimeMs": elapsed}, nil
}

func (s *Service) ListDNSAuditLogs(pageNum, pageSize int) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var total int64
	if err := s.db.Model(&model.DNSAuditLog{}).Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.DNSAuditLog
	if err := s.db.Order("id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}
func (s *Service) writeDNSAudit(actor DNSAuditActor, action, providerName, zone, domainName, recordType, oldValue, newValue string, err error) {
	item := model.DNSAuditLog{AdminID: actor.AdminID, Username: actor.Username, IPAddress: actor.IP, Action: action, Provider: providerName, Zone: zone, Domain: domainName, RecordType: recordType, OldValue: oldValue, NewValue: newValue, Success: err == nil}
	if err != nil {
		item.Error = err.Error()
	}
	_ = s.db.Create(&item).Error
}

var dnsLabelPattern = regexp.MustCompile(`^[a-zA-Z0-9_*-](?:[a-zA-Z0-9_*-]{0,61}[a-zA-Z0-9_*-])?$`)

func validateDNSHost(value string) error {
	if value == "@" {
		return nil
	}
	if value == "" {
		return errors.New("host record is required")
	}
	for _, part := range strings.Split(value, ".") {
		if !dnsLabelPattern.MatchString(part) {
			return fmt.Errorf("invalid host record format: %q", value)
		}
	}
	return nil
}
func normalizeZone(value string) (string, error) {
	value = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
	if value == "" || !strings.Contains(value, ".") {
		return "", errors.New("zone must be a fully qualified domain name, for example ops.internal")
	}
	if err := validateDNSHost(value); err != nil {
		return "", err
	}
	return value, nil
}
func normalizeFQDN(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	trimmed := strings.TrimSuffix(value, ".")
	if !strings.Contains(trimmed, ".") {
		return "", errors.New("CNAME record value must be a fully qualified domain name")
	}
	if err := validateDNSHost(trimmed); err != nil {
		return "", err
	}
	return dns.Fqdn(trimmed), nil
}
func internalFQDN(host, zone string) string {
	if host == "@" || host == "" {
		return dns.Fqdn(zone)
	}
	return dns.Fqdn(host + "." + zone)
}
func validateRecordConflict(tx *gorm.DB, payload InternalRecordPayload) error {
	opposite := "CNAME"
	if payload.Type == "CNAME" {
		opposite = "A"
	}
	var count int64
	query := tx.Model(&model.InternalDNSRecord{}).Where("zone_id = ? AND host = ? AND type = ?", payload.ZoneID, payload.Host, opposite)
	if payload.ID > 0 {
		query = query.Where("id <> ?", payload.ID)
	}
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("CNAME and A records cannot coexist for the same name")
	}
	return nil
}
func validateCNAMELoop(tx *gorm.DB, payload InternalRecordPayload, zone string) error {
	source := internalFQDN(payload.Host, zone)
	target := dns.Fqdn(strings.ToLower(payload.Value))
	if source == target {
		return errors.New("CNAME cannot point to itself")
	}
	var records []model.InternalDNSRecord
	if err := tx.Where("zone_id = ? AND type = 'CNAME'", payload.ZoneID).Find(&records).Error; err != nil {
		return err
	}
	edges := map[string]string{source: target}
	for _, record := range records {
		if record.ID != payload.ID {
			edges[internalFQDN(record.Host, zone)] = dns.Fqdn(strings.ToLower(record.Value))
		}
	}
	if cnameHasLoop(edges, source) {
		return errors.New("CNAME configuration creates a cycle")
	}
	return nil
}
func cnameHasLoop(edges map[string]string, source string) bool {
	seen := map[string]struct{}{}
	current := source
	for i := 0; i <= len(edges); i++ {
		if _, ok := seen[current]; ok {
			return true
		}
		seen[current] = struct{}{}
		next, ok := edges[current]
		if !ok {
			return false
		}
		current = next
	}
	return true
}
func maskDNSKey(value string) string {
	runes := []rune(value)
	if len(runes) <= 8 {
		return "****"
	}
	return string(runes[:4]) + "…" + string(runes[len(runes)-4:])
}
