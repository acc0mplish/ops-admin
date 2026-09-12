package service

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
	"ops-admin/backend/internal/domain/dnsserver"
	"ops-admin/backend/internal/infra/inventory"
	"ops-admin/backend/model"
	"ops-admin/backend/util"
)

type Service struct {
	db                   *gorm.DB
	opsScheduler         *OpsScheduler
	opsSchedulerOnce     sync.Once
	monitorScheduler     *MonitorScheduler
	monitorSchedulerOnce sync.Once
	dbBackupScheduler    *DatabaseBackupScheduler
	dbBackupOnce         sync.Once
	finOpsScheduler      *FinOpsScheduler
	finOpsSchedulerOnce  sync.Once
	notifyDispatcherOnce sync.Once
	notifyConcurrency    chan struct{}
	dnsManager           *dnsserver.Manager
	certificateConfig    CertificateRuntimeConfig
	certificateOnce      sync.Once
	monitorNotifyMu      sync.Mutex
	// k8sState groups the Kubernetes client state (gateway SSH pool, overview
	// cache, singleflight) absorbed from five Service fields in Phase D2.
	// Held by pointer only — the embedded mutexes and singleflight.Group must
	// not be copied (plan §12 #12, claim C13).
	k8sState *k8sClientState
}

func New(db *gorm.DB) *Service {
	svc := &Service{
		db:                db,
		certificateConfig: defaultCertificateRuntimeConfig(),
		k8sState: &k8sClientState{
			gatewaySSHClients: make(map[uint]*ssh.Client),
			k8sOverviewCache:  make(map[uint]k8sOverviewCacheEntry),
		},
	}
	svc.dnsManager = dnsserver.NewManager(db)
	svc.ensureDefaultEnvironments()
	svc.initOpsScheduler()
	svc.initMonitorScheduler()
	svc.initDatabaseBackupScheduler()
	svc.initFinOpsScheduler()
	svc.initNotifyDispatcher()
	return svc
}

type IDPayload struct {
	ID uint `json:"id"`
}

type BatchIDPayload struct {
	IDs []uint `json:"ids"`
}

func Trimmed(s string) string {
	return strings.TrimSpace(s)
}

func shortenText(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func optionalUint(value uint) *uint {
	if value == 0 {
		return nil
	}
	return &value
}

func lastField(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}

type cloudInstance struct {
	InstanceID string
	HostName   string
	PrivateIP  string
	PublicIP   string
	CPU        string
	Memory     string
	Disk       string
	OS         string
	Region     string
	SSHUser    string
	SSHPort    int
}

// CaptureCloudInstancesForCompare is the §15 cloud pair's legacy side (plan
// phase4 M10 — J7): the SAME path the v1 cloud sync drives
// (fetchCloudInstances), wrapped read-only for the compare CLI. No v1 row is
// written and nothing is cached — a single-shot compare process therefore
// always captures fresh (the J4 fresh-cache posture; the k8s pair gets the
// same guarantee from an uncached Service). Credentials resolved here stay in
// process memory: the engine consumes the neutral capture only (보존 제약 #7).
func (s *Service) CaptureCloudInstancesForCompare(accountID uint) (inventory.CloudLegacyCapture, error) {
	capture := inventory.CloudLegacyCapture{CapturedAt: time.Now()}
	var account model.AssetCloudAccount
	if err := s.db.Where("id = ? AND status = ?", accountID, 1).First(&account).Error; err != nil {
		return capture, fmt.Errorf("cloud account %d not found or disabled", accountID)
	}
	accessKey, err := decryptForCompare(account.AccessKey)
	if err != nil {
		return capture, err
	}
	secretKey, err := decryptForCompare(account.SecretKey)
	if err != nil {
		return capture, err
	}
	provider := strings.ToLower(Trimmed(account.Provider))
	region := strings.Join(normalizeCloudRegions(account.Regions, account.Region), ",")
	instances, err := fetchCloudInstances(provider, accessKey, secretKey, region)
	if err != nil {
		return capture, err
	}
	for _, item := range instances {
		capture.Instances = append(capture.Instances, inventory.CloudLegacyVM{
			InstanceID: item.InstanceID, HostName: item.HostName,
			PrivateIP: item.PrivateIP, PublicIP: item.PublicIP,
			CPU: item.CPU, Memory: item.Memory, Disk: item.Disk,
			OS: item.OS, Region: item.Region,
			SSHUser: item.SSHUser, SSHPort: item.SSHPort,
		})
	}
	return capture, nil
}

// decryptForCompare opens a v2-sealed v1 credential column and keeps P-class
// plaintext as-is — the same decrypt-if-envelope-else-plaintext posture the
// backfill applies to the same columns (§4.3).
func decryptForCompare(value string) (string, error) {
	value = Trimmed(value)
	if value == "" || !util.IsV2Envelope(value) {
		return value, nil
	}
	return util.DecryptSecretV2(value)
}

func fetchCloudInstances(provider, accessKey, secretKey, region string) ([]cloudInstance, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "tencent", "tencentcloud":
		service := util.NewTencentCloudService(accessKey, secretKey)
		instances, err := service.GetInstances(normalizeCloudRegions(nil, region))
		if err != nil {
			return nil, err
		}
		result := make([]cloudInstance, 0, len(instances))
		for _, item := range instances {
			result = append(result, cloudInstance{
				InstanceID: item.InstanceID,
				HostName:   firstNonEmpty(item.HostName, item.InstanceID),
				PrivateIP:  item.PrivateIP,
				PublicIP:   item.PublicIP,
				CPU:        fmt.Sprintf("%d cores", item.CPU),
				Memory:     fmt.Sprintf("%dGB", item.Memory/1024),
				Disk:       fmt.Sprintf("%dGB", item.Disk),
				OS:         item.OS,
				Region:     item.Region,
				SSHUser:    "root",
				SSHPort:    22,
			})
		}
		return result, nil
	case "aliyun", "alicloud":
		return fetchAliyunCloudInstances(accessKey, secretKey, region)
	default:
		return nil, fmt.Errorf("unsupported cloud provider: %s", provider)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func defaultSSHPort(port int) int {
	if port > 0 {
		return port
	}
	return 22
}
