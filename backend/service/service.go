package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
	"ops-admin/backend/internal/domain/dnsserver"
	"ops-admin/backend/internal/infra/inventory"
	infraModel "ops-admin/backend/internal/infra/model"
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

type AssetCloudSyncPayload struct {
	GroupID            uint     `json:"groupId"`
	Provider           string   `json:"provider"`
	CredentialID       uint     `json:"credentialId"`
	ConnectionMode     string   `json:"connectionMode"`
	GatewayID          uint     `json:"gatewayId"`
	Environment        string   `json:"environment"`
	Regions            []string `json:"regions"`
	Region             string   `json:"region"`
	CloudAccountID     uint     `json:"cloudAccountId"`
	UseExistingAccount bool     `json:"useExistingAccount"`
	AccessKey          string   `json:"accessKey"`
	SecretKey          string   `json:"secretKey"`
	AccountName        string   `json:"accountName"`
	SaveAccount        bool     `json:"saveAccount"`
}

type AssetCredentialPayload struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	AuthType    string `json:"authType"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	PrivateKey  string `json:"privateKey"`
	Passphrase  string `json:"passphrase"`
	Status      int    `json:"status"`
	Description string `json:"description"`
}

type AssetCloudAccountPayload struct {
	ID          uint     `json:"id"`
	Name        string   `json:"name"`
	Provider    string   `json:"provider"`
	AccessKey   string   `json:"accessKey"`
	SecretKey   string   `json:"secretKey"`
	Regions     []string `json:"regions"`
	Region      string   `json:"region"`
	Status      int      `json:"status"`
	Description string   `json:"description"`
}

type AssetDatabasePayload struct {
	ID             uint     `json:"id"`
	Name           string   `json:"name"`
	DBType         string   `json:"dbType"`
	Host           string   `json:"host"`
	Port           int      `json:"port"`
	Username       string   `json:"username"`
	Password       string   `json:"password"`
	ConnectionMode string   `json:"connectionMode"`
	GatewayID      uint     `json:"gatewayId"`
	DBName         string   `json:"dbName"`
	Charset        string   `json:"charset"`
	Env            string   `json:"env"`
	Tags           []string `json:"tags"`
	AccessMode     string   `json:"accessMode"`
	MonitorEnabled bool     `json:"monitorEnabled"`
	Status         int      `json:"status"`
	Description    string   `json:"description"`
	Operator       string   `json:"-"`
}

func (s *Service) ListAssetCredentials(pageNum, pageSize int, keyword string, authType string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.AssetCredential{})
	if keyword != "" {
		query = query.Where("name like ? or username like ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if authType != "" {
		query = query.Where("auth_type = ?", authType)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.AssetCredential
	if err := query.Order("id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	for i := range list {
		maskCredential(&list[i])
	}
	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func (s *Service) ListAssetCredentialOptions() ([]model.AssetCredential, error) {
	var list []model.AssetCredential
	err := s.db.Where("status = ?", 1).Order("id desc").Find(&list).Error
	for i := range list {
		maskCredential(&list[i])
	}
	return list, err
}

func (s *Service) GetAssetCredential(id uint) (*model.AssetCredential, error) {
	var item model.AssetCredential
	return &item, s.db.First(&item, id).Error
}

func (s *Service) CreateAssetCredential(payload AssetCredentialPayload) error {
	item := model.AssetCredential{
		Name:        Trimmed(payload.Name),
		AuthType:    normalizedAuthType(payload.AuthType),
		Username:    Trimmed(payload.Username),
		Password:    payload.Password,
		PrivateKey:  payload.PrivateKey,
		Passphrase:  payload.Passphrase,
		Status:      payload.Status,
		Description: Trimmed(payload.Description),
	}
	if item.Status == 0 {
		item.Status = 1
	}
	return s.db.Create(&item).Error
}

func (s *Service) UpdateAssetCredential(payload AssetCredentialPayload) error {
	updates := map[string]any{
		"name":        Trimmed(payload.Name),
		"auth_type":   normalizedAuthType(payload.AuthType),
		"username":    Trimmed(payload.Username),
		"status":      payload.Status,
		"description": Trimmed(payload.Description),
	}
	if payload.Password != "" {
		updates["password"] = payload.Password
	}
	if payload.PrivateKey != "" {
		updates["private_key"] = payload.PrivateKey
	}
	if payload.Passphrase != "" {
		updates["passphrase"] = payload.Passphrase
	}
	return s.db.Model(&model.AssetCredential{}).Where("id = ?", payload.ID).Updates(updates).Error
}

func (s *Service) DeleteAssetCredential(id uint) error {
	var count int64
	s.db.Model(&model.AssetHost{}).Where("credential_id = ?", id).Count(&count)
	if count > 0 {
		return errors.New("credential is referenced by hosts and cannot be deleted")
	}
	return s.db.Delete(&model.AssetCredential{}, id).Error
}

func (s *Service) ListAssetCloudAccounts(pageNum, pageSize int, keyword string, provider string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.AssetCloudAccount{})
	if keyword != "" {
		query = query.Where("name like ? or access_key like ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if provider != "" {
		query = query.Where("provider = ?", provider)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.AssetCloudAccount
	if err := query.Order("id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	for i := range list {
		hydrateCloudAccountRegions(&list[i])
		list[i].SecretKey = maskedSecret(list[i].SecretKey)
	}
	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func (s *Service) ListAssetCloudAccountOptions() ([]model.AssetCloudAccount, error) {
	var list []model.AssetCloudAccount
	err := s.db.Where("status = ?", 1).Order("id desc").Find(&list).Error
	for i := range list {
		hydrateCloudAccountRegions(&list[i])
		list[i].SecretKey = maskedSecret(list[i].SecretKey)
	}
	return list, err
}

func (s *Service) GetAssetCloudAccount(id uint) (*model.AssetCloudAccount, error) {
	var item model.AssetCloudAccount
	if err := s.db.First(&item, id).Error; err != nil {
		return &item, err
	}
	hydrateCloudAccountRegions(&item)
	return &item, nil
}

func normalizeCloudRegions(regions []string, legacy string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(regions)+1)
	appendRegion := func(value string) {
		for _, region := range strings.FieldsFunc(value, func(r rune) bool {
			return r == ',' || r == '，' || r == ';' || r == '；' || r == '\n' || r == '\r' || r == '\t' || r == ' '
		}) {
			region = strings.TrimSpace(region)
			if region == "" {
				continue
			}
			if _, exists := seen[region]; exists {
				continue
			}
			seen[region] = struct{}{}
			result = append(result, region)
		}
	}
	for _, region := range regions {
		appendRegion(region)
	}
	appendRegion(legacy)
	return result
}

func cloudRegionsJSON(regions []string) string {
	encoded, err := json.Marshal(regions)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

func hydrateCloudAccountRegions(account *model.AssetCloudAccount) {
	account.Regions = normalizeCloudRegions(account.Regions, account.Region)
	if account.Region == "" {
		account.Region = strings.Join(account.Regions, ",")
	}
}

func (s *Service) CreateAssetCloudAccount(payload AssetCloudAccountPayload) error {
	regions := normalizeCloudRegions(payload.Regions, payload.Region)
	if len(regions) == 0 {
		return errors.New("at least one sync region is required")
	}
	item := model.AssetCloudAccount{
		Name:        Trimmed(payload.Name),
		Provider:    Trimmed(payload.Provider),
		AccessKey:   Trimmed(payload.AccessKey),
		SecretKey:   payload.SecretKey,
		Regions:     regions,
		Region:      strings.Join(regions, ","),
		Status:      payload.Status,
		Description: Trimmed(payload.Description),
	}
	if item.Status == 0 {
		item.Status = 1
	}
	return s.db.Create(&item).Error
}

func (s *Service) UpdateAssetCloudAccount(payload AssetCloudAccountPayload) error {
	regions := normalizeCloudRegions(payload.Regions, payload.Region)
	if len(regions) == 0 {
		return errors.New("at least one sync region is required")
	}
	var existing model.AssetCloudAccount
	if err := s.db.Select("id", "access_key", "secret_key").First(&existing, payload.ID).Error; err != nil {
		return err
	}
	// J5-3 (plan phase4 r2): a rotated v1 credential makes the backfilled V2
	// inventory binding a lie — the broker would serve the OLD material.
	// Detect the rotation and unlink the binding in the same transaction as
	// the save; `sync-inventory` re-creates the chain from the new material.
	credentialRotated := Trimmed(payload.AccessKey) != existing.AccessKey ||
		(payload.SecretKey != "" && payload.SecretKey != existing.SecretKey)
	updates := map[string]any{
		"name":        Trimmed(payload.Name),
		"provider":    Trimmed(payload.Provider),
		"access_key":  Trimmed(payload.AccessKey),
		"regions":     cloudRegionsJSON(regions),
		"region":      strings.Join(regions, ","),
		"status":      payload.Status,
		"description": Trimmed(payload.Description),
	}
	if payload.SecretKey != "" {
		updates["secret_key"] = payload.SecretKey
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.AssetCloudAccount{}).Where("id = ?", payload.ID).Updates(updates).Error; err != nil {
			return err
		}
		if credentialRotated {
			if err := tx.Where("purpose = ?", "inventory").
				Where("provider_connection_id IN (?)",
					tx.Table("provider_connection").Select("id").
						Where("source_model = ? AND source_id = ?", "asset_cloud_account", payload.ID),
				).Delete(&infraModel.ProviderCredentialBinding{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) DeleteAssetCloudAccount(id uint) error {
	var count int64
	s.db.Model(&model.AssetHost{}).Where("cloud_account_id = ?", id).Count(&count)
	if count > 0 {
		return errors.New("cloud account is referenced by hosts and cannot be deleted")
	}
	return s.db.Delete(&model.AssetCloudAccount{}, id).Error
}

func (s *Service) GetAssetOverview() (map[string]any, error) {
	var (
		hostTotal           int64
		hostOnline          int64
		hostOffline         int64
		hostAuthFailed      int64
		groupTotal          int64
		credentialTotal     int64
		credentialEnabled   int64
		cloudAccountTotal   int64
		cloudAccountEnabled int64
		databaseTotal       int64
		databaseHealthy     int64
		k8sClusterTotal     int64
		k8sClusterOnline    int64
		k8sNodeTotal        int64
		incompleteHosts     int64
		incompleteDatabases int64
		incompleteClusters  int64
	)

	if err := s.db.Model(&model.AssetHost{}).Count(&hostTotal).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.AssetHost{}).Where("alive_status = ?", 1).Count(&hostOnline).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.AssetHost{}).Where("alive_status = ?", 2).Count(&hostOffline).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.AssetHost{}).Where("auth_status = ?", 2).Count(&hostAuthFailed).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.AssetHostGroup{}).Count(&groupTotal).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.AssetCredential{}).Count(&credentialTotal).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.AssetCredential{}).Where("status = ?", 1).Count(&credentialEnabled).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.AssetCloudAccount{}).Count(&cloudAccountTotal).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.AssetCloudAccount{}).Where("status = ?", 1).Count(&cloudAccountEnabled).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.AssetDatabase{}).Count(&databaseTotal).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.AssetDatabase{}).Where("connect_status = ?", 1).Count(&databaseHealthy).Error; err != nil {
		return nil, err
	}
	// Kubernetes cluster stats resolve through the V2 connection source
	// (I-a S6 — the k8s_cluster aggregates are retired; §J5). "online" maps
	// to the connection status the V2 chains record ("active").
	k8sConns, err := s.k8sClusterConnections()
	if err != nil {
		return nil, err
	}
	k8sClusterTotal = int64(len(k8sConns))
	for i := range k8sConns {
		if k8sConns[i].Status == "active" {
			k8sClusterOnline++
		}
		if nodeCount, ok := k8sConfigUintValue(&k8sConns[i], "node_count"); ok {
			k8sNodeTotal += int64(nodeCount)
		}
		if k8sConfigString(&k8sConns[i], "env") == "" {
			incompleteClusters++
		}
	}
	if err := s.db.Model(&model.AssetHost{}).Where("TRIM(COALESCE(environment, '')) = ''").Count(&incompleteHosts).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.AssetDatabase{}).Where("TRIM(COALESCE(env, '')) = ''").Count(&incompleteDatabases).Error; err != nil {
		return nil, err
	}

	type distributionRow struct {
		Name  string `json:"name" gorm:"column:name"`
		Count int64  `json:"count" gorm:"column:count"`
	}

	var providerRows []distributionRow
	if err := s.db.Model(&model.AssetHost{}).
		Select("COALESCE(NULLIF(provider, ''), 'On-premises') as name, COUNT(*) as count").
		Group("COALESCE(NULLIF(provider, ''), 'On-premises')").
		Order("count DESC").
		Limit(8).
		Scan(&providerRows).Error; err != nil {
		return nil, err
	}

	var environmentRows []distributionRow
	if err := s.db.Model(&model.AssetHost{}).
		Where("NULLIF(environment, '') IS NOT NULL").
		Select("environment as name, COUNT(*) as count").
		Group("environment").
		Order("count DESC").
		Limit(8).
		Scan(&environmentRows).Error; err != nil {
		return nil, err
	}

	var groups []model.AssetHostGroup
	if err := s.db.Order("sort ASC, updated_at DESC").Find(&groups).Error; err != nil {
		return nil, err
	}
	countMap, err := s.assetHostGroupHostCountMap()
	if err != nil {
		return nil, err
	}

	groupItems := make([]map[string]any, 0, len(groups))
	for _, item := range groups {
		groupItems = append(groupItems, map[string]any{
			"id":        item.ID,
			"name":      item.Name,
			"code":      item.Code,
			"status":    item.Status,
			"hostCount": countMap[item.ID],
			"updatedAt": item.UpdatedAt,
		})
	}
	sort.Slice(groupItems, func(i, j int) bool {
		return groupItems[i]["hostCount"].(int64) > groupItems[j]["hostCount"].(int64)
	})
	if len(groupItems) > 6 {
		groupItems = groupItems[:6]
	}

	var recentHosts []model.AssetHost
	if err := s.db.Preload("Group").Preload("HostGroups").Preload("Credential").Preload("CloudAccount").
		Order("updated_at DESC").
		Limit(6).
		Find(&recentHosts).Error; err != nil {
		return nil, err
	}
	ensureHostGroupFallbackList(recentHosts)

	hostItems := make([]map[string]any, 0, len(recentHosts))
	for _, item := range recentHosts {
		groupNames := make([]string, 0, len(item.HostGroups))
		for _, group := range item.HostGroups {
			groupNames = append(groupNames, group.Name)
		}
		hostItems = append(hostItems, map[string]any{
			"id":          item.ID,
			"hostName":    item.HostName,
			"sshIp":       item.SSHIP,
			"publicIp":    item.PublicIP,
			"privateIp":   item.PrivateIP,
			"provider":    item.Provider,
			"environment": item.Environment,
			"aliveStatus": item.AliveStatus,
			"authStatus":  item.AuthStatus,
			"groupNames":  groupNames,
			"updatedAt":   item.UpdatedAt,
		})
	}

	var recentDatabases []model.AssetDatabase
	if err := s.db.Order("updated_at DESC").Limit(6).Find(&recentDatabases).Error; err != nil {
		return nil, err
	}
	databaseItems := make([]map[string]any, 0, len(recentDatabases))
	for _, item := range recentDatabases {
		databaseItems = append(databaseItems, map[string]any{
			"id":            item.ID,
			"name":          item.Name,
			"dbType":        item.DBType,
			"host":          item.Host,
			"port":          item.Port,
			"dbName":        item.DBName,
			"version":       item.Version,
			"status":        item.Status,
			"connectStatus": item.ConnectStatus,
			"updatedAt":     item.UpdatedAt,
		})
	}

	// Recent clusters likewise ride the V2 connection source (I-a S6).
	recentClusters := make([]infraModel.ProviderConnection, len(k8sConns))
	copy(recentClusters, k8sConns)
	sort.Slice(recentClusters, func(i, j int) bool {
		return recentClusters[i].UpdatedAt.After(recentClusters[j].UpdatedAt)
	})
	if len(recentClusters) > 6 {
		recentClusters = recentClusters[:6]
	}
	clusterItems := make([]map[string]any, 0, len(recentClusters))
	for i := range recentClusters {
		item := &recentClusters[i]
		nodeCount := 0
		if parsed, ok := k8sConfigUintValue(item, "node_count"); ok {
			nodeCount = int(parsed)
		}
		clusterItems = append(clusterItems, map[string]any{
			"id":        item.SourceID,
			"name":      item.Name,
			"status":    item.Status,
			"apiServer": item.Endpoint,
			"version":   item.Version,
			"nodeCount": nodeCount,
			"updatedAt": item.UpdatedAt,
		})
	}

	return map[string]any{
		"summary": map[string]any{
			"hostTotal":           hostTotal,
			"hostOnline":          hostOnline,
			"hostOffline":         hostOffline,
			"groupTotal":          groupTotal,
			"credentialTotal":     credentialTotal,
			"credentialEnabled":   credentialEnabled,
			"cloudAccountTotal":   cloudAccountTotal,
			"cloudAccountEnabled": cloudAccountEnabled,
			"databaseTotal":       databaseTotal,
			"databaseHealthy":     databaseHealthy,
			"k8sClusterTotal":     k8sClusterTotal,
			"k8sClusterOnline":    k8sClusterOnline,
			"k8sNodeTotal":        k8sNodeTotal,
		},
		"health": map[string]any{
			"offlineHosts":        hostOffline,
			"authFailedHosts":     hostAuthFailed,
			"healthyDatabases":    databaseHealthy,
			"abnormalDatabases":   databaseTotal - databaseHealthy,
			"abnormalClusters":    k8sClusterTotal - k8sClusterOnline,
			"incompleteAssets":    incompleteHosts + incompleteDatabases + incompleteClusters,
			"incompleteHosts":     incompleteHosts,
			"incompleteDatabases": incompleteDatabases,
			"incompleteClusters":  incompleteClusters,
		},
		"distributions": map[string]any{
			"providers":    providerRows,
			"environments": environmentRows,
		},
		"topGroups":       groupItems,
		"recentHosts":     hostItems,
		"recentDatabases": databaseItems,
		"recentClusters":  clusterItems,
	}, nil
}

func (s *Service) paginateLogs(entity any, pageNum, pageSize int, field string, keyword string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(entity)
	if keyword != "" {
		query = query.Where(field+" like ?", "%"+keyword+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	switch entity.(type) {
	case *model.LoginLog:
		var list []model.LoginLog
		if err := query.Order("id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
			return nil, err
		}
		return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
	case *model.OperationLog:
		var list []model.OperationLog
		if err := query.Order("id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
			return nil, err
		}
		return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
	default:
		return nil, errors.New("unsupported entity")
	}
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

func normalizedAuthType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "key", "private_key", "\u5bc6\u94a5\u8ba4\u8bc1":
		return "key"
	default:
		return "password"
	}
}

func maskCredential(item *model.AssetCredential) {
	item.Password = maskedSecret(item.Password)
	item.PrivateKey = maskedSecret(item.PrivateKey)
	item.Passphrase = maskedSecret(item.Passphrase)
}

func maskedSecret(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return "******"
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
