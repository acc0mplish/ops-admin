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
