import { computed, reactive, ref, watch } from 'vue'
import { queryK8sWorkloadDetail } from '../api/k8s'
import { kt } from '../utils/k8s-extra-i18n'
import { NAMESPACE_FILTER_KEY } from './useK8sClusterState'

// I5-H — extracted from views/assets/K8s.vue (behavior preservation + rewiring
// contract, i5-plan §3.8). Original coordinates per block:
//   74-82 filter state · 131-132 pod paging · 81-82 workload image cache
//   349-355·357-423 namespace/pod options·filtered projections·yaml is elsewhere
//   477-543 kuboard namespace/workload rows·summaries · 546-652 filter handlers
//   654-658 handleWorkloadTypeChange stays in K8s.vue (cross-domain selection)
//   1508-1515 pod paging handlers · 1993-2061 workload helpers·image hydration
//   2766-2771 paged-pod clamp watch
// Judgment (r1): kuboardWorkloadRows·hydrateWorkloadImages are projections of
// filteredWorkloads·workloadTypeFilter, so they ride with the filters domain to
// keep the plan's named composable set (useK8sFilters = 필터+필터 파생 보드).

// Workload capability predicates (원본 K8s.vue:1067-1073) — module-level single
// source: workloadSummary here and the batch operations in useK8sResourceActions
// both consume them.
export function supportsScale(row) {
  return ['Deployment', 'StatefulSet'].includes(row.type)
}

export function supportsRestart(row) {
  return ['Deployment', 'StatefulSet', 'DaemonSet'].includes(row.type)
}

export function useK8sFilters({
  cluster,
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
  onTabChange
}) {
  const namespaceFilter = ref('__all__')
  const resourceKeyword = ref('')
  const namespaceKeyword = ref('')
  const workloadTypeFilter = ref('all')
  const podWorkloadFilter = ref('__all__')
  const podScopedNames = ref([])
  const podPage = ref(1)
  const podPageSize = ref(20)
  const workloadImageMap = reactive({})
  const workloadImageLoadingMap = reactive({})

  const namespaceOptions = computed(() => {
    const options = [{ label: kt('k8sAllNamespaces'), value: '__all__' }]
    for (const item of namespaces.value || []) {
      options.push({ label: item.name, value: item.name })
    }
    return options
  })

  function podWorkloadFilterValue(pod) {
    const name = String(pod?.workloadName || '').trim()
    if (!name) return '__standalone__'
    const type = String(pod?.workloadType || 'Workload').trim()
    return `${type}/${name}`
  }

  const podWorkloadOptions = computed(() => {
    const grouped = new Map()
    for (const pod of pods.value || []) {
      if (namespaceFilter.value !== '__all__' && pod.namespace !== namespaceFilter.value) continue
      const value = podWorkloadFilterValue(pod)
      const item = grouped.get(value) || {
        value,
        label: pod.workloadName || kt('standalonePod'),
        count: 0
      }
      item.count += 1
      grouped.set(value, item)
    }
    return [
      { value: '__all__', label: kt('allWorkloads') },
      ...Array.from(grouped.values())
        .sort((left, right) => left.label.localeCompare(right.label))
        .map((item) => ({ ...item, label: `${item.label}(${item.count})` }))
    ]
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
    return [{ value: 'all', label: kt('k8sWorkloadTypeAll'), count: filteredWorkloads.value.length }, ...dynamic]
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
    if (list === pods.value && podWorkloadFilter.value !== '__all__') {
      result = result.filter((item) => podWorkloadFilterValue(item) === podWorkloadFilter.value)
    }
    const keyword = resourceKeyword.value.trim().toLowerCase()
    if (!keyword) return result
    return result.filter((item) => {
      const values = [
        item.name,
        item.namespace,
        item.workloadName,
        item.workloadType,
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

  // Load-hook impls — invoked by useK8sClusterState's load orchestration.
  function resetNamespaceFilter() {
    namespaceFilter.value = '__all__'
  }

  function clearWorkloadImageCache() {
    Object.keys(workloadImageMap).forEach((key) => delete workloadImageMap[key])
  }

  function handleNamespaceFilterChange(value) {
    namespaceFilter.value = value || '__all__'
    podWorkloadFilter.value = '__all__'
    podScopedNames.value = []
    podPage.value = 1
    localStorage.setItem(NAMESPACE_FILTER_KEY, namespaceFilter.value)
  }

  function handlePodWorkloadFilterChange(value) {
    podWorkloadFilter.value = value || '__all__'
    podScopedNames.value = []
    podPage.value = 1
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
    podWorkloadFilter.value = '__all__'
    podScopedNames.value = []
    localStorage.setItem(NAMESPACE_FILTER_KEY, namespaceFilter.value)
    onTabChange('workloads')
  }

  async function openWorkloadPods(row) {
    if (!cluster.value?.id || !row?.name) return
    namespaceFilter.value = row.namespace || '__all__'
    localStorage.setItem(NAMESPACE_FILTER_KEY, namespaceFilter.value)
    const workloadFilter = podWorkloadFilterValue({ workloadName: row.name, workloadType: row.type })
    const hasResolvedWorkload = pods.value.some((pod) => podWorkloadFilterValue(pod) === workloadFilter)
    podWorkloadFilter.value = hasResolvedWorkload ? workloadFilter : '__all__'
    podScopedNames.value = []
    podPage.value = 1
    resourceKeyword.value = ''
    onTabChange('pods')
    try {
      const detail = await queryK8sWorkloadDetail(cluster.value.id, row.namespace, row.type, row.name)
      const relatedPods = Array.isArray(detail?.pods) ? detail.pods.map((item) => item.name).filter(Boolean) : []
      if (relatedPods.length) {
        podScopedNames.value = relatedPods
      }
    } catch (error) {
      console.warn('Failed to load workload pods', error)
    }
  }

  function handlePodPageSizeChange(size) {
    podPageSize.value = size
    podPage.value = 1
  }

  function handlePodPageChange(page) {
    podPage.value = page
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
    if (desiredCount > 0 && readyCount >= desiredCount) return kt('k8sStatusRunning')
    if (readyCount > 0) return kt('k8sStatusWarning')
    return kt('k8sStatusOffline')
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

  watch(filteredPods, () => {
    const maxPage = Math.max(1, Math.ceil(filteredPods.value.length / podPageSize.value))
    if (podPage.value > maxPage) {
      podPage.value = maxPage
    }
  })

  return {
    namespaceFilter,
    resourceKeyword,
    namespaceKeyword,
    workloadTypeFilter,
    podWorkloadFilter,
    podScopedNames,
    podPage,
    podPageSize,
    namespaceOptions,
    podWorkloadOptions,
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
    kuboardNamespaceRows,
    filteredKuboardNamespaceRows,
    namespaceSummary,
    workloadTypeOptions,
    kuboardWorkloadRows,
    workloadSummary,
    hasItems,
    shouldShowNamespaceFilter,
    restoreNamespaceFilter,
    resetNamespaceFilter,
    clearWorkloadImageCache,
    handleNamespaceFilterChange,
    handlePodWorkloadFilterChange,
    handleResourceKeywordChange,
    handleNamespaceKeywordChange,
    openNamespaceWorkloads,
    openWorkloadPods,
    handlePodPageSizeChange,
    handlePodPageChange,
    hydrateWorkloadImages
  }
}
