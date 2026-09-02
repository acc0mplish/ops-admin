import { currentLocale } from './i18n-runtime'

const ko = {
  selectAccountRange: 'Cloud Account와 시작/종료 날짜를 먼저 선택하십시오.',
  resourceBreakdown: 'Resource 비용 분석',
  resourceBreakdownDesc: 'Cloud Account와 날짜 범위를 선택하면 Resource별 Billing 비용을 집계합니다.',
  selectCloudAccount: 'Cloud Account 선택',
  startDate: '시작 날짜',
  endDate: '종료 날짜',
  startBreakdown: '분석 시작',
  filterHint: 'Cloud Account와 날짜 범위를 선택하면 자동으로 조회합니다.',
  resourceCostDetail: 'Resource 비용 상세',
  resourceCount: '총 {count}개 Resource',
  allRegions: '전체 Region',
  allResourceTypes: '전체 Resource Type',
  resourceName: 'Resource 이름',
  unlinkedResource: '연결되지 않은 Resource',
  resourceType: 'Resource Type',
  uncategorized: '미분류',
  resourceConfig: 'Resource 구성',
  cloudProvider: 'Cloud Provider',
  region: 'Region',
  unavailable: '미제공',
  originalPrice: '원가',
  discount: '할인',
  actualPayment: '실제 결제',
  billingEntries: 'Billing 항목',
  noResourceCost: '현재 필터 조건에 Resource 비용이 없습니다.'
}

const en = {
  selectAccountRange: 'Select a cloud account, start date, and end date first.',
  resourceBreakdown: 'Resource Cost Breakdown',
  resourceBreakdownDesc: 'Select a cloud account and date range to aggregate billing costs by resource.',
  selectCloudAccount: 'Select Cloud Account',
  startDate: 'Start Date',
  endDate: 'End Date',
  startBreakdown: 'Start Breakdown',
  filterHint: 'The query runs automatically after a cloud account and date range are selected.',
  resourceCostDetail: 'Resource Cost Details',
  resourceCount: '{count} resources',
  allRegions: 'All Regions',
  allResourceTypes: 'All Resource Types',
  resourceName: 'Resource Name',
  unlinkedResource: 'Unlinked Resource',
  resourceType: 'Resource Type',
  uncategorized: 'Uncategorized',
  resourceConfig: 'Resource Configuration',
  cloudProvider: 'Cloud Provider',
  region: 'Region',
  unavailable: 'Unavailable',
  originalPrice: 'Original Price',
  discount: 'Discount',
  actualPayment: 'Actual Payment',
  billingEntries: 'Billing Entries',
  noResourceCost: 'No resource cost data matches the current filters.'
}

export function ft(key, params = {}) {
  const dict = currentLocale.value === 'en-US' ? en : ko
  let text = dict[key] || en[key] || key
  Object.entries(params).forEach(([name, value]) => { text = text.replaceAll(`{${name}}`, String(value)) })
  return text
}
