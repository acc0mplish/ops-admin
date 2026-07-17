package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"ops-admin/backend/model"
)

const finOpsBillPageSize = 300

// fetchFinOpsCosts selects the official billing API for supported providers.
// A custom HTTP endpoint remains available for adapters that return {"records": []}.
func (s *Service) fetchFinOpsCosts(ctx context.Context, account model.IntegrationFinOpsAccount, maxPages int) ([]FinOpsCostInput, error) {
	switch strings.ToLower(strings.TrimSpace(account.Provider)) {
	case "alicloud":
		return s.fetchAliCloudBill(ctx, account, maxPages)
	case "tencent":
		return s.fetchTencentBill(ctx, account, maxPages)
	default:
		return s.fetchCustomFinOpsBill(ctx, account)
	}
}

func finOpsBillingSource(provider string) string {
	switch strings.ToLower(provider) {
	case "alicloud":
		return "阿里云官方账单 API"
	case "tencent":
		return "腾讯云官方账单 API"
	default:
		return "自定义账单接口"
	}
}

func (s *Service) fetchCustomFinOpsBill(ctx context.Context, account model.IntegrationFinOpsAccount) ([]FinOpsCostInput, error) {
	endpoint := strings.TrimSpace(account.BillingEndpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("%s尚未配置账单 HTTP 地址", finOpsBillingSource(account.Provider))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if account.BillingToken != "" {
		req.Header.Set("Authorization", "Bearer "+account.BillingToken)
	}
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("账单接口返回 HTTP %d", resp.StatusCode)
	}
	var wrapper struct {
		Records []FinOpsCostInput `json:"records"`
	}
	if err = json.Unmarshal(body, &wrapper); err != nil || wrapper.Records == nil {
		if err = json.Unmarshal(body, &wrapper.Records); err != nil {
			return nil, fmt.Errorf("账单接口数据格式无效: %w", err)
		}
	}
	return wrapper.Records, nil
}

func (s *Service) fetchAliCloudBill(ctx context.Context, account model.IntegrationFinOpsAccount, maxPages int) ([]FinOpsCostInput, error) {
	if strings.TrimSpace(account.AccessKey) == "" || strings.TrimSpace(account.SecretKey) == "" {
		return nil, fmt.Errorf("阿里云账单同步需要 AccessKey 与 SecretKey")
	}
	if maxPages < 1 {
		maxPages = 1
	}
	cycle := time.Now().Format("2006-01")
	all := make([]FinOpsCostInput, 0)
	for page := 1; page <= maxPages; page++ {
		params := url.Values{
			"Action":           {"QueryBill"},
			"Version":          {"2017-12-14"},
			"Format":           {"JSON"},
			"AccessKeyId":      {account.AccessKey},
			"SignatureMethod":  {"HMAC-SHA1"},
			"Timestamp":        {time.Now().UTC().Format("2006-01-02T15:04:05Z")},
			"SignatureVersion": {"1.0"},
			"SignatureNonce":   {finOpsNonce()},
			"BillingCycle":     {cycle},
			"PageNum":          {strconv.Itoa(page)},
			"PageSize":         {strconv.Itoa(finOpsBillPageSize)},
		}
		params.Set("Signature", finOpsAliCloudSignature(params, account.SecretKey))
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://business.aliyuncs.com/?"+params.Encode(), nil)
		if err != nil {
			return nil, err
		}
		resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
		if err != nil {
			return nil, err
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("阿里云账单接口返回 HTTP %d: %s", resp.StatusCode, finOpsAPIError(body))
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("解析阿里云账单响应失败: %w", err)
		}
		items := finOpsMaps(finOpsPath(payload, "Data", "Items", "Item"))
		all = append(all, finOpsAliCloudRecords(items, account, cycle)...)
		if len(items) < finOpsBillPageSize {
			break
		}
	}
	return all, nil
}

func (s *Service) fetchTencentBill(ctx context.Context, account model.IntegrationFinOpsAccount, maxPages int) ([]FinOpsCostInput, error) {
	if strings.TrimSpace(account.AccessKey) == "" || strings.TrimSpace(account.SecretKey) == "" {
		return nil, fmt.Errorf("腾讯云账单同步需要 SecretId 与 SecretKey")
	}
	if maxPages < 1 {
		maxPages = 1
	}
	month := time.Now().Format("2006-01")
	all := make([]FinOpsCostInput, 0)
	for page := 0; page < maxPages; page++ {
		body, _ := json.Marshal(map[string]any{"Month": month, "Offset": page * finOpsBillPageSize, "Limit": finOpsBillPageSize})
		req, err := finOpsTencentRequest(ctx, account, "DescribeBillDetail", body)
		if err != nil {
			return nil, err
		}
		resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
		if err != nil {
			return nil, err
		}
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("腾讯云账单接口返回 HTTP %d: %s", resp.StatusCode, finOpsAPIError(responseBody))
		}
		var payload map[string]any
		if err := json.Unmarshal(responseBody, &payload); err != nil {
			return nil, fmt.Errorf("解析腾讯云账单响应失败: %w", err)
		}
		if response := finOpsMap(payload["Response"]); response != nil {
			if apiError := finOpsMap(response["Error"]); apiError != nil {
				return nil, fmt.Errorf("腾讯云账单接口错误: %s", finOpsString(apiError["Message"]))
			}
			items := finOpsMaps(response["DetailSet"])
			all = append(all, finOpsTencentRecords(items, account, month)...)
			if len(items) < finOpsBillPageSize {
				break
			}
		}
	}
	return all, nil
}

func finOpsAliCloudSignature(params url.Values, secret string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, finOpsAliCloudEncode(key)+"="+finOpsAliCloudEncode(params.Get(key)))
	}
	stringToSign := "GET&%2F&" + finOpsAliCloudEncode(strings.Join(pairs, "&"))
	mac := hmac.New(sha1.New, []byte(secret+"&"))
	_, _ = mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func finOpsAliCloudEncode(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}

func finOpsTencentRequest(ctx context.Context, account model.IntegrationFinOpsAccount, action string, body []byte) (*http.Request, error) {
	const host = "billing.tencentcloudapi.com"
	const service = "billing"
	timestamp := time.Now().Unix()
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")
	hashedPayload := sha256.Sum256(body)
	canonicalRequest := "POST\n/\n\ncontent-type:application/json; charset=utf-8\nhost:" + host + "\n\ncontent-type;host\n" + hex.EncodeToString(hashedPayload[:])
	credentialScope := date + "/" + service + "/tc3_request"
	hashedCanonical := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := "TC3-HMAC-SHA256\n" + strconv.FormatInt(timestamp, 10) + "\n" + credentialScope + "\n" + hex.EncodeToString(hashedCanonical[:])
	secretDate := finOpsHMAC([]byte("TC3"+account.SecretKey), date)
	secretService := finOpsHMAC(secretDate, service)
	secretSigning := finOpsHMAC(secretService, "tc3_request")
	signature := hex.EncodeToString(finOpsHMAC(secretSigning, stringToSign))
	authorization := "TC3-HMAC-SHA256 Credential=" + account.AccessKey + "/" + credentialScope + ", SignedHeaders=content-type;host, Signature=" + signature
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+host, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", authorization)
	req.Header.Set("Host", host)
	req.Header.Set("X-TC-Action", action)
	req.Header.Set("X-TC-Version", "2018-07-09")
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(timestamp, 10))
	if account.Region != "" {
		req.Header.Set("X-TC-Region", account.Region)
	}
	return req, nil
}

func finOpsHMAC(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}

func finOpsAliCloudRecords(items []map[string]any, account model.IntegrationFinOpsAccount, cycle string) []FinOpsCostInput {
	records := make([]FinOpsCostInput, 0, len(items))
	for _, item := range items {
		records = append(records, FinOpsCostInput{ExternalID: finOpsFirst(item, "BillAccountId", "BillOwnerId", "InstanceID", "ProductCode") + "|" + finOpsFirst(item, "BillingCycle", "BillingDate", "BillingDateTime", "UsageStartTime"), BillingDate: finOpsFirst(item, "BillingCycle", "BillingDate", "UsageStartTime", "BillingDateTime", "Date"), Service: finOpsFirst(item, "ProductDetail", "ProductName", "ProductCode"), Region: finOpsFirst(item, "Region", "RegionName"), ResourceID: finOpsFirst(item, "InstanceID", "ResourceId"), ResourceName: finOpsFirst(item, "InstanceID", "ResourceName"), ResourceType: finOpsFirst(item, "ProductCode", "ProductType"), Amount: finOpsFloat(item, "PretaxAmount", "DeductedByCashCoupons", "CashAmount", "Amount"), Currency: finOpsFirst(item, "Currency", "CurrencyCode"), UsageQuantity: finOpsFloat(item, "Usage", "UsageQuantity"), UsageUnit: finOpsFirst(item, "UsageUnit", "SubscriptionType"), Tags: map[string]string{"provider": "alicloud", "billingCycle": cycle}})
	}
	return records
}

func finOpsTencentRecords(items []map[string]any, account model.IntegrationFinOpsAccount, month string) []FinOpsCostInput {
	records := make([]FinOpsCostInput, 0, len(items))
	for _, item := range items {
		records = append(records, FinOpsCostInput{ExternalID: finOpsFirst(item, "BillId", "ResourceId", "ProductCode") + "|" + finOpsFirst(item, "OperateTime", "PayTime", "BillMonth"), BillingDate: finOpsFirst(item, "OperateTime", "PayTime", "BillMonth", "Month"), Service: finOpsFirst(item, "BusinessCodeName", "ProductCodeName", "ProductCode"), Region: finOpsFirst(item, "RegionName", "RegionId"), ResourceID: finOpsFirst(item, "ResourceId", "InstanceId"), ResourceName: finOpsFirst(item, "ResourceName", "InstanceName"), ResourceType: finOpsFirst(item, "PayModeName", "ProductCode"), Amount: finOpsFloat(item, "RealTotalCost", "CashPayAmount", "TotalCost"), Currency: finOpsFirst(item, "Currency", "CurrencyCode"), UsageQuantity: finOpsFloat(item, "UsageAmount", "UsageQuantity"), UsageUnit: finOpsFirst(item, "UsageUnit"), Tags: map[string]string{"provider": "tencent", "billingMonth": month}})
	}
	return records
}

func finOpsPath(value map[string]any, keys ...string) any {
	var current any = value
	for _, key := range keys {
		current = finOpsMap(current)[key]
	}
	return current
}
func finOpsMap(value any) map[string]any { result, _ := value.(map[string]any); return result }
func finOpsMaps(value any) []map[string]any {
	if item := finOpsMap(value); item != nil {
		return []map[string]any{item}
	}
	raw, _ := value.([]any)
	result := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if item := finOpsMap(item); item != nil {
			result = append(result, item)
		}
	}
	return result
}
func finOpsFirst(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if text := finOpsString(value[key]); text != "" {
			return text
		}
	}
	return ""
}
func finOpsString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}
func finOpsFloat(value map[string]any, keys ...string) float64 {
	for _, key := range keys {
		if text := finOpsString(value[key]); text != "" {
			parsed, err := strconv.ParseFloat(strings.ReplaceAll(text, ",", ""), 64)
			if err == nil {
				return parsed
			}
		}
	}
	return 0
}
func finOpsAPIError(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 500 {
		return text[:500]
	}
	return text
}
func finOpsNonce() string {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(value)
}
