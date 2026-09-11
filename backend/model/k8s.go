package model

import "time"

// K8sCluster — V2 Phase 6 I-b 이후 이 구조체는 순수 응답 DTO다(§J5 S5 —
// 타입은 존재, gorm 매핑 제거). 서비스 계층의 클러스터 소스는
// provider_connection + SecretRef 체인(I-a S5)이고 legacy k8s_cluster 테이블은
// step0007에서 drop됐으며(C66) v1 AutoMigrate 소유분도 제외됐다. gorm 태그와
// TableName은 제거됐다 — 이 타입을 테이블로 조회하는 경로는 존재하지 않는다.
type K8sCluster struct {
	ID                  uint              `json:"id"`
	Name                string            `json:"name"`
	Status              string            `json:"status"`
	APIServer           string            `json:"apiServer"`
	Version             string            `json:"version"`
	NodeCount           int               `json:"nodeCount"`
	Env                 string            `json:"env"`
	Tags                []string          `json:"tags"`
	ConnectionMode      string            `json:"connectionMode"`
	GatewayID           *uint             `json:"gatewayId"`
	Gateway             AssetGateway      `json:"gateway"`
	MonitorDatasourceID *uint             `json:"monitorDatasourceId"`
	MonitorDatasource   MonitorDatasource `json:"monitorDatasource"`
	Description         string            `json:"description"`
	KubeConfig          string            `json:"kubeConfig"`
	LastSyncAt          *time.Time        `json:"lastSyncAt"`
	CreatedAt           time.Time         `json:"createTime"`
	UpdatedAt           time.Time         `json:"updateTime"`
}

type K8sWorkloadActionPayload struct {
	ClusterID    uint   `json:"clusterId"`
	Namespace    string `json:"namespace"`
	WorkloadType string `json:"workloadType"`
	WorkloadName string `json:"workloadName"`
	Replicas     int    `json:"replicas"`
}

type K8sWorkloadContainerResources struct {
	Name            string          `json:"name"`
	RequestCPU      string          `json:"requestCPU"`
	LimitCPU        string          `json:"limitCPU"`
	RequestMemory   string          `json:"requestMemory"`
	LimitMemory     string          `json:"limitMemory"`
	ImagePullPolicy string          `json:"imagePullPolicy"`
	Env             []K8sEnvVarItem `json:"env"`
}

type K8sWorkloadResourcesPayload struct {
	ClusterID    uint                            `json:"clusterId"`
	Namespace    string                          `json:"namespace"`
	WorkloadType string                          `json:"workloadType"`
	WorkloadName string                          `json:"workloadName"`
	Containers   []K8sWorkloadContainerResources `json:"containers"`
}

type K8sIstioTrafficRoute struct {
	Index  int    `json:"index"`
	Host   string `json:"host"`
	Subset string `json:"subset"`
	Port   int    `json:"port"`
	Weight int    `json:"weight"`
	Label  string `json:"label"`
}

type K8sClusterView struct {
	ID                    uint       `json:"id"`
	Name                  string     `json:"name"`
	Status                string     `json:"status"`
	StatusText            string     `json:"statusText"`
	APIServer             string     `json:"apiServer"`
	Version               string     `json:"version"`
	NodeCount             int        `json:"nodeCount"`
	Env                   string     `json:"env"`
	Tags                  []string   `json:"tags"`
	ConnectionMode        string     `json:"connectionMode"`
	GatewayID             *uint      `json:"gatewayId"`
	GatewayName           string     `json:"gatewayName"`
	MonitorDatasourceID   *uint      `json:"monitorDatasourceId"`
	MonitorDatasourceName string     `json:"monitorDatasourceName"`
	Description           string     `json:"description"`
	LastSyncAt            *time.Time `json:"lastSyncAt"`
	CreatedAt             time.Time  `json:"createTime"`
	UpdatedAt             time.Time  `json:"updateTime"`
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
	Label     string `json:"label"`
	Value     string `json:"value"`
	Sensitive bool   `json:"sensitive"`
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
	OS         string `json:"os"`
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
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	WorkloadName string `json:"workloadName"`
	WorkloadType string `json:"workloadType"`
	Status       string `json:"status"`
	Node         string `json:"node"`
	NodeIP       string `json:"nodeIP"`
	Restarts     int    `json:"restarts"`
	Age          string `json:"age"`
	IP           string `json:"ip"`
}

type K8sEnvVarItem struct {
	Name      string         `json:"name"`
	Value     string         `json:"value"`
	ValueFrom map[string]any `json:"valueFrom,omitempty"`
	Source    string         `json:"source,omitempty"`
}

type K8sContainerItem struct {
	Name            string          `json:"name"`
	Image           string          `json:"image"`
	Ready           bool            `json:"ready"`
	Restart         int             `json:"restart"`
	RequestCPU      string          `json:"requestCPU"`
	LimitCPU        string          `json:"limitCPU"`
	RequestMemory   string          `json:"requestMemory"`
	LimitMemory     string          `json:"limitMemory"`
	ImagePullPolicy string          `json:"imagePullPolicy"`
	Env             []K8sEnvVarItem `json:"env"`
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
	Requests  string `json:"requests"`
	Limits    string `json:"limits"`
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
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Type       string `json:"type"`
	ClusterIP  string `json:"clusterIP"`
	ExternalIP string `json:"externalIP"`
	Ports      string `json:"ports"`
	Endpoints  int    `json:"endpoints"`
	Age        string `json:"age"`
}

type K8sServiceDetail struct {
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	Type         string            `json:"type"`
	ClusterIP    string            `json:"clusterIP"`
	ExternalIP   string            `json:"externalIP"`
	ExternalName string            `json:"externalName"`
	Ports        []K8sKVTextItem   `json:"ports"`
	PortSpecs    []K8sServicePort  `json:"portSpecs"`
	Selector     map[string]string `json:"selector"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	Endpoints    int               `json:"endpoints"`
	Age          string            `json:"age"`
	YAML         string            `json:"yaml"`
}

type K8sServicePort struct {
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	Port       int    `json:"port"`
	TargetPort string `json:"targetPort"`
	NodePort   int    `json:"nodePort"`
}

type K8sServiceUpdatePayload struct {
	ClusterID    uint              `json:"clusterId"`
	Namespace    string            `json:"namespace"`
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	Headless     bool              `json:"headless"`
	ExternalName string            `json:"externalName"`
	Selector     map[string]string `json:"selector"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	Ports        []K8sServicePort  `json:"ports"`
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
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	Namespace      string `json:"namespace"`
	NamespaceScope string `json:"namespaceScope"`
	Status         string `json:"status"`
	Capacity       string `json:"capacity"`
	StorageClass   string `json:"storageClass"`
	SourceType     string `json:"sourceType"`
	Path           string `json:"path"`
	NFSServer      string `json:"nfsServer"`
	AccessModes    string `json:"accessModes"`
	ReclaimPolicy  string `json:"reclaimPolicy"`
}

type K8sStorageDetail struct {
	Name           string            `json:"name"`
	Kind           string            `json:"kind"`
	Namespace      string            `json:"namespace"`
	NamespaceScope string            `json:"namespaceScope"`
	Status         string            `json:"status"`
	Capacity       string            `json:"capacity"`
	StorageClass   string            `json:"storageClass"`
	SourceType     string            `json:"sourceType"`
	Path           string            `json:"path"`
	NFSServer      string            `json:"nfsServer"`
	AccessModes    string            `json:"accessModes"`
	ReclaimPolicy  string            `json:"reclaimPolicy"`
	Labels         map[string]string `json:"labels"`
	Annotations    map[string]string `json:"annotations"`
	Age            string            `json:"age"`
	YAML           string            `json:"yaml"`
}

type K8sNetworkSection struct {
	Services  []K8sServiceItem `json:"services"`
	Ingresses []K8sIngressItem `json:"ingresses"`
}

type K8sIstioResourceItem struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Kind      string `json:"kind"`
	Hosts     string `json:"hosts"`
	Gateways  string `json:"gateways"`
	Target    string `json:"target"`
	Address   string `json:"address"`
	Ports     string `json:"ports"`
	Age       string `json:"age"`
}

type K8sIstioResourceDetail struct {
	Name        string                 `json:"name"`
	Namespace   string                 `json:"namespace"`
	Kind        string                 `json:"kind"`
	Labels      map[string]string      `json:"labels"`
	Annotations map[string]string      `json:"annotations"`
	Summary     []K8sKVTextItem        `json:"summary"`
	Items       []K8sKVTextItem        `json:"items"`
	Traffic     []K8sIstioTrafficRoute `json:"traffic"`
	Age         string                 `json:"age"`
	YAML        string                 `json:"yaml"`
}

type K8sAdvancedNetworkSection struct {
	GatewayAPIGateways []K8sIstioResourceItem `json:"gatewayApiGateways"`
	HTTPRoutes         []K8sIstioResourceItem `json:"httpRoutes"`
}

type K8sConfigStorageSection struct {
	ConfigMaps []K8sConfigMapItem `json:"configMaps"`
	Secrets    []K8sSecretItem    `json:"secrets"`
	Storage    []K8sStorageItem   `json:"storage"`
}

type K8sClusterDetail struct {
	Cluster         K8sClusterView            `json:"cluster"`
	Overview        K8sOverview               `json:"overview"`
	Nodes           []K8sNodeItem             `json:"nodes"`
	Namespaces      []K8sNamespaceItem        `json:"namespaces"`
	Pods            []K8sPodItem              `json:"pods"`
	Workloads       []K8sWorkloadItem         `json:"workloads"`
	Network         K8sNetworkSection         `json:"network"`
	AdvancedNetwork K8sAdvancedNetworkSection `json:"advancedNetwork"`
	ConfigStorage   K8sConfigStorageSection   `json:"configStorage"`
}
