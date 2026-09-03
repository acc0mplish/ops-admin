import { currentLocale } from './i18n-runtime'
import { ct } from './common-i18n'

const errorCodeKeys = {
  AUTH_INVALID_CREDENTIALS: 'invalidCredentials',
  AUTH_ACCOUNT_DISABLED: 'accountDisabled',
  AUTH_SESSION_EXPIRED: 'sessionExpired',
  AUTH_PERMISSION_DENIED: 'permissionDenied',
  RESOURCE_NOT_FOUND: 'resourceNotFound',
  REQUIRED_VALUE_MISSING: 'requiredValueMissing',
  RESOURCE_ALREADY_EXISTS: 'resourceAlreadyExists',
  INVALID_REQUEST: 'invalidRequest',
  UNSUPPORTED_OPERATION: 'unsupportedOperation',
  OPERATION_TIMEOUT: 'operationTimedOut',
  CONNECTION_FAILED: 'connectionFailed',
  OPERATION_FAILED: 'operationFailed'
}

const errorCodeMessages = {
  INVALID_DIAGNOSIS_TARGET: { ko: '진단 대상 정보가 올바르지 않습니다.', en: 'The diagnosis target is invalid.' },
  ARTHAS_FILE_REQUIRED: { ko: 'arthas-boot.jar 파일을 선택하십시오.', en: 'Select an arthas-boot.jar file.' },
  INVALID_ASSET_SERVICE_PAYLOAD: { ko: '서비스 설정 요청이 올바르지 않습니다.', en: 'The asset service payload is invalid.' },
  INVALID_DELETE_PAYLOAD: { ko: '삭제 요청이 올바르지 않습니다.', en: 'The delete request is invalid.' },
  INVALID_WORKLOAD_ROLLBACK_PAYLOAD: { ko: 'Workload Rollback 요청이 올바르지 않습니다.', en: 'The workload rollback request is invalid.' },
  ASSET_SERVICE_FIELDS_REQUIRED: { ko: '서비스 이름, Kubernetes 클러스터, Namespace를 모두 입력하십시오.', en: 'Service name, Kubernetes cluster, and namespace are required.' },
  ASSET_SERVICE_WORKLOAD_REQUIRED: { ko: '하나 이상의 Workload를 선택하십시오.', en: 'Select at least one workload.' },
  K8S_CLUSTER_REQUIRED: { ko: 'Kubernetes 클러스터를 선택하십시오.', en: 'Select a Kubernetes cluster.' },
  K8S_CLUSTER_NOT_FOUND: { ko: '선택한 Kubernetes 클러스터를 찾을 수 없습니다.', en: 'The selected Kubernetes cluster was not found.' },
  K8S_CLUSTER_CONNECTION_FAILED: { ko: 'Kubernetes 클러스터에 연결하지 못했습니다.', en: 'Failed to connect to the Kubernetes cluster.' },
  ASSET_SERVICE_NOT_FOUND: { ko: '서비스를 찾을 수 없습니다.', en: 'The asset service was not found.' },
  ASSET_SERVICE_WORKLOAD_MISMATCH: { ko: '선택한 Workload가 이 서비스에 속하지 않습니다.', en: 'The selected workload does not belong to this service.' },
  ASSET_SERVICE_ROLLBACK_DEPLOYMENT_ONLY: { ko: 'Version Rollback은 Deployment에서만 지원합니다.', en: 'Version rollback is supported only for Deployments.' },
  ASSET_SERVICE_ROLLBACK_REVISION_NOT_FOUND: { ko: 'Rollback할 Revision을 찾을 수 없습니다.', en: 'The rollback revision was not found.' },
  ASSET_SERVICE_POD_MISMATCH: { ko: '선택한 Pod가 이 Workload에 속하지 않습니다.', en: 'The selected Pod does not belong to this workload.' },
  SSL_CERTIFICATE_KEY_MISMATCH: { ko: '인증서와 Private Key가 일치하지 않습니다.', en: 'The certificate and private key do not match.' },
  DIAGNOSIS_POD_CONTAINER_REQUIRED: { ko: '진단할 Pod와 Container를 선택하십시오.', en: 'Select a Pod and container to diagnose.' },
  INVALID_PROCESS_ID: { ko: 'Process ID가 올바르지 않습니다.', en: 'The process ID is invalid.' },
  DIAGNOSIS_CONTAINER_NOT_FOUND: { ko: '선택한 Pod에서 Container를 찾을 수 없습니다.', en: 'The selected container was not found in the Pod.' },
  DIAGNOSIS_EXECUTION_FAILED: { ko: 'Container 진단 명령 실행에 실패했습니다.', en: 'Failed to execute the container diagnosis command.' },
  ARTHAS_DOWNLOAD_FAILED: { ko: 'Arthas 다운로드에 실패했습니다.', en: 'Failed to download Arthas.' },
  ARTHAS_FILE_EMPTY: { ko: 'arthas-boot.jar 파일이 비어 있습니다.', en: 'The arthas-boot.jar file is empty.' },
  ARTHAS_FILE_TOO_LARGE: { ko: 'arthas-boot.jar 파일은 {maxMb}MB를 초과할 수 없습니다.', en: 'The arthas-boot.jar file must not exceed {maxMb}MB.' },
  ARTHAS_UPLOAD_FAILED: { ko: 'Arthas 파일을 Container에 업로드하지 못했습니다.', en: 'Failed to upload the Arthas file to the container.' },
  INVALID_FLAMEGRAPH_DURATION: { ko: 'Flamegraph 수집 시간은 10초, 30초, 60초 또는 120초여야 합니다.', en: 'The flamegraph duration must be 10, 30, 60, or 120 seconds.' },
  INVALID_FLAMEGRAPH_EVENT: { ko: 'Flamegraph Event는 CPU 또는 Allocation이어야 합니다.', en: 'The flamegraph event must be CPU or allocation.' },
  FLAMEGRAPH_START_FAILED: { ko: 'Flamegraph Profiler를 시작하지 못했습니다.', en: 'Failed to start the flamegraph profiler.' },
  FLAMEGRAPH_STOP_FAILED: { ko: 'Flamegraph Profiler를 종료하지 못했습니다.', en: 'Failed to stop the flamegraph profiler.' },
  FLAMEGRAPH_READ_FAILED: { ko: '생성된 Flamegraph를 읽지 못했습니다.', en: 'Failed to read the generated flamegraph.' },
  FLAMEGRAPH_INVALID_OUTPUT: { ko: '생성된 Flamegraph 결과가 올바르지 않습니다.', en: 'The generated flamegraph output is invalid.' },
  FLAMEGRAPH_CPU_NO_SAMPLES: { ko: 'CPU Sample이 수집되지 않았습니다. 수집 중 애플리케이션 부하를 발생시킨 뒤 다시 시도하십시오.', en: 'No CPU samples were collected. Generate application load during sampling and try again.' },
  FLAMEGRAPH_ALLOC_NO_SAMPLES: { ko: 'Allocation Sample이 수집되지 않았습니다. 수집 중 객체 할당 요청을 발생시킨 뒤 다시 시도하십시오.', en: 'No allocation samples were collected. Trigger object allocation during sampling and try again.' },
  PROCESS_ID_REQUIRED: { ko: 'Process ID를 선택하십시오.', en: 'Select a process ID.' },
  ARTHAS_DASHBOARD_FAILED: { ko: 'Arthas JVM Dashboard 조회에 실패했습니다.', en: 'Failed to query the Arthas JVM dashboard.' },
  ARTHAS_FLAMEGRAPH_FAILED: { ko: 'Arthas Flamegraph 생성에 실패했습니다.', en: 'Failed to generate the Arthas flamegraph.' },
  CLASS_PATTERN_REQUIRED: { ko: 'Class Pattern을 입력하십시오.', en: 'Enter a class pattern.' },
  INVALID_CLASS_PATTERN: { ko: 'Class Pattern 형식이 올바르지 않습니다.', en: 'The class pattern is invalid.' },
  UNSUPPORTED_DIAGNOSIS_OPERATION: { ko: '지원하지 않는 진단 작업입니다.', en: 'The diagnosis operation is not supported.' },
  ARTHAS_CLI_FAILED: { ko: 'Arthas CLI 진단 실행에 실패했습니다.', en: 'Failed to execute the Arthas CLI diagnosis.' },
  MONITOR_INVALID_DATE_RANGE: { ko: '모니터링 조회 기간 형식 또는 범위가 올바르지 않습니다.', en: 'The monitoring date range is invalid.' },
  MONITOR_DATASOURCE_NAME_REQUIRED: { ko: '데이터소스 이름을 입력하십시오.', en: 'Enter a datasource name.' },
  MONITOR_DATASOURCE_URL_REQUIRED: { ko: '데이터소스 URL을 입력하십시오.', en: 'Enter a datasource URL.' },
  MONITOR_DATASOURCE_IN_USE: { ko: '사용 중인 데이터소스는 삭제할 수 없습니다.', en: 'A datasource that is in use cannot be deleted.' },
  MONITOR_QUERY_REQUIRED: { ko: '모니터링 Query를 입력하십시오.', en: 'Enter a monitoring query.' },
  MONITOR_DATASOURCE_TYPE_UNSUPPORTED: { ko: '선택한 작업에서 지원하지 않는 데이터소스 유형입니다.', en: 'The datasource type is not supported for this operation.' },
  MONITOR_TIME_RANGE_INVALID: { ko: '종료 시간은 시작 시간보다 이후여야 합니다.', en: 'The end time must be later than the start time.' },
  MONITOR_TRACE_ID_REQUIRED: { ko: '데이터소스와 Trace ID를 입력하십시오.', en: 'Datasource and trace ID are required.' },
  MONITOR_TRACE_ID_INVALID: { ko: 'Trace ID 형식이 올바르지 않습니다.', en: 'The trace ID format is invalid.' },
  MONITOR_TRACE_NOT_FOUND: { ko: '요청한 Trace를 찾을 수 없습니다.', en: 'The requested trace was not found.' },
  MONITOR_DATASOURCE_UNAVAILABLE: { ko: '조건에 맞는 모니터링 데이터소스를 사용할 수 없습니다.', en: 'No matching monitoring datasource is available.' },
  MONITOR_UPSTREAM_REQUEST_FAILED: { ko: '{system} 요청을 처리하지 못했습니다.', en: 'The {system} request failed.' },
  K8S_CLUSTER_ALREADY_EXISTS: { ko: '같은 이름의 Kubernetes 클러스터가 이미 존재합니다.', en: 'A Kubernetes cluster with the same name already exists.' },
  K8S_INVALID_CLUSTER_CONFIG: { ko: 'Kubernetes 클러스터 설정 또는 Kubeconfig가 올바르지 않습니다.', en: 'The Kubernetes cluster configuration or kubeconfig is invalid.' },
  K8S_INVALID_YAML: { ko: 'Kubernetes YAML 형식 또는 내용이 올바르지 않습니다.', en: 'The Kubernetes YAML is invalid.' },
  K8S_RESOURCE_NOT_FOUND: { ko: 'Kubernetes 리소스를 찾을 수 없습니다.', en: 'The Kubernetes resource was not found.' },
  K8S_TRAFFIC_WEIGHT_INVALID: { ko: 'Traffic Weight는 0 이상이어야 하며 합계가 100이어야 합니다.', en: 'Traffic weights must be non-negative and total 100.' },
  K8S_WORKLOAD_SELECTION_REQUIRED: { ko: '하나 이상의 Workload를 선택하십시오.', en: 'Select at least one workload.' },
  K8S_CONTAINER_NOT_FOUND: { ko: '선택한 Workload에서 Container를 찾을 수 없습니다.', en: 'No matching container was found in the selected workload.' },
  K8S_NAMESPACE_REQUIRED: { ko: 'Namespace를 입력하십시오.', en: 'Enter a namespace.' },
  K8S_RESOURCE_ID_REQUIRED: { ko: 'Kubernetes 리소스의 Namespace와 이름을 입력하십시오.', en: 'Enter the Kubernetes resource namespace and name.' },
  K8S_RESOURCE_IMMUTABLE: { ko: '변경할 수 없는 Kubernetes 필드가 포함되어 있습니다. 리소스를 다시 생성하거나 변경 가능한 필드만 수정하십시오.', en: 'The request changes immutable Kubernetes fields. Recreate the resource or update only mutable fields.' },
  K8S_RESOURCE_CONFLICT: { ko: 'YAML 리소스 식별자가 기존 Kubernetes 리소스와 충돌합니다.', en: 'The YAML resource identity conflicts with an existing Kubernetes resource.' }
}

const legacyEnglishPatterns = [
  [/invalid username or password|username or password is incorrect|invalid credentials/i, 'invalidCredentials'],
  [/account is disabled|disabled account/i, 'accountDisabled'],
  [/permission denied|forbidden|not authorized|unauthorized|access denied/i, 'permissionDenied'],
  [/not found|does not exist|doesn't exist|no .* available/i, 'resourceNotFound'],
  [/required|must be provided|must not be empty|cannot be empty/i, 'requiredValueMissing'],
  [/already exists|duplicate/i, 'resourceAlreadyExists'],
  [/not supported|unsupported/i, 'unsupportedOperation'],
  [/timed? out|timeout/i, 'operationTimedOut'],
  [/connect(?:ion)? failed|unable to connect|failed to connect|connection refused/i, 'connectionFailed'],
  [/invalid|malformed|must be numeric|must match/i, 'invalidRequest'],
  [/failed|unable to|could not|cannot|can't/i, 'operationFailed']
]

const HANGUL_RE = /[\uAC00-\uD7A3]/
const HAN_RE = /[\u3400-\u4DBF\u4E00-\u9FFF]/
const LATIN_RE = /[A-Za-z]/

function interpolate(text, params = {}) {
  let result = text
  Object.entries(params || {}).forEach(([name, value]) => {
    result = result.replaceAll(`{${name}}`, String(value))
  })
  return result
}

function payloadMessage(payload) {
  if (typeof payload === 'string') return payload
  if (!payload || typeof payload !== 'object') return ''
  return String(payload.message || payload.error || payload.detail || '')
}

function payloadCode(payload) {
  if (!payload || typeof payload !== 'object') return ''
  return String(payload.errorCode || payload.error_code || payload.codeName || '')
}

function payloadParams(payload) {
  if (!payload || typeof payload !== 'object') return {}
  return payload.errorParams || payload.error_params || payload.params || {}
}

export function resolveApiError(payload, fallbackKey = 'requestFailed') {
  const message = payloadMessage(payload).trim()
  const errorCode = payloadCode(payload).trim()
  const params = payloadParams(payload)

  if (errorCode && errorCodeMessages[errorCode]) {
    const localized = errorCodeMessages[errorCode]
    return interpolate(currentLocale.value === 'en-US' ? localized.en : localized.ko, params)
  }

  if (errorCode && errorCodeKeys[errorCode]) {
    return interpolate(ct(errorCodeKeys[errorCode]), params)
  }

  if (currentLocale.value === 'en-US') {
    return message || ct(fallbackKey)
  }

  if (message && HANGUL_RE.test(message)) {
    return message
  }

  if (message && LATIN_RE.test(message)) {
    const match = legacyEnglishPatterns.find(([pattern]) => pattern.test(message))
    if (match) return ct(match[1])
  }

  if (message && HAN_RE.test(message)) {
    return ct(fallbackKey)
  }

  return ct(fallbackKey)
}
