import { currentLocale } from './i18n-runtime'

const ko = {
  connectionSuccess: 'LDAP 연결 및 Bind 검증에 성공했습니다.', saved: 'LDAP 설정을 저장했습니다.', settings: '시스템 설정', settingsDesc: '플랫폼 UI와 통합 인증 설정을 관리합니다. LDAP 비밀번호는 서버에만 저장되며 화면에 다시 표시하지 않습니다.',
  general: '일반 설정', integration: 'LDAP 인증 통합', ldapDesc: 'Directory Service를 구성하고 LDAP 사용자를 로컬 사용자 관리로 동기화합니다.', enabled: 'LDAP 사용', disabled: '사용 안 함',
  filterHint: '사용자 필터는 {{username}} placeholder를 지원합니다. 동기화 시 LDAP 사용자명으로 로컬 사용자 정보를 생성하거나 갱신합니다.', serverUrl: 'LDAP 서버 주소', transport: '전송 보안', plain: 'Plain LDAP', bindDn: 'Bind DN', bindPassword: 'Bind 비밀번호', passwordConfigured: '설정됨. 변경하지 않으려면 비워 두십시오.', passwordRequired: 'Bind 비밀번호를 입력하십시오.', userFilter: '사용자 필터',
  fieldMapping: 'LDAP 사용자 필드 매핑', usernameAttr: '사용자명 속성', displayAttr: '표시 이름 속성', emailAttr: '이메일 속성', phoneAttr: '휴대전화 속성', defaults: '동기화 기본값', defaultRole: '기본 Role', defaultDept: '기본 부서', defaultPost: '기본 직무', roleRequired: '신규 LDAP 사용자에 필요', optional: '선택', certificateValidation: '인증서 검증', skipValidation: '검증 건너뛰기', validateCertificate: '인증서 검증', testConnection: '연결 테스트', saveLdap: 'LDAP 설정 저장'
}

const en = {
  connectionSuccess: 'LDAP connection and bind validation succeeded.', saved: 'LDAP configuration saved.', settings: 'System Settings', settingsDesc: 'Manage platform appearance and unified authentication integration. LDAP passwords are stored only on the server and are never echoed to the UI.',
  general: 'General Settings', integration: 'LDAP Authentication', ldapDesc: 'Configure a directory service and synchronize LDAP users into local user management.', enabled: 'Enable LDAP', disabled: 'Disabled',
  filterHint: 'The user filter supports the {{username}} placeholder. Synchronization creates or updates local user profiles using LDAP usernames.', serverUrl: 'LDAP Server URL', transport: 'Transport Security', plain: 'Plain LDAP', bindDn: 'Bind DN', bindPassword: 'Bind Password', passwordConfigured: 'Configured. Leave blank to keep the current password.', passwordRequired: 'Enter the bind password.', userFilter: 'User Filter',
  fieldMapping: 'LDAP User Attribute Mapping', usernameAttr: 'Username Attribute', displayAttr: 'Display Name Attribute', emailAttr: 'Email Attribute', phoneAttr: 'Phone Attribute', defaults: 'Synchronization Defaults', defaultRole: 'Default Role', defaultDept: 'Default Department', defaultPost: 'Default Position', roleRequired: 'Required for new LDAP users', optional: 'Optional', certificateValidation: 'Certificate Validation', skipValidation: 'Skip Validation', validateCertificate: 'Validate Certificate', testConnection: 'Test Connection', saveLdap: 'Save LDAP Configuration'
}

export function lt(key) {
  const dict = currentLocale.value === 'en-US' ? en : ko
  return dict[key] || en[key] || key
}
