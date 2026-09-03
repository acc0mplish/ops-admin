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


if __name__ == "__main__":
    patch_asset_service()
    patch_asset_service_controller()
    print("Applied backend error-code remediation batch 1.")
