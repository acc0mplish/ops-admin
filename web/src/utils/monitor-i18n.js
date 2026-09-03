import { currentLocale } from './i18n-runtime'

const ko = {
  traceSearch: 'Trace 조회', traceSearchDesc: 'Jaeger에서 Distributed Trace를 조회해 Service 호출 지연과 오류 Span을 분석합니다.', noJaegerDatasource: 'Jaeger Datasource가 구성되지 않았습니다.', noJaegerDatasourceDesc: 'Datasource 관리에서 Jaeger Query Service 주소를 추가하고 연결을 확인하십시오.', selectJaegerDatasource: 'Jaeger Datasource 선택', selectService: 'Service 선택', selectOperationOptional: 'Operation 선택 (선택 사항)', last15Minutes: '최근 15분', lastHour: '최근 1시간', last6Hours: '최근 6시간', last24Hours: '최근 24시간', last7Days: '최근 7일', customTime: '사용자 지정', startTime: '시작 시간', endTime: '종료 시간', to: '부터', tagJsonExample: 'Tag JSON, 예: {"http.status_code":"500"}', traceIdDirect: 'Trace ID (직접 조회 가능)', query: '조회', currentDatasourceHint: '현재 Datasource: {name}. Tag Filter는 Jaeger 형식의 JSON Object를 사용합니다.', selectServiceOrTrace: 'Service를 선택해 조회하거나 Trace ID를 입력해 호출 흐름을 직접 찾으십시오.', servicePath: 'Service Path', status: '상태', startedAt: 'Started At', totalDuration: 'Total Duration', spanCount: 'Span 수', actions: '작업', detail: '상세', pageSize: '페이지당 {count}건', success: '성공', abnormal: '오류', selectServiceWarning: 'Service를 선택하거나 Trace ID를 직접 입력하십시오.', selectFullTimeRange: '시작 시간과 종료 시간을 모두 선택하십시오.', configureJaegerFirst: '먼저 Datasource 관리에서 Jaeger Datasource를 구성하십시오.'
}

const en = {
  traceSearch: 'Trace Search', traceSearchDesc: 'Search distributed traces in Jaeger to analyze service latency and failing spans.', noJaegerDatasource: 'No Jaeger Datasource Configured', noJaegerDatasourceDesc: 'Add and verify a Jaeger Query service endpoint in Datasource Management first.', selectJaegerDatasource: 'Select Jaeger Datasource', selectService: 'Select Service', selectOperationOptional: 'Select Operation (Optional)', last15Minutes: 'Last 15 Minutes', lastHour: 'Last 1 Hour', last6Hours: 'Last 6 Hours', last24Hours: 'Last 24 Hours', last7Days: 'Last 7 Days', customTime: 'Custom Time', startTime: 'Start Time', endTime: 'End Time', to: 'to', tagJsonExample: 'Tag JSON, e.g. {"http.status_code":"500"}', traceIdDirect: 'Trace ID (Direct Lookup)', query: 'Query', currentDatasourceHint: 'Current datasource: {name}. Tag filters must use a Jaeger-format JSON object.', selectServiceOrTrace: 'Select a service and query, or enter a Trace ID to locate a trace directly.', servicePath: 'Service Path', status: 'Status', startedAt: 'Started At', totalDuration: 'Total Duration', spanCount: 'Span Count', actions: 'Actions', detail: 'Detail', pageSize: '{count} per page', success: 'Success', abnormal: 'Error', selectServiceWarning: 'Select a service or enter a Trace ID directly.', selectFullTimeRange: 'Select both start and end time.', configureJaegerFirst: 'Configure a Jaeger datasource in Datasource Management first.'
}

export function mt(key, params = {}) {
  const dict = currentLocale.value === 'en-US' ? en : ko
  let text = dict[key] || en[key] || key
  Object.entries(params).forEach(([name, value]) => { text = text.replaceAll(`{${name}}`, String(value)) })
  return text
}
