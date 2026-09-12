import { reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { K8S_OPERATIONS, K8S_RESOURCE_TARGETS, queryK8sIngressDetail, queryK8sServiceDetail } from '../api/k8s'
import { kt } from '../utils/k8s-extra-i18n'

// I5-H — extracted from views/assets/K8s.vue (behavior preservation + rewiring
// contract, i5-plan §3.8). Original coordinates per block:
//   169-193 service/ingress drawer·edit state · 1359-1405 detail·yaml entry
//   (yaml entries moved to useK8sYamlEditor) · 1407-1506 service edit form,
//   port/selector/metadata rows, submit, copy
export function useK8sServiceEdit({ cluster, refreshCurrentClusterData, runOpTasks }) {
  const serviceDrawerVisible = ref(false)
  const serviceDrawerLoading = ref(false)
  const serviceDetail = ref(null)
  const serviceEditVisible = ref(false)
  const serviceEditLoading = ref(false)
  const serviceEditSaving = ref(false)
  const serviceEditForm = reactive({
    name: '',
    namespace: '',
    type: 'ClusterIP',
    headless: false,
    externalName: '',
    selectors: [],
    labels: [],
    annotations: [],
    ports: []
  })

  const ingressDrawerVisible = ref(false)
  const ingressDrawerLoading = ref(false)
  const ingressDetail = ref(null)

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

  async function openServiceEdit(row) {
    if (!cluster.value?.id) return
    serviceEditVisible.value = true
    serviceEditLoading.value = true
    try {
      const detail = await queryK8sServiceDetail(cluster.value.id, row.namespace, row.name)
      serviceEditForm.name = detail.name
      serviceEditForm.namespace = detail.namespace
      serviceEditForm.type = detail.clusterIP === 'None' ? 'Headless' : (detail.type || 'ClusterIP')
      serviceEditForm.headless = serviceEditForm.type === 'Headless'
      serviceEditForm.externalName = detail.externalName || ''
      serviceEditForm.selectors = Object.entries(detail.selector || {}).map(([key, value]) => ({ key, value }))
      serviceEditForm.labels = Object.entries(detail.labels || {}).map(([key, value]) => ({ key, value }))
      serviceEditForm.annotations = Object.entries(detail.annotations || {}).map(([key, value]) => ({ key, value }))
      serviceEditForm.ports = (detail.portSpecs || []).map((port) => ({
        name: port.name || '', protocol: port.protocol || 'TCP', port: port.port || 80,
        targetPort: port.targetPort || String(port.port || 80), nodePort: port.nodePort || undefined
      }))
      if (!serviceEditForm.ports.length && serviceEditForm.type !== 'ExternalName') addServicePort()
    } catch (error) {
      serviceEditVisible.value = false
    } finally {
      serviceEditLoading.value = false
    }
  }

  function addServiceSelector() {
    serviceEditForm.selectors.push({ key: '', value: '' })
  }

  function removeServiceSelector(index) {
    serviceEditForm.selectors.splice(index, 1)
  }

  function addServiceMetadataEntry(field) {
    serviceEditForm[field].push({ key: '', value: '' })
  }

  function removeServiceMetadataEntry(field, index) {
    serviceEditForm[field].splice(index, 1)
  }

  function addServicePort() {
    serviceEditForm.ports.push({ name: '', protocol: 'TCP', port: 80, targetPort: '80', nodePort: undefined })
  }

  function removeServicePort(index) {
    serviceEditForm.ports.splice(index, 1)
  }

  async function submitServiceEdit() {
    if (!cluster.value?.id) return
    if (serviceEditForm.type !== 'ExternalName' && !serviceEditForm.ports.length) {
      ElMessage.warning(kt('minServicePort'))
      return
    }
    const selector = Object.fromEntries(serviceEditForm.selectors
      .filter((item) => item.key?.trim() && item.value?.trim())
      .map((item) => [item.key.trim(), item.value.trim()]))
    const metadataMap = (items) => Object.fromEntries(items
      .filter((item) => item.key?.trim() && item.value?.trim())
      .map((item) => [item.key.trim(), item.value.trim()]))
    serviceEditSaving.value = true
    try {
      const ok = await runOpTasks(cluster.value.id, kt('k8sOpProgressTitle', { op: kt('k8sOpServiceUpdate') }), [{
        op: K8S_OPERATIONS.serviceUpdate,
        target: K8S_RESOURCE_TARGETS.service(serviceEditForm.namespace, '', serviceEditForm.name),
        payload: {
          type: serviceEditForm.type === 'Headless' ? 'ClusterIP' : serviceEditForm.type,
          headless: serviceEditForm.type === 'Headless',
          externalName: serviceEditForm.externalName,
          selector,
          labels: metadataMap(serviceEditForm.labels),
          annotations: metadataMap(serviceEditForm.annotations),
          ports: serviceEditForm.ports.map((port) => ({
            name: port.name?.trim() || '', protocol: port.protocol || 'TCP', port: Number(port.port),
            targetPort: String(port.targetPort || port.port), nodePort: Number(port.nodePort) || 0
          }))
        },
        display: `${serviceEditForm.namespace}/${serviceEditForm.name}`
      }])
      if (ok) ElMessage.success(kt('serviceUpdated', { name: serviceEditForm.name }))
      serviceEditVisible.value = false
      await refreshCurrentClusterData()
      if (serviceDetail.value?.name === serviceEditForm.name && serviceDetail.value?.namespace === serviceEditForm.namespace) {
        serviceDetail.value = await queryK8sServiceDetail(cluster.value.id, serviceEditForm.namespace, serviceEditForm.name)
      }
    } finally {
      serviceEditSaving.value = false
    }
  }

  async function copyServiceName(row) {
    try {
      await navigator.clipboard.writeText(row.name)
      ElMessage.success(kt('serviceNameCopied', { name: row.name }))
    } catch (error) {
      ElMessage.warning(kt('clipboardUnavailable'))
    }
  }

  return {
    serviceDrawerVisible,
    serviceDrawerLoading,
    serviceDetail,
    serviceEditVisible,
    serviceEditLoading,
    serviceEditSaving,
    serviceEditForm,
    ingressDrawerVisible,
    ingressDrawerLoading,
    ingressDetail,
    openServiceDetail,
    openIngressDetail,
    openServiceEdit,
    addServiceSelector,
    removeServiceSelector,
    addServiceMetadataEntry,
    removeServiceMetadataEntry,
    addServicePort,
    removeServicePort,
    submitServiceEdit,
    copyServiceName
  }
}
