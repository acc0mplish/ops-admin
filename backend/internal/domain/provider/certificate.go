package provider

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type CloudCertificate struct {
	ID           string
	Name         string
	MainDomain   string
	Domains      []string
	Type         string
	Issuer       string
	SerialNumber string
	Fingerprint  string
	NotBefore    *time.Time
	NotAfter     *time.Time
	Status       string
}

type CertificateUpload struct {
	Name             string
	CertificatePEM   string
	PrivateKeyPEM    string
	CertificateChain string
}

type CertificateCloudProvider interface {
	ListCertificates(ctx context.Context) ([]CloudCertificate, error)
	GetCertificate(ctx context.Context, certificateID string) (*CloudCertificate, error)
	UploadCertificate(ctx context.Context, cert CertificateUpload) (string, error)
	DeleteCertificate(ctx context.Context, certificateID string) error
}

func NewCertificateCloud(name, accessKey, secretKey string) (CertificateCloudProvider, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "aliyun", "alicloud":
		return &AliyunCertificateProvider{AccessKey: accessKey, SecretKey: secretKey}, nil
	case "tencent", "dnspod":
		return &TencentCertificateProvider{SecretID: accessKey, SecretKey: secretKey}, nil
	default:
		return nil, fmt.Errorf("unsupported certificate provider %q", name)
	}
}

func certificateType(domains []string) string {
	for _, domain := range domains {
		if strings.HasPrefix(strings.TrimSpace(domain), "*.") {
			return "WILDCARD"
		}
	}
	if len(domains) > 1 {
		return "SAN"
	}
	return "SINGLE"
}

func splitCloudDomains(value string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, item := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' || r == '\n' }) {
		item = strings.ToLower(strings.TrimSpace(item))
		if item != "" && !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

func parseCloudTime(value string) *time.Time {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339, "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return &parsed
		}
	}
	return nil
}
