<script setup>
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRoute, useRouter } from 'vue-router'
import { Connection, Grid, Histogram, Monitor, Promotion, SetUp } from '@element-plus/icons-vue'
import {
  queryK8sClusterList,
  queryK8sClusterOverview,
  queryK8sNamespaceDetail,
  queryK8sNamespaceEvents,
  queryK8sNodeDetail,
  queryK8sNodePods,
  queryK8sPodDetail,
  queryK8sPodContainers,
  queryK8sPodEvents,
  queryK8sPodLogs,
  queryK8sWorkloadDetail,
  queryK8sServiceDetail,
  queryK8sIngressDetail,
  queryK8sConfigMapDetail,
  queryK8sSecretDetail,
  queryK8sStorageDetail,
  scaleK8sWorkload,
  restartK8sWorkload,
  updateK8sResourceYAML
} from '../../api/k8s'

const route = useRoute()
const router = useRouter()
const CLUSTER_KEY = 'ops-admin-k8s-current-cluster'
const NAMESPACE_FILTER_KEY = 'ops-admin-k8s-namespace-filter'

const sectionTabs = [
  { key: 'overview', label: 'Overview', path: '/assets/k8s/overview', icon: Histogram },
  { key: 'nodes', label: 'Nodes', path: '/assets/k8s/nodes', icon: Monitor },
  { key: 'namespaces', label: 'Namespaces', path: '/assets/k8s/namespaces', icon: Grid },
  { key: 'workloads', label: 'Workloads', path: '/assets/k8s/workloads', icon: SetUp },
  { key: 'pods', label: 'Pods', path: '/assets/k8s/pods', icon: Promotion },
  { key: 'services', label: 'Services', path: '/assets/k8s/services', icon: Connection },
  { key: 'ingresses', label: 'Ingress', path: '/assets/k8s/ingresses', icon: Connection },
  { key: 'config-storage', label: 'Config & Storage', path: '/assets/k8s/config-storage', icon: Grid }
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
const configMaps = ref([])
const secrets = ref([])
const storages = ref([])
const namespaceFilter = ref('__all__')
const resourceKeyword = ref('')

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

const namespaceDrawerVisible = ref(false)
const namespaceDrawerLoading = ref(false)
const namespaceDetail = ref(null)
const namespaceEvents = ref([])

const workloadDrawerVisible = ref(false)
const workloadDrawerLoading = ref(false)
const workloadDetail = ref(null)

const podDrawerVisible = ref(false)
const podDrawerLoading = ref(false)
const podDetail = ref(null)
const podEvents = ref([])
const podLogs = ref('')
const selectedContainer = ref('')
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

const scaleDialogVisible = ref(false)
const scaleLoading = ref(false)
const scaleForm = reactive({
  namespace: '',
  workloadType: '',
  workloadName: '',
  replicas: 1
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

function certificateRemainText(daysRemaining) {
  if (typeof daysRemaining !== 'number') return '-'
  if (daysRemaining < 0) return `Expired ${Math.abs(daysRemaining)} days ago`
  if (daysRemaining === 0) return 'Expires today'
  return `${daysRemaining} days remaining`
}

const podContainerOptions = computed(() => {
  if (!podDetail.value?.containers) return []
  return podDetail.value.containers.map((item) => item.name)
})

const namespaceOptions = computed(() => {
  const options = [{ label: 'All namespaces', value: '__all__' }]
  for (const item of namespaces.value || []) {
    options.push({ label: item.name, value: item.name })
  }
  return options
})

const filteredPods = computed(() => filterList(pods.value))
const filteredWorkloads = computed(() => filterList(workloads.value))
const filteredServices = computed(() => filterList(services.value))
const filteredIngresses = computed(() => filterList(ingresses.value))
const filteredConfigMaps = computed(() => filterList(configMaps.value))
const filteredSecrets = computed(() => filterList(secrets.value))
const filteredStorages = computed(() => filterList(storages.value))
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

function hasItems(list) {
  return Array.isArray(list) && list.length > 0
}

function shouldShowNamespaceFilter(tab) {
  return ['pods', 'workloads', 'services', 'ingresses', 'config-storage'].includes(tab)
}

function filterList(list) {
  if (!Array.isArray(list)) return []
  let result = list
  if (namespaceFilter.value !== '__all__') {
    result = result.filter((item) => item.namespace === namespaceFilter.value)
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
  localStorage.setItem(NAMESPACE_FILTER_KEY, namespaceFilter.value)
}

function handleResourceKeywordChange(value) {
  resourceKeyword.value = value || ''
}

function setYAMLEditor(payload) {
  yamlEditor.title = payload.title || 'YAML Editor'
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
    const data = await queryK8sClusterOverview(clusterId)
    cluster.value = data.cluster
    overview.value = data.overview
    nodes.value = data.nodes || []
    namespaces.value = data.namespaces || []
    pods.value = data.pods || []
    workloads.value = data.workloads || []
    services.value = data.network?.services || []
    ingresses.value = data.network?.ingresses || []
    configMaps.value = data.configStorage?.configMaps || []
    secrets.value = data.configStorage?.secrets || []
    storages.value = data.configStorage?.storage || []
    restoreNamespaceFilter()
    localStorage.setItem(CLUSTER_KEY, String(clusterId))
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
    title: 'Edit Namespace YAML - ' + detail.name,
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
    title: 'Edit Workload YAML - ' + detail.name,
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
    ElMessage.success('Workload scaled successfully')
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
  await ElMessageBox.confirm('Restart ' + row.type + ' ' + row.name + '? This will trigger a rollout restart for the selected workload.', 'Confirm Restart', {
    type: 'warning'
  })
  await restartK8sWorkload({
    clusterId: cluster.value.id,
    namespace: row.namespace,
    workloadType: row.type,
    workloadName: row.name
  })
  ElMessage.success('Workload restarted successfully')
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
  podLogs.value = ''
  selectedContainer.value = ''
  currentPodQuery.namespace = row.namespace
  currentPodQuery.podName = row.name
  try {
    const [detail, events] = await Promise.all([
      queryK8sPodDetail(cluster.value.id, row.namespace, row.name),
      queryK8sPodEvents(cluster.value.id, row.namespace, row.name)
    ])
    podDetail.value = detail
    podEvents.value = events || []
    selectedContainer.value = detail.containers?.[0]?.name || ''
    await refreshPodLogs()
  } finally {
    podDrawerLoading.value = false
  }
}

async function openPodYAML(row) {
  if (!cluster.value?.id) return
  const detail = await queryK8sPodDetail(cluster.value.id, row.namespace, row.name)
  setYAMLEditor({
    title: 'Edit Pod YAML - ' + detail.name,
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
    selectedContainer.value
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
    title: 'Edit Service YAML - ' + detail.name,
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
    title: 'Edit Ingress YAML - ' + detail.name,
    resourceType: 'ingress',
    namespace: detail.namespace,
    name: detail.name,
    yaml: detail.yaml
  })
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
    title: 'Edit ConfigMap YAML - ' + detail.name,
    resourceType: 'configmap',
    namespace: detail.namespace,
    name: detail.name,
    yaml: detail.yaml
  })
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
    title: 'Edit Secret YAML - ' + detail.name,
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
    title: 'Edit ' + detail.kind + ' YAML - ' + detail.name,
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
          storageDetail.value.namespace === '-' ? '' : storageDetail.value.namespace,
          storageDetail.value.name
        )
      }
      break
  }
}

async function submitYAMLUpdate() {
  if (!cluster.value?.id) return
  await ElMessageBox.confirm(
    `This will update the current Kubernetes resource YAML. Added lines: ${yamlChangeSummary.value.added}, removed lines: ${yamlChangeSummary.value.removed}. Continue?`,
    'Confirm YAML Update',
    {
      type: 'warning',
      confirmButtonText: 'Confirm',
      cancelButtonText: 'Cancel'
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
    ElMessage.success('YAML updated successfully')
    yamlDialogVisible.value = false
    await refreshCurrentYAMLResource()
  } finally {
    yamlSaving.value = false
  }
}

onMounted(async () => {
  await loadClusters()
})
</script>

<template>
  <div class="k8s-page" v-loading="loading">
    <section class="cluster-header">
      <div class="cluster-toolbar">
        <div class="cluster-selector-block">
          <span class="field-label">Cluster Selector</span>
          <el-select
            :model-value="cluster?.id"
            placeholder="Select cluster"
            size="large"
            filterable
            class="cluster-select"
            :disabled="!clusterOptions.length"
            :loading="switching"
            @update:model-value="handleClusterChange"
          >
            <el-option v-for="item in clusterOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </div>

        <div v-if="cluster" class="cluster-brief">
          <div class="cluster-identity">
            <strong>{{ cluster.name }}</strong>
            <el-tag :type="statusType" effect="light">{{ cluster.statusText }}</el-tag>
          </div>
          <div class="cluster-meta">
            <span class="meta-chip">
              <em>API Server</em>
              <b>{{ cluster.apiServer }}</b>
            </span>
            <span class="meta-chip">
              <em>Version</em>
              <b>{{ cluster.version }}</b>
            </span>
            <span class="meta-chip">
              <em>Nodes</em>
              <b>{{ cluster.nodeCount }} total</b>
            </span>
          </div>
        </div>
      </div>

      <div v-if="!cluster" class="empty-cluster-state">
        <h3>No available K8s cluster</h3>
        <p>Please add and verify a usable cluster in cluster management before returning here to view overview and resource data.</p>
      </div>
    </section>

    <section class="section-tabs">
      <el-tabs :model-value="currentTab" @tab-change="handleTabChange">
        <el-tab-pane v-for="tab in sectionTabs" :key="tab.key" :name="tab.key" :label="tab.label" />
      </el-tabs>
    </section>

    <section v-if="hasCluster && shouldShowNamespaceFilter(currentTab)" class="filter-toolbar">
      <el-select
        :model-value="namespaceFilter"
        filterable
        placeholder="All namespaces"
        class="namespace-select"
        @update:model-value="handleNamespaceFilterChange"
      >
        <el-option
          v-for="item in namespaceOptions"
          :key="item.value"
          :label="item.label"
          :value="item.value"
        />
      </el-select>
      <el-input
        :model-value="resourceKeyword"
        clearable
        placeholder="Search resources"
        class="resource-search"
        @update:model-value="handleResourceKeywordChange"
      />
    </section>

    <section v-if="hasCluster && currentTab === 'overview' && overview" class="section-body">
      <div class="stats-grid">
        <article class="stats-panel">
          <span>Health Score</span>
          <strong>{{ overview.healthScore }}</strong>
          <small>Current alerts {{ overview.alertCount }}</small>
        </article>
        <article class="stats-panel">
          <span>CPU Usage</span>
          <strong>{{ overview.cpuUsage }}</strong>
          <small>Workloads {{ overview.requestRate }}</small>
        </article>
        <article class="stats-panel">
          <span>Memory Usage</span>
          <strong>{{ overview.memoryUsage }}</strong>
          <small>Pod Usage {{ overview.podUsage }}</small>
        </article>
      </div>

      <div class="summary-band">
        <div class="summary-list">
          <div v-for="item in overview.distribution" :key="item.label" class="summary-item">
            <span>{{ item.label }}</span>
            <strong>{{ item.value }}</strong>
          </div>
        </div>
      </div>

      <div v-if="overview.certificates?.length" class="cert-band">
        <div class="cert-band-head">
          <div>
            <strong>Certificates</strong>
            <span>Parsed from the current kubeconfig, including CA and client certificates</span>
          </div>
        </div>
        <div class="cert-grid">
          <article v-for="item in overview.certificates" :key="item.type" class="cert-card">
            <div class="cert-card-head">
              <div class="cert-title">
                <strong>{{ item.name }}</strong>
                <span>{{ item.subject }}</span>
              </div>
              <el-tag size="small" :type="certificateStatusType(item.status)" effect="light">
                {{ item.statusText }}
              </el-tag>
            </div>
            <div class="cert-meta-grid">
              <div class="cert-meta-item">
                <span>Issuer</span>
                <strong>{{ item.issuer }}</strong>
              </div>
              <div class="cert-meta-item">
                <span>Remaining</span>
                <strong>{{ certificateRemainText(item.daysRemaining) }}</strong>
              </div>
              <div class="cert-meta-item">
                <span>Valid From</span>
                <strong>{{ item.notBefore }}</strong>
              </div>
              <div class="cert-meta-item">
                <span>Expires At</span>
                <strong>{{ item.notAfter }}</strong>
              </div>
            </div>
          </article>
        </div>
      </div>
    </section>

    <section v-if="hasCluster && currentTab === 'nodes'" class="section-body">
      <el-table v-if="hasItems(nodes)" :data="nodes" class="data-table">
        <el-table-column prop="name" label="Name" min-width="180" />
        <el-table-column prop="role" label="Role" width="140" />
        <el-table-column prop="status" label="Status" width="120" />
        <el-table-column prop="version" label="Version" width="120" />
        <el-table-column prop="internalIP" label="Internal IP" min-width="150" />
        <el-table-column prop="cpu" label="CPU" width="100" />
        <el-table-column prop="memory" label="Memory" width="120" />
        <el-table-column prop="pods" label="Pod Allocation" width="120" />
        <el-table-column label="Actions" width="120" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openNodeDetail(row)">Detail</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-else description="No realtime node data available" />
    </section>

    <section v-if="hasCluster && currentTab === 'namespaces'" class="section-body">
      <el-table v-if="hasItems(namespaces)" :data="namespaces" class="data-table">
        <el-table-column prop="name" label="Name" min-width="180" />
        <el-table-column prop="status" label="Status" width="120" />
        <el-table-column prop="pods" label="Pods" width="100" />
        <el-table-column prop="services" label="Services" width="100" />
        <el-table-column prop="workloads" label="Workloads" width="120" />
        <el-table-column prop="createdAt" label="Created At" min-width="180" />
        <el-table-column label="Actions" width="180" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openNamespaceDetail(row)">Detail</el-button>
            <el-button link type="primary" @click="openNamespaceYAML(row)">YAML</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-else description="No realtime namespace data available" />
    </section>

    <section v-if="hasCluster && currentTab === 'pods'" class="section-body">
      <el-table v-if="hasItems(filteredPods)" :data="filteredPods" class="data-table">
        <el-table-column prop="name" label="Pod Name" min-width="240" />
        <el-table-column prop="namespace" label="Namespace" width="140" />
        <el-table-column prop="status" label="Status" width="150" />
        <el-table-column prop="node" label="Node" min-width="160" />
        <el-table-column prop="restarts" label="Restarts" width="100" />
        <el-table-column prop="age" label="Age" width="100" />
        <el-table-column prop="ip" label="Pod IP" min-width="120" />
        <el-table-column label="Actions" width="220" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openPodDetail(row)">Detail</el-button>
            <el-button link type="primary" @click="openPodYAML(row)">YAML</el-button>
            <el-button link type="primary" @click="openPodTerminal(row)">Terminal</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-else description="No realtime pod data available" />
    </section>

    <section v-if="hasCluster && currentTab === 'workloads'" class="section-body">
      <el-table v-if="hasItems(filteredWorkloads)" :data="filteredWorkloads" class="data-table">
        <el-table-column prop="name" label="Name" min-width="180" />
        <el-table-column prop="type" label="Type" width="140" />
        <el-table-column prop="namespace" label="Namespace" width="140" />
        <el-table-column prop="ready" label="Ready" width="110" />
        <el-table-column prop="updated" label="Updated" width="100" />
        <el-table-column prop="available" label="Available" width="100" />
        <el-table-column prop="age" label="Age" width="120" />
        <el-table-column label="Actions" min-width="290" fixed="right">
          <template #default="{ row }">
            <div class="action-row">
              <el-button link type="primary" @click="openWorkloadDetail(row)">Detail</el-button>
              <el-button link type="primary" @click="openWorkloadYAML(row)">YAML</el-button>
              <el-button v-if="supportsScale(row)" link type="primary" @click="openScaleDialog(row)">Scale</el-button>
              <el-button v-if="supportsRestart(row)" link type="warning" @click="handleRestartWorkload(row)">Restart</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-else description="No realtime workload data available" />
    </section>

    <section v-if="hasCluster && currentTab === 'services'" class="section-body">
      <el-table v-if="hasItems(filteredServices)" :data="filteredServices" class="data-table">
        <el-table-column prop="name" label="Name" min-width="160" />
        <el-table-column prop="namespace" label="Namespace" width="120" />
        <el-table-column prop="type" label="Type" width="120" />
        <el-table-column prop="clusterIP" label="Cluster IP" min-width="140" />
        <el-table-column prop="ports" label="Ports" min-width="160" />
        <el-table-column prop="endpoints" label="Endpoints" width="110" />
        <el-table-column label="Actions" width="180" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openServiceDetail(row)">Detail</el-button>
            <el-button link type="primary" @click="openServiceYAML(row)">YAML</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-else description="No realtime service data available" />
    </section>

    <section v-if="hasCluster && currentTab === 'ingresses'" class="section-body">
      <el-table v-if="hasItems(filteredIngresses)" :data="filteredIngresses" class="data-table">
        <el-table-column prop="name" label="Name" min-width="160" />
        <el-table-column prop="namespace" label="Namespace" width="120" />
        <el-table-column prop="host" label="Host" min-width="180" />
        <el-table-column prop="address" label="Address" min-width="140" />
        <el-table-column prop="tls" label="TLS" width="120" />
        <el-table-column prop="age" label="Age" width="110" />
        <el-table-column label="Actions" width="180" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openIngressDetail(row)">Detail</el-button>
            <el-button link type="primary" @click="openIngressYAML(row)">YAML</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-else description="No realtime ingress data available" />
    </section>
    <section v-if="hasCluster && currentTab === 'config-storage'" class="section-body config-grid">
      <div class="subsection">
        <div class="subsection-head">
          <strong>ConfigMaps</strong>
        </div>
        <el-table v-if="hasItems(filteredConfigMaps)" :data="filteredConfigMaps" class="data-table">
          <el-table-column prop="name" label="Name" min-width="180" />
          <el-table-column prop="namespace" label="Namespace" width="120" />
          <el-table-column prop="keys" label="Keys" width="100" />
          <el-table-column prop="age" label="Age" width="110" />
          <el-table-column label="Actions" width="180" fixed="right">
            <template #default="{ row }">
              <el-button link type="primary" @click="openConfigMapDetail(row)">Detail</el-button>
              <el-button link type="primary" @click="openConfigMapYAML(row)">YAML</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-else description="No realtime configmap data available" />
      </div>

      <div class="subsection">
        <div class="subsection-head">
          <strong>Secrets</strong>
        </div>
        <el-table v-if="hasItems(filteredSecrets)" :data="filteredSecrets" class="data-table">
          <el-table-column prop="name" label="Name" min-width="180" />
          <el-table-column prop="namespace" label="Namespace" width="120" />
          <el-table-column prop="type" label="Type" min-width="160" />
          <el-table-column prop="age" label="Age" width="110" />
          <el-table-column label="Actions" width="180" fixed="right">
            <template #default="{ row }">
              <el-button link type="primary" @click="openSecretDetail(row)">Detail</el-button>
              <el-button link type="primary" @click="openSecretYAML(row)">YAML</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-else description="No realtime secret data available" />
      </div>

      <div class="subsection">
        <div class="subsection-head">
          <strong>Storage</strong>
        </div>
        <el-table v-if="hasItems(filteredStorages)" :data="filteredStorages" class="data-table">
          <el-table-column prop="name" label="Name" min-width="200" />
          <el-table-column prop="kind" label="Kind" width="140" />
          <el-table-column prop="namespace" label="Namespace" width="120" />
          <el-table-column prop="status" label="Status" width="120" />
          <el-table-column prop="capacity" label="Capacity" width="120" />
          <el-table-column prop="storageClass" label="StorageClass" min-width="140" />
          <el-table-column label="Actions" width="180" fixed="right">
            <template #default="{ row }">
              <el-button link type="primary" @click="openStorageDetail(row)">Detail</el-button>
              <el-button link type="primary" @click="openStorageYAML(row)">YAML</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-else description="No realtime storage data available" />
      </div>
    </section>

    <section v-if="!hasCluster" class="section-body">
      <el-empty description="Please add a usable K8s cluster in cluster management first" />
    </section>

    <el-drawer v-model="nodeDrawerVisible" title="Node Detail" size="56%">
      <div v-loading="nodeDrawerLoading" class="drawer-content">
        <template v-if="nodeDetail">
          <el-descriptions :column="2" border>
            <el-descriptions-item label="Name">{{ nodeDetail.name }}</el-descriptions-item>
            <el-descriptions-item label="Status">{{ nodeDetail.status }}</el-descriptions-item>
            <el-descriptions-item label="Roles">{{ nodeDetail.roles }}</el-descriptions-item>
            <el-descriptions-item label="Version">{{ nodeDetail.version }}</el-descriptions-item>
            <el-descriptions-item label="Internal IP">{{ nodeDetail.internalIP }}</el-descriptions-item>
            <el-descriptions-item label="Architecture">{{ nodeDetail.architecture }}</el-descriptions-item>
            <el-descriptions-item label="OS">{{ nodeDetail.os }}</el-descriptions-item>
            <el-descriptions-item label="Kernel">{{ nodeDetail.kernel }}</el-descriptions-item>
            <el-descriptions-item label="Container Runtime">{{ nodeDetail.containerRuntime }}</el-descriptions-item>
            <el-descriptions-item label="CPU">{{ nodeDetail.allocatableCPU }} / {{ nodeDetail.capacityCPU }}</el-descriptions-item>
            <el-descriptions-item label="Memory">{{ nodeDetail.allocatableMemory }} / {{ nodeDetail.capacityMemory }}</el-descriptions-item>
          </el-descriptions>

          <div class="drawer-section">
            <strong>Node Labels</strong>
            <div class="tag-group">
              <el-tag v-for="(value, key) in nodeDetail.labels || {}" :key="key" class="info-tag" effect="plain">
                {{ key }}={{ value }}
              </el-tag>
            </div>
          </div>

          <div class="drawer-section">
            <strong>Pods on Node</strong>
            <el-table :data="nodePods" class="data-table">
              <el-table-column prop="name" label="Pod" min-width="220" />
              <el-table-column prop="namespace" label="Namespace" width="140" />
              <el-table-column prop="status" label="Status" width="120" />
              <el-table-column prop="restarts" label="Restarts" width="90" />
              <el-table-column prop="age" label="Age" width="100" />
            </el-table>
          </div>
        </template>
      </div>
    </el-drawer>

    <el-drawer v-model="namespaceDrawerVisible" title="Namespace Detail" size="56%">
      <div v-loading="namespaceDrawerLoading" class="drawer-content">
        <template v-if="namespaceDetail">
          <el-descriptions :column="2" border>
            <el-descriptions-item label="Namespace">{{ namespaceDetail.name }}</el-descriptions-item>
            <el-descriptions-item label="Status">{{ namespaceDetail.status }}</el-descriptions-item>
            <el-descriptions-item label="Created At">{{ namespaceDetail.createdAt }}</el-descriptions-item>
            <el-descriptions-item label="Pods">{{ namespaceDetail.pods }}</el-descriptions-item>
            <el-descriptions-item label="Services">{{ namespaceDetail.services }}</el-descriptions-item>
            <el-descriptions-item label="Workloads">{{ namespaceDetail.workloads }}</el-descriptions-item>
            <el-descriptions-item label="ConfigMaps">{{ namespaceDetail.configMaps }}</el-descriptions-item>
            <el-descriptions-item label="Secrets">{{ namespaceDetail.secrets }}</el-descriptions-item>
            <el-descriptions-item label="Storage">{{ namespaceDetail.storage }}</el-descriptions-item>
          </el-descriptions>

          <div class="drawer-section">
            <strong>Labels</strong>
            <div class="tag-group">
              <el-tag v-for="(value, key) in namespaceDetail.labels || {}" :key="key" class="info-tag" effect="plain">
                {{ key }}={{ value }}
              </el-tag>
            </div>
          </div>

          <div class="drawer-section" v-if="hasItems(namespaceEvents)">
            <strong>Events</strong>
            <el-table :data="namespaceEvents" class="data-table">
              <el-table-column prop="type" label="Type" width="100" />
              <el-table-column prop="reason" label="Reason" width="140" />
              <el-table-column prop="message" label="Message" min-width="280" />
              <el-table-column prop="count" label="Count" width="80" />
              <el-table-column prop="lastTime" label="Last Time" width="160" />
            </el-table>
          </div>

        </template>
      </div>
    </el-drawer>

    <el-drawer v-model="workloadDrawerVisible" title="Workload Detail" size="62%">
      <div v-loading="workloadDrawerLoading" class="drawer-content">
        <template v-if="workloadDetail">
          <el-descriptions :column="2" border>
            <el-descriptions-item label="Name">{{ workloadDetail.name }}</el-descriptions-item>
            <el-descriptions-item label="Type">{{ workloadDetail.type }}</el-descriptions-item>
            <el-descriptions-item label="Namespace">{{ workloadDetail.namespace }}</el-descriptions-item>
            <el-descriptions-item label="Ready">{{ workloadDetail.ready }}</el-descriptions-item>
            <el-descriptions-item label="Updated">{{ workloadDetail.updated }}</el-descriptions-item>
            <el-descriptions-item label="Available">{{ workloadDetail.available }}</el-descriptions-item>
            <el-descriptions-item label="Age">{{ workloadDetail.age }}</el-descriptions-item>
          </el-descriptions>

          <div class="drawer-section">
            <strong>Selector</strong>
            <div class="tag-group">
              <el-tag v-for="(value, key) in workloadDetail.selector || {}" :key="key" class="info-tag" effect="plain">
                {{ key }}={{ value }}
              </el-tag>
            </div>
          </div>

          <div class="drawer-section">
            <strong>Labels</strong>
            <div class="tag-group">
              <el-tag v-for="(value, key) in workloadDetail.labels || {}" :key="key" class="info-tag" effect="plain">
                {{ key }}={{ value }}
              </el-tag>
            </div>
          </div>

          <div class="drawer-section">
            <strong>Containers</strong>
            <el-table :data="workloadDetail.containers || []" class="data-table">
              <el-table-column prop="name" label="Container" min-width="180" />
              <el-table-column prop="image" label="Image" min-width="280" />
            </el-table>
          </div>

          <div class="drawer-section">
            <strong>Related Pods</strong>
            <el-table :data="workloadDetail.pods || []" class="data-table">
              <el-table-column prop="name" label="Pod Name" min-width="220" />
              <el-table-column prop="status" label="Status" width="120" />
              <el-table-column prop="node" label="Node" min-width="150" />
              <el-table-column prop="restarts" label="Restarts" width="100" />
              <el-table-column prop="age" label="Age" width="100" />
            </el-table>
          </div>

        </template>
      </div>
    </el-drawer>

    <el-drawer v-model="serviceDrawerVisible" title="Service Detail" size="58%">
      <div v-loading="serviceDrawerLoading" class="drawer-content">
        <template v-if="serviceDetail">
          <el-descriptions :column="2" border>
            <el-descriptions-item label="Name">{{ serviceDetail.name }}</el-descriptions-item>
            <el-descriptions-item label="Type">{{ serviceDetail.type }}</el-descriptions-item>
            <el-descriptions-item label="Namespace">{{ serviceDetail.namespace }}</el-descriptions-item>
            <el-descriptions-item label="Cluster IP">{{ serviceDetail.clusterIP }}</el-descriptions-item>
            <el-descriptions-item label="Endpoints">{{ serviceDetail.endpoints }}</el-descriptions-item>
            <el-descriptions-item label="Age">{{ serviceDetail.age }}</el-descriptions-item>
          </el-descriptions>

          <div class="drawer-section">
            <strong>Selector</strong>
            <div class="tag-group">
              <el-tag v-for="(value, key) in serviceDetail.selector || {}" :key="key" class="info-tag" effect="plain">
                {{ key }}={{ value }}
              </el-tag>
            </div>
          </div>

          <div class="drawer-section">
            <strong>Ports</strong>
            <el-table :data="serviceDetail.ports || []" class="data-table">
              <el-table-column prop="label" label="Name" min-width="160" />
              <el-table-column prop="value" label="Target" min-width="220" />
            </el-table>
          </div>

        </template>
      </div>
    </el-drawer>

    <el-drawer v-model="ingressDrawerVisible" title="Ingress Detail" size="58%">
      <div v-loading="ingressDrawerLoading" class="drawer-content">
        <template v-if="ingressDetail">
          <el-descriptions :column="2" border>
            <el-descriptions-item label="Name">{{ ingressDetail.name }}</el-descriptions-item>
            <el-descriptions-item label="Namespace">{{ ingressDetail.namespace }}</el-descriptions-item>
            <el-descriptions-item label="Host">{{ ingressDetail.host }}</el-descriptions-item>
            <el-descriptions-item label="Address">{{ ingressDetail.address }}</el-descriptions-item>
            <el-descriptions-item label="TLS">{{ ingressDetail.tls }}</el-descriptions-item>
            <el-descriptions-item label="IngressClass">{{ ingressDetail.className }}</el-descriptions-item>
            <el-descriptions-item label="Age">{{ ingressDetail.age }}</el-descriptions-item>
          </el-descriptions>

          <div class="drawer-section">
            <strong>Rules</strong>
            <el-table :data="ingressDetail.rules || []" class="data-table">
              <el-table-column prop="label" label="Rule" min-width="240" />
              <el-table-column prop="value" label="Backend" min-width="220" />
            </el-table>
          </div>

        </template>
      </div>
    </el-drawer>

    <el-drawer v-model="configMapDrawerVisible" title="ConfigMap Detail" size="58%">
      <div v-loading="configMapDrawerLoading" class="drawer-content">
        <template v-if="configMapDetail">
          <el-descriptions :column="2" border>
            <el-descriptions-item label="Name">{{ configMapDetail.name }}</el-descriptions-item>
            <el-descriptions-item label="Namespace">{{ configMapDetail.namespace }}</el-descriptions-item>
            <el-descriptions-item label="Age">{{ configMapDetail.age }}</el-descriptions-item>
          </el-descriptions>

          <div class="drawer-section">
            <strong>Keys</strong>
            <el-table :data="configMapDetail.keys || []" class="data-table">
              <el-table-column prop="label" label="Key" min-width="220" />
              <el-table-column prop="value" label="Size" min-width="120" />
            </el-table>
          </div>

        </template>
      </div>
    </el-drawer>

    <el-drawer v-model="secretDrawerVisible" title="Secret Detail" size="58%">
      <div v-loading="secretDrawerLoading" class="drawer-content">
        <template v-if="secretDetail">
          <el-descriptions :column="2" border>
            <el-descriptions-item label="Name">{{ secretDetail.name }}</el-descriptions-item>
            <el-descriptions-item label="Namespace">{{ secretDetail.namespace }}</el-descriptions-item>
            <el-descriptions-item label="Type">{{ secretDetail.type }}</el-descriptions-item>
            <el-descriptions-item label="Age">{{ secretDetail.age }}</el-descriptions-item>
          </el-descriptions>

          <div class="drawer-section">
            <strong>Keys</strong>
            <el-table :data="secretDetail.keys || []" class="data-table">
              <el-table-column prop="label" label="Key" min-width="220" />
              <el-table-column prop="value" label="Status" min-width="120" />
            </el-table>
          </div>

        </template>
      </div>
    </el-drawer>

    <el-drawer v-model="storageDrawerVisible" title="Storage Detail" size="58%">
      <div v-loading="storageDrawerLoading" class="drawer-content">
        <template v-if="storageDetail">
          <el-descriptions :column="2" border>
            <el-descriptions-item label="Name">{{ storageDetail.name }}</el-descriptions-item>
            <el-descriptions-item label="Kind">{{ storageDetail.kind }}</el-descriptions-item>
            <el-descriptions-item label="Namespace">{{ storageDetail.namespace }}</el-descriptions-item>
            <el-descriptions-item label="Status">{{ storageDetail.status }}</el-descriptions-item>
            <el-descriptions-item label="Capacity">{{ storageDetail.capacity }}</el-descriptions-item>
            <el-descriptions-item label="StorageClass">{{ storageDetail.storageClass }}</el-descriptions-item>
            <el-descriptions-item label="Age">{{ storageDetail.age }}</el-descriptions-item>
          </el-descriptions>

        </template>
      </div>
    </el-drawer>

    <el-drawer v-model="podDrawerVisible" title="Pod Detail" size="60%">
      <div v-loading="podDrawerLoading" class="drawer-content">
        <template v-if="podDetail">
          <el-descriptions :column="2" border>
            <el-descriptions-item label="Pod Name">{{ podDetail.name }}</el-descriptions-item>
            <el-descriptions-item label="Status">{{ podDetail.status }}</el-descriptions-item>
            <el-descriptions-item label="Namespace">{{ podDetail.namespace }}</el-descriptions-item>
            <el-descriptions-item label="Node">{{ podDetail.node }}</el-descriptions-item>
            <el-descriptions-item label="Pod IP">{{ podDetail.podIP }}</el-descriptions-item>
            <el-descriptions-item label="Host IP">{{ podDetail.hostIP }}</el-descriptions-item>
            <el-descriptions-item label="QoSClass">{{ podDetail.qosClass }}</el-descriptions-item>
            <el-descriptions-item label="ServiceAccount">{{ podDetail.serviceAccount }}</el-descriptions-item>
            <el-descriptions-item label="Created At">{{ podDetail.createdAt }}</el-descriptions-item>
          </el-descriptions>

          <div class="drawer-section">
            <strong>Containers</strong>
            <el-table :data="podDetail.containers || []" class="data-table">
              <el-table-column prop="name" label="Container" min-width="160" />
              <el-table-column prop="image" label="Image" min-width="240" />
              <el-table-column label="Ready" width="100">
                <template #default="{ row }">
                  <el-tag :type="row.ready ? 'success' : 'warning'" effect="light">
                    {{ row.ready ? 'Ready' : 'Not Ready' }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="restart" label="Restarts" width="100" />
            </el-table>
          </div>

          <div class="drawer-section">
            <strong>Labels</strong>
            <div class="tag-group">
              <el-tag v-for="(value, key) in podDetail.labels || {}" :key="key" class="info-tag" effect="plain">
                {{ key }}={{ value }}
              </el-tag>
            </div>
          </div>

          <div class="drawer-section">
            <div class="section-head">
              <strong>Logs</strong>
              <el-select
                v-if="podContainerOptions.length"
                v-model="selectedContainer"
                placeholder="Select container"
                class="container-select"
                @change="refreshPodLogs"
              >
                <el-option v-for="item in podContainerOptions" :key="item" :label="item" :value="item" />
              </el-select>
            </div>
            <pre class="log-panel">{{ podLogs || 'No logs available' }}</pre>
          </div>

          <div class="drawer-section">
            <strong>Events</strong>
            <el-table :data="podEvents" class="data-table">
              <el-table-column prop="type" label="Type" width="100" />
              <el-table-column prop="reason" label="Reason" width="140" />
              <el-table-column prop="message" label="Message" min-width="280" />
              <el-table-column prop="count" label="Count" width="80" />
              <el-table-column prop="lastTime" label="Last Time" width="160" />
            </el-table>
          </div>

        </template>
      </div>
    </el-drawer>

    <el-dialog v-model="yamlDialogVisible" :title="yamlEditor.title" width="1180px" class="yaml-editor-dialog">
      <div class="yaml-workspace">
        <section class="yaml-pane editor">
          <div class="yaml-pane-head">
            <strong>YAML Editor</strong>
            <span>Total {{ yamlEditor.yaml.split('\n').length }} lines</span>
          </div>
          <div class="yaml-search-bar">
            <el-input
              v-model="yamlSearch.keyword"
              placeholder="Search YAML"
              clearable
              @input="runYAMLSearch(false)"
              @clear="runYAMLSearch(false)"
            />
            <span class="yaml-search-summary">
              {{ yamlSearch.matches.length ? `${yamlSearch.activeIndex + 1}/${yamlSearch.matches.length}` : '0/0' }}
            </span>
            <el-button :disabled="!yamlSearch.matches.length" @click="searchYAMLPrev">Previous</el-button>
            <el-button :disabled="!yamlSearch.matches.length" @click="searchYAMLNext">Next</el-button>
          </div>
          <div class="yaml-editor-shell">
            <div class="yaml-line-gutter">
              <div class="yaml-line-gutter-inner" :style="{ transform: `translateY(-${yamlEditorScrollTop}px)` }">
                <div
                  v-for="line in yamlLineNumbers"
                  :key="line"
                  :class="['yaml-line-number', { active: line === yamlCurrentLine }]"
                >
                  {{ line }}
                </div>
              </div>
            </div>
            <div class="yaml-editor-stage">
              <div class="yaml-current-line" :style="{ top: yamlCurrentLineOffset }"></div>
              <textarea
                ref="yamlTextareaRef"
                v-model="yamlEditor.yaml"
                class="yaml-native-textarea"
                spellcheck="false"
                placeholder="Edit YAML here"
                @input="handleYAMLInput"
                @click="updateYAMLCurrentLine"
                @keyup="updateYAMLCurrentLine"
                @mouseup="updateYAMLCurrentLine"
                @scroll="handleYAMLScroll"
              ></textarea>
            </div>
          </div>
        </section>
        <section class="yaml-pane preview">
          <div class="yaml-diff-head">
            <strong>Diff Preview</strong>
            <span>+{{ yamlChangeSummary.added }} / -{{ yamlChangeSummary.removed }}</span>
          </div>
          <div class="yaml-preview-toolbar">
            <span class="yaml-preview-hint">
              {{ yamlChangeSummary.changed ? 'Changed lines are highlighted below' : 'No changes yet' }}
            </span>
          </div>
          <div class="yaml-preview-shell">
            <div class="yaml-line-gutter preview">
              <div class="yaml-line-gutter-inner">
                <div
                  v-for="line in yamlPreviewLineNumbers"
                  :key="`preview-${line}`"
                  class="yaml-line-number"
                >
                  {{ line }}
                </div>
              </div>
            </div>
            <div class="yaml-diff-panel">
              <div
                v-for="(item, index) in yamlDiffLines"
                :key="`${index}-${item.type}`"
                :class="['yaml-diff-line', item.type]"
              >
                <span class="marker">
                  {{ item.type === 'added' ? '+' : item.type === 'removed' ? '-' : ' ' }}
                </span>
                <code>{{ item.text || ' ' }}</code>
              </div>
            </div>
          </div>
        </section>
      </div>
      <template #footer>
        <el-button @click="yamlDialogVisible = false">Cancel</el-button>
        <el-button type="primary" :loading="yamlSaving" @click="submitYAMLUpdate">Save</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="scaleDialogVisible" title="Scale Workload" width="420px">
      <el-form label-width="90px">
        <el-form-item label="Namespace">
          <el-input :model-value="scaleForm.namespace" readonly />
        </el-form-item>
        <el-form-item label="Workload">
          <el-input :model-value="`${scaleForm.workloadType} / ${scaleForm.workloadName}`" readonly />
        </el-form-item>
        <el-form-item label="Replicas">
          <el-input-number v-model="scaleForm.replicas" :min="0" :max="999" controls-position="right" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="scaleDialogVisible = false">Cancel</el-button>
        <el-button type="primary" :loading="scaleLoading" @click="submitScale">Save</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.k8s-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.cluster-header {
  padding: 18px 20px;
  border: 1px solid #e5ebf5;
  border-radius: 8px;
  background: #fff;
}

.cluster-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
}

.cluster-selector-block {
  width: 320px;
}

.field-label {
  display: block;
  margin-bottom: 6px;
  color: #7b8798;
  font-size: 12px;
}

.cluster-select {
  width: 100%;
}

.cluster-brief {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
}

.cluster-identity {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.cluster-identity strong {
  font-size: 24px;
  color: #0f172a;
  line-height: 1.2;
}

.cluster-meta {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 10px;
}

.meta-chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  max-width: 100%;
  padding: 10px 12px;
  border: 1px solid #e8edf5;
  border-radius: 8px;
  background: #f8fafc;
  color: #4b5565;
  font-size: 13px;
}

.meta-chip em {
  font-style: normal;
  color: #7b8798;
  white-space: nowrap;
}

.meta-chip b {
  color: #111827;
  font-weight: 600;
  word-break: break-all;
}

.empty-cluster-state {
  margin-top: 16px;
  padding: 22px 16px;
  border: 1px dashed #c9d6ea;
  border-radius: 8px;
  background: #fafcff;
  text-align: center;
}

.empty-cluster-state h3 {
  margin: 0 0 8px;
  color: #0f172a;
  font-size: 18px;
}

.empty-cluster-state p {
  margin: 0;
  color: #6b7a93;
}

.section-tabs {
  padding: 0 2px;
}

.filter-toolbar {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 12px;
}

.namespace-select {
  width: 180px;
}

.resource-search {
  width: 240px;
}

.section-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
}

.stats-panel {
  padding: 16px;
  border: 1px solid #e1e8f5;
  border-radius: 8px;
  background: #fff;
}

.stats-panel span {
  display: block;
  color: #6b7a93;
  font-size: 13px;
}

.stats-panel strong {
  display: block;
  margin-top: 10px;
  color: #0f172a;
  font-size: 20px;
  line-height: 1.4;
}

.stats-panel small {
  display: block;
  margin-top: 10px;
  color: #7a889f;
}

.summary-band {
  padding: 18px 20px;
  border: 1px solid #e1e8f5;
  border-radius: 8px;
  background: #fff;
}

.summary-list {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
}

.summary-item {
  padding: 14px 16px;
  border-radius: 8px;
  background: #f7f9fd;
}

.summary-item span {
  display: block;
  color: #6b7a93;
  font-size: 13px;
}

.summary-item strong {
  display: block;
  margin-top: 8px;
  color: #10213e;
  font-size: 18px;
}

.cert-band {
  padding: 18px 20px;
  border: 1px solid #e1e8f5;
  border-radius: 8px;
  background: #fff;
}

.cert-band-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 14px;
}

.cert-band-head strong {
  display: block;
  color: #0f172a;
  font-size: 16px;
}

.cert-band-head span {
  display: block;
  margin-top: 4px;
  color: #7a889f;
  font-size: 13px;
}

.cert-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.cert-card {
  padding: 16px;
  border: 1px solid #e7edf7;
  border-radius: 8px;
  background: #f8faff;
}

.cert-card-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.cert-title {
  min-width: 0;
}

.cert-title strong {
  display: block;
  color: #10213e;
  font-size: 15px;
  line-height: 1.4;
}

.cert-title span {
  display: block;
  margin-top: 6px;
  color: #60708a;
  font-size: 13px;
  line-height: 1.5;
  word-break: break-word;
}

.cert-meta-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px 16px;
  margin-top: 14px;
}

.cert-meta-item {
  min-width: 0;
}

.cert-meta-item span {
  display: block;
  color: #7a889f;
  font-size: 12px;
}

.cert-meta-item strong {
  display: block;
  margin-top: 6px;
  color: #10213e;
  font-size: 13px;
  line-height: 1.5;
  font-weight: 600;
  word-break: break-word;
}

.network-grid,
.config-grid {
  gap: 18px;
}

.subsection,
.drawer-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.subsection-head strong,
.drawer-section strong {
  color: #0f172a;
  font-size: 16px;
}

.data-table {
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.06);
}

.drawer-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.tag-group {
  display: flex;
  flex-wrap: wrap;
}

.info-tag {
  margin-right: 8px;
  margin-bottom: 8px;
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.container-select {
  width: 220px;
}

.action-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.log-panel,
.yaml-panel {
  margin: 0;
  padding: 14px;
  min-height: 220px;
  max-height: 360px;
  overflow: auto;
  border: 1px solid #e5ebf5;
  border-radius: 8px;
  background: #0f172a;
  color: #e2e8f0;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}

.yaml-editor-dialog :deep(.el-dialog) {
  width: 80vw;
  max-width: 1440px;
}

.yaml-editor-dialog :deep(.el-dialog__body) {
  padding-top: 14px;
}

.yaml-workspace {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
  min-height: 680px;
}

.yaml-pane {
  display: flex;
  flex-direction: column;
  min-width: 0;
  border: 1px solid #e5ebf5;
  border-radius: 8px;
  overflow: hidden;
  background: #fff;
}

.yaml-pane-head,
.yaml-diff-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  background: #f8fafc;
  color: #475569;
  font-size: 13px;
  border-bottom: 1px solid #e5ebf5;
}

.yaml-pane.editor :deep(.el-textarea) {
  flex: 1;
}

.yaml-search-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px;
  border-bottom: 1px solid #e5ebf5;
  background: #fff;
}

.yaml-search-bar :deep(.el-input) {
  flex: 1;
}

.yaml-search-bar :deep(.el-input__wrapper) {
  border-radius: 6px;
  box-shadow: 0 0 0 1px #d9e2ef inset;
}

.yaml-search-summary {
  min-width: 48px;
  color: #64748b;
  font-size: 12px;
  text-align: center;
}

.yaml-preview-toolbar {
  display: flex;
  align-items: center;
  min-height: 57px;
  padding: 12px;
  border-bottom: 1px solid #e5ebf5;
  background: #fff;
}

.yaml-preview-hint {
  color: #64748b;
  font-size: 12px;
}

.yaml-editor-shell {
  display: grid;
  grid-template-columns: 64px minmax(0, 1fr);
  flex: 1;
  min-height: 620px;
  background: #0f172a;
}

.yaml-preview-shell {
  display: grid;
  grid-template-columns: 64px minmax(0, 1fr);
  flex: 1;
  min-height: 620px;
  background: #0f172a;
}

.yaml-line-gutter {
  overflow: hidden;
  padding: 14px 0;
  border-right: 1px solid rgba(148, 163, 184, 0.16);
  background: #111c2f;
}

.yaml-line-gutter.preview {
  background: #101a2c;
}

.yaml-line-gutter-inner {
  will-change: transform;
}

.yaml-line-number {
  height: 20px;
  padding: 0 12px 0 0;
  color: #64748b;
  font-family: Consolas, 'Courier New', monospace;
  font-size: 12px;
  line-height: 20px;
  text-align: right;
}

.yaml-line-number.active {
  color: #f8fafc;
  background: rgba(59, 130, 246, 0.22);
}

.yaml-editor-stage {
  position: relative;
  overflow: hidden;
}

.yaml-current-line {
  position: absolute;
  left: 0;
  right: 0;
  height: 20px;
  background: rgba(59, 130, 246, 0.16);
  pointer-events: none;
  z-index: 0;
}

.yaml-native-textarea {
  position: relative;
  z-index: 1;
  width: 100%;
  height: 100%;
  min-height: 620px;
  padding: 14px 16px;
  border: 0;
  outline: none;
  resize: none;
  background: transparent;
  color: #e2e8f0;
  font-family: Consolas, 'Courier New', monospace;
  font-size: 12px;
  line-height: 20px;
  white-space: pre;
  overflow: auto;
  tab-size: 2;
}

.yaml-diff-panel {
  flex: 1;
  overflow: auto;
  background: #0f172a;
  min-height: 620px;
  padding: 14px 0;
}

.yaml-diff-line {
  display: grid;
  grid-template-columns: 24px minmax(0, 1fr);
  gap: 8px;
  padding: 0 14px;
  min-height: 20px;
  color: #e2e8f0;
  font-family: Consolas, 'Courier New', monospace;
  font-size: 12px;
  line-height: 20px;
}

.yaml-diff-line code {
  background: transparent;
  color: inherit;
  white-space: pre;
  word-break: normal;
  overflow-wrap: normal;
}

.yaml-diff-line.added {
  background: rgba(34, 197, 94, 0.14);
}

.yaml-diff-line.removed {
  background: rgba(239, 68, 68, 0.14);
}

.yaml-diff-line .marker {
  color: #94a3b8;
}

@media (max-width: 1100px) {
  .cluster-toolbar {
    flex-direction: column;
    align-items: stretch;
  }

  .cluster-selector-block {
    width: 100%;
  }

  .cluster-brief {
    flex-direction: column;
    align-items: flex-start;
  }

  .cluster-meta {
    justify-content: flex-start;
  }

  .stats-grid,
  .summary-list,
  .cert-grid,
  .cert-meta-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .yaml-workspace {
    grid-template-columns: 1fr;
  }

  .yaml-editor-dialog :deep(.el-dialog) {
    width: calc(100vw - 32px);
  }
}

@media (max-width: 720px) {
  .stats-grid,
  .summary-list,
  .cert-grid,
  .cert-meta-grid {
    grid-template-columns: 1fr;
  }

  .cluster-identity {
    flex-wrap: wrap;
  }

  .namespace-select {
    width: 100%;
  }

  .resource-search {
    width: 100%;
  }

  .meta-chip {
    width: 100%;
    justify-content: space-between;
  }

  .section-head {
    flex-direction: column;
    align-items: stretch;
  }

  .container-select {
    width: 100%;
  }

  .action-row {
    flex-wrap: wrap;
    gap: 6px 10px;
  }
}
</style>
