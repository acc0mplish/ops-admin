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
  INVALID_DIAGNOSIS_TARGET: {
    ko: '진단 대상 정보가 올바르지 않습니다.',
    en: 'The diagnosis target is invalid.'
  },
  ARTHAS_FILE_REQUIRED: {
    ko: 'arthas-boot.jar 파일을 선택하십시오.',
    en: 'Select an arthas-boot.jar file.'
  },
  INVALID_ASSET_SERVICE_PAYLOAD: {
    ko: '서비스 설정 요청이 올바르지 않습니다.',
    en: 'The asset service payload is invalid.'
  },
  INVALID_DELETE_PAYLOAD: {
    ko: '삭제 요청이 올바르지 않습니다.',
    en: 'The delete request is invalid.'
  },
  INVALID_WORKLOAD_ROLLBACK_PAYLOAD: {
    ko: 'Workload Rollback 요청이 올바르지 않습니다.',
    en: 'The workload rollback request is invalid.'
  },
  ASSET_SERVICE_FIELDS_REQUIRED: {
    ko: '서비스 이름, Kubernetes 클러스터, Namespace를 모두 입력하십시오.',
    en: 'Service name, Kubernetes cluster, and namespace are required.'
  },
  ASSET_SERVICE_WORKLOAD_REQUIRED: {
    ko: '하나 이상의 Workload를 선택하십시오.',
    en: 'Select at least one workload.'
  },
  K8S_CLUSTER_REQUIRED: {
    ko: 'Kubernetes 클러스터를 선택하십시오.',
    en: 'Select a Kubernetes cluster.'
  },
  K8S_CLUSTER_NOT_FOUND: {
    ko: '선택한 Kubernetes 클러스터를 찾을 수 없습니다.',
    en: 'The selected Kubernetes cluster was not found.'
  },
  K8S_CLUSTER_CONNECTION_FAILED: {
    ko: 'Kubernetes 클러스터에 연결하지 못했습니다.',
    en: 'Failed to connect to the Kubernetes cluster.'
  },
  ASSET_SERVICE_NOT_FOUND: {
    ko: '서비스를 찾을 수 없습니다.',
    en: 'The asset service was not found.'
  },
  ASSET_SERVICE_WORKLOAD_MISMATCH: {
    ko: '선택한 Workload가 이 서비스에 속하지 않습니다.',
    en: 'The selected workload does not belong to this service.'
  },
  ASSET_SERVICE_ROLLBACK_DEPLOYMENT_ONLY: {
    ko: 'Version Rollback은 Deployment에서만 지원합니다.',
    en: 'Version rollback is supported only for Deployments.'
  },
  ASSET_SERVICE_ROLLBACK_REVISION_NOT_FOUND: {
    ko: 'Rollback할 Revision을 찾을 수 없습니다.',
    en: 'The rollback revision was not found.'
  },
  ASSET_SERVICE_POD_MISMATCH: {
    ko: '선택한 Pod가 이 Workload에 속하지 않습니다.',
    en: 'The selected Pod does not belong to this workload.'
  },
  SSL_CERTIFICATE_KEY_MISMATCH: {
    ko: '인증서와 Private Key가 일치하지 않습니다.',
    en: 'The certificate and private key do not match.'
  }
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
