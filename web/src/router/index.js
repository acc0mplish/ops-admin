import { createRouter, createWebHistory } from 'vue-router'

import MainLayout from '../layouts/MainLayout.vue'
import Login from '../views/Login.vue'
import Dashboard from '../views/Dashboard.vue'
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
import AppTopology from '../views/applications/AppTopology.vue'
import NotifyTemplate from '../views/notify/NotifyTemplate.vue'
import NotifyChannel from '../views/notify/NotifyChannel.vue'
import NotifyRule from '../views/notify/NotifyRule.vue'
import NotifySendLog from '../views/notify/NotifySendLog.vue'
import MonitorOverview from '../views/monitor/MonitorOverview.vue'
import MonitorDatasource from '../views/monitor/MonitorDatasource.vue'
import MonitorQuery from '../views/monitor/MonitorQuery.vue'
import MonitorLogQuery from '../views/monitor/MonitorLogQuery.vue'
import MonitorAlertRule from '../views/monitor/MonitorAlertRule.vue'
import MonitorAlertEvent from '../views/monitor/MonitorAlertEvent.vue'
import MonitorSilenceRule from '../views/monitor/MonitorSilenceRule.vue'
import MonitorAggregationRule from '../views/monitor/MonitorAggregationRule.vue'
import MonitorDashboard from '../views/monitor/MonitorDashboard.vue'
import ModuleWorkspace from '../views/modules/ModuleWorkspace.vue'
import { getToken } from '../utils/auth'

const moduleCards = {
  assetOverview: [
    { title: '资产总览', description: '统一查看主机、凭据、云账号和分组的整体情况。' },
    { title: '资源分布', description: '按环境、区域和业务维度查看资源分布。' },
    { title: '健康状态', description: '聚合展示异常、离线和待处理资源。' }
  ]
}

const routes = [
  { path: '/login', component: Login, meta: { title: '登录' } },
  {
    path: '/',
    component: MainLayout,
    redirect: '/dashboard',
    children: [
      { path: '/dashboard', component: Dashboard, meta: { title: '仪表盘', app: 'console', affix: true } },
      { path: '/profile', component: Profile, meta: { title: '个人信息', app: 'console' } },
      { path: '/system/admin', component: Admin, meta: { title: '用户信息', app: 'console' } },
      { path: '/system/role', component: Role, meta: { title: '角色信息', app: 'console' } },
      { path: '/system/menu', component: Menu, meta: { title: '菜单信息', app: 'console' } },
      { path: '/system/dept', component: Dept, meta: { title: '部门信息', app: 'console' } },
      { path: '/system/post', component: Post, meta: { title: '岗位信息', app: 'console' } },
      { path: '/system/settings', component: SystemSettings, meta: { title: '系统设置', app: 'console' } },
      { path: '/system/basic-config', redirect: '/system/settings' },
      { path: '/logs/login', component: LoginLog, meta: { title: '登录日志', app: 'console' } },
      { path: '/logs/operation', component: OperationLog, meta: { title: '操作日志', app: 'console' } },

      {
        path: '/assets/overview',
        component: AssetOverview,
        meta: {
          title: '资产概览',
          app: 'assets',
          summary: '统一查看主机资产、凭据、云账号、数据库和主机组的整体状态。'
        }
      },
      {
        path: '/assets/terminal',
        component: TerminalLogin,
        meta: {
          title: '终端登录',
          app: 'assets',
          summary: '通过主机关联凭据发起 SSH Web 终端连接。'
        }
      },
      {
        path: '/assets/server/hosts',
        component: Host,
        meta: {
          title: '主机管理',
          app: 'assets',
          summary: '集中维护服务器、云主机和网络设备台账。'
        }
      },
      {
        path: '/assets/server/groups',
        component: HostGroup,
        meta: {
          title: '主机组管理',
          app: 'assets',
          summary: '通过主机组组织资产范围，为批量运维和权限控制提供边界。'
        }
      },
      {
        path: '/assets/server/credentials',
        component: Credential,
        meta: {
          title: '凭据管理',
          app: 'assets',
          summary: '集中维护密码认证和密钥认证两类主机登录凭据。'
        }
      },
      {
        path: '/assets/server/cloud-accounts',
        component: CloudAccount,
        meta: {
          title: '云账号管理',
          app: 'assets',
          summary: '维护云厂商访问账号，为云资产同步提供凭证来源。'
        }
      },
      {
        path: '/assets/databases',
        component: Database,
        meta: {
          title: '数据库管理',
          app: 'assets',
          summary: '集中维护 MySQL 数据库资产，并进入 SQL 工作台进行表结构、数据与执行记录管理。'
        }
      },
      { path: '/assets/databases/:id/detail', name: 'AssetDatabaseDetail', component: AssetDetail, props: { resourceType: 'database' }, meta: { title: '数据库详情', app: 'assets' } },
      { path: '/assets/databases/workbench', component: DatabaseWorkbenchEntry, meta: { title: 'DBMS 工作台', app: 'assets' } },
      { path: '/assets/databases/import', component: DatabaseImport, meta: { title: '数据导入', app: 'assets' } },
      { path: '/assets/databases/backups', component: DatabaseBackup, meta: { title: '备份管理', app: 'assets' } },
      {
        path: '/assets/databases/:id/workbench',
        name: 'DatabaseWorkbench',
        component: DatabaseWorkbench,
        meta: { title: 'DBMS 工作台', app: 'assets' }
      },
      { path: '/assets/server/databases', redirect: '/assets/databases' },
      { path: '/assets/server/databases/:id/workbench', redirect: (to) => `/assets/databases/${to.params.id}/workbench` },
      {
        path: '/assets/gateways',
        component: Gateway,
        meta: {
          title: '网关管理',
          app: 'assets',
          summary: '维护 SSH 跳板网关，为内网主机、数据库和 K8s 集群提供访问入口。'
        }
      },
      { path: '/assets/k8s', redirect: '/assets/k8s/clusters' },
      { path: '/assets/k8s/clusters', component: K8sClusterManage, meta: { title: '集群管理', app: 'assets' } },
      { path: '/assets/k8s/clusters/:id/detail', name: 'K8sClusterDetail', component: AssetDetail, props: { resourceType: 'k8s' }, meta: { title: '集群详情', app: 'assets' } },
      { path: '/assets/k8s/overview', component: K8s, meta: { title: '集群概览', app: 'assets' } },
      { path: '/assets/k8s/nodes', component: K8s, meta: { title: '节点管理', app: 'assets' } },
      { path: '/assets/k8s/namespaces', component: K8s, meta: { title: '命名空间', app: 'assets' } },
      { path: '/assets/k8s/workloads', component: K8s, meta: { title: '工作负载', app: 'assets' } },
      { path: '/assets/k8s/pods', component: K8s, meta: { title: 'Pod 管理', app: 'assets' } },
      { path: '/assets/k8s/network', redirect: '/assets/k8s/services' },
      { path: '/assets/k8s/services', component: K8s, meta: { title: '服务', app: 'assets' } },
      { path: '/assets/k8s/ingresses', component: K8s, meta: { title: 'Ingress', app: 'assets' } },
      { path: '/assets/k8s/advanced-network', component: K8s, meta: { title: '高级网络', app: 'assets' } },
      { path: '/assets/k8s/config-storage', component: K8s, meta: { title: '配置与存储', app: 'assets' } },
      {
        path: '/assets/k8s/pod-terminal/:clusterId/:namespace/:podName',
        name: 'K8sPodTerminal',
        component: K8sPodTerminal,
        meta: { title: 'Pod 终端', app: 'assets' }
      },
      { path: '/assets/server/credentials/password', redirect: '/assets/server/credentials' },
      { path: '/assets/server/credentials/key', redirect: '/assets/server/credentials' },
      { path: '/assets/hosts', redirect: '/assets/server/hosts' },
      { path: '/assets/server/hosts/:id/detail', name: 'AssetHostDetail', component: AssetDetail, props: { resourceType: 'host' }, meta: { title: '主机详情', app: 'assets' } },
      { path: '/assets/tags', redirect: '/assets/server/groups' },

      { path: '/ops', redirect: '/ops/quick-exec/command' },
      { path: '/ops/jobs', redirect: '/ops/jobs/list' },
      { path: '/ops/environments', component: OpsEnvironment, meta: { title: '环境模型', app: 'ops' } },
      { path: '/ops/overview', redirect: '/ops/quick-exec/command' },
      { path: '/ops/scripts', redirect: '/ops/scripts/library' },
      { path: '/ops/history', redirect: '/ops/quick-exec/history' },
      { path: '/ops/releases', redirect: '/ops/quick-exec/history' },
      { path: '/ops/quick-exec', redirect: '/ops/quick-exec/command' },
      { path: '/ops/schedule', redirect: '/ops/schedule/tasks' },
      { path: '/ops/applications', redirect: '/applications/projects' },
      { path: '/ops/applications/center', redirect: '/applications/projects' },
      {
        path: '/ops/scripts/library',
        component: OpsScriptLibrary,
        meta: {
          title: '脚本库',
          app: 'ops',
          summary: '统一管理标准运维脚本，支持启用、禁用和执行参数模板。'
        }
      },
      {
        path: '/ops/quick-exec/command',
        component: OpsCommandExecute,
        meta: {
          title: '命令执行',
          app: 'ops',
          summary: '通过 SSH 发起命令批量执行。'
        }
      },
      {
        path: '/ops/quick-exec/script',
        component: OpsScriptExecute,
        meta: {
          title: '脚本执行',
          app: 'ops',
          summary: '从脚本库中选择脚本并通过 SSH 批量执行。'
        }
      },
      {
        path: '/ops/quick-exec/file-dispatch',
        component: OpsFileDispatch,
        meta: {
          title: '文件分发',
          app: 'ops',
          summary: '支持本地上传或源服务器读取的一对多文件分发。'
        }
      },
      {
        path: '/ops/quick-exec/history',
        component: OpsExecutionHistory,
        meta: {
          title: '快速执行历史',
          app: 'ops',
          summary: '记录所有快速执行任务、结果和失败主机明细。'
        }
      },
      {
        path: '/applications/topology',
        component: AppTopology,
        meta: {
          title: '应用拓扑',
          app: 'applications',
          summary: '以应用为中心串联主机、K8s、数据库、监控告警和发布记录。'
        }
      },
      {
        path: '/ops/schedule/tasks',
        component: OpsScheduleTaskList,
        meta: {
          title: '任务列表',
          app: 'ops',
          summary: '统一维护脚本任务和 HTTP 探针任务，支持批量启用、禁用和删除。'
        }
      },
      {
        path: '/ops/schedule/logs',
        component: OpsScheduleLog,
        meta: {
          title: '任务日志',
          app: 'ops',
          summary: '查看定时任务每次调度的执行结果、摘要和详细输出。'
        }
      },
      {
        path: '/ops/schedule/templates',
        component: OpsScheduleTemplate,
        meta: {
          title: '任务模板',
          app: 'ops',
          summary: '沉淀可复用的脚本任务模板和 HTTP 探针模板。'
        }
      },
      {
        path: '/ops/jobs/designer',
        component: OpsJobDesigner,
        meta: {
          title: '作业编排',
          app: 'ops',
          summary: '通过可视化编排将脚本执行、文件分发和人工确认步骤组装成作业。'
        }
      },
      {
        path: '/ops/jobs/list',
        component: OpsJobList,
        meta: {
          title: '作业列表',
          app: 'ops',
          summary: '管理作业定义，支持启用、禁用、运行与跳转编排。'
        }
      },
      {
        path: '/ops/jobs/approvals',
        component: OpsJobApprovals,
        meta: {
          title: '人工确认',
          app: 'ops',
          summary: '集中处理卡在人工确认步骤的作业待办，确认通过后继续执行，拒绝后终止作业。'
        }
      },
      {
        path: '/ops/jobs/history',
        component: OpsJobHistory,
        meta: {
          title: '作业历史',
          app: 'ops',
          summary: '查看作业执行过程、步骤结果和人工确认状态。'
        }
      },
      {
        path: '/ops/jobs/templates',
        component: OpsJobTemplate,
        meta: {
          title: '作业模板',
          app: 'ops',
          summary: '沉淀可导入的作业模板，并在编排时一键套用。'
        }
      },
      { path: '/applications', redirect: '/applications/projects' },
      { path: '/applications/center', redirect: '/applications/projects' },
      {
        path: '/applications/projects',
        component: AppProjectList,
        meta: { title: '项目列表', app: 'applications', summary: '维护应用项目、服务类别和 Git/SVN 仓库地址。' }
      },
      {
        path: '/applications/build-tasks',
        component: AppBuildTaskList,
        meta: { title: '构建任务', app: 'applications', summary: '基于项目仓库创建构建发布任务，并支持立即构建。' }
      },
      {
        path: '/applications/build-history',
        component: AppBuildHistory,
        meta: { title: '构建历史', app: 'applications', summary: '查看构建记录、阶段结果和完整构建日志。' }
      },
      {
        path: '/applications/pipelines',
        component: AppPipelineCenter,
        meta: { title: 'CI/CD 流水线', app: 'applications', summary: '从模板创建流水线，编排代码拉取、构建、镜像、发布和通知阶段。' }
      },

      { path: '/notify', redirect: '/notify/rules' },
      {
        path: '/notify/templates',
        component: NotifyTemplate,
        meta: { title: '消息模板', app: 'notify', summary: '维护钉钉、企微、飞书和自定义 Webhook 的消息模板。' }
      },
      {
        path: '/notify/channels',
        component: NotifyChannel,
        meta: { title: '通知媒介', app: 'notify', summary: '维护机器人和 HTTP Webhook 通知媒介。' }
      },
      {
        path: '/notify/rules',
        component: NotifyRule,
        meta: { title: '通知规则', app: 'notify', summary: '组合通知事件、模板与媒介，并绑定到作业编排或定时任务。' }
      },
      {
        path: '/notify/send-logs',
        component: NotifySendLog,
        meta: { title: '发送日志', app: 'notify', summary: '查看通知发送请求、响应与失败原因。' }
      },

      {
        path: '/monitor/overview',
        component: MonitorOverview,
        meta: {
          title: '监控概览',
          app: 'monitor',
          summary: '统一查看数据源、规则、当前告警和最近事件。'
        }
      },
      {
        path: '/monitor/datasources',
        component: MonitorDatasource,
        meta: {
          title: '数据源管理',
          app: 'monitor',
          summary: '接入 Prometheus 或 VictoriaMetrics 数据源。'
        }
      },
      {
        path: '/monitor/query',
        component: MonitorQuery,
        meta: {
          title: '即时查询',
          app: 'monitor',
          summary: '输入 PromQL 即时查询指标结果。'
        }
      },
      {
        path: '/monitor/logs',
        component: MonitorLogQuery,
        meta: {
          title: '日志查询',
          app: 'monitor',
          summary: '通过 Elasticsearch 检索应用与 Kubernetes 日志，并按字段、时间范围查看上下文。'
        }
      },
      {
        path: '/monitor/alert-rules',
        component: MonitorAlertRule,
        meta: { title: '告警规则', app: 'monitor', summary: '维护基于 PromQL 的周期告警规则。' }
      },
      {
        path: '/monitor/alert-events',
        component: MonitorAlertEvent,
        meta: { title: '告警事件', app: 'monitor', summary: '查看、认领和关闭告警事件。' }
      },
      {
        path: '/monitor/silences',
        component: MonitorSilenceRule,
        meta: { title: '告警屏蔽', app: 'monitor', summary: '按规则、等级、标签和时间窗口静默告警通知。' }
      },
      {
        path: '/monitor/aggregations',
        component: MonitorAggregationRule,
        meta: { title: '聚合收敛', app: 'monitor', summary: '按 Label 分组收敛重复告警通知。' }
      },
      {
        path: '/monitor/dashboards',
        component: MonitorDashboard,
        meta: { title: '监控大屏', app: 'monitor', summary: '以网格方式展示监控指标大屏。' }
      },
      {
        path: '/monitor/inspections',
        component: MonitorDashboard,
        meta: { title: '巡检大屏', app: 'monitor', summary: '以列表方式巡检监控面板并导出报告。' }
      },
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

