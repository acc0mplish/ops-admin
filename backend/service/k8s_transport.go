// k8s_transport.go — moved verbatim from k8s.go (Phase D2, E5 seam #11).
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
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ops-admin/backend/model"

	"gopkg.in/yaml.v3"
)

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
