import { currentLocale } from './i18n-runtime'

const ko = {
  targetHosts: 'Target Host', selectHosts: 'Host 선택', clear: '초기화', hostGroup: 'Host Group', selectHostGroup: 'Host Group 선택', targetPreview: 'Target Preview', noTargetHosts: '선택한 Target Host가 없습니다.', selectTargetHosts: 'Target Host 선택', hostName: 'Host 이름', sshUser: 'SSH User', cancel: '취소', confirm: '확인',
  executionResult: '실행 결과', executionStatus: '실행 상태', success: '성공', failure: '실패', pending: '대기', running: '실행 중', host: 'Host', message: 'Message', duration: 'Duration', stdout: 'Standard Output', stderr: 'Standard Error', close: '닫기',
  targetSelection: 'Target 선택', environment: 'Environment', allEnvironments: '전체 Environment', searchHosts: 'Host 검색', selectedCount: '{count}개 선택', noHosts: '사용 가능한 Host가 없습니다.'
}

const en = {
  targetHosts: 'Target Hosts', selectHosts: 'Select Hosts', clear: 'Clear', hostGroup: 'Host Group', selectHostGroup: 'Select Host Group', targetPreview: 'Target Preview', noTargetHosts: 'No target hosts selected.', selectTargetHosts: 'Select Target Hosts', hostName: 'Host Name', sshUser: 'SSH User', cancel: 'Cancel', confirm: 'Confirm',
  executionResult: 'Execution Result', executionStatus: 'Execution Status', success: 'Success', failure: 'Failure', pending: 'Pending', running: 'Running', host: 'Host', message: 'Message', duration: 'Duration', stdout: 'Standard Output', stderr: 'Standard Error', close: 'Close',
  targetSelection: 'Target Selection', environment: 'Environment', allEnvironments: 'All Environments', searchHosts: 'Search Hosts', selectedCount: '{count} selected', noHosts: 'No hosts are available.'
}

export function ot(key, params = {}) {
  const dict = currentLocale.value === 'en-US' ? en : ko
  let text = dict[key] || en[key] || key
  Object.entries(params).forEach(([name, value]) => { text = text.replaceAll(`{${name}}`, String(value)) })
  return text
}
