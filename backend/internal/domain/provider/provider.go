package provider

import (
	"context"
	"fmt"
	"strings"
)

type Domain struct {
	Name        string `json:"domain"`
	RecordCount int    `json:"recordCount"`
	Status      string `json:"status"`
}

type DNSRecord struct {
	ID       string `json:"id"`
	Domain   string `json:"domain"`
	Host     string `json:"host"`
	Type     string `json:"type"`
	Value    string `json:"value"`
	TTL      int    `json:"ttl"`
	Line     string `json:"line"`
	Status   string `json:"status"`
	Provider string `json:"provider"`
}

type RecordRequest struct {
	Domain   string `json:"domain"`
	RecordID string `json:"recordId"`
	Host     string `json:"host"`
	Type     string `json:"type"`
	Value    string `json:"value"`
	TTL      int    `json:"ttl"`
	Line     string `json:"line"`
}

type PublicDNSProvider interface {
	ListDomains(ctx context.Context) ([]Domain, error)
	ListRecords(ctx context.Context, domain string) ([]DNSRecord, error)
	CreateRecord(ctx context.Context, req RecordRequest) error
	UpdateRecord(ctx context.Context, req RecordRequest) error
	DeleteRecord(ctx context.Context, domain, recordID string) error
	EnableRecord(ctx context.Context, domain, recordID string) error
	DisableRecord(ctx context.Context, domain, recordID string) error
}

func New(name, accessKey, secretKey string) (PublicDNSProvider, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "aliyun", "alicloud":
		return &AliyunDNSProvider{AccessKey: accessKey, SecretKey: secretKey}, nil
	case "tencent", "dnspod":
		return &TencentDNSProvider{SecretID: accessKey, SecretKey: secretKey}, nil
	default:
		return nil, fmt.Errorf("unsupported DNS provider %q", name)
	}
}
