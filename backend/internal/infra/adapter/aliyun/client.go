package aliyun

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"ops-admin/backend/internal/infra/contract"
	"ops-admin/backend/internal/infra/metrics"
)

// Aliyun ECS raw RPC client — service/asset_cloud_aliyun.go(v1)의 RPC 규약
// (HMAC-SHA1·SignatureNonce·Version 2014-05-26)을 어댑터 결계 안으로 재구현한
// 것이다(판정 J2 — adapter→service import는 arch rule 2 위반이므로 재구현).
// 엔드포인트는 req.Connection.Endpoint가 우선이고(J1 Path M — mock 주입면),
// 빈 값이면 프로바이더 기본 ecs.<region>.aliyuncs.com(리전 조합)을 쓴다.
//
// 자격 평문(accessKey·secretKey)은 이 구조체의 필드로 요청 조립에만 쓰이고
// 로그·에러 메시지·Raw 페이로드 어디에도 기록되지 않는다(보존 제약 3).

const (
	aliyunECSAPIVersion = "2014-05-26"
	// defaultAPIEndpoint is the region-less ECS endpoint legacy v1 uses
	// (asset_cloud_aliyun.go aliyunECSAPIEndpoint).
	defaultAPIEndpoint = "https://ecs.aliyuncs.com/"
	requestTimeout     = 8 * time.Second
	responseLimit      = 4 << 20
)

// credential is the parsed Material["inventory"] JSON blob {accessKey,secretKey}
// (가정 A4 — SecretRef 재질).
type credential struct {
	accessKey string
	secretKey string
}

// resolveCredential parses the connection material — the J2 path: the adapter
// reads req.Connection only.
func resolveCredential(conn contract.ConnectionView) (credential, error) {
	raw := ""
	if conn.Material != nil {
		raw = strings.TrimSpace(conn.Material["inventory"])
	}
	if raw == "" {
		return credential{}, fmt.Errorf("aliyun: missing credential material Material[\"inventory\"] (broker purpose \"inventory\" resolve — J2)")
	}
	var blob struct {
		AccessKey string `json:"accessKey"`
		SecretKey string `json:"secretKey"`
	}
	if err := json.Unmarshal([]byte(raw), &blob); err != nil {
		return credential{}, fmt.Errorf("aliyun: parse credential material: %w", err)
	}
	cred := credential{
		accessKey: strings.TrimSpace(blob.AccessKey),
		secretKey: strings.TrimSpace(blob.SecretKey),
	}
	if cred.accessKey == "" || cred.secretKey == "" {
		return credential{}, fmt.Errorf("aliyun: credential material is missing accessKey or secretKey")
	}
	return cred, nil
}

// ecsClient signs and executes one RPC call at a time — HTTP clients are
// per-request (stateless credentials via req.Connection, arch rule 2).
type ecsClient struct {
	cred     credential
	endpoint func(region string) string
	metrics  *metrics.Counters
	http     *http.Client
}

// endpointFor resolves the request host: req.Connection.Endpoint wins (mock
// 주입면 — 판정 A3), then the adapter-level override, then the regional ECS
// default.
func (a *Adapter) endpointFor(conn contract.ConnectionView, region string) string {
	if base := strings.TrimSpace(conn.Endpoint); base != "" {
		return strings.TrimRight(base, "/")
	}
	if a.endpointOverride != nil {
		if base := strings.TrimSpace(a.endpointOverride()); base != "" {
			return strings.TrimRight(base, "/")
		}
	}
	region = strings.TrimSpace(region)
	if region == "" {
		return defaultAPIEndpoint
	}
	return "https://ecs." + region + ".aliyuncs.com/"
}

func (a *Adapter) clientFor(conn contract.ConnectionView, cred credential) *ecsClient {
	return &ecsClient{
		cred:     cred,
		endpoint: func(region string) string { return a.endpointFor(conn, region) },
		metrics:  a.metrics,
		http:     &http.Client{Timeout: requestTimeout},
	}
}

// aliyunRPCError is the error envelope of the RPC protocol (legacy
// aliyunRPCError).
type aliyunRPCError struct {
	Code    string `json:"Code"`
	Message string `json:"Message"`
}

// request executes one signed GET and decodes the JSON envelope. Signal
// mapping (§9.3 — 계획 §3.1, k8s client의 분류와 동일 어휘):
//
//	429 / Throttling*                 → rate_limited   (계측: IncRateLimit)
//	401/403 / 권한·서명 코드          → permission_denied (계측: IncAPIError)
//	transport 실패                    → unreachable    (계측: IncAPIError)
//	기타                              → 일반 error     (계측: IncAPIError)
//
// 에러 메시지는 status·코드·action만 담는다 — 자격 값·서명 파라미터·응답
// 본문 전문은 들어가지 않는다(보존 제약 3).
func (c *ecsClient) request(ctx context.Context, action, region, op string, extra url.Values, target any) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	params := url.Values{}
	params.Set("Format", "JSON")
	params.Set("Version", aliyunECSAPIVersion)
	params.Set("AccessKeyId", c.cred.accessKey)
	params.Set("SignatureMethod", "HMAC-SHA1")
	params.Set("Timestamp", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	params.Set("SignatureVersion", "1.0")
	params.Set("SignatureNonce", fmt.Sprintf("ops-admin-%d", time.Now().UnixNano()))
	params.Set("Action", action)
	if strings.TrimSpace(region) != "" {
		params.Set("RegionId", strings.TrimSpace(region))
	}
	for key, values := range extra {
		for _, value := range values {
			params.Add(key, value)
		}
	}
	params.Set("Signature", signParams(params, c.cred.secretKey))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint(region)+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("aliyun: request build %s: %w", action, err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.metrics.IncAPIError(ProviderName, op, "transport")
		return &contract.ProviderSignalError{Kind: contract.SignalUnreachable, Message: "aliyun ECS endpoint unreachable"}
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, responseLimit))
	apiErr := aliyunRPCError{}
	_ = json.Unmarshal(body, &apiErr) // error envelope is best-effort (legacy 동일)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 || apiErr.Code != "" {
		return c.signalError(action, region, op, resp.StatusCode, apiErr)
	}
	if readErr != nil {
		return fmt.Errorf("aliyun: read %s response: %w", action, readErr)
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("aliyun: decode %s response: %w", action, err)
	}
	return nil
}

// signalError classifies a failed RPC into the §9.3 signal vocabulary or a
// plain error, incrementing the matching counters (k8s client 분류와 1:1).
func (c *ecsClient) signalError(action, region, op string, status int, apiErr aliyunRPCError) error {
	code := strings.TrimSpace(apiErr.Code)
	switch {
	case status == http.StatusTooManyRequests || strings.HasPrefix(code, "Throttling"):
		c.metrics.IncRateLimit(ProviderName)
		return &contract.ProviderSignalError{
			Kind:    contract.SignalRateLimited,
			Message: fmt.Sprintf("aliyun: ECS %s throttled (status %d, code %s)", action, status, code),
		}
	case status == http.StatusUnauthorized || status == http.StatusForbidden,
		code == "InvalidAccessKeyId.NotFound",
		code == "SignatureDoesNotMatch",
		code == "ForbiddenAccess",
		code == "Forbidden.RAM",
		code == "InvalidAccessKeyId":
		c.metrics.IncAPIError(ProviderName, op, code)
		return &contract.ProviderSignalError{
			Kind:    contract.SignalPermissionDenied,
			Message: fmt.Sprintf("aliyun: ECS %s permission denied (status %d, code %s)", action, status, code),
		}
	}
	c.metrics.IncAPIError(ProviderName, op, code)
	label := code
	if label == "" {
		label = strconv.Itoa(status)
	}
	return fmt.Errorf("aliyun: ECS %s failed for %s: %s", action, region, label)
}

// describeRegions lists the account-visible region IDs (legacy
// DescribeRegions 폴백 — 판정 J3).
func (c *ecsClient) describeRegions(ctx context.Context, op string) ([]string, error) {
	var response struct {
		Regions struct {
			Region []struct {
				RegionID string `json:"RegionId"`
			} `json:"Region"`
		} `json:"Regions"`
	}
	if err := c.request(ctx, "DescribeRegions", "", op, nil, &response); err != nil {
		return nil, fmt.Errorf("aliyun: unable to discover regions: %w", err)
	}
	regions := make([]string, 0, len(response.Regions.Region))
	for _, item := range response.Regions.Region {
		if strings.TrimSpace(item.RegionID) != "" {
			regions = append(regions, strings.TrimSpace(item.RegionID))
		}
	}
	return regions, nil
}

// describeInstances fetches one page of the region's ECS instances. Termination
// follows the legacy rule: stop when the batch is empty or the page cursor has
// reached TotalCount.
func (c *ecsClient) describeInstances(ctx context.Context, region string, page, size int) ([]aliyunECSInstance, int, error) {
	extra := url.Values{}
	extra.Set("PageNumber", strconv.Itoa(page))
	extra.Set("PageSize", strconv.Itoa(size))
	var response struct {
		TotalCount int `json:"TotalCount"`
		Instances  struct {
			Instance []aliyunECSInstance `json:"Instance"`
		} `json:"Instances"`
	}
	if err := c.request(ctx, "DescribeInstances", region, "discover", extra, &response); err != nil {
		return nil, 0, fmt.Errorf("aliyun: ECS instance query failed for %s: %w", region, err)
	}
	return response.Instances.Instance, response.TotalCount, nil
}

// --- 서명 — legacy finOpsAliCloudSignature와 동일 RPC 규약(재구현, J2). ---

// signParams computes the HMAC-SHA1 RPC signature over the sorted,
// RFC3986-encoded parameter set (the Signature parameter itself excluded).
func signParams(params url.Values, secretKey string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, aliyunEncode(key)+"="+aliyunEncode(params.Get(key)))
	}
	stringToSign := "GET&%2F&" + aliyunEncode(strings.Join(pairs, "&"))
	mac := hmac.New(sha1.New, []byte(secretKey+"&"))
	_, _ = mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// aliyunEncode is the RFC3986 percent-encoding the RPC signature requires
// (QueryEscape plus '+' → '%20').
func aliyunEncode(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}
