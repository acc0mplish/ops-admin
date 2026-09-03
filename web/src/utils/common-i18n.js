import { currentLocale } from './i18n-runtime'

const ko = {
  idleSessionExpired: '6시간 동안 활동이 없어 세션이 종료되었습니다. 다시 로그인하십시오.',
  sessionExpired: '로그인이 만료되었습니다. 다시 로그인하십시오.',
  requestFailed: '요청에 실패했습니다.',
  networkError: '네트워크 오류가 발생했습니다.',
  tokenRefreshFailed: '로그인 갱신에 실패했습니다. 다시 로그인하십시오.',
  invalidCredentials: '사용자명 또는 비밀번호가 올바르지 않습니다.',
  accountDisabled: '비활성화된 계정입니다. 관리자에게 문의하십시오.',
  permissionDenied: '이 작업을 수행할 권한이 없습니다.',
  resourceNotFound: '요청한 리소스를 찾을 수 없습니다.',
  requiredValueMissing: '필수 입력값이 누락되었습니다.',
  resourceAlreadyExists: '이미 존재하는 항목입니다.',
  invalidRequest: '요청 값이 올바르지 않습니다.',
  unsupportedOperation: '지원하지 않는 작업입니다.',
  operationTimedOut: '작업 시간이 초과되었습니다.',
  connectionFailed: '대상 시스템에 연결하지 못했습니다.',
  operationFailed: '작업을 완료하지 못했습니다.',
  unassigned: '미지정',
  productionAck: '프로덕션',
  destructiveAck: '확인',
  executeAck: '실행',
  productionRisk: '대상에 프로덕션 환경이 포함되어 있습니다.',
  destructiveRisk: '이 작업은 기존 데이터를 변경하거나 삭제할 수 있습니다.',
  targetCountRisk: '이번 작업은 {count}개 대상에 적용됩니다.',
  riskPrompt: '{risk}\n\n작업: {operation}\n대상: {target}\n\n계속하려면 “{ack}”을(를) 입력하십시오.',
  productionConfirmTitle: '프로덕션 작업 확인',
  highRiskConfirmTitle: '고위험 작업 확인',
  acknowledgementRequired: '확인하려면 “{ack}”을(를) 입력하십시오.',
  confirmContinue: '확인 후 계속',
  cancel: '취소'
}

const en = {
  idleSessionExpired: 'The session ended after 6 hours without activity. Please sign in again.',
  sessionExpired: 'Your login has expired. Please sign in again.',
  requestFailed: 'Request failed.',
  networkError: 'A network error occurred.',
  tokenRefreshFailed: 'Failed to refresh the login session. Please sign in again.',
  invalidCredentials: 'The username or password is incorrect.',
  accountDisabled: 'This account is disabled. Contact an administrator.',
  permissionDenied: 'You do not have permission to perform this operation.',
  resourceNotFound: 'The requested resource was not found.',
  requiredValueMissing: 'A required value is missing.',
  resourceAlreadyExists: 'The resource already exists.',
  invalidRequest: 'The request value is invalid.',
  unsupportedOperation: 'This operation is not supported.',
  operationTimedOut: 'The operation timed out.',
  connectionFailed: 'Failed to connect to the target system.',
  operationFailed: 'The operation could not be completed.',
  unassigned: 'Unassigned',
  productionAck: 'PRODUCTION',
  destructiveAck: 'CONFIRM',
  executeAck: 'EXECUTE',
  productionRisk: 'The target includes a production environment.',
  destructiveRisk: 'This operation may modify or delete existing data.',
  targetCountRisk: 'This operation will affect {count} target(s).',
  riskPrompt: '{risk}\n\nOperation: {operation}\nTarget: {target}\n\nEnter “{ack}” to continue.',
  productionConfirmTitle: 'Production Operation Confirmation',
  highRiskConfirmTitle: 'High-Risk Operation Confirmation',
  acknowledgementRequired: 'Enter “{ack}” to confirm.',
  confirmContinue: 'Confirm and Continue',
  cancel: 'Cancel'
}

export function ct(key, params = {}) {
  const dict = currentLocale.value === 'en-US' ? en : ko
  let text = dict[key] || en[key] || key
  Object.entries(params).forEach(([name, value]) => {
    text = text.replaceAll(`{${name}}`, String(value))
  })
  return text
}
