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
  queryTestDesc: '현재 플랫폼의 메모리 snapshot과 upstream forwarding 경로를 사용해 Internal/Public Domain을 테스트하며 MySQL hot path를 읽지 않습니다.',
  startResolution: '조회 시작',
  resolutionSuccess: '조회 성공',
  resolutionFailure: '조회 실패',
  responseCode: '응답 코드',
  noAnswers: 'DNS record가 반환되지 않았습니다.',
  waitingResolution: '조회 대기',
  waitingResolutionDesc: '결과에 Record Value, TTL, Response Time과 DNS Server를 표시합니다.',
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
  queryTestDesc: 'Tests internal or public domains using the platform memory snapshot and upstream forwarding path without reading the MySQL hot path.',
  startResolution: 'Start Resolution',
  resolutionSuccess: 'Resolution Succeeded',
  resolutionFailure: 'Resolution Failed',
  responseCode: 'Response Code',
  noAnswers: 'No DNS records were returned.',
  waitingResolution: 'Waiting for Resolution',
  waitingResolutionDesc: 'Results show record values, TTL, response time, and the DNS server.',
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
