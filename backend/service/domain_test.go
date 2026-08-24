package service

import (
	"context"
	"errors"
	"testing"

	"ops-admin/backend/internal/domain/provider"
)

type mockDNSProvider struct {
	calls  []string
	failID string
}

func (m *mockDNSProvider) ListDomains(context.Context) ([]provider.Domain, error) {
	return []provider.Domain{{Name: "example.com", RecordCount: 2}}, nil
}
func (m *mockDNSProvider) ListRecords(context.Context, string) ([]provider.DNSRecord, error) {
	return []provider.DNSRecord{{ID: "1", Host: "www", Type: "A", Value: "192.0.2.1"}}, nil
}
func (m *mockDNSProvider) CreateRecord(_ context.Context, r provider.RecordRequest) error {
	m.calls = append(m.calls, "create:"+r.RecordID)
	return m.failure(r.RecordID)
}
func (m *mockDNSProvider) UpdateRecord(_ context.Context, r provider.RecordRequest) error {
	m.calls = append(m.calls, "update:"+r.RecordID)
	return m.failure(r.RecordID)
}
func (m *mockDNSProvider) DeleteRecord(_ context.Context, _ string, id string) error {
	m.calls = append(m.calls, "delete:"+id)
	return m.failure(id)
}
func (m *mockDNSProvider) EnableRecord(_ context.Context, _ string, id string) error {
	m.calls = append(m.calls, "enable:"+id)
	return m.failure(id)
}
func (m *mockDNSProvider) DisableRecord(_ context.Context, _ string, id string) error {
	m.calls = append(m.calls, "disable:"+id)
	return m.failure(id)
}
func (m *mockDNSProvider) failure(id string) error {
	if id == m.failID {
		return errors.New("mock provider failure")
	}
	return nil
}

func TestPublicDNSProviderContractWithMock(t *testing.T) {
	mock := &mockDNSProvider{}
	domains, err := mock.ListDomains(context.Background())
	if err != nil || len(domains) != 1 {
		t.Fatal("domain list failed")
	}
	records, err := mock.ListRecords(context.Background(), "example.com")
	if err != nil || len(records) != 1 {
		t.Fatal("record list failed")
	}
	for _, action := range []string{"create", "update", "delete", "enable", "disable"} {
		if err := executeProviderAction(context.Background(), mock, action, provider.RecordRequest{Domain: "example.com", RecordID: "1"}); err != nil {
			t.Fatalf("%s failed: %v", action, err)
		}
	}
	if len(mock.calls) != 5 {
		t.Fatalf("calls=%v", mock.calls)
	}
}
func TestPublicDNSProviderPartialFailureIsVisible(t *testing.T) {
	mock := &mockDNSProvider{failID: "2"}
	results := map[string]error{}
	for _, id := range []string{"1", "2", "3"} {
		results[id] = executeProviderAction(context.Background(), mock, "disable", provider.RecordRequest{Domain: "example.com", RecordID: id})
	}
	if results["1"] != nil || results["2"] == nil || results["3"] != nil {
		t.Fatalf("partial results lost: %#v", results)
	}
}

func TestFindProviderRecordRejectsRecordOutsideCurrentDomain(t *testing.T) {
	mock := &mockDNSProvider{}
	record, err := findProviderRecord(context.Background(), mock, "example.com", "1")
	if err != nil || record.ID != "1" {
		t.Fatalf("expected current-domain record, got %#v err=%v", record, err)
	}
	if _, err := findProviderRecord(context.Background(), mock, "example.com", "other-domain-record"); err == nil {
		t.Fatal("record outside the current domain was accepted")
	}
}
func TestCNAMECycleDetection(t *testing.T) {
	edges := map[string]string{"a.ops.internal.": "b.ops.internal.", "b.ops.internal.": "a.ops.internal."}
	if !cnameHasLoop(edges, "a.ops.internal.") {
		t.Fatal("two-node CNAME loop not detected")
	}
	edges["b.ops.internal."] = "grafana.ops.internal."
	if cnameHasLoop(edges, "a.ops.internal.") {
		t.Fatal("valid CNAME chain reported as loop")
	}
}

func TestNormalizeInternalRecordPayload(t *testing.T) {
	payload, err := normalizeInternalRecordPayload(InternalRecordPayload{Host: " App ", Type: "a", Value: "10.20.30.40", TTL: 0, Status: 0})
	if err != nil {
		t.Fatal(err)
	}
	if payload.Host != "app" || payload.Type != "A" || payload.TTL != 300 || payload.Status != 1 {
		t.Fatalf("unexpected normalized payload: %#v", payload)
	}
	if _, err := normalizeInternalRecordPayload(InternalRecordPayload{Host: "bad", Type: "A", Value: "not-an-ip"}); err == nil {
		t.Fatal("invalid IPv4 was accepted")
	}
}

func TestUniqueUintIDs(t *testing.T) {
	ids := uniqueUintIDs([]uint{3, 0, 3, 2, 2, 1})
	if len(ids) != 3 || ids[0] != 3 || ids[1] != 2 || ids[2] != 1 {
		t.Fatalf("unexpected ids: %#v", ids)
	}
}
