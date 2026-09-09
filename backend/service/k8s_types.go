// k8s_types.go — moved verbatim from k8s.go (Phase BCD, E5 seam #2).
package service

const k8sClusterConnectError = "cluster connection failed; verify kubeconfig"

type kubeConfig struct {
	APIVersion     string `yaml:"apiVersion"`
	CurrentContext string `yaml:"current-context"`
	Clusters       []struct {
		Name    string `yaml:"name"`
		Cluster struct {
			Server                   string `yaml:"server"`
			CertificateAuthorityData string `yaml:"certificate-authority-data"`
			InsecureSkipTLSVerify    bool   `yaml:"insecure-skip-tls-verify"`
		} `yaml:"cluster"`
	} `yaml:"clusters"`
	Contexts []struct {
		Name    string `yaml:"name"`
		Context struct {
			Cluster string `yaml:"cluster"`
			User    string `yaml:"user"`
		} `yaml:"context"`
	} `yaml:"contexts"`
	Users []struct {
		Name string `yaml:"name"`
		User struct {
			Token                 string `yaml:"token"`
			Username              string `yaml:"username"`
			Password              string `yaml:"password"`
			ClientCertificateData string `yaml:"client-certificate-data"`
			ClientKeyData         string `yaml:"client-key-data"`
		} `yaml:"user"`
	} `yaml:"users"`
}

type kubeClusterRuntime struct {
	Server                string
	InsecureSkipTLSVerify bool
	CertificateAuthority  string
	Token                 string
	Username              string
	Password              string
	ClientCertificateData string
	ClientKeyData         string
}

type kubeVersionResponse struct {
	GitVersion string `json:"gitVersion"`
}

type kubeNodeListResponse struct {
	Items []kubeNode `json:"items"`
}

type kubeNamespaceListResponse struct {
	Items []kubeNamespace `json:"items"`
}

type kubePodListResponse struct {
	Items []kubePod `json:"items"`
}

type kubeServiceListResponse struct {
	Items []kubeService `json:"items"`
}

type kubeIngressListResponse struct {
	Items []kubeIngress `json:"items"`
}

type kubeConfigMapListResponse struct {
	Items []kubeConfigMap `json:"items"`
}

type kubeSecretListResponse struct {
	Items []kubeSecret `json:"items"`
}

type kubePVCListResponse struct {
	Items []kubePersistentVolumeClaim `json:"items"`
}

type kubePVListResponse struct {
	Items []kubePersistentVolume `json:"items"`
}

type kubeDeploymentListResponse struct {
	Items []kubeDeployment `json:"items"`
}

type kubeReplicaSetListResponse struct {
	Items []kubeReplicaSet `json:"items"`
}

type kubeStatefulSetListResponse struct {
	Items []kubeStatefulSet `json:"items"`
}

type kubeDaemonSetListResponse struct {
	Items []kubeDaemonSet `json:"items"`
}

type kubeJobListResponse struct {
	Items []kubeJob `json:"items"`
}

type kubeCronJobListResponse struct {
	Items []kubeCronJob `json:"items"`
}

type kubeEndpointListResponse struct {
	Items []kubeEndpoints `json:"items"`
}

type kubeMetadata struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	CreationTimestamp string            `json:"creationTimestamp"`
	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	OwnerReferences   []struct {
		Kind string `json:"kind"`
		Name string `json:"name"`
	} `json:"ownerReferences"`
}

type kubeNode struct {
	Metadata kubeMetadata `json:"metadata"`
	Spec     struct {
		Unschedulable bool     `json:"unschedulable"`
		PodCIDR       string   `json:"podCIDR"`
		PodCIDRs      []string `json:"podCIDRs"`
	} `json:"spec"`
	Status struct {
		NodeInfo struct {
			KubeletVersion          string `json:"kubeletVersion"`
			OSImage                 string `json:"osImage"`
			KernelVersion           string `json:"kernelVersion"`
			ContainerRuntimeVersion string `json:"containerRuntimeVersion"`
			Architecture            string `json:"architecture"`
		} `json:"nodeInfo"`
		Addresses []struct {
			Type    string `json:"type"`
			Address string `json:"address"`
		} `json:"addresses"`
		Conditions []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"conditions"`
		Capacity    map[string]string `json:"capacity"`
		Allocatable map[string]string `json:"allocatable"`
	} `json:"status"`
}

type kubeEnvVar struct {
	Name      string         `json:"name"`
	Value     string         `json:"value"`
	ValueFrom map[string]any `json:"valueFrom"`
}

type kubeContainer struct {
	Name            string       `json:"name"`
	Image           string       `json:"image"`
	ImagePullPolicy string       `json:"imagePullPolicy"`
	Env             []kubeEnvVar `json:"env"`
	Resources       struct {
		Requests map[string]string `json:"requests"`
		Limits   map[string]string `json:"limits"`
	} `json:"resources"`
}

type kubePod struct {
	Metadata kubeMetadata `json:"metadata"`
	Spec     struct {
		NodeName           string          `json:"nodeName"`
		ServiceAccountName string          `json:"serviceAccountName"`
		Containers         []kubeContainer `json:"containers"`
	} `json:"spec"`
	Status struct {
		Phase             string `json:"phase"`
		PodIP             string `json:"podIP"`
		HostIP            string `json:"hostIP"`
		QoSClass          string `json:"qosClass"`
		ContainerStatuses []struct {
			Name         string `json:"name"`
			RestartCount int    `json:"restartCount"`
			Ready        bool   `json:"ready"`
		} `json:"containerStatuses"`
	} `json:"status"`
}

type kubeReplicaSet struct {
	Metadata kubeMetadata `json:"metadata"`
	Spec     struct {
		Replicas *int           `json:"replicas"`
		Template map[string]any `json:"template"`
	} `json:"spec"`
	Status struct {
		ReadyReplicas     int `json:"readyReplicas"`
		AvailableReplicas int `json:"availableReplicas"`
	} `json:"status"`
}

type kubeNamespace struct {
	Metadata kubeMetadata `json:"metadata"`
	Status   struct {
		Phase string `json:"phase"`
	} `json:"status"`
}

type kubeService struct {
	Metadata kubeMetadata `json:"metadata"`
	Spec     struct {
		Type         string            `json:"type"`
		ClusterIP    string            `json:"clusterIP"`
		ExternalName string            `json:"externalName"`
		ExternalIPs  []string          `json:"externalIPs"`
		Selector     map[string]string `json:"selector"`
		Ports        []struct {
			Name       string      `json:"name"`
			Port       int         `json:"port"`
			NodePort   int         `json:"nodePort"`
			Protocol   string      `json:"protocol"`
			TargetPort interface{} `json:"targetPort"`
		} `json:"ports"`
	} `json:"spec"`
	Status struct {
		LoadBalancer struct {
			Ingress []struct {
				IP       string `json:"ip"`
				Hostname string `json:"hostname"`
			} `json:"ingress"`
		} `json:"loadBalancer"`
	} `json:"status"`
}

type kubeEndpoints struct {
	Metadata kubeMetadata `json:"metadata"`
	Subsets  []struct {
		Addresses         []struct{} `json:"addresses"`
		NotReadyAddresses []struct{} `json:"notReadyAddresses"`
	} `json:"subsets"`
}

type kubeIngress struct {
	Metadata kubeMetadata `json:"metadata"`
	Spec     struct {
		IngressClassName string `json:"ingressClassName"`
		Rules            []struct {
			Host string `json:"host"`
			HTTP struct {
				Paths []struct {
					Path    string `json:"path"`
					Backend struct {
						Service struct {
							Name string `json:"name"`
							Port struct {
								Number int    `json:"number"`
								Name   string `json:"name"`
							} `json:"port"`
						} `json:"service"`
					} `json:"backend"`
				} `json:"paths"`
			} `json:"http"`
		} `json:"rules"`
		TLS []struct{} `json:"tls"`
	} `json:"spec"`
	Status struct {
		LoadBalancer struct {
			Ingress []struct {
				IP       string `json:"ip"`
				Hostname string `json:"hostname"`
			} `json:"ingress"`
		} `json:"loadBalancer"`
	} `json:"status"`
}

type kubeConfigMap struct {
	Metadata kubeMetadata      `json:"metadata"`
	Data     map[string]string `json:"data"`
	Binary   map[string]string `json:"binaryData"`
}

type kubeSecret struct {
	Metadata kubeMetadata      `json:"metadata"`
	Type     string            `json:"type"`
	Data     map[string]string `json:"data"`
}

type kubePersistentVolumeClaim struct {
	Metadata kubeMetadata `json:"metadata"`
	Spec     struct {
		StorageClassName string   `json:"storageClassName"`
		AccessModes      []string `json:"accessModes"`
		Resources        struct {
			Requests map[string]string `json:"requests"`
		} `json:"resources"`
	} `json:"spec"`
	Status struct {
		Phase    string            `json:"phase"`
		Capacity map[string]string `json:"capacity"`
	} `json:"status"`
}

type kubePersistentVolume struct {
	Metadata kubeMetadata `json:"metadata"`
	Spec     struct {
		StorageClassName              string            `json:"storageClassName"`
		Capacity                      map[string]string `json:"capacity"`
		AccessModes                   []string          `json:"accessModes"`
		PersistentVolumeReclaimPolicy string            `json:"persistentVolumeReclaimPolicy"`
		HostPath                      *struct {
			Path string `json:"path"`
		} `json:"hostPath"`
		NFS *struct {
			Server string `json:"server"`
			Path   string `json:"path"`
		} `json:"nfs"`
	} `json:"spec"`
	Status struct {
		Phase    string            `json:"phase"`
		Capacity map[string]string `json:"capacity"`
	} `json:"status"`
}

type kubeDeployment struct {
	Metadata kubeMetadata `json:"metadata"`
	Spec     struct {
		Replicas *int `json:"replicas"`
		Selector struct {
			MatchLabels map[string]string `json:"matchLabels"`
		} `json:"selector"`
		Template struct {
			Spec struct {
				Containers []kubeContainer `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
	Status struct {
		UpdatedReplicas   int `json:"updatedReplicas"`
		ReadyReplicas     int `json:"readyReplicas"`
		AvailableReplicas int `json:"availableReplicas"`
	} `json:"status"`
}

type kubeStatefulSet struct {
	Metadata kubeMetadata `json:"metadata"`
	Spec     struct {
		Replicas *int `json:"replicas"`
		Selector struct {
			MatchLabels map[string]string `json:"matchLabels"`
		} `json:"selector"`
		Template struct {
			Spec struct {
				Containers []kubeContainer `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
	Status struct {
		UpdatedReplicas   int `json:"updatedReplicas"`
		ReadyReplicas     int `json:"readyReplicas"`
		AvailableReplicas int `json:"availableReplicas"`
	} `json:"status"`
}

type kubeDaemonSet struct {
	Metadata kubeMetadata `json:"metadata"`
	Spec     struct {
		Selector struct {
			MatchLabels map[string]string `json:"matchLabels"`
		} `json:"selector"`
		Template struct {
			Spec struct {
				Containers []kubeContainer `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
	Status struct {
		DesiredNumberScheduled int `json:"desiredNumberScheduled"`
		UpdatedNumberScheduled int `json:"updatedNumberScheduled"`
		NumberReady            int `json:"numberReady"`
		NumberAvailable        int `json:"numberAvailable"`
	} `json:"status"`
}

type kubeJob struct {
	Metadata kubeMetadata `json:"metadata"`
	Spec     struct {
		Completions *int `json:"completions"`
		Selector    *struct {
			MatchLabels map[string]string `json:"matchLabels"`
		} `json:"selector"`
		Template struct {
			Spec struct {
				Containers []kubeContainer `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
	Status struct {
		Active    int `json:"active"`
		Succeeded int `json:"succeeded"`
		Failed    int `json:"failed"`
		Ready     int `json:"ready"`
	} `json:"status"`
}

type kubeCronJob struct {
	Metadata kubeMetadata `json:"metadata"`
	Spec     struct {
		Schedule    string `json:"schedule"`
		Suspend     *bool  `json:"suspend"`
		JobTemplate struct {
			Spec struct {
				Template struct {
					Spec struct {
						Containers []kubeContainer `json:"containers"`
					} `json:"spec"`
				} `json:"template"`
			} `json:"spec"`
		} `json:"jobTemplate"`
	} `json:"spec"`
	Status struct {
		Active []struct{} `json:"active"`
	} `json:"status"`
}
