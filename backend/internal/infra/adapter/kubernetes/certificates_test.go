package kubernetes

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"ops-admin/backend/internal/infra/contract"
)

// certificates_test.go — P1-G1 (④ A′-1) 인증서 파생 관측 테스트. 800 하드캡
// 분할(§9-7) — adapter_test.go에서 Health 관측 경로 테스트를 이 파일로 옮겼다
// (테스트는 검증 대상 파일을 따라 나뉜다).

// certPEMFor — 테스트용 자기서명 인증서(base64 PEM). service 측
// charSelfSignedCertPEM과 동일 형상의 어댑터 패키지 로컬 복제다.
func certPEMFor(t *testing.T, commonName string, notBefore, notAfter time.Time) string {
	t.Helper()
	certPEM, _ := certKeyPairFor(t, commonName, notBefore, notAfter)
	return certPEM
}

// certKeyPairFor — 자기서명 인증서와 대응 개인키의 base64 PEM 쌍. Health 경로는
// 클라이언트 인증서를 실제 TLS 클라이언트로 적재하므로(newK8sHTTPClient) 키가
// 유효한 쌍이어야 probe가 healthy가 된다.
func certKeyPairFor(t *testing.T, commonName string, notBefore, notAfter time.Time) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: commonName},
		Issuer:       pkix.Name{CommonName: commonName},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("x509.CreateCertificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("x509.MarshalECPrivateKey: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return base64.StdEncoding.EncodeToString(certPEM), base64.StdEncoding.EncodeToString(keyPEM)
}

// kubeconfigWithCerts — 인증서 성분을 싣은 kubeconfig(Health 관측 경로 테스트용).
func kubeconfigWithCerts(server, caData, clientData, clientKeyData string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Config
current-context: kind-test
clusters:
  - name: test
    cluster:
      server: %s
      certificate-authority-data: %s
contexts:
  - name: kind-test
    context:
      cluster: test
      user: test
users:
  - name: test
    user:
      client-certificate-data: %s
      client-key-data: %s
`, server, caData, clientData, clientKeyData)
}

// P1-G1 (④ A′-1): Health가 kubeconfig 인증서 성분의 파생 관측을 Observation으로
// 실는다 — 9필드 파생 메타데이터만이고 원본 material·키는 관측 어디에도 없다.
func TestHealthDerivesCertificateObservation(t *testing.T) {
	f := newK8sFixture(t, "v1.29.4")
	now := time.Now()
	caData := certPEMFor(t, "test-ca", now.Add(-time.Hour), now.Add(365*24*time.Hour))
	clientData, clientKeyData := certKeyPairFor(t, "test-client", now.Add(-time.Hour), now.Add(60*24*time.Hour))
	conn := f.Connection()
	conn.Material["inventory"] = kubeconfigWithCerts(f.mock.srv.URL, caData, clientData, clientKeyData)

	h := f.adapter.Health(context.Background(), conn)
	if !h.Healthy {
		t.Fatalf("Health = %+v, want healthy", h)
	}
	if len(h.Observation) == 0 {
		t.Fatal("Health Observation is empty — the derivation must ride the healthy probe")
	}
	entries, ok := h.Observation[certificateObservationKey].([]contract.JSONMap)
	if !ok || len(entries) != 2 {
		t.Fatalf("observation[%q] = %#v, want 2 entries", certificateObservationKey, h.Observation)
	}
	ca, client := entries[0], entries[1]
	if ca["name"] != "CA Certificate" || ca["type"] != "certificate-authority" || ca["subject"] != "test-ca" {
		t.Errorf("CA entry = %#v", ca)
	}
	if client["name"] != "Client Certificate" || client["type"] != "client-certificate" || client["subject"] != "test-client" {
		t.Errorf("client entry = %#v", client)
	}
	for key, entry := range map[string]contract.JSONMap{"ca": ca, "client": client} {
		days, _ := entry["daysRemaining"].(int)
		status, _ := entry["status"].(string)
		if entry["notBefore"] == "" || entry["notAfter"] == "" || entry["issuer"] == "" || status == "" {
			t.Errorf("%s entry missing derived fields: %#v", key, entry)
		}
		if days <= 0 {
			t.Errorf("%s entry daysRemaining = %v, want positive", key, entry["daysRemaining"])
		}
	}
	// 파생 메타데이터만 — base64 인증서 본문·자재 문자열은 관측에 없다.
	blob, err := json.Marshal(h.Observation)
	if err != nil {
		t.Fatalf("marshal observation: %v", err)
	}
	if strings.Contains(string(blob), "certificate-authority-data") || strings.Contains(string(blob), caData) {
		t.Errorf("observation leaks certificate material: %s", blob)
	}
}

// 관측 미산출: 인증서 성분이 없는 kubeconfig(token만 — kubeconfigFor 픽스처)는
// Observation이 nil이다(영값 호환 — healthy 판정 무관).
func TestHealthWithoutCertificatesLeavesObservationEmpty(t *testing.T) {
	f := newK8sFixture(t, "v1.29.4")
	h := f.adapter.Health(context.Background(), f.Connection())
	if !h.Healthy {
		t.Fatalf("Health = %+v, want healthy", h)
	}
	if h.Observation != nil {
		t.Errorf("Observation = %#v, want nil for a cert-less kubeconfig", h.Observation)
	}
}

// 도출 함수의 실패 처분 — 자재 해석 실패는 nil(관측 부재)이다. 파생 엔트리
// 생략 계약(certificateEntry ok=false)과 DeriveCertificateObservation의
// 자재 경계를 직접 단얫한다.
func TestDeriveCertificateObservationRejectsBrokenMaterial(t *testing.T) {
	now := time.Now()
	validCA := certPEMFor(t, "direct-ca", now.Add(-time.Hour), now.Add(90*24*time.Hour))

	cases := []struct {
		name     string
		material string
		wantNil  bool
		wantLen  int
	}{
		{name: "빈 자재", material: "", wantNil: true},
		{name: "kubeconfig 아님", material: "not: a: kubeconfig: [", wantNil: true},
		{name: "유효 CA 1장", material: kubeconfigWithCerts("https://x:6443", validCA, "", ""), wantLen: 1},
		{name: "CA base64 깨짐 → 0장", material: kubeconfigWithCerts("https://x:6443", "!!!broken!!!", "", ""), wantNil: true},
		{name: "PEM 아님(base64만) → 0장", material: kubeconfigWithCerts("https://x:6443", "aGVsbG8=", "", ""), wantNil: true},
	}
	for _, tc := range cases {
		observation := DeriveCertificateObservation(tc.material)
		if tc.wantNil {
			if observation != nil {
				t.Errorf("%s: observation = %#v, want nil", tc.name, observation)
			}
			continue
		}
		entries, ok := observation[certificateObservationKey].([]contract.JSONMap)
		if !ok || len(entries) != tc.wantLen {
			t.Errorf("%s: observation = %#v, want %d entries", tc.name, observation, tc.wantLen)
		}
	}
}
