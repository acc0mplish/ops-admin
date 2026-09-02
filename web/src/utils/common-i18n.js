import { currentLocale } from './i18n-runtime'

const ko = {
  idleSessionExpired: '6시간 동안 활동이 없어 세션이 종료되었습니다. 다시 로그인하십시오.',
  sessionExpired: '로그인이 만료되었습니다. 다시 로그인하십시오.',
  requestFailed: '요청에 실패했습니다.',
  networkError: '네트워크 오류가 발생했습니다.',
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
