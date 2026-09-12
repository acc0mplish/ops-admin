package service

import (
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
	"ops-admin/backend/internal/domain/dnsserver"
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
