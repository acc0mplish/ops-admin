// k8s_types_mesh.go — moved verbatim from k8s.go (Phase BCD, E5 seam #3).
package service

type kubeIstioGatewayListResponse struct {
	Items []kubeIstioGateway `json:"items"`
}

type kubeIstioVirtualServiceListResponse struct {
	Items []kubeIstioVirtualService `json:"items"`
}

type kubeIstioDestinationRuleListResponse struct {
	Items []kubeIstioDestinationRule `json:"items"`
}

type kubeIstioServiceEntryListResponse struct {
	Items []kubeIstioServiceEntry `json:"items"`
}

type kubeGatewayAPIListResponse struct {
	Items []kubeGatewayAPI `json:"items"`
}

type kubeHTTPRouteListResponse struct {
	Items []kubeHTTPRoute `json:"items"`
}

type kubeIstioGateway struct {
	Metadata kubeMetadata `json:"metadata"`
	Spec     struct {
		Selector map[string]string `json:"selector"`
		Servers  []struct {
			Port struct {
				Name     string `json:"name"`
				Number   int    `json:"number"`
				Protocol string `json:"protocol"`
			} `json:"port"`
			Hosts []string `json:"hosts"`
		} `json:"servers"`
	} `json:"spec"`
}

type kubeIstioVirtualService struct {
	APIVersion string       `json:"apiVersion"`
	Metadata   kubeMetadata `json:"metadata"`
	Spec       struct {
		Hosts    []string `json:"hosts"`
		Gateways []string `json:"gateways"`
		HTTP     []struct {
			Match []struct {
				URI struct {
					Exact  string `json:"exact"`
					Prefix string `json:"prefix"`
				} `json:"uri"`
			} `json:"match"`
			Route []struct {
				Destination struct {
					Host   string `json:"host"`
					Subset string `json:"subset"`
					Port   struct {
						Number int `json:"number"`
					} `json:"port"`
				} `json:"destination"`
				Weight int `json:"weight"`
			} `json:"route"`
		} `json:"http"`
		TCP []struct {
			Route []struct {
				Destination struct {
					Host string `json:"host"`
					Port struct {
						Number int `json:"number"`
					} `json:"port"`
				} `json:"destination"`
			} `json:"route"`
		} `json:"tcp"`
	} `json:"spec"`
}

type kubeIstioDestinationRule struct {
	Metadata kubeMetadata `json:"metadata"`
	Spec     struct {
		Host    string `json:"host"`
		Subsets []struct {
			Name string `json:"name"`
		} `json:"subsets"`
	} `json:"spec"`
}

type kubeIstioServiceEntry struct {
	Metadata kubeMetadata `json:"metadata"`
	Spec     struct {
		Hosts      []string `json:"hosts"`
		Addresses  []string `json:"addresses"`
		Location   string   `json:"location"`
		Resolution string   `json:"resolution"`
		Ports      []struct {
			Name     string `json:"name"`
			Number   int    `json:"number"`
			Protocol string `json:"protocol"`
		} `json:"ports"`
	} `json:"spec"`
}

type kubeGatewayAPI struct {
	APIVersion string       `json:"apiVersion"`
	Kind       string       `json:"kind"`
	Metadata   kubeMetadata `json:"metadata"`
	Spec       struct {
		GatewayClassName string `json:"gatewayClassName"`
		Listeners        []struct {
			Name     string `json:"name"`
			Hostname string `json:"hostname"`
			Port     int    `json:"port"`
			Protocol string `json:"protocol"`
		} `json:"listeners"`
	} `json:"spec"`
	Status struct {
		Addresses []struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"addresses"`
	} `json:"status"`
}

type kubeHTTPRoute struct {
	APIVersion string       `json:"apiVersion"`
	Kind       string       `json:"kind"`
	Metadata   kubeMetadata `json:"metadata"`
	Spec       struct {
		Hostnames  []string `json:"hostnames"`
		ParentRefs []struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"parentRefs"`
		Rules []struct {
			Matches []struct {
				Path struct {
					Type  string `json:"type"`
					Value string `json:"value"`
				} `json:"path"`
			} `json:"matches"`
			BackendRefs []struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
				Port      int    `json:"port"`
				Weight    int    `json:"weight"`
			} `json:"backendRefs"`
		} `json:"rules"`
	} `json:"spec"`
}

type k8sClusterProbe struct {
	APIServer string
	Version   string
	NodeCount int
	Status    string
}

type k8sFetchedData struct {
	Nodes                 []kubeNode
	Namespaces            []kubeNamespace
	Pods                  []kubePod
	Services              []kubeService
	Endpoints             []kubeEndpoints
	Ingresses             []kubeIngress
	ConfigMaps            []kubeConfigMap
	Secrets               []kubeSecret
	PVCs                  []kubePersistentVolumeClaim
	PVs                   []kubePersistentVolume
	Deployments           []kubeDeployment
	ReplicaSets           []kubeReplicaSet
	StatefulSet           []kubeStatefulSet
	DaemonSets            []kubeDaemonSet
	Jobs                  []kubeJob
	CronJobs              []kubeCronJob
	GatewayAPIGateways    []kubeGatewayAPI
	HTTPRoutes            []kubeHTTPRoute
	IstioGateways         []kubeIstioGateway
	IstioVirtualServices  []kubeIstioVirtualService
	IstioDestinationRules []kubeIstioDestinationRule
	IstioServiceEntries   []kubeIstioServiceEntry
}

type k8sAggregateMetrics struct {
	TotalAllocCPUMilli    int64
	TotalAllocMemoryBytes int64
	TotalReqCPUMilli      int64
	TotalReqMemoryBytes   int64
	AlertCount            int
}

type k8sManifestIdentity struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
}
