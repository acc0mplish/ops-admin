// Package tencent is the V2 read-only Tencent Cloud adapter (Phase 4 PR 28,
// 계획 §2 Phase B·판정 J2(c)). The transport is an in-tree TC3-HMAC-SHA256
// POST client — NOT an SDK import: the endpoint and scheme must be fully
// injectable for the mock path (J1 Path M — Connection.Endpoint), and the
// future family extension (cbs/vpc — E-3 이월분) stays a single client
// (§10.2 신규 의존 금지 — 보존 제약 4). The signature core (tc3Authorization)
// carries the same convention the in-tree finops precedent
// (service/finops_cloud_billing.go finOpsTencentRequest) already proves —
// 가정 A6: 표준 헤더 X-TC-Action/Version/Timestamp/Region.
//
// The adapter owns no control-plane tables and no DB handle (arch rule 2).
// Credentials arrive only via DiscoverRequest.Connection.Material["inventory"]
// (broker-resolved, purpose "inventory" — J2, JSON blob {"accessKey","secretKey"}).
package tencent

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/metrics"
)

const (
	// defaultHost — CVM API 기본 엔드포인트(Connection.Endpoint 비면).
	defaultHost = "cvm.tencentcloudapi.com"
	// apiVersion — CVM 2017-03-12 (legacy util/tencentcloud.go cvm/v20170312
	// SDK 모듈과 동일 API 버전).
	apiVersion = "2017-03-12"
	// service — TC3 credential scope 서명 성분.
	tc3Service = "cvm"
	// requestTimeout — legacy SDK 호출과 동일 의미론의 1요청 상한.
	requestTimeout = 8 * time.Second
	// timestampSkew — TC3 서명 유효 시계 규약(Tencent 공식 5분).
	timestampSkew = 5 * time.Minute
	// contentType — 서명에 들어가는 콘텐츠 타입(finops 선례와 동일 문자열).
	contentType = "application/json; charset=utf-8"
)

// credential is the decrypted inventory material ({"accessKey","secretKey"}).
type credential struct {
	accessKey string
	secretKey string
}

// cvmClient is one region's CVM API client — per-request HTTP clients, no
// long-lived provider state (stateless credentials via req.Connection).
type cvmClient struct {
	http       *http.Client
	credential credential
	endpoint   string // full base URL (scheme + host[:port])
	region     string
	metrics    *metrics.Counters
	// timestamp is the clock seam (테스트의 스크우 음성 제어 — T-B9). nil이면
	// 실제 시계다.
	timestamp func() int64
}

// newCVMClient builds the region client over a resolved endpoint
// (normalizeEndpoint 산물).
func newCVMClient(cred credential, endpoint, region string, m *metrics.Counters) *cvmClient {
	return &cvmClient{
		http:       &http.Client{Timeout: requestTimeout},
		credential: cred,
		endpoint:   endpoint,
		region:     region,
		metrics:    m,
	}
}

// hmacSHA256 — finops finOpsHMAC과 동일 헬퍼(선례 승계).
func hmacSHA256(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}

// tc3Authorization is the TC3-HMAC-SHA256 signature core in its reusable form
// (판정 J2(c)·계획 F9 완화 — 서명 코어의 재사용 형태). Canonical URI is "/"
// with no query — the only shape this client issues; SignedHeaders are fixed
// to content-type;host (finops finOpsTencentRequest 규약 승계 — A6). The
// contracttest mock re-derives the same chain independently and compares.
func tc3Authorization(accessKey, secretKey, host, ct string, timestamp int64, service string, payload []byte) string {
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")
	payloadHash := sha256.Sum256(payload)
	canonicalRequest := "POST\n/\n\ncontent-type:" + ct + "\nhost:" + host + "\n\ncontent-type;host\n" + hex.EncodeToString(payloadHash[:])
	hashedCanonical := sha256.Sum256([]byte(canonicalRequest))
	credentialScope := date + "/" + service + "/tc3_request"
	stringToSign := "TC3-HMAC-SHA256\n" + strconv.FormatInt(timestamp, 10) + "\n" + credentialScope + "\n" + hex.EncodeToString(hashedCanonical[:])
	secretDate := hmacSHA256([]byte("TC3"+secretKey), date)
	secretService := hmacSHA256(secretDate, service)
	secretSigning := hmacSHA256(secretService, "tc3_request")
	signature := hex.EncodeToString(hmacSHA256(secretSigning, stringToSign))
	return "TC3-HMAC-SHA256 Credential=" + accessKey + "/" + credentialScope +
		", SignedHeaders=content-type;host, Signature=" + signature
}

// normalizeEndpoint resolves the request endpoint: Connection.Endpoint wins,
// an empty value falls back to the provider default; a schemeless value gets
// https:// prefixed (계획 §3.1 — mock 경로는 http 스킴을 그대로 수용).
func normalizeEndpoint(endpoint string) string {
	endpoint = trimSpace(endpoint)
	if endpoint == "" {
		return "https://" + defaultHost
	}
	if !containsScheme(endpoint) {
		return "https://" + endpoint
	}
	return endpoint
}

func containsScheme(endpoint string) bool {
	for i := 0; i+2 < len(endpoint); i++ {
		if endpoint[i] == ':' && endpoint[i+1] == '/' && endpoint[i+2] == '/' {
			return true
		}
	}
	return false
}

// describeInstancesRequest/Response — CVM DescribeInstances 와이어 형상
// (legacy SDK cvm/v20170312와 동일 필드). InstanceSet 디코딩은
// normalizer.go의 nil-safe cvmInstance다.
type describeInstancesRequest struct {
	Offset int64 `json:"Offset"`
	Limit  int64 `json:"Limit"`
}

// tencentAPIError — Tencent 공통 응답 오류 봉투({"Response":{"Error":{...}}}).
type tencentAPIError struct {
	Code    string `json:"Code"`
	Message string `json:"Message"`
}

type describeInstancesResponse struct {
	Response struct {
		TotalCount  int64            `json:"TotalCount"`
		InstanceSet []cvmInstance    `json:"InstanceSet"`
		Error       *tencentAPIError `json:"Error"`
		RequestId   string           `json:"RequestId"`
	} `json:"Response"`
}

// describeIssues — one DescribeInstances page.
func (c *cvmClient) describeInstances(ctx context.Context, offset, limit int64) (describeInstancesResponse, error) {
	var page describeInstancesResponse
	if err := c.do(ctx, "DescribeInstances", describeInstancesRequest{Offset: offset, Limit: limit}, &page); err != nil {
		return describeInstancesResponse{}, err
	}
	return page, nil
}

// do performs one signed POST. Provider-side states surface as
// contract.ProviderSignalError kinds (§9.3 매핑 — 계획 §3.1):
//
//	429 / RequestLimitExceeded·Throttling → rate_limited (계측: IncRateLimit)
//	401/403 / AuthFailure·UnauthorizedOperation·AccessDenied → permission_denied
//	transport fail                                        → unreachable
//	기타 비신호 코드·상태                                   → 일반 error
//
// 어느 경로의 에러 메시지에도 자격 물질은 진입하지 않는다(보존 제약 3).
func (c *cvmClient) do(ctx context.Context, action string, body any, target any) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("tencent: %s request encode: %w", action, err)
	}
	base, err := url.Parse(c.endpoint)
	if err != nil {
		return fmt.Errorf("tencent: endpoint parse: %w", err)
	}

	timestamp := c.now()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("tencent: %s request build: %w", action, err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", tc3Authorization(
		c.credential.accessKey, c.credential.secretKey,
		base.Host, contentType, timestamp, tc3Service, payload))
	req.Header.Set("X-TC-Action", action)
	req.Header.Set("X-TC-Version", apiVersion)
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(timestamp, 10))
	if c.region != "" {
		req.Header.Set("X-TC-Region", c.region)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		c.metrics.IncAPIError(ProviderName, "discover", "transport")
		// transport error = unreachable — 자격 물질 없는 신호 에러.
		return &contract.ProviderSignalError{
			Kind:    contract.SignalUnreachable,
			Message: "tencent API unreachable",
		}
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// 신호 분류는 상태 코드 우선 — 본문 봉투 오류 코드가 있으면 보강한다.
		var envelope describeInstancesResponse
		code := ""
		if json.Unmarshal(respBody, &envelope) == nil && envelope.Response.Error != nil {
			code = envelope.Response.Error.Code
		}
		return c.classify(action, resp.StatusCode, code)
	}

	if target == nil {
		return nil
	}
	if err := json.Unmarshal(respBody, target); err != nil {
		return fmt.Errorf("tencent: %s response decode: %w", action, err)
	}
	return nil
}

// classify maps a provider-side failure to the closed §9.3 signal vocabulary
// or a plain error, instrumenting the §18.2 counter lines either way.
func (c *cvmClient) classify(action string, status int, code string) error {
	switch {
	case status == http.StatusTooManyRequests ||
		hasAnyPrefix(code, "RequestLimitExceeded", "Throttling"):
		c.metrics.IncRateLimit(ProviderName)
		return &contract.ProviderSignalError{
			Kind:    contract.SignalRateLimited,
			Message: fmt.Sprintf("tencent %s rate limited (status %d)", action, status),
		}
	case status == http.StatusUnauthorized || status == http.StatusForbidden ||
		hasAnyPrefix(code, "AuthFailure", "UnauthorizedOperation", "AccessDenied"):
		label := code
		if label == "" {
			label = strconv.Itoa(status)
		}
		c.metrics.IncAPIError(ProviderName, "discover", label)
		return &contract.ProviderSignalError{
			Kind:    contract.SignalPermissionDenied,
			Message: fmt.Sprintf("tencent %s permission denied (%s)", action, label),
		}
	}
	label := code
	if label == "" {
		label = strconv.Itoa(status)
	}
	c.metrics.IncAPIError(ProviderName, "discover", label)
	return fmt.Errorf("tencent: %s failed (status %d, code %s)", action, status, label)
}

func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if len(s) >= len(p) && s[:len(p)] == p {
			return true
		}
	}
	return false
}

// trimSpace — strings.TrimSpace 위임(클라이언트 파일 스코프 헬퍼).
func trimSpace(s string) string { return strings.TrimSpace(s) }

// now resolves the signing clock (테스트 주입면 — nil이면 실제 시계).
func (c *cvmClient) now() int64 {
	if c.timestamp != nil {
		return c.timestamp()
	}
	return time.Now().Unix()
}
