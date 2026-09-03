#!/usr/bin/env python3
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[2]


def replace_exact(path: Path, old: str, new: str, expected: int | None = None) -> int:
    text = path.read_text(encoding="utf-8")
    count = text.count(old)
    if expected is not None and count != expected:
        raise SystemExit(f"{path}: expected {expected} occurrence(s) of {old!r}, found {count}")
    if count:
        path.write_text(text.replace(old, new), encoding="utf-8")
    return count


def replace_regex(path: Path, pattern: str, replacement: str, expected: int = 1) -> int:
    text = path.read_text(encoding="utf-8")
    updated, count = re.subn(pattern, replacement, text, flags=re.S)
    if count != expected:
        raise SystemExit(f"{path}: expected {expected} regex replacement(s), found {count}: {pattern}")
    path.write_text(updated, encoding="utf-8")
    return count


def patch_asset_service() -> None:
    path = ROOT / "backend/service/asset_service.go"
    replace_exact(path, '"errors"\n', '"ops-admin/backend/apperr"\n', 1)

    replacements = {
        'errors.New("service name, kubernetes cluster and namespace are required")': 'apperr.New("ASSET_SERVICE_FIELDS_REQUIRED", nil)',
        'errors.New("at least one workload is required")': 'apperr.New("ASSET_SERVICE_WORKLOAD_REQUIRED", nil)',
        'errors.New("selected kubernetes cluster does not exist")': 'apperr.New("K8S_CLUSTER_NOT_FOUND", nil)',
        'errors.New("service does not exist")': 'apperr.New("ASSET_SERVICE_NOT_FOUND", nil)',
        'errors.New("kubernetes cluster is required")': 'apperr.New("K8S_CLUSTER_REQUIRED", nil)',
        'errors.New("workload does not belong to this service")': 'apperr.New("ASSET_SERVICE_WORKLOAD_MISMATCH", nil)',
        'errors.New("only deployment supports version rollback")': 'apperr.New("ASSET_SERVICE_ROLLBACK_DEPLOYMENT_ONLY", nil)',
        'errors.New("rollback revision does not exist")': 'apperr.New("ASSET_SERVICE_ROLLBACK_REVISION_NOT_FOUND", nil)',
        'errors.New("pod does not belong to this workload")': 'apperr.New("ASSET_SERVICE_POD_MISMATCH", nil)',
        'errors.New(k8sClusterConnectError)': 'apperr.New("K8S_CLUSTER_CONNECTION_FAILED", nil)',
    }
    expected = {
        'errors.New("service name, kubernetes cluster and namespace are required")': 1,
        'errors.New("at least one workload is required")': 1,
        'errors.New("selected kubernetes cluster does not exist")': 1,
        'errors.New("service does not exist")': 1,
        'errors.New("kubernetes cluster is required")': 1,
        'errors.New("workload does not belong to this service")': 6,
        'errors.New("only deployment supports version rollback")': 2,
        'errors.New("rollback revision does not exist")': 1,
        'errors.New("pod does not belong to this workload")': 1,
        'errors.New(k8sClusterConnectError)': 5,
    }
    for old, new in replacements.items():
        replace_exact(path, old, new, expected[old])


def patch_asset_service_controller() -> None:
    path = ROOT / "backend/controller/asset_service.go"
    text = path.read_text(encoding="utf-8")

    direct_replacements = {
        'httpx.Failed(c, 400, "invalid diagnosis target")': 'httpx.FailedCode(c, 400, "INVALID_DIAGNOSIS_TARGET", nil)',
        'httpx.Failed(c, 400, "arthas-boot.jar is required")': 'httpx.FailedCode(c, 400, "ARTHAS_FILE_REQUIRED", nil)',
        'httpx.Failed(c, http.StatusBadRequest, "invalid service payload")': 'httpx.FailedCode(c, http.StatusBadRequest, "INVALID_ASSET_SERVICE_PAYLOAD", nil)',
        'httpx.Failed(c, http.StatusBadRequest, "invalid delete payload")': 'httpx.FailedCode(c, http.StatusBadRequest, "INVALID_DELETE_PAYLOAD", nil)',
        'httpx.Failed(c, http.StatusBadRequest, "invalid workload rollback payload")': 'httpx.FailedCode(c, http.StatusBadRequest, "INVALID_WORKLOAD_ROLLBACK_PAYLOAD", nil)',
    }
    expected = {
        'httpx.Failed(c, 400, "invalid diagnosis target")': 5,
        'httpx.Failed(c, 400, "arthas-boot.jar is required")': 1,
        'httpx.Failed(c, http.StatusBadRequest, "invalid service payload")': 1,
        'httpx.Failed(c, http.StatusBadRequest, "invalid delete payload")': 1,
        'httpx.Failed(c, http.StatusBadRequest, "invalid workload rollback payload")': 1,
    }
    for old, new in direct_replacements.items():
        count = text.count(old)
        if count != expected[old]:
            raise SystemExit(f"{path}: expected {expected[old]} occurrence(s) of {old!r}, found {count}")
        text = text.replace(old, new)

    text, count = re.subn(
        r"httpx\.Failed\(c,\s*([^,\n]+),\s*err\.Error\(\)\)",
        r"httpx.FailedError(c, \1, err)",
        text,
    )
    if count < 10:
        raise SystemExit(f"{path}: expected at least 10 err.Error response conversions, found {count}")

    path.write_text(text, encoding="utf-8")


def patch_audit_log_markers() -> None:
    implementation = ROOT / "backend/middleware/operation_log.go"
    replacements = {
        'parts = append(parts, "body: sensitive SSL certificate and private-key content omitted")': 'parts = append(parts, "body: [request-body-omitted:sensitive-ssl]")',
        'parts = append(parts, "body: multipart/form-data upload content omitted")': 'parts = append(parts, "body: [request-body-omitted:multipart]")',
        'parts = append(parts, "body: oversized request content omitted")': 'parts = append(parts, "body: [request-body-omitted:oversized]")',
    }
    for old, new in replacements.items():
        replace_exact(implementation, old, new, 1)

    test = ROOT / "backend/middleware/operation_log_test.go"
    replace_exact(
        test,
        r'if !strings.Contains(summary, "\u5df2\u8df3\u8fc7\u8bb0\u5f55") {',
        'if !strings.Contains(summary, "[request-body-omitted:sensitive-ssl]") {',
        1,
    )


def patch_ai_time_compatibility() -> None:
    path = ROOT / "backend/service/integration_ai.go"
    replace_exact(
        path,
        '{prefix: "어제", days: -1}, {prefix: "yesterday", days: -1},',
        r'{prefix: "어제", days: -1}, {prefix: "\u6628\u5929", days: -1}, {prefix: "yesterday", days: -1},',
        1,
    )
    replace_exact(
        path,
        '{prefix: "오늘", days: 0}, {prefix: "today", days: 0},',
        r'{prefix: "오늘", days: 0}, {prefix: "\u4eca\u5929", days: 0}, {prefix: "today", days: 0},',
        1,
    )


def patch_notify_templates_and_tests() -> None:
    implementation = ROOT / "backend/service/notify.go"
    replace_exact(implementation, '[Scheduled Task] {{taskName}} · {{status}}', '[정기 작업] {{taskName}} · {{status}}', 1)
    replace_exact(implementation, '[Pipeline Notification] {{pipelineName}} · {{stageName}}', '[파이프라인 알림] {{pipelineName}} · {{stageName}}', 1)
    replace_exact(implementation, '[Job Notification] {{jobName}} · {{stepName}}', '[Job 알림] {{jobName}} · {{stepName}}', 1)

    test = ROOT / "backend/service/notify_test.go"
    replace_exact(
        test,
        r'if job != "\u901a\u77e5/\u53d1\u5e03\u4f5c\u4e1a/\u6d88\u606f\u901a\u77e5" {',
        r'if job != "알림/\u53d1\u5e03\u4f5c\u4e1a/\u6d88\u606f\u901a\u77e5" {',
        1,
    )
    replace_exact(
        test,
        r'if monitor != "\u5df2\u6062\u590d" {',
        'if monitor != "복구" {',
        1,
    )
    replace_exact(
        test,
        r'if !strings.Contains(title, "\u3010\u4f5c\u4e1a\u901a\u77e5\u3011") || !strings.Contains(content, "{{jobHistoryId}}") {',
        'if !strings.Contains(title, "[Job 알림]") || !strings.Contains(content, "{{jobHistoryId}}") {',
        1,
    )


def patch_ssl_key_mismatch_error() -> None:
    implementation = ROOT / "backend/service/ssl_certificate.go"
    replace_exact(
        implementation,
        '"gorm.io/gorm/clause"\n\t"ops-admin/backend/internal/domain/provider"',
        '"gorm.io/gorm/clause"\n\t"ops-admin/backend/apperr"\n\t"ops-admin/backend/internal/domain/provider"',
        1,
    )
    replace_exact(
        implementation,
        'errors.New("certificate and private key do not match")',
        'apperr.New("SSL_CERTIFICATE_KEY_MISMATCH", nil)',
        1,
    )

    test = ROOT / "backend/service/ssl_certificate_test.go"
    replace_exact(test, '\t"strings"\n', '', 1)
    replace_exact(
        test,
        '\t"ops-admin/backend/internal/domain/provider"\n',
        '\t"ops-admin/backend/apperr"\n\t"ops-admin/backend/internal/domain/provider"\n',
        1,
    )
    replace_regex(
        test,
        r'func TestParseCertificateAndKeyRejectsMismatch\(t \*testing\.T\)\{.*?(?=\nfunc TestParseCertificateAndKeyRejectsMalformedPEM)',
        '''func TestParseCertificateAndKeyRejectsMismatch(t *testing.T) {
\tnow := time.Now()
\tcertPEM, _ := issueTestCertificate(t, []string{"api.example.com"}, now.Add(-time.Hour), now.Add(90*24*time.Hour))
\t_, otherKey := issueTestCertificate(t, []string{"other.example.com"}, now.Add(-time.Hour), now.Add(90*24*time.Hour))
\t_, err := parseCertificateAndKey(certPEM, otherKey)
\tcode, _, ok := apperr.Extract(err)
\tif !ok || code != "SSL_CERTIFICATE_KEY_MISMATCH" {
\t\tt.Fatalf("expected SSL_CERTIFICATE_KEY_MISMATCH, got %v", err)
\t}
}
''',
        1,
    )


if __name__ == "__main__":
    patch_asset_service()
    patch_asset_service_controller()
    patch_audit_log_markers()
    patch_ai_time_compatibility()
    patch_notify_templates_and_tests()
    patch_ssl_key_mismatch_error()
    print("Applied backend error-code remediation batch 1 and localization regression fixes.")
