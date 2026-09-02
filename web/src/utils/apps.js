export const APP_KEY = 'ops-admin-current-app'

const leaf = (title, path, icon, titleKey) => ({ title, ...(titleKey ? { titleKey } : {}), path, icon, children: [] })

export const appDefinitions = [
  {
    key: 'console',
    name: 'Console',
    labelKey: 'appConsole',
    icon: 'Monitor',
    defaultRoute: '/dashboard',
    menuSource: 'backend'
  },
  {
    key: 'assets',
    name: 'Asset Management',
    labelKey: 'appAssets',
    icon: 'Box',
    defaultRoute: '/assets/overview',
    menus: [
      leaf('Asset Overview', '/assets/overview', 'DataBoard', 'assetsOverview'),
      leaf('Environment Model', '/assets/environments', 'SetUp', 'opsEnvironments'),
      leaf('Terminal Login', '/assets/terminal', 'Platform'),
      {
        title: 'Server Management', titleKey: 'serverManagement', path: '/assets/server', icon: 'Cpu',
        children: [
          leaf('Host Management', '/assets/server/hosts', 'Monitor', 'hostManagement'),
          leaf('Host Group Management', '/assets/server/groups', 'FolderOpened', 'hostGroupManagement'),
          leaf('Credential Management', '/assets/server/credentials', 'Key', 'credentialManagement'),
          leaf('Cloud Account Management', '/assets/server/cloud-accounts', 'Cloudy', 'cloudAccountManagement')
        ]
      },
      {
        title: 'Database Management', titleKey: 'databaseManagement', path: '/assets/databases', icon: 'Coin',
        children: [
          leaf('Database List', '/assets/databases', 'List', 'databaseList'),
          leaf('DBMS Workbench', '/assets/databases/workbench', 'EditPen', 'databaseWorkbench'),
          leaf('Data Import', '/assets/databases/import', 'Upload', 'databaseImport'),
          leaf('Backup Management', '/assets/databases/backups', 'FolderOpened', 'databaseBackup')
        ]
      },
      leaf('Gateway Management', '/assets/gateways', 'Switch', 'gatewayManagement')
    ]
  },
  {
    key: 'containers',
    name: 'Container Management',
    labelKey: 'appContainers',
    icon: 'Box',
    defaultRoute: '/containers/k8s/clusters',
    menus: [
      leaf('Service Management', '/containers/services', 'Grid'),
      leaf('Service Health Diagnosis', '/containers/services/health-diagnosis', 'Monitor'),
      {
        title: 'Kubernetes Management', titleKey: 'k8sManagement', path: '/containers/k8s', icon: 'Connection',
        children: [
          leaf('Cluster Management', '/containers/k8s/clusters', 'FolderOpened', 'k8sClusters'),
          leaf('Cluster Overview', '/containers/k8s/overview', 'DataAnalysis', 'k8sOverview'),
          leaf('Node Management', '/containers/k8s/nodes', 'Monitor', 'k8sNodes'),
          leaf('Namespace', '/containers/k8s/namespaces', 'Grid', 'k8sNamespaces'),
          leaf('Workload', '/containers/k8s/workloads', 'SetUp', 'k8sWorkloads'),
          leaf('Pod Management', '/containers/k8s/pods', 'Box', 'k8sPods'),
          leaf('Service', '/containers/k8s/services', 'Share', 'k8sServices'),
          leaf('Ingress', '/containers/k8s/ingresses', 'Connection', 'k8sIngresses'),
          leaf('Advanced Network', '/containers/k8s/advanced-network', 'Connection', 'k8sAdvancedNetwork'),
          leaf('Config & Storage', '/containers/k8s/config-storage', 'Files', 'k8sConfigStorage')
        ]
      }
    ]
  },
  {
    key: 'ops',
    name: 'Standard Operations',
    labelKey: 'appOps',
    icon: 'Operation',
    defaultRoute: '/ops/quick-exec/command',
    menus: [
      leaf('Script Library', '/ops/scripts/library', 'Document', 'opsScriptLibrary'),
      {
        title: 'Quick Execute', titleKey: 'opsQuickExecute', path: '/ops/quick-exec', icon: 'Timer',
        children: [
          leaf('Command Execution', '/ops/quick-exec/command', 'CaretRight', 'opsCommandExecute'),
          leaf('Script Execution', '/ops/quick-exec/script', 'EditPen', 'opsScriptExecute'),
          leaf('File Distribution', '/ops/quick-exec/file-dispatch', 'Files', 'opsFileDispatch'),
          leaf('Quick Execution History', '/ops/quick-exec/history', 'DocumentCopy', 'opsExecutionHistory')
        ]
      },
      {
        title: 'Scheduled Tasks', titleKey: 'opsSchedule', path: '/ops/schedule', icon: 'Clock',
        children: [
          leaf('Task List', '/ops/schedule/tasks', 'List', 'opsScheduleTasks'),
          leaf('Task Logs', '/ops/schedule/logs', 'Document', 'opsScheduleLogs'),
          leaf('Task Templates', '/ops/schedule/templates', 'Tickets', 'opsScheduleTemplates')
        ]
      },
      {
        title: 'Jobs', titleKey: 'opsJobs', path: '/ops/jobs', icon: 'Share',
        children: [
          leaf('Job Designer', '/ops/jobs/designer', 'Grid', 'opsJobDesigner'),
          leaf('Job List', '/ops/jobs/list', 'List', 'opsJobList'),
          leaf('Approvals', '/ops/jobs/approvals', 'Bell', 'opsJobApprovals'),
          leaf('Job History', '/ops/jobs/history', 'Document', 'opsJobHistory'),
          leaf('Job Templates', '/ops/jobs/templates', 'Tickets', 'opsJobTemplates')
        ]
      }
    ]
  },
  {
    key: 'applications',
    name: 'Application Center',
    labelKey: 'appApplications',
    icon: 'Box',
    defaultRoute: '/applications/projects',
    menus: [
      leaf('Application Management', '/applications/projects', 'Tickets', 'appProjectList'),
      {
        title: 'Build & Deploy', titleKey: 'appBuildDeploy', path: '/applications/build-tasks', icon: 'SetUp',
        children: [
          leaf('Build Tasks', '/applications/build-tasks', 'Operation', 'appBuildTasks'),
          leaf('Build History', '/applications/build-history', 'Document', 'appBuildHistory')
        ]
      },
      leaf('Image Registry', '/applications/image-registries', 'Box'),
      leaf('CI/CD Pipeline', '/applications/pipelines', 'Share', 'appPipelines')
    ]
  },
  {
    key: 'notify',
    name: 'Notifications',
    labelKey: 'appNotify',
    icon: 'Bell',
    defaultRoute: '/notify/rules',
    menus: [
      leaf('Notification Rules', '/notify/rules', 'Operation', 'notifyRules'),
      leaf('Message Templates', '/notify/templates', 'Document', 'notifyTemplates'),
      leaf('Notification Channels', '/notify/channels', 'Connection', 'notifyChannels'),
      leaf('Send Logs', '/notify/send-logs', 'Tickets', 'notifySendLogs')
    ]
  },
  {
    key: 'integration',
    name: 'Integration Center',
    labelKey: 'appIntegration',
    icon: 'Grid',
    defaultRoute: '/integration/navigation',
    menus: [
      leaf('Navigation Management', '/integration/navigation', 'Grid', 'integrationNavigation'),
      {
        title: 'AI Assistant', titleKey: 'integrationAI', path: '/integration/ai/chat', icon: 'ChatLineRound',
        children: [
          leaf('AI Chat', '/integration/ai/chat', 'ChatDotRound', 'integrationAIChat'),
          leaf('Conversation Management', '/integration/ai/conversations', 'Clock', 'integrationAIConversations'),
          leaf('Model Management', '/integration/ai/models', 'Cpu', 'integrationAIModels'),
          leaf('Knowledge Base', '/integration/ai/knowledge-base', 'Document', 'integrationAIKnowledgeBase'),
          leaf('Toolset', '/integration/ai/tools', 'Operation', 'integrationAITools')
        ]
      },
      {
        title: 'Cloud Cost Analysis', titleKey: 'integrationCloudCost', path: '/integration/finops/dashboard', icon: 'Coin',
        children: [
          leaf('Cost Dashboard', '/integration/finops/dashboard', 'DataAnalysis', 'integrationCostDashboard'),
          leaf('Cloud Accounts', '/integration/finops/accounts', 'CreditCard', 'integrationCloudAccounts'),
          leaf('Cost Breakdown', '/integration/finops/breakdown', 'PieChart', 'integrationCostBreakdown'),
          leaf('Optimization Recommendations', '/integration/finops/recommendations', 'Opportunity', 'integrationCostRecommendations'),
          leaf('Resource Breakdown', '/integration/finops/resources', 'Box', 'integrationCostResources'),
          leaf('Billing Sync', '/integration/finops/sync', 'Refresh', 'integrationCostSync')
        ]
      }
    ]
  },
  {
    key: 'monitor',
    name: 'Monitoring Center',
    labelKey: 'appMonitor',
    icon: 'Histogram',
    defaultRoute: '/monitor/overview',
    menus: [
      leaf('Monitoring Overview', '/monitor/overview', 'TrendCharts', 'monitorOverview'),
      leaf('Command Center Dashboard', '/monitor/command-center', 'DataBoard'),
      leaf('Datasource Management', '/monitor/datasources', 'Connection', 'monitorDatasources'),
      leaf('Instant Query', '/monitor/query', 'Search', 'monitorQuery'),
      leaf('Log Explorer', '/monitor/logs', 'Document', 'monitorLogs'),
      leaf('Trace Explorer', '/monitor/traces', 'Share', 'monitorTraces'),
      {
        title: 'Alert Management', path: '/monitor/alert-management', icon: 'Bell',
        children: [
          leaf('Alert Templates', '/monitor/alert-templates', 'CollectionTag'),
          leaf('Alert Rules', '/monitor/alert-rules', 'Bell', 'monitorAlertRules'),
          leaf('Alert Events', '/monitor/alert-events', 'Warning', 'monitorAlertEvents'),
          leaf('Silences', '/monitor/silences', 'MuteNotification', 'monitorSilences'),
          leaf('Alert Aggregation', '/monitor/aggregations', 'Filter', 'monitorAggregations')
        ]
      },
      leaf('Monitoring Dashboard', '/monitor/dashboards', 'PieChart', 'monitorDashboards'),
      leaf('Inspection Dashboard', '/monitor/inspections', 'Tickets', 'monitorInspections')
    ]
  },
  {
    key: 'domains',
    name: 'Domain Management',
    labelKey: 'appDomains',
    icon: 'Connection',
    defaultRoute: '/domains/public',
    menus: [
      {
        title: 'Public DNS', path: '/domains/public', icon: 'Position',
        children: [
          leaf('Domain List', '/domains/public', 'List'),
          leaf('DNS Accounts', '/domains/public/accounts', 'Key'),
          leaf('SSL Certificates', '/domains/public/certificates', 'Lock')
        ]
      },
      {
        title: 'Private DNS', path: '/domains/internal', icon: 'OfficeBuilding',
        children: [
          leaf('Zone Management', '/domains/internal', 'Collection'),
          leaf('DNS Settings', '/domains/internal/settings', 'Setting')
        ]
      },
      {
        title: 'Diagnostics & Audit', path: '/domains/query-test', icon: 'DataAnalysis',
        children: [
          leaf('DNS Query Test', '/domains/query-test', 'Search'),
          leaf('Operation Audit', '/domains/audit', 'Memo')
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
