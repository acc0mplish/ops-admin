import { currentLocale } from './i18n-runtime'

const ko = {
  overviewTitle: '인프라 개요', overviewDesc: 'V2 인프라 제어평면의 프로바이더, 연결, 인벤토리 상태를 한 화면에서 확인합니다.',
  providersTitle: '프로바이더 연결', providersDesc: 'V2 파이프라인에 등록된 인프라 프로바이더 연결과 시크릿 참조를 조회합니다.',
  inventoryTitle: 'K8s 인벤토리', inventoryDesc: 'V2 동기화가 수집한 Kubernetes 리소스 인벤토리를 조회합니다.',
  refresh: '새로고침', loadFailed: '데이터를 불러오지 못했습니다.', search: '검색', reset: '초기화',
  statProviderTypes: '프로바이더 유형', statConnections: '등록 연결', statActiveConnections: '활성 연결', statResources: '인벤토리 리소스',
  typesTitle: '프로바이더 유형', connectionsTitle: '연결 목록',
  typeCol: '유형', adapterVersion: '어댑터 버전', protocolVersion: '프로토콜 버전', contextKinds: '컨텍스트 종류', builtin: '기본 제공',
  connectionName: '연결 이름', endpointCol: '엔드포인트', statusCol: '상태', versionCol: '버전', secretRefCol: '시크릿 참조', updatedAt: '수정 시각', uidCol: 'UID',
  statusActive: '활성', statusInactive: '비활성', statusUnknown: '알 수 없음',
  keywordPlaceholder: '연결 이름 / UID / 엔드포인트 검색', kindFilterLabel: '리소스 종류', kindAll: '전체',
  resourceDisplayName: '표시 이름', kindCol: '종류', subtypeCol: '하위 유형', lifecycleCol: '수명주기', healthCol: '헬스', managedCol: '관리 상태', lastSeenCol: '마지막 관측', urnCol: 'URN',
  detailTitle: '리소스 상세', detailGeneration: 'Generation', detailObservedAt: '관측 시각', detailNormalized: '정규화 데이터', detailRaw: '원본 데이터', close: '닫기',
  builtinYes: '예', builtinNo: '아니오', paginationTotal: '총 {count}개',
  infoTab: '정보', operationsTab: '오퍼레이션',
  operationCol: '오퍼레이션', mutatingCol: '변경', riskCol: '위험도', permissionCol: '권한',
  runOp: '실행', planDialogTitle: '오퍼레이션 계획 미리보기',
  planRestartedAt: '재시작 기준시각', planResourceRevision: '리소스 개정', planPolicyVersion: '정책 버전',
  planExecuteNote: '실행 시 Idempotency-Key가 발급되어 태스크가 생성됩니다.',
  executeSuccess: '태스크가 생성되었습니다: {uid}',
  noOperations: '이 리소스에 사용 가능한 오퍼레이션이 없습니다.',
  tasksTitle: '작업 및 승인', tasksDesc: 'V2 오퍼레이션 태스크의 실행 상태와 승인 대기열을 조회하고 승인·거부·취소를 수행합니다.',
  statusFilterLabel: '상태 필터', statusAll: '전체',
  uidLookupPlaceholder: '태스크 UID 조회', lookup: '조회',
  taskUidCol: '태스크 UID', approvalCol: '승인 상태', approverCol: '승인자', resourceCol: '리소스 UID', attemptsCol: '시도', createdAtCol: '생성 시각',
  taskDetailTitle: '작업 상세',
  cancelRequestedCol: '취소 요청', requiresApprovalCol: '승인 필요', errorCodeCol: '에러 코드', errorMessageCol: '에러 메시지',
  detailPayload: '실행 Payload', detailEvents: '이벤트 타임라인',
  approve: '승인', reject: '거부', cancelTask: '취소',
  approveSuccess: '승인되었습니다.', rejectSuccess: '거부되었습니다.', cancelSuccess: '취소가 요청되었습니다.',
  actionFailed: '작업을 수행하지 못했습니다.', eventAttempt: '시도'
}

const en = {
  overviewTitle: 'Infrastructure Overview', overviewDesc: 'Providers, connections, and inventory posture of the V2 infrastructure control plane in one view.',
  providersTitle: 'Provider Connections', providersDesc: 'Inspect infrastructure provider connections and their secret references registered in the V2 pipeline.',
  inventoryTitle: 'K8s Inventory', inventoryDesc: 'Browse the Kubernetes resource inventory collected by the V2 sync pipeline.',
  refresh: 'Refresh', loadFailed: 'Failed to load data.', search: 'Search', reset: 'Reset',
  statProviderTypes: 'Provider Types', statConnections: 'Connections', statActiveConnections: 'Active Connections', statResources: 'Inventory Resources',
  typesTitle: 'Provider Types', connectionsTitle: 'Connections',
  typeCol: 'Type', adapterVersion: 'Adapter Version', protocolVersion: 'Protocol Version', contextKinds: 'Context Kinds', builtin: 'Built-in',
  connectionName: 'Connection Name', endpointCol: 'Endpoint', statusCol: 'Status', versionCol: 'Version', secretRefCol: 'Secret Ref', updatedAt: 'Updated At', uidCol: 'UID',
  statusActive: 'Active', statusInactive: 'Inactive', statusUnknown: 'Unknown',
  keywordPlaceholder: 'Search connection name / UID / endpoint', kindFilterLabel: 'Resource Kind', kindAll: 'All',
  resourceDisplayName: 'Display Name', kindCol: 'Kind', subtypeCol: 'Subtype', lifecycleCol: 'Lifecycle', healthCol: 'Health', managedCol: 'Managed State', lastSeenCol: 'Last Seen', urnCol: 'URN',
  detailTitle: 'Resource Detail', detailGeneration: 'Generation', detailObservedAt: 'Observed At', detailNormalized: 'Normalized Data', detailRaw: 'Raw Data', close: 'Close',
  builtinYes: 'Yes', builtinNo: 'No', paginationTotal: '{count} total',
  infoTab: 'Info', operationsTab: 'Operations',
  operationCol: 'Operation', mutatingCol: 'Mutating', riskCol: 'Risk Level', permissionCol: 'Permission',
  runOp: 'Run', planDialogTitle: 'Operation Plan Preview',
  planRestartedAt: 'Restarted At', planResourceRevision: 'Resource Revision', planPolicyVersion: 'Policy Version',
  planExecuteNote: 'Executing issues an Idempotency-Key and creates a task.',
  executeSuccess: 'Task created: {uid}',
  noOperations: 'No operations available for this resource.',
  tasksTitle: 'Tasks & Approvals', tasksDesc: 'Inspect V2 operation task execution states and the approval queue, then approve, reject, or cancel.',
  statusFilterLabel: 'Status Filter', statusAll: 'All',
  uidLookupPlaceholder: 'Look up task UID', lookup: 'Look Up',
  taskUidCol: 'Task UID', approvalCol: 'Approval', approverCol: 'Approver', resourceCol: 'Resource UID', attemptsCol: 'Attempts', createdAtCol: 'Created At',
  taskDetailTitle: 'Task Detail',
  cancelRequestedCol: 'Cancel Requested', requiresApprovalCol: 'Requires Approval', errorCodeCol: 'Error Code', errorMessageCol: 'Error Message',
  detailPayload: 'Execution Payload', detailEvents: 'Event Timeline',
  approve: 'Approve', reject: 'Reject', cancelTask: 'Cancel',
  approveSuccess: 'Approved.', rejectSuccess: 'Rejected.', cancelSuccess: 'Cancellation requested.',
  actionFailed: 'Failed to perform the operation.', eventAttempt: 'Attempt'
}

export function inft(key, params = {}) {
  const dict = currentLocale.value === 'en-US' ? en : ko
  let text = dict[key] || en[key] || key
  Object.entries(params).forEach(([name, value]) => { text = text.replaceAll(`{${name}}`, String(value)) })
  return text
}
