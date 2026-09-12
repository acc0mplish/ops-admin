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
