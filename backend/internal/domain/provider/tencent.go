package provider

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
	"strconv"
	"strings"
	"time"
)

const tencentDNSHost = "dnspod.tencentcloudapi.com"

type TencentDNSProvider struct{ SecretID, SecretKey string }

func (p *TencentDNSProvider) call(ctx context.Context, action string, payload any, output any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	timestamp := time.Now().Unix()
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")
	hashed := sha256.Sum256(body)
	canonical := "POST\n/\n\ncontent-type:application/json; charset=utf-8\nhost:" + tencentDNSHost + "\n\ncontent-type;host\n" + hex.EncodeToString(hashed[:])
	scope := date + "/dnspod/tc3_request"
	canonicalHash := sha256.Sum256([]byte(canonical))
	stringToSign := "TC3-HMAC-SHA256\n" + strconv.FormatInt(timestamp, 10) + "\n" + scope + "\n" + hex.EncodeToString(canonicalHash[:])
	secretDate := tencentHMAC([]byte("TC3"+p.SecretKey), date)
	secretService := tencentHMAC(secretDate, "dnspod")
	secretSigning := tencentHMAC(secretService, "tc3_request")
	authorization := "TC3-HMAC-SHA256 Credential=" + p.SecretID + "/" + scope + ", SignedHeaders=content-type;host, Signature=" + hex.EncodeToString(tencentHMAC(secretSigning, stringToSign))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+tencentDNSHost, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", authorization)
	req.Header.Set("X-TC-Action", action)
	req.Header.Set("X-TC-Version", "2021-03-23")
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(timestamp, 10))
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	var envelope struct {
		Response struct {
			Error     *struct{ Code, Message string }
			RequestID string
		}
	}
	_ = json.Unmarshal(data, &envelope)
	if resp.StatusCode/100 != 2 || envelope.Response.Error != nil {
		if envelope.Response.Error != nil {
			return fmt.Errorf("%s: %s", envelope.Response.Error.Code, envelope.Response.Error.Message)
		}
		return fmt.Errorf("%s: %s", resp.Status, string(data))
	}
	return json.Unmarshal(data, output)
}

func (p *TencentDNSProvider) ListDomains(ctx context.Context) ([]Domain, error) {
	result := []Domain{}
	for offset := 0; offset < 10000; offset += 100 {
		var response struct {
			Response struct {
				DomainCountInfo struct{ DomainTotal int }
				DomainList      []struct {
					Name, Status string
					RecordCount  int
				}
			}
		}
		if err := p.call(ctx, "DescribeDomainList", map[string]any{"Offset": offset, "Limit": 100}, &response); err != nil {
			return nil, err
		}
		for _, item := range response.Response.DomainList {
			result = append(result, Domain{Name: item.Name, RecordCount: item.RecordCount, Status: strings.ToLower(item.Status)})
		}
		if len(response.Response.DomainList) < 100 {
			break
		}
	}
	return result, nil
}
func (p *TencentDNSProvider) ListRecords(ctx context.Context, domain string) ([]DNSRecord, error) {
	result := []DNSRecord{}
	for offset := 0; offset < 100000; offset += 100 {
		var response struct {
			Response struct {
				RecordList []struct {
					RecordID                        uint64
					Name, Type, Value, Line, Status string
					TTL                             int
				}
			}
		}
		if err := p.call(ctx, "DescribeRecordList", map[string]any{"Domain": domain, "Offset": offset, "Limit": 100}, &response); err != nil {
			return nil, err
		}
		for _, item := range response.Response.RecordList {
			result = append(result, DNSRecord{ID: strconv.FormatUint(item.RecordID, 10), Domain: domain, Host: item.Name, Type: item.Type, Value: item.Value, TTL: item.TTL, Line: item.Line, Status: strings.ToLower(item.Status), Provider: "tencent"})
		}
		if len(response.Response.RecordList) < 100 {
			break
		}
	}
	return result, nil
}
func (p *TencentDNSProvider) CreateRecord(ctx context.Context, req RecordRequest) error {
	return p.simple(ctx, "CreateRecord", recordPayload(req, false))
}
func (p *TencentDNSProvider) UpdateRecord(ctx context.Context, req RecordRequest) error {
	return p.simple(ctx, "ModifyRecord", recordPayload(req, true))
}
func recordPayload(req RecordRequest, includeID bool) map[string]any {
	payload := map[string]any{"Domain": req.Domain, "SubDomain": req.Host, "RecordType": req.Type, "RecordLine": first(req.Line, "\u9ed8\u8ba4"), "Value": req.Value, "TTL": req.TTL}
	if includeID {
		id, _ := strconv.ParseUint(req.RecordID, 10, 64)
		payload["RecordId"] = id
	}
	return payload
}
func (p *TencentDNSProvider) DeleteRecord(ctx context.Context, domain, id string) error {
	return p.idAction(ctx, "DeleteRecord", domain, id, "")
}
func (p *TencentDNSProvider) EnableRecord(ctx context.Context, domain, id string) error {
	return p.idAction(ctx, "ModifyRecordStatus", domain, id, "ENABLE")
}
func (p *TencentDNSProvider) DisableRecord(ctx context.Context, domain, id string) error {
	return p.idAction(ctx, "ModifyRecordStatus", domain, id, "DISABLE")
}
func (p *TencentDNSProvider) idAction(ctx context.Context, action, domain, id, status string) error {
	recordID, _ := strconv.ParseUint(id, 10, 64)
	payload := map[string]any{"Domain": domain, "RecordId": recordID}
	if status != "" {
		payload["Status"] = status
	}
	return p.simple(ctx, action, payload)
}
func (p *TencentDNSProvider) simple(ctx context.Context, action string, payload any) error {
	var response map[string]any
	return p.call(ctx, action, payload, &response)
}
func tencentHMAC(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}
