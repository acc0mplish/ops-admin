export const APP_KEY = 'ops-admin-current-app'

const leaf = (title, path, icon, titleKey) => ({ title, ...(titleKey ? { titleKey } : {}), path, icon, children: [] })

export const appDefinitions = [
  {
    key: 'console',
    name: '콘솔',
    labelKey: 'appConsole',
    icon: 'Monitor',
    defaultRoute: '/dashboard',
    menuSource: 'backend'
  },
  {
    key: 'assets',
    name: '자산 관리',
    labelKey: 'appAssets',
    icon: 'Box',
    defaultRoute: '/assets/overview',
    menus: [
      leaf('자산 개요', '/assets/overview', 'DataBoard', 'assetsOverview'),
      leaf('환경 모델', '/assets/environments', 'SetUp', 'opsEnvironments'),
      leaf('터미널 로그인', '/assets/terminal', 'Platform'),
      {
        title: '서버 관리', titleKey: 'serverManagement', path: '/assets/server', icon: 'Cpu',
        children: [
          leaf('호스트 관리', '/assets/server/hosts', 'Monitor', 'hostManagement'),
          leaf('호스트 그룹 관리', '/assets/server/groups', 'FolderOpened', 'hostGroupManagement'),
          leaf('Credential 관리', '/assets/server/credentials', 'Key', 'credentialManagement'),
          leaf('클라우드 계정 관리', '/assets/server/cloud-accounts', 'Cloudy', 'cloudAccountManagement')
        ]
      },
      {
        title: '데이터베이스 관리', titleKey: 'databaseManagement', path: '/assets/databases', icon: 'Coin',
        children: [
          leaf('데이터베이스 목록', '/assets/databases', 'List', 'databaseList'),
          leaf('DBMS 워크벤치', '/assets/databases/workbench', 'EditPen', 'databaseWorkbench'),
          leaf('데이터 Import', '/assets/databases/import', 'Upload', 'databaseImport'),
          leaf('백업 관리', '/assets/databases/backups', 'FolderOpened', 'databaseBackup')
        ]
      },
      leaf('Gateway 관리', '/assets/gateways', 'Switch', 'gatewayManagement')
    ]
  },
  {
    key: 'containers',
    name: '컨테이너 관리',
    labelKey: 'appContainers',
    icon: 'Box',
    defaultRoute: '/containers/k8s/clusters',
    menus: [
      leaf('서비스 관리', '/containers/services', 'Grid'),
      leaf('서비스 상태 진단', '/containers/services/health-diagnosis', 'Monitor'),
      {
        title: 'Kubernetes 관리', titleKey: 'k8sManagement', path: '/containers/k8s', icon: 'Connection',
        children: [
          leaf('클러스터 관리', '/containers/k8s/clusters', 'FolderOpened', 'k8sClusters'),
          leaf('클러스터 개요', '/containers/k8s/overview', 'DataAnalysis', 'k8sOverview'),
          leaf('Node 관리', '/containers/k8s/nodes', 'Monitor', 'k8sNodes'),
          leaf('Namespace', '/containers/k8s/namespaces', 'Grid', 'k8sNamespaces'),
          leaf('Workload', '/containers/k8s/workloads', 'SetUp', 'k8sWorkloads'),
          leaf('Pod 관리', '/containers/k8s/pods', 'Box', 'k8sPods'),
          leaf('Service', '/containers/k8s/services', 'Share', 'k8sServices'),
          leaf('Ingress', '/containers/k8s/ingresses', 'Connection', 'k8sIngresses'),
          leaf('고급 네트워크', '/containers/k8s/advanced-network', 'Connection', 'k8sAdvancedNetwork'),
          leaf('Config & Storage', '/containers/k8s/config-storage', 'Files', 'k8sConfigStorage')
        ]
      }
    ]
  },
  {
    key: 'ops',
    name: '표준 운영',
    labelKey: 'appOps',
    icon: 'Operation',
    defaultRoute: '/ops/quick-exec/command',
    menus: [
      leaf('스크립트 라이브러리', '/ops/scripts/library', 'Document', 'opsScriptLibrary'),
      {
        title: '빠른 실행', titleKey: 'opsQuickExecute', path: '/ops/quick-exec', icon: 'Timer',
        children: [
          leaf('명령 실행', '/ops/quick-exec/command', 'CaretRight', 'opsCommandExecute'),
          leaf('스크립트 실행', '/ops/quick-exec/script', 'EditPen', 'opsScriptExecute'),
          leaf('파일 배포', '/ops/quick-exec/file-dispatch', 'Files', 'opsFileDispatch'),
          leaf('빠른 실행 이력', '/ops/quick-exec/history', 'DocumentCopy', 'opsExecutionHistory')
        ]
      },
      {
        title: '스케줄 작업', titleKey: 'opsSchedule', path: '/ops/schedule', icon: 'Clock',
        children: [
          leaf('작업 목록', '/ops/schedule/tasks', 'List', 'opsScheduleTasks'),
          leaf('작업 로그', '/ops/schedule/logs', 'Document', 'opsScheduleLogs'),
          leaf('작업 템플릿', '/ops/schedule/templates', 'Tickets', 'opsScheduleTemplates')
        ]
      },
      {
        title: 'Job', titleKey: 'opsJobs', path: '/ops/jobs', icon: 'Share',
        children: [
          leaf('Job 오케스트레이션', '/ops/jobs/designer', 'Grid', 'opsJobDesigner'),
          leaf('Job 목록', '/ops/jobs/list', 'List', 'opsJobList'),
          leaf('수동 승인', '/ops/jobs/approvals', 'Bell', 'opsJobApprovals'),
          leaf('Job 이력', '/ops/jobs/history', 'Document', 'opsJobHistory'),
          leaf('Job 템플릿', '/ops/jobs/templates', 'Tickets', 'opsJobTemplates')
        ]
      }
    ]
  },
  {
    key: 'applications',
    name: '애플리케이션 센터',
    labelKey: 'appApplications',
    icon: 'Box',
    defaultRoute: '/applications/projects',
    menus: [
      leaf('애플리케이션 관리', '/applications/projects', 'Tickets', 'appProjectList'),
      {
        title: 'Build & Deploy', titleKey: 'appBuildDeploy', path: '/applications/build-tasks', icon: 'SetUp',
        children: [
          leaf('Build 작업', '/applications/build-tasks', 'Operation', 'appBuildTasks'),
          leaf('Build 이력', '/applications/build-history', 'Document', 'appBuildHistory')
        ]
      },
      leaf('Image Registry', '/applications/image-registries', 'Box'),
      leaf('CI/CD Pipeline', '/applications/pipelines', 'Share', 'appPipelines')
    ]
  },
  {
    key: 'notify',
    name: '알림',
    labelKey: 'appNotify',
    icon: 'Bell',
    defaultRoute: '/notify/rules',
    menus: [
      leaf('알림 규칙', '/notify/rules', 'Operation', 'notifyRules'),
      leaf('메시지 템플릿', '/notify/templates', 'Document', 'notifyTemplates'),
      leaf('알림 채널', '/notify/channels', 'Connection', 'notifyChannels'),
      leaf('전송 로그', '/notify/send-logs', 'Tickets', 'notifySendLogs')
    ]
  },
  {
    key: 'integration',
    name: '통합 센터',
    labelKey: 'appIntegration',
    icon: 'Grid',
    defaultRoute: '/integration/navigation',
    menus: [
      leaf('Navigation 관리', '/integration/navigation', 'Grid', 'integrationNavigation'),
      {
        title: 'AI Assistant', titleKey: 'integrationAI', path: '/integration/ai/chat', icon: 'ChatLineRound',
        children: [
          leaf('AI 대화', '/integration/ai/chat', 'ChatDotRound', 'integrationAIChat'),
          leaf('대화 세션 관리', '/integration/ai/conversations', 'Clock', 'integrationAIConversations'),
          leaf('모델 관리', '/integration/ai/models', 'Cpu', 'integrationAIModels'),
          leaf('Knowledge Base 관리', '/integration/ai/knowledge-base', 'Document', 'integrationAIKnowledgeBase'),
          leaf('Toolset', '/integration/ai/tools', 'Operation', 'integrationAITools')
        ]
      },
      {
        title: '클라우드 비용 분석', titleKey: 'integrationCloudCost', path: '/integration/finops/dashboard', icon: 'Coin',
        children: [
          leaf('비용 Dashboard', '/integration/finops/dashboard', 'DataAnalysis', 'integrationCostDashboard'),
          leaf('클라우드 계정', '/integration/finops/accounts', 'CreditCard', 'integrationCloudAccounts'),
          leaf('비용 Breakdown', '/integration/finops/breakdown', 'PieChart', 'integrationCostBreakdown'),
          leaf('최적화 권고', '/integration/finops/recommendations', 'Opportunity', 'integrationCostRecommendations'),
          leaf('리소스 Breakdown', '/integration/finops/resources', 'Box', 'integrationCostResources'),
          leaf('청구 데이터 동기화', '/integration/finops/sync', 'Refresh', 'integrationCostSync')
        ]
      }
    ]
  },
  {
    key: 'monitor',
    name: '모니터링 센터',
    labelKey: 'appMonitor',
    icon: 'Histogram',
    defaultRoute: '/monitor/overview',
    menus: [
      leaf('모니터링 개요', '/monitor/overview', 'TrendCharts', 'monitorOverview'),
      leaf('통합 관제 Dashboard', '/monitor/command-center', 'DataBoard'),
      leaf('Datasource 관리', '/monitor/datasources', 'Connection', 'monitorDatasources'),
      leaf('즉시 Query', '/monitor/query', 'Search', 'monitorQuery'),
      leaf('로그 Query', '/monitor/logs', 'Document', 'monitorLogs'),
      leaf('Trace', '/monitor/traces', 'Share', 'monitorTraces'),
      {
        title: 'Alert 관리', path: '/monitor/alert-management', icon: 'Bell',
        children: [
          leaf('Alert 템플릿', '/monitor/alert-templates', 'CollectionTag'),
          leaf('Alert 규칙', '/monitor/alert-rules', 'Bell', 'monitorAlertRules'),
          leaf('Alert 이벤트', '/monitor/alert-events', 'Warning', 'monitorAlertEvents'),
          leaf('Silence', '/monitor/silences', 'MuteNotification', 'monitorSilences'),
          leaf('Alert 집계', '/monitor/aggregations', 'Filter', 'monitorAggregations')
        ]
      },
      leaf('모니터링 Dashboard', '/monitor/dashboards', 'PieChart', 'monitorDashboards'),
      leaf('점검 Dashboard', '/monitor/inspections', 'Tickets', 'monitorInspections')
    ]
  },
  {
    key: 'domains',
    name: '도메인 관리',
    labelKey: 'appDomains',
    icon: 'Connection',
    defaultRoute: '/domains/public',
    menus: [
      {
        title: 'Public DNS', path: '/domains/public', icon: 'Position',
        children: [
          leaf('도메인 목록', '/domains/public', 'List'),
          leaf('DNS 계정', '/domains/public/accounts', 'Key'),
          leaf('SSL 인증서', '/domains/public/certificates', 'Lock')
        ]
      },
      {
        title: 'Private DNS', path: '/domains/internal', icon: 'OfficeBuilding',
        children: [
          leaf('Zone 관리', '/domains/internal', 'Collection'),
          leaf('DNS 설정', '/domains/internal/settings', 'Setting')
        ]
      },
      {
        title: '진단 및 Audit', path: '/domains/query-test', icon: 'DataAnalysis',
        children: [
          leaf('DNS 조회 테스트', '/domains/query-test', 'Search'),
          leaf('작업 Audit', '/domains/audit', 'Memo')
        ]
      }
    ]
  }
]

export function getAppByKey(key) {
  return appDefinitions.find((item) => item.key === key) || appDefinitions[0]
}

export function getAppByRoute(path) {
  if (path.startsWith('/assets')) return getAppByKey('assets')
  if (path.startsWith('/containers')) return getAppByKey('containers')
  if (path.startsWith('/ops')) return getAppByKey('ops')
  if (path.startsWith('/applications')) return getAppByKey('applications')
  if (path.startsWith('/notify')) return getAppByKey('notify')
  if (path.startsWith('/monitor')) return getAppByKey('monitor')
  if (path.startsWith('/integration')) return getAppByKey('integration')
  if (path.startsWith('/domains')) return getAppByKey('domains')
  return getAppByKey('console')
}

export function setCurrentApp(key) {
  localStorage.setItem(APP_KEY, key)
}

export function getCurrentApp() {
  return localStorage.getItem(APP_KEY) || 'console'
}
