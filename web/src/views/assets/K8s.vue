<script setup>
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRoute, useRouter } from 'vue-router'
import { Connection, Grid, Histogram, Monitor, Promotion, SetUp } from '@element-plus/icons-vue'
import K8sConsoleLayout from './k8s/K8sConsoleLayout.vue'
import K8sSectionContent from './k8s/K8sSectionContent.vue'
import K8sDrawers from './k8s/K8sDrawers.vue'
import K8sDialogs from './k8s/K8sDialogs.vue'
import './k8s/k8s-page.css'
import {
  queryK8sClusterList,
  queryK8sClusterOverview,
  queryK8sNamespaceDetail,
  queryK8sNamespaceEvents,
  queryK8sNodeDetail,
  queryK8sNodePods,
  updateK8sNodeLabels,
  queryK8sPodDetail,
  queryK8sPodContainers,
  queryK8sPodEvents,
  queryK8sPodLogs,
  queryK8sWorkloadDetail,
  queryK8sServiceDetail,
  queryK8sIngressDetail,
  queryK8sIstioResourceDetail,
  queryK8sConfigMapDetail,
  queryK8sSecretDetail,
  queryK8sStorageDetail,
  scaleK8sWorkload,
  restartK8sWorkload,
  updateK8sWorkloadImages,
	updateK8sWorkloadResources,
  updateK8sIstioTraffic,
  updateK8sHTTPRouteTraffic,
  createK8sResourceYAML,
  deleteK8sResource,
  updateK8sResourceYAML
} from '../../api/k8s'
import { t } from '../../utils/i18n'

const route = useRoute()
const router = useRouter()
const CLUSTER_KEY = 'ops-admin-k8s-current-cluster'
const NAMESPACE_FILTER_KEY = 'ops-admin-k8s-namespace-filter'

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
const namespaceFilter = ref('__all__')
const resourceKeyword = ref('')
const namespaceKeyword = ref('')
const workloadTypeFilter = ref('all')
const podScopedNames = ref([])
const selectedWorkloads = ref([])
const workloadImageMap = reactive({})
const workloadImageLoadingMap = reactive({})

const configMapDrawerVisible = ref(false)
const configMapDrawerLoading = ref(false)
const configMapDetail = ref(null)

const secretDrawerVisible = ref(false)
const secretDrawerLoading = ref(false)
const secretDetail = ref(null)

const storageDrawerVisible = ref(false)
const storageDrawerLoading = ref(false)
const storageDetail = ref(null)

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

const workloadDrawerVisible = ref(false)
const workloadDrawerLoading = ref(false)
const workloadDetail = ref(null)
const workloadResourceDialogVisible = ref(false)
const workloadResourceSaving = ref(false)
const workloadResourceForm = reactive({ namespace: '', workloadType: '', workloadName: '', containers: [] })

const podDrawerVisible = ref(false)
const podDrawerLoading = ref(false)
const podDetail = ref(null)
const podEvents = ref([])
const podLogDrawerVisible = ref(false)
const podLogLoading = ref(false)
const podLogs = ref('')
const selectedContainer = ref('')
const podLogTailLines = ref(200)
const podPage = ref(1)
const podPageSize = ref(20)
const configStorageTab = ref('configmaps')
const configStorageCreateVisible = ref(false)
const configStorageCreateSaving = ref(false)
const configStorageEditing = ref(false)
const storageClassCreateVisible = ref(false)
const storageClassCreateSaving = ref(false)
const storageClassCreateForm = reactive({
  name: '',
  sourceType: 'hostpath',
  capacity: '10Gi',
  reclaimPolicy: 'Delete',
  accessMode: 'ReadWriteOnce',
  path: '',
  nfsServer: '',
  scopeNamespaceEnabled: false,
  scopeNamespace: ''
})

watch(() => storageClassCreateForm.sourceType, (sourceType) => {
  if (sourceType === 'hostpath') storageClassCreateForm.accessMode = 'ReadWriteOnce'
})
const configStorageCreateForm = reactive({
  kind: 'configmap',
  namespace: '',
  name: '',
  entries: [{ key: '', value: '' }],
  secretType: 'Opaque',
  capacity: '1Gi',
  storageClass: '',
  accessMode: 'ReadWriteOnce'
})
const currentPodQuery = reactive({
  namespace: '',
  podName: ''
})

const serviceDrawerVisible = ref(false)
const serviceDrawerLoading = ref(false)
const serviceDetail = ref(null)

const ingressDrawerVisible = ref(false)
const ingressDrawerLoading = ref(false)
const ingressDetail = ref(null)

const istioDrawerVisible = ref(false)
const istioDrawerLoading = ref(false)
const istioDetail = ref(null)

const scaleDialogVisible = ref(false)
const scaleLoading = ref(false)
const scaleForm = reactive({
  namespace: '',
  workloadType: '',
  workloadName: '',
  replicas: 1
})

const imageVersionDialogVisible = ref(false)
const imageVersionSaving = ref(false)
const imageVersionForm = reactive({
  version: ''
})

const istioCreateDialogVisible = ref(false)
const istioCreateSaving = ref(false)
const istioCreateForm = reactive({
  resourceType: 'gateway',
  yaml: ''
})

const trafficDialogVisible = ref(false)
const trafficSaving = ref(false)
const trafficForm = reactive({
  resourceType: 'virtualservice',
  namespace: '',
  name: '',
  routes: []
})

const yamlDialogVisible = ref(false)
const yamlSaving = ref(false)
const yamlTextareaRef = ref()
const yamlEditor = reactive({
  title: '',
  resourceType: '',
  namespace: '',
  name: '',
  workloadType: '',
  originalYAML: '',
  yaml: ''
})
const yamlSearch = reactive({
  keyword: '',
  matches: [],
  activeIndex: -1
})
const yamlEditorScrollTop = ref(0)
const yamlCurrentLine = ref(1)
const yamlLineHeight = 20

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
  if (daysRemaining < 0) return t('k8sExpiredDaysAgo', { days: Math.abs(daysRemaining) })
  if (daysRemaining === 0) return t('k8sExpiresToday')
  return t('k8sDaysRemaining', { days: daysRemaining })
}

const podContainerOptions = computed(() => {
  if (!podDetail.value?.containers) return []
  return podDetail.value.containers.map((item) => item.name)
})
const podLogLines = computed(() => String(podLogs.value || '').split('\n').filter((line, index, list) => line || index < list.length - 1))

const namespaceOptions = computed(() => {
  const options = [{ label: t('k8sAllNamespaces'), value: '__all__' }]
  for (const item of namespaces.value || []) {
    options.push({ label: item.name, value: item.name })
  }
  return options
})

const filteredPods = computed(() => filterList(pods.value))
const pagedPods = computed(() => {
  const start = (podPage.value - 1) * podPageSize.value
  return filteredPods.value.slice(start, start + podPageSize.value)
})
const filteredWorkloads = computed(() => filterList(workloads.value))
const filteredServices = computed(() => filterList(services.value))
const filteredIngresses = computed(() => filterList(ingresses.value))
const filteredGatewayApiGateways = computed(() => filterList(gatewayApiGateways.value))
const filteredHTTPRoutes = computed(() => filterList(httpRoutes.value))
const filteredConfigMaps = computed(() => filterList(configMaps.value))
const filteredSecrets = computed(() => filterList(secrets.value))
const filteredStorages = computed(() => filterList(storages.value))
const filteredStorageClasses = computed(() => filteredStorages.value.filter((item) => item.kind === 'PV'))
const filteredStorageVolumes = computed(() => filteredStorages.value.filter((item) => item.kind === 'PVC'))
const yamlDiffLines = computed(() => buildYAMLDiffLines(yamlEditor.originalYAML, yamlEditor.yaml))
const yamlLineNumbers = computed(() => {
  const total = Math.max(1, yamlEditor.yaml.split('\n').length)
  return Array.from({ length: total }, (_, index) => index + 1)
})
const yamlPreviewLineNumbers = computed(() => {
  const total = Math.max(1, yamlDiffLines.value.length)
  return Array.from({ length: total }, (_, index) => index + 1)
})
const yamlChangeSummary = computed(() => {
  let added = 0
  let removed = 0
  for (const item of yamlDiffLines.value) {
    if (item.type === 'added') added++
    if (item.type === 'removed') removed++
  }
  return {
    added,
    removed,
    changed: added + removed
  }
})
const yamlCurrentLineOffset = computed(() => `${Math.max(0, (yamlCurrentLine.value - 1) * yamlLineHeight - yamlEditorScrollTop.value)}px`)
const trafficTotalWeight = computed(() =>
  (trafficForm.routes || []).reduce((total, item) => total + Number(item.weight || 0), 0)
)

const kuboardMenuGroups = computed(() => [
  {
    key: 'cluster',
    label: t('k8sMenuCluster'),
    items: sectionTabs.filter((item) => ['overview', 'nodes', 'namespaces'].includes(item.key))
  },
  {
    key: 'workload',
    label: t('k8sMenuWorkloads'),
    items: sectionTabs.filter((item) => ['workloads', 'pods'].includes(item.key))
  },
  {
    key: 'network',
    label: t('k8sMenuNetwork'),
    items: sectionTabs.filter((item) => ['services', 'ingresses', 'advanced-network'].includes(item.key))
  },
  {
    key: 'config',
    label: t('k8sMenuConfig'),
    items: sectionTabs.filter((item) => ['config-storage'].includes(item.key))
  }
])

const currentSection = computed(() => {
  const current = sectionTabs.find((item) => item.key === currentTab.value)
  if (!current) {
    return {
      key: 'overview',
      title: t('k8sOverview'),
      description: t('k8sSectionOverviewDesc')
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

const kuboardNamespaceRows = computed(() =>
  (namespaces.value || []).map((item) => ({
    ...item,
    podsCount: Number(item.podsCount ?? item.pods ?? 0),
    servicesCount: Number(item.servicesCount ?? item.services ?? 0),
    workloadsCount: Number(item.workloadsCount ?? item.workloads ?? 0),
    phase: item.phase || item.status || '-',
    age: item.age || formatAgeFromTimestamp(item.createdAt)
  }))
)

const filteredKuboardNamespaceRows = computed(() => {
  const keyword = namespaceKeyword.value.trim().toLowerCase()
  if (!keyword) return kuboardNamespaceRows.value
  return kuboardNamespaceRows.value.filter((item) =>
    [item.name, item.phase, item.age].some((value) => String(value || '').toLowerCase().includes(keyword))
  )
})

const namespaceSummary = computed(() => {
  const rows = kuboardNamespaceRows.value
  return {
    total: rows.length,
    active: rows.filter((item) => ['active', 'running'].includes(String(item.phase || '').toLowerCase())).length,
    pods: rows.reduce((total, item) => total + Number(item.podsCount || 0), 0),
    services: rows.reduce((total, item) => total + Number(item.servicesCount || 0), 0),
    workloads: rows.reduce((total, item) => total + Number(item.workloadsCount || 0), 0)
  }
})

const workloadTypeOptions = computed(() => {
  const counts = new Map()
  for (const item of filteredWorkloads.value) {
    counts.set(item.type, (counts.get(item.type) || 0) + 1)
  }
  const order = ['Deployment', 'StatefulSet', 'DaemonSet', 'CronJob', 'Job']
  const dynamic = order
    .filter((type) => counts.has(type))
    .map((type) => ({ value: type, label: type, count: counts.get(type) || 0 }))
  return [{ value: 'all', label: t('k8sWorkloadTypeAll'), count: filteredWorkloads.value.length }, ...dynamic]
})

const kuboardWorkloadRows = computed(() => {
  const activeType = workloadTypeFilter.value
  return filteredWorkloads.value
    .filter((item) => activeType === 'all' || item.type === activeType)
    .map((item) => {
      const key = buildWorkloadCacheKey(item)
      return {
        ...item,
        status: item.status || item.phase || deriveWorkloadPhase(item),
        age: item.age || formatAgeFromTimestamp(item.createdAt || item.createTime),
        images: item.images || workloadImageMap[key] || extractImagesFromContainers(item.containers)
      }
    })
})

const workloadSummary = computed(() => {
  const rows = kuboardWorkloadRows.value
  return {
    total: rows.length,
    healthy: rows.filter((item) => isWorkloadHealthy(item)).length,
    namespaces: new Set(rows.map((item) => item.namespace).filter(Boolean)).size,
    restartable: rows.filter((item) => supportsRestart(item)).length
  }
})

const workloadSelectionCount = computed(() => selectedWorkloads.value.length)

function hasItems(list) {
  return Array.isArray(list) && list.length > 0
}

function shouldShowNamespaceFilter(tab) {
  return ['pods', 'workloads', 'services', 'ingresses', 'advanced-network', 'config-storage'].includes(tab)
}

function filterList(list) {
  if (!Array.isArray(list)) return []
  let result = list
  if (namespaceFilter.value !== '__all__') {
    result = result.filter((item) => item.namespace === namespaceFilter.value)
  }
  if (list === pods.value && podScopedNames.value.length) {
    result = result.filter((item) => podScopedNames.value.includes(item.name))
  }
  const keyword = resourceKeyword.value.trim().toLowerCase()
  if (!keyword) return result
  return result.filter((item) => {
    const values = [
      item.name,
      item.namespace,
      item.type,
      item.host,
      item.address,
      item.clusterIP,
      item.externalIP,
      item.hosts,
      item.gateways,
      item.target,
      item.ports,
      item.storageClass,
      item.kind,
      item.status
    ]
    return values.some((value) => String(value || '').toLowerCase().includes(keyword))
  })
}

function restoreNamespaceFilter() {
  const storedValue = localStorage.getItem(NAMESPACE_FILTER_KEY) || '__all__'
  const exists = storedValue === '__all__' || namespaces.value.some((item) => item.name === storedValue)
  namespaceFilter.value = exists ? storedValue : '__all__'
}

function handleNamespaceFilterChange(value) {
  namespaceFilter.value = value || '__all__'
  podScopedNames.value = []
  podPage.value = 1
  localStorage.setItem(NAMESPACE_FILTER_KEY, namespaceFilter.value)
}

function handleResourceKeywordChange(value) {
  resourceKeyword.value = value || ''
  podScopedNames.value = []
  podPage.value = 1
}

function handleNamespaceKeywordChange(value) {
  namespaceKeyword.value = value || ''
}

function openNamespaceWorkloads(row) {
  if (!row?.name) return
  namespaceFilter.value = row.name
  resourceKeyword.value = ''
  workloadTypeFilter.value = 'all'
  podScopedNames.value = []
  localStorage.setItem(NAMESPACE_FILTER_KEY, namespaceFilter.value)
  handleTabChange('workloads')
}

async function openWorkloadPods(row) {
  if (!cluster.value?.id || !row?.name) return
  namespaceFilter.value = row.namespace || '__all__'
  localStorage.setItem(NAMESPACE_FILTER_KEY, namespaceFilter.value)
  const detail = await queryK8sWorkloadDetail(cluster.value.id, row.namespace, row.type, row.name)
  const relatedPods = Array.isArray(detail?.pods) ? detail.pods.map((item) => item.name).filter(Boolean) : []
  podScopedNames.value = relatedPods
  podPage.value = 1
  resourceKeyword.value = relatedPods.length ? '' : row.name
  handleTabChange('pods')
}

function handleWorkloadTypeChange(value) {
  workloadTypeFilter.value = value || 'all'
  selectedWorkloads.value = []
  void hydrateWorkloadImages()
}

function handleWorkloadSelectionChange(rows) {
  selectedWorkloads.value = Array.isArray(rows) ? rows : []
}

function openImageVersionDialog() {
  if (!selectedWorkloads.value.length) {
    ElMessage.warning(t('k8sSelectWorkloadsFirst'))
    return
  }
  imageVersionForm.version = ''
  imageVersionDialogVisible.value = true
}

async function submitWorkloadImageVersionUpdate() {
  if (!cluster.value?.id) return
  const version = String(imageVersionForm.version || '').trim()
  if (!version) {
    ElMessage.warning(t('k8sImageVersionRequired'))
    return
  }
  await ElMessageBox.confirm(
    t('k8sConfirmBatchImageUpdateMessage', { count: selectedWorkloads.value.length, version }),
    t('k8sConfirmBatchImageUpdateTitle'),
    {
      type: 'warning',
      confirmButtonText: t('confirmChange'),
      cancelButtonText: t('cancel')
    }
  )

  imageVersionSaving.value = true
  try {
    await updateK8sWorkloadImages({
      clusterId: cluster.value.id,
      version,
      items: selectedWorkloads.value.map((item) => ({
        namespace: item.namespace,
        workloadType: item.type,
        workloadName: item.name
      }))
    })
    ElMessage.success(t('k8sBatchImageUpdatedSuccess'))
    imageVersionDialogVisible.value = false
    selectedWorkloads.value = []
    await refreshCurrentClusterData()
    if (workloadDrawerVisible.value && workloadDetail.value?.name) {
      workloadDetail.value = await queryK8sWorkloadDetail(
        cluster.value.id,
        workloadDetail.value.namespace,
        workloadDetail.value.type,
        workloadDetail.value.name
      )
    }
  } finally {
    imageVersionSaving.value = false
  }
}

function resolveIstioNamespace() {
  return namespaceFilter.value !== '__all__' ? namespaceFilter.value : 'default'
}

function buildIstioTemplate(resourceType) {
  const namespace = resolveIstioNamespace()
  switch (resourceType) {
    case 'gatewayapi':
      return `apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: example-gateway
  namespace: ${namespace}
spec:
  gatewayClassName: istio
  listeners:
    - name: http
      protocol: HTTP
      port: 80
      hostname: example.local
`
    case 'httproute':
      return `apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: example-httproute
  namespace: ${namespace}
spec:
  parentRefs:
    - name: example-gateway
  hostnames:
    - example.local
  rules:
    - backendRefs:
        - name: example-service
          port: 80
          weight: 100
`
    case 'gateway':
      return `apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  name: example-gateway
  namespace: ${namespace}
spec:
  selector:
    istio: ingressgateway
  servers:
    - port:
        number: 80
        name: http
        protocol: HTTP
      hosts:
        - "*"
`
    case 'virtualservice':
      return `apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: example-virtualservice
  namespace: ${namespace}
spec:
  hosts:
    - example.local
  gateways:
    - example-gateway
  http:
    - route:
        - destination:
            host: example-service
            subset: v1
            port:
              number: 80
          weight: 100
`
    case 'destinationrule':
      return `apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: example-destinationrule
  namespace: ${namespace}
spec:
  host: example-service
  subsets:
    - name: v1
      labels:
        version: v1
    - name: v2
      labels:
        version: v2
`
    case 'serviceentry':
      return `apiVersion: networking.istio.io/v1beta1
kind: ServiceEntry
metadata:
  name: example-serviceentry
  namespace: ${namespace}
spec:
  hosts:
    - api.external.local
  location: MESH_EXTERNAL
  resolution: DNS
  ports:
    - number: 443
      name: https
      protocol: HTTPS
`
    default:
      return ''
  }
}

function setYAMLEditor(payload) {
  yamlEditor.title = payload.title || t('k8sYamlEditor')
  yamlEditor.resourceType = payload.resourceType || ''
  yamlEditor.namespace = payload.namespace || ''
  yamlEditor.name = payload.name || ''
  yamlEditor.workloadType = payload.workloadType || ''
  yamlEditor.originalYAML = payload.yaml || ''
  yamlEditor.yaml = payload.yaml || ''
  yamlSearch.keyword = ''
  yamlSearch.matches = []
  yamlSearch.activeIndex = -1
  yamlEditorScrollTop.value = 0
  yamlCurrentLine.value = 1
  yamlDialogVisible.value = true
  nextTick(() => {
    const textarea = getYAMLTextareaElement()
    textarea?.focus()
    textarea?.setSelectionRange(0, 0)
  })
}

function getYAMLTextareaElement() {
  return yamlTextareaRef.value || null
}

function handleYAMLScroll(event) {
  yamlEditorScrollTop.value = event.target.scrollTop || 0
}

function updateYAMLCurrentLine() {
  const textarea = getYAMLTextareaElement()
  if (!textarea) return
  const content = yamlEditor.yaml || ''
  const caret = textarea.selectionStart || 0
  yamlCurrentLine.value = content.slice(0, caret).split('\n').length
  yamlEditorScrollTop.value = textarea.scrollTop || 0
}

function handleYAMLInput() {
  updateYAMLCurrentLine()
  runYAMLSearch(false)
}

function runYAMLSearch(keepIndex = true) {
  const keyword = yamlSearch.keyword
  const content = yamlEditor.yaml || ''
  if (!keyword) {
    yamlSearch.matches = []
    yamlSearch.activeIndex = -1
    return
  }

  const source = content.toLowerCase()
  const target = keyword.toLowerCase()
  const matches = []
  let start = 0
  while (start <= source.length) {
    const index = source.indexOf(target, start)
    if (index === -1) break
    matches.push({ start: index, end: index + keyword.length })
    start = index + Math.max(1, keyword.length)
  }
  yamlSearch.matches = matches
  if (!matches.length) {
    yamlSearch.activeIndex = -1
    return
  }
  if (keepIndex && yamlSearch.activeIndex >= 0 && yamlSearch.activeIndex < matches.length) {
    return
  }
  yamlSearch.activeIndex = 0
}

function focusYAMLSearchMatch(index) {
  const textarea = getYAMLTextareaElement()
  if (!textarea || !yamlSearch.matches.length) return
  const nextIndex = (index + yamlSearch.matches.length) % yamlSearch.matches.length
  const match = yamlSearch.matches[nextIndex]
  yamlSearch.activeIndex = nextIndex
  textarea.focus()
  textarea.setSelectionRange(match.start, match.end)
  updateYAMLCurrentLine()
  const lineBefore = yamlEditor.yaml.slice(0, match.start).split('\n').length - 1
  const targetScrollTop = Math.max(0, lineBefore * yamlLineHeight - textarea.clientHeight / 2)
  textarea.scrollTop = targetScrollTop
  yamlEditorScrollTop.value = targetScrollTop
}

function searchYAMLNext() {
  if (!yamlSearch.matches.length) return
  const nextIndex = yamlSearch.activeIndex + 1 >= yamlSearch.matches.length ? 0 : yamlSearch.activeIndex + 1
  focusYAMLSearchMatch(nextIndex)
}

function searchYAMLPrev() {
  if (!yamlSearch.matches.length) return
  const nextIndex = yamlSearch.activeIndex - 1 < 0 ? yamlSearch.matches.length - 1 : yamlSearch.activeIndex - 1
  focusYAMLSearchMatch(nextIndex)
}

function buildYAMLDiffLines(beforeText, afterText) {
  const before = String(beforeText || '').replace(/\r\n/g, '\n').split('\n')
  const after = String(afterText || '').replace(/\r\n/g, '\n').split('\n')
  const dp = Array.from({ length: before.length + 1 }, () => Array(after.length + 1).fill(0))

  for (let i = before.length - 1; i >= 0; i--) {
    for (let j = after.length - 1; j >= 0; j--) {
      if (before[i] === after[j]) {
        dp[i][j] = dp[i + 1][j + 1] + 1
      } else {
        dp[i][j] = Math.max(dp[i + 1][j], dp[i][j + 1])
      }
    }
  }

  const result = []
  let i = 0
  let j = 0
  while (i < before.length && j < after.length) {
    if (before[i] === after[j]) {
      result.push({ type: 'same', text: before[i] })
      i++
      j++
      continue
    }
    if (dp[i + 1][j] >= dp[i][j + 1]) {
      result.push({ type: 'removed', text: before[i] })
      i++
    } else {
      result.push({ type: 'added', text: after[j] })
      j++
    }
  }
  while (i < before.length) {
    result.push({ type: 'removed', text: before[i] })
    i++
  }
  while (j < after.length) {
    result.push({ type: 'added', text: after[j] })
    j++
  }
  return result
}

function supportsScale(row) {
  return ['Deployment', 'StatefulSet'].includes(row.type)
}

function supportsRestart(row) {
  return ['Deployment', 'StatefulSet', 'DaemonSet'].includes(row.type)
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
    namespaceFilter.value = '__all__'
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
    Object.keys(workloadImageMap).forEach((key) => delete workloadImageMap[key])
    selectedWorkloads.value = []
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
    restoreNamespaceFilter()
    localStorage.setItem(CLUSTER_KEY, String(clusterId))
    void hydrateWorkloadImages()
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

async function openNamespaceYAML(row) {
  if (!cluster.value?.id) return
  const detail = await queryK8sNamespaceDetail(cluster.value.id, row.name)
  setYAMLEditor({
    title: buildYAMLTitle('namespace', detail.name),
    resourceType: 'namespace',
    name: detail.name,
    yaml: detail.yaml
  })
}

async function openWorkloadDetail(row) {
  if (!cluster.value?.id) return
  workloadDrawerVisible.value = true
  workloadDrawerLoading.value = true
  workloadDetail.value = null
  try {
    workloadDetail.value = await queryK8sWorkloadDetail(cluster.value.id, row.namespace, row.type, row.name)
  } finally {
    workloadDrawerLoading.value = false
  }
}

async function openWorkloadYAML(row) {
  if (!cluster.value?.id) return
  const detail = await queryK8sWorkloadDetail(cluster.value.id, row.namespace, row.type, row.name)
  setYAMLEditor({
    title: buildYAMLTitle('workload', detail.name),
    resourceType: 'workload',
    namespace: detail.namespace,
    name: detail.name,
    workloadType: detail.type,
    yaml: detail.yaml
  })
}

function openScaleDialog(row) {
  scaleForm.namespace = row.namespace
  scaleForm.workloadType = row.type
  scaleForm.workloadName = row.name
  const parts = String(row.ready || '').split('/')
  const currentReplicas = Number(parts[1] || parts[0] || 1)
  scaleForm.replicas = Number.isFinite(currentReplicas) ? currentReplicas : 1
  scaleDialogVisible.value = true
}

async function submitScale() {
  if (!cluster.value?.id) return
  scaleLoading.value = true
  try {
    await scaleK8sWorkload({
      clusterId: cluster.value.id,
      namespace: scaleForm.namespace,
      workloadType: scaleForm.workloadType,
      workloadName: scaleForm.workloadName,
      replicas: Number(scaleForm.replicas)
    })
    ElMessage.success(t('k8sWorkloadScaledSuccess'))
    scaleDialogVisible.value = false
    await refreshCurrentClusterData()
    if (workloadDrawerVisible.value && workloadDetail.value?.name === scaleForm.workloadName) {
      workloadDetail.value = await queryK8sWorkloadDetail(
        cluster.value.id,
        scaleForm.namespace,
        scaleForm.workloadType,
        scaleForm.workloadName
      )
    }
  } finally {
    scaleLoading.value = false
  }
}

async function handleRestartWorkload(row) {
  if (!cluster.value?.id) return
  await ElMessageBox.confirm(t('k8sConfirmRestartMessage', { type: row.type, name: row.name }), t('k8sConfirmRestartTitle'), {
    type: 'warning'
  })
  await restartK8sWorkload({
    clusterId: cluster.value.id,
    namespace: row.namespace,
    workloadType: row.type,
    workloadName: row.name
  })
  ElMessage.success(t('k8sWorkloadRestartedSuccess'))
  await refreshCurrentClusterData()
  if (workloadDrawerVisible.value && workloadDetail.value?.name === row.name) {
    workloadDetail.value = await queryK8sWorkloadDetail(cluster.value.id, row.namespace, row.type, row.name)
  }
}

async function openPodDetail(row) {
  if (!cluster.value?.id) return
  podDrawerVisible.value = true
  podDrawerLoading.value = true
  podDetail.value = null
  podEvents.value = []
  try {
    const [detail, events] = await Promise.all([
      queryK8sPodDetail(cluster.value.id, row.namespace, row.name),
      queryK8sPodEvents(cluster.value.id, row.namespace, row.name)
    ])
    podDetail.value = detail
    podEvents.value = events || []
  } finally {
    podDrawerLoading.value = false
  }
}

async function openPodLogs(row) {
  if (!cluster.value?.id || !row?.namespace || !row?.name) return
  podLogDrawerVisible.value = true
  podLogLoading.value = true
  podLogs.value = ''
  selectedContainer.value = ''
  podLogTailLines.value = 200
  currentPodQuery.namespace = row.namespace
  currentPodQuery.podName = row.name
  try {
    const containers = await queryK8sPodContainers(cluster.value.id, row.namespace, row.name)
    selectedContainer.value = containers?.[0] || ''
    await refreshPodLogs()
  } finally {
    podLogLoading.value = false
  }
}

async function openPodYAML(row) {
  if (!cluster.value?.id) return
  const detail = await queryK8sPodDetail(cluster.value.id, row.namespace, row.name)
  setYAMLEditor({
    title: buildYAMLTitle('pod', detail.name),
    resourceType: 'pod',
    namespace: detail.namespace,
    name: detail.name,
    yaml: detail.yaml
  })
}

async function refreshPodLogs() {
  if (!cluster.value?.id || !currentPodQuery.namespace || !currentPodQuery.podName) return
  const data = await queryK8sPodLogs(
    cluster.value.id,
    currentPodQuery.namespace,
    currentPodQuery.podName,
    selectedContainer.value,
    podLogTailLines.value
  )
  podLogs.value = data.content || ''
}

async function openPodTerminal(row) {
  if (!cluster.value?.id) return
  let container = ''
  try {
    const containers = await queryK8sPodContainers(cluster.value.id, row.namespace, row.name)
    container = containers?.[0] || ''
  } catch (error) {
    container = ''
  }
  router.push({
    name: 'K8sPodTerminal',
    params: {
      clusterId: String(cluster.value.id),
      namespace: row.namespace,
      podName: row.name
    },
    query: container ? { container } : undefined
  })
}

async function openServiceDetail(row) {
  if (!cluster.value?.id) return
  serviceDrawerVisible.value = true
  serviceDrawerLoading.value = true
  serviceDetail.value = null
  try {
    serviceDetail.value = await queryK8sServiceDetail(cluster.value.id, row.namespace, row.name)
  } finally {
    serviceDrawerLoading.value = false
  }
}

async function openServiceYAML(row) {
  if (!cluster.value?.id) return
  const detail = await queryK8sServiceDetail(cluster.value.id, row.namespace, row.name)
  setYAMLEditor({
    title: buildYAMLTitle('service', detail.name),
    resourceType: 'service',
    namespace: detail.namespace,
    name: detail.name,
    yaml: detail.yaml
  })
}

async function openIngressDetail(row) {
  if (!cluster.value?.id) return
  ingressDrawerVisible.value = true
  ingressDrawerLoading.value = true
  ingressDetail.value = null
  try {
    ingressDetail.value = await queryK8sIngressDetail(cluster.value.id, row.namespace, row.name)
  } finally {
    ingressDrawerLoading.value = false
  }
}

async function openIngressYAML(row) {
  if (!cluster.value?.id) return
  const detail = await queryK8sIngressDetail(cluster.value.id, row.namespace, row.name)
  setYAMLEditor({
    title: buildYAMLTitle('ingress', detail.name),
    resourceType: 'ingress',
    namespace: detail.namespace,
    name: detail.name,
    yaml: detail.yaml
  })
}

function handlePodPageSizeChange(size) {
  podPageSize.value = size
  podPage.value = 1
}

function handlePodPageChange(page) {
  podPage.value = page
}

async function handleDeletePod(row) {
  if (!cluster.value?.id || !row?.namespace || !row?.name) return
  await ElMessageBox.confirm(
    `确认删除 Pod ${row.namespace}/${row.name}？`,
    '确认删除',
    {
      type: 'warning',
      confirmButtonText: t('k8sDelete'),
      cancelButtonText: t('cancel')
    }
  )
  await deleteK8sResource({
    clusterId: cluster.value.id,
    resourceType: 'pod',
    namespace: row.namespace,
    name: row.name
  })
  ElMessage.success('Pod 已删除')
  if (podDrawerVisible.value && podDetail.value?.name === row.name && podDetail.value?.namespace === row.namespace) {
    podDrawerVisible.value = false
  }
  if (podLogDrawerVisible.value && currentPodQuery.podName === row.name && currentPodQuery.namespace === row.namespace) {
    podLogDrawerVisible.value = false
  }
  await refreshCurrentClusterData()
  const maxPage = Math.max(1, Math.ceil(filteredPods.value.length / podPageSize.value))
  if (podPage.value > maxPage) {
    podPage.value = maxPage
  }
}

async function openWorkloadResourceSettings(row) {
  if (!cluster.value?.id) return
  const detail = await queryK8sWorkloadDetail(cluster.value.id, row.namespace, row.type, row.name)
  workloadResourceForm.namespace = detail.namespace
  workloadResourceForm.workloadType = detail.type
  workloadResourceForm.workloadName = detail.name
  workloadResourceForm.containers = (detail.containers || []).map((item) => ({
    name: item.name,
    image: item.image || '',
    requestCPU: item.requestCPU || '',
    limitCPU: item.limitCPU || '',
    requestMemory: item.requestMemory || '',
    limitMemory: item.limitMemory || '',
    imagePullPolicy: item.imagePullPolicy || 'IfNotPresent',
    env: (item.env || []).map((env) => ({
      name: env.name || '',
      value: env.value || '',
      valueFrom: env.valueFrom || null,
      source: env.source || ''
    }))
  }))
  workloadResourceDialogVisible.value = true
}

function addWorkloadEnvironment(container) {
  if (!Array.isArray(container.env)) container.env = []
  container.env.push({ name: '', value: '', valueFrom: null, source: '' })
}

function removeWorkloadEnvironment(container, index) {
  if (Array.isArray(container.env)) container.env.splice(index, 1)
}

async function submitWorkloadResourceSettings() {
  if (!cluster.value?.id || !workloadResourceForm.containers.length) return
  workloadResourceSaving.value = true
  try {
    await updateK8sWorkloadResources({ clusterId: cluster.value.id, ...workloadResourceForm })
    ElMessage.success('Pod 资源设置已更新')
    workloadResourceDialogVisible.value = false
    await refreshCurrentClusterData()
    if (workloadDrawerVisible.value && workloadDetail.value?.name === workloadResourceForm.workloadName) {
      workloadDetail.value = await queryK8sWorkloadDetail(cluster.value.id, workloadResourceForm.namespace, workloadResourceForm.workloadType, workloadResourceForm.workloadName)
    }
  } finally {
    workloadResourceSaving.value = false
  }
}

function openNamespaceCreate() {
  namespaceCreateForm.name = ''
  namespaceCreateVisible.value = true
}

function configStorageCreateKind() {
  if (configStorageTab.value === 'secrets') return 'secret'
  if (configStorageTab.value === 'storage-volumes') return 'pvc'
  return 'configmap'
}

const storageAccessModeOptions = computed(() => (
  storageClassCreateForm.sourceType === 'hostpath'
    ? [{ value: 'ReadWriteOnce', label: '单节点读写（ReadWriteOnce）' }]
    : [
        { value: 'ReadWriteOnce', label: '单节点读写（ReadWriteOnce）' },
        { value: 'ReadOnlyMany', label: '多节点只读（ReadOnlyMany）' },
        { value: 'ReadWriteMany', label: '多节点读写（ReadWriteMany）' }
      ]
))

const pvcStorageClassOptions = computed(() => (
  storages.value
    .filter((item) => item.kind === 'PV' && String(item.storageClass || item.name || '').trim())
    .map((item) => {
      const value = String(item.storageClass || item.name).trim()
      const scope = String(item.namespaceScope || '').trim()
      return {
        value,
        label: scope && scope !== '集群级'
          ? `${value}（限定：${scope}）`
          : `${value}（集群级）`,
        scope,
        accessModes: String(item.accessModes || 'ReadWriteOnce')
      }
    })
))

const selectedPVCStorageClass = computed(() => (
  pvcStorageClassOptions.value.find((item) => item.value === configStorageCreateForm.storageClass) || null
))

const pvcStorageClassScope = computed(() => {
  const scope = selectedPVCStorageClass.value?.scope || ''
  return scope && scope !== '集群级' ? scope : ''
})

const pvcNamespaceOptions = computed(() => {
  const options = namespaceOptions.value.filter((item) => item.value !== '__all__')
  return pvcStorageClassScope.value
    ? options.filter((item) => item.value === pvcStorageClassScope.value)
    : options
})

const pvcNamespaceLocked = computed(() => Boolean(pvcStorageClassScope.value))

const pvcAccessMode = computed(() => (
  String(selectedPVCStorageClass.value?.accessModes || 'ReadWriteOnce')
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)[0] || 'ReadWriteOnce'
))

const pvcAccessModeLabel = computed(() => {
  if (!selectedPVCStorageClass.value) return '请先选择存储类'
  const labels = {
    ReadWriteOnce: '单节点读写（ReadWriteOnce）',
    ReadOnlyMany: '多节点只读（ReadOnlyMany）',
    ReadWriteMany: '多节点读写（ReadWriteMany）'
  }
  return labels[pvcAccessMode.value] || pvcAccessMode.value
})

watch(() => configStorageCreateForm.storageClass, () => {
  if (configStorageCreateForm.kind !== 'pvc') return
  configStorageCreateForm.accessMode = pvcAccessMode.value
  if (pvcStorageClassScope.value) configStorageCreateForm.namespace = pvcStorageClassScope.value
})

function openStorageClassCreate() {
  storageClassCreateForm.name = ''
  storageClassCreateForm.sourceType = 'hostpath'
  storageClassCreateForm.capacity = '10Gi'
  storageClassCreateForm.reclaimPolicy = 'Delete'
  storageClassCreateForm.accessMode = 'ReadWriteOnce'
  storageClassCreateForm.path = ''
  storageClassCreateForm.nfsServer = ''
  storageClassCreateForm.scopeNamespaceEnabled = false
  storageClassCreateForm.scopeNamespace = ''
  storageClassCreateVisible.value = true
}

async function submitStorageClassCreate() {
  if (!cluster.value?.id) return
  const name = String(storageClassCreateForm.name || '').trim().toLowerCase()
  const path = String(storageClassCreateForm.path || '').trim()
  const capacity = String(storageClassCreateForm.capacity || '').trim()
  const sourceType = storageClassCreateForm.sourceType
  const nfsServer = String(storageClassCreateForm.nfsServer || '').trim()
  const scopeNamespace = storageClassCreateForm.scopeNamespaceEnabled
    ? String(storageClassCreateForm.scopeNamespace || '').trim()
    : ''
  if (!validK8sResourceName(name)) {
    ElMessage.warning('请输入符合 Kubernetes 命名规范的存储类名称')
    return
  }
  if (!capacity || !path) {
    ElMessage.warning('请填写容量和存储路径')
    return
  }
  if (sourceType === 'nfs' && !nfsServer) {
    ElMessage.warning('NFS 存储源需要填写 NFS 服务地址')
    return
  }
  if (storageClassCreateForm.scopeNamespaceEnabled && !scopeNamespace) {
    ElMessage.warning('请选择限定的命名空间')
    return
  }
  const manifest = {
    apiVersion: 'v1',
    kind: 'PersistentVolume',
    metadata: { name },
    spec: {
      capacity: { storage: capacity },
      volumeMode: 'Filesystem',
      accessModes: [storageClassCreateForm.accessMode],
      persistentVolumeReclaimPolicy: storageClassCreateForm.reclaimPolicy,
      storageClassName: name
    }
  }
  if (scopeNamespace) {
    manifest.metadata.annotations = { 'ops-admin.io/namespace-scope': scopeNamespace }
  }
  if (sourceType === 'hostpath') {
    manifest.spec.hostPath = { path, type: 'DirectoryOrCreate' }
  } else {
    manifest.spec.nfs = { server: nfsServer, path }
  }
  storageClassCreateSaving.value = true
  try {
    await createK8sResourceYAML({
      clusterId: cluster.value.id,
      resourceType: 'pv',
      name,
      yaml: JSON.stringify(manifest, null, 2)
    })
    ElMessage.success('新增存储类成功')
    storageClassCreateVisible.value = false
    await refreshCurrentClusterData()
  } finally {
    storageClassCreateSaving.value = false
  }
}

async function deleteStorageClass(row) {
  if (!cluster.value?.id) return
  if (String(row?.status || '').toLowerCase() !== 'available') {
    ElMessage.warning('仅未绑定存储卷的存储类可以删除')
    return
  }
  await ElMessageBox.confirm(
    `确认删除存储类“${row.name}”？删除后不可恢复。`,
    '删除存储类',
    { type: 'warning', confirmButtonText: '确认删除', cancelButtonText: '取消' }
  )
  await deleteK8sResource({
    clusterId: cluster.value.id,
    resourceType: 'pv',
    name: row.name
  })
  ElMessage.success('存储类已删除')
  await refreshCurrentClusterData()
}

async function deleteStorageVolume(row) {
  if (!cluster.value?.id || !row?.namespace || !row?.name) return
  await ElMessageBox.confirm(
    `确认删除存储卷“${row.namespace}/${row.name}”？删除后不可恢复。`,
    '删除存储卷',
    { type: 'warning', confirmButtonText: '确认删除', cancelButtonText: '取消' }
  )
  await deleteK8sResource({
    clusterId: cluster.value.id,
    resourceType: 'pvc',
    namespace: row.namespace,
    name: row.name
  })
  if (storageDrawerVisible.value && storageDetail.value?.kind === 'PVC' && storageDetail.value?.name === row.name && storageDetail.value?.namespace === row.namespace) {
    storageDrawerVisible.value = false
  }
  ElMessage.success('存储卷已删除')
  await refreshCurrentClusterData()
}

function configStorageCreateTitle() {
  const action = configStorageEditing.value ? '编辑' : '新建'
  const titles = {
    configmap: `${action} ConfigMap`,
    secret: `${action} Secret`,
    pvc: '新增存储（PVC）'
  }
  return titles[configStorageCreateForm.kind] || titles.configmap
}

function openConfigStorageCreate() {
  const kind = configStorageCreateKind()
  configStorageEditing.value = false
  configStorageCreateForm.kind = kind
  configStorageCreateForm.namespace = namespaceFilter.value !== '__all__'
    ? namespaceFilter.value
    : (namespaces.value[0]?.name || '')
  configStorageCreateForm.name = ''
  configStorageCreateForm.entries = [{ key: '', value: '' }]
  configStorageCreateForm.secretType = 'Opaque'
  configStorageCreateForm.capacity = '1Gi'
  configStorageCreateForm.storageClass = ''
  configStorageCreateForm.accessMode = 'ReadWriteOnce'
  configStorageCreateVisible.value = true
}

function addConfigStorageEntry() {
  configStorageCreateForm.entries.push({ key: '', value: '' })
}

function removeConfigStorageEntry(index) {
  if (configStorageCreateForm.entries.length <= 1) return
  configStorageCreateForm.entries.splice(index, 1)
}

function validK8sResourceName(value) {
  const name = String(value || '').trim().toLowerCase()
  return /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/.test(name) && name.length <= 63
}

async function submitConfigStorageCreate() {
  if (!cluster.value?.id) return
  const { kind, namespace } = configStorageCreateForm
  const name = String(configStorageCreateForm.name || '').trim().toLowerCase()
  if (!namespace) {
    ElMessage.warning('请选择命名空间')
    return
  }
  if (!validK8sResourceName(name)) {
    ElMessage.warning('名称必须为不超过 63 位的小写字母、数字或连字符，且不能以连字符开头或结尾')
    return
  }

  let manifest
  if (kind === 'pvc') {
    const capacity = String(configStorageCreateForm.capacity || '').trim()
    const storageClass = String(configStorageCreateForm.storageClass || '').trim()
    if (!capacity) {
      ElMessage.warning('请输入存储容量，例如 1Gi')
      return
    }
    if (!storageClass || !selectedPVCStorageClass.value) {
      ElMessage.warning('请选择存储类')
      return
    }
    if (pvcStorageClassScope.value && namespace !== pvcStorageClassScope.value) {
      ElMessage.warning(`存储类“${storageClass}”仅允许命名空间“${pvcStorageClassScope.value}”创建存储卷`)
      return
    }
    manifest = {
      apiVersion: 'v1',
      kind: 'PersistentVolumeClaim',
      metadata: { name, namespace },
      spec: {
        accessModes: [pvcAccessMode.value],
        resources: { requests: { storage: capacity } },
        storageClassName: storageClass
      }
    }
  } else {
    const entries = configStorageCreateForm.entries || []
    const values = {}
    for (const entry of entries) {
      const key = String(entry.key || '').trim()
      if (!key) {
        ElMessage.warning('请填写每一项的键名')
        return
      }
      if (Object.prototype.hasOwnProperty.call(values, key)) {
        ElMessage.warning(`键名“${key}”重复，请调整后再创建`)
        return
      }
      values[key] = String(entry.value ?? '')
    }
    manifest = {
      apiVersion: 'v1',
      kind: kind === 'secret' ? 'Secret' : 'ConfigMap',
      metadata: { name, namespace }
    }
    if (kind === 'secret') {
      manifest.type = configStorageCreateForm.secretType || 'Opaque'
      manifest.stringData = values
    } else {
      manifest.data = values
    }
  }

  configStorageCreateSaving.value = true
  try {
    const payload = {
      clusterId: cluster.value.id,
      resourceType: kind,
      namespace,
      name,
      yaml: JSON.stringify(manifest, null, 2)
    }
    if (configStorageEditing.value) {
      await updateK8sResourceYAML(payload)
    } else {
      await createK8sResourceYAML(payload)
    }
    ElMessage.success(`${configStorageCreateTitle()}成功`)
    configStorageCreateVisible.value = false
    await refreshCurrentClusterData()
  } finally {
    configStorageCreateSaving.value = false
  }
}

async function submitNamespaceCreate() {
  if (!cluster.value?.id) return
  const name = String(namespaceCreateForm.name || '').trim().toLowerCase()
  if (!/^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/.test(name) || name.length > 63) {
    ElMessage.warning(t('k8sNamespaceNameInvalid'))
    return
  }
  namespaceCreateSaving.value = true
  try {
    await createK8sResourceYAML({
      clusterId: cluster.value.id,
      resourceType: 'namespace',
      name,
      yaml: `apiVersion: v1\nkind: Namespace\nmetadata:\n  name: ${name}\n`
    })
    ElMessage.success(t('k8sNamespaceCreatedSuccess'))
    namespaceCreateVisible.value = false
    await refreshCurrentClusterData()
  } finally {
    namespaceCreateSaving.value = false
  }
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
      ElMessage.warning('标签键不能为空')
      return
    }
    if (Object.prototype.hasOwnProperty.call(labels, key)) {
      ElMessage.warning(`标签键重复：${key}`)
      return
    }
    labels[key] = String(item.value ?? '').trim()
  }
  nodeLabelsSaving.value = true
  try {
    await updateK8sNodeLabels({ clusterId: cluster.value.id, nodeName: nodeLabelTarget.value.name, labels })
    ElMessage.success('节点标签已更新')
    nodeLabelsVisible.value = false
    await refreshCurrentClusterData()
    if (nodeDetail.value?.name === nodeLabelTarget.value.name) {
      nodeDetail.value = await queryK8sNodeDetail(cluster.value.id, nodeLabelTarget.value.name)
    }
  } finally {
    nodeLabelsSaving.value = false
  }
}

function formatAgeFromTimestamp(value) {
  if (!value) return '-'
  const timestamp = new Date(value)
  if (Number.isNaN(timestamp.getTime())) return String(value)
  const diffMs = Date.now() - timestamp.getTime()
  const diffMinutes = Math.max(0, Math.floor(diffMs / 60000))
  if (diffMinutes < 60) return `${diffMinutes || 1}m`
  const diffHours = Math.floor(diffMinutes / 60)
  if (diffHours < 24) return `${diffHours}h`
  const diffDays = Math.floor(diffHours / 24)
  if (diffDays < 30) return `${diffDays}d`
  const diffMonths = Math.floor(diffDays / 30)
  if (diffMonths < 12) return `${diffMonths}mo`
  const diffYears = Math.floor(diffDays / 365)
  return `${diffYears}y`
}

function deriveWorkloadPhase(item) {
  const readyText = String(item.ready || '')
  if (!readyText.includes('/')) return item.type || '-'
  const [readyCountText, desiredCountText] = readyText.split('/')
  const readyCount = Number(readyCountText || 0)
  const desiredCount = Number(desiredCountText || 0)
  if (desiredCount > 0 && readyCount >= desiredCount) return t('k8sStatusRunning')
  if (readyCount > 0) return t('k8sStatusWarning')
  return t('k8sStatusOffline')
}

function isWorkloadHealthy(item) {
  const readyText = String(item.ready || '')
  if (!readyText.includes('/')) return Boolean(item.available)
  const [readyCountText, desiredCountText] = readyText.split('/')
  const readyCount = Number(readyCountText || 0)
  const desiredCount = Number(desiredCountText || 0)
  return desiredCount > 0 ? readyCount >= desiredCount : readyCount > 0
}

function buildWorkloadCacheKey(item) {
  return `${item.namespace || ''}/${item.type || ''}/${item.name || ''}`
}

function extractImagesFromContainers(containers) {
  if (!Array.isArray(containers) || !containers.length) return ''
  return containers.map((item) => item.image).filter(Boolean).join('\n')
}

async function hydrateWorkloadImages() {
  if (!cluster.value?.id) return
  const targets = kuboardWorkloadRows.value
    .filter((item) => !item.images)
    .filter((item) => ['Deployment', 'StatefulSet', 'DaemonSet', 'CronJob', 'Job'].includes(item.type))
    .slice(0, 24)

  await Promise.all(
    targets.map(async (item) => {
      const key = buildWorkloadCacheKey(item)
      if (workloadImageMap[key] || workloadImageLoadingMap[key]) return
      workloadImageLoadingMap[key] = true
      try {
        const detail = await queryK8sWorkloadDetail(cluster.value.id, item.namespace, item.type, item.name)
        workloadImageMap[key] = extractImagesFromContainers(detail?.containers)
      } catch (error) {
        workloadImageMap[key] = ''
      } finally {
        delete workloadImageLoadingMap[key]
      }
    })
  )
}

async function openIstioResourceDetail(row, resourceType) {
  if (!cluster.value?.id) return
  istioDrawerVisible.value = true
  istioDrawerLoading.value = true
  istioDetail.value = null
  try {
    istioDetail.value = await queryK8sIstioResourceDetail(cluster.value.id, resourceType, row.namespace, row.name)
  } finally {
    istioDrawerLoading.value = false
  }
}

async function openIstioResourceYAML(row, resourceType) {
  if (!cluster.value?.id) return
  const detail = await queryK8sIstioResourceDetail(cluster.value.id, resourceType, row.namespace, row.name)
  setYAMLEditor({
    title: buildYAMLTitle(resourceType, detail.name),
    resourceType,
    namespace: detail.namespace,
    name: detail.name,
    yaml: detail.yaml
  })
}

function openIstioCreateDialog(resourceType) {
  istioCreateForm.resourceType = resourceType
  istioCreateForm.yaml = buildIstioTemplate(resourceType)
  istioCreateDialogVisible.value = true
}

async function submitIstioCreate() {
  if (!cluster.value?.id) return
  await ElMessageBox.confirm(
    t('k8sCreateIstioResourceConfirm', { resource: yamlResourceLabel(istioCreateForm.resourceType) }),
    t('k8sCreateIstioResourceTitle', { resource: yamlResourceLabel(istioCreateForm.resourceType) }),
    {
      type: 'warning',
      confirmButtonText: t('confirmChange'),
      cancelButtonText: t('cancel')
    }
  )
  istioCreateSaving.value = true
  try {
    await createK8sResourceYAML({
      clusterId: cluster.value.id,
      resourceType: istioCreateForm.resourceType,
      namespace: '',
      yaml: istioCreateForm.yaml
    })
    ElMessage.success(t('k8sIstioResourceCreatedSuccess'))
    istioCreateDialogVisible.value = false
    await refreshCurrentClusterData()
  } finally {
    istioCreateSaving.value = false
  }
}

async function handleDeleteIstioResource(row, resourceType) {
  if (!cluster.value?.id) return
  await ElMessageBox.confirm(
    t('k8sDeleteIstioResourceConfirm', {
      resource: yamlResourceLabel(resourceType),
      name: row.name
    }),
    t('k8sDeleteIstioResourceTitle'),
    {
      type: 'warning',
      confirmButtonText: t('k8sDelete'),
      cancelButtonText: t('cancel')
    }
  )
  await deleteK8sResource({
    clusterId: cluster.value.id,
    resourceType,
    namespace: row.namespace,
    name: row.name
  })
  ElMessage.success(t('k8sIstioResourceDeletedSuccess'))
  if (istioDrawerVisible.value && istioDetail.value?.name === row.name) {
    istioDrawerVisible.value = false
  }
  await refreshCurrentClusterData()
}

async function openTrafficDialog(row) {
  if (!cluster.value?.id) return
  const resourceType = row.resourceType || 'virtualservice'
  const detail = await queryK8sIstioResourceDetail(cluster.value.id, resourceType, row.namespace, row.name)
  if (!detail.traffic?.length) {
    ElMessage.warning(t('k8sNoTrafficRoutes'))
    return
  }
  trafficForm.resourceType = resourceType
  trafficForm.namespace = detail.namespace
  trafficForm.name = detail.name
  trafficForm.routes = detail.traffic.map((item) => ({
    index: item.index,
    host: item.host,
    subset: item.subset,
    port: item.port,
    label: item.label,
    weight: Number(item.weight || 0)
  }))
  trafficDialogVisible.value = true
}

async function submitTrafficAdjust() {
  if (!cluster.value?.id) return
  if (trafficTotalWeight.value !== 100) {
    ElMessage.warning(t('k8sTrafficWeightTotalInvalid', { total: String(trafficTotalWeight.value) }))
    return
  }
  await ElMessageBox.confirm(
    t('k8sAdjustTrafficConfirm', { name: trafficForm.name }),
    t('k8sAdjustTrafficTitle'),
    {
      type: 'warning',
      confirmButtonText: t('confirmChange'),
      cancelButtonText: t('cancel')
    }
  )
  trafficSaving.value = true
  try {
    const submitter = trafficForm.resourceType === 'httproute' ? updateK8sHTTPRouteTraffic : updateK8sIstioTraffic
    await submitter({
      clusterId: cluster.value.id,
      namespace: trafficForm.namespace,
      name: trafficForm.name,
      routes: trafficForm.routes.map((item) => ({
        index: item.index,
        weight: Number(item.weight || 0)
      }))
    })
    ElMessage.success(t('k8sTrafficUpdatedSuccess'))
    trafficDialogVisible.value = false
    await refreshCurrentClusterData()
    if (istioDrawerVisible.value && istioDetail.value?.name === trafficForm.name) {
      istioDetail.value = await queryK8sIstioResourceDetail(
        cluster.value.id,
        trafficForm.resourceType || 'virtualservice',
        trafficForm.namespace,
        trafficForm.name
      )
    }
  } finally {
    trafficSaving.value = false
  }
}

async function openConfigMapDetail(row) {
  if (!cluster.value?.id) return
  configMapDrawerVisible.value = true
  configMapDrawerLoading.value = true
  configMapDetail.value = null
  try {
    configMapDetail.value = await queryK8sConfigMapDetail(cluster.value.id, row.namespace, row.name)
  } finally {
    configMapDrawerLoading.value = false
  }
}

async function openConfigMapYAML(row) {
  if (!cluster.value?.id) return
  const detail = await queryK8sConfigMapDetail(cluster.value.id, row.namespace, row.name)
  setYAMLEditor({
    title: buildYAMLTitle('configmap', detail.name),
    resourceType: 'configmap',
    namespace: detail.namespace,
    name: detail.name,
    yaml: detail.yaml
  })
}

async function openConfigMapEdit(row) {
  if (!cluster.value?.id) return
  const detail = await queryK8sConfigMapDetail(cluster.value.id, row.namespace, row.name)
  configStorageEditing.value = true
  configStorageCreateForm.kind = 'configmap'
  configStorageCreateForm.namespace = detail.namespace
  configStorageCreateForm.name = detail.name
  configStorageCreateForm.entries = (detail.keys || []).map((item) => ({ key: item.label, value: item.value || '' }))
  if (!configStorageCreateForm.entries.length) configStorageCreateForm.entries = [{ key: '', value: '' }]
  configStorageCreateVisible.value = true
}

async function deleteConfigMap(row) {
  if (!cluster.value?.id) return
  await ElMessageBox.confirm(`确认删除 ConfigMap “${row.name}” 吗？此操作不可恢复。`, '删除 ConfigMap', {
    type: 'warning',
    confirmButtonText: t('k8sDelete'),
    cancelButtonText: t('cancel')
  })
  await deleteK8sResource({
    clusterId: cluster.value.id,
    resourceType: 'configmap',
    namespace: row.namespace,
    name: row.name
  })
  if (configMapDetail.value?.name === row.name && configMapDetail.value?.namespace === row.namespace) {
    configMapDrawerVisible.value = false
  }
  ElMessage.success('ConfigMap 已删除')
  await refreshCurrentTab()
}

async function openSecretDetail(row) {
  if (!cluster.value?.id) return
  secretDrawerVisible.value = true
  secretDrawerLoading.value = true
  secretDetail.value = null
  try {
    secretDetail.value = await queryK8sSecretDetail(cluster.value.id, row.namespace, row.name)
  } finally {
    secretDrawerLoading.value = false
  }
}

async function openSecretYAML(row) {
  if (!cluster.value?.id) return
  const detail = await queryK8sSecretDetail(cluster.value.id, row.namespace, row.name)
  setYAMLEditor({
    title: buildYAMLTitle('secret', detail.name),
    resourceType: 'secret',
    namespace: detail.namespace,
    name: detail.name,
    yaml: detail.yaml
  })
}

async function openStorageDetail(row) {
  if (!cluster.value?.id) return
  storageDrawerVisible.value = true
  storageDrawerLoading.value = true
  storageDetail.value = null
  try {
    storageDetail.value = await queryK8sStorageDetail(cluster.value.id, row.kind, row.namespace, row.name)
  } finally {
    storageDrawerLoading.value = false
  }
}

async function openStorageYAML(row) {
  if (!cluster.value?.id) return
  const detail = await queryK8sStorageDetail(cluster.value.id, row.kind, row.namespace, row.name)
  setYAMLEditor({
    title: buildYAMLTitle(String(detail.kind || '').toLowerCase(), detail.name),
    resourceType: String(detail.kind || '').toLowerCase(),
    namespace: detail.namespace === '-' ? '' : detail.namespace,
    name: detail.name,
    yaml: detail.yaml
  })
}

async function refreshCurrentYAMLResource() {
  if (!cluster.value?.id) return
  switch (yamlEditor.resourceType) {
    case 'namespace':
      if (namespaceDetail.value?.name) {
        namespaceDetail.value = await queryK8sNamespaceDetail(cluster.value.id, namespaceDetail.value.name)
      }
      break
    case 'workload':
      if (workloadDetail.value?.name) {
        workloadDetail.value = await queryK8sWorkloadDetail(
          cluster.value.id,
          workloadDetail.value.namespace,
          workloadDetail.value.type,
          workloadDetail.value.name
        )
      }
      break
    case 'pod':
      if (podDetail.value?.name) {
        podDetail.value = await queryK8sPodDetail(cluster.value.id, podDetail.value.namespace, podDetail.value.name)
      }
      break
    case 'service':
      if (serviceDetail.value?.name) {
        serviceDetail.value = await queryK8sServiceDetail(cluster.value.id, serviceDetail.value.namespace, serviceDetail.value.name)
      }
      break
    case 'ingress':
      if (ingressDetail.value?.name) {
        ingressDetail.value = await queryK8sIngressDetail(cluster.value.id, ingressDetail.value.namespace, ingressDetail.value.name)
      }
      break
    case 'gateway':
    case 'virtualservice':
    case 'destinationrule':
    case 'serviceentry':
      if (istioDetail.value?.name) {
        istioDetail.value = await queryK8sIstioResourceDetail(
          cluster.value.id,
          yamlEditor.resourceType,
          istioDetail.value.namespace,
          istioDetail.value.name
        )
      }
      break
    case 'configmap':
      if (configMapDetail.value?.name) {
        configMapDetail.value = await queryK8sConfigMapDetail(cluster.value.id, configMapDetail.value.namespace, configMapDetail.value.name)
      }
      break
    case 'secret':
      if (secretDetail.value?.name) {
        secretDetail.value = await queryK8sSecretDetail(cluster.value.id, secretDetail.value.namespace, secretDetail.value.name)
      }
      break
    case 'pvc':
    case 'pv':
      if (storageDetail.value?.name) {
        storageDetail.value = await queryK8sStorageDetail(
          cluster.value.id,
          storageDetail.value.kind,
          storageDetail.value.kind === 'PV' ? '' : storageDetail.value.namespace,
          storageDetail.value.name
        )
      }
      break
  }
}

async function submitYAMLUpdate() {
  if (!cluster.value?.id) return
  await ElMessageBox.confirm(
    t('k8sConfirmYamlUpdateMessage', {
      added: String(yamlChangeSummary.value.added),
      removed: String(yamlChangeSummary.value.removed)
    }),
    t('k8sConfirmYamlUpdateTitle'),
    {
      type: 'warning',
      confirmButtonText: t('confirmChange'),
      cancelButtonText: t('cancel')
    }
  )
  yamlSaving.value = true
  try {
    await updateK8sResourceYAML({
      clusterId: cluster.value.id,
      resourceType: yamlEditor.resourceType,
      namespace: yamlEditor.namespace,
      name: yamlEditor.name,
      workloadType: yamlEditor.workloadType,
      yaml: yamlEditor.yaml
    })
    ElMessage.success(t('k8sYamlUpdatedSuccess'))
    yamlDialogVisible.value = false
    await refreshCurrentYAMLResource()
  } finally {
    yamlSaving.value = false
  }
}

function yamlResourceLabel(key) {
  const map = {
    namespace: 'k8sResourceNamespace',
    workload: 'k8sResourceWorkload',
    pod: 'k8sResourcePod',
    service: 'k8sResourceService',
    ingress: 'k8sResourceIngress',
    gatewayapi: 'k8sResourceGatewayApi',
    gateway: 'k8sResourceGateway',
    httproute: 'k8sResourceHTTPRoute',
    virtualservice: 'k8sResourceVirtualService',
    destinationrule: 'k8sResourceDestinationRule',
    serviceentry: 'k8sResourceServiceEntry',
    configmap: 'k8sResourceConfigMap',
    secret: 'k8sResourceSecret',
    storage: 'k8sResourceStorage',
    pvc: 'k8sResourceStorage',
    pv: 'k8sResourceStorage'
  }
  return t(map[key] || 'k8sYamlEditor')
}

function buildYAMLTitle(resourceKey, name) {
  return t('k8sEditResourceYamlTitle', {
    resource: yamlResourceLabel(resourceKey),
    name
  })
}

async function openSecretEdit(row) {
  if (!cluster.value?.id) return
  const detail = await queryK8sSecretDetail(cluster.value.id, row.namespace, row.name)
  configStorageEditing.value = true
  configStorageCreateForm.kind = 'secret'
  configStorageCreateForm.namespace = detail.namespace
  configStorageCreateForm.name = detail.name
  configStorageCreateForm.secretType = detail.type || 'Opaque'
  configStorageCreateForm.entries = (detail.keys || []).map((item) => ({ key: item.label, value: item.value || '' }))
  if (!configStorageCreateForm.entries.length) configStorageCreateForm.entries = [{ key: '', value: '' }]
  configStorageCreateVisible.value = true
}

async function deleteSecret(row) {
  if (!cluster.value?.id) return
  await ElMessageBox.confirm(`确认删除 Secret “${row.name}” 吗？此操作不可恢复。`, '删除 Secret', {
    type: 'warning',
    confirmButtonText: t('k8sDelete'),
    cancelButtonText: t('cancel')
  })
  await deleteK8sResource({
    clusterId: cluster.value.id,
    resourceType: 'secret',
    namespace: row.namespace,
    name: row.name
  })
  if (secretDetail.value?.name === row.name && secretDetail.value?.namespace === row.namespace) {
    secretDrawerVisible.value = false
  }
  ElMessage.success('Secret 已删除')
  await refreshCurrentTab()
}

function translateIstioDetailLabel(label) {
  const map = {
    GatewayClass: 'k8sType',
    Selector: 'k8sSelector',
    Hosts: 'k8sHost',
    Ports: 'k8sPorts',
    Gateways: 'k8sGateways',
    Target: 'k8sTarget',
    Subsets: 'k8sSubsets',
    Location: 'k8sLocation',
    Resolution: 'k8sResolution',
    Addresses: 'k8sAddress'
  }
  return map[label] ? t(map[label]) : label
}

const page = reactive({
  t,
  loading,
  switching,
  clusterOptions,
  cluster,
  overview,
  nodes,
  namespaces,
  pods,
  podPage,
  podPageSize,
  configStorageTab,
  configStorageCreateVisible,
  configStorageCreateSaving,
  configStorageEditing,
  configStorageCreateForm,
  storageClassCreateVisible,
  storageClassCreateSaving,
  storageClassCreateForm,
  workloads,
  services,
  ingresses,
  gatewayApiGateways,
  httpRoutes,
  configMaps,
  secrets,
  storages,
  namespaceFilter,
  resourceKeyword,
  namespaceKeyword,
  podScopedNames,
  configMapDrawerVisible,
  configMapDrawerLoading,
  configMapDetail,
  secretDrawerVisible,
  secretDrawerLoading,
  secretDetail,
  storageDrawerVisible,
  storageDrawerLoading,
  storageDetail,
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
  workloadDrawerVisible,
  workloadDrawerLoading,
  workloadDetail,
	workloadResourceDialogVisible,
	workloadResourceSaving,
	workloadResourceForm,
  selectedWorkloads,
  podDrawerVisible,
  podDrawerLoading,
  podDetail,
  podEvents,
  podLogDrawerVisible,
  podLogLoading,
  podLogs,
  selectedContainer,
  podLogTailLines,
  currentPodQuery,
  serviceDrawerVisible,
  serviceDrawerLoading,
  serviceDetail,
  ingressDrawerVisible,
  ingressDrawerLoading,
  ingressDetail,
  istioDrawerVisible,
  istioDrawerLoading,
  istioDetail,
  scaleDialogVisible,
  scaleLoading,
  scaleForm,
  imageVersionDialogVisible,
  imageVersionSaving,
  imageVersionForm,
  istioCreateDialogVisible,
  istioCreateSaving,
  istioCreateForm,
  trafficDialogVisible,
  trafficSaving,
  trafficForm,
  yamlDialogVisible,
  yamlSaving,
  yamlTextareaRef,
  yamlEditor,
  yamlSearch,
  yamlEditorScrollTop,
  yamlCurrentLine,
  currentTab,
  hasCluster,
  statusType,
  namespaceOptions,
  configStorageCreateTitle,
  storageAccessModeOptions,
  pvcStorageClassOptions,
  pvcNamespaceOptions,
  pvcNamespaceLocked,
  pvcAccessModeLabel,
  kuboardMenuGroups,
  currentSection,
  kuboardNamespaceRows,
  filteredKuboardNamespaceRows,
  namespaceSummary,
  workloadTypeFilter,
  workloadTypeOptions,
  kuboardWorkloadRows,
  workloadSummary,
  workloadSelectionCount,
  filteredPods,
  pagedPods,
  filteredWorkloads,
  filteredServices,
  filteredIngresses,
  filteredGatewayApiGateways,
  filteredHTTPRoutes,
  filteredConfigMaps,
  filteredSecrets,
  filteredStorages,
  filteredStorageClasses,
  filteredStorageVolumes,
  yamlDiffLines,
  yamlLineNumbers,
  yamlPreviewLineNumbers,
  yamlChangeSummary,
  yamlCurrentLineOffset,
  trafficTotalWeight,
  podContainerOptions,
  podLogLines,
  sectionTabs,
  clusterStatusText,
  certificateStatusType,
  certificateStatusText,
  certificateRemainText,
  hasItems,
  shouldShowNamespaceFilter,
  handleClusterChange,
  handleTabChange,
  handleNamespaceFilterChange,
  handleResourceKeywordChange,
  handleNamespaceKeywordChange,
  openNamespaceWorkloads,
  openWorkloadPods,
  handleWorkloadTypeChange,
  refreshCurrentClusterData,
  openNodeDetail,
  openNodeLabels,
  addNodeLabel,
  removeNodeLabel,
  saveNodeLabels,
  nodePodPercent,
  podStatusTagType,
  openNamespaceDetail,
  openNamespaceYAML,
  openNamespaceCreate,
  submitNamespaceCreate,
  openConfigStorageCreate,
  openStorageClassCreate,
  submitStorageClassCreate,
  deleteStorageClass,
  deleteStorageVolume,
  addConfigStorageEntry,
  removeConfigStorageEntry,
  submitConfigStorageCreate,
  openWorkloadDetail,
	openWorkloadResourceSettings,
	submitWorkloadResourceSettings,
	addWorkloadEnvironment,
	removeWorkloadEnvironment,
	openWorkloadYAML,
  handleWorkloadSelectionChange,
  supportsScale,
  supportsRestart,
  openImageVersionDialog,
  openScaleDialog,
  handleRestartWorkload,
  submitWorkloadImageVersionUpdate,
  openPodDetail,
  openPodLogs,
  openPodYAML,
  openPodTerminal,
  handlePodPageSizeChange,
  handlePodPageChange,
  handleDeletePod,
  refreshPodLogs,
  openServiceDetail,
  openServiceYAML,
  openIngressDetail,
  openIngressYAML,
  openIstioResourceDetail,
  openIstioResourceYAML,
  openIstioCreateDialog,
  submitIstioCreate,
  handleDeleteIstioResource,
  openTrafficDialog,
  submitTrafficAdjust,
  openConfigMapDetail,
  openConfigMapYAML,
  openConfigMapEdit,
  deleteConfigMap,
  openSecretDetail,
  openSecretYAML,
  openSecretEdit,
  deleteSecret,
  openStorageDetail,
  openStorageYAML,
  translateIstioDetailLabel,
  runYAMLSearch,
  searchYAMLPrev,
  searchYAMLNext,
  handleYAMLInput,
  updateYAMLCurrentLine,
  handleYAMLScroll,
  submitYAMLUpdate,
  submitScale,
  yamlResourceLabel
})

onMounted(async () => {
  await loadClusters()
})

watch(
  () => [currentTab.value, namespaceFilter.value, resourceKeyword.value, workloadTypeFilter.value, workloads.value.length],
  () => {
    selectedWorkloads.value = []
    if (currentTab.value === 'workloads') {
      void hydrateWorkloadImages()
    }
  }
)

watch(filteredPods, () => {
  const maxPage = Math.max(1, Math.ceil(filteredPods.value.length / podPageSize.value))
  if (podPage.value > maxPage) {
    podPage.value = maxPage
  }
})
</script>

<template>
  <div class="k8s-page" :class="`k8s-page--${page.currentTab}`" v-loading="page.loading">
    <K8sConsoleLayout :page="page">
      <K8sSectionContent :page="page" />
    </K8sConsoleLayout>
    <K8sDrawers :page="page" />
    <K8sDialogs :page="page" />
  </div>
</template>
