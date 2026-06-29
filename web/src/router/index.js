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
import BasicConfig from '../views/system/BasicConfig.vue'
import Host from '../views/assets/Host.vue'
import HostGroup from '../views/assets/HostGroup.vue'
import Credential from '../views/assets/Credential.vue'
import CloudAccount from '../views/assets/CloudAccount.vue'
import AssetOverview from '../views/assets/AssetOverview.vue'
import Database from '../views/assets/Database.vue'
import DatabaseWorkbench from '../views/assets/DatabaseWorkbench.vue'
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
      { path: '/system/basic-config', component: BasicConfig, meta: { title: '基础配置', app: 'console' } },
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
      {
        path: '/assets/databases/:id/workbench',
        name: 'DatabaseWorkbench',
        component: DatabaseWorkbench,
        meta: { title: '数据库工作台', app: 'assets' }
      },
      { path: '/assets/server/databases', redirect: '/assets/databases' },
      { path: '/assets/server/databases/:id/workbench', redirect: (to) => `/assets/databases/${to.params.id}/workbench` },
      { path: '/assets/k8s', redirect: '/assets/k8s/clusters' },
      { path: '/assets/k8s/clusters', component: K8sClusterManage, meta: { title: 'Clusters', app: 'assets' } },
      { path: '/assets/k8s/overview', component: K8s, meta: { title: 'Overview', app: 'assets' } },
      { path: '/assets/k8s/nodes', component: K8s, meta: { title: 'Nodes', app: 'assets' } },
      { path: '/assets/k8s/namespaces', component: K8s, meta: { title: 'Namespaces', app: 'assets' } },
      { path: '/assets/k8s/workloads', component: K8s, meta: { title: 'Workloads', app: 'assets' } },
      { path: '/assets/k8s/pods', component: K8s, meta: { title: 'Pods', app: 'assets' } },
      { path: '/assets/k8s/network', redirect: '/assets/k8s/services' },
      { path: '/assets/k8s/services', component: K8s, meta: { title: 'Services', app: 'assets' } },
      { path: '/assets/k8s/ingresses', component: K8s, meta: { title: 'Ingress', app: 'assets' } },
      { path: '/assets/k8s/advanced-network', component: K8s, meta: { title: 'Advanced Network', app: 'assets' } },
      { path: '/assets/k8s/config-storage', component: K8s, meta: { title: 'Config & Storage', app: 'assets' } },
      {
        path: '/assets/k8s/pod-terminal/:clusterId/:namespace/:podName',
        name: 'K8sPodTerminal',
        component: K8sPodTerminal,
        meta: { title: 'Pod Terminal', app: 'assets' }
      },
      { path: '/assets/server/credentials/password', redirect: '/assets/server/credentials' },
      { path: '/assets/server/credentials/key', redirect: '/assets/server/credentials' },
      { path: '/assets/hosts', redirect: '/assets/server/hosts' },
      { path: '/assets/tags', redirect: '/assets/server/groups' },

      { path: '/ops', redirect: '/ops/quick-exec/command' },
      { path: '/ops/jobs', redirect: '/ops/jobs/list' },
      { path: '/ops/overview', redirect: '/ops/quick-exec/command' },
      { path: '/ops/scripts', redirect: '/ops/scripts/library' },
      { path: '/ops/history', redirect: '/ops/quick-exec/history' },
      { path: '/ops/releases', redirect: '/ops/quick-exec/history' },
      { path: '/ops/quick-exec', redirect: '/ops/quick-exec/command' },
      { path: '/ops/schedule', redirect: '/ops/schedule/tasks' },
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

      {
        path: '/monitor/overview',
        component: ModuleWorkspace,
        meta: {
          title: '监控大盘',
          app: 'monitor',
          summary: '统一查看系统健康、可用性和告警趋势。',
          cards: [
            { title: '健康概览', description: '展示主机、服务和任务的总体健康状态。' },
            { title: '趋势分析', description: '按时间维度查看异常波动和抖动。' },
            { title: '值班视角', description: '为后续扩展 NOC 和值班视角打基础。' }
          ]
        }
      },
      {
        path: '/monitor/alerts',
        component: ModuleWorkspace,
        meta: {
          title: '告警中心',
          app: 'monitor',
          summary: '集中接收、筛选和处理告警事件。',
          cards: [
            { title: '告警列表', description: '按等级、状态和来源查看告警。' },
            { title: '告警归并', description: '减少重复告警，突出核心故障。' },
            { title: '处理闭环', description: '沉淀认领、处理和恢复流程。' }
          ]
        }
      },
      {
        path: '/monitor/metrics',
        component: ModuleWorkspace,
        meta: {
          title: '指标看板',
          app: 'monitor',
          summary: '为核心资源和业务指标提供统一看板。',
          cards: [
            { title: '主机指标', description: 'CPU、内存、磁盘和网络等基础监控。' },
            { title: '服务指标', description: '接口耗时、错误率和吞吐等关键指标。' },
            { title: '自定义看板', description: '为不同团队预留自定义看板能力。' }
          ]
        }
      }
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

