export const APP_KEY = 'ops-admin-current-app'

export const appDefinitions = [
  {
    key: 'console',
    name: '控制台',
    labelKey: 'appConsole',
    icon: 'Monitor',
    defaultRoute: '/dashboard',
    menuSource: 'backend'
  },
  {
    key: 'assets',
    name: '资产管理',
    labelKey: 'appAssets',
    icon: 'Box',
    defaultRoute: '/assets/overview',
    menus: [
      {
        title: '资产概览',
        titleKey: 'assetsOverview',
        path: '/assets/overview',
        icon: 'DataBoard',
        children: []
      },
      {
        title: '终端登录',
        path: '/assets/terminal',
        icon: 'Platform',
        children: []
      },
      {
        title: '服务器管理',
        titleKey: 'serverManagement',
        path: '/assets/server',
        icon: 'Cpu',
        children: [
          {
            title: '主机管理',
            titleKey: 'hostManagement',
            path: '/assets/server/hosts',
            icon: 'Monitor',
            children: []
          },
          {
            title: '主机组管理',
            titleKey: 'hostGroupManagement',
            path: '/assets/server/groups',
            icon: 'FolderOpened',
            children: []
          },
          {
            title: '凭据管理',
            titleKey: 'credentialManagement',
            path: '/assets/server/credentials',
            icon: 'Key',
            children: []
          },
          {
            title: '云账号管理',
            titleKey: 'cloudAccountManagement',
            path: '/assets/server/cloud-accounts',
            icon: 'Cloudy',
            children: []
          }
        ]
      },
      {
        title: 'K8s 管理',
        path: '/assets/k8s',
        icon: 'Connection',
        children: [
          {
            title: 'Clusters',
            path: '/assets/k8s/clusters',
            icon: 'FolderOpened',
            children: []
          },
          {
            title: 'Overview',
            path: '/assets/k8s/overview',
            icon: 'DataAnalysis',
            children: []
          },
          {
            title: 'Nodes',
            path: '/assets/k8s/nodes',
            icon: 'Monitor',
            children: []
          },
          {
            title: 'Namespaces',
            path: '/assets/k8s/namespaces',
            icon: 'Grid',
            children: []
          },
          {
            title: 'Workloads',
            path: '/assets/k8s/workloads',
            icon: 'SetUp',
            children: []
          },
          {
            title: 'Pods',
            path: '/assets/k8s/pods',
            icon: 'Box',
            children: []
          },
          {
            title: 'Services',
            path: '/assets/k8s/services',
            icon: 'Share',
            children: []
          },
          {
            title: 'Ingress',
            path: '/assets/k8s/ingresses',
            icon: 'Connection',
            children: []
          },
          {
            title: 'Config & Storage',
            path: '/assets/k8s/config-storage',
            icon: 'Files',
            children: []
          }
        ]
      }
    ]
  },
  {
    key: 'ops',
    name: '标准运维',
    labelKey: 'appOps',
    icon: 'Operation',
    defaultRoute: '/ops/jobs',
    menus: [
      {
        title: '作业中心',
        titleKey: 'opsJobs',
        path: '/ops/jobs',
        icon: 'Timer',
        children: []
      },
      {
        title: '运维总览',
        titleKey: 'opsOverview',
        path: '/ops/overview',
        icon: 'DataAnalysis',
        children: []
      },
      {
        title: '脚本执行',
        titleKey: 'opsScripts',
        path: '/ops/scripts',
        icon: 'EditPen',
        children: []
      },
      {
        title: '发布编排',
        titleKey: 'opsReleases',
        path: '/ops/releases',
        icon: 'Promotion',
        children: []
      }
    ]
  },
  {
    key: 'monitor',
    name: '监控中心',
    labelKey: 'appMonitor',
    icon: 'Histogram',
    defaultRoute: '/monitor/overview',
    menus: [
      {
        title: '监控大盘',
        titleKey: 'monitorOverview',
        path: '/monitor/overview',
        icon: 'TrendCharts',
        children: []
      },
      {
        title: '告警中心',
        titleKey: 'monitorAlerts',
        path: '/monitor/alerts',
        icon: 'Bell',
        children: []
      },
      {
        title: '指标看板',
        titleKey: 'monitorMetrics',
        path: '/monitor/metrics',
        icon: 'PieChart',
        children: []
      }
    ]
  }
]

export function getAppByKey(key) {
  return appDefinitions.find((item) => item.key === key) || appDefinitions[0]
}

export function getAppByRoute(path) {
  if (path.startsWith('/assets')) {
    return getAppByKey('assets')
  }
  if (path.startsWith('/ops')) {
    return getAppByKey('ops')
  }
  if (path.startsWith('/monitor')) {
    return getAppByKey('monitor')
  }
  return getAppByKey('console')
}

export function setCurrentApp(key) {
  localStorage.setItem(APP_KEY, key)
}

export function getCurrentApp() {
  return localStorage.getItem(APP_KEY) || 'console'
}
