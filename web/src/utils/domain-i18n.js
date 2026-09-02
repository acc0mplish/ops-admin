import { currentLocale } from './i18n-runtime'

const ko = {
  auditTitle: '도메인 작업 감사',
  auditDesc: 'Public/Internal DNS의 작업자, Source, Object, 이전 값, 새 값과 실행 결과를 기록합니다.',
  refresh: '새로고침',
  time: '시간',
  operator: '작업 사용자',
  sourceIp: 'Source IP',
  actionType: '작업 유형',
  domain: '도메인',
  type: '유형',
  change: '변경',
  result: '결과',
  operationSuccess: '작업 성공',
  success: '성공',
  failure: '실패',
  queryTestTitle: 'DNS 조회 테스트',
  internalSettingsTitle: 'Internal DNS 설정',
  publicDomainsTitle: 'Public Domain',
  search: '조회',
  save: '저장',
  test: '테스트',
  enabled: '활성',
  disabled: '비활성'
}

const en = {
  auditTitle: 'Domain Operation Audit',
  auditDesc: 'Records operators, sources, objects, previous values, new values, and per-item execution results for public and internal DNS changes.',
  refresh: 'Refresh',
  time: 'Time',
  operator: 'Operator',
  sourceIp: 'Source IP',
  actionType: 'Action Type',
  domain: 'Domain',
  type: 'Type',
  change: 'Change',
  result: 'Result',
  operationSuccess: 'Operation succeeded',
  success: 'Success',
  failure: 'Failure',
  queryTestTitle: 'DNS Query Test',
  internalSettingsTitle: 'Internal DNS Settings',
  publicDomainsTitle: 'Public Domains',
  search: 'Search',
  save: 'Save',
  test: 'Test',
  enabled: 'Enabled',
  disabled: 'Disabled'
}

export function dt(key, params = {}) {
  const dict = currentLocale.value === 'en-US' ? en : ko
  let text = dict[key] || en[key] || key
  Object.entries(params).forEach(([name, value]) => {
    text = text.replaceAll(`{${name}}`, String(value))
  })
  return text
}
