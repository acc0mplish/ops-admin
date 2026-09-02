package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAliyunCertificateProviderListsCloudCertificates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("Action") != "ListUserCertificateOrder" {
			t.Fatalf("unexpected action: %s", request.URL.Query().Get("Action"))
		}
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{"TotalCount":1,"CertificateOrderList":[{"CertId":101,"Name":"api-example","CommonName":"api.example.com","Sans":"api.example.com,www.example.com","Issuer":"Test CA","SerialNo":"A1","Fingerprint":"FF","CertStartTime":"2026-01-01 00:00:00","CertEndTime":"2027-01-01 00:00:00","Status":"ISSUED"}]}`)
	}))
	defer server.Close()

	cloud := &AliyunCertificateProvider{AccessKey: "test", SecretKey: "test", Endpoint: server.URL + "/", HTTPClient: server.Client()}
	items, err := cloud.ListCertificates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "101" || items[0].Type != "SAN" || len(items[0].Domains) != 2 {
		t.Fatalf("unexpected certificates: %#v", items)
	}
}

func TestTencentCertificateProviderListsCloudCertificates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-TC-Action") != "DescribeCertificates" {
			t.Fatalf("unexpected action: %s", request.Header.Get("X-TC-Action"))
		}
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{"Response":{"TotalCount":1,"Certificates":[{"CertificateId":"tx-101","Alias":"wildcard-example","Domain":"*.example.com","CertSANs":["*.example.com","example.com"],"ProductZhName":"Test CA","CertBeginTime":"2026-01-01 00:00:00","CertEndTime":"2027-01-01 00:00:00","Status":1,"StatusMsg":"\u6b63\u5e38"}]}}`)
	}))
	defer server.Close()

	cloud := &TencentCertificateProvider{SecretID: "test", SecretKey: "test", Endpoint: server.URL, HTTPClient: server.Client()}
	items, err := cloud.ListCertificates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "tx-101" || items[0].Type != "WILDCARD" || items[0].Issuer != "Test CA" {
		t.Fatalf("unexpected certificates: %#v", items)
	}
}
