import { computed, reactive, ref } from 'vue'
import { queryK8sPodContainers, queryK8sPodDetail, queryK8sPodEvents, queryK8sPodLogs } from '../api/k8s'

// I5-H — extracted from views/assets/K8s.vue (behavior preservation + rewiring
// contract, i5-plan §3.8; plan §3.8 "필요 시 useK8sPodLogs"). Original
// coordinates per block: 122-132 pod drawer·log state · 164-167 currentPodQuery
// · 343-347 container options·log lines · 1277-1357 pod detail·logs·terminal ·
// 1327-1337 refreshPodLogs
export function useK8sPodLogs({ cluster, router }) {
  const podDrawerVisible = ref(false)
  const podDrawerLoading = ref(false)
  const podDetail = ref(null)
  const podEvents = ref([])
  const podLogDrawerVisible = ref(false)
  const podLogLoading = ref(false)
  const podLogs = ref('')
  const selectedContainer = ref('')
  const podLogTailLines = ref(200)
  const currentPodQuery = reactive({
    namespace: '',
    podName: ''
  })

  const podContainerOptions = computed(() => {
    if (!podDetail.value?.containers) return []
    return podDetail.value.containers.map((item) => item.name)
  })
  const podLogLines = computed(() => String(podLogs.value || '').split('\n').filter((line, index, list) => line || index < list.length - 1))

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

  return {
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
    podContainerOptions,
    podLogLines,
    openPodDetail,
    openPodLogs,
    refreshPodLogs,
    openPodTerminal
  }
}
