import { computed, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  K8S_OPERATIONS,
  K8S_RESOURCE_TARGETS,
  K8S_TRAFFIC_TARGETS,
  queryK8sIstioResourceDetail,
  queryK8sWorkloadDetail
} from '../api/k8s'
import { kt } from '../utils/k8s-extra-i18n'
import { t } from '../utils/i18n'
import { supportsRestart, supportsScale } from './useK8sFilters'
import { buildIstioTemplate, yamlEditorTarget } from './useK8sYamlEditor'

// I5-H — extracted from views/assets/K8s.vue (behavior preservation + rewiring
// contract, i5-plan §3.8). Original coordinates per block:
//   80·115-120·195-261 selection·workload drawer·scale·batch·image·istio·traffic
//   state · 544·660-808 selection count·batch commands · 810-812 istio namespace
//   · 1067-1073 supports predicates (module exports in useK8sFilters) ·
//   1203-1213 workload drawer · 1228-1275 scale·restart · 1517-1577 resource
//   settings · 2063-2212 istio·traffic · 2485-2499 istio detail labels
// Rewiring notes: buildIstioTemplate moved to useK8sYamlEditor as a module
// function taking the namespace argument (was: closed over
// resolveIstioNamespace()); supportsScale/Restart are module exports of
// useK8sFilters (single source, consumed by workloadSummary there).
export function useK8sResourceActions({
  cluster,
  refreshCurrentClusterData,
  runOpTasks,
  runRestartTasks,
  pushFinishHook,
  namespaceFilter
}) {
  const selectedWorkloads = ref([])

  const workloadDrawerVisible = ref(false)
  const workloadDrawerLoading = ref(false)
  const workloadDetail = ref(null)
  const workloadResourceDialogVisible = ref(false)
  const workloadResourceSaving = ref(false)
  const workloadResourceForm = reactive({ namespace: '', workloadType: '', workloadName: '', containers: [] })

  const scaleDialogVisible = ref(false)
  const scaleLoading = ref(false)
  const scaleForm = reactive({
    namespace: '',
    workloadType: '',
    workloadName: '',
    replicas: 1
  })
  const batchScaleDialogVisible = ref(false)
  const batchScaleSaving = ref(false)
  const batchScaleForm = reactive({ replicas: 1 })

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

  const istioDrawerVisible = ref(false)
  const istioDrawerLoading = ref(false)
  const istioDetail = ref(null)

  const workloadSelectionCount = computed(() => selectedWorkloads.value.length)

  function handleWorkloadSelectionChange(rows) {
    selectedWorkloads.value = Array.isArray(rows) ? rows : []
  }

  function resetWorkloadSelection() {
    selectedWorkloads.value = []
  }

  function resolveIstioNamespace() {
    return namespaceFilter.value !== '__all__' ? namespaceFilter.value : 'default'
  }

  function openImageVersionDialog() {
    if (!selectedWorkloads.value.length) {
      ElMessage.warning(kt('k8sSelectWorkloadsFirst'))
      return
    }
    imageVersionForm.version = ''
    imageVersionDialogVisible.value = true
  }

  function handleWorkloadBatchCommand(command) {
    if (!selectedWorkloads.value.length) {
      ElMessage.warning(kt('k8sSelectWorkloadsFirst'))
      return
    }
    if (command === 'images') return openImageVersionDialog()
    if (command === 'scale') {
      const first = selectedWorkloads.value[0]
      const parts = String(first.ready || '').split('/')
      batchScaleForm.replicas = Number(parts[1] || parts[0] || 1) || 1
      batchScaleDialogVisible.value = true
      return
    }
    if (command === 'restart') return submitBatchWorkloadRestart()
    if (command === 'delete') return submitBatchWorkloadDelete()
  }

  async function submitBatchScale() {
    if (!cluster.value?.id) return
    const targets = selectedWorkloads.value.filter((item) => supportsScale(item))
    if (!targets.length) {
      ElMessage.warning(kt('noScalableWorkloads'))
      return
    }
    try {
      await ElMessageBox.confirm(kt('batchScaleConfirm', { count: targets.length, replicas: batchScaleForm.replicas }), kt('batchScaleConfirmTitle'), { type: 'warning', confirmButtonText: kt('runScale'), cancelButtonText: kt('cancel') })
    } catch {
      return
    }
    batchScaleSaving.value = true
    try {
      const ok = await runOpTasks(cluster.value.id, kt('k8sOpProgressTitle', { op: kt('k8sOpScale') }), targets.map((item) => ({
        op: K8S_OPERATIONS.scale,
        target: K8S_RESOURCE_TARGETS.workload(item.namespace, item.type, item.name),
        payload: { replicas: Number(batchScaleForm.replicas) },
        display: `${item.namespace}/${item.name}`
      })))
      if (ok) ElMessage.success(kt('batchScaleSubmitted', { count: targets.length }))
      batchScaleDialogVisible.value = false
      selectedWorkloads.value = []
    } finally {
      batchScaleSaving.value = false
    }
  }

  async function submitBatchWorkloadRestart() {
    if (!cluster.value?.id) return
    const targets = selectedWorkloads.value.filter((item) => supportsRestart(item))
    if (!targets.length) {
      ElMessage.warning(kt('noRestartableWorkloads'))
      return
    }
    try {
      await ElMessageBox.confirm(kt('batchRestartConfirm', { count: targets.length }), kt('batchRestartConfirmTitle'), { type: 'warning', confirmButtonText: kt('runRestart'), cancelButtonText: kt('cancel') })
    } catch {
      return
    }
    selectedWorkloads.value = []
    await runRestartTasks(targets.map((item) => ({ namespace: item.namespace, type: item.type, name: item.name })))
  }

  async function handleDeleteWorkload(row) {
    if (!cluster.value?.id || !row?.namespace || !row?.name) return
    try {
      await ElMessageBox.confirm(kt('deleteWorkloadConfirm', { target: `${row.namespace}/${row.name}` }), kt('deleteWorkloadConfirmTitle'), { type: 'warning', confirmButtonText: kt('delete'), cancelButtonText: kt('cancel') })
    } catch {
      return
    }
    const ok = await runOpTasks(cluster.value.id, kt('k8sOpProgressTitle', { op: kt('k8sOpResourceDelete') }), [{
      op: K8S_OPERATIONS.resourceDelete,
      target: K8S_RESOURCE_TARGETS.workload(row.namespace, row.type, row.name),
      payload: {},
      display: `${row.namespace}/${row.name}`
    }])
    if (ok) ElMessage.success(kt('workloadDeleted', { name: row.name }))
  }

  async function submitBatchWorkloadDelete() {
    if (!cluster.value?.id) return
    const targets = [...selectedWorkloads.value]
    try {
      await ElMessageBox.confirm(kt('batchDeleteWorkloadsConfirm', { count: targets.length }), kt('batchDeleteWorkloadsTitle'), { type: 'error', confirmButtonText: kt('delete'), cancelButtonText: kt('cancel') })
    } catch {
      return
    }
    const ok = await runOpTasks(cluster.value.id, kt('k8sOpProgressTitle', { op: kt('k8sOpResourceDelete') }), targets.map((item) => ({
      op: K8S_OPERATIONS.resourceDelete,
      target: K8S_RESOURCE_TARGETS.workload(item.namespace, item.type, item.name),
      payload: {},
      display: `${item.namespace}/${item.name}`
    })))
    if (ok) ElMessage.success(kt('workloadsDeleted', { count: targets.length }))
    selectedWorkloads.value = []
  }

  async function submitWorkloadImageVersionUpdate() {
    if (!cluster.value?.id) return
    const version = String(imageVersionForm.version || '').trim()
    if (!version) {
      ElMessage.warning(kt('k8sImageVersionRequired'))
      return
    }
    await ElMessageBox.confirm(
      kt('k8sConfirmBatchImageUpdateMessage', { count: selectedWorkloads.value.length, version }),
      kt('k8sConfirmBatchImageUpdateTitle'),
      {
        type: 'warning',
        confirmButtonText: kt('confirmChange'),
        cancelButtonText: kt('cancel')
      }
    )

    imageVersionSaving.value = true
    try {
      const ok = await runOpTasks(cluster.value.id, kt('k8sOpProgressTitle', { op: kt('k8sOpImageUpdate') }), selectedWorkloads.value.map((item) => ({
        op: K8S_OPERATIONS.imageUpdate,
        target: K8S_RESOURCE_TARGETS.workload(item.namespace, item.type, item.name),
        payload: { version },
        display: `${item.namespace}/${item.name}`
      })))
      if (ok) ElMessage.success(kt('k8sBatchImageUpdatedSuccess'))
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
      const ok = await runOpTasks(cluster.value.id, kt('k8sOpProgressTitle', { op: kt('k8sOpScale') }), [{
        op: K8S_OPERATIONS.scale,
        target: K8S_RESOURCE_TARGETS.workload(scaleForm.namespace, scaleForm.workloadType, scaleForm.workloadName),
        payload: { replicas: Number(scaleForm.replicas) },
        display: `${scaleForm.namespace}/${scaleForm.workloadName}`
      }])
      if (ok) ElMessage.success(kt('k8sWorkloadScaledSuccess'))
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
    await ElMessageBox.confirm(kt('k8sConfirmRestartMessage', { type: row.type, name: row.name }), kt('k8sConfirmRestartTitle'), {
      type: 'warning'
    })
    pushFinishHook(() => {
      if (workloadDrawerVisible.value && workloadDetail.value?.name === row.name) {
        queryK8sWorkloadDetail(cluster.value.id, row.namespace, row.type, row.name).then((detail) => { workloadDetail.value = detail })
      }
    })
    await runRestartTasks([{ namespace: row.namespace, type: row.type, name: row.name }])
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
      const ok = await runOpTasks(cluster.value.id, kt('k8sOpProgressTitle', { op: kt('k8sOpResourcesUpdate') }), [{
        op: K8S_OPERATIONS.resourcesUpdate,
        target: K8S_RESOURCE_TARGETS.workload(workloadResourceForm.namespace, workloadResourceForm.workloadType, workloadResourceForm.workloadName),
        payload: {
          containers: workloadResourceForm.containers.map((container) => ({
            name: container.name,
            requests: { cpu: container.requestCPU, memory: container.requestMemory },
            limits: { cpu: container.limitCPU, memory: container.limitMemory },
            imagePullPolicy: container.imagePullPolicy,
            env: (container.env || []).map((env) => ({ name: env.name, value: env.value, valueFrom: env.valueFrom }))
          }))
        },
        display: `${workloadResourceForm.namespace}/${workloadResourceForm.workloadName}`
      }])
      if (ok) ElMessage.success(kt('podResourceUpdated'))
      workloadResourceDialogVisible.value = false
      await refreshCurrentClusterData()
      if (workloadDrawerVisible.value && workloadDetail.value?.name === workloadResourceForm.workloadName) {
        workloadDetail.value = await queryK8sWorkloadDetail(cluster.value.id, workloadResourceForm.namespace, workloadResourceForm.workloadType, workloadResourceForm.workloadName)
      }
    } finally {
      workloadResourceSaving.value = false
    }
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

  function openIstioCreateDialog(resourceType) {
    istioCreateForm.resourceType = resourceType
    istioCreateForm.yaml = buildIstioTemplate(resourceType, resolveIstioNamespace())
    istioCreateDialogVisible.value = true
  }

  async function submitIstioCreate() {
    if (!cluster.value?.id) return
    await ElMessageBox.confirm(
      kt('k8sCreateIstioResourceConfirm', { resource: yamlResourceLabel(istioCreateForm.resourceType) }),
      kt('k8sCreateIstioResourceTitle', { resource: yamlResourceLabel(istioCreateForm.resourceType) }),
      {
        type: 'warning',
        confirmButtonText: kt('confirmChange'),
        cancelButtonText: kt('cancel')
      }
    )
    istioCreateSaving.value = true
    try {
      const ok = await runOpTasks(cluster.value.id, kt('k8sOpProgressTitle', { op: kt('k8sOpResourceCreate') }), [{
        op: K8S_OPERATIONS.resourceCreate,
        connectionScoped: true,
        payload: { yaml: istioCreateForm.yaml },
        display: yamlResourceLabel(istioCreateForm.resourceType)
      }])
      if (ok) ElMessage.success(kt('k8sIstioResourceCreatedSuccess'))
      istioCreateDialogVisible.value = false
    } finally {
      istioCreateSaving.value = false
    }
  }

  async function handleDeleteIstioResource(row, resourceType) {
    if (!cluster.value?.id) return
    // gatewayapi(→network.gateway)·httproute(→network.http_route)는 V2
    // resource.delete 면 내다. 그 밖의 종은 V2 면 밖이라 이 표에 없다.
    const target = yamlEditorTarget(resourceType, row.namespace, row.name)
    if (!target) {
      ElMessage.warning(kt('k8sV2FaceUnavailable'))
      return
    }
    await ElMessageBox.confirm(
      kt('k8sDeleteIstioResourceConfirm', {
        resource: yamlResourceLabel(resourceType),
        name: row.name
      }),
      kt('k8sDeleteIstioResourceTitle'),
      {
        type: 'warning',
        confirmButtonText: kt('k8sDelete'),
        cancelButtonText: kt('cancel')
      }
    )
    const ok = await runOpTasks(cluster.value.id, kt('k8sOpProgressTitle', { op: kt('k8sOpResourceDelete') }), [{
      op: K8S_OPERATIONS.resourceDelete,
      target,
      payload: {},
      display: `${row.namespace || ''}${row.namespace ? '/' : ''}${row.name}`
    }])
    if (ok) ElMessage.success(kt('k8sIstioResourceDeletedSuccess'))
    if (istioDrawerVisible.value && istioDetail.value?.name === row.name) {
      istioDrawerVisible.value = false
    }
  }

  async function openTrafficDialog(row) {
    if (!cluster.value?.id) return
    const resourceType = row.resourceType || 'virtualservice'
    const detail = await queryK8sIstioResourceDetail(cluster.value.id, resourceType, row.namespace, row.name)
    if (!detail.traffic?.length) {
      ElMessage.warning(kt('k8sNoTrafficRoutes'))
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
      ElMessage.warning(kt('k8sTrafficWeightTotalInvalid', { total: String(trafficTotalWeight.value) }))
      return
    }
    await ElMessageBox.confirm(
      kt('k8sAdjustTrafficConfirm', { name: trafficForm.name }),
      kt('k8sAdjustTrafficTitle'),
      {
        type: 'warning',
        confirmButtonText: kt('confirmChange'),
        cancelButtonText: kt('cancel')
      }
    )
    trafficSaving.value = true
    try {
      const isHTTPRoute = trafficForm.resourceType === 'httproute'
      const ok = await runOpTasks(cluster.value.id, kt('k8sOpProgressTitle', { op: isHTTPRoute ? kt('k8sOpHTTPRouteTrafficUpdate') : kt('k8sOpIstioTrafficUpdate') }), [{
        op: isHTTPRoute ? K8S_OPERATIONS.httpRouteTrafficUpdate : K8S_OPERATIONS.istioTrafficUpdate,
        target: K8S_TRAFFIC_TARGETS[trafficForm.resourceType || 'virtualservice'](trafficForm.namespace, '', trafficForm.name),
        payload: { routes: trafficForm.routes.map((item) => ({ index: item.index, weight: Number(item.weight || 0) })) },
        display: `${trafficForm.namespace}/${trafficForm.name}`
      }])
      if (ok) ElMessage.success(kt('k8sTrafficUpdatedSuccess'))
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

  const trafficTotalWeight = computed(() =>
    (trafficForm.routes || []).reduce((total, item) => total + Number(item.weight || 0), 0)
  )

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

  return {
    selectedWorkloads,
    workloadSelectionCount,
    workloadDrawerVisible,
    workloadDrawerLoading,
    workloadDetail,
    workloadResourceDialogVisible,
    workloadResourceSaving,
    workloadResourceForm,
    scaleDialogVisible,
    scaleLoading,
    scaleForm,
    batchScaleDialogVisible,
    batchScaleSaving,
    batchScaleForm,
    imageVersionDialogVisible,
    imageVersionSaving,
    imageVersionForm,
    istioCreateDialogVisible,
    istioCreateSaving,
    istioCreateForm,
    trafficDialogVisible,
    trafficSaving,
    trafficForm,
    trafficTotalWeight,
    istioDrawerVisible,
    istioDrawerLoading,
    istioDetail,
    handleWorkloadSelectionChange,
    resetWorkloadSelection,
    openImageVersionDialog,
    handleWorkloadBatchCommand,
    submitBatchScale,
    handleDeleteWorkload,
    submitWorkloadImageVersionUpdate,
    openWorkloadDetail,
    openScaleDialog,
    submitScale,
    handleRestartWorkload,
    openWorkloadResourceSettings,
    submitWorkloadResourceSettings,
    addWorkloadEnvironment,
    removeWorkloadEnvironment,
    openIstioResourceDetail,
    openIstioCreateDialog,
    submitIstioCreate,
    handleDeleteIstioResource,
    openTrafficDialog,
    submitTrafficAdjust,
    translateIstioDetailLabel,
    supportsScale,
    supportsRestart
  }
}
