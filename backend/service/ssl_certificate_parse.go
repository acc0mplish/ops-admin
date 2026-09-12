package service

import (
	"bytes"
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
