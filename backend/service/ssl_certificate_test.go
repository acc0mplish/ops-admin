package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	"ops-admin/backend/internal/domain/provider"
	"ops-admin/backend/model"
)

type certificateDNSMock struct {
	records []provider.DNSRecord
	created []provider.RecordRequest
	deleted []string
}

func (m *certificateDNSMock) ListDomains(context.Context) ([]provider.Domain, error) { return nil, nil }
func (m *certificateDNSMock) ListRecords(context.Context, string) ([]provider.DNSRecord, error) {
	return append([]provider.DNSRecord{}, m.records...), nil
}
func (m *certificateDNSMock) CreateRecord(_ context.Context, req provider.RecordRequest) error {
	m.created = append(m.created, req)
	m.records = append(m.records, provider.DNSRecord{ID: "challenge-1", Domain: req.Domain, Host: req.Host, Type: req.Type, Value: req.Value})
	return nil
}
func (m *certificateDNSMock) UpdateRecord(context.Context, provider.RecordRequest) error { return nil }
func (m *certificateDNSMock) DeleteRecord(_ context.Context, _ string, id string) error {
	m.deleted = append(m.deleted, id)
	return nil
}
func (m *certificateDNSMock) EnableRecord(context.Context, string, string) error  { return nil }
func (m *certificateDNSMock) DisableRecord(context.Context, string, string) error { return nil }

func issueTestCertificate(t *testing.T, domains []string, notBefore, notAfter time.Time) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1001),
		Subject:               pkix.Name{CommonName: domains[0], Organization: []string{"Ops Admin Test"}},
		Issuer:                pkix.Name{CommonName: "Ops Admin Test CA"},
		DNSNames:              domains,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: mustPKCS8(t, key)})
	return string(certPEM), string(keyPEM)
}

func mustPKCS8(t *testing.T, key *rsa.PrivateKey) []byte {
	t.Helper()
	data, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestParseCertificateAndKeySingleDomain(t *testing.T) {
	certPEM, keyPEM := issueTestCertificate(t, []string{"api.example.com"}, time.Now().Add(-time.Hour), time.Now().Add(90*24*time.Hour))
	parsed, err := parseCertificateAndKey(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Type != model.SSLCertificateTypeSingle || len(parsed.Domains) != 1 || parsed.Domains[0] != "api.example.com" {
		t.Fatalf("unexpected parsed certificate: %#v", parsed)
	}
	if parsed.KeyAlgorithm != "RSA-2048" || parsed.Fingerprint == "" || parsed.Serial == "" {
		t.Fatalf("missing metadata: %#v", parsed)
	}
}

func TestParseCertificateAndKeyClassifiesWildcardAndSAN(t *testing.T) {
	now := time.Now()
	wildCert, wildKey := issueTestCertificate(t, []string{"*.example.com", "example.com"}, now.Add(-time.Hour), now.Add(90*24*time.Hour))
	wildcard, err := parseCertificateAndKey(wildCert, wildKey)
	if err != nil {
		t.Fatal(err)
	}
	if wildcard.Type != model.SSLCertificateTypeWildcard {
		t.Fatalf("type=%s", wildcard.Type)
	}

	sanCert, sanKey := issueTestCertificate(t, []string{"api.example.com", "www.example.com"}, now.Add(-time.Hour), now.Add(90*24*time.Hour))
	san, err := parseCertificateAndKey(sanCert, sanKey)
	if err != nil {
		t.Fatal(err)
	}
	if san.Type != model.SSLCertificateTypeSAN || len(san.Domains) != 2 {
		t.Fatalf("unexpected SAN result: %#v", san)
	}
}

func TestParseCertificateAndKeyRejectsMismatch(t *testing.T) {
	now := time.Now()
	certPEM, _ := issueTestCertificate(t, []string{"api.example.com"}, now.Add(-time.Hour), now.Add(90*24*time.Hour))
	_, otherKey := issueTestCertificate(t, []string{"other.example.com"}, now.Add(-time.Hour), now.Add(90*24*time.Hour))
	_, err := parseCertificateAndKey(certPEM, otherKey)
	if err == nil || !strings.Contains(err.Error(), "不匹配") {
		t.Fatalf("expected mismatch error, got %v", err)
	}
}

func TestParseCertificateAndKeyRejectsMalformedPEM(t *testing.T) {
	if _, err := parseCertificateAndKey("not a certificate", "not a key"); err == nil {
		t.Fatal("malformed PEM was accepted")
	}
}

func TestACMEDNSChallengeCreatesAndCleansTXTRecord(t *testing.T) {
	mock := &certificateDNSMock{}
	challenge := &acmeDNSChallengeProvider{dns: mock, mainDomain: "example.com", timeout: time.Second, interval: 10 * time.Millisecond, recordIDsByKey: map[string][]string{}, recordFor: func(string, string) (string, string) {
		return "_acme-challenge.api.example.com.", "validation-token"
	}}
	if err := challenge.Present("api.example.com", "token", "key-auth"); err != nil {
		t.Fatal(err)
	}
	if len(mock.created) != 1 || mock.created[0].Type != "TXT" || mock.created[0].Host != "_acme-challenge.api" {
		t.Fatalf("unexpected TXT creation: %#v", mock.created)
	}
	if err := challenge.CleanUp("api.example.com", "token", "key-auth"); err != nil {
		t.Fatal(err)
	}
	if len(mock.deleted) != 1 || mock.deleted[0] != "challenge-1" {
		t.Fatalf("challenge was not cleaned: %#v", mock.deleted)
	}
}

func TestCertificateDeletionSafetyRulesWithMockProvider(t *testing.T) {
	now := time.Now()
	past, future := now.Add(-time.Hour), now.Add(time.Hour)
	tests := []struct {
		name      string
		cert      model.SSLCertificate
		domains   []string
		records   []provider.DNSRecord
		wantError bool
	}{
		{name: "expired single allowed", cert: model.SSLCertificate{Type: model.SSLCertificateTypeSingle, MainDomain: "example.com", NotAfter: &past}, domains: []string{"api.example.com"}},
		{name: "expired wildcard allowed", cert: model.SSLCertificate{Type: model.SSLCertificateTypeWildcard, MainDomain: "example.com", NotAfter: &past}, domains: []string{"*.example.com"}},
		{name: "valid wildcard blocked", cert: model.SSLCertificate{Type: model.SSLCertificateTypeWildcard, MainDomain: "example.com", NotAfter: &future}, domains: []string{"*.example.com"}, wantError: true},
		{name: "valid single without record allowed", cert: model.SSLCertificate{Type: model.SSLCertificateTypeSingle, MainDomain: "example.com", NotAfter: &future}, domains: []string{"api.example.com"}},
		{name: "valid single with record blocked", cert: model.SSLCertificate{Type: model.SSLCertificateTypeSingle, MainDomain: "example.com", NotAfter: &future}, domains: []string{"api.example.com"}, records: []provider.DNSRecord{{ID: "1", Domain: "example.com", Host: "api", Type: "A", Value: "192.0.2.10"}}, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mock := &certificateDNSMock{records: test.records}
			err := validateCertificateDeletionWithProvider(test.cert, test.domains, mock, now)
			if (err != nil) != test.wantError {
				t.Fatalf("error=%v wantError=%v", err, test.wantError)
			}
		})
	}
}
