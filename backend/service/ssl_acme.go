package service

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
	"gorm.io/gorm"
	"ops-admin/backend/internal/domain/provider"
	"ops-admin/backend/model"
	"ops-admin/backend/util"
)

type CertificateRuntimeConfig struct {
	Email                 string
	ProductionCA          string
	StagingCA             string
	DNSPollingSeconds     int
	DNSPropagationSeconds int
	ExpiryWarningDays     int
}

func defaultCertificateRuntimeConfig() CertificateRuntimeConfig {
	config := CertificateRuntimeConfig{
		Email:                 os.Getenv("OPS_ADMIN_ACME_EMAIL"),
		ProductionCA:          firstNonEmpty(os.Getenv("OPS_ADMIN_ACME_CA_PRODUCTION"), lego.LEDirectoryProduction),
		StagingCA:             firstNonEmpty(os.Getenv("OPS_ADMIN_ACME_CA_STAGING"), lego.LEDirectoryStaging),
		DNSPollingSeconds:     envCertificateInt("OPS_ADMIN_ACME_DNS_POLLING_SECONDS", 2),
		DNSPropagationSeconds: envCertificateInt("OPS_ADMIN_ACME_DNS_TIMEOUT_SECONDS", 120),
		ExpiryWarningDays:     envCertificateInt("OPS_ADMIN_SSL_EXPIRY_WARNING_DAYS", 30),
	}
	return config
}

func (s *Service) ConfigureCertificate(config CertificateRuntimeConfig) {
	defaults := defaultCertificateRuntimeConfig()
	if strings.TrimSpace(config.Email) == "" {
		config.Email = defaults.Email
	}
	if strings.TrimSpace(config.ProductionCA) == "" {
		config.ProductionCA = defaults.ProductionCA
	}
	if strings.TrimSpace(config.StagingCA) == "" {
		config.StagingCA = defaults.StagingCA
	}
	if config.DNSPollingSeconds <= 0 {
		config.DNSPollingSeconds = defaults.DNSPollingSeconds
	}
	if config.DNSPropagationSeconds <= 0 {
		config.DNSPropagationSeconds = defaults.DNSPropagationSeconds
	}
	if config.ExpiryWarningDays <= 0 {
		config.ExpiryWarningDays = defaults.ExpiryWarningDays
	}
	s.certificateConfig = config
	s.certificateOnce.Do(func() {
		if s.opsScheduler != nil && s.opsScheduler.cron != nil {
			_, _ = s.opsScheduler.cron.AddFunc("0 15 2 * * *", func() { s.runAutomaticCertificateMaintenance() })
		}
		s.resumeCertificateTasks()
	})
}

func (s *Service) certificateCAForEnvironment(environment string) string {
	if strings.EqualFold(strings.TrimSpace(environment), "staging") || strings.EqualFold(strings.TrimSpace(environment), "test") {
		return s.certificateConfig.StagingCA
	}
	return s.certificateConfig.ProductionCA
}

type acmeUser struct {
	email        string
	registration *registration.Resource
	privateKey   crypto.PrivateKey
}

func (u *acmeUser) GetEmail() string                        { return u.email }
func (u *acmeUser) GetRegistration() *registration.Resource { return u.registration }
func (u *acmeUser) GetPrivateKey() crypto.PrivateKey        { return u.privateKey }

type acmeDNSChallengeProvider struct {
	dns            provider.PublicDNSProvider
	mainDomain     string
	timeout        time.Duration
	interval       time.Duration
	onStage        func(string, int)
	recordFor      func(string, string) (string, string)
	mu             sync.Mutex
	recordIDsByKey map[string][]string
}

func (p *acmeDNSChallengeProvider) Present(domain, token, keyAuth string) error {
	recordFor := p.recordFor
	if recordFor == nil {
		recordFor = dns01.GetRecord
	}
	fqdn, value := recordFor(domain, keyAuth)
	host, err := challengeHost(fqdn, p.mainDomain)
	if err != nil {
		return err
	}
	if p.onStage != nil {
		p.onStage("CREATING_DNS_CHALLENGE", 25)
	}
	req := provider.RecordRequest{Domain: p.mainDomain, Host: host, Type: "TXT", Value: value, TTL: 600, Line: "default"}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	err = p.dns.CreateRecord(ctx, req)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create ACME TXT record: %w", err)
	}
	ids := p.findChallengeRecordIDs(host, value)
	p.mu.Lock()
	p.recordIDsByKey[challengeKey(fqdn, value)] = ids
	p.mu.Unlock()
	if p.onStage != nil {
		p.onStage("WAITING_DNS_PROPAGATION", 40)
	}
	return nil
}

func (p *acmeDNSChallengeProvider) CleanUp(domain, token, keyAuth string) error {
	recordFor := p.recordFor
	if recordFor == nil {
		recordFor = dns01.GetRecord
	}
	fqdn, value := recordFor(domain, keyAuth)
	host, err := challengeHost(fqdn, p.mainDomain)
	if err != nil {
		return err
	}
	p.mu.Lock()
	ids := append([]string{}, p.recordIDsByKey[challengeKey(fqdn, value)]...)
	delete(p.recordIDsByKey, challengeKey(fqdn, value))
	p.mu.Unlock()
	if len(ids) == 0 {
		ids = p.findChallengeRecordIDs(host, value)
	}
	var cleanupErr error
	for _, id := range ids {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		err := p.dns.DeleteRecord(ctx, p.mainDomain, id)
		cancel()
		if err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	if p.onStage != nil {
		p.onStage("ACME_VALIDATING", 62)
	}
	if cleanupErr != nil {
		return fmt.Errorf("failed to clean up ACME TXT record: %w", cleanupErr)
	}
	return nil
}

func (p *acmeDNSChallengeProvider) Timeout() (time.Duration, time.Duration) {
	return p.timeout, p.interval
}

func (p *acmeDNSChallengeProvider) findChallengeRecordIDs(host, value string) []string {
	for attempt := 0; attempt < 5; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		records, err := p.dns.ListRecords(ctx, p.mainDomain)
		cancel()
		if err == nil {
			ids := []string{}
			for _, record := range records {
				if strings.EqualFold(record.Type, "TXT") && normalizePublicName(record.Host) == normalizePublicName(host) && strings.Trim(record.Value, "\"") == strings.Trim(value, "\"") {
					ids = append(ids, record.ID)
				}
			}
			if len(ids) > 0 {
				return ids
			}
		}
		time.Sleep(time.Second)
	}
	return nil
}

func (s *Service) executeACMECertificateTask(task model.SSLCertificateTask) (taskErr error) {
	var cert model.SSLCertificate
	if err := s.db.First(&cert, task.CertificateID).Error; err != nil {
		return err
	}
	domains := s.loadCertificateDomainNames(cert.ID)
	if len(domains) == 0 {
		return errors.New("certificate has no requested domain")
	}
	oldStatus := cert.Status
	workflowStatus := model.SSLCertificateApplying
	if task.TaskType == "RENEW" {
		workflowStatus = model.SSLCertificateRenewing
	}
	if err := s.db.Model(&cert).Updates(map[string]any{"status": workflowStatus, "last_renew_attempt": time.Now()}).Error; err != nil {
		return err
	}
	defer func() {
		if taskErr == nil {
			return
		}
		status := model.SSLCertificateApplyFailed
		if task.TaskType == "RENEW" {
			status = model.SSLCertificateRenewFailed
		}
		updates := map[string]any{"status": status}
		if task.TaskType == "RENEW" {
			updates["last_renew_error"] = safeCertificateError(taskErr)
			updates["renew_retry_count"] = gorm.Expr("renew_retry_count + 1")
		}
		if cert.NotAfter != nil && time.Now().Before(*cert.NotAfter) && task.TaskType == "RENEW" {
			updates["status"] = status
			_ = oldStatus
		}
		_ = s.db.Model(&model.SSLCertificate{}).Where("id = ?", cert.ID).Updates(updates).Error
		s.writeCertificateAudit(DNSAuditActor{AdminID: task.AdminID, Username: task.Username, IP: task.IPAddress}, cert.ID, map[bool]string{true: "Renew Certificate", false: "Apply Certificate"}[task.TaskType == "RENEW"], cert.MainDomain, domains, cert.Provider, cert.DNSAccountID, taskErr)
	}()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	user := &acmeUser{email: cert.ACMEEmail, privateKey: privateKey}
	config := lego.NewConfig(user)
	config.CADirURL = firstNonEmpty(cert.ACMECA, s.certificateConfig.ProductionCA)
	config.Certificate.Timeout = 90 * time.Second
	client, err := lego.NewClient(config)
	if err != nil {
		return fmt.Errorf("failed to create ACME client: %w", err)
	}
	registrationResource, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		return fmt.Errorf("failed to register ACME account: %w", err)
	}
	user.registration = registrationResource
	dnsProvider, _, err := s.publicProvider(cert.DNSAccountID)
	if err != nil {
		return err
	}
	challengeProvider := &acmeDNSChallengeProvider{dns: dnsProvider, mainDomain: cert.MainDomain, timeout: time.Duration(s.certificateConfig.DNSPropagationSeconds) * time.Second, interval: time.Duration(s.certificateConfig.DNSPollingSeconds) * time.Second, recordIDsByKey: map[string][]string{}, onStage: func(stage string, progress int) {
		s.updateCertificateTask(task.ID, stage, progress)
		if stage == "WAITING_DNS_PROPAGATION" {
			_ = s.db.Model(&cert).Update("status", model.SSLCertificateDNSPending).Error
		} else if stage == "ACME_VALIDATING" {
			_ = s.db.Model(&cert).Update("status", model.SSLCertificateValidating).Error
		}
	}}
	if err := client.Challenge.SetDNS01Provider(challengeProvider); err != nil {
		return fmt.Errorf("failed to configure DNS-01 challenge: %w", err)
	}
	s.updateCertificateTask(task.ID, "CREATING_ACME_ORDER", 12)
	resource, err := client.Certificate.Obtain(certificate.ObtainRequest{Domains: domains, Bundle: false, AlwaysDeactivateAuthorizations: true})
	if err != nil {
		return fmt.Errorf("ACME issuance failed: %w", err)
	}
	s.updateCertificateTask(task.ID, "SAVING_CERTIFICATE", 78)
	parsed, err := parseCertificateAndKey(string(resource.Certificate), string(resource.PrivateKey))
	if err != nil {
		return fmt.Errorf("failed to parse ACME issuance result: %w", err)
	}
	cipherText, err := util.EncryptSecret(string(resource.PrivateKey))
	if err != nil {
		return fmt.Errorf("failed to encrypt private key: %w", err)
	}
	notBefore, notAfter := parsed.Leaf.NotBefore, parsed.Leaf.NotAfter
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if task.TaskType == "RENEW" && cert.CertificatePEM != "" && cert.PrivateKeyCipher != "" {
			var versionCount int64
			if err := tx.Model(&model.SSLCertificateVersion{}).Where("certificate_id = ?", cert.ID).Count(&versionCount).Error; err != nil {
				return err
			}
			version := model.SSLCertificateVersion{CertificateID: cert.ID, Version: int(versionCount) + 1, CertificatePEM: cert.CertificatePEM, PrivateKeyCipher: cert.PrivateKeyCipher, CertificateChain: cert.CertificateChain, FingerprintSHA256: cert.FingerprintSHA256, NotBefore: cert.NotBefore, NotAfter: cert.NotAfter}
			if err := tx.Create(&version).Error; err != nil {
				return err
			}
		}
		return tx.Model(&cert).Updates(map[string]any{"status": lifecycleStatus(&notAfter, s.certificateConfig.ExpiryWarningDays), "issuer": parsed.Issuer, "serial_number": parsed.Serial, "fingerprint_sha256": parsed.Fingerprint, "key_algorithm": parsed.KeyAlgorithm, "not_before": &notBefore, "not_after": &notAfter, "certificate_pem": strings.TrimSpace(string(resource.Certificate)), "private_key_cipher": cipherText, "certificate_chain": strings.TrimSpace(string(resource.IssuerCertificate)), "has_private_key": true, "last_renew_error": "", "renew_retry_count": 0, "cloud_sync_status": model.SSLCertificateSyncPending}).Error
	})
	if err != nil {
		return err
	}
	s.writeCertificateAudit(DNSAuditActor{AdminID: task.AdminID, Username: task.Username, IP: task.IPAddress}, cert.ID, map[bool]string{true: "Renew Certificate", false: "Apply Certificate"}[task.TaskType == "RENEW"], cert.MainDomain, domains, cert.Provider, cert.DNSAccountID, nil)
	s.updateCertificateTask(task.ID, "UPLOADING_CLOUD", 90)
	if err := s.syncCertificateToCloudTask(task); err != nil {
		// Issuance and cloud upload are deliberately independent. The certificate
		// remains valid locally and the failed sync can be retried from the UI.
		return nil
	}
	return nil
}

func (s *Service) runAutomaticCertificateMaintenance() {
	s.refreshCertificateLifecycleStatuses()
	var certificates []model.SSLCertificate
	if err := s.db.Where("source = ? AND auto_renew = ? AND not_after IS NOT NULL", model.SSLCertificateSourceACME, true).Find(&certificates).Error; err != nil {
		return
	}
	actor := DNSAuditActor{Username: "scheduler", IP: "127.0.0.1"}
	for _, cert := range certificates {
		days := cert.RenewBeforeDays
		if days <= 0 {
			days = 30
		}
		if cert.NotAfter != nil && time.Until(*cert.NotAfter) <= time.Duration(days)*24*time.Hour {
			_, _ = s.QueueCertificateTask(cert.ID, "RENEW", actor)
		}
	}
}

func (s *Service) resumeCertificateTasks() {
	_ = s.db.Model(&model.SSLCertificateTask{}).Where("status = ?", certificateTaskRunning).Update("status", certificateTaskPending).Error
	var tasks []model.SSLCertificateTask
	if s.db.Where("status = ?", certificateTaskPending).Order("id asc").Find(&tasks).Error != nil {
		return
	}
	for _, task := range tasks {
		go s.runCertificateTask(task.ID)
	}
}

func challengeHost(fqdn, mainDomain string) (string, error) {
	fqdn = strings.TrimSuffix(normalizePublicName(fqdn), ".")
	mainDomain = normalizePublicName(mainDomain)
	if fqdn == mainDomain {
		return "@", nil
	}
	suffix := "." + mainDomain
	if !strings.HasSuffix(fqdn, suffix) {
		return "", fmt.Errorf("ACME challenge %s does not belong to main domain %s", fqdn, mainDomain)
	}
	return strings.TrimSuffix(fqdn, suffix), nil
}

func challengeKey(fqdn, value string) string { return normalizePublicName(fqdn) + "|" + value }
func envCertificateInt(name string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
