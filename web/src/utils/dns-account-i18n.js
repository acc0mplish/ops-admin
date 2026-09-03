import { currentLocale } from './i18n-runtime'

const ko = {
  title: 'Public DNS Account', description: 'Aliyun DNS와 Tencent Cloud DNSPod를 연결합니다. Credential은 암호화 저장하며 화면에는 마스킹된 식별자만 표시합니다.', add: 'DNS Account 추가', searchName: 'Account 이름 검색', provider: 'Provider', search: '조회', accountName: 'Account 이름', credentialHint: 'Credential 식별자', connectionStatus: '연결 상태', healthy: '연결 정상', failed: '연결 실패', notTested: '미테스트', accountStatus: 'Account 상태', enable: '활성화', disable: '비활성화', lastTest: '최근 테스트', actions: '작업', testConnection: '연결 테스트', edit: '수정', delete: '삭제', noAccounts: 'Public DNS Account가 없습니다.', noAccountsDesc: 'Cloud Provider Account를 추가한 뒤 Public Domain을 동기화할 수 있습니다.', editTitle: 'DNS Account 수정', addTitle: 'DNS Account 추가', dnsProvider: 'DNS Provider', leaveBlank: '비워 두면 기존 값을 유지합니다.', credentialSecurity: 'Credential은 AES-GCM으로 암호화 저장하며 목록 및 상세 API는 원본 값을 반환하지 않습니다.', status: '상태', cancel: '취소', saveAccount: 'Account 저장', accountNameRequired: 'Account 이름을 입력하십시오.', credentialsRequired: 'Cloud Provider Credential을 모두 입력하십시오.', updated: 'DNS Account를 업데이트했습니다.', created: 'DNS Account를 생성했습니다.', connectionSucceeded: '연결에 성공했습니다.', deleteTitle: 'DNS Account 삭제', deleteConfirm: 'Account “{name}”을 삭제하면 Domain Sync Snapshot도 함께 제거됩니다. 계속하시겠습니까?', deleted: 'DNS Account를 삭제했습니다.', aliyun: 'Aliyun DNS', tencent: 'Tencent Cloud DNSPod'
}

const en = {
  title: 'Public DNS Accounts', description: 'Connect Aliyun DNS and Tencent Cloud DNSPod. Credentials are encrypted at rest and only masked identifiers are shown in the UI.', add: 'Add DNS Account', searchName: 'Search account name', provider: 'Provider', search: 'Search', accountName: 'Account Name', credentialHint: 'Credential Identifier', connectionStatus: 'Connection Status', healthy: 'Connected', failed: 'Connection Failed', notTested: 'Not Tested', accountStatus: 'Account Status', enable: 'Enable', disable: 'Disable', lastTest: 'Last Test', actions: 'Actions', testConnection: 'Test Connection', edit: 'Edit', delete: 'Delete', noAccounts: 'No public DNS accounts configured.', noAccountsDesc: 'Add a cloud provider account before synchronizing public domains.', editTitle: 'Edit DNS Account', addTitle: 'Add DNS Account', dnsProvider: 'DNS Provider', leaveBlank: 'Leave blank to keep the existing value.', credentialSecurity: 'Credentials are encrypted with AES-GCM; list and detail APIs never return the original values.', status: 'Status', cancel: 'Cancel', saveAccount: 'Save Account', accountNameRequired: 'Enter an account name.', credentialsRequired: 'Enter all cloud provider credentials.', updated: 'DNS account updated.', created: 'DNS account created.', connectionSucceeded: 'Connection succeeded.', deleteTitle: 'Delete DNS Account', deleteConfirm: 'Deleting account “{name}” will also remove its domain synchronization snapshot. Continue?', deleted: 'DNS account deleted.', aliyun: 'Aliyun DNS', tencent: 'Tencent Cloud DNSPod'
}

export function dat(key, params = {}) {
  const dict = currentLocale.value === 'en-US' ? en : ko
  let text = dict[key] || en[key] || key
  Object.entries(params).forEach(([name, value]) => { text = text.replaceAll(`{${name}}`, String(value)) })
  return text
}
