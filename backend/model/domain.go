package model

import "time"

type PublicDNSAccount struct {
	ID                   uint       `json:"id" gorm:"primaryKey"`
	Name                 string     `json:"name" gorm:"size:100;not null"`
	Provider             string     `json:"provider" gorm:"size:32;index;not null"`
	AccessKeyCipher      string     `json:"-" gorm:"type:text;not null"`
	SecretKeyCipher      string     `json:"-" gorm:"type:text;not null"`
	Status               int        `json:"status" gorm:"default:1;index;not null"`
	LastConnectionStatus string     `json:"lastConnectionStatus" gorm:"size:32"`
	LastConnectionError  string     `json:"lastConnectionError" gorm:"size:500"`
	LastConnectionAt     *time.Time `json:"lastConnectionAt"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}

func (PublicDNSAccount) TableName() string { return "domain_public_dns_account" }

type PublicDomainSnapshot struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	AccountID   uint      `json:"accountId" gorm:"uniqueIndex:uk_domain_account;index;not null"`
	Provider    string    `json:"provider" gorm:"size:32;index;not null"`
	Domain      string    `json:"domain" gorm:"size:255;uniqueIndex:uk_domain_account;index;not null"`
	RecordCount int       `json:"recordCount" gorm:"default:0"`
	Status      string    `json:"status" gorm:"size:32"`
	SyncedAt    time.Time `json:"syncedAt" gorm:"index"`
}

func (PublicDomainSnapshot) TableName() string { return "domain_public_snapshot" }

type InternalDNSSetting struct {
	ID             uint       `json:"id" gorm:"primaryKey"`
	Enabled        bool       `json:"enabled" gorm:"default:false;not null"`
	ListenAddress  string     `json:"listenAddress" gorm:"size:64;default:'0.0.0.0';not null"`
	ListenPort     int        `json:"listenPort" gorm:"default:53;not null"`
	UpstreamsJSON  string     `json:"-" gorm:"type:text"`
	TimeoutSeconds int        `json:"timeoutSeconds" gorm:"default:2;not null"`
	LastError      string     `json:"lastError" gorm:"size:500"`
	LastStartedAt  *time.Time `json:"lastStartedAt"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

func (InternalDNSSetting) TableName() string { return "domain_internal_dns_setting" }

type InternalDNSZone struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"size:255;uniqueIndex;not null"`
	Description string    `json:"description" gorm:"size:500"`
	Status      int       `json:"status" gorm:"default:1;index;not null"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	RecordCount int64     `json:"recordCount" gorm:"-"`
}

func (InternalDNSZone) TableName() string { return "domain_internal_dns_zone" }

type InternalDNSRecord struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	ZoneID    uint      `json:"zoneId" gorm:"uniqueIndex:uk_zone_host_type;index;not null"`
	Host      string    `json:"host" gorm:"size:255;uniqueIndex:uk_zone_host_type;not null"`
	Type      string    `json:"type" gorm:"size:16;uniqueIndex:uk_zone_host_type;not null"`
	Value     string    `json:"value" gorm:"size:500;not null"`
	TTL       uint32    `json:"ttl" gorm:"default:300;not null"`
	Status    int       `json:"status" gorm:"default:1;index;not null"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (InternalDNSRecord) TableName() string { return "domain_internal_dns_record" }

type DNSAuditLog struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	AdminID    uint      `json:"adminId" gorm:"index"`
	Username   string    `json:"username" gorm:"size:64"`
	IPAddress  string    `json:"ipAddress" gorm:"size:64"`
	Action     string    `json:"action" gorm:"size:64;index"`
	Provider   string    `json:"provider" gorm:"size:32;index"`
	Zone       string    `json:"zone" gorm:"size:255;index"`
	Domain     string    `json:"domain" gorm:"size:255;index"`
	RecordType string    `json:"recordType" gorm:"size:16"`
	OldValue   string    `json:"oldValue" gorm:"type:text"`
	NewValue   string    `json:"newValue" gorm:"type:text"`
	Success    bool      `json:"success" gorm:"index"`
	Error      string    `json:"error" gorm:"size:1000"`
	CreatedAt  time.Time `json:"createdAt" gorm:"index"`
}

func (DNSAuditLog) TableName() string { return "domain_dns_audit_log" }

type SSLCertificateSource string

const (
	SSLCertificateSourceAliyun  SSLCertificateSource = "ALIYUN"
	SSLCertificateSourceTencent SSLCertificateSource = "TENCENT"
	SSLCertificateSourceManual  SSLCertificateSource = "MANUAL"
	SSLCertificateSourceACME    SSLCertificateSource = "ACME"
)

type SSLCertificateType string

const (
	SSLCertificateTypeSingle   SSLCertificateType = "SINGLE"
	SSLCertificateTypeWildcard SSLCertificateType = "WILDCARD"
	SSLCertificateTypeSAN      SSLCertificateType = "SAN"
)

type SSLCertificateStatus string

const (
	SSLCertificatePending          SSLCertificateStatus = "PENDING"
	SSLCertificateApplying         SSLCertificateStatus = "APPLYING"
	SSLCertificateDNSPending       SSLCertificateStatus = "DNS_PENDING"
	SSLCertificateValidating       SSLCertificateStatus = "VALIDATING"
	SSLCertificateIssuing          SSLCertificateStatus = "ISSUING"
	SSLCertificateIssued           SSLCertificateStatus = "ISSUED"
	SSLCertificateNormal           SSLCertificateStatus = "NORMAL"
	SSLCertificateExpiring         SSLCertificateStatus = "EXPIRING"
	SSLCertificateExpired          SSLCertificateStatus = "EXPIRED"
	SSLCertificateRenewing         SSLCertificateStatus = "RENEWING"
	SSLCertificateRenewFailed      SSLCertificateStatus = "RENEW_FAILED"
	SSLCertificateApplyFailed      SSLCertificateStatus = "APPLY_FAILED"
	SSLCertificateStatusSyncFailed SSLCertificateStatus = "SYNC_FAILED"
	SSLCertificateRevoked          SSLCertificateStatus = "REVOKED"
)

type SSLCertificateSyncStatus string

const (
	SSLCertificateSyncLocal       SSLCertificateSyncStatus = "LOCAL"
	SSLCertificateSyncSynced      SSLCertificateSyncStatus = "SYNCED"
	SSLCertificateSyncPending     SSLCertificateSyncStatus = "PENDING"
	SSLCertificateCloudSyncFailed SSLCertificateSyncStatus = "FAILED"
)

type SSLCertificate struct {
	ID                 uint                     `json:"id" gorm:"primaryKey"`
	Name               string                   `json:"name" gorm:"size:128;not null"`
	MainDomain         string                   `json:"mainDomain" gorm:"size:255;index;not null"`
	Type               SSLCertificateType       `json:"type" gorm:"size:16;index;not null"`
	Source             SSLCertificateSource     `json:"source" gorm:"size:16;index;not null"`
	Provider           string                   `json:"provider" gorm:"size:32;index"`
	DNSAccountID       uint                     `json:"dnsAccountId" gorm:"index"`
	Status             SSLCertificateStatus     `json:"status" gorm:"size:32;index;not null"`
	Issuer             string                   `json:"issuer" gorm:"size:255"`
	SerialNumber       string                   `json:"serialNumber" gorm:"size:255"`
	FingerprintSHA256  string                   `json:"fingerprintSha256" gorm:"size:128;index"`
	KeyAlgorithm       string                   `json:"keyAlgorithm" gorm:"size:64"`
	NotBefore          *time.Time               `json:"notBefore" gorm:"index"`
	NotAfter           *time.Time               `json:"notAfter" gorm:"index"`
	CertificatePEM     string                   `json:"-" gorm:"type:longtext"`
	PrivateKeyCipher   string                   `json:"-" gorm:"type:longtext"`
	CertificateChain   string                   `json:"-" gorm:"type:longtext"`
	AutoRenew          bool                     `json:"autoRenew" gorm:"default:false;index;not null"`
	RenewBeforeDays    int                      `json:"renewBeforeDays" gorm:"default:30;not null"`
	CloudCertificateID string                   `json:"cloudCertificateId" gorm:"size:255;index"`
	CloudSyncStatus    SSLCertificateSyncStatus `json:"cloudSyncStatus" gorm:"size:16;index;not null"`
	LastSyncAt         *time.Time               `json:"lastSyncAt"`
	LastSyncError      string                   `json:"lastSyncError" gorm:"size:1000"`
	LastRenewAttempt   *time.Time               `json:"lastRenewAttempt"`
	LastRenewError     string                   `json:"lastRenewError" gorm:"size:1000"`
	RenewRetryCount    int                      `json:"renewRetryCount" gorm:"default:0;not null"`
	ACMECA             string                   `json:"acmeCa" gorm:"size:500"`
	ACMEEmail          string                   `json:"acmeEmail" gorm:"size:255"`
	IncludeRootDomain  bool                     `json:"includeRootDomain" gorm:"default:false;not null"`
	HasPrivateKey      bool                     `json:"hasPrivateKey" gorm:"default:false;not null"`
	CreatedBy          uint                     `json:"createdBy" gorm:"index"`
	CreatedAt          time.Time                `json:"createdAt"`
	UpdatedAt          time.Time                `json:"updatedAt"`
}

func (SSLCertificate) TableName() string { return "ssl_certificates" }

type SSLCertificateDomain struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	CertificateID uint      `json:"certificateId" gorm:"uniqueIndex:uk_ssl_certificate_domain;index;not null"`
	Domain        string    `json:"domain" gorm:"size:255;uniqueIndex:uk_ssl_certificate_domain;index;not null"`
	DomainType    string    `json:"domainType" gorm:"size:16;not null"`
	CreatedAt     time.Time `json:"createdAt"`
}

func (SSLCertificateDomain) TableName() string { return "ssl_certificate_domains" }

type SSLCertificateVersion struct {
	ID                uint       `json:"id" gorm:"primaryKey"`
	CertificateID     uint       `json:"certificateId" gorm:"index;not null"`
	Version           int        `json:"version" gorm:"not null"`
	CertificatePEM    string     `json:"-" gorm:"type:longtext;not null"`
	PrivateKeyCipher  string     `json:"-" gorm:"type:longtext;not null"`
	CertificateChain  string     `json:"-" gorm:"type:longtext"`
	FingerprintSHA256 string     `json:"fingerprintSha256" gorm:"size:128"`
	NotBefore         *time.Time `json:"notBefore"`
	NotAfter          *time.Time `json:"notAfter"`
	CreatedAt         time.Time  `json:"createdAt"`
}

func (SSLCertificateVersion) TableName() string { return "ssl_certificate_versions" }

type SSLCertificateTask struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	CertificateID uint       `json:"certificateId" gorm:"index;not null"`
	ActiveKey     *string    `json:"-" gorm:"size:191;uniqueIndex"`
	AdminID       uint       `json:"adminId" gorm:"index"`
	Username      string     `json:"username" gorm:"size:64"`
	IPAddress     string     `json:"ipAddress" gorm:"size:64"`
	TaskType      string     `json:"taskType" gorm:"size:16;index;not null"`
	Status        string     `json:"status" gorm:"size:16;index;not null"`
	Provider      string     `json:"provider" gorm:"size:32;index"`
	Stage         string     `json:"stage" gorm:"size:64"`
	Progress      int        `json:"progress" gorm:"default:0;not null"`
	ErrorMessage  string     `json:"errorMessage" gorm:"size:2000"`
	StartedAt     *time.Time `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt"`
	CreatedAt     time.Time  `json:"createdAt" gorm:"index"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

func (SSLCertificateTask) TableName() string { return "ssl_certificate_tasks" }

type SSLCertificateAuditLog struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	CertificateID uint      `json:"certificateId" gorm:"index"`
	AdminID       uint      `json:"adminId" gorm:"index"`
	Username      string    `json:"username" gorm:"size:64"`
	IPAddress     string    `json:"ipAddress" gorm:"size:64"`
	Action        string    `json:"action" gorm:"size:64;index;not null"`
	MainDomain    string    `json:"mainDomain" gorm:"size:255;index"`
	Domains       string    `json:"domains" gorm:"size:1000"`
	Provider      string    `json:"provider" gorm:"size:32;index"`
	AccountID     uint      `json:"accountId" gorm:"index"`
	Success       bool      `json:"success" gorm:"index"`
	Error         string    `json:"error" gorm:"size:1000"`
	CreatedAt     time.Time `json:"createdAt" gorm:"index"`
}

func (SSLCertificateAuditLog) TableName() string { return "ssl_certificate_audit_logs" }
