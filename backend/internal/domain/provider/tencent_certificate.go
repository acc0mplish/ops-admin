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
	"time"
)

const tencentCertificateHost = "ssl.tencentcloudapi.com"

type TencentCertificateProvider struct {
	SecretID, SecretKey string
	Endpoint            string
	HTTPClient          *http.Client
}

func (p *TencentCertificateProvider) call(ctx context.Context, action string, payload any, output any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	timestamp := time.Now().Unix()
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")
	hashed := sha256.Sum256(body)
	canonical := "POST\n/\n\ncontent-type:application/json; charset=utf-8\nhost:" + tencentCertificateHost + "\n\ncontent-type;host\n" + hex.EncodeToString(hashed[:])
	scope := date + "/ssl/tc3_request"
	canonicalHash := sha256.Sum256([]byte(canonical))
	stringToSign := "TC3-HMAC-SHA256\n" + strconv.FormatInt(timestamp, 10) + "\n" + scope + "\n" + hex.EncodeToString(canonicalHash[:])
	secretDate := certificateTencentHMAC([]byte("TC3"+p.SecretKey), date)
	secretService := certificateTencentHMAC(secretDate, "ssl")
	secretSigning := certificateTencentHMAC(secretService, "tc3_request")
	authorization := "TC3-HMAC-SHA256 Credential=" + p.SecretID + "/" + scope + ", SignedHeaders=content-type;host, Signature=" + hex.EncodeToString(certificateTencentHMAC(secretSigning, stringToSign))
	endpoint := p.Endpoint
	if endpoint == "" {
		endpoint = "https://" + tencentCertificateHost
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", authorization)
	req.Header.Set("X-TC-Action", action)
	req.Header.Set("X-TC-Version", "2019-12-05")
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(timestamp, 10))
	client := p.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	var envelope struct {
		Response struct {
			Error *struct{ Code, Message string }
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

type tencentCertificateItem struct {
	CertificateID, Alias, Domain, ProductZhName, CertBeginTime, CertEndTime, StatusMsg string
	Status                                                                             int
	CertSANs                                                                           []string
}

func (p *TencentCertificateProvider) ListCertificates(ctx context.Context) ([]CloudCertificate, error) {
	result := []CloudCertificate{}
	for offset := 0; offset < 100000; offset += 1000 {
		var response struct {
			Response struct {
				TotalCount   int
				Certificates []tencentCertificateItem
			}
		}
		if err := p.call(ctx, "DescribeCertificates", map[string]any{"Offset": offset, "Limit": 1000, "CertificateType": "SVR"}, &response); err != nil {
			return nil, err
		}
		for _, item := range response.Response.Certificates {
			result = append(result, mapTencentCertificate(item))
		}
		if len(response.Response.Certificates) < 1000 || offset+1000 >= response.Response.TotalCount {
			break
		}
	}
	return result, nil
}

func (p *TencentCertificateProvider) GetCertificate(ctx context.Context, certificateID string) (*CloudCertificate, error) {
	var response struct{ Response tencentCertificateItem }
	if err := p.call(ctx, "DescribeCertificate", map[string]any{"CertificateId": certificateID}, &response); err != nil {
		return nil, err
	}
	item := mapTencentCertificate(response.Response)
	if item.ID == "" {
		item.ID = certificateID
	}
	return &item, nil
}

func mapTencentCertificate(item tencentCertificateItem) CloudCertificate {
	domains := item.CertSANs
	if len(domains) == 0 {
		domains = splitCloudDomains(item.Domain)
	}
	status := item.StatusMsg
	if status == "" {
		status = strconv.Itoa(item.Status)
	}
	return CloudCertificate{ID: item.CertificateID, Name: first(item.Alias, item.Domain, item.CertificateID), MainDomain: item.Domain, Domains: domains, Type: certificateType(domains), Issuer: item.ProductZhName, NotBefore: parseCloudTime(item.CertBeginTime), NotAfter: parseCloudTime(item.CertEndTime), Status: status}
}

func (p *TencentCertificateProvider) UploadCertificate(ctx context.Context, cert CertificateUpload) (string, error) {
	var response struct {
		Response struct{ CertificateID, RepeatCertID string }
	}
	payload := map[string]any{"CertificatePublicKey": cert.CertificatePEM + "\n" + cert.CertificateChain, "CertificatePrivateKey": cert.PrivateKeyPEM, "CertificateType": "SVR", "Alias": cert.Name, "Repeatable": false}
	if err := p.call(ctx, "UploadCertificate", payload, &response); err != nil {
		return "", err
	}
	return first(response.Response.CertificateID, response.Response.RepeatCertID), nil
}

func (p *TencentCertificateProvider) DeleteCertificate(ctx context.Context, certificateID string) error {
	var response struct{ Response struct{ DeleteResult bool } }
	if err := p.call(ctx, "DeleteCertificate", map[string]any{"CertificateId": certificateID, "IsCheckResource": false}, &response); err != nil {
		return err
	}
	if !response.Response.DeleteResult {
		return fmt.Errorf("腾讯云未确认删除证书 %s", certificateID)
	}
	return nil
}

func certificateTencentHMAC(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}
