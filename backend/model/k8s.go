package model

import "time"

type K8sCluster struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	Name        string     `json:"name" gorm:"size:128;not null;uniqueIndex"`
	Status      string     `json:"status" gorm:"size:32;not null;default:running"`
	APIServer   string     `json:"apiServer" gorm:"size:255;not null"`
	Version     string     `json:"version" gorm:"size:64;not null"`
	NodeCount   int        `json:"nodeCount" gorm:"default:0"`
	Description string     `json:"description" gorm:"size:255"`
	KubeConfig  string     `json:"kubeConfig" gorm:"type:text"`
	LastSyncAt  *time.Time `json:"lastSyncAt"`
	CreatedAt   time.Time  `json:"createTime"`
	UpdatedAt   time.Time  `json:"updateTime"`
}

func (K8sCluster) TableName() string {
	return "k8s_cluster"
}

type K8sClusterPayload struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	KubeConfig  string `json:"kubeConfig"`
}

type K8sWorkloadActionPayload struct {
	ClusterID    uint   `json:"clusterId"`
	Namespace    string `json:"namespace"`
	WorkloadType string `json:"workloadType"`
	WorkloadName string `json:"workloadName"`
	Replicas     int    `json:"replicas"`
}

type K8sResourceYAMLPayload struct {
	ClusterID    uint   `json:"clusterId"`
	ResourceType string `json:"resourceType"`
	Namespace    string `json:"namespace"`
	Name         string `json:"name"`
	WorkloadType string `json:"workloadType"`
	YAML         string `json:"yaml"`
}

type K8sClusterView struct {
	ID          uint       `json:"id"`
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	StatusText  string     `json:"statusText"`
	APIServer   string     `json:"apiServer"`
	Version     string     `json:"version"`
	NodeCount   int        `json:"nodeCount"`
	Description string     `json:"description"`
	LastSyncAt  *time.Time `json:"lastSyncAt"`
	CreatedAt   time.Time  `json:"createTime"`
	UpdatedAt   time.Time  `json:"updateTime"`
}

type K8sOverview struct {
	HealthScore  int              `json:"healthScore"`
	CPUUsage     string           `json:"cpuUsage"`
	MemoryUsage  string           `json:"memoryUsage"`
	PodUsage     string           `json:"podUsage"`
	RequestRate  string           `json:"requestRate"`
	AlertCount   int              `json:"alertCount"`
	Distribution []K8sKVTextItem  `json:"distribution"`
	Certificates []K8sCertificate `json:"certificates"`
}

type K8sKVTextItem struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type K8sCertificate struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Subject       string `json:"subject"`
	Issuer        string `json:"issuer"`
	NotBefore     string `json:"notBefore"`
	NotAfter      string `json:"notAfter"`
	DaysRemaining int    `json:"daysRemaining"`
	Status        string `json:"status"`
	StatusText    string `json:"statusText"`
}

type K8sNodeItem struct {
	Name       string `json:"name"`
	Role       string `json:"role"`
	Status     string `json:"status"`
	Version    string `json:"version"`
	InternalIP string `json:"internalIP"`
	CPU        string `json:"cpu"`
	Memory     string `json:"memory"`
	Pods       string `json:"pods"`
}

type K8sNamespaceItem struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Pods      int    `json:"pods"`
	Services  int    `json:"services"`
	Workloads int    `json:"workloads"`
	CreatedAt string `json:"createdAt"`
}

type K8sPodItem struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	Node      string `json:"node"`
	Restarts  int    `json:"restarts"`
	Age       string `json:"age"`
	IP        string `json:"ip"`
}

type K8sContainerItem struct {
	Name    string `json:"name"`
	Image   string `json:"image"`
	Ready   bool   `json:"ready"`
	Restart int    `json:"restart"`
}

type K8sNodeDetail struct {
	Name           string            `json:"name"`
	Status         string            `json:"status"`
	Roles          string            `json:"roles"`
	Version        string            `json:"version"`
	InternalIP     string            `json:"internalIP"`
	OS             string            `json:"os"`
	Kernel         string            `json:"kernel"`
	ContainerRT    string            `json:"containerRuntime"`
	Architecture   string            `json:"architecture"`
	Labels         map[string]string `json:"labels"`
	CapacityCPU    string            `json:"capacityCPU"`
	CapacityMem    string            `json:"capacityMemory"`
	AllocatableCPU string            `json:"allocatableCPU"`
	AllocatableMem string            `json:"allocatableMemory"`
	Pods           []K8sPodItem      `json:"pods"`
}

type K8sPodDetail struct {
	Name           string             `json:"name"`
	Namespace      string             `json:"namespace"`
	Status         string             `json:"status"`
	Node           string             `json:"node"`
	PodIP          string             `json:"podIP"`
	HostIP         string             `json:"hostIP"`
	QoSClass       string             `json:"qosClass"`
	ServiceAccount string             `json:"serviceAccount"`
	Labels         map[string]string  `json:"labels"`
	Containers     []K8sContainerItem `json:"containers"`
	CreatedAt      string             `json:"createdAt"`
	YAML           string             `json:"yaml"`
}

type K8sEventItem struct {
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	Message   string `json:"message"`
	Count     int    `json:"count"`
	FirstTime string `json:"firstTime"`
	LastTime  string `json:"lastTime"`
}

type K8sNamespaceDetail struct {
	Name        string            `json:"name"`
	Status      string            `json:"status"`
	CreatedAt   string            `json:"createdAt"`
	Labels      map[string]string `json:"labels"`
	Pods        int               `json:"pods"`
	Services    int               `json:"services"`
	Workloads   int               `json:"workloads"`
	ConfigMaps  int               `json:"configMaps"`
	Secrets     int               `json:"secrets"`
	Storage     int               `json:"storage"`
	Annotations map[string]string `json:"annotations"`
	YAML        string            `json:"yaml"`
}

type K8sWorkloadItem struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Namespace string `json:"namespace"`
	Ready     string `json:"ready"`
	Updated   int    `json:"updated"`
	Available int    `json:"available"`
	Age       string `json:"age"`
}

type K8sWorkloadDetail struct {
	Name        string             `json:"name"`
	Type        string             `json:"type"`
	Namespace   string             `json:"namespace"`
	Ready       string             `json:"ready"`
	Updated     int                `json:"updated"`
	Available   int                `json:"available"`
	Age         string             `json:"age"`
	Labels      map[string]string  `json:"labels"`
	Annotations map[string]string  `json:"annotations"`
	Selector    map[string]string  `json:"selector"`
	Pods        []K8sPodItem       `json:"pods"`
	Containers  []K8sContainerItem `json:"containers"`
	YAML        string             `json:"yaml"`
}

type K8sServiceItem struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
	ClusterIP string `json:"clusterIP"`
	Ports     string `json:"ports"`
	Endpoints int    `json:"endpoints"`
}

type K8sServiceDetail struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Type        string            `json:"type"`
	ClusterIP   string            `json:"clusterIP"`
	Ports       []K8sKVTextItem   `json:"ports"`
	Selector    map[string]string `json:"selector"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Endpoints   int               `json:"endpoints"`
	Age         string            `json:"age"`
	YAML        string            `json:"yaml"`
}

type K8sIngressItem struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Host      string `json:"host"`
	Address   string `json:"address"`
	TLS       string `json:"tls"`
	Age       string `json:"age"`
}

type K8sIngressDetail struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Host        string            `json:"host"`
	Address     string            `json:"address"`
	TLS         string            `json:"tls"`
	ClassName   string            `json:"className"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Rules       []K8sKVTextItem   `json:"rules"`
	Age         string            `json:"age"`
	YAML        string            `json:"yaml"`
}

type K8sConfigMapItem struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Keys      int    `json:"keys"`
	Age       string `json:"age"`
}

type K8sConfigMapDetail struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Keys        []K8sKVTextItem   `json:"keys"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Age         string            `json:"age"`
	YAML        string            `json:"yaml"`
}

type K8sSecretItem struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
	Age       string `json:"age"`
}

type K8sSecretDetail struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Type        string            `json:"type"`
	Keys        []K8sKVTextItem   `json:"keys"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Age         string            `json:"age"`
	YAML        string            `json:"yaml"`
}

type K8sStorageItem struct {
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	Namespace    string `json:"namespace"`
	Status       string `json:"status"`
	Capacity     string `json:"capacity"`
	StorageClass string `json:"storageClass"`
}

type K8sStorageDetail struct {
	Name         string            `json:"name"`
	Kind         string            `json:"kind"`
	Namespace    string            `json:"namespace"`
	Status       string            `json:"status"`
	Capacity     string            `json:"capacity"`
	StorageClass string            `json:"storageClass"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	Age          string            `json:"age"`
	YAML         string            `json:"yaml"`
}

type K8sNetworkSection struct {
	Services  []K8sServiceItem `json:"services"`
	Ingresses []K8sIngressItem `json:"ingresses"`
}

type K8sConfigStorageSection struct {
	ConfigMaps []K8sConfigMapItem `json:"configMaps"`
	Secrets    []K8sSecretItem    `json:"secrets"`
	Storage    []K8sStorageItem   `json:"storage"`
}

type K8sClusterDetail struct {
	Cluster       K8sClusterView          `json:"cluster"`
	Overview      K8sOverview             `json:"overview"`
	Nodes         []K8sNodeItem           `json:"nodes"`
	Namespaces    []K8sNamespaceItem      `json:"namespaces"`
	Pods          []K8sPodItem            `json:"pods"`
	Workloads     []K8sWorkloadItem       `json:"workloads"`
	Network       K8sNetworkSection       `json:"network"`
	ConfigStorage K8sConfigStorageSection `json:"configStorage"`
}
