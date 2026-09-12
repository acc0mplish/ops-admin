import { computed, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Connection, Grid, Histogram, Monitor, Promotion, SetUp } from '@element-plus/icons-vue'
import {
  K8S_OPERATIONS,
  queryK8sClusterList,
  queryK8sClusterOverview,
  queryK8sNamespaceDetail,
  queryK8sNamespaceEvents,
  queryK8sNodeDetail,
  queryK8sNodePods
} from '../api/k8s'
import { kt } from '../utils/k8s-extra-i18n'
// `t` — main-catalog keys (k8sStatusWarning·k8sSectionOverviewDesc …) that stayed
// in i18n.js. See K8s.vue r15577d note: dropping this import blanked the console.
import { t } from '../utils/i18n'

// I5-H — extracted from views/assets/K8s.vue (behavior preservation + rewiring
// contract, i5-plan §3.8). Original coordinates per block:
//   41-56 constants·sectionTabs · 58-113 cluster/data state · 96-113 node state
//   105-113 namespace state · 285-303 tab/status computeds · 305-341 status text
//   427-475 menu groups·current section · 1075-1154 load orchestration
//   1156-1201 node·namespace drawers · 1579-1582·1903-1923 namespace create
//   1925-1937 node utils · 1939-1991 node labels
// Free variables are injected: route/router and the §16.2 operation runner come
// in as arguments; cross-domain load hooks are wired by the view after all
// composables exist (see K8s.vue).
export const NAMESPACE_FILTER_KEY = 'ops-admin-k8s-namespace-filter'

const CLUSTER_KEY = 'ops-admin-k8s-current-cluster'

export function useK8sClusterState({ route, router, runOpTasks, hooks }) {
  const sectionTabs = [
    { key: 'overview', labelKey: 'k8sOverview', path: '/containers/k8s/overview', icon: Histogram },
    { key: 'nodes', labelKey: 'k8sNodes', path: '/containers/k8s/nodes', icon: Monitor },
    { key: 'namespaces', labelKey: 'k8sNamespaces', path: '/containers/k8s/namespaces', icon: Grid },
    { key: 'workloads', labelKey: 'k8sWorkloads', path: '/containers/k8s/workloads', icon: SetUp },
    { key: 'pods', labelKey: 'k8sPods', path: '/containers/k8s/pods', icon: Promotion },
    { key: 'services', labelKey: 'k8sServices', path: '/containers/k8s/services', icon: Connection },
    { key: 'ingresses', labelKey: 'k8sIngresses', path: '/containers/k8s/ingresses', icon: Connection },
    { key: 'advanced-network', labelKey: 'k8sAdvancedNetwork', path: '/containers/k8s/advanced-network', icon: Connection },
    { key: 'config-storage', labelKey: 'k8sConfigStorage', path: '/containers/k8s/config-storage', icon: Grid }
  ]

  const loading = ref(false)
  const switching = ref(false)
  const clusterOptions = ref([])
  const cluster = ref(null)
  const overview = ref(null)
  const nodes = ref([])
  const namespaces = ref([])
  const pods = ref([])
  const workloads = ref([])
  const services = ref([])
  const ingresses = ref([])
  const gatewayApiGateways = ref([])
  const httpRoutes = ref([])
  const configMaps = ref([])
  const secrets = ref([])
  const storages = ref([])

  const nodeDrawerVisible = ref(false)
  const nodeDrawerLoading = ref(false)
  const nodeDetail = ref(null)
  const nodePods = ref([])
  const nodeLabelsVisible = ref(false)
  const nodeLabelsSaving = ref(false)
  const nodeLabelTarget = ref(null)
  const nodeLabelItems = ref([])

  const namespaceDrawerVisible = ref(false)
  const namespaceDrawerLoading = ref(false)
  const namespaceDetail = ref(null)
  const namespaceEvents = ref([])
  const namespaceCreateVisible = ref(false)
  const namespaceCreateSaving = ref(false)
  const namespaceCreateForm = reactive({
    name: ''
  })

  const currentTab = computed(() => {
    const found = sectionTabs.find((item) => item.path === route.path)
    return found?.key || 'overview'
  })

  const hasCluster = computed(() => Boolean(cluster.value))

  const statusType = computed(() => {
    switch (cluster.value?.status) {
      case 'running':
        return 'success'
      case 'warning':
        return 'warning'
      case 'offline':
        return 'danger'
      default:
        return 'info'
    }
  })

  const kuboardMenuGroups = computed(() => [
    {
      key: 'cluster',
      label: kt('k8sMenuCluster'),
      items: sectionTabs.filter((item) => ['overview', 'nodes', 'namespaces'].includes(item.key))
    },
    {
      key: 'workload',
      label: kt('k8sMenuWorkloads'),
      items: sectionTabs.filter((item) => ['workloads', 'pods'].includes(item.key))
    },
    {
      key: 'network',
      label: kt('k8sMenuNetwork'),
      items: sectionTabs.filter((item) => ['services', 'ingresses', 'advanced-network'].includes(item.key))
    },
    {
      key: 'config',
      label: kt('k8sMenuConfig'),
      items: sectionTabs.filter((item) => ['config-storage'].includes(item.key))
    }
  ])

  const currentSection = computed(() => {
    const current = sectionTabs.find((item) => item.key === currentTab.value)
    if (!current) {
      return {
        key: 'overview',
        title: kt('k8sOverview'),
        description: kt('k8sSectionOverviewDesc')
      }
    }
    const descMap = {
      overview: 'k8sSectionOverviewDesc',
      nodes: 'k8sSectionNodesDesc',
      namespaces: 'k8sSectionNamespacesDesc',
      workloads: 'k8sSectionWorkloadsDesc',
      pods: 'k8sSectionPodsDesc',
      services: 'k8sSectionServicesDesc',
      ingresses: 'k8sSectionIngressDesc',
      'advanced-network': 'k8sSectionAdvancedNetworkDesc',
      'config-storage': 'k8sSectionConfigDesc'
    }
    return {
      key: current.key,
      title: t(current.labelKey),
      description: t(descMap[current.key] || 'k8sSectionOverviewDesc')
    }
  })

  function clusterStatusText(status) {
    const map = {
      running: 'k8sStatusRunning',
      warning: 'k8sStatusWarning',
      offline: 'k8sStatusOffline'
    }
    return t(map[status] || 'k8sStatusWarning')
  }

  function certificateStatusType(status) {
    switch (status) {
      case 'valid':
        return 'success'
      case 'warning':
        return 'warning'
      case 'expired':
        return 'danger'
      default:
        return 'info'
    }
  }

  function certificateStatusText(status) {
    const map = {
      valid: 'k8sStatusValid',
      warning: 'k8sStatusWarning',
      expired: 'k8sStatusExpired'
    }
    return t(map[status] || 'k8sStatusWarning')
  }

  function certificateRemainText(daysRemaining) {
    if (typeof daysRemaining !== 'number') return '-'
    if (daysRemaining < 0) return kt('k8sExpiredDaysAgo', { days: Math.abs(daysRemaining) })
    if (daysRemaining === 0) return kt('k8sExpiresToday')
    return kt('k8sDaysRemaining', { days: daysRemaining })
  }

  function nodePodPercent(pods) {
    const [used, total] = String(pods || '').split('/').map((item) => Number(item))
    if (!Number.isFinite(used) || !Number.isFinite(total) || total <= 0) return 0
    return Math.min(100, Math.round((used / total) * 100))
  }

  function podStatusTagType(status) {
    const value = String(status || '').toLowerCase()
    if (['running', 'succeeded', 'completed'].includes(value)) return 'success'
    if (['pending', 'terminating', 'containercreating'].includes(value)) return 'warning'
    if (['failed', 'error', 'unknown', 'crashloopbackoff', 'imagepullbackoff'].includes(value)) return 'danger'
    return 'info'
  }

  async function loadClusters(preferId) {
    clusterOptions.value = await queryK8sClusterList()
    if (!clusterOptions.value.length) {
      cluster.value = null
      overview.value = null
      nodes.value = []
      namespaces.value = []
      pods.value = []
      workloads.value = []
        services.value = []
        ingresses.value = []
        gatewayApiGateways.value = []
        httpRoutes.value = []
        configMaps.value = []
      secrets.value = []
      storages.value = []
      hooks?.resetNamespaceFilter?.()
      localStorage.removeItem(CLUSTER_KEY)
      localStorage.removeItem(NAMESPACE_FILTER_KEY)
      return
    }

    const storedId = Number(localStorage.getItem(CLUSTER_KEY))
    const target =
      clusterOptions.value.find((item) => item.id === preferId) ||
      clusterOptions.value.find((item) => item.id === storedId) ||
      clusterOptions.value[0]

    if (target) {
      await loadClusterData(target.id)
    }
  }

  async function loadClusterData(clusterId) {
    loading.value = true
    try {
      hooks?.clearWorkloadImageCache?.()
      hooks?.resetWorkloadSelection?.()
      const data = await queryK8sClusterOverview(clusterId)
      cluster.value = data.cluster
      overview.value = data.overview
      nodes.value = data.nodes || []
      namespaces.value = data.namespaces || []
      pods.value = data.pods || []
      workloads.value = data.workloads || []
      services.value = data.network?.services || []
      ingresses.value = data.network?.ingresses || []
      gatewayApiGateways.value = data.advancedNetwork?.gatewayApiGateways || []
      httpRoutes.value = data.advancedNetwork?.httpRoutes || []
      configMaps.value = data.configStorage?.configMaps || []
      secrets.value = data.configStorage?.secrets || []
      storages.value = data.configStorage?.storage || []
      hooks?.restoreNamespaceFilter?.()
      localStorage.setItem(CLUSTER_KEY, String(clusterId))
      void hooks?.hydrateWorkloadImages?.()
    } finally {
      loading.value = false
    }
  }

  async function refreshCurrentClusterData() {
    if (!cluster.value?.id) return
    await loadClusterData(cluster.value.id)
  }

  async function handleClusterChange(clusterId) {
    switching.value = true
    try {
      await loadClusterData(clusterId)
    } finally {
      switching.value = false
    }
  }

  function handleTabChange(tabKey) {
    const target = sectionTabs.find((item) => item.key === tabKey)
    if (target && target.path !== route.path) {
      router.push(target.path)
    }
  }

  async function openNodeDetail(row) {
    if (!cluster.value?.id) return
    nodeDrawerVisible.value = true
    nodeDrawerLoading.value = true
    nodeDetail.value = null
    nodePods.value = []
    try {
      const [detail, podsData] = await Promise.all([
        queryK8sNodeDetail(cluster.value.id, row.name),
        queryK8sNodePods(cluster.value.id, row.name)
      ])
      nodeDetail.value = detail
      nodePods.value = podsData || []
    } finally {
      nodeDrawerLoading.value = false
    }
  }

  async function openNamespaceDetail(row) {
    if (!cluster.value?.id) return
    namespaceDrawerVisible.value = true
    namespaceDrawerLoading.value = true
    namespaceDetail.value = null
    namespaceEvents.value = []
    try {
      const [detail, events] = await Promise.all([
        queryK8sNamespaceDetail(cluster.value.id, row.name),
        queryK8sNamespaceEvents(cluster.value.id, row.name)
      ])
      namespaceDetail.value = detail
      namespaceEvents.value = events || []
    } finally {
      namespaceDrawerLoading.value = false
    }
  }

  function openNamespaceCreate() {
    namespaceCreateForm.name = ''
    namespaceCreateVisible.value = true
  }

  async function submitNamespaceCreate() {
    if (!cluster.value?.id) return
    const name = String(namespaceCreateForm.name || '').trim().toLowerCase()
    if (!/^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/.test(name) || name.length > 63) {
      ElMessage.warning(kt('k8sNamespaceNameInvalid'))
      return
    }
    namespaceCreateSaving.value = true
    try {
      const ok = await runOpTasks(cluster.value.id, kt('k8sOpProgressTitle', { op: kt('k8sOpResourceCreate') }), [{
        op: K8S_OPERATIONS.resourceCreate,
        connectionScoped: true,
        payload: { yaml: `apiVersion: v1\nkind: Namespace\nmetadata:\n  name: ${name}\n` },
        display: name
      }])
      if (ok) ElMessage.success(kt('k8sNamespaceCreatedSuccess'))
      namespaceCreateVisible.value = false
    } finally {
      namespaceCreateSaving.value = false
    }
  }

  async function openNodeLabels(row) {
    if (!cluster.value?.id || !row?.name) return
    nodeLabelTarget.value = { name: row.name }
    nodeLabelsVisible.value = true
    try {
      const detail = await queryK8sNodeDetail(cluster.value.id, row.name)
      nodeLabelItems.value = Object.entries(detail?.labels || {}).map(([key, value]) => ({ key, value }))
    } catch (error) {
      nodeLabelsVisible.value = false
    }
  }

  function addNodeLabel() {
    nodeLabelItems.value.push({ key: '', value: '' })
  }

  function removeNodeLabel(index) {
    nodeLabelItems.value.splice(index, 1)
  }

  async function saveNodeLabels() {
    if (!cluster.value?.id || !nodeLabelTarget.value?.name) return
    const labels = {}
    for (const item of nodeLabelItems.value) {
      const key = String(item.key || '').trim()
      if (!key) {
        ElMessage.warning(kt('enterLabelKey'))
        return
      }
      if (Object.prototype.hasOwnProperty.call(labels, key)) {
        ElMessage.warning(kt('duplicateLabelKey', { key }))
        return
      }
      labels[key] = String(item.value ?? '').trim()
    }
    nodeLabelsSaving.value = true
    try {
      const ok = await runOpTasks(cluster.value.id, kt('k8sOpProgressTitle', { op: kt('k8sOpNodeLabelsUpdate') }), [{
        op: K8S_OPERATIONS.nodeLabelsUpdate,
        target: { kind: 'orchestration.node', displayName: nodeLabelTarget.value.name },
        payload: { labels },
        display: nodeLabelTarget.value.name
      }])
      if (ok) ElMessage.success(kt('nodeLabelUpdated'))
      nodeLabelsVisible.value = false
      await refreshCurrentClusterData()
      if (nodeDetail.value?.name === nodeLabelTarget.value.name) {
        nodeDetail.value = await queryK8sNodeDetail(cluster.value.id, nodeLabelTarget.value.name)
      }
    } finally {
      nodeLabelsSaving.value = false
    }
  }

  return {
    sectionTabs,
    loading,
    switching,
    clusterOptions,
    cluster,
    overview,
    nodes,
    namespaces,
    pods,
    workloads,
    services,
    ingresses,
    gatewayApiGateways,
    httpRoutes,
    configMaps,
    secrets,
    storages,
    nodeDrawerVisible,
    nodeDrawerLoading,
    nodeDetail,
    nodePods,
    nodeLabelsVisible,
    nodeLabelsSaving,
    nodeLabelTarget,
    nodeLabelItems,
    namespaceDrawerVisible,
    namespaceDrawerLoading,
    namespaceDetail,
    namespaceEvents,
    namespaceCreateVisible,
    namespaceCreateSaving,
    namespaceCreateForm,
    currentTab,
    hasCluster,
    statusType,
    kuboardMenuGroups,
    currentSection,
    clusterStatusText,
    certificateStatusType,
    certificateStatusText,
    certificateRemainText,
    nodePodPercent,
    podStatusTagType,
    loadClusters,
    loadClusterData,
    refreshCurrentClusterData,
    handleClusterChange,
    handleTabChange,
    openNodeDetail,
    openNodeLabels,
    addNodeLabel,
    removeNodeLabel,
    saveNodeLabels,
    openNamespaceDetail,
    openNamespaceCreate,
    submitNamespaceCreate
  }
}
