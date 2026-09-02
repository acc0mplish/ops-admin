import { computed, ref } from 'vue'
import { t as koText, translateEntity as translateKoEntity, translateRoute as translateKoRoute } from './i18n'

const DISPLAY_LOCALE_KEY = 'ops-admin-display-locale'
export const DEFAULT_LOCALE = 'ko-KR'
export const FALLBACK_LOCALE = 'en-US'
export const SUPPORTED_LOCALES = ['ko-KR', 'en-US']

const ko = {
  loginHeroSubtitle: '통합 운영 관리 플랫폼',
  passwordFieldsRequired: '비밀번호 정보를 모두 입력하십시오.',
  passwordMinLength: '새 비밀번호는 6자 이상이어야 합니다.',
  passwordMismatch: '새 비밀번호와 확인 비밀번호가 일치하지 않습니다.',
  department: '부서',
  position: '직책',
  status: '상태',
  active: '활성',
  inactive: '비활성',
  noPersonalNote: '등록된 개인 설명이 없습니다.',
  createdAt: '생성 일시',
  updatedAt: '수정 일시',
  administrator: '관리자',
  consoleScope: '콘솔 작업 범위',
  systemGovernance: '시스템 거버넌스',
  permissionConfiguration: '권한 구성',
  auditTrail: '감사 추적',
  siteSloganDefault: '개인 운영 관리 플랫폼',
  platformNavigation: '플랫폼 탐색',
  applicationPlatform: '애플리케이션 플랫폼',
  applicationPlatformHint: '애플리케이션을 선택해 워크스페이스와 메뉴를 전환합니다.',
  menuEntries: '메뉴 {count}개',
  globalSearch: '전체 검색',
  currentApplication: '현재 애플리케이션',
  switchApplication: '전환',
  closeLeftTabs: '왼쪽 탭 닫기',
  closeRightTabs: '오른쪽 탭 닫기',
  closeOtherTabs: '다른 탭 닫기',
  quickJumpSearch: '빠른 이동 및 검색',
  searchPagesPlaceholder: '전체 애플리케이션의 페이지, 모듈 또는 경로 검색',
  noMatchingPage: '일치하는 페이지 또는 작업이 없습니다.',
  untitledPage: '미지정 페이지'
}

const en = {
  language: 'Korean',
  english: 'English',
  switchLanguage: 'Language',
  dashboard: 'Dashboard',
  profile: 'Profile',
  system: 'System Management',
  logs: 'Audit Logs',
  admin: 'User Management',
  role: 'Role Management',
  menu: 'Menu Management',
  dept: 'Department Management',
  post: 'Position Management',
  basicConfig: 'Basic Config',
  loginLog: 'Login Log',
  operationLog: 'Operation Log',
  userCenter: 'User Center',
  logout: 'Log out',
  editProfile: 'Edit Profile',
  changePassword: 'Change Password',
  accountInfo: 'Account Information',
  extraInfo: 'Additional Information',
  saveConfig: 'Save Config',
  configTitle: 'Basic Config',
  configDesc: 'Manage site title, login copy, logo, and primary UI color.',
  siteName: 'Site Name',
  siteSlogan: 'Site Slogan',
  logoType: 'Logo Type',
  logoValue: 'Logo Value',
  loginTitle: 'Login Title',
  loginSubtitle: 'Login Subtitle',
  useLoginBackground: 'Use Background',
  loginBackground: 'Background URL',
  primaryColor: 'Primary Color',
  sidebarTheme: 'Sidebar Theme',
  darkTheme: 'Dark',
  lightTheme: 'Light',
  textLogo: 'Text Logo',
  iconStyle: 'Icon Style',
  uploadLogo: 'Upload',
  imageUrl: 'Image URL',
  chooseImage: 'Choose Image',
  uploadBackground: 'Upload Background',
  logoPreview: 'Logo Preview',
  loginPreview: 'Login Preview',
  welcomeBack: 'Welcome back, {name}',
  dashboardDesc: 'Review and extend the core capabilities required for operations management.',
  userCount: 'Users',
  roleCount: 'Roles',
  deptCount: 'Departments',
  postCount: 'Positions',
  userCountNote: 'System accounts',
  roleCountNote: 'Permission roles',
  deptCountNote: 'Organization nodes',
  postCountNote: 'Defined positions',
  loginWelcome: 'Sign in',
  loginHint: 'Enter your account and password to access the console.',
  loginSystem: 'Sign in',
  loginHeroSubtitle: 'Integrated operations management platform',
  username: 'Username',
  password: 'Password',
  nickname: 'Display Name',
  email: 'Email',
  phone: 'Phone',
  note: 'Note',
  confirmPassword: 'Confirm Password',
  currentPassword: 'Current Password',
  newPassword: 'New Password',
  cancel: 'Cancel',
  save: 'Save',
  confirmChange: 'Confirm Change',
  saveSuccess: 'Saved successfully.',
  profileUpdateSuccess: 'Profile updated successfully.',
  passwordUpdateSuccess: 'Password changed successfully.',
  passwordFieldsRequired: 'Enter all password fields.',
  passwordMinLength: 'The new password must be at least 6 characters.',
  passwordMismatch: 'The new password and confirmation do not match.',
  department: 'Department',
  position: 'Position',
  status: 'Status',
  active: 'Active',
  inactive: 'Inactive',
  noPersonalNote: 'No personal description has been registered.',
  createdAt: 'Created At',
  updatedAt: 'Updated At',
  administrator: 'Administrator',
  superAdmin: 'Super Administrator',
  headquarters: 'Headquarters',
  noDept: 'No department assigned',
  noPost: 'No position assigned',
  appSwitch: 'Applications',
  appConsole: 'Console',
  appAssets: 'Asset Management',
  appContainers: 'Container Management',
  appOps: 'Standard Operations',
  appApplications: 'Application Center',
  appNotify: 'Notifications',
  appMonitor: 'Monitoring Center',
  appIntegration: 'Integration Center',
  appDomains: 'Domain Management',
  integrationNavigation: 'Navigation',
  integrationAI: 'AI Assistant',
  integrationAIChat: 'AI Chat',
  integrationAIConversations: 'Conversations',
  integrationAIModels: 'Models',
  integrationAIKnowledgeBase: 'Knowledge Base',
  integrationAITools: 'Tool Registry',
  integrationCloudCost: 'Cloud Cost Analysis',
  integrationCostDashboard: 'Cost Dashboard',
  integrationCloudAccounts: 'Cloud Accounts',
  integrationCostBreakdown: 'Cost Breakdown',
  integrationCostRecommendations: 'Optimization Recommendations',
  integrationCostResources: 'Resource Breakdown',
  integrationCostSync: 'Billing Sync',
  assetsOverview: 'Asset Overview',
  serverManagement: 'Server Management',
  hostManagement: 'Host Management',
  hostGroupManagement: 'Host Group Management',
  credentialManagement: 'Credential Management',
  passwordAuth: 'Password Auth',
  keyAuth: 'SSH Key Auth',
  cloudAccountManagement: 'Cloud Account Management',
  databaseManagement: 'Database Management',
  databaseList: 'Database List',
  databaseWorkbench: 'DBMS Workbench',
  databaseImport: 'Data Import',
  databaseBackup: 'Backup Management',
  gatewayManagement: 'Gateway Management',
  assetsHosts: 'Host Management',
  assetsTags: 'Tags',
  opsOverview: 'Operations Overview',
  opsJobs: 'Job Center',
  opsScripts: 'Scripts',
  opsReleases: 'Release Orchestration',
  opsScriptLibrary: 'Script Library',
  opsEnvironments: 'Environment Model',
  opsApplicationCenter: 'Application Center',
  applicationCenter: 'Application Center',
  appProjectManage: 'Application Management',
  appProjectList: 'Application Management',
  appBuildDeploy: 'Build & Deploy',
  appBuildTasks: 'Build Tasks',
  appBuildHistory: 'Build History',
  appPipelines: 'CI/CD Pipelines',
  appTopology: 'Application Topology',
  opsQuickExecute: 'Quick Execute',
  opsCommandExecute: 'Command Execution',
  opsScriptExecute: 'Script Execution',
  opsFileDispatch: 'File Distribution',
  opsExecutionHistory: 'Quick Execution History',
  opsSchedule: 'Scheduled Tasks',
  opsScheduleTasks: 'Task List',
  opsScheduleLogs: 'Task Logs',
  opsScheduleTemplates: 'Task Templates',
  opsJobDesigner: 'Job Designer',
  opsJobList: 'Job List',
  opsJobApprovals: 'Approvals',
  opsJobHistory: 'Job History',
  opsJobTemplates: 'Job Templates',
  notifyRules: 'Notification Rules',
  notifyTemplates: 'Message Templates',
  notifyChannels: 'Notification Channels',
  notifySendLogs: 'Send Logs',
  monitorOverview: 'Monitoring Overview',
  monitorAlerts: 'Alert Events',
  monitorMetrics: 'Instant Query',
  monitorDatasources: 'Datasources',
  monitorQuery: 'Instant Query',
  monitorLogs: 'Log Explorer',
  monitorTraces: 'Trace Explorer',
  monitorAlertRules: 'Alert Rules',
  monitorAlertEvents: 'Alert Events',
  monitorSilences: 'Silences',
  monitorAggregations: 'Aggregations',
  monitorDashboards: 'Monitoring Dashboards',
  monitorInspections: 'Inspection Dashboards',
  moduleReady: 'This module entry is ready for operational capabilities.',
  k8sManagement: 'Kubernetes Management',
  k8sClusters: 'Clusters',
  k8sOverview: 'Cluster Overview',
  k8sNodes: 'Nodes',
  k8sNamespaces: 'Namespaces',
  k8sWorkloads: 'Workloads',
  k8sPods: 'Pods',
  k8sServices: 'Services',
  k8sIngresses: 'Ingress',
  k8sAdvancedNetwork: 'Advanced Network',
  k8sConfigStorage: 'Config & Storage',
  k8sPodTerminal: 'Pod Terminal',
  consoleScope: 'Console scope',
  systemGovernance: 'System governance',
  permissionConfiguration: 'Permission configuration',
  auditTrail: 'Audit trail',
  siteSloganDefault: 'Personal operations platform',
  platformNavigation: 'Platform navigation',
  applicationPlatform: 'Application platform',
  applicationPlatformHint: 'Select an application to switch its workspace and navigation.',
  menuEntries: '{count} menu entries',
  globalSearch: 'Global search',
  currentApplication: 'Current application',
  switchApplication: 'Switch',
  closeLeftTabs: 'Close tabs to the left',
  closeRightTabs: 'Close tabs to the right',
  closeOtherTabs: 'Close other tabs',
  quickJumpSearch: 'Quick navigation and search',
  searchPagesPlaceholder: 'Search pages, modules, or paths across all applications',
  noMatchingPage: 'No matching pages or actions found.',
  untitledPage: 'Untitled page'
}

function humanizeKey(key) {
  return String(key || '')
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .replace(/^k8s\b/i, 'Kubernetes')
    .replace(/\bApi\b/g, 'API')
    .replace(/\bDns\b/g, 'DNS')
    .replace(/\bSsl\b/g, 'SSL')
    .replace(/\bAi\b/g, 'AI')
    .replace(/\bDbms\b/g, 'DBMS')
    .replace(/\bYaml\b/g, 'YAML')
    .replace(/\bHttp\b/g, 'HTTP')
    .replace(/\bIp\b/g, 'IP')
    .replace(/^./, (char) => char.toUpperCase())
}

const savedLocale = localStorage.getItem(DISPLAY_LOCALE_KEY)
export const locale = ref(SUPPORTED_LOCALES.includes(savedLocale) ? savedLocale : DEFAULT_LOCALE)

if (!SUPPORTED_LOCALES.includes(savedLocale)) {
  localStorage.setItem(DISPLAY_LOCALE_KEY, DEFAULT_LOCALE)
}

export function setLocale(value) {
  const next = SUPPORTED_LOCALES.includes(value) ? value : DEFAULT_LOCALE
  locale.value = next
  localStorage.setItem(DISPLAY_LOCALE_KEY, next)
}

export function getLocale() {
  return locale.value
}

export function t(key, params = {}) {
  let text
  if (locale.value === FALLBACK_LOCALE) {
    text = en[key] || humanizeKey(key)
  } else {
    text = ko[key] || koText(key, params)
  }
  Object.keys(params).forEach((name) => {
    text = text.replaceAll(`{${name}}`, String(params[name]))
  })
  return text
}

export function translateRoute(path, fallback = '') {
  if (locale.value === DEFAULT_LOCALE) {
    return translateKoRoute(path, fallback)
  }
  return fallback || path || ''
}

export function translateEntity(value, fallback = '-') {
  if (locale.value === DEFAULT_LOCALE) {
    return translateKoEntity(value, fallback)
  }
  const map = {
    'Super Admin': 'Super Administrator',
    'Super Administrator': 'Super Administrator',
    Headquarters: 'Headquarters'
  }
  return map[value] || value || fallback
}

export function useI18nState() {
  return {
    locale: computed(() => locale.value),
    t
  }
}

export const currentLocale = computed(() => locale.value)
