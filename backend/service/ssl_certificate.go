package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"ops-admin/backend/apperr"
	"ops-admin/backend/internal/domain/provider"
	"ops-admin/backend/model"
	"ops-admin/backend/util"
)

const (
	certificateTaskPending = "PENDING"
	certificateTaskRunning = "RUNNING"
	certificateTaskSuccess = "SUCCESS"
	certificateTaskFailed  = "FAILED"
)

type SSLCertificateUploadPayload struct {
	Name             string `json:"name"`
	MainDomain       string `json:"mainDomain"`
	CertificatePEM   string `json:"certificatePem"`
	PrivateKeyPEM    string `json:"privateKeyPem"`
	CertificateChain string `json:"certificateChain"`
	AutoRenew        bool   `json:"autoRenew"`
	RenewBeforeDays  int    `json:"renewBeforeDays"`
	SyncToCloud      bool   `json:"syncToCloud"`
}

type SSLCertificateApplyPayload struct {
	Name              string                   `json:"name"`
	MainDomain        string                   `json:"mainDomain"`
	Type              model.SSLCertificateType `json:"type"`
	CertificateDomain string                   `json:"certificateDomain"`
	IncludeRootDomain bool                     `json:"includeRootDomain"`
	AutoRenew         bool                     `json:"autoRenew"`
	RenewBeforeDays   int                      `json:"renewBeforeDays"`
	ACMEEnvironment   string                   `json:"acmeEnvironment"`
	ACMEEmail         string                   `json:"acmeEmail"`
}

type SSLCertificateDeletePayload struct {
	ID          uint `json:"id"`
	DeleteCloud bool `json:"deleteCloud"`
}

type SSLCertificateRenewSettingPayload struct {
	ID              uint `json:"id"`
	AutoRenew       bool `json:"autoRenew"`
	RenewBeforeDays int  `json:"renewBeforeDays"`
}

type certificateParsed struct {
	Leaf         *x509.Certificate
	Domains      []string
	Type         model.SSLCertificateType
	Issuer       string
	Serial       string
	Fingerprint  string
	KeyAlgorithm string
}

func (s *Service) ListSSLCertificates(pageNum, pageSize int, keyword, status, source string, accountID uint) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	s.refreshCertificateLifecycleStatuses()
	query := s.db.Model(&model.SSLCertificate{})
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR main_domain LIKE ?", like, like)
	}
	if status = strings.TrimSpace(status); status != "" {
		query = query.Where("status = ?", strings.ToUpper(status))
	}
	if source = strings.TrimSpace(source); source != "" {
		query = query.Where("source = ?", strings.ToUpper(source))
	}
	if accountID > 0 {
		query = query.Where("dns_account_id = ?", accountID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	items := []model.SSLCertificate{}
	if err := query.Order("not_after asc,id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, err
	}
	rows, err := s.certificateDTOs(items)
	if err != nil {
		return nil, err
	}
	stats, err := s.certificateStats()
	if err != nil {
		return nil, err
	}
	return map[string]any{"list": rows, "total": total, "stats": stats}, nil
}

func (s *Service) GetSSLCertificate(id uint) (map[string]any, error) {
	var item model.SSLCertificate
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	rows, err := s.certificateDTOs([]model.SSLCertificate{item})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	tasks := []model.SSLCertificateTask{}
	_ = s.db.Where("certificate_id = ?", id).Order("id desc").Limit(20).Find(&tasks).Error
	rows[0]["tasks"] = tasks
	return rows[0], nil
}

func (s *Service) certificateDTOs(items []model.SSLCertificate) ([]map[string]any, error) {
	ids := make([]uint, 0, len(items))
	accountIDs := []uint{}
	for _, item := range items {
		ids = append(ids, item.ID)
		if item.DNSAccountID > 0 {
			accountIDs = append(accountIDs, item.DNSAccountID)
		}
	}
	domainsByID := map[uint][]model.SSLCertificateDomain{}
	if len(ids) > 0 {
		var domains []model.SSLCertificateDomain
		if err := s.db.Where("certificate_id IN ?", ids).Order("id asc").Find(&domains).Error; err != nil {
			return nil, err
		}
		for _, domain := range domains {
			domainsByID[domain.CertificateID] = append(domainsByID[domain.CertificateID], domain)
		}
	}
	accounts := map[uint]model.PublicDNSAccount{}
	if len(accountIDs) > 0 {
		var list []model.PublicDNSAccount
		if err := s.db.Where("id IN ?", accountIDs).Find(&list).Error; err != nil {
			return nil, err
		}
		for _, account := range list {
			accounts[account.ID] = account
		}
	}
	rows := make([]map[string]any, 0, len(items))
	for _, item := range items {
		domainRows := domainsByID[item.ID]
		domains := make([]string, 0, len(domainRows))
		domainObjects := make([]map[string]any, 0, len(domainRows))
		for _, domain := range domainRows {
			domains = append(domains, domain.Domain)
			domainObjects = append(domainObjects, map[string]any{"domain": domain.Domain, "domainType": domain.DomainType})
		}
		remaining := 0
		if item.NotAfter != nil {
			remaining = int(time.Until(*item.NotAfter).Hours() / 24)
			if remaining < 0 {
				remaining = 0
			}
		}
		account := accounts[item.DNSAccountID]
		rows = append(rows, map[string]any{
			"id": item.ID, "name": item.Name, "mainDomain": item.MainDomain, "type": item.Type, "source": item.Source,
			"provider": item.Provider, "dnsAccountId": item.DNSAccountID, "dnsAccountName": account.Name, "status": item.Status,
			"issuer": item.Issuer, "serialNumber": item.SerialNumber, "fingerprintSha256": item.FingerprintSHA256,
			"keyAlgorithm": item.KeyAlgorithm, "notBefore": item.NotBefore, "notAfter": item.NotAfter, "remainingDays": remaining,
			"autoRenew": item.AutoRenew, "renewBeforeDays": item.RenewBeforeDays, "cloudCertificateId": item.CloudCertificateID,
			"cloudSyncStatus": item.CloudSyncStatus, "lastSyncAt": item.LastSyncAt, "lastSyncError": item.LastSyncError,
			"lastRenewAttempt": item.LastRenewAttempt, "lastRenewError": item.LastRenewError, "renewRetryCount": item.RenewRetryCount,
			"acmeCa": item.ACMECA, "includeRootDomain": item.IncludeRootDomain, "hasPrivateKey": item.HasPrivateKey,
			"domains": domains, "domainDetails": domainObjects, "createdBy": item.CreatedBy, "createdAt": item.CreatedAt, "updatedAt": item.UpdatedAt,
		})
	}
	return rows, nil
}

func (s *Service) certificateStats() (map[string]int64, error) {
	stats := map[string]int64{"total": 0, "valid": 0, "expiring": 0, "expired": 0, "autoRenew": 0}
	for key, query := range map[string]*gorm.DB{
		"total":     s.db.Model(&model.SSLCertificate{}),
		"valid":     s.db.Model(&model.SSLCertificate{}).Where("status IN ?", []model.SSLCertificateStatus{model.SSLCertificateNormal, model.SSLCertificateIssued}),
		"expiring":  s.db.Model(&model.SSLCertificate{}).Where("status = ?", model.SSLCertificateExpiring),
		"expired":   s.db.Model(&model.SSLCertificate{}).Where("status = ?", model.SSLCertificateExpired),
		"autoRenew": s.db.Model(&model.SSLCertificate{}).Where("auto_renew = ?", true),
	} {
		var count int64
		if err := query.Count(&count).Error; err != nil {
			return nil, err
		}
		stats[key] = count
	}
	return stats, nil
}

func (s *Service) UploadSSLCertificate(payload SSLCertificateUploadPayload, actor DNSAuditActor) (uint, error) {
	payload.Name = strings.TrimSpace(payload.Name)
	payload.MainDomain = normalizePublicName(payload.MainDomain)
	if payload.Name == "" {
		return 0, errors.New("certificate name is required")
	}
	parsed, err := parseCertificateAndKey(payload.CertificatePEM, payload.PrivateKeyPEM)
	if err != nil {
		s.writeCertificateAudit(actor, 0, "Upload Certificate", payload.MainDomain, nil, "", 0, err)
		return 0, err
	}
	if time.Now().After(parsed.Leaf.NotAfter) {
		err = errors.New("expired certificate cannot be uploaded")
		s.writeCertificateAudit(actor, 0, "Upload Certificate", payload.MainDomain, parsed.Domains, "", 0, err)
		return 0, err
	}
	snapshot, err := s.resolvePublicDomainSnapshot(payload.MainDomain, parsed.Domains, 0)
	if err != nil {
		s.writeCertificateAudit(actor, 0, "Upload Certificate", payload.MainDomain, parsed.Domains, "", 0, err)
		return 0, err
	}
	if payload.MainDomain == "" {
		payload.MainDomain = snapshot.Domain
	}
	for _, domain := range parsed.Domains {
		if !domainWithinMain(domain, payload.MainDomain) {
			return 0, fmt.Errorf("certificate domain %s does not belong to public main domain %s", domain, payload.MainDomain)
		}
	}
	cipherText, err := util.EncryptSecretV2(strings.TrimSpace(payload.PrivateKeyPEM))
	if err != nil {
		return 0, err
	}
	if payload.RenewBeforeDays <= 0 {
		payload.RenewBeforeDays = 30
	}
	notBefore, notAfter := parsed.Leaf.NotBefore, parsed.Leaf.NotAfter
	item := model.SSLCertificate{Name: payload.Name, MainDomain: payload.MainDomain, Type: parsed.Type, Source: model.SSLCertificateSourceManual, Provider: snapshot.Provider, DNSAccountID: snapshot.AccountID, Status: lifecycleStatus(&notAfter, 30), Issuer: parsed.Issuer, SerialNumber: parsed.Serial, FingerprintSHA256: parsed.Fingerprint, KeyAlgorithm: parsed.KeyAlgorithm, NotBefore: &notBefore, NotAfter: &notAfter, CertificatePEM: strings.TrimSpace(payload.CertificatePEM), PrivateKeyCipher: cipherText, CertificateChain: strings.TrimSpace(payload.CertificateChain), AutoRenew: false, RenewBeforeDays: payload.RenewBeforeDays, CloudSyncStatus: model.SSLCertificateSyncLocal, HasPrivateKey: true, CreatedBy: actor.AdminID}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		return replaceCertificateDomains(tx, item.ID, payload.MainDomain, parsed.Domains)
	})
	s.writeCertificateAudit(actor, item.ID, "Upload Certificate", payload.MainDomain, parsed.Domains, item.Provider, item.DNSAccountID, err)
	if err == nil && payload.SyncToCloud {
		_, _ = s.QueueCertificateTask(item.ID, "SYNC", actor)
	}
	return item.ID, err
}

func (s *Service) CreateSSLCertificateApplication(payload SSLCertificateApplyPayload, actor DNSAuditActor) (uint, uint, error) {
	payload.MainDomain = normalizePublicName(payload.MainDomain)
	payload.CertificateDomain = normalizePublicName(payload.CertificateDomain)
	if payload.MainDomain == "" || payload.CertificateDomain == "" {
		return 0, 0, errors.New("main domain and certificate domain are required")
	}
	if payload.Type != model.SSLCertificateTypeSingle && payload.Type != model.SSLCertificateTypeWildcard {
		return 0, 0, errors.New("the current phase supports only single-domain and wildcard certificates")
	}
	if payload.Type == model.SSLCertificateTypeWildcard && !strings.HasPrefix(payload.CertificateDomain, "*.") {
		return 0, 0, errors.New("wildcard certificate must use the *.example.com format")
	}
	if payload.Type == model.SSLCertificateTypeSingle && strings.HasPrefix(payload.CertificateDomain, "*.") {
		return 0, 0, errors.New("single-domain certificate cannot contain a wildcard")
	}
	if !domainWithinMain(payload.CertificateDomain, payload.MainDomain) {
		return 0, 0, errors.New("certificate domain does not belong to the selected main domain")
	}
	snapshot, err := s.resolvePublicDomainSnapshot(payload.MainDomain, []string{payload.CertificateDomain}, 0)
	if err != nil {
		return 0, 0, err
	}
	var account model.PublicDNSAccount
	if err := s.db.First(&account, snapshot.AccountID).Error; err != nil || account.Status != 1 {
		return 0, 0, errors.New("the current main domain has no available DNS cloud account for DNS ownership validation")
	}
	if payload.Name = strings.TrimSpace(payload.Name); payload.Name == "" {
		payload.Name = payload.CertificateDomain
	}
	if payload.RenewBeforeDays <= 0 {
		payload.RenewBeforeDays = 30
	}
	ca, email := s.certificateCAForEnvironment(payload.ACMEEnvironment), strings.TrimSpace(payload.ACMEEmail)
	if email == "" {
		email = strings.TrimSpace(s.certificateConfig.Email)
	}
	if email == "" {
		return 0, 0, errors.New("ACME contact email is not configured")
	}
	item := model.SSLCertificate{Name: payload.Name, MainDomain: payload.MainDomain, Type: payload.Type, Source: model.SSLCertificateSourceACME, Provider: snapshot.Provider, DNSAccountID: snapshot.AccountID, Status: model.SSLCertificatePending, AutoRenew: payload.AutoRenew, RenewBeforeDays: payload.RenewBeforeDays, CloudSyncStatus: model.SSLCertificateSyncLocal, ACMECA: ca, ACMEEmail: email, IncludeRootDomain: payload.IncludeRootDomain, CreatedBy: actor.AdminID}
	domains := []string{payload.CertificateDomain}
	if payload.Type == model.SSLCertificateTypeWildcard && payload.IncludeRootDomain {
		domains = append(domains, payload.MainDomain)
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		return replaceCertificateDomains(tx, item.ID, payload.MainDomain, domains)
	})
	if err != nil {
		return 0, 0, err
	}
	taskID, err := s.QueueCertificateTask(item.ID, "APPLY", actor)
	s.writeCertificateAudit(actor, item.ID, "Apply Certificate", item.MainDomain, domains, item.Provider, item.DNSAccountID, err)
	return item.ID, taskID, err
}

func (s *Service) QueueCertificateCloudSync(accountID uint, actor DNSAuditActor) ([]uint, error) {
	accounts := []model.PublicDNSAccount{}
	query := s.db.Where("status = ?", 1)
	if accountID > 0 {
		query = query.Where("id = ?", accountID)
	}
	if err := query.Find(&accounts).Error; err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, errors.New("no available DNS cloud account")
	}
	ids := []uint{}
	for _, account := range accounts {
		key := fmt.Sprintf("account:%d:sync", account.ID)
		var existing model.SSLCertificateTask
		if err := s.db.Where("active_key = ?", key).First(&existing).Error; err == nil {
			ids = append(ids, existing.ID)
			continue
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return ids, err
		}
		task := model.SSLCertificateTask{CertificateID: 0, ActiveKey: &key, AdminID: actor.AdminID, Username: actor.Username, IPAddress: actor.IP, TaskType: "SYNC", Status: certificateTaskPending, Provider: account.Provider, Stage: fmt.Sprintf("ACCOUNT:%d", account.ID)}
		if err := s.db.Create(&task).Error; err != nil {
			if lookupErr := s.db.Where("active_key = ?", key).First(&existing).Error; lookupErr == nil {
				ids = append(ids, existing.ID)
				continue
			}
			return ids, err
		}
		ids = append(ids, task.ID)
		go s.runCertificateTask(task.ID)
	}
	return ids, nil
}

func (s *Service) QueueCertificateTask(certificateID uint, taskType string, actor DNSAuditActor) (uint, error) {
	taskType = strings.ToUpper(strings.TrimSpace(taskType))
	if taskType != "APPLY" && taskType != "RENEW" && taskType != "SYNC" && taskType != "DELETE" {
		return 0, errors.New("unsupported certificate task type")
	}
	var cert model.SSLCertificate
	if err := s.db.First(&cert, certificateID).Error; err != nil {
		return 0, err
	}
	if taskType == "RENEW" && cert.Source != model.SSLCertificateSourceACME {
		return 0, errors.New("only certificates issued by the platform ACME workflow support renewal")
	}
	keyType := strings.ToLower(taskType)
	if taskType == "APPLY" || taskType == "RENEW" {
		keyType = "issuance"
	}
	key := fmt.Sprintf("certificate:%d:%s", certificateID, keyType)
	var existing model.SSLCertificateTask
	if err := s.db.Where("active_key = ?", key).First(&existing).Error; err == nil {
		return 0, certificateTaskConflictError(taskType)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	task := model.SSLCertificateTask{CertificateID: certificateID, ActiveKey: &key, AdminID: actor.AdminID, Username: actor.Username, IPAddress: actor.IP, TaskType: taskType, Status: certificateTaskPending, Provider: cert.Provider, Stage: "QUEUED"}
	if err := s.db.Create(&task).Error; err != nil {
		if lookupErr := s.db.Where("active_key = ?", key).First(&existing).Error; lookupErr == nil {
			return 0, certificateTaskConflictError(taskType)
		}
		return 0, err
	}
	go s.runCertificateTask(task.ID)
	return task.ID, nil
}

func certificateTaskConflictError(taskType string) error {
	if taskType == "APPLY" || taskType == "RENEW" {
		return errors.New("an issuance or renewal task is already running for this certificate")
	}
	return errors.New("an identical task is already running for this certificate")
}

func (s *Service) ListSSLCertificateTasks(certificateID uint, limit int) ([]model.SSLCertificateTask, error) {
	if limit < 1 || limit > 100 {
		limit = 30
	}
	list := []model.SSLCertificateTask{}
	query := s.db.Order("id desc").Limit(limit)
	if certificateID > 0 {
		query = query.Where("certificate_id = ?", certificateID)
	}
	return list, query.Find(&list).Error
}

func (s *Service) runCertificateTask(taskID uint) {
	now := time.Now()
	result := s.db.Model(&model.SSLCertificateTask{}).Where("id = ? AND status = ?", taskID, certificateTaskPending).Updates(map[string]any{"status": certificateTaskRunning, "started_at": &now, "stage": "STARTING", "progress": 2})
	if result.Error != nil || result.RowsAffected != 1 {
		return
	}
	var task model.SSLCertificateTask
	if s.db.First(&task, taskID).Error != nil {
		return
	}
	var err error
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("certificate task panic: %v", recovered)
		}
		finished := time.Now()
		updates := map[string]any{"status": certificateTaskSuccess, "stage": "COMPLETED", "progress": 100, "finished_at": &finished, "error_message": "", "active_key": nil}
		if err != nil {
			updates["status"] = certificateTaskFailed
			updates["stage"] = "FAILED"
			updates["error_message"] = safeCertificateError(err)
		}
		_ = s.db.Model(&model.SSLCertificateTask{}).Where("id = ?", taskID).Updates(updates).Error
	}()
	switch task.TaskType {
	case "SYNC":
		if task.CertificateID == 0 {
			err = s.syncCloudAccountTask(task)
		} else {
			err = s.syncCertificateToCloudTask(task)
		}
	case "APPLY", "RENEW":
		err = s.executeACMECertificateTask(task)
	case "DELETE":
		err = s.deleteCertificateTask(task)
	default:
		err = errors.New("unknown certificate task")
	}
}

func (s *Service) syncCloudAccountTask(task model.SSLCertificateTask) error {
	accountID := uint(0)
	_, _ = fmt.Sscanf(task.Stage, "ACCOUNT:%d", &accountID)
	cloud, account, err := s.certificateCloudProvider(accountID)
	if err != nil {
		return err
	}
	s.updateCertificateTask(task.ID, "FETCHING_CLOUD", 20)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	items, err := cloud.ListCertificates(ctx)
	if err != nil {
		return err
	}
	for index, item := range items {
		if err := s.upsertCloudCertificate(account, item); err != nil {
			return err
		}
		s.updateCertificateTask(task.ID, "SAVING", 20+int(float64(index+1)/float64(max(1, len(items)))*70))
	}
	s.writeCertificateAudit(DNSAuditActor{AdminID: task.AdminID, Username: task.Username, IP: task.IPAddress}, 0, "Synchronize Certificate", "", nil, account.Provider, account.ID, nil)
	return nil
}

func (s *Service) upsertCloudCertificate(account *model.PublicDNSAccount, cloud provider.CloudCertificate) error {
	if strings.TrimSpace(cloud.ID) == "" {
		return errors.New("cloud certificate is missing a certificate ID")
	}
	mainDomain := normalizePublicName(cloud.MainDomain)
	snapshot, err := s.resolvePublicDomainSnapshot(mainDomain, cloud.Domains, account.ID)
	if err == nil {
		mainDomain = snapshot.Domain
	}
	if mainDomain == "" && len(cloud.Domains) > 0 {
		mainDomain = strings.TrimPrefix(normalizePublicName(cloud.Domains[0]), "*.")
	}
	source := model.SSLCertificateSourceAliyun
	if account.Provider == "tencent" {
		source = model.SSLCertificateSourceTencent
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		var item model.SSLCertificate
		err := tx.Where("provider = ? AND dns_account_id = ? AND cloud_certificate_id = ?", account.Provider, account.ID, cloud.ID).First(&item).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		updates := map[string]any{"name": firstNonEmpty(cloud.Name, mainDomain, cloud.ID), "main_domain": mainDomain, "type": normalizeCertificateType(cloud.Type), "source": source, "provider": account.Provider, "dns_account_id": account.ID, "status": lifecycleStatus(cloud.NotAfter, s.certificateConfig.ExpiryWarningDays), "issuer": cloud.Issuer, "serial_number": cloud.SerialNumber, "fingerprint_sha256": cloud.Fingerprint, "not_before": cloud.NotBefore, "not_after": cloud.NotAfter, "cloud_certificate_id": cloud.ID, "cloud_sync_status": model.SSLCertificateSyncSynced, "last_sync_at": time.Now(), "last_sync_error": ""}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			item = model.SSLCertificate{AutoRenew: false, RenewBeforeDays: 30, HasPrivateKey: false}
			if err := tx.Model(&item).Create(updates).Error; err != nil {
				return err
			}
		} else if err := tx.Model(&item).Updates(updates).Error; err != nil {
			return err
		}
		return replaceCertificateDomains(tx, item.ID, mainDomain, cloud.Domains)
	})
}

func (s *Service) syncCertificateToCloudTask(task model.SSLCertificateTask) error {
	var cert model.SSLCertificate
	if err := s.db.First(&cert, task.CertificateID).Error; err != nil {
		return err
	}
	if cert.CertificatePEM == "" || cert.PrivateKeyCipher == "" {
		return errors.New("the current certificate has no certificate body and private key available for upload")
	}
	keyField, fieldErr := registeredSecretField("ssl_certificates", "private_key_cipher")
	if fieldErr != nil {
		return fieldErr
	}
	privateKey, err := util.ReadSecretField(cert.PrivateKeyCipher, keyField, false)
	if err != nil {
		return err
	}
	cloud, _, err := s.certificateCloudProvider(cert.DNSAccountID)
	if err != nil {
		return err
	}
	s.updateCertificateTask(task.ID, "UPLOADING_CLOUD", 55)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cloudID, err := cloud.UploadCertificate(ctx, provider.CertificateUpload{Name: cert.Name, CertificatePEM: cert.CertificatePEM, PrivateKeyPEM: privateKey, CertificateChain: cert.CertificateChain})
	now := time.Now()
	updates := map[string]any{"last_sync_at": &now, "last_sync_error": "", "cloud_sync_status": model.SSLCertificateSyncSynced, "cloud_certificate_id": cloudID}
	if err != nil {
		updates["last_sync_error"] = safeCertificateError(err)
		updates["cloud_sync_status"] = model.SSLCertificateCloudSyncFailed
	}
	_ = s.db.Model(&cert).Updates(updates).Error
	s.writeCertificateAudit(DNSAuditActor{AdminID: task.AdminID, Username: task.Username, IP: task.IPAddress}, cert.ID, "Synchronize to Cloud", cert.MainDomain, s.loadCertificateDomainNames(cert.ID), cert.Provider, cert.DNSAccountID, err)
	return err
}

func (s *Service) UpdateSSLCertificateRenewSettings(payload SSLCertificateRenewSettingPayload, actor DNSAuditActor) error {
	if payload.RenewBeforeDays < 1 || payload.RenewBeforeDays > 90 {
		return errors.New("renew-before days must be between 1 and 90")
	}
	var cert model.SSLCertificate
	if err := s.db.First(&cert, payload.ID).Error; err != nil {
		return err
	}
	if payload.AutoRenew && cert.Source != model.SSLCertificateSourceACME {
		return errors.New("only platform ACME certificates support automatic renewal")
	}
	err := s.db.Model(&cert).Updates(map[string]any{"auto_renew": payload.AutoRenew, "renew_before_days": payload.RenewBeforeDays}).Error
	s.writeCertificateAudit(actor, cert.ID, "Update Automatic Renewal", cert.MainDomain, s.loadCertificateDomainNames(cert.ID), cert.Provider, cert.DNSAccountID, err)
	return err
}

func (s *Service) DeleteSSLCertificate(payload SSLCertificateDeletePayload, actor DNSAuditActor) (uint, error) {
	var cert model.SSLCertificate
	if err := s.db.First(&cert, payload.ID).Error; err != nil {
		return 0, err
	}
	if err := s.validateCertificateDeletion(cert); err != nil {
		s.writeCertificateAudit(actor, cert.ID, "Delete Certificate", cert.MainDomain, s.loadCertificateDomainNames(cert.ID), cert.Provider, cert.DNSAccountID, err)
		return 0, err
	}
	if payload.DeleteCloud && cert.CloudCertificateID != "" {
		return s.QueueCertificateTask(cert.ID, "DELETE", actor)
	}
	err := s.deleteCertificateLocal(cert.ID)
	s.writeCertificateAudit(actor, cert.ID, "Delete Certificate", cert.MainDomain, s.loadCertificateDomainNames(cert.ID), cert.Provider, cert.DNSAccountID, err)
	return 0, err
}

func (s *Service) validateCertificateDeletion(cert model.SSLCertificate) error {
	domains := s.loadCertificateDomainNames(cert.ID)
	if cert.NotAfter != nil && !time.Now().Before(*cert.NotAfter) {
		return nil
	}
	if cert.Type == model.SSLCertificateTypeWildcard {
		return errors.New("an unexpired wildcard certificate cannot be deleted")
	}
	p, _, err := s.publicProvider(cert.DNSAccountID)
	if err != nil {
		return fmt.Errorf("failed to refresh public DNS before deletion: %w", err)
	}
	return validateCertificateDeletionWithProvider(cert, domains, p, time.Now())
}

func validateCertificateDeletionWithProvider(cert model.SSLCertificate, domains []string, p provider.PublicDNSProvider, now time.Time) error {
	if cert.NotAfter != nil && !now.Before(*cert.NotAfter) {
		return nil
	}
	if cert.Type == model.SSLCertificateTypeWildcard {
		return errors.New("an unexpired wildcard certificate cannot be deleted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	records, err := p.ListRecords(ctx, cert.MainDomain)
	if err != nil {
		return fmt.Errorf("failed to refresh public DNS before deletion: %w", err)
	}
	for _, record := range records {
		fqdn := normalizePublicName(record.Host + "." + record.Domain)
		if record.Host == "@" {
			fqdn = normalizePublicName(record.Domain)
		}
		for _, domain := range domains {
			if fqdn == normalizePublicName(domain) {
				return errors.New("public DNS records still exist for this certificate domain; delete the related records before deleting the certificate")
			}
		}
	}
	return nil
}

func (s *Service) deleteCertificateTask(task model.SSLCertificateTask) error {
	var cert model.SSLCertificate
	if err := s.db.First(&cert, task.CertificateID).Error; err != nil {
		return err
	}
	if err := s.validateCertificateDeletion(cert); err != nil {
		return err
	}
	cloud, _, err := s.certificateCloudProvider(cert.DNSAccountID)
	if err != nil {
		return err
	}
	s.updateCertificateTask(task.ID, "DELETING_CLOUD", 45)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := cloud.DeleteCertificate(ctx, cert.CloudCertificateID); err != nil {
		// The process may have stopped after the cloud deletion succeeded but
		// before the local transaction ran. Confirm absence so a recovered task
		// remains idempotent even when the provider reports "not found".
		checkCtx, checkCancel := context.WithTimeout(context.Background(), 60*time.Second)
		items, checkErr := cloud.ListCertificates(checkCtx)
		checkCancel()
		stillExists := checkErr != nil
		if checkErr == nil {
			for _, item := range items {
				if item.ID == cert.CloudCertificateID {
					stillExists = true
					break
				}
			}
		}
		if stillExists {
			return fmt.Errorf("cloud deletion failed; local certificate was retained: %w", err)
		}
	}
	s.updateCertificateTask(task.ID, "DELETING_LOCAL", 80)
	domains := s.loadCertificateDomainNames(cert.ID)
	err = s.deleteCertificateLocal(cert.ID)
	s.writeCertificateAudit(DNSAuditActor{AdminID: task.AdminID, Username: task.Username, IP: task.IPAddress}, cert.ID, "Delete Platform and Cloud Certificate", cert.MainDomain, domains, cert.Provider, cert.DNSAccountID, err)
	return err
}

func (s *Service) deleteCertificateLocal(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("certificate_id = ?", id).Delete(&model.SSLCertificateDomain{}).Error; err != nil {
			return err
		}
		if err := tx.Where("certificate_id = ?", id).Delete(&model.SSLCertificateVersion{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.SSLCertificate{}, id).Error
	})
}

func (s *Service) DownloadSSLCertificate(id uint, kind string) ([]byte, string, string, error) {
	var cert model.SSLCertificate
	if err := s.db.First(&cert, id).Error; err != nil {
		return nil, "", "", err
	}
	base := sanitizeCertificateFilename(firstNonEmpty(cert.MainDomain, cert.Name, fmt.Sprintf("certificate-%d", cert.ID)))
	switch strings.ToLower(kind) {
	case "certificate":
		return []byte(cert.CertificatePEM), base + ".crt.pem", "application/x-pem-file", nil
	case "chain":
		return []byte(cert.CertificateChain), base + ".chain.pem", "application/x-pem-file", nil
	case "private-key":
		if cert.PrivateKeyCipher == "" {
			return nil, "", "", errors.New("the certificate has no private key stored by the platform")
		}
		keyField, fieldErr := registeredSecretField("ssl_certificates", "private_key_cipher")
		if fieldErr != nil {
			return nil, "", "", fieldErr
		}
		key, err := util.ReadSecretField(cert.PrivateKeyCipher, keyField, false)
		return []byte(key), base + ".key.pem", "application/x-pem-file", err
	case "zip":
		if cert.PrivateKeyCipher == "" {
			return nil, "", "", errors.New("the certificate has no private key stored by the platform")
		}
		keyField, fieldErr := registeredSecretField("ssl_certificates", "private_key_cipher")
		if fieldErr != nil {
			return nil, "", "", fieldErr
		}
		key, err := util.ReadSecretField(cert.PrivateKeyCipher, keyField, false)
		if err != nil {
			return nil, "", "", err
		}
		var buffer bytes.Buffer
		archive := zip.NewWriter(&buffer)
		for name, content := range map[string]string{base + ".crt.pem": cert.CertificatePEM, base + ".key.pem": key, base + ".chain.pem": cert.CertificateChain, base + ".fullchain.pem": strings.TrimSpace(cert.CertificatePEM + "\n" + cert.CertificateChain)} {
			writer, _ := archive.Create(name)
			_, _ = writer.Write([]byte(content))
		}
		if err := archive.Close(); err != nil {
			return nil, "", "", err
		}
		return buffer.Bytes(), base + ".zip", "application/zip", nil
	default:
		return nil, "", "", errors.New("unsupported certificate download type")
	}
}

func (s *Service) AuditSSLCertificateDownload(id uint, kind string, actor DNSAuditActor, err error) {
	var cert model.SSLCertificate
	if loadErr := s.db.First(&cert, id).Error; loadErr != nil {
		return
	}
	action := "Download Certificate"
	if kind == "private-key" || kind == "zip" {
		action = "Download Private Key"
	}
	s.writeCertificateAudit(actor, cert.ID, action, cert.MainDomain, s.loadCertificateDomainNames(cert.ID), cert.Provider, cert.DNSAccountID, err)
}

func (s *Service) ListSSLCertificateAudits(limit int) ([]model.SSLCertificateAuditLog, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	list := []model.SSLCertificateAuditLog{}
	return list, s.db.Order("id desc").Limit(limit).Find(&list).Error
}

func (s *Service) SSLCertificateDomainOptions() ([]map[string]any, error) {
	var rows []struct {
		model.PublicDomainSnapshot
		AccountName   string `json:"accountName"`
		AccountStatus int    `json:"accountStatus"`
	}
	err := s.db.Table("domain_public_snapshot s").Select("s.*,a.name AS account_name,a.status AS account_status").Joins("JOIN domain_public_dns_account a ON a.id=s.account_id").Order("s.domain asc").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := []map[string]any{}
	for _, row := range rows {
		if row.AccountStatus == 1 {
			result = append(result, map[string]any{"domain": row.Domain, "provider": row.Provider, "accountId": row.AccountID, "accountName": row.AccountName})
		}
	}
	return result, nil
}

func parseCertificateAndKey(certPEM, keyPEM string) (*certificateParsed, error) {
	certBlock, _ := pem.Decode([]byte(strings.TrimSpace(certPEM)))
	if certBlock == nil || certBlock.Type != "CERTIFICATE" {
		return nil, errors.New("invalid certificate PEM format")
	}
	leaf, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}
	privateKey, algorithm, err := parsePrivateKey([]byte(strings.TrimSpace(keyPEM)))
	if err != nil {
		return nil, err
	}
	publicDER, err := x509.MarshalPKIXPublicKey(privateKey.Public())
	if err != nil {
		return nil, err
	}
	certPublicDER, err := x509.MarshalPKIXPublicKey(leaf.PublicKey)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(publicDER, certPublicDER) {
		return nil, apperr.New("SSL_CERTIFICATE_KEY_MISMATCH", nil)
	}
	domains := append([]string{}, leaf.DNSNames...)
	if len(domains) == 0 && strings.TrimSpace(leaf.Subject.CommonName) != "" {
		domains = []string{leaf.Subject.CommonName}
	}
	for index := range domains {
		domains[index] = normalizePublicName(domains[index])
	}
	domains = uniqueStrings(domains)
	if len(domains) == 0 {
		return nil, errors.New("certificate contains no recognizable CN or SAN domain")
	}
	typeValue := model.SSLCertificateTypeSingle
	for _, domain := range domains {
		if strings.HasPrefix(domain, "*.") {
			typeValue = model.SSLCertificateTypeWildcard
			break
		}
	}
	if typeValue == model.SSLCertificateTypeSingle && len(domains) > 1 {
		typeValue = model.SSLCertificateTypeSAN
	}
	fingerprint := sha256.Sum256(leaf.Raw)
	return &certificateParsed{Leaf: leaf, Domains: domains, Type: typeValue, Issuer: leaf.Issuer.String(), Serial: strings.ToUpper(leaf.SerialNumber.Text(16)), Fingerprint: strings.ToUpper(hex.EncodeToString(fingerprint[:])), KeyAlgorithm: algorithm}, nil
}

func parsePrivateKey(data []byte) (crypto.Signer, string, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, "", errors.New("invalid private-key PEM format")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		switch value := key.(type) {
		case *rsa.PrivateKey:
			return value, fmt.Sprintf("RSA-%d", value.N.BitLen()), nil
		case *ecdsa.PrivateKey:
			return value, "ECDSA-" + value.Curve.Params().Name, nil
		}
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, fmt.Sprintf("RSA-%d", key.N.BitLen()), nil
	}
	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key, "ECDSA-" + key.Curve.Params().Name, nil
	}
	return nil, "", errors.New("unsupported or unparseable private key")
}

func (s *Service) certificateCloudProvider(accountID uint) (provider.CertificateCloudProvider, *model.PublicDNSAccount, error) {
	var account model.PublicDNSAccount
	if err := s.db.First(&account, accountID).Error; err != nil {
		return nil, nil, err
	}
	if account.Status != 1 {
		return nil, nil, errors.New("DNS cloud account is disabled")
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
		return nil, nil, errors.New("certificate DNS account access key is empty")
	}
	secret, err := util.ReadSecretField(account.SecretKeyCipher, secretField, false)
	if err != nil {
		return nil, nil, err
	}
	if secret == "" {
		return nil, nil, errors.New("certificate DNS account secret key is empty")
	}
	cloud, err := provider.NewCertificateCloud(account.Provider, access, secret)
	return cloud, &account, err
}

func (s *Service) resolvePublicDomainSnapshot(mainDomain string, domains []string, accountID uint) (*model.PublicDomainSnapshot, error) {
	mainDomain = normalizePublicName(mainDomain)
	query := s.db.Model(&model.PublicDomainSnapshot{})
	if accountID > 0 {
		query = query.Where("account_id = ?", accountID)
	}
	var snapshots []model.PublicDomainSnapshot
	if err := query.Order("LENGTH(domain) desc").Find(&snapshots).Error; err != nil {
		return nil, err
	}
	for _, snapshot := range snapshots {
		if mainDomain != "" && normalizePublicName(snapshot.Domain) == mainDomain {
			return &snapshot, nil
		}
		for _, domain := range domains {
			if domainWithinMain(domain, snapshot.Domain) {
				return &snapshot, nil
			}
		}
	}
	return nil, errors.New("the current main domain has no available DNS cloud account for DNS ownership validation")
}

func replaceCertificateDomains(tx *gorm.DB, certificateID uint, mainDomain string, domains []string) error {
	if err := tx.Where("certificate_id = ?", certificateID).Delete(&model.SSLCertificateDomain{}).Error; err != nil {
		return err
	}
	rows := []model.SSLCertificateDomain{}
	for index, domain := range uniqueStrings(domains) {
		domainType := "SAN"
		if strings.HasPrefix(domain, "*.") {
			domainType = "WILDCARD"
		} else if index == 0 || normalizePublicName(domain) == normalizePublicName(mainDomain) {
			domainType = "MAIN"
		}
		rows = append(rows, model.SSLCertificateDomain{CertificateID: certificateID, Domain: normalizePublicName(domain), DomainType: domainType})
	}
	if len(rows) == 0 {
		return errors.New("certificate domain is required")
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}

func (s *Service) loadCertificateDomainNames(id uint) []string {
	var rows []model.SSLCertificateDomain
	_ = s.db.Where("certificate_id = ?", id).Order("id asc").Find(&rows).Error
	result := []string{}
	for _, row := range rows {
		result = append(result, row.Domain)
	}
	return result
}
func (s *Service) updateCertificateTask(id uint, stage string, progress int) {
	_ = s.db.Model(&model.SSLCertificateTask{}).Where("id = ?", id).Updates(map[string]any{"stage": stage, "progress": progress}).Error
}
func (s *Service) writeCertificateAudit(actor DNSAuditActor, certificateID uint, action, mainDomain string, domains []string, providerName string, accountID uint, err error) {
	item := model.SSLCertificateAuditLog{CertificateID: certificateID, AdminID: actor.AdminID, Username: actor.Username, IPAddress: actor.IP, Action: action, MainDomain: mainDomain, Domains: strings.Join(domains, ","), Provider: providerName, AccountID: accountID, Success: err == nil}
	if err != nil {
		item.Error = safeCertificateError(err)
	}
	_ = s.db.Create(&item).Error
}
func (s *Service) refreshCertificateLifecycleStatuses() {
	var items []model.SSLCertificate
	if s.db.Where("not_after IS NOT NULL AND status NOT IN ?", []model.SSLCertificateStatus{model.SSLCertificateApplying, model.SSLCertificateRenewing, model.SSLCertificateApplyFailed, model.SSLCertificateRenewFailed, model.SSLCertificateRevoked}).Find(&items).Error != nil {
		return
	}
	for _, item := range items {
		status := lifecycleStatus(item.NotAfter, s.certificateConfig.ExpiryWarningDays)
		if status != item.Status {
			_ = s.db.Model(&item).Update("status", status).Error
		}
	}
}
func lifecycleStatus(notAfter *time.Time, warningDays int) model.SSLCertificateStatus {
	if notAfter == nil {
		return model.SSLCertificateIssued
	}
	if !time.Now().Before(*notAfter) {
		return model.SSLCertificateExpired
	}
	if warningDays <= 0 {
		warningDays = 30
	}
	if time.Until(*notAfter) <= time.Duration(warningDays)*24*time.Hour {
		return model.SSLCertificateExpiring
	}
	return model.SSLCertificateNormal
}
func normalizeCertificateType(value string) model.SSLCertificateType {
	switch strings.ToUpper(value) {
	case "WILDCARD":
		return model.SSLCertificateTypeWildcard
	case "SAN":
		return model.SSLCertificateTypeSAN
	default:
		return model.SSLCertificateTypeSingle
	}
}
func normalizePublicName(value string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
}
func domainWithinMain(domain, main string) bool {
	domain = strings.TrimPrefix(normalizePublicName(domain), "*.")
	main = normalizePublicName(main)
	return domain == main || strings.HasSuffix(domain, "."+main)
}
func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		value = normalizePublicName(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}
func safeCertificateError(err error) string {
	if err == nil {
		return ""
	}
	value := err.Error()
	if len(value) > 1000 {
		value = value[:1000]
	}
	return value
}
func sanitizeCertificateFilename(value string) string {
	replacer := strings.NewReplacer("*", "wildcard", "/", "-", "\\", "-", "..", "-")
	value = replacer.Replace(value)
	if value == "" {
		return "certificate"
	}
	return value
}
