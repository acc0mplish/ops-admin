import { currentLocale } from './i18n-runtime'

const ko = {
  selectServiceWorkloadForLogs: 'Service와 Workload를 선택한 뒤 Pod Log를 확인하십시오.',
  podLogs: 'Pod Log',
  podLogsHint: '기본값은 최근 200줄이며 최대 최근 1000줄까지 조회할 수 있습니다.',
  recentLines: '최근 {count}줄',
  refreshLogs: 'Log 새로고침',
  noLogContent: '표시할 Log 내용이 없습니다.',
  selectPodForLogs: 'Pod를 선택하면 최근 200줄의 Log를 불러옵니다.',
  noDatabaseConnection: '사용 가능한 Database 연결이 없습니다. Database 목록에서 연결을 먼저 구성하십시오.',
  enteringDatabaseWorkbench: 'DBMS Workbench로 이동하고 있습니다...'
}

const en = {
  selectServiceWorkloadForLogs: 'Select a service and workload to view Pod logs.',
  podLogs: 'Pod Logs',
  podLogsHint: 'The default is the latest 200 lines; up to 1000 recent lines can be retrieved.',
  recentLines: 'Latest {count} lines',
  refreshLogs: 'Refresh Logs',
  noLogContent: 'No log content available.',
  selectPodForLogs: 'Select a Pod to load the latest 200 log lines.',
  noDatabaseConnection: 'No database connection is available. Configure a connection in the database list first.',
  enteringDatabaseWorkbench: 'Opening DBMS Workbench...'
}

export function at(key, params = {}) {
  const dict = currentLocale.value === 'en-US' ? en : ko
  let text = dict[key] || en[key] || key
  Object.entries(params).forEach(([name, value]) => { text = text.replaceAll(`{${name}}`, String(value)) })
  return text
}
