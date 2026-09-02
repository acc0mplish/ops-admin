package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const aliyunCASEndpoint = "https://cas.aliyuncs.com/"

type AliyunCertificateProvider struct {
	AccessKey, SecretKey string
	Endpoint             string
	HTTPClient           *http.Client
}

func (p *AliyunCertificateProvider) call(ctx context.Context, action string, extra url.Values, output any) error {
	params := url.Values{"Format": {"JSON"}, "Version": {"2020-04-07"}, "AccessKeyId": {p.AccessKey}, "SignatureMethod": {"HMAC-SHA1"}, "SignatureVersion": {"1.0"}, "SignatureNonce": {fmt.Sprintf("ops-admin-cert-%d", time.Now().UnixNano())}, "Timestamp": {time.Now().UTC().Format("2006-01-02T15:04:05Z")}, "Action": {action}}
	for key, values := range extra {
		for _, value := range values {
			params.Add(key, value)
		}
	}
	params.Set("Signature", aliyunSignature(params, p.SecretKey))
	endpoint := first(p.Endpoint, aliyunCASEndpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	client := p.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	var apiErr struct{ Code, Message string }
	_ = json.Unmarshal(body, &apiErr)
	if resp.StatusCode/100 != 2 || apiErr.Code != "" {
		return fmt.Errorf("%s: %s", first(apiErr.Code, resp.Status), first(apiErr.Message, string(body)))
	}
	return json.Unmarshal(body, output)
}

func (p *AliyunCertificateProvider) ListCertificates(ctx context.Context) ([]CloudCertificate, error) {
	result := []CloudCertificate{}
	for page := 1; page <= 100; page++ {
		var response struct {
			TotalCount           int
			CertificateOrderList []struct {
				CertID                                                                                                int64 `json:"CertId"`
				ResourceID, Name, CommonName, Sans, Issuer, SerialNo, Fingerprint, CertStartTime, CertEndTime, Status string
			}
		}
		err := p.call(ctx, "ListUserCertificateOrder", url.Values{"OrderType": {"CERT"}, "CurrentPage": {strconv.Itoa(page)}, "ShowSize": {"100"}}, &response)
		if err != nil {
			return nil, err
		}
		for _, item := range response.CertificateOrderList {
			domains := splitCloudDomains(first(item.Sans, item.CommonName))
			id := strconv.FormatInt(item.CertID, 10)
			if item.CertID == 0 {
				id = item.ResourceID
			}
			result = append(result, CloudCertificate{ID: id, Name: first(item.Name, item.CommonName, id), MainDomain: item.CommonName, Domains: domains, Type: certificateType(domains), Issuer: item.Issuer, SerialNumber: item.SerialNo, Fingerprint: item.Fingerprint, NotBefore: parseCloudTime(item.CertStartTime), NotAfter: parseCloudTime(item.CertEndTime), Status: item.Status})
		}
		if len(response.CertificateOrderList) < 100 || page*100 >= response.TotalCount {
			break
		}
	}
	return result, nil
}

func (p *AliyunCertificateProvider) GetCertificate(ctx context.Context, certificateID string) (*CloudCertificate, error) {
	var response struct {
		CertificateID                                                                                   int64
		InstanceID, CertificateName, CommonName, Domain, Issuer, Serial, FingerPrint, CertificateStatus string
		NotBefore, NotAfter                                                                             int64
		SubjectAlternativeNames                                                                         []string
	}
	id, err := strconv.ParseInt(strings.TrimSpace(certificateID), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("Aliyun certificate ID must be numeric: %w", err)
	}
	if err := p.call(ctx, "GetCertificateDetail", url.Values{"CertificateId": {strconv.FormatInt(id, 10)}}, &response); err != nil {
		return nil, err
	}
	domains := response.SubjectAlternativeNames
	if len(domains) == 0 {
		domains = splitCloudDomains(first(response.Domain, response.CommonName))
	}
	notBefore, notAfter := millisTime(response.NotBefore), millisTime(response.NotAfter)
	return &CloudCertificate{ID: certificateID, Name: response.CertificateName, MainDomain: response.CommonName, Domains: domains, Type: certificateType(domains), Issuer: response.Issuer, SerialNumber: response.Serial, Fingerprint: response.FingerPrint, NotBefore: notBefore, NotAfter: notAfter, Status: response.CertificateStatus}, nil
}

func (p *AliyunCertificateProvider) UploadCertificate(ctx context.Context, cert CertificateUpload) (string, error) {
	var response struct {
		CertID     int64 `json:"CertId"`
		ResourceID string
	}
	values := url.Values{"Name": {cert.Name}, "Cert": {strings.TrimSpace(cert.CertificatePEM + "\n" + cert.CertificateChain)}, "Key": {cert.PrivateKeyPEM}}
	if err := p.call(ctx, "UploadUserCertificate", values, &response); err != nil {
		return "", err
	}
	if response.CertID > 0 {
		return strconv.FormatInt(response.CertID, 10), nil
	}
	return response.ResourceID, nil
}

func (p *AliyunCertificateProvider) DeleteCertificate(ctx context.Context, certificateID string) error {
	id, err := strconv.ParseInt(strings.TrimSpace(certificateID), 10, 64)
	if err != nil {
		return fmt.Errorf("Aliyun only supports deleting certificates by numeric CertId: %w", err)
	}
	var response map[string]any
	return p.call(ctx, "DeleteUserCertificate", url.Values{"CertId": {strconv.FormatInt(id, 10)}}, &response)
}

func millisTime(value int64) *time.Time {
	if value <= 0 {
		return nil
	}
	if value > 1_000_000_000_000 {
		value /= 1000
	}
	parsed := time.Unix(value, 0)
	return &parsed
}
