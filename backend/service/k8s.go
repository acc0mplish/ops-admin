package service

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"ops-admin/backend/model"

	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func (s *Service) ListK8sClusters() ([]model.K8sClusterView, error) {
	var list []model.K8sCluster
	if err := s.db.Preload("Gateway").Preload("MonitorDatasource").Order("id asc").Find(&list).Error; err != nil {
		return nil, err
	}

	result := make([]model.K8sClusterView, 0, len(list))
	for _, item := range list {
		result = append(result, toK8sClusterView(item))
	}
	return result, nil
}

func (s *Service) GetK8sCluster(id uint) (model.K8sCluster, error) {
	var cluster model.K8sCluster
	if err := s.db.Preload("Gateway").Preload("MonitorDatasource").First(&cluster, id).Error; err != nil {
		return cluster, err
	}
	return cluster, nil
}

func (s *Service) CreateK8sCluster(payload model.K8sClusterPayload) (model.K8sCluster, error) {
	monitorDatasourceID, err := s.resolveK8sMonitorDatasource(payload.MonitorDatasourceID)
	if err != nil {
		return model.K8sCluster{}, err
	}
	cluster := model.K8sCluster{
		Name:                strings.TrimSpace(payload.Name),
		Description:         strings.TrimSpace(payload.Description),
		KubeConfig:          strings.TrimSpace(payload.KubeConfig),
		Env:                 normalizeEnvCode(payload.Env),
		Tags:                normalizeAssetTags(payload.Tags),
		ConnectionMode:      normalizeConnectionMode(payload.ConnectionMode),
		GatewayID:           optionalGatewayID(payload.ConnectionMode, payload.GatewayID),
		MonitorDatasourceID: monitorDatasourceID,
	}
	if err := validateK8sClusterPayload(cluster); err != nil {
		return cluster, err
	}

	var count int64
	if err := s.db.Model(&model.K8sCluster{}).Where("name = ?", cluster.Name).Count(&count).Error; err != nil {
		return cluster, err
	}
	if count > 0 {
		return cluster, errors.New("Kubernetes cluster name already exists")
	}

	probe, err := s.probeK8sCluster(cluster)
	if err != nil {
		return cluster, err
	}

	now := time.Now()
	cluster.APIServer = probe.APIServer
	cluster.Version = probe.Version
	cluster.NodeCount = probe.NodeCount
	cluster.Status = probe.Status
	cluster.LastSyncAt = &now

	if err := s.db.Create(&cluster).Error; err != nil {
		return cluster, err
	}
	s.recordAssetChange("k8s", cluster.ID, cluster.Name, "create", "Create Kubernetes Cluster", payload.Operator)
	return cluster, nil
}

func (s *Service) UpdateK8sCluster(payload model.K8sClusterPayload) (model.K8sCluster, error) {
	cluster, err := s.GetK8sCluster(payload.ID)
	if err != nil {
		return cluster, err
	}

	cluster.Name = strings.TrimSpace(payload.Name)
	cluster.Description = strings.TrimSpace(payload.Description)
	cluster.KubeConfig = strings.TrimSpace(payload.KubeConfig)
	cluster.Env = normalizeEnvCode(payload.Env)
	cluster.Tags = normalizeAssetTags(payload.Tags)
	cluster.ConnectionMode = normalizeConnectionMode(payload.ConnectionMode)
	cluster.GatewayID = optionalGatewayID(payload.ConnectionMode, payload.GatewayID)
	monitorDatasourceID, err := s.resolveK8sMonitorDatasource(payload.MonitorDatasourceID)
	if err != nil {
		return cluster, err
	}
	cluster.MonitorDatasourceID = monitorDatasourceID

	if err := validateK8sClusterPayload(cluster); err != nil {
		return cluster, err
	}

	var count int64
	if err := s.db.Model(&model.K8sCluster{}).Where("name = ? AND id <> ?", cluster.Name, cluster.ID).Count(&count).Error; err != nil {
		return cluster, err
	}
	if count > 0 {
		return cluster, errors.New("Kubernetes cluster name already exists")
	}

	probe, err := s.probeK8sCluster(cluster)
	if err != nil {
		return cluster, err
	}

	now := time.Now()
	cluster.APIServer = probe.APIServer
	cluster.Version = probe.Version
	cluster.NodeCount = probe.NodeCount
	cluster.Status = probe.Status
	cluster.LastSyncAt = &now

	if err := s.db.Save(&cluster).Error; err != nil {
		return cluster, err
	}
	s.recordAssetChange("k8s", cluster.ID, cluster.Name, "update", "Update Kubernetes Cluster Configuration and Validate Connection", payload.Operator)
	return cluster, nil
}

func (s *Service) DeleteK8sCluster(id uint) error {
	cluster, _ := s.GetK8sCluster(id)
	if err := s.db.Delete(&model.K8sCluster{}, id).Error; err != nil {
		return err
	}
	s.recordAssetChange("k8s", id, cluster.Name, "delete", "Delete Kubernetes Cluster", "system")
	return nil
}

const k8sOverviewCacheTTL = 15 * time.Second

func (s *Service) GetK8sClusterDetail(clusterID uint) (model.K8sClusterDetail, error) {
	if detail, ok := s.cachedK8sClusterDetail(clusterID); ok {
		return detail, nil
	}
	result, err, _ := s.k8sOverviewGroup.Do(fmt.Sprintf("cluster-overview:%d", clusterID), func() (any, error) {
		if detail, ok := s.cachedK8sClusterDetail(clusterID); ok {
			return detail, nil
		}
		detail, err := s.getK8sClusterDetailUncached(clusterID)
		if err != nil {
			return model.K8sClusterDetail{}, err
		}
		s.k8sOverviewMu.Lock()
		s.k8sOverviewCache[clusterID] = k8sOverviewCacheEntry{detail: detail, expiresAt: time.Now().Add(k8sOverviewCacheTTL)}
		s.k8sOverviewMu.Unlock()
		return detail, nil
	})
	if err != nil {
		return model.K8sClusterDetail{}, err
	}
	return result.(model.K8sClusterDetail), nil
}

func (s *Service) cachedK8sClusterDetail(clusterID uint) (model.K8sClusterDetail, bool) {
	s.k8sOverviewMu.Lock()
	defer s.k8sOverviewMu.Unlock()
	entry, ok := s.k8sOverviewCache[clusterID]
	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			delete(s.k8sOverviewCache, clusterID)
		}
		return model.K8sClusterDetail{}, false
	}
	return entry.detail, true
}

func (s *Service) invalidateK8sClusterDetailCache(clusterID uint) {
	s.k8sOverviewMu.Lock()
	delete(s.k8sOverviewCache, clusterID)
	s.k8sOverviewMu.Unlock()
}

func (s *Service) getK8sClusterDetailUncached(clusterID uint) (model.K8sClusterDetail, error) {
	cluster, err := s.GetK8sCluster(clusterID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.K8sClusterDetail{}, errors.New("k8s cluster not found")
		}
		return model.K8sClusterDetail{}, err
	}

	runtime, err := parseKubeConfig(cluster.KubeConfig)
	if err != nil {
		return model.K8sClusterDetail{}, fmt.Errorf("failed to parse kubeconfig: %w", err)
	}
	// Cluster details must use a client that matches the configured connection mode.
	// In gateway mode, the client connects to the gateway before accessing the private Kubernetes API;
	// the Ops Admin host must not dial the private kubeconfig address directly.
	client, cleanup, err := s.newK8sHTTPClientForCluster(cluster, runtime)
	if err != nil {
		return model.K8sClusterDetail{}, fmt.Errorf("failed to create Kubernetes API client: %w", err)
	}
	defer cleanup()

	data, err := fetchK8sData(client, runtime)
	if err != nil {
		route := "direct connection"
		if normalizeConnectionMode(cluster.ConnectionMode) == "gateway" {
			route = "gateway relay"
		}
		return model.K8sClusterDetail{}, fmt.Errorf("failed to retrieve Kubernetes cluster details through %s: %w", route, err)
	}

	metrics := calculateK8sAggregateMetrics(data.Nodes, data.Pods)
	detailCluster := toK8sClusterView(cluster)
	if metrics.AlertCount > 0 {
		detailCluster.Status = "warning"
		detailCluster.StatusText = k8sStatusText("warning")
	}

	namespaceCounts := buildNamespaceCounts(data)
	endpointCounts := buildEndpointCounts(data.Endpoints)
	workloads := buildWorkloadItems(data)
	sort.Slice(workloads, func(i, j int) bool {
		if workloads[i].Namespace == workloads[j].Namespace {
			return workloads[i].Name < workloads[j].Name
		}
		return workloads[i].Namespace < workloads[j].Namespace
	})

	return model.K8sClusterDetail{
		Cluster: detailCluster,
		Overview: model.K8sOverview{
			HealthScore:  calculateHealthScore(metrics.AlertCount),
			CPUUsage:     formatUsagePercent(metrics.TotalReqCPUMilli, metrics.TotalAllocCPUMilli),
			MemoryUsage:  formatUsagePercent(metrics.TotalReqMemoryBytes, metrics.TotalAllocMemoryBytes),
			PodUsage:     fmt.Sprintf("%d Pods", len(data.Pods)),
			RequestRate:  fmt.Sprintf("%d Workloads", len(workloads)),
			AlertCount:   metrics.AlertCount,
			Distribution: buildOverviewDistribution(detailCluster, data.Nodes, data.ConfigMaps),
			Certificates: buildOverviewCertificates(runtime),
		},
		Nodes:      buildNodeItems(data.Nodes, data.Pods),
		Namespaces: buildNamespaceItems(data.Namespaces, namespaceCounts),
		Pods:       buildPodItemsWithWorkloads(data),
		Workloads:  workloads,
		Network:    buildNetworkSection(data.Services, data.Ingresses, endpointCounts),
		AdvancedNetwork: buildAdvancedNetworkSection(
			data.GatewayAPIGateways,
			data.HTTPRoutes,
			data.Services,
		),
		ConfigStorage: buildConfigStorageSection(data.ConfigMaps, data.Secrets, data.PVCs, data.PVs),
	}, nil
}

func (s *Service) GetK8sNodeDetail(clusterID uint, nodeName string) (model.K8sNodeDetail, error) {
	cluster, runtime, client, err := s.k8sClientForCluster(clusterID)
	if err != nil {
		return model.K8sNodeDetail{}, err
	}

	var node kubeNode
	if err := k8sGetJSON(client, runtime, "/api/v1/nodes/"+nodeName, &node); err != nil {
		return model.K8sNodeDetail{}, errors.New(k8sClusterConnectError)
	}

	pods, err := fetchPodsForNode(client, runtime, nodeName)
	if err != nil {
		return model.K8sNodeDetail{}, errors.New(k8sClusterConnectError)
	}

	_ = cluster
	return model.K8sNodeDetail{
		Name:           node.Metadata.Name,
		Status:         nodeReadyStatus(node),
		Roles:          joinNodeRoles(node.Metadata.Labels),
		Version:        fallbackText(node.Status.NodeInfo.KubeletVersion),
		InternalIP:     firstNodeInternalIP(node),
		OS:             fallbackText(node.Status.NodeInfo.OSImage),
		Kernel:         fallbackText(node.Status.NodeInfo.KernelVersion),
		ContainerRT:    fallbackText(node.Status.NodeInfo.ContainerRuntimeVersion),
		Architecture:   fallbackText(node.Status.NodeInfo.Architecture),
		Labels:         node.Metadata.Labels,
		CapacityCPU:    fallbackText(node.Status.Capacity["cpu"]),
		CapacityMem:    fallbackText(node.Status.Capacity["memory"]),
		AllocatableCPU: fallbackText(node.Status.Allocatable["cpu"]),
		AllocatableMem: fallbackText(node.Status.Allocatable["memory"]),
		Pods:           buildPodItems(pods),
	}, nil
}

func (s *Service) GetK8sNodePods(clusterID uint, nodeName string) ([]model.K8sPodItem, error) {
	_, runtime, client, err := s.k8sClientForCluster(clusterID)
	if err != nil {
		return nil, err
	}
	pods, err := fetchPodsForNode(client, runtime, nodeName)
	if err != nil {
		return nil, errors.New(k8sClusterConnectError)
	}
	return buildPodItems(pods), nil
}

// UpdateK8sNodeLabels applies the submitted label set to a Kubernetes node.
// Existing labels not included in the set are explicitly removed via a merge patch.
func (s *Service) UpdateK8sNodeLabels(payload model.K8sNodeLabelsPayload) error {
	_, runtime, client, err := s.k8sClientForCluster(payload.ClusterID)
	if err != nil {
		return err
	}
	var node kubeNode
	if err := k8sGetJSON(client, runtime, "/api/v1/nodes/"+url.PathEscape(payload.NodeName), &node); err != nil {
		return errors.New(k8sClusterConnectError)
	}
	labelsPatch := make(map[string]any, len(node.Metadata.Labels)+len(payload.Labels))
	for key := range node.Metadata.Labels {
		if _, keep := payload.Labels[key]; !keep {
			labelsPatch[key] = nil
		}
	}
	for key, value := range payload.Labels {
		key = strings.TrimSpace(key)
		if key == "" {
			return errors.New("node label key is required")
		}
		labelsPatch[key] = strings.TrimSpace(value)
	}
	if err := k8sPatchJSON(client, runtime, "/api/v1/nodes/"+url.PathEscape(payload.NodeName), map[string]any{
		"metadata": map[string]any{"labels": labelsPatch},
	}, "application/merge-patch+json", nil); err != nil {
		return errors.New(k8sClusterConnectError)
	}
	return nil
}

func (s *Service) GetK8sPodDetail(clusterID uint, namespace string, podName string) (model.K8sPodDetail, error) {
	_, runtime, client, err := s.k8sClientForCluster(clusterID)
	if err != nil {
		return model.K8sPodDetail{}, err
	}

	var pod kubePod
	if err := k8sGetJSON(client, runtime, "/api/v1/namespaces/"+namespace+"/pods/"+podName, &pod); err != nil {
		return model.K8sPodDetail{}, errors.New(k8sClusterConnectError)
	}

	containers := make([]model.K8sContainerItem, 0, len(pod.Spec.Containers))
	statusMap := make(map[string]struct {
		ready   bool
		restart int
	}, len(pod.Status.ContainerStatuses))
	for _, status := range pod.Status.ContainerStatuses {
		statusMap[status.Name] = struct {
			ready   bool
			restart int
		}{ready: status.Ready, restart: status.RestartCount}
	}
	for _, container := range pod.Spec.Containers {
		containerStatus := statusMap[container.Name]
		containers = append(containers, model.K8sContainerItem{
			Name:            container.Name,
			Image:           container.Image,
			Ready:           containerStatus.ready,
			Restart:         containerStatus.restart,
			RequestCPU:      container.Resources.Requests["cpu"],
			LimitCPU:        container.Resources.Limits["cpu"],
			RequestMemory:   container.Resources.Requests["memory"],
			LimitMemory:     container.Resources.Limits["memory"],
			ImagePullPolicy: container.ImagePullPolicy,
			Env:             buildContainerEnvItems(container.Env),
		})
	}

	return model.K8sPodDetail{
		Name:           pod.Metadata.Name,
		Namespace:      pod.Metadata.Namespace,
		Status:         fallbackText(pod.Status.Phase),
		Node:           fallbackText(pod.Spec.NodeName),
		PodIP:          fallbackText(pod.Status.PodIP),
		HostIP:         fallbackText(pod.Status.HostIP),
		QoSClass:       fallbackText(pod.Status.QoSClass),
		ServiceAccount: fallbackText(pod.Spec.ServiceAccountName),
		Labels:         pod.Metadata.Labels,
		Containers:     containers,
		CreatedAt:      formatTimestamp(pod.Metadata.CreationTimestamp),
		YAML:           marshalK8sYAML(pod),
	}, nil
}

func (s *Service) probeK8sCluster(cluster model.K8sCluster) (k8sClusterProbe, error) {
	config, cleanup, err := s.k8sRESTConfigForCluster(cluster)
	if err != nil {
		return k8sClusterProbe{}, fmt.Errorf("failed to parse cluster configuration: %w", err)
	}
	defer cleanup()
	config.Timeout = 8 * time.Second

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return k8sClusterProbe{}, fmt.Errorf("failed to initialize Kubernetes client: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	version, err := clientset.Discovery().ServerVersion()
	if err != nil {
		if normalizeConnectionMode(cluster.ConnectionMode) == "gateway" {
			return k8sClusterProbe{}, fmt.Errorf("failed to connect to API Server through gateway (%s): %w", config.Host, err)
		}
		return k8sClusterProbe{}, fmt.Errorf("failed to connect to API Server (%s): %w", config.Host, err)
	}
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return k8sClusterProbe{}, fmt.Errorf("connection succeeded but node listing failed; verify nodes/list permission: %w", err)
	}

	return k8sClusterProbe{
		APIServer: config.Host,
		Version:   version.GitVersion,
		NodeCount: len(nodes.Items),
		Status:    "running",
	}, nil
}

func parseKubeConfig(content string) (kubeClusterRuntime, error) {
	var cfg kubeConfig
	if err := yaml.Unmarshal([]byte(content), &cfg); err != nil {
		return kubeClusterRuntime{}, err
	}

	contextName := strings.TrimSpace(cfg.CurrentContext)
	if contextName == "" && len(cfg.Contexts) > 0 {
		contextName = cfg.Contexts[0].Name
	}
	if contextName == "" {
		return kubeClusterRuntime{}, errors.New("missing context")
	}

	var clusterName string
	var userName string
	for i := range cfg.Contexts {
		if cfg.Contexts[i].Name == contextName {
			clusterName = strings.TrimSpace(cfg.Contexts[i].Context.Cluster)
			userName = strings.TrimSpace(cfg.Contexts[i].Context.User)
			break
		}
	}
	if clusterName == "" {
		return kubeClusterRuntime{}, errors.New("cluster not found")
	}

	runtime := kubeClusterRuntime{}
	for i := range cfg.Clusters {
		if cfg.Clusters[i].Name == clusterName {
			runtime.Server = strings.TrimSpace(cfg.Clusters[i].Cluster.Server)
			runtime.InsecureSkipTLSVerify = cfg.Clusters[i].Cluster.InsecureSkipTLSVerify
			runtime.CertificateAuthority = strings.TrimSpace(cfg.Clusters[i].Cluster.CertificateAuthorityData)
			break
		}
	}
	if runtime.Server == "" {
		return kubeClusterRuntime{}, errors.New("server not found")
	}

	for i := range cfg.Users {
		if cfg.Users[i].Name == userName {
			runtime.Token = strings.TrimSpace(cfg.Users[i].User.Token)
			runtime.Username = strings.TrimSpace(cfg.Users[i].User.Username)
			runtime.Password = strings.TrimSpace(cfg.Users[i].User.Password)
			runtime.ClientCertificateData = strings.TrimSpace(cfg.Users[i].User.ClientCertificateData)
			runtime.ClientKeyData = strings.TrimSpace(cfg.Users[i].User.ClientKeyData)
			break
		}
	}

	return runtime, nil
}

func newK8sHTTPClient(runtime kubeClusterRuntime) (*http.Client, error) {
	client, err := newK8sHTTPClientWithDial(runtime, nil)
	return client, err
}

func newK8sHTTPClientWithDial(runtime kubeClusterRuntime, dialContext func(context.Context, string, string) (net.Conn, error)) (*http.Client, error) {
	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: runtime.InsecureSkipTLSVerify,
	}

	if runtime.CertificateAuthority != "" {
		caBytes, err := base64.StdEncoding.DecodeString(runtime.CertificateAuthority)
		if err != nil {
			return nil, err
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caBytes) {
			return nil, errors.New("invalid certificate authority")
		}
		tlsConfig.RootCAs = pool
	}

	if runtime.ClientCertificateData != "" && runtime.ClientKeyData != "" {
		certBytes, err := base64.StdEncoding.DecodeString(runtime.ClientCertificateData)
		if err != nil {
			return nil, err
		}
		keyBytes, err := base64.StdEncoding.DecodeString(runtime.ClientKeyData)
		if err != nil {
			return nil, err
		}
		cert, err := tls.X509KeyPair(certBytes, keyBytes)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	if dialContext != nil {
		transport.DialContext = dialContext
	}
	return &http.Client{Timeout: 8 * time.Second, Transport: transport}, nil
}

func (s *Service) newK8sHTTPClientForCluster(cluster model.K8sCluster, runtime kubeClusterRuntime) (*http.Client, func(), error) {
	if normalizeConnectionMode(cluster.ConnectionMode) != "gateway" || cluster.GatewayID == nil || *cluster.GatewayID == 0 {
		client, err := newK8sHTTPClient(runtime)
		return client, func() {}, err
	}
	gatewayID := *cluster.GatewayID
	dialContext := func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, cleanup, err := s.dialThroughGateway(ctx, gatewayID, network, address)
		if err != nil {
			return nil, err
		}
		return cleanupConn{Conn: conn, cleanup: cleanup}, nil
	}
	client, err := newK8sHTTPClientWithDial(runtime, dialContext)
	if err != nil {
		return nil, func() {}, err
	}
	return client, func() {}, nil
}

func fetchK8sVersion(client *http.Client, runtime kubeClusterRuntime) (string, error) {
	var payload kubeVersionResponse
	if err := k8sGetJSON(client, runtime, "/version", &payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.GitVersion) == "" {
		return "", errors.New("empty version")
	}
	return payload.GitVersion, nil
}

func fetchK8sNodeCount(client *http.Client, runtime kubeClusterRuntime) (int, error) {
	var payload kubeNodeListResponse
	if err := k8sGetJSON(client, runtime, "/api/v1/nodes", &payload); err != nil {
		return 0, err
	}
	return len(payload.Items), nil
}

func flattenGatewayHosts(item kubeIstioGateway) []string {
	hosts := make([]string, 0)
	for _, server := range item.Spec.Servers {
		hosts = append(hosts, server.Hosts...)
	}
	return uniqueNonEmptyStrings(hosts)
}

func flattenGatewayPorts(item kubeIstioGateway) []string {
	ports := make([]string, 0, len(item.Spec.Servers))
	for _, server := range item.Spec.Servers {
		ports = append(ports, formatIstioPort(server.Port.Number, server.Port.Protocol))
	}
	return uniqueNonEmptyStrings(ports)
}

func k8sGetJSON(client *http.Client, runtime kubeClusterRuntime, path string, target any) error {
	return k8sGetJSONWithQuery(client, runtime, path, nil, target)
}

func k8sGetJSONAnyPath(client *http.Client, runtime kubeClusterRuntime, paths []string, target any) error {
	var lastErr error
	for _, path := range paths {
		if err := k8sGetJSON(client, runtime, path, target); err != nil {
			lastErr = err
			if isK8sNotFoundError(err) {
				continue
			}
			return err
		}
		return nil
	}
	if lastErr == nil {
		lastErr = errors.New("resource path not found")
	}
	return lastErr
}

func k8sGetIstioJSON(client *http.Client, runtime kubeClusterRuntime, resource string, namespace string, name string, target any) error {
	paths := buildIstioResourcePaths(resource, namespace, name)
	return k8sGetJSONAnyPath(client, runtime, paths, target)
}

func k8sGetGatewayAPIJSON(client *http.Client, runtime kubeClusterRuntime, resource string, namespace string, name string, target any) error {
	paths := buildGatewayAPIResourcePaths(resource, namespace, name)
	return k8sGetJSONAnyPath(client, runtime, paths, target)
}

func buildIstioResourcePaths(resource string, namespace string, name string) []string {
	return buildIstioResourcePathsWithPreferred(resource, namespace, name, "")
}

func buildIstioResourcePathsWithPreferred(resource string, namespace string, name string, preferredVersion string) []string {
	versions := []string{"v1", "v1beta1"}
	preferredVersion = strings.TrimSpace(strings.TrimPrefix(preferredVersion, "networking.istio.io/"))
	if preferredVersion == "v1beta1" {
		versions = []string{"v1beta1", "v1"}
	}
	paths := make([]string, 0, len(versions))
	for _, version := range versions {
		base := fmt.Sprintf("/apis/networking.istio.io/%s", version)
		if strings.TrimSpace(namespace) != "" {
			base += "/namespaces/" + strings.TrimSpace(namespace)
		}
		base += "/" + resource
		if strings.TrimSpace(name) != "" {
			base += "/" + strings.TrimSpace(name)
		}
		paths = append(paths, base)
	}
	return paths
}

func buildGatewayAPIResourcePaths(resource string, namespace string, name string) []string {
	return buildGatewayAPIResourcePathsWithPreferred(resource, namespace, name, "")
}

func buildGatewayAPIResourcePathsWithPreferred(resource string, namespace string, name string, preferredVersion string) []string {
	versions := []string{"v1", "v1beta1"}
	preferredVersion = strings.TrimSpace(strings.TrimPrefix(preferredVersion, "gateway.networking.k8s.io/"))
	if preferredVersion == "v1beta1" {
		versions = []string{"v1beta1", "v1"}
	}
	paths := make([]string, 0, len(versions))
	for _, version := range versions {
		base := fmt.Sprintf("/apis/gateway.networking.k8s.io/%s", version)
		if strings.TrimSpace(namespace) != "" {
			base += "/namespaces/" + strings.TrimSpace(namespace)
		}
		base += "/" + resource
		if strings.TrimSpace(name) != "" {
			base += "/" + strings.TrimSpace(name)
		}
		paths = append(paths, base)
	}
	return paths
}

func k8sPatchJSON(client *http.Client, runtime kubeClusterRuntime, path string, body any, contentType string, target any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return k8sDoJSON(client, runtime, http.MethodPatch, path, nil, payload, contentType, target)
}

func k8sGetJSONWithQuery(client *http.Client, runtime kubeClusterRuntime, path string, query map[string]string, target any) error {
	return k8sDoJSON(client, runtime, http.MethodGet, path, query, nil, "application/json", target)
}

func k8sDoJSONAnyPath(
	client *http.Client,
	runtime kubeClusterRuntime,
	method string,
	paths []string,
	query map[string]string,
	body []byte,
	contentType string,
	target any,
) error {
	var lastErr error
	for _, path := range paths {
		err := k8sDoJSON(client, runtime, method, path, query, body, contentType, target)
		if err == nil {
			return nil
		}
		lastErr = err
		if isK8sNotFoundError(err) {
			continue
		}
		return err
	}
	if lastErr == nil {
		lastErr = errors.New("resource path not found")
	}
	return lastErr
}

func k8sDoJSON(client *http.Client, runtime kubeClusterRuntime, method string, path string, query map[string]string, body []byte, contentType string, target any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	endpointURL := strings.TrimRight(runtime.Server, "/") + path
	if len(query) > 0 {
		values := url.Values{}
		for key, value := range query {
			values.Set(key, value)
		}
		endpointURL += "?" + values.Encode()
	}

	var reader io.Reader
	if len(body) > 0 {
		reader = strings.NewReader(string(body))
	}
	req, err := http.NewRequestWithContext(ctx, method, endpointURL, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if len(body) > 0 && strings.TrimSpace(contentType) != "" {
		req.Header.Set("Content-Type", contentType)
	}

	if runtime.Token != "" {
		req.Header.Set("Authorization", "Bearer "+runtime.Token)
	}
	if runtime.Username != "" || runtime.Password != "" {
		req.SetBasicAuth(runtime.Username, runtime.Password)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message, _ := io.ReadAll(resp.Body)
		if len(message) > 0 {
			return fmt.Errorf("unexpected status: %d, %s", resp.StatusCode, strings.TrimSpace(string(message)))
		}
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	if target == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func buildK8sYAMLResourcePath(payload model.K8sResourceYAMLPayload) (string, error) {
	resourceType := strings.ToLower(Trimmed(payload.ResourceType))
	namespace := Trimmed(payload.Namespace)
	name := Trimmed(payload.Name)

	switch resourceType {
	case "namespace":
		if name == "" {
			return "", errors.New("namespace name is required")
		}
		return "/api/v1/namespaces/" + name, nil
	case "pod":
		if namespace == "" || name == "" {
			return "", errors.New("pod namespace and name are required")
		}
		return fmt.Sprintf("/api/v1/namespaces/%s/pods/%s", namespace, name), nil
	case "service":
		if namespace == "" || name == "" {
			return "", errors.New("service namespace and name are required")
		}
		return fmt.Sprintf("/api/v1/namespaces/%s/services/%s", namespace, name), nil
	case "ingress":
		if namespace == "" || name == "" {
			return "", errors.New("ingress namespace and name are required")
		}
		return fmt.Sprintf("/apis/networking.k8s.io/v1/namespaces/%s/ingresses/%s", namespace, name), nil
	case "configmap":
		if namespace == "" || name == "" {
			return "", errors.New("configmap namespace and name are required")
		}
		return fmt.Sprintf("/api/v1/namespaces/%s/configmaps/%s", namespace, name), nil
	case "secret":
		if namespace == "" || name == "" {
			return "", errors.New("secret namespace and name are required")
		}
		return fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", namespace, name), nil
	case "pvc":
		if namespace == "" || name == "" {
			return "", errors.New("pvc namespace and name are required")
		}
		return fmt.Sprintf("/api/v1/namespaces/%s/persistentvolumeclaims/%s", namespace, name), nil
	case "pv":
		if name == "" {
			return "", errors.New("pv name is required")
		}
		return "/api/v1/persistentvolumes/" + name, nil
	case "workload":
		if namespace == "" || name == "" {
			return "", errors.New("workload namespace and name are required")
		}
		switch strings.ToLower(Trimmed(payload.WorkloadType)) {
		case "deployment":
			return fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments/%s", namespace, name), nil
		case "statefulset":
			return fmt.Sprintf("/apis/apps/v1/namespaces/%s/statefulsets/%s", namespace, name), nil
		case "daemonset":
			return fmt.Sprintf("/apis/apps/v1/namespaces/%s/daemonsets/%s", namespace, name), nil
		case "job":
			return fmt.Sprintf("/apis/batch/v1/namespaces/%s/jobs/%s", namespace, name), nil
		case "cronjob":
			return fmt.Sprintf("/apis/batch/v1/namespaces/%s/cronjobs/%s", namespace, name), nil
		default:
			return "", errors.New("unsupported workload type")
		}
	default:
		return "", errors.New("unsupported resource type")
	}
}

func buildK8sCreateResourcePaths(payload model.K8sResourceYAMLPayload, manifest k8sManifestIdentity) ([]string, error) {
	resourceType := strings.ToLower(Trimmed(payload.ResourceType))
	namespace := firstNonEmpty(Trimmed(payload.Namespace), Trimmed(manifest.Metadata.Namespace))

	switch resourceType {
	case "namespace":
		return []string{"/api/v1/namespaces"}, nil
	case "pod":
		if namespace == "" {
			return nil, errors.New("pod namespace is required")
		}
		return []string{fmt.Sprintf("/api/v1/namespaces/%s/pods", namespace)}, nil
	case "service":
		if namespace == "" {
			return nil, errors.New("service namespace is required")
		}
		return []string{fmt.Sprintf("/api/v1/namespaces/%s/services", namespace)}, nil
	case "ingress":
		if namespace == "" {
			return nil, errors.New("ingress namespace is required")
		}
		return []string{fmt.Sprintf("/apis/networking.k8s.io/v1/namespaces/%s/ingresses", namespace)}, nil
	case "configmap":
		if namespace == "" {
			return nil, errors.New("configmap namespace is required")
		}
		return []string{fmt.Sprintf("/api/v1/namespaces/%s/configmaps", namespace)}, nil
	case "secret":
		if namespace == "" {
			return nil, errors.New("secret namespace is required")
		}
		return []string{fmt.Sprintf("/api/v1/namespaces/%s/secrets", namespace)}, nil
	case "pvc":
		if namespace == "" {
			return nil, errors.New("pvc namespace is required")
		}
		return []string{fmt.Sprintf("/api/v1/namespaces/%s/persistentvolumeclaims", namespace)}, nil
	case "pv":
		return []string{"/api/v1/persistentvolumes"}, nil
	case "workload":
		if namespace == "" {
			return nil, errors.New("workload namespace is required")
		}
		switch strings.ToLower(Trimmed(payload.WorkloadType)) {
		case "deployment":
			return []string{fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments", namespace)}, nil
		case "statefulset":
			return []string{fmt.Sprintf("/apis/apps/v1/namespaces/%s/statefulsets", namespace)}, nil
		case "daemonset":
			return []string{fmt.Sprintf("/apis/apps/v1/namespaces/%s/daemonsets", namespace)}, nil
		case "job":
			return []string{fmt.Sprintf("/apis/batch/v1/namespaces/%s/jobs", namespace)}, nil
		case "cronjob":
			return []string{fmt.Sprintf("/apis/batch/v1/namespaces/%s/cronjobs", namespace)}, nil
		default:
			return nil, errors.New("unsupported workload type")
		}
	case "gateway", "virtualservice", "destinationrule", "serviceentry":
		if namespace == "" {
			return nil, fmt.Errorf("%s namespace is required", resourceType)
		}
		resourceMap := map[string]string{
			"gateway":         "gateways",
			"virtualservice":  "virtualservices",
			"destinationrule": "destinationrules",
			"serviceentry":    "serviceentries",
		}
		return buildIstioResourcePathsWithPreferred(
			resourceMap[resourceType],
			namespace,
			"",
			manifest.APIVersion,
		), nil
	case "gatewayapi":
		if namespace == "" {
			return nil, errors.New("gateway namespace is required")
		}
		return buildGatewayAPIResourcePathsWithPreferred("gateways", namespace, "", manifest.APIVersion), nil
	case "httproute":
		if namespace == "" {
			return nil, errors.New("httproute namespace is required")
		}
		return buildGatewayAPIResourcePathsWithPreferred("httproutes", namespace, "", manifest.APIVersion), nil
	default:
		return nil, errors.New("unsupported resource type")
	}
}

func buildK8sYAMLResourcePaths(payload model.K8sResourceYAMLPayload) ([]string, error) {
	resourceType := strings.ToLower(Trimmed(payload.ResourceType))
	namespace := Trimmed(payload.Namespace)
	name := Trimmed(payload.Name)

	switch resourceType {
	case "gateway", "virtualservice", "destinationrule", "serviceentry":
		if namespace == "" || name == "" {
			return nil, fmt.Errorf("%s namespace and name are required", resourceType)
		}
		resourceMap := map[string]string{
			"gateway":         "gateways",
			"virtualservice":  "virtualservices",
			"destinationrule": "destinationrules",
			"serviceentry":    "serviceentries",
		}
		return buildIstioResourcePaths(resourceMap[resourceType], namespace, name), nil
	case "gatewayapi":
		if namespace == "" || name == "" {
			return nil, errors.New("gateway namespace and name are required")
		}
		return buildGatewayAPIResourcePaths("gateways", namespace, name), nil
	case "httproute":
		if namespace == "" || name == "" {
			return nil, errors.New("httproute namespace and name are required")
		}
		return buildGatewayAPIResourcePaths("httproutes", namespace, name), nil
	default:
		path, err := buildK8sYAMLResourcePath(payload)
		if err != nil {
			return nil, err
		}
		return []string{path}, nil
	}
}

func buildK8sDeleteResourcePaths(payload model.K8sResourceDeletePayload) ([]string, error) {
	return buildK8sYAMLResourcePaths(model.K8sResourceYAMLPayload{
		ResourceType: payload.ResourceType,
		Namespace:    payload.Namespace,
		Name:         payload.Name,
		WorkloadType: payload.WorkloadType,
	})
}

func parseK8sManifestIdentity(body []byte) (k8sManifestIdentity, error) {
	var manifest k8sManifestIdentity
	if err := json.Unmarshal(body, &manifest); err != nil {
		return manifest, errors.New("invalid yaml content")
	}
	if Trimmed(manifest.Kind) == "" {
		return manifest, errors.New("resource kind is required")
	}
	return manifest, nil
}

func isK8sNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unexpected status: 404") || strings.Contains(message, "\"code\":404")
}

func friendlyK8sYAMLError(payload model.K8sResourceYAMLPayload, err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	lower := strings.ToLower(message)

	if strings.Contains(lower, "field is immutable") || strings.Contains(lower, "immutable") {
		switch strings.ToLower(Trimmed(payload.ResourceType)) {
		case "pod":
			return errors.New("the Pod contains immutable fields that Kubernetes cannot update directly; modify only mutable fields or recreate the Pod")
		case "pv", "pvc":
			return errors.New("the storage resource contains immutable fields and cannot be overwritten directly; modify only mutable fields or use the storage-change workflow")
		default:
			return errors.New("the resource contains immutable fields and cannot be overwritten directly; check whether metadata, selector, or volume fields were changed")
		}
	}

	if strings.Contains(lower, "already exists") {
		return errors.New("the YAML resource identity conflicts with an existing cluster resource; verify the name, namespace, and related objects")
	}
	if strings.Contains(lower, "not found") {
		return errors.New("target resource does not exist; it may have been deleted or moved to another namespace; refresh and retry")
	}
	if strings.Contains(lower, "invalid") || strings.Contains(lower, "unprocessable entity") {
		return errors.New("YAML validation failed; verify field formats, apiVersion, kind, and spec content")
	}
	return err
}

func k8sGetText(client *http.Client, runtime kubeClusterRuntime, path string, query map[string]string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	endpointURL := strings.TrimRight(runtime.Server, "/") + path
	if len(query) > 0 {
		values := url.Values{}
		for key, value := range query {
			values.Set(key, value)
		}
		endpointURL += "?" + values.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpointURL, nil)
	if err != nil {
		return "", err
	}

	if runtime.Token != "" {
		req.Header.Set("Authorization", "Bearer "+runtime.Token)
	}
	if runtime.Username != "" || runtime.Password != "" {
		req.SetBasicAuth(runtime.Username, runtime.Password)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func k8sStatusText(status string) string {
	switch normalizedK8sStatus(status) {
	case "warning":
		return "Partial Alerts"
	case "offline":
		return "Offline"
	default:
		return "Running"
	}
}

func normalizedK8sStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "warning":
		return "warning"
	case "offline":
		return "offline"
	default:
		return "running"
	}
}

func calculateHealthScore(alertCount int) int {
	if alertCount <= 0 {
		return 100
	}
	score := 100 - alertCount*8
	if score < 40 {
		return 40
	}
	return score
}

func formatUsagePercent(used int64, total int64) string {
	if used <= 0 || total <= 0 {
		return "-"
	}
	value := float64(used) / float64(total) * 100
	return fmt.Sprintf("%.1f%%", value)
}

func nodeReadyStatus(node kubeNode) string {
	for _, condition := range node.Status.Conditions {
		if condition.Type == "Ready" {
			if condition.Status == "True" {
				return "Ready"
			}
			return "NotReady"
		}
	}
	return "Unknown"
}

func firstNodeInternalIP(node kubeNode) string {
	for _, address := range node.Status.Addresses {
		if address.Type == "InternalIP" && strings.TrimSpace(address.Address) != "" {
			return address.Address
		}
	}
	return "-"
}

func joinNodeRoles(labels map[string]string) string {
	roles := make([]string, 0, 3)
	for key := range labels {
		if strings.HasPrefix(key, "node-role.kubernetes.io/") {
			role := strings.TrimPrefix(key, "node-role.kubernetes.io/")
			if role == "" {
				role = "worker"
			}
			roles = append(roles, role)
		}
	}
	if len(roles) == 0 {
		return "worker"
	}
	sort.Strings(roles)
	return strings.Join(roles, ",")
}

func parseCPUToMilli(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if strings.HasSuffix(value, "m") {
		number := strings.TrimSuffix(value, "m")
		parsed, _ := strconv.ParseFloat(number, 64)
		return int64(parsed)
	}
	parsed, _ := strconv.ParseFloat(value, 64)
	return int64(parsed * 1000)
}

func parseBytesQuantity(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}

	units := map[string]float64{
		"Ki": 1024,
		"Mi": 1024 * 1024,
		"Gi": 1024 * 1024 * 1024,
		"Ti": 1024 * 1024 * 1024 * 1024,
		"Pi": 1024 * 1024 * 1024 * 1024 * 1024,
		"K":  1000,
		"M":  1000 * 1000,
		"G":  1000 * 1000 * 1000,
		"T":  1000 * 1000 * 1000 * 1000,
	}

	for suffix, multiplier := range units {
		if strings.HasSuffix(value, suffix) {
			number := strings.TrimSpace(strings.TrimSuffix(value, suffix))
			parsed, _ := strconv.ParseFloat(number, 64)
			return int64(parsed * multiplier)
		}
	}

	parsed, _ := strconv.ParseFloat(value, 64)
	return int64(parsed)
}

func formatMemoryMB(value string) string {
	bytes := parseBytesQuantity(value)
	if bytes <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d MB", int64(math.Round(float64(bytes)/1000/1000)))
}

func humanizeAge(timestamp string) string {
	createdAt, err := time.Parse(time.RFC3339, strings.TrimSpace(timestamp))
	if err != nil {
		return "-"
	}

	duration := time.Since(createdAt)
	if duration < time.Minute {
		return "Just now"
	}
	if duration < time.Hour {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	}
	if duration < 24*time.Hour {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	}
	if duration < 30*24*time.Hour {
		return fmt.Sprintf("%dd", int(duration.Hours()/24))
	}
	return createdAt.Format("2006-01-02")
}

func formatTimestamp(value string) string {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return "-"
	}
	return t.Format("2006-01-02 15:04")
}

func stringifyTargetPort(value interface{}) string {
	switch current := value.(type) {
	case string:
		return current
	case float64:
		return strconv.Itoa(int(current))
	default:
		return fmt.Sprint(current)
	}
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func cronJobReadyText(suspend *bool, active int) string {
	if suspend != nil && *suspend {
		return "Suspended"
	}
	if active > 0 {
		return fmt.Sprintf("%d Active", active)
	}
	return "Scheduled"
}

func fallbackText(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func intLabel(value int, suffix string) string {
	return strconv.Itoa(value) + suffix
}
