package kubernetes

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"time"

	"ops-admin/backend/internal/infra/contract"
)

// certificates.go — P1-G1 (판정 ④ A′-1 확정 — 사용자 승인 2026-09-10):
// kubeconfig의 인증서 성분에서 파생 관측을 도출한다. Health가 이미 파싱한
// clusterRuntime을 재사용하므로 파싱 경로 신설은 0이다.
//
// v1 parseOverviewCertificate(service/k8s_overview.go) 의미론의 재구현이다 —
// client.go 헤더의 어댑터 재구현 원칙과 동일(God service import 금지, arch
// rule 2). 도출은 파생 메타데이터 9필드(name·type·subject·issuer·notBefore·
// notAfter·daysRemaining·status·statusText)만이고, 원본 인증서·개인키·
// kubeconfig 문자열은 관측에 실리지 않는다(보존 제약 #7 · mapping.md §6).

// certificateObservationKey — 관측 최상위 키. 조립(inventory.
// BuildOverviewCertificates)이 읽는 어휘이고 K8sCertificate 9필드의 JSON
// 태그와 1:1이다.
const certificateObservationKey = "certificates"

// DeriveCertificateObservation — kubeconfig material에서 인증서 파생 관측을
// 도출한다. 반환 형상: {"certificates": [ {name,type,subject,issuer,notBefore,
// notAfter,daysRemaining,status,statusText} ]} — 자재 해석 실패는 nil(관측
// 부재)이고 healthy 판정과 무관하다. Z 오라클 표면이기도 하다 — service 테스트
// 글루가 프로덕션 도출 경로를 통과시킨다(정상화 oracle NormalizeSection 선례).
func DeriveCertificateObservation(material string) contract.JSONMap {
	rt, err := parseKubeConfig(material)
	if err != nil {
		return nil
	}
	return certificateObservation(rt)
}

// certificateObservation — runtime의 CA·클라이언트 인증서 관측을 도출한다.
// v1 buildOverviewCertificates 동치 — CA 먼저, 클라이언트 다음, 이름·타입
// 리터럴까지 동일하다.
func certificateObservation(rt clusterRuntime) contract.JSONMap {
	entries := make([]contract.JSONMap, 0, 2)
	if entry, ok := certificateEntry("CA Certificate", "certificate-authority", rt.CertificateAuthority); ok {
		entries = append(entries, entry)
	}
	if entry, ok := certificateEntry("Client Certificate", "client-certificate", rt.ClientCertificateData); ok {
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return nil
	}
	return contract.JSONMap{certificateObservationKey: entries}
}

// certificateEntry — v1 parseOverviewCertificate 동치: base64 → PEM → x509
// 파싱 후 파생 9필드만 반환한다. 빈 값·base64 깨짐·PEM 아님·파싱 실패는
// 엔트리 생략(ok=false)이다 — v1의 ok=false와 같은 처분.
func certificateEntry(name string, certType string, encoded string) (contract.JSONMap, bool) {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return nil, false
	}

	certBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, false
	}

	block, _ := pem.Decode(certBytes)
	if block == nil {
		return nil, false
	}

	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, false
	}

	daysRemaining := int(time.Until(certificate.NotAfter).Hours() / 24)
	status, statusText := CertificateStatus(certificate.NotAfter)

	return contract.JSONMap{
		"name":          name,
		"type":          certType,
		"subject":       CertificateCommonName(certificate.Subject.CommonName, certificate.Subject.String()),
		"issuer":        CertificateCommonName(certificate.Issuer.CommonName, certificate.Issuer.String()),
		"notBefore":     certificate.NotBefore.Local().Format("2006-01-02 15:04:05"),
		"notAfter":      certificate.NotAfter.Local().Format("2006-01-02 15:04:05"),
		"daysRemaining": daysRemaining,
		"status":        status,
		"statusText":    statusText,
	}, true
}

// CertificateCommonName — v1 certificateCommonName 동치(직접 스왑 표면):
// CN 있으면 트림해 쓰고, 없으면 폴백(보통 전체 Subject DN), 그마저 비면 "-".
func CertificateCommonName(commonName string, fallback string) string {
	if strings.TrimSpace(commonName) != "" {
		return strings.TrimSpace(commonName)
	}
	if strings.TrimSpace(fallback) == "" {
		return "-"
	}
	return fallback
}

// CertificateStatus — v1 k8sCertificateStatus 동치(직접 스왑 표면): 만료(<= 0)
// expired·30일 경계(<=) warning·그 외 valid. 30일 경계가 현재 계약이다
// (TestCharK8sCertificateStatus).
func CertificateStatus(notAfter time.Time) (string, string) {
	remaining := time.Until(notAfter)
	switch {
	case remaining <= 0:
		return "expired", "Expired"
	case remaining <= 30*24*time.Hour:
		return "warning", "Expiring Soon"
	default:
		return "valid", "Valid"
	}
}
