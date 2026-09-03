#!/usr/bin/env python3
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


def replace_exact(path: Path, old: str, new: str, expected: int = 1) -> None:
    text = path.read_text(encoding="utf-8")
    count = text.count(old)
    if count != expected:
        raise SystemExit(f"{path}: expected {expected} occurrence(s) of {old!r}, found {count}")
    path.write_text(text.replace(old, new), encoding="utf-8")


def patch_diagnosis_errors() -> None:
    path = ROOT / "backend/service/asset_service_diagnosis.go"
    replace_exact(path, '\t"errors"\n', '\t"ops-admin/backend/apperr"\n', 1)

    replacements = {
        'errors.New("workload does not belong to this service")': 'apperr.New("ASSET_SERVICE_WORKLOAD_MISMATCH", nil)',
        'errors.New("pod and container are required")': 'apperr.New("DIAGNOSIS_POD_CONTAINER_REQUIRED", nil)',
        'errors.New("invalid process id")': 'apperr.New("INVALID_PROCESS_ID", nil)',
        'errors.New(k8sClusterConnectError)': 'apperr.New("K8S_CLUSTER_CONNECTION_FAILED", nil)',
        'errors.New("pod container not found")': 'apperr.New("DIAGNOSIS_CONTAINER_NOT_FOUND", nil)',
        'fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))': 'apperr.Wrap("DIAGNOSIS_EXECUTION_FAILED", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String())), nil)',
        'fmt.Errorf("Arthas download failed: %w", err)': 'apperr.Wrap("ARTHAS_DOWNLOAD_FAILED", err, nil)',
        'errors.New("arthas-boot.jar is empty")': 'apperr.New("ARTHAS_FILE_EMPTY", nil)',
        'errors.New("arthas-boot.jar exceeds 80MB")': 'apperr.New("ARTHAS_FILE_TOO_LARGE", map[string]any{"maxMb": 80})',
        'fmt.Errorf("upload to container failed: %w: %s", err, strings.TrimSpace(stderr.String()))': 'apperr.Wrap("ARTHAS_UPLOAD_FAILED", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String())), nil)',
        'errors.New("invalid flamegraph sampling duration")': 'apperr.New("INVALID_FLAMEGRAPH_DURATION", nil)',
        'errors.New("invalid flamegraph event")': 'apperr.New("INVALID_FLAMEGRAPH_EVENT", nil)',
        'fmt.Errorf("start profiler failed: %w", err)': 'apperr.Wrap("FLAMEGRAPH_START_FAILED", err, nil)',
        'fmt.Errorf("stop profiler failed: %w", stopErr)': 'apperr.Wrap("FLAMEGRAPH_STOP_FAILED", stopErr, nil)',
        'fmt.Errorf("read flamegraph failed: %w", err)': 'apperr.Wrap("FLAMEGRAPH_READ_FAILED", err, nil)',
        'errors.New("generated flamegraph is not valid HTML")': 'apperr.New("FLAMEGRAPH_INVALID_OUTPUT", nil)',
        'errors.New("CPU profiler collected no samples; generate application load during the sampling period and try again")': 'apperr.New("FLAMEGRAPH_CPU_NO_SAMPLES", nil)',
        'errors.New("allocation profiler collected no samples; trigger requests that allocate objects during the sampling period and try again")': 'apperr.New("FLAMEGRAPH_ALLOC_NO_SAMPLES", nil)',
        'errors.New("process id is required")': 'apperr.New("PROCESS_ID_REQUIRED", nil)',
        'fmt.Errorf("Arthas JVM dashboard failed: %w", err)': 'apperr.Wrap("ARTHAS_DASHBOARD_FAILED", err, nil)',
        'fmt.Errorf("Arthas flamegraph failed: %w", err)': 'apperr.Wrap("ARTHAS_FLAMEGRAPH_FAILED", err, nil)',
        'errors.New("class pattern is required")': 'apperr.New("CLASS_PATTERN_REQUIRED", nil)',
        'errors.New("invalid class pattern")': 'apperr.New("INVALID_CLASS_PATTERN", nil)',
        'errors.New("unsupported diagnosis operation")': 'apperr.New("UNSUPPORTED_DIAGNOSIS_OPERATION", nil)',
        'fmt.Errorf("Arthas CLI diagnosis failed: %w", err)': 'apperr.Wrap("ARTHAS_CLI_FAILED", err, nil)',
    }
    expected = {
        'errors.New("workload does not belong to this service")': 1,
        'errors.New("pod and container are required")': 1,
        'errors.New("invalid process id")': 1,
        'errors.New(k8sClusterConnectError)': 4,
        'errors.New("pod container not found")': 2,
        'fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))': 1,
        'fmt.Errorf("Arthas download failed: %w", err)': 1,
        'errors.New("arthas-boot.jar is empty")': 1,
        'errors.New("arthas-boot.jar exceeds 80MB")': 1,
        'fmt.Errorf("upload to container failed: %w: %s", err, strings.TrimSpace(stderr.String()))': 1,
        'errors.New("invalid flamegraph sampling duration")': 1,
        'errors.New("invalid flamegraph event")': 1,
        'fmt.Errorf("start profiler failed: %w", err)': 1,
        'fmt.Errorf("stop profiler failed: %w", stopErr)': 1,
        'fmt.Errorf("read flamegraph failed: %w", err)': 1,
        'errors.New("generated flamegraph is not valid HTML")': 1,
        'errors.New("CPU profiler collected no samples; generate application load during the sampling period and try again")': 1,
        'errors.New("allocation profiler collected no samples; trigger requests that allocate objects during the sampling period and try again")': 1,
        'errors.New("process id is required")': 1,
        'fmt.Errorf("Arthas JVM dashboard failed: %w", err)': 1,
        'fmt.Errorf("Arthas flamegraph failed: %w", err)': 1,
        'errors.New("class pattern is required")': 1,
        'errors.New("invalid class pattern")': 1,
        'errors.New("unsupported diagnosis operation")': 1,
        'fmt.Errorf("Arthas CLI diagnosis failed: %w", err)': 1,
    }
    for old, new in replacements.items():
        replace_exact(path, old, new, expected[old])


if __name__ == "__main__":
    patch_diagnosis_errors()
    print("Applied diagnosis error-code remediation batch 2.")
