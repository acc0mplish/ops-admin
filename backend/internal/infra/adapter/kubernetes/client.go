// Package kubernetes is the V2 read-only K8s/K3s adapter (Phase 2 PR 20,
// 계획 §3.1). The transport is a faithful REIMPLEMENTATION of the legacy
// semantics in service/k8s.go (kubeClusterRuntime / parseKubeConfig /
// newK8sHTTPClientWithDial / k8sGetJSON) — not an import: adapters must not
// reach into the God service (arch rule 2, 계획 J1). Equivalence is what the
// §15 compare protocol proves; consolidation lands with M2 decomposition
// step 2 (§13 트랜스포트 중복 통합).
//
// The adapter owns no control-plane tables and no DB handle (arch rule 2).
// Credentials arrive only via DiscoverRequest.Connection.Material["inventory"]
// (broker-resolved, purpose "inventory" — J2).
package kubernetes

import (
	"bytes"
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
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/metrics"
)

// requestTimeout — legacy k8sDoJSON의 8초 타임아웃과 동일 의미론.
const requestTimeout = 8 * time.Second

// clusterRuntime is the parsed kubeconfig credential set — legacy
// kubeClusterRuntime과 동일 필드·의미론.
type clusterRuntime struct {
	Server                string
	InsecureSkipTLSVerify bool
	CertificateAuthority  string
	Token                 string
	Username              string
	Password              string
	ClientCertificateData string
	ClientKeyData         string
}

// kubeConfig mirrors the kubeconfig YAML subset the legacy parser reads.
type kubeConfig struct {
	CurrentContext string `yaml:"current-context"`
	Clusters       []struct {
		Name    string `yaml:"name"`
		Cluster struct {
			Server                   string `yaml:"server"`
			InsecureSkipTLSVerify    bool   `yaml:"insecure-skip-tls-verify"`
			CertificateAuthorityData string `yaml:"certificate-authority-data"`
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

// parseKubeConfig resolves the current-context (or the first context when
// unset) into a clusterRuntime — legacy parseKubeConfig와 동일 의미론.
func parseKubeConfig(content string) (clusterRuntime, error) {
	var cfg kubeConfig
	if err := yaml.Unmarshal([]byte(content), &cfg); err != nil {
		return clusterRuntime{}, fmt.Errorf("kubeconfig parse: %w", err)
	}

	contextName := strings.TrimSpace(cfg.CurrentContext)
	if contextName == "" && len(cfg.Contexts) > 0 {
		contextName = cfg.Contexts[0].Name
	}
	if contextName == "" {
		return clusterRuntime{}, errors.New("kubeconfig: missing context")
	}

	var clusterName, userName string
	for i := range cfg.Contexts {
		if cfg.Contexts[i].Name == contextName {
			clusterName = strings.TrimSpace(cfg.Contexts[i].Context.Cluster)
			userName = strings.TrimSpace(cfg.Contexts[i].Context.User)
			break
		}
	}
	if clusterName == "" {
		return clusterRuntime{}, errors.New("kubeconfig: cluster not found")
	}

	rt := clusterRuntime{}
	for i := range cfg.Clusters {
		if cfg.Clusters[i].Name == clusterName {
			rt.Server = strings.TrimSpace(cfg.Clusters[i].Cluster.Server)
			rt.InsecureSkipTLSVerify = cfg.Clusters[i].Cluster.InsecureSkipTLSVerify
			rt.CertificateAuthority = strings.TrimSpace(cfg.Clusters[i].Cluster.CertificateAuthorityData)
			break
		}
	}
	if rt.Server == "" {
		return clusterRuntime{}, errors.New("kubeconfig: server not found")
	}

	for i := range cfg.Users {
		if cfg.Users[i].Name == userName {
			rt.Token = strings.TrimSpace(cfg.Users[i].User.Token)
			rt.Username = strings.TrimSpace(cfg.Users[i].User.Username)
			rt.Password = strings.TrimSpace(cfg.Users[i].User.Password)
			rt.ClientCertificateData = strings.TrimSpace(cfg.Users[i].User.ClientCertificateData)
			rt.ClientKeyData = strings.TrimSpace(cfg.Users[i].User.ClientKeyData)
			break
		}
	}

	return rt, nil
}

// newK8sHTTPClient builds the REST client — legacy newK8sHTTPClientWithDial과
// 동일 의미론(TLS 1.2 최소·CA 풀·클라이언트 인증서·dial 주입). dialContext는
// 게이트웨이 홉(A4 — 주입면, 실환경 게이트웨이 전송은 M2)이다.
func newK8sHTTPClient(rt clusterRuntime, dialContext func(ctx context.Context, network, addr string) (net.Conn, error)) (*http.Client, error) {
	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: rt.InsecureSkipTLSVerify,
	}

	if rt.CertificateAuthority != "" {
		caBytes, err := base64.StdEncoding.DecodeString(rt.CertificateAuthority)
		if err != nil {
			return nil, fmt.Errorf("kubeconfig: certificate-authority-data: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caBytes) {
			return nil, errors.New("kubeconfig: invalid certificate authority")
		}
		tlsConfig.RootCAs = pool
	}

	if rt.ClientCertificateData != "" && rt.ClientKeyData != "" {
		certBytes, err := base64.StdEncoding.DecodeString(rt.ClientCertificateData)
		if err != nil {
			return nil, fmt.Errorf("kubeconfig: client-certificate-data: %w", err)
		}
		keyBytes, err := base64.StdEncoding.DecodeString(rt.ClientKeyData)
		if err != nil {
			return nil, fmt.Errorf("kubeconfig: client-key-data: %w", err)
		}
		cert, err := tls.X509KeyPair(certBytes, keyBytes)
		if err != nil {
			return nil, fmt.Errorf("kubeconfig: client key pair: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	transport := &http.Transport{TLSClientConfig: tlsConfig}
	if dialContext != nil {
		transport.DialContext = dialContext
	}
	return &http.Client{Timeout: requestTimeout, Transport: transport}, nil
}

// k8sClient is the 429-instrumented REST wrapper (J6 — 어댑터 REST 래퍼가
// rate-limit 신호를 계측 카운터로 기록한다).
type k8sClient struct {
	http    *http.Client
	rt      clusterRuntime
	metrics *metrics.Counters
}

func newK8sClient(rt clusterRuntime, dialContext func(ctx context.Context, network, addr string) (net.Conn, error), m *metrics.Counters) (*k8sClient, error) {
	hc, err := newK8sHTTPClient(rt, dialContext)
	if err != nil {
		return nil, err
	}
	return &k8sClient{http: hc, rt: rt, metrics: m}, nil
}

// errNotFound marks a provider-side 404 — doJSON이 404를 식별 가능한 형태로
// 반환하게 한 최소 수정(J-P1-1). GatewayAPI 섹션의 버전 폴백은 이 센티넬로만
// "해당 API 버전 부재"를 판정하고(404는 섹션 스킵 신호), 401/403/429/전송은
// 기존 ProviderSignalError 분류를 그대로 유지한다. errors.Is로만 소비한다.
var errNotFound = errors.New("kubernetes: not found (404)")

// getJSON fetches one JSON endpoint on the discovery path — the metrics op
// label stays "discover" (기존 계약 무변경, Phase 2 PR 20). Signal mapping lives
// in doJSON.
func (c *k8sClient) getJSON(ctx context.Context, path string, query map[string]string, target any) error {
	return c.doJSON(ctx, http.MethodGet, path, query, "", nil, "discover", target)
}

// getJSONOp — getJSON with an explicit §18.2 metrics op label. The executor's
// poll path labels its own op so provider_api_errors_total distinguishes
// mutation-path cycles from discovery.
func (c *k8sClient) getJSONOp(ctx context.Context, path string, query map[string]string, op string, target any) error {
	if op == "" {
		op = "discover"
	}
	return c.doJSON(ctx, http.MethodGet, path, query, "", nil, op, target)
}

// patchJSON issues one PATCH with the Kubernetes strategic-merge-patch content
// type (Phase 3 B / M3) — v1 RestartK8sWorkload가 워크로드 3종에 쓰던 것과
// 동일한 patch 방식(R9 — 경로 3종은 v1이 이미 사용 중). `patch`는 구조체로
// 전달한다: json.Marshal의 필드 순서는 선언 순으로 고정되므로 동일 입력의
// 재실행은 byte-identical 본문이 된다(J1 멱등의 전송 계약).
func (c *k8sClient) patchJSON(ctx context.Context, path string, patch any, op string, target any) error {
	if op == "" {
		op = "execute"
	}
	return c.doJSON(ctx, http.MethodPatch, path, nil, "application/strategic-merge-patch+json", patch, op, target)
}

// doJSON performs one REST call. Provider-side states surface as
// contract.ProviderSignalError kinds (§9.3 매핑 — 계획 §3.3):
//
//	429            → rate_limited (계측: IncRateLimit)
//	401/403        → permission_denied (계측: IncAPIError)
//	transport fail → unreachable (계측: IncAPIError)
//	404            → errNotFound (계측: IncAPIError — 버전 폴백·섹션 스킵 소비자,
//	                 J-P1-1. 신호 분류는 기존 그대로다)
//	기타 비-2xx     → 일반 error (계측: IncAPIError)
func (c *k8sClient) doJSON(ctx context.Context, method, path string, query map[string]string, contentType string, body any, op string, target any) error {
	start := time.Now()
	defer func() { c.metrics.ObserveAPILatency(ProviderName, op, time.Since(start)) }()
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := strings.TrimRight(c.rt.Server, "/") + path
	if len(query) > 0 {
		values := url.Values{}
		for k, v := range query {
			values.Set(k, v)
		}
		endpoint += "?" + values.Encode()
	}

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("kubernetes: request encode %s %s: %w", method, path, err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return fmt.Errorf("kubernetes: request build: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.rt.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.rt.Token)
	}
	if c.rt.Username != "" || c.rt.Password != "" {
		req.SetBasicAuth(c.rt.Username, c.rt.Password)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		c.metrics.IncAPIError(ProviderName, op, "transport")
		// transport error = unreachable — 자격 물질 없는 신호 에러.
		return &contract.ProviderSignalError{
			Kind:    contract.SignalUnreachable,
			Message: "cluster API unreachable",
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		snippet := strings.TrimSpace(string(respBody))
		switch resp.StatusCode {
		case http.StatusTooManyRequests:
			c.metrics.IncRateLimit(ProviderName)
			return &contract.ProviderSignalError{
				Kind:    contract.SignalRateLimited,
				Message: fmt.Sprintf("rate limited (status %d)", resp.StatusCode),
			}
		case http.StatusUnauthorized, http.StatusForbidden:
			c.metrics.IncAPIError(ProviderName, op, strconv.Itoa(resp.StatusCode))
			return &contract.ProviderSignalError{
				Kind:    contract.SignalPermissionDenied,
				Message: fmt.Sprintf("permission denied (status %d)", resp.StatusCode),
			}
		case http.StatusNotFound:
			// 404는 섹션 부재 신호다(J-P1-1 — GatewayAPI CRD 미설치 클러스터의
			// 버전 폴백 판정). 계측은 기존 "기타 비-2xx" 경로와 동일하게 유지해
			// 최소 수정을 지킨다 — 소비자가 errors.Is로 스킵을 판정한다.
			c.metrics.IncAPIError(ProviderName, op, strconv.Itoa(resp.StatusCode))
			return errNotFound
		}
		c.metrics.IncAPIError(ProviderName, op, strconv.Itoa(resp.StatusCode))
		if snippet != "" {
			return fmt.Errorf("kubernetes: unexpected status: %d, %s", resp.StatusCode, snippet)
		}
		return fmt.Errorf("kubernetes: unexpected status: %d", resp.StatusCode)
	}

	if target == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("kubernetes: decode %s: %w", path, err)
	}
	return nil
}
