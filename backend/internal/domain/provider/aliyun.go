package provider

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
)

const aliyunDNSEndpoint = "https://alidns.aliyuncs.com/"

type AliyunDNSProvider struct{ AccessKey, SecretKey string }

func (p *AliyunDNSProvider) call(ctx context.Context, action string, extra url.Values, output any) error {
	params := url.Values{"Format": {"JSON"}, "Version": {"2015-01-09"}, "AccessKeyId": {p.AccessKey}, "SignatureMethod": {"HMAC-SHA1"}, "SignatureVersion": {"1.0"}, "SignatureNonce": {fmt.Sprintf("ops-admin-dns-%d", time.Now().UnixNano())}, "Timestamp": {time.Now().UTC().Format("2006-01-02T15:04:05Z")}, "Action": {action}}
	for key, values := range extra {
		for _, value := range values {
			params.Add(key, value)
		}
	}
	params.Set("Signature", aliyunSignature(params, p.SecretKey))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, aliyunDNSEndpoint+"?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	var apiErr struct{ Code, Message string }
	_ = json.Unmarshal(body, &apiErr)
	if resp.StatusCode/100 != 2 || apiErr.Code != "" {
		return fmt.Errorf("%s: %s", first(apiErr.Code, resp.Status), first(apiErr.Message, string(body)))
	}
	return json.Unmarshal(body, output)
}

func (p *AliyunDNSProvider) ListDomains(ctx context.Context) ([]Domain, error) {
	result := []Domain{}
	for page := 1; page <= 100; page++ {
		var response struct {
			TotalCount int
			Domains    struct {
				Domain []struct {
					DomainName  string
					RecordCount int
				}
			}
		}
		err := p.call(ctx, "DescribeDomains", url.Values{"PageNumber": {strconv.Itoa(page)}, "PageSize": {"100"}}, &response)
		if err != nil {
			return nil, err
		}
		for _, item := range response.Domains.Domain {
			result = append(result, Domain{Name: item.DomainName, RecordCount: item.RecordCount, Status: "enabled"})
		}
		if page*100 >= response.TotalCount {
			break
		}
	}
	return result, nil
}

func (p *AliyunDNSProvider) ListRecords(ctx context.Context, domain string) ([]DNSRecord, error) {
	result := []DNSRecord{}
	for page := 1; page <= 100; page++ {
		var response struct {
			TotalCount    int
			DomainRecords struct {
				Record []struct {
					RecordID, RR, Type, Value, Line, Status string
					TTL                                     int
				}
			}
		}
		err := p.call(ctx, "DescribeDomainRecords", url.Values{"DomainName": {domain}, "PageNumber": {strconv.Itoa(page)}, "PageSize": {"100"}}, &response)
		if err != nil {
			return nil, err
		}
		for _, item := range response.DomainRecords.Record {
			result = append(result, DNSRecord{ID: item.RecordID, Domain: domain, Host: item.RR, Type: item.Type, Value: item.Value, TTL: item.TTL, Line: item.Line, Status: strings.ToLower(item.Status), Provider: "aliyun"})
		}
		if page*100 >= response.TotalCount {
			break
		}
	}
	return result, nil
}

func (p *AliyunDNSProvider) CreateRecord(ctx context.Context, req RecordRequest) error {
	return p.mutate(ctx, "AddDomainRecord", req, false)
}
func (p *AliyunDNSProvider) UpdateRecord(ctx context.Context, req RecordRequest) error {
	return p.mutate(ctx, "UpdateDomainRecord", req, true)
}
func (p *AliyunDNSProvider) mutate(ctx context.Context, action string, req RecordRequest, includeID bool) error {
	values := url.Values{"DomainName": {req.Domain}, "RR": {req.Host}, "Type": {req.Type}, "Value": {req.Value}, "TTL": {strconv.Itoa(req.TTL)}, "Line": {first(req.Line, "default")}}
	if includeID {
		values.Set("RecordId", req.RecordID)
		values.Del("DomainName")
	}
	var response map[string]any
	return p.call(ctx, action, values, &response)
}
func (p *AliyunDNSProvider) DeleteRecord(ctx context.Context, domain, id string) error {
	return p.simple(ctx, "DeleteDomainRecord", url.Values{"RecordId": {id}})
}
func (p *AliyunDNSProvider) EnableRecord(ctx context.Context, domain, id string) error {
	return p.status(ctx, id, "Enable")
}
func (p *AliyunDNSProvider) DisableRecord(ctx context.Context, domain, id string) error {
	return p.status(ctx, id, "Disable")
}
func (p *AliyunDNSProvider) status(ctx context.Context, id, status string) error {
	return p.simple(ctx, "SetDomainRecordStatus", url.Values{"RecordId": {id}, "Status": {status}})
}
func (p *AliyunDNSProvider) simple(ctx context.Context, action string, values url.Values) error {
	var response map[string]any
	return p.call(ctx, action, values, &response)
}

func aliyunSignature(params url.Values, secret string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, aliyunEncode(key)+"="+aliyunEncode(params.Get(key)))
	}
	mac := hmac.New(sha1.New, []byte(secret+"&"))
	_, _ = mac.Write([]byte("GET&%2F&" + aliyunEncode(strings.Join(pairs, "&"))))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
func aliyunEncode(value string) string { return strings.ReplaceAll(url.QueryEscape(value), "+", "%20") }
func first(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
