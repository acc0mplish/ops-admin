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
        titleKey: 'k8sManagement',
        path: '/assets/k8s',
        icon: 'Connection',
        children: [
          {
            title: '集群管理',
            titleKey: 'k8sClusters',
            path: '/assets/k8s/clusters',
            icon: 'FolderOpened',
            children: []
          },
          {
            title: '集群概览',
            titleKey: 'k8sOverview',
            path: '/assets/k8s/overview',
            icon: 'DataAnalysis',
            children: []
          },
          {
            title: '节点管理',
            titleKey: 'k8sNodes',
            path: '/assets/k8s/nodes',
            icon: 'Monitor',
            children: []
          },
          {
            title: '命名空间',
            titleKey: 'k8sNamespaces',
            path: '/assets/k8s/namespaces',
            icon: 'Grid',
            children: []
          },
          {
            title: '工作负载',
            titleKey: 'k8sWorkloads',
            path: '/assets/k8s/workloads',
            icon: 'SetUp',
            children: []
          },
          {
            title: 'Pod 管理',
            titleKey: 'k8sPods',
            path: '/assets/k8s/pods',
            icon: 'Box',
            children: []
          },
          {
            title: '服务',
            titleKey: 'k8sServices',
            path: '/assets/k8s/services',
            icon: 'Share',
            children: []
          },
          {
            title: 'Ingress',
            titleKey: 'k8sIngresses',
            path: '/assets/k8s/ingresses',
            icon: 'Connection',
            children: []
          },
          {
            title: '高级网络',
            titleKey: 'k8sAdvancedNetwork',
            path: '/assets/k8s/advanced-network',
            icon: 'Connection',
            children: []
          },
          {
            title: '配置与存储',
            titleKey: 'k8sConfigStorage',
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
