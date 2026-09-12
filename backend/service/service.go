package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
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

func (s *Service) newSSHClient(host model.AssetHost) (*ssh.Client, error) {
	return s.newSSHClientWithTimeout(host, 5*time.Second)
}

func (s *Service) newSSHClientWithTimeout(host model.AssetHost, timeout time.Duration) (*ssh.Client, error) {
	if host.SSHIP == "" {
		return nil, errors.New("host has no SSH address configured")
	}
	if host.SSHUser == "" {
		return nil, errors.New("host has no SSH user configured")
	}
	authMethod, err := credentialAuthMethod(host.Credential)
	if err != nil {
		return nil, err
	}
	if timeout <= 0 {
		return nil, errors.New("host synchronization timed out")
	}
	config := &ssh.ClientConfig{
		User:            host.SSHUser,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}
	address := net.JoinHostPort(host.SSHIP, strconv.Itoa(host.SSHPort))
	if normalizeConnectionMode(host.ConnectionMode) == "gateway" && host.GatewayID != nil && *host.GatewayID > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		conn, cleanup, err := s.dialThroughGateway(ctx, *host.GatewayID, "tcp", address)
		cancel()
		if err != nil {
			return nil, err
		}
		_ = conn.SetDeadline(time.Now().Add(timeout))
		clientConn, chans, reqs, err := ssh.NewClientConn(conn, address, config)
		if err != nil {
			_ = conn.Close()
			cleanup()
			return nil, err
		}
		_ = conn.SetDeadline(time.Time{})
		client := ssh.NewClient(clientConn, chans, reqs)
		go func() {
			_ = clientConn.Wait()
			cleanup()
		}()
		return client, nil
	}
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))
	clientConn, chans, reqs, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return ssh.NewClient(clientConn, chans, reqs), nil
}

func credentialAuthMethod(credential model.AssetCredential) (ssh.AuthMethod, error) {
	switch normalizedAuthType(credential.AuthType) {
	case "key":
		var signer ssh.Signer
		var err error
		if strings.TrimSpace(credential.Passphrase) != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(credential.PrivateKey), []byte(credential.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(credential.PrivateKey))
		}
		if err != nil {
			return nil, err
		}
		return ssh.PublicKeys(signer), nil
	default:
		if credential.Password == "" {
			return nil, errors.New("password credential is empty")
		}
		return ssh.Password(credential.Password), nil
	}
}

func remainingTimeout(deadline time.Time, maximum time.Duration) time.Duration {
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return 0
	}
	if remaining < maximum {
		return remaining
	}
	return maximum
}

func collectHostInfo(client *ssh.Client, deadline time.Time) (map[string]string, bool) {
	result := map[string]string{}
	commands := map[string]string{
		"os":         ". /etc/os-release 2>/dev/null && printf '%s' \"$PRETTY_NAME\" || lsb_release -ds 2>/dev/null || uname -sr 2>/dev/null || true",
		"arch":       "uname -m 2>/dev/null || true",
		"cpu":        "nproc 2>/dev/null || grep -c processor /proc/cpuinfo 2>/dev/null || true",
		"memory":     "free -m 2>/dev/null | awk '/Mem:/ {printf \"%dG\", int(($2 + 1023) / 1024)}' || true",
		"disk":       "df -h / 2>/dev/null | awk 'NR==2 {print $2}' || true",
		"private_ip": "hostname -I 2>/dev/null | awk '{print $1}' || true",
		"public_ip":  "curl -s --max-time 2 ifconfig.me 2>/dev/null || wget -qO- -T 2 ifconfig.me 2>/dev/null || true",
	}
	for field, command := range commands {
		timeout := remainingTimeout(deadline, 5*time.Second)
		if timeout <= 0 {
			return result, true
		}
		value, timedOut := runSSHCommandWithTimeout(client, command, timeout)
		if timedOut {
			return result, true
		}
		if field == "cpu" && value != "" {
			value = value + " cores"
		}
		result[field] = value
	}
	return result, false
}

func runSSHCommand(client *ssh.Client, command string) string {
	output, _ := runSSHCommandWithTimeout(client, command, 5*time.Second)
	return output
}

func runSSHCommandWithTimeout(client *ssh.Client, command string, timeout time.Duration) (string, bool) {
	session, err := client.NewSession()
	if err != nil {
		return "", false
	}
	defer session.Close()
	type commandResult struct {
		output []byte
		err    error
	}
	done := make(chan commandResult, 1)
	go func() {
		output, runErr := session.CombinedOutput(command)
		done <- commandResult{output: output, err: runErr}
	}()
	select {
	case result := <-done:
		if result.err != nil {
			return "", false
		}
		return strings.TrimSpace(string(result.output)), false
	case <-time.After(timeout):
		_ = session.Close()
		return "", true
	}
}

func lastField(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}

func formatConfig(host model.AssetHost) string {
	parts := make([]string, 0, 3)
	if host.CPU != "" {
		parts = append(parts, host.CPU)
	}
	if host.Memory != "" {
		parts = append(parts, host.Memory)
	}
	if host.Disk != "" {
		parts = append(parts, host.Disk)
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " / ")
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
