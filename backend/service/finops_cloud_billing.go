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
	"errors"
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
	return s.fetchFinOpsCostsForMonth(ctx, account, time.Now().Format("2006-01"), maxPages)
}

// fetchFinOpsCostsForMonth fetches a single natural month's bill.  Keeping the
// month at this boundary makes callers unable to accidentally turn a multi-month
// sync into one opaque provider request.
func (s *Service) fetchFinOpsCostsForMonth(ctx context.Context, account model.IntegrationFinOpsAccount, month string, maxPages int) ([]FinOpsCostInput, error) {
	switch strings.ToLower(strings.TrimSpace(account.Provider)) {
	case "alicloud":
		return s.fetchAliCloudBill(ctx, account, month, maxPages)
	case "tencent":
		return s.fetchTencentBill(ctx, account, month, maxPages)
	default:
		return s.fetchCustomFinOpsBill(ctx, account, month)
	}
}

func finOpsBillingSource(provider string) string {
	switch strings.ToLower(provider) {
	case "alicloud":
		return "Aliyun Official Billing API"
	case "tencent":
		return "Tencent Cloud Official Billing API"
	default:
		return "Custom Billing API"
	}
}

func (s *Service) fetchCustomFinOpsBill(ctx context.Context, account model.IntegrationFinOpsAccount, month string) ([]FinOpsCostInput, error) {
	endpoint := strings.TrimSpace(account.BillingEndpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("%s has no billing HTTP endpoint configured", finOpsBillingSource(account.Provider))
	}
	requestURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	// Custom adapters receive one explicit month per request as well.
	query := requestURL.Query()
	query.Set("month", month)
	requestURL.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
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
		return nil, fmt.Errorf("billing API returned HTTP %d", resp.StatusCode)
	}
	var wrapper struct {
		Records []FinOpsCostInput `json:"records"`
	}
	if err = json.Unmarshal(body, &wrapper); err != nil || wrapper.Records == nil {
		if err = json.Unmarshal(body, &wrapper.Records); err != nil {
			return nil, fmt.Errorf("invalid billing API response format: %w", err)
		}
	}
	return wrapper.Records, nil
}

func (s *Service) fetchAliCloudBill(ctx context.Context, account model.IntegrationFinOpsAccount, cycle string, maxPages int) ([]FinOpsCostInput, error) {
	if strings.TrimSpace(account.AccessKey) == "" || strings.TrimSpace(account.SecretKey) == "" {
		return nil, fmt.Errorf("Aliyun billing synchronization requires AccessKey and SecretKey")
	}
	if maxPages < 1 {
		maxPages = 1
	}
	// Fetch one monthly instance bill only.  Calling the cloud API once per day
	// made a six-month sync exceed the request timeout.  We keep the source
	// dimensions, then create an explicitly estimated daily view locally.
	return s.fetchAliCloudMonthlyInstanceBill(ctx, account, cycle, maxPages)
}

func (s *Service) fetchAliCloudMonthlyInstanceBill(ctx context.Context, account model.IntegrationFinOpsAccount, cycle string, maxPages int) ([]FinOpsCostInput, error) {
	all := make([]FinOpsCostInput, 0)
	nextToken := ""
	seenTokens := map[string]bool{}
	for page := 1; page <= maxPages; page++ {
		params := url.Values{
			"Action":           {"DescribeInstanceBill"},
			"Version":          {"2017-12-14"},
			"Format":           {"JSON"},
			"AccessKeyId":      {account.AccessKey},
			"SignatureMethod":  {"HMAC-SHA1"},
			"Timestamp":        {time.Now().UTC().Format("2006-01-02T15:04:05Z")},
			"SignatureVersion": {"1.0"},
			"SignatureNonce":   {finOpsNonce()},
			"BillingCycle":     {cycle},
			"IsHideZeroCharge": {"true"},
			"MaxResults":       {strconv.Itoa(finOpsBillPageSize)},
		}
		if nextToken != "" {
			params.Set("NextToken", nextToken)
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
			return nil, fmt.Errorf("Aliyun billing API returned HTTP %d: %s", resp.StatusCode, finOpsAPIError(body))
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("failed to parse Aliyun billing response: %w", err)
		}
		items, declaredCount := finOpsAliCloudBillItems(payload)
		if declaredCount > 0 && len(items) == 0 {
			return nil, fmt.Errorf("Aliyun billing API declared %d records but no detail rows could be parsed", declaredCount)
		}
		// The instance bill does not always return a unique RecordID for every
		// detail row. Prefix every source row with its page and row position so a
		// missing/repeated RecordID cannot overwrite another charge during upsert.
		all = append(all, finOpsAliCloudEstimatedDailyRecordsWithPrefix(items, account, cycle, fmt.Sprintf("page:%d", page))...)
		nextToken = finOpsFirst(finOpsMap(payload["Data"]), "NextToken")
		if nextToken == "" {
			break
		}
		if seenTokens[nextToken] {
			return nil, errors.New("Aliyun billing API returned a repeated NextToken; stopped to avoid duplicate charges")
		}
		seenTokens[nextToken] = true
	}
	return all, nil
}

func (s *Service) fetchTencentBill(ctx context.Context, account model.IntegrationFinOpsAccount, month string, maxPages int) ([]FinOpsCostInput, error) {
	if strings.TrimSpace(account.AccessKey) == "" || strings.TrimSpace(account.SecretKey) == "" {
		return nil, fmt.Errorf("Tencent Cloud billing synchronization requires SecretId and SecretKey")
	}
	if maxPages < 1 {
		maxPages = 1
	}
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
			return nil, fmt.Errorf("Tencent Cloud billing API returned HTTP %d: %s", resp.StatusCode, finOpsAPIError(responseBody))
		}
		var payload map[string]any
		if err := json.Unmarshal(responseBody, &payload); err != nil {
			return nil, fmt.Errorf("failed to parse Tencent Cloud billing response: %w", err)
		}
		if response := finOpsMap(payload["Response"]); response != nil {
			if apiError := finOpsMap(response["Error"]); apiError != nil {
				return nil, fmt.Errorf("Tencent Cloud billing API error: %s", finOpsString(apiError["Message"]))
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
		billingDate := finOpsMonthBillingDate(finOpsFirst(item, "BillingCycle", "BillingDate", "UsageStartTime", "BillingDateTime", "Date"), cycle)
		// CashAmount is the cash actually paid after coupons. Keep the full
		// difference from list price as discount, including coupon deductions.
		actual, hasCashAmount := finOpsOptionalFloat(item, "CashAmount")
		if !hasCashAmount {
			actual = finOpsFloat(item, "PretaxAmount", "Amount")
		}
		original := finOpsFloat(item, "PretaxGrossAmount", "ListPrice", "OriginalAmount")
		if original == 0 {
			original = actual
		}
		discount := original - actual
		records = append(records, FinOpsCostInput{ExternalID: finOpsAliCloudExternalID(item, cycle), BillingDate: billingDate, Service: finOpsFirst(item, "ProductDetail", "ProductName", "ProductCode"), Region: finOpsFirst(item, "Region", "RegionName"), ResourceID: finOpsFirst(item, "InstanceID", "ResourceId"), ResourceName: finOpsFirst(item, "InstanceID", "ResourceName"), ResourceType: finOpsFirst(item, "ProductCode", "ProductType"), ResourceConfig: finOpsFirst(item, "InstanceSpec", "InstanceType", "Specification"), Amount: actual, OriginalPrice: original, Discount: discount, ActualPayment: actual, Currency: finOpsFirst(item, "Currency", "CurrencyCode"), UsageQuantity: finOpsFloat(item, "Usage", "UsageQuantity"), UsageUnit: finOpsFirst(item, "UsageUnit", "SubscriptionType"), Tags: map[string]string{"provider": "alicloud", "billingCycle": cycle}})
	}
	return records
}

func finOpsAliCloudExternalID(item map[string]any, cycle string) string {
	if recordID := finOpsFirst(item, "RecordID", "RecordId", "BillId", "BillID"); recordID != "" {
		return "alicloud|" + cycle + "|" + recordID
	}
	return "alicloud|" + cycle + "|" + finOpsFirst(item, "SubOrderId", "InstanceID", "ResourceId", "ProductCode") + "|" + finOpsFirst(item, "PaymentTime", "UsageStartTime", "BillingDate", "BillingCycle")
}

func finOpsAliCloudEstimatedDailyRecords(items []map[string]any, account model.IntegrationFinOpsAccount, cycle string) []FinOpsCostInput {
	return finOpsAliCloudEstimatedDailyRecordsWithPrefix(items, account, cycle, "")
}

func finOpsAliCloudEstimatedDailyRecordsWithPrefix(items []map[string]any, account model.IntegrationFinOpsAccount, cycle, sourcePrefix string) []FinOpsCostInput {
	month, err := time.ParseInLocation("2006-01", cycle, time.Local)
	if err != nil {
		return nil
	}
	lastDay := month.AddDate(0, 1, -1)
	if current := time.Now(); month.Year() == current.Year() && month.Month() == current.Month() {
		lastDay = time.Date(current.Year(), current.Month(), current.Day(), 0, 0, 0, 0, current.Location())
	}
	days := int(lastDay.Sub(month).Hours()/24) + 1
	if days < 1 {
		return nil
	}
	records := make([]FinOpsCostInput, 0, len(items)*days)
	for itemIndex, item := range items {
		instanceID := finOpsFirst(item, "InstanceID", "InstanceId", "ResourceId")
		resourceName := finOpsFirst(item, "InstanceName", "ResourceName", "InstanceID", "InstanceId")
		actual, hasCashAmount := finOpsOptionalFloat(item, "CashAmount")
		if !hasCashAmount {
			actual = finOpsFloat(item, "PretaxAmount", "Amount")
		}
		original := finOpsFloat(item, "PretaxGrossAmount", "ListPrice", "OriginalAmount")
		if original == 0 {
			original = actual
		}
		discount := original - actual
		resourceConfig := finOpsFirst(item, "InstanceSpec", "InstanceType", "Specification")
		lineID := fmt.Sprintf("%s|line:%s:%d", finOpsAliCloudExternalID(item, cycle), sourcePrefix, itemIndex)
		for day := month; !day.After(lastDay); day = day.AddDate(0, 0, 1) {
			records = append(records, FinOpsCostInput{
				ExternalID: lineID + "|estimated|" + day.Format("2006-01-02"), BillingDate: day.Format("2006-01-02"),
				Service: finOpsFirst(item, "ProductName", "ProductDetail", "ProductCode"), Region: finOpsFirst(item, "Region", "RegionName", "RegionId"),
				ResourceID: instanceID, ResourceName: resourceName, ResourceType: finOpsFirst(item, "ProductCode", "ProductType", "ProductName"), ResourceConfig: resourceConfig,
				Amount: actual / float64(days), OriginalPrice: original / float64(days), Discount: discount / float64(days), ActualPayment: actual / float64(days), Currency: finOpsFirst(item, "Currency", "CurrencyCode"),
				UsageQuantity: finOpsFloat(item, "Usage", "UsageQuantity") / float64(days), UsageUnit: finOpsFirst(item, "UsageUnit", "SubscriptionType"),
				Tags: map[string]string{"provider": "alicloud", "billingCycle": cycle, "granularity": "daily_estimate"},
			})
		}
	}
	return records
}

func finOpsTencentRecords(items []map[string]any, account model.IntegrationFinOpsAccount, month string) []FinOpsCostInput {
	records := make([]FinOpsCostInput, 0, len(items))
	for _, item := range items {
		billingDate := finOpsMonthBillingDate(finOpsFirst(item, "OperateTime", "PayTime", "BillMonth", "Month"), month)
		actual := finOpsFloat(item, "RealTotalCost", "CashPayAmount", "TotalCost")
		original := finOpsFloat(item, "TotalCost", "OriginalCost", "ListPrice", "RealTotalCost")
		records = append(records, FinOpsCostInput{ExternalID: finOpsFirst(item, "BillId", "ResourceId", "ProductCode") + "|" + finOpsFirst(item, "OperateTime", "PayTime", "BillMonth"), BillingDate: billingDate, Service: finOpsFirst(item, "BusinessCodeName", "ProductCodeName", "ProductCode"), Region: finOpsFirst(item, "RegionName", "RegionId"), ResourceID: finOpsFirst(item, "ResourceId", "InstanceId"), ResourceName: finOpsFirst(item, "ResourceName", "InstanceName"), ResourceType: finOpsFirst(item, "PayModeName", "ProductCode"), ResourceConfig: finOpsFirst(item, "InstanceType", "Specification", "ProductCodeName"), Amount: actual, OriginalPrice: original, Discount: max(0, original-actual), ActualPayment: actual, Currency: finOpsFirst(item, "Currency", "CurrencyCode"), UsageQuantity: finOpsFloat(item, "UsageAmount", "UsageQuantity"), UsageUnit: finOpsFirst(item, "UsageUnit"), Tags: map[string]string{"provider": "tencent", "billingMonth": month}})
	}
	return records
}

// A provider may return an invalid detail timestamp for an otherwise valid
// monthly bill. The requested billing month remains the authoritative boundary
// for this sync, so use it rather than failing the entire month.
func finOpsMonthBillingDate(value, month string) string {
	if _, err := parseFinOpsDate(value); err == nil {
		return value
	}
	return month
}

func finOpsPath(value map[string]any, keys ...string) any {
	var current any = value
	for _, key := range keys {
		current = finOpsMap(current)[key]
	}
	return current
}

// AliCloud returns the instance-bill list in slightly different envelopes for
// legacy and upgraded APIs.  Accept all documented variants rather than
// treating a non-empty response as a successful zero-record sync.
func finOpsAliCloudBillItems(payload map[string]any) ([]map[string]any, int) {
	data := finOpsMap(payload["Data"])
	items := finOpsMaps(finOpsPath(payload, "Data", "Items", "Item"))
	if len(items) == 0 {
		items = finOpsMaps(data["Items"])
	}
	if len(items) == 0 {
		items = finOpsMaps(data["Item"])
	}
	return items, int(finOpsFloat(data, "TotalCount", "Count"))
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
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case uint:
		return strconv.FormatUint(uint64(typed), 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
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

func finOpsOptionalFloat(value map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		text := finOpsString(value[key])
		if text == "" {
			continue
		}
		parsed, err := strconv.ParseFloat(strings.ReplaceAll(text, ",", ""), 64)
		if err == nil {
			return parsed, true
		}
	}
	return 0, false
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
