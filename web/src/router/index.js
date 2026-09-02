import { createRouter, createWebHistory } from 'vue-router'

import MainLayout from '../layouts/MainLayout.vue'
import Login from '../views/Login.vue'
import Dashboard from '../views/Dashboard.vue'
import BusinessTopology from '../views/BusinessTopology.vue'
import Profile from '../views/Profile.vue'
import Admin from '../views/system/Admin.vue'
import Role from '../views/system/Role.vue'
import Menu from '../views/system/Menu.vue'
import Dept from '../views/system/Dept.vue'
import Post from '../views/system/Post.vue'
import LoginLog from '../views/system/LoginLog.vue'
import OperationLog from '../views/system/OperationLog.vue'
import SystemSettings from '../views/system/SystemSettings.vue'
import Host from '../views/assets/Host.vue'
import HostGroup from '../views/assets/HostGroup.vue'
import Credential from '../views/assets/Credential.vue'
import CloudAccount from '../views/assets/CloudAccount.vue'
import AssetOverview from '../views/assets/AssetOverview.vue'
import AssetDetail from '../views/assets/AssetDetail.vue'
import Database from '../views/assets/Database.vue'
import DatabaseWorkbench from '../views/assets/DatabaseWorkbench.vue'
import DatabaseWorkbenchEntry from '../views/assets/DatabaseWorkbenchEntry.vue'
import DatabaseImport from '../views/assets/DatabaseImport.vue'
import DatabaseBackup from '../views/assets/DatabaseBackup.vue'
import Gateway from '../views/assets/Gateway.vue'
import TerminalLogin from '../views/assets/Terminal.vue'
import K8s from '../views/assets/K8s.vue'
import K8sClusterManage from '../views/assets/K8sClusterManage.vue'
import K8sPodTerminal from '../views/assets/K8sPodTerminal.vue'
import AssetApplication from '../views/assets/Application.vue'
import AssetApplicationTopology from '../views/assets/ApplicationTopology.vue'
import AssetServiceWorkloadDetail from '../views/assets/ServiceWorkloadDetail.vue'
import AssetServiceWorkloadLogs from '../views/assets/ServiceWorkloadLogs.vue'
import AssetServiceHealthDiagnosis from '../views/assets/ServiceHealthDiagnosis.vue'
import OpsScriptLibrary from '../views/ops/OpsScriptLibrary.vue'
import OpsExecutionHistory from '../views/ops/OpsExecutionHistory.vue'
import OpsCommandExecute from '../views/ops/OpsCommandExecute.vue'
import OpsScriptExecute from '../views/ops/OpsScriptExecute.vue'
import OpsFileDispatch from '../views/ops/OpsFileDispatch.vue'
import OpsScheduleTaskList from '../views/ops/OpsScheduleTaskList.vue'
import OpsScheduleLog from '../views/ops/OpsScheduleLog.vue'
import OpsScheduleTemplate from '../views/ops/OpsScheduleTemplate.vue'
import OpsJobDesigner from '../views/ops/OpsJobDesigner.vue'
import OpsJobList from '../views/ops/OpsJobList.vue'
import OpsJobApprovals from '../views/ops/OpsJobApprovals.vue'
import OpsJobHistory from '../views/ops/OpsJobHistory.vue'
import OpsJobTemplate from '../views/ops/OpsJobTemplate.vue'
import OpsEnvironment from '../views/ops/OpsEnvironment.vue'
import AppProjectList from '../views/applications/AppProjectList.vue'
import AppBuildTaskList from '../views/applications/AppBuildTaskList.vue'
import AppBuildHistory from '../views/applications/AppBuildHistory.vue'
import AppPipelineCenter from '../views/applications/AppPipelineCenter.vue'
import AppImageRegistry from '../views/applications/AppImageRegistry.vue'
import NotifyTemplate from '../views/notify/NotifyTemplate.vue'
import NotifyChannel from '../views/notify/NotifyChannel.vue'
import NotifyRule from '../views/notify/NotifyRule.vue'
import NotifySendLog from '../views/notify/NotifySendLog.vue'
import MonitorOverview from '../views/monitor/MonitorOverview.vue'
import MonitorCommandCenter from '../views/monitor/MonitorCommandCenter.vue'
import MonitorDatasource from '../views/monitor/MonitorDatasource.vue'
import MonitorQuery from '../views/monitor/MonitorQuery.vue'
import MonitorLogQuery from '../views/monitor/MonitorLogQuery.vue'
import MonitorTraceQuery from '../views/monitor/MonitorTraceQuery.vue'
import MonitorTraceDetail from '../views/monitor/MonitorTraceDetail.vue'
import MonitorAlertRule from '../views/monitor/MonitorAlertRule.vue'
import MonitorAlertTemplate from '../views/monitor/MonitorAlertTemplate.vue'
import MonitorAlertEvent from '../views/monitor/MonitorAlertEvent.vue'
import MonitorSilenceRule from '../views/monitor/MonitorSilenceRule.vue'
import MonitorAggregationRule from '../views/monitor/MonitorAggregationRule.vue'
import MonitorDashboard from '../views/monitor/MonitorDashboard.vue'
import IntegrationNavigation from '../views/integration/IntegrationNavigation.vue'
import PublicNavigation from '../views/integration/PublicNavigation.vue'
import AIAssistantChat from '../views/integration/AIAssistantChat.vue'
import AIConversations from '../views/integration/AIConversations.vue'
import AIModels from '../views/integration/AIModels.vue'
import AITools from '../views/integration/AITools.vue'
import AIKnowledgeBase from '../views/integration/AIKnowledgeBase.vue'
import FinOpsDashboard from '../views/integration/finops/FinOpsDashboard.vue'
import FinOpsAccounts from '../views/integration/finops/FinOpsAccounts.vue'
import FinOpsBreakdown from '../views/integration/finops/FinOpsBreakdown.vue'
import FinOpsRecommendations from '../views/integration/finops/FinOpsRecommendations.vue'
import FinOpsResources from '../views/integration/finops/FinOpsResources.vue'
import FinOpsSync from '../views/integration/finops/FinOpsSync.vue'
import DomainLayout from '../views/domains/DomainLayout.vue'
import PublicDomains from '../views/domains/PublicDomains.vue'
import DNSAccounts from '../views/domains/DNSAccounts.vue'
import PublicRecords from '../views/domains/PublicRecords.vue'
import InternalZones from '../views/domains/InternalZones.vue'
import InternalRecords from '../views/domains/InternalRecords.vue'
import InternalDNSSettings from '../views/domains/InternalDNSSettings.vue'
import DNSQueryTest from '../views/domains/DNSQueryTest.vue'
import DNSAudit from '../views/domains/DNSAudit.vue'
import SSLCertificates from '../views/domains/SSLCertificates.vue'
import SSLCertificateDetail from '../views/domains/SSLCertificateDetail.vue'
import { getToken } from '../utils/auth'

const routes = [
  { path: '/login', component: Login, meta: { title: 'Login' } },
  { path: '/public/navigation/:token', component: PublicNavigation, meta: { title: 'Public Navigation', public: true } },
  {
    path: '/',
    component: MainLayout,
    redirect: '/dashboard',
    children: [
      { path: '/dashboard', component: Dashboard, meta: { title: 'Dashboard', app: 'console', affix: true } },
      { path: '/business-topology', component: BusinessTopology, meta: { title: 'Business Topology', app: 'console', summary: 'Shows application business functions and bound runtime resources by environment.' } },
      { path: '/profile', component: Profile, meta: { title: 'Profile', app: 'console' } },
      { path: '/system/admin', component: Admin, meta: { title: 'Users', app: 'console' } },
      { path: '/system/role', component: Role, meta: { title: 'Roles', app: 'console' } },
      { path: '/system/menu', component: Menu, meta: { title: 'Menus', app: 'console' } },
      { path: '/system/dept', component: Dept, meta: { title: 'Departments', app: 'console' } },
      { path: '/system/post', component: Post, meta: { title: 'Positions', app: 'console' } },
      { path: '/system/settings', component: SystemSettings, meta: { title: 'System Settings', app: 'console' } },
      { path: '/system/basic-config', redirect: '/system/settings' },
      {
        path: '/domains',
        component: DomainLayout,
        redirect: '/domains/public',
        meta: { title: 'Domain Management', app: 'domains' },
        children: [
          { path: '/domains/public', component: PublicDomains, meta: { title: 'Public Domains', app: 'domains' } },
          { path: '/domains/public/accounts', component: DNSAccounts, meta: { title: 'DNS Accounts', app: 'domains' } },
          { path: '/domains/public/certificates', component: SSLCertificates, meta: { title: 'SSL Certificates', app: 'domains' } },
          { path: '/domains/public/certificates/:id', component: SSLCertificateDetail, meta: { title: 'Certificate Details', app: 'domains' } },
          { path: '/domains/public/:accountId/:domain/records', component: PublicRecords, meta: { title: 'Public DNS Records', app: 'domains' } },
          { path: '/domains/internal', component: InternalZones, meta: { title: 'Zone Management', app: 'domains' } },
          { path: '/domains/internal/zones/:zoneId/records', component: InternalRecords, meta: { title: 'Internal DNS Records', app: 'domains' } },
          { path: '/domains/internal/settings', component: InternalDNSSettings, meta: { title: 'DNS Settings', app: 'domains' } },
          { path: '/domains/query-test', component: DNSQueryTest, meta: { title: 'DNS Query Test', app: 'domains' } },
          { path: '/domains/audit', component: DNSAudit, meta: { title: 'Operation Audit', app: 'domains' } }
        ]
      },
      { path: '/logs/login', component: LoginLog, meta: { title: 'Login Log', app: 'console' } },
      { path: '/logs/operation', component: OperationLog, meta: { title: 'Operation Log', app: 'console' } },
      {
        path: '/integration/navigation',
        component: IntegrationNavigation,
        meta: { title: 'Navigation Management', app: 'integration', summary: 'Maintains grouped system shortcuts and can publish unauthenticated public navigation.' }
      },
      { path: '/integration/ai/chat', component: AIAssistantChat, meta: { title: 'AI Chat', app: 'integration', summary: 'Queries monitoring and Kubernetes through natural language and executes operations safely.' } },
      { path: '/integration/ai/conversations', component: AIConversations, meta: { title: 'Conversation Management', app: 'integration', summary: 'Manages AI assistant multi-turn conversations, context, and history.' } },
      { path: '/integration/ai/models', component: AIModels, meta: { title: 'Model Management', app: 'integration', summary: 'Configures OpenAI-compatible model APIs and default generation parameters.' } },
      { path: '/integration/ai/knowledge-base', component: AIKnowledgeBase, meta: { title: 'Knowledge Base', app: 'integration', summary: 'Maintains local Markdown knowledge documents for AI assistant retrieval.' } },
      { path: '/integration/ai/tools', component: AITools, meta: { title: 'Tool Registry', app: 'integration', summary: 'Manages PromQL, Grafana, and Kubernetes AI tools.' } },
      { path: '/integration/finops/dashboard', component: FinOpsDashboard, meta: { title: 'Cost Dashboard', app: 'integration', summary: 'Shows multi-cloud cost overview, trends, and cost structure.' } },
      { path: '/integration/finops/accounts', component: FinOpsAccounts, meta: { title: 'Cloud Accounts', app: 'integration', summary: 'Manages billing accounts for AWS, Azure, GCP, Alibaba Cloud, and Tencent Cloud.' } },
      { path: '/integration/finops/breakdown', component: FinOpsBreakdown, meta: { title: 'Cost Breakdown', app: 'integration', summary: 'Breaks down cloud cost by account, region, service, resource, and tag.' } },
      { path: '/integration/finops/recommendations', component: FinOpsRecommendations, meta: { title: 'Optimization Recommendations', app: 'integration', summary: 'Identifies idle resources, oversized resources, and cost anomalies.' } },
      { path: '/integration/finops/resources', component: FinOpsResources, meta: { title: 'Resource Breakdown', app: 'integration', summary: 'Shows cloud resource count, cost distribution, and utilization.' } },
      { path: '/integration/finops/sync', component: FinOpsSync, meta: { title: 'Billing Sync', app: 'integration', summary: 'Configures billing synchronization frequency and tracks sync history.' } },
      {
        path: '/assets/overview',
        component: AssetOverview,
        meta: { title: 'Asset Overview', app: 'assets', summary: 'Shows overall status of hosts, credentials, cloud accounts, databases, and host groups.' }
      },
      {
        path: '/assets/terminal',
        component: TerminalLogin,
        meta: { title: 'Terminal Login', app: 'assets', summary: 'Starts SSH web terminal sessions using credentials associated with a host.', keepAlive: true }
      },
      { path: '/assets/server/hosts', component: Host, meta: { title: 'Host Management', app: 'assets', summary: 'Maintains server, cloud host, and network device inventory.' } },
      { path: '/assets/server/groups', component: HostGroup, meta: { title: 'Host Group Management', app: 'assets', summary: 'Organizes asset scope for bulk operations and permission boundaries.' } },
      { path: '/assets/server/credentials', component: Credential, meta: { title: 'Credential Management', app: 'assets', summary: 'Maintains password and key-based host login credentials.' } },
      { path: '/assets/server/cloud-accounts', component: CloudAccount, meta: { title: 'Cloud Account Management', app: 'assets', summary: 'Maintains cloud provider access accounts used for cloud asset synchronization.' } },
      { path: '/assets/databases', component: Database, meta: { title: 'Database Management', app: 'assets', summary: 'Maintains MySQL database assets and opens the SQL workbench for schema, data, and execution history.' } },
      { path: '/assets/databases/:id/detail', name: 'AssetDatabaseDetail', component: AssetDetail, props: { resourceType: 'database' }, meta: { title: 'Database Details', app: 'assets' } },
      { path: '/assets/databases/workbench', component: DatabaseWorkbenchEntry, meta: { title: 'DBMS Workbench', app: 'assets' } },
      { path: '/assets/databases/import', component: DatabaseImport, meta: { title: 'Data Import', app: 'assets' } },
      { path: '/assets/databases/backups', component: DatabaseBackup, meta: { title: 'Backup Management', app: 'assets' } },
      { path: '/assets/databases/:id/workbench', name: 'DatabaseWorkbench', component: DatabaseWorkbench, meta: { title: 'DBMS Workbench', app: 'assets' } },
      { path: '/assets/server/databases', redirect: '/assets/databases' },
      { path: '/assets/server/databases/:id/workbench', redirect: (to) => `/assets/databases/${to.params.id}/workbench` },
      { path: '/assets/gateways', component: Gateway, meta: { title: 'Gateway Management', app: 'assets', summary: 'Maintains SSH bastion gateways for internal hosts, databases, and Kubernetes clusters.' } },
      { path: '/containers/services', component: AssetApplication, meta: { title: 'Service Management', app: 'containers', summary: 'Discovers workloads from Kubernetes namespaces and maintains business services and communication links.' } },
      { path: '/containers/services/health-diagnosis', component: AssetServiceHealthDiagnosis, meta: { title: 'Service Health Diagnosis', app: 'containers', summary: 'Selects service workloads, Pods, containers, and Java processes for runtime diagnostics with Arthas.' } },
      { path: '/containers/services/topology', component: AssetApplicationTopology, meta: { title: 'Service Resource Topology', app: 'containers', summary: 'Shows player entry, GM management, ZooKeeper, and game service communication links.' } },
      { path: '/containers/services/workload', component: AssetServiceWorkloadDetail, meta: { title: 'Service Details', app: 'containers', summary: 'Shows resource relationships among service workloads, Service, Deployment, ReplicaSet, and Pod.' } },
      { path: '/containers/services/logs', component: AssetServiceWorkloadLogs, meta: { title: 'Service Logs', app: 'containers', summary: 'Shows Kubernetes logs for Pods in the selected service workload.' } },
      { path: '/assets/services/:pathMatch(.*)*', redirect: (to) => `/containers/services/${to.params.pathMatch || ''}` },
      { path: '/containers/k8s', redirect: '/containers/k8s/clusters' },
      { path: '/containers/k8s/clusters', component: K8sClusterManage, meta: { title: 'Cluster Management', app: 'containers' } },
      { path: '/containers/k8s/clusters/:id/detail', name: 'K8sClusterDetail', component: AssetDetail, props: { resourceType: 'k8s' }, meta: { title: 'Cluster Details', app: 'containers' } },
      { path: '/containers/k8s/overview', component: K8s, meta: { title: 'Cluster Overview', app: 'containers' } },
      { path: '/containers/k8s/nodes', component: K8s, meta: { title: 'Node Management', app: 'containers' } },
      { path: '/containers/k8s/namespaces', component: K8s, meta: { title: 'Namespaces', app: 'containers' } },
      { path: '/containers/k8s/workloads', component: K8s, meta: { title: 'Workloads', app: 'containers' } },
      { path: '/containers/k8s/pods', component: K8s, meta: { title: 'Pods', app: 'containers' } },
      { path: '/containers/k8s/network', redirect: '/containers/k8s/services' },
      { path: '/containers/k8s/services', component: K8s, meta: { title: 'Services', app: 'containers' } },
      { path: '/containers/k8s/ingresses', component: K8s, meta: { title: 'Ingress', app: 'containers' } },
      { path: '/containers/k8s/advanced-network', component: K8s, meta: { title: 'Advanced Network', app: 'containers' } },
      { path: '/containers/k8s/config-storage', component: K8s, meta: { title: 'Config & Storage', app: 'containers' } },
      { path: '/containers/k8s/pod-terminal/:clusterId/:namespace/:podName', name: 'K8sPodTerminal', component: K8sPodTerminal, meta: { title: 'Pod Terminal', app: 'containers', keepAlive: true } },
      { path: '/assets/k8s/:pathMatch(.*)*', redirect: (to) => `/containers/k8s/${to.params.pathMatch || ''}` },
      { path: '/assets/server/credentials/password', redirect: '/assets/server/credentials' },
      { path: '/assets/server/credentials/key', redirect: '/assets/server/credentials' },
      { path: '/assets/hosts', redirect: '/assets/server/hosts' },
      { path: '/assets/server/hosts/:id/detail', name: 'AssetHostDetail', component: AssetDetail, props: { resourceType: 'host' }, meta: { title: 'Host Details', app: 'assets' } },
      { path: '/assets/tags', redirect: '/assets/server/groups' },
      { path: '/ops', redirect: '/ops/quick-exec/command' },
      { path: '/ops/jobs', redirect: '/ops/jobs/list' },
      { path: '/ops/environments', redirect: '/assets/environments' },
      { path: '/assets/environments', component: OpsEnvironment, meta: { title: 'Environment Model', app: 'assets' } },
      { path: '/ops/overview', redirect: '/ops/quick-exec/command' },
      { path: '/ops/scripts', redirect: '/ops/scripts/library' },
      { path: '/ops/history', redirect: '/ops/quick-exec/history' },
      { path: '/ops/releases', redirect: '/ops/quick-exec/history' },
      { path: '/ops/quick-exec', redirect: '/ops/quick-exec/command' },
      { path: '/ops/schedule', redirect: '/ops/schedule/tasks' },
      { path: '/ops/applications', redirect: '/applications/projects' },
      { path: '/ops/applications/center', redirect: '/applications/projects' },
      { path: '/ops/scripts/library', component: OpsScriptLibrary, meta: { title: 'Script Library', app: 'ops', summary: 'Manages standard operation scripts, enablement state, and execution parameter templates.' } },
      { path: '/ops/quick-exec/command', component: OpsCommandExecute, meta: { title: 'Command Execution', app: 'ops', summary: 'Runs commands in bulk through SSH.' } },
      { path: '/ops/quick-exec/script', component: OpsScriptExecute, meta: { title: 'Script Execution', app: 'ops', summary: 'Selects scripts from the library and executes them in bulk through SSH.' } },
      { path: '/ops/quick-exec/file-dispatch', component: OpsFileDispatch, meta: { title: 'File Distribution', app: 'ops', summary: 'Distributes files one-to-many from local upload or a source server.' } },
      { path: '/ops/quick-exec/history', component: OpsExecutionHistory, meta: { title: 'Quick Execution History', app: 'ops', summary: 'Records quick execution tasks, results, and failed-host details.' } },
      { path: '/ops/schedule/tasks', component: OpsScheduleTaskList, meta: { title: 'Scheduled Tasks', app: 'ops', summary: 'Maintains script tasks and HTTP probe tasks with bulk enable, disable, and delete.' } },
      { path: '/ops/schedule/logs', component: OpsScheduleLog, meta: { title: 'Task Logs', app: 'ops', summary: 'Shows each scheduled execution result, summary, and detailed output.' } },
      { path: '/ops/schedule/templates', component: OpsScheduleTemplate, meta: { title: 'Task Templates', app: 'ops', summary: 'Stores reusable script-task and HTTP-probe templates.' } },
      { path: '/ops/jobs/designer', component: OpsJobDesigner, meta: { title: 'Job Designer', app: 'ops', summary: 'Visually composes script execution, file distribution, and manual approval steps into jobs.' } },
      { path: '/ops/jobs/list', component: OpsJobList, meta: { title: 'Job List', app: 'ops', summary: 'Manages job definitions with enable, disable, run, and designer navigation.' } },
      { path: '/ops/jobs/approvals', component: OpsJobApprovals, meta: { title: 'Manual Approvals', app: 'ops', summary: 'Processes jobs waiting for manual approval; approve to continue or reject to terminate.' } },
      { path: '/ops/jobs/history', component: OpsJobHistory, meta: { title: 'Job History', app: 'ops', summary: 'Shows job execution flow, step results, and manual approval state.' } },
      { path: '/ops/jobs/templates', component: OpsJobTemplate, meta: { title: 'Job Templates', app: 'ops', summary: 'Stores reusable job templates that can be applied in the designer.' } },
      { path: '/applications', redirect: '/applications/projects' },
      { path: '/applications/center', redirect: '/applications/projects' },
      { path: '/applications/projects', component: AppProjectList, meta: { title: 'Application Management', app: 'applications', summary: 'Maintains application projects, service categories, and Git/SVN repository addresses.' } },
      { path: '/applications/build-tasks', component: AppBuildTaskList, meta: { title: 'Build Tasks', app: 'applications', summary: 'Creates build and release tasks from project repositories and supports immediate builds.' } },
      { path: '/applications/build-history', component: AppBuildHistory, meta: { title: 'Build History', app: 'applications', summary: 'Shows build records, stage results, and complete build logs.' } },
      { path: '/applications/image-registries', component: AppImageRegistry, meta: { title: 'Image Registries', app: 'applications', summary: 'Maintains Docker registry endpoints and credentials used by CI/CD pipelines to build and push images.' } },
      { path: '/applications/pipelines', component: AppPipelineCenter, meta: { title: 'CI/CD Pipelines', app: 'applications', summary: 'Creates pipelines from templates and orchestrates checkout, build, image, deployment, and notification stages.' } },
      { path: '/notify', redirect: '/notify/rules' },
      { path: '/notify/templates', component: NotifyTemplate, meta: { title: 'Message Templates', app: 'notify', summary: 'Maintains message templates for DingTalk, WeCom, Feishu, and custom Webhooks.' } },
      { path: '/notify/channels', component: NotifyChannel, meta: { title: 'Notification Channels', app: 'notify', summary: 'Maintains bot and HTTP Webhook notification channels.' } },
      { path: '/notify/rules', component: NotifyRule, meta: { title: 'Notification Rules', app: 'notify', summary: 'Combines notification events, templates, and channels and binds them to jobs or scheduled tasks.' } },
      { path: '/notify/send-logs', component: NotifySendLog, meta: { title: 'Send Logs', app: 'notify', summary: 'Shows notification requests, responses, and failure reasons.' } },
      { path: '/monitor/overview', component: MonitorOverview, meta: { title: 'Monitoring Overview', app: 'monitor', summary: 'Shows datasources, rules, active alerts, and recent events in one view.' } },
      { path: '/monitor/command-center', component: MonitorCommandCenter, meta: { title: 'Operations Command Center', app: 'monitor', summary: 'Combines assets, alerts, monitoring, and resource hotspots into a full-screen operations cockpit.' } },
      { path: '/monitor/datasources', component: MonitorDatasource, meta: { title: 'Datasource Management', app: 'monitor', summary: 'Connects metrics, logs, and Jaeger trace datasources.' } },
      { path: '/monitor/query', component: MonitorQuery, meta: { title: 'Instant Query', app: 'monitor', summary: 'Runs instant metric queries with PromQL.' } },
      { path: '/monitor/logs', component: MonitorLogQuery, meta: { title: 'Log Explorer', app: 'monitor', summary: 'Searches application and Kubernetes logs through Elasticsearch with field and time-range context.' } },
      { path: '/monitor/traces', component: MonitorTraceQuery, meta: { title: 'Trace Explorer', app: 'monitor', summary: 'Queries distributed traces and Span details through Jaeger.', keepAlive: true } },
      { path: '/monitor/traces/:traceId', component: MonitorTraceDetail, meta: { title: 'Trace Details', app: 'monitor', summary: 'Shows distributed trace details on a timeline.' } },
      { path: '/monitor/alert-templates', component: MonitorAlertTemplate, meta: { title: 'Alert Templates', app: 'monitor', summary: 'Stores reusable alert conditions for quickly creating alert rules.' } },
      { path: '/monitor/alert-rules', component: MonitorAlertRule, meta: { title: 'Alert Rules', app: 'monitor', summary: 'Maintains periodic PromQL-based alert rules.' } },
      { path: '/monitor/alert-events', component: MonitorAlertEvent, meta: { title: 'Alert Events', app: 'monitor', summary: 'Shows, acknowledges, and closes alert events.' } },
      { path: '/monitor/silences', component: MonitorSilenceRule, meta: { title: 'Alert Silences', app: 'monitor', summary: 'Silences alert notifications by rule, severity, label, and time window.' } },
      { path: '/monitor/aggregations', component: MonitorAggregationRule, meta: { title: 'Alert Aggregation', app: 'monitor', summary: 'Groups duplicate alert notifications by label.' } },
      { path: '/monitor/dashboards', component: MonitorDashboard, meta: { title: 'Monitoring Dashboards', app: 'monitor', summary: 'Displays monitoring metrics in a grid dashboard.' } },
      { path: '/monitor/inspections', component: MonitorDashboard, meta: { title: 'Inspection Dashboards', app: 'monitor', summary: 'Inspects monitoring panels in list form and exports reports.' } },
      { path: '/monitor/alerts', redirect: '/monitor/alert-events' },
      { path: '/monitor/metrics', redirect: '/monitor/query' }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to, from, next) => {
  if (to.meta.public) {
    next()
    return
  }
  if (to.path === '/login') {
    if (getToken()) {
      next('/dashboard')
      return
    }
    next()
    return
  }

  if (!getToken()) {
    next('/login')
    return
  }
  next()
})

export default router
