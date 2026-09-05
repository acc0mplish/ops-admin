import { currentLocale } from './i18n-runtime'

const ko = {
  userManagement: '사용자 관리', searchUsername: '사용자명으로 검색', query: '검색', syncFromLdap: 'LDAP에서 동기화', addUser: '사용자 추가',
  account: '계정', displayName: '표시 이름', role: 'Role', department: '부서', position: '직무', email: '이메일', phone: '휴대전화', status: '상태', active: '활성', inactive: '비활성', actions: '작업',
  edit: '수정', deactivate: '비활성화', activate: '활성화', resetPassword: '비밀번호 초기화', delete: '삭제', cancel: '취소', save: '저장', create: '생성', password: '비밀번호', note: '비고', reset: '초기화', detail: '상세',
  editUser: '사용자 수정', passwordKeep: '변경하지 않으려면 비워 두십시오.', passwordInput: '비밀번호를 입력하십시오.', ldapSyncTitle: 'LDAP 사용자 동기화',
  ldapSyncTip: 'Directory Service에서 사용자를 먼저 조회한 뒤 동기화할 계정을 선택하십시오. 기존 로컬 사용자는 표시 이름, 이메일, 휴대전화만 갱신하며 비밀번호, Role, 상태는 덮어쓰지 않습니다.',
  ldapFilter: '사용자명으로 LDAP 사용자 필터링', ldapQuery: 'LDAP 조회', username: '사용자명', syncSelected: '선택 사용자 동기화 ({count})',
  userUpdated: '사용자 정보를 업데이트했습니다.', userCreated: '사용자를 생성했습니다.', deleteUserConfirm: '사용자 {name}을(를) 삭제하시겠습니까?', deleteConfirm: '삭제 확인', userDeleted: '사용자를 삭제했습니다.',
  userStatusUpdated: '사용자 상태를 변경했습니다.', passwordResetDefault: '비밀번호를 123456으로 초기화했습니다.', ldapSelectRequired: '동기화할 LDAP 사용자를 하나 이상 선택하십시오.', ldapSyncResult: 'LDAP 동기화 완료: 신규 {created}명, 업데이트 {updated}명',
  departmentManagement: '부서 관리', addDepartment: '부서 추가', editDepartment: '부서 수정', departmentName: '부서명', departmentType: '부서 유형', parentDepartment: '상위 부서', rootDepartment: '최상위 부서', company: '회사', center: '센터',
  positionManagement: '직무 관리', addPosition: '직무 추가', editPosition: '직무 수정', positionCode: '직무 코드', positionName: '직무명', remark: '설명',
  roleManagement: 'Role 관리', addRole: 'Role 추가', editRole: 'Role 수정', roleName: 'Role 이름', roleKey: 'Role Key', description: '설명', assignPermissions: '권한 할당',
  menuManagement: '메뉴 관리', addMenu: '메뉴 추가', editMenu: '메뉴 수정', menuName: '메뉴명', menuType: '메뉴 유형', directory: '디렉터리', page: '페이지', permission: '권한', route: 'Route', permissionKey: 'Permission Key', icon: 'Icon', sort: '정렬', parentMenu: '상위 메뉴', rootMenu: '최상위 메뉴',
  loginLog: '로그인 로그', operationLog: '작업 로그', loginAccount: '로그인 계정', ipAddress: 'IP 주소', loginTime: '로그인 시각', result: '결과', success: '성공', failure: '실패', clear: '전체 삭제',
  auditTotal: '감사 로그 수', auditTotalHint: '현재 필터 범위의 작업 기록', highRiskOperations: '고위험 작업', highRiskHint: '삭제, SQL, Kubernetes, 배포 등 민감 작업', failedOperations: '실패 작업', failedOperationsHint: 'API 실패 또는 예외가 발생한 작업', averageDuration: '평균 소요 시간', averageDurationHint: '지연 작업과 느린 API 분석',
  searchAccount: '계정으로 검색', searchOperation: '설명 / URL / IP / Parameter', riskLevel: '위험도', highRisk: '높음', mediumRisk: '중간', normalRisk: '일반', executionResult: '실행 결과', batchDelete: '선택 삭제', clearLogs: '로그 비우기', risk: '위험', method: 'Method', duration: '소요 시간', statusCode: 'Status Code', operationTime: '작업 시각', sourceIp: 'Source IP', requestMethod: 'Request Method', operationAuditDetail: '작업 감사 상세', requestSummary: 'Request Summary', noRequestParameters: 'Request parameter 없음',
  createdSuccess: '생성했습니다.', updatedSuccess: '수정했습니다.', deletedSuccess: '삭제했습니다.', statusUpdated: '상태를 변경했습니다.', confirmDelete: '{name}을(를) 삭제하시겠습니까?', clearLogsConfirm: '로그를 모두 삭제하시겠습니까?'
}

const en = {
  userManagement: 'User Management', searchUsername: 'Search by username', query: 'Search', syncFromLdap: 'Sync from LDAP', addUser: 'Add User',
  account: 'Account', displayName: 'Display Name', role: 'Role', department: 'Department', position: 'Position', email: 'Email', phone: 'Phone', status: 'Status', active: 'Active', inactive: 'Inactive', actions: 'Actions',
  edit: 'Edit', deactivate: 'Deactivate', activate: 'Activate', resetPassword: 'Reset Password', delete: 'Delete', cancel: 'Cancel', save: 'Save', create: 'Create', password: 'Password', note: 'Note', reset: 'Reset', detail: 'Detail',
  editUser: 'Edit User', passwordKeep: 'Leave blank to keep the current password.', passwordInput: 'Enter a password.', ldapSyncTitle: 'LDAP User Sync',
  ldapSyncTip: 'Preview directory users first, then select accounts to sync. Existing local users only update display name, email, and phone; password, role, and status are preserved.',
  ldapFilter: 'Filter LDAP users by username', ldapQuery: 'Query LDAP', username: 'Username', syncSelected: 'Sync Selected Users ({count})',
  userUpdated: 'User updated.', userCreated: 'User created.', deleteUserConfirm: 'Delete user {name}?', deleteConfirm: 'Confirm Delete', userDeleted: 'User deleted.',
  userStatusUpdated: 'User status updated.', passwordResetDefault: 'Password reset to 123456.', ldapSelectRequired: 'Select at least one LDAP user.', ldapSyncResult: 'LDAP sync complete: {created} created, {updated} updated',
  departmentManagement: 'Department Management', addDepartment: 'Add Department', editDepartment: 'Edit Department', departmentName: 'Department Name', departmentType: 'Department Type', parentDepartment: 'Parent Department', rootDepartment: 'Root Department', company: 'Company', center: 'Center',
  positionManagement: 'Position Management', addPosition: 'Add Position', editPosition: 'Edit Position', positionCode: 'Position Code', positionName: 'Position Name', remark: 'Description',
  roleManagement: 'Role Management', addRole: 'Add Role', editRole: 'Edit Role', roleName: 'Role Name', roleKey: 'Role Key', description: 'Description', assignPermissions: 'Assign Permissions',
  menuManagement: 'Menu Management', addMenu: 'Add Menu', editMenu: 'Edit Menu', menuName: 'Menu Name', menuType: 'Menu Type', directory: 'Directory', page: 'Page', permission: 'Permission', route: 'Route', permissionKey: 'Permission Key', icon: 'Icon', sort: 'Sort', parentMenu: 'Parent Menu', rootMenu: 'Root Menu',
  loginLog: 'Login Log', operationLog: 'Operation Log', loginAccount: 'Login Account', ipAddress: 'IP Address', loginTime: 'Login Time', result: 'Result', success: 'Success', failure: 'Failure', clear: 'Clear All',
  auditTotal: 'Audit Records', auditTotalHint: 'Operations in the current filter scope', highRiskOperations: 'High-Risk Operations', highRiskHint: 'Sensitive actions such as delete, SQL, Kubernetes, and deployment', failedOperations: 'Failed Operations', failedOperationsHint: 'Operations where the API failed or raised an exception', averageDuration: 'Average Duration', averageDurationHint: 'Helps locate slow operations and APIs',
  searchAccount: 'Search by account', searchOperation: 'Description / URL / IP / Parameters', riskLevel: 'Risk Level', highRisk: 'High', mediumRisk: 'Medium', normalRisk: 'Normal', executionResult: 'Execution Result', batchDelete: 'Delete Selected', clearLogs: 'Clear Logs', risk: 'Risk', method: 'Method', duration: 'Duration', statusCode: 'Status Code', operationTime: 'Operation Time', sourceIp: 'Source IP', requestMethod: 'Request Method', operationAuditDetail: 'Operation Audit Detail', requestSummary: 'Request Summary', noRequestParameters: 'No request parameters',
  createdSuccess: 'Created successfully.', updatedSuccess: 'Updated successfully.', deletedSuccess: 'Deleted successfully.', statusUpdated: 'Status updated.', confirmDelete: 'Delete {name}?', clearLogsConfirm: 'Delete all logs?'
}

export function st(key, params = {}) {
  const dict = currentLocale.value === 'en-US' ? en : ko
  let text = dict[key] || en[key] || key
  Object.entries(params).forEach(([name, value]) => {
    text = text.replaceAll(`{${name}}`, String(value))
  })
  return text
}
