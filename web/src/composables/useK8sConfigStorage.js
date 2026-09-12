import { computed, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  K8S_OPERATIONS,
  K8S_RESOURCE_TARGETS,
  queryK8sConfigMapDetail,
  queryK8sSecretDetail,
  queryK8sStorageDetail
} from '../api/k8s'
import { kt } from '../utils/k8s-extra-i18n'

// I5-H — extracted from views/assets/K8s.vue (behavior preservation + rewiring
// contract, i5-plan §3.8). Original coordinates per block:
//   84-94 configmap/secret/storage drawer state · 133-163 config-storage create
//   state·forms · 151-153·1652-1656 source-type/scope watches · 1584-1588 kind
//   · 1590-1656 pvc computeds · 1658-1809 storage class·config storage create
//   · 1811-1901 submit · 2214-2315 configmap/secret/storage drawers·edits ·
//   2453-2483 secret edit·delete
export function useK8sConfigStorage({ cluster, namespaces, namespaceFilter, namespaceOptions, storages, runOpTasks }) {
  const configMapDrawerVisible = ref(false)
  const configMapDrawerLoading = ref(false)
  const configMapDetail = ref(null)

  const secretDrawerVisible = ref(false)
  const secretDrawerLoading = ref(false)
  const secretDetail = ref(null)

  const storageDrawerVisible = ref(false)
  const storageDrawerLoading = ref(false)
  const storageDetail = ref(null)

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

  function configStorageCreateKind() {
    if (configStorageTab.value === 'secrets') return 'secret'
    if (configStorageTab.value === 'storage-volumes') return 'pvc'
    return 'configmap'
  }

  const storageAccessModeOptions = computed(() => (
    storageClassCreateForm.sourceType === 'hostpath'
      ? [{ value: 'ReadWriteOnce', label: kt('accessModeRWO') }]
      : [
          { value: 'ReadWriteOnce', label: kt('accessModeRWO') },
          { value: 'ReadOnlyMany', label: kt('accessModeROX') },
          { value: 'ReadWriteMany', label: kt('accessModeRWX') }
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
          label: scope && scope !== 'Cluster-scoped'
            ? kt('nsScopedLabel', { value, scope })
            : kt('clusterScopedLabel', { value }),
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
    return scope && scope !== 'Cluster-scoped' ? scope : ''
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
    if (!selectedPVCStorageClass.value) return kt('selectStorageClassFirst')
    const labels = {
      ReadWriteOnce: kt('accessModeRWO'),
      ReadOnlyMany: kt('accessModeROX'),
      ReadWriteMany: kt('accessModeRWX')
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
      ElMessage.warning(kt('enterStorageClassName'))
      return
    }
    if (!capacity || !path) {
      ElMessage.warning(kt('enterCapacityAndPath'))
      return
    }
    if (sourceType === 'nfs' && !nfsServer) {
      ElMessage.warning(kt('nfsServerRequired'))
      return
    }
    if (storageClassCreateForm.scopeNamespaceEnabled && !scopeNamespace) {
      ElMessage.warning(kt('selectScopeNamespace'))
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
      const ok = await runOpTasks(cluster.value.id, kt('k8sOpProgressTitle', { op: kt('k8sOpResourceCreate') }), [{
        op: K8S_OPERATIONS.resourceCreate,
        connectionScoped: true,
        payload: { yaml: JSON.stringify(manifest, null, 2) },
        display: name
      }])
      if (ok) ElMessage.success(kt('storageClassCreated'))
      storageClassCreateVisible.value = false
    } finally {
      storageClassCreateSaving.value = false
    }
  }

  async function deleteStorageClass(row) {
    if (!cluster.value?.id) return
    if (String(row?.status || '').toLowerCase() !== 'available') {
      ElMessage.warning(kt('deleteUnboundStorageClassOnly'))
      return
    }
    await ElMessageBox.confirm(
      kt('deleteStorageClassConfirm', { name: row.name }),
      kt('deleteStorageClassTitle'),
      { type: 'warning', confirmButtonText: kt('delete'), cancelButtonText: kt('cancel') }
    )
    const ok = await runOpTasks(cluster.value.id, kt('k8sOpProgressTitle', { op: kt('k8sOpResourceDelete') }), [{
      op: K8S_OPERATIONS.resourceDelete,
      target: K8S_RESOURCE_TARGETS.pv('', '', row.name),
      payload: {},
      display: row.name
    }])
    if (ok) ElMessage.success(kt('storageClassDeleted'))
  }

  async function deleteStorageVolume(row) {
    if (!cluster.value?.id || !row?.namespace || !row?.name) return
    await ElMessageBox.confirm(
      kt('deleteStorageConfirm', { target: `${row.namespace}/${row.name}` }),
      kt('deleteStorageTitle'),
      { type: 'warning', confirmButtonText: kt('delete'), cancelButtonText: kt('cancel') }
    )
    const ok = await runOpTasks(cluster.value.id, kt('k8sOpProgressTitle', { op: kt('k8sOpResourceDelete') }), [{
      op: K8S_OPERATIONS.resourceDelete,
      target: K8S_RESOURCE_TARGETS.pvc(row.namespace, '', row.name),
      payload: {},
      display: `${row.namespace}/${row.name}`
    }])
    if (storageDrawerVisible.value && storageDetail.value?.kind === 'PVC' && storageDetail.value?.name === row.name && storageDetail.value?.namespace === row.namespace) {
      storageDrawerVisible.value = false
    }
    if (ok) ElMessage.success(kt('storageDeleted'))
  }

  function configStorageCreateTitle() {
    const action = configStorageEditing.value ? kt('editAction') : kt('createAction')
    const titles = {
      configmap: kt('actionConfigMap', { action }),
      secret: kt('actionSecret', { action }),
      pvc: kt('newStoragePvc')
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
      ElMessage.warning(kt('selectNamespaceWarning'))
      return
    }
    if (!validK8sResourceName(name)) {
      ElMessage.warning(kt('invalidResourceName'))
      return
    }

    let manifest
    if (kind === 'pvc') {
      const capacity = String(configStorageCreateForm.capacity || '').trim()
      const storageClass = String(configStorageCreateForm.storageClass || '').trim()
      if (!capacity) {
        ElMessage.warning(kt('enterStorageCapacity'))
        return
      }
      if (!storageClass || !selectedPVCStorageClass.value) {
        ElMessage.warning(kt('selectStorageClassRequired'))
        return
      }
      if (pvcStorageClassScope.value && namespace !== pvcStorageClassScope.value) {
        ElMessage.warning(kt('storageClassScopeWarning', { storageClass, namespace: pvcStorageClassScope.value }))
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
          ElMessage.warning(kt('enterEntryKeys'))
          return
        }
        if (Object.prototype.hasOwnProperty.call(values, key)) {
          ElMessage.warning(kt('duplicateEntryKey', { key }))
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
    let ok = true
    try {
      if (configStorageEditing.value) {
        ok = await runOpTasks(cluster.value.id, kt('k8sOpProgressTitle', { op: kt('k8sOpResourceApply') }), [{
          op: K8S_OPERATIONS.resourceApply,
          target: K8S_RESOURCE_TARGETS[kind](namespace, '', name),
          payload: { yaml: JSON.stringify(manifest, null, 2) },
          display: `${namespace}/${name}`
        }])
      } else {
        ok = await runOpTasks(cluster.value.id, kt('k8sOpProgressTitle', { op: kt('k8sOpResourceCreate') }), [{
          op: K8S_OPERATIONS.resourceCreate,
          connectionScoped: true,
          payload: { yaml: JSON.stringify(manifest, null, 2) },
          display: `${namespace}/${name}`
        }])
      }
      if (ok) ElMessage.success(kt('configStorageDone', { title: configStorageCreateTitle() }))
      configStorageCreateVisible.value = false
    } finally {
      configStorageCreateSaving.value = false
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
    await ElMessageBox.confirm(kt('deleteConfigMapConfirm', { name: row.name }), kt('deleteConfigMapTitle'), {
      type: 'warning',
      confirmButtonText: kt('k8sDelete'),
      cancelButtonText: kt('cancel')
    })
    const ok = await runOpTasks(cluster.value.id, kt('k8sOpProgressTitle', { op: kt('k8sOpResourceDelete') }), [{
      op: K8S_OPERATIONS.resourceDelete,
      target: K8S_RESOURCE_TARGETS.configmap(row.namespace, '', row.name),
      payload: {},
      display: `${row.namespace}/${row.name}`
    }])
    if (configMapDetail.value?.name === row.name && configMapDetail.value?.namespace === row.namespace) {
      configMapDrawerVisible.value = false
    }
    if (ok) ElMessage.success(kt('configMapDeleted'))
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
    await ElMessageBox.confirm(kt('deleteSecretConfirm', { name: row.name }), kt('deleteSecretTitle'), {
      type: 'warning',
      confirmButtonText: kt('k8sDelete'),
      cancelButtonText: kt('cancel')
    })
    const ok = await runOpTasks(cluster.value.id, kt('k8sOpProgressTitle', { op: kt('k8sOpResourceDelete') }), [{
      op: K8S_OPERATIONS.resourceDelete,
      target: K8S_RESOURCE_TARGETS.secret(row.namespace, '', row.name),
      payload: {},
      display: `${row.namespace}/${row.name}`
    }])
    if (secretDetail.value?.name === row.name && secretDetail.value?.namespace === row.namespace) {
      secretDrawerVisible.value = false
    }
    if (ok) ElMessage.success(kt('secretDeleted'))
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

  return {
    configMapDrawerVisible,
    configMapDrawerLoading,
    configMapDetail,
    secretDrawerVisible,
    secretDrawerLoading,
    secretDetail,
    storageDrawerVisible,
    storageDrawerLoading,
    storageDetail,
    configStorageTab,
    configStorageCreateVisible,
    configStorageCreateSaving,
    configStorageEditing,
    configStorageCreateForm,
    storageClassCreateVisible,
    storageClassCreateSaving,
    storageClassCreateForm,
    configStorageCreateTitle,
    storageAccessModeOptions,
    pvcStorageClassOptions,
    pvcNamespaceOptions,
    pvcNamespaceLocked,
    pvcAccessModeLabel,
    openConfigStorageCreate,
    openStorageClassCreate,
    submitStorageClassCreate,
    deleteStorageClass,
    deleteStorageVolume,
    addConfigStorageEntry,
    removeConfigStorageEntry,
    submitConfigStorageCreate,
    openConfigMapDetail,
    openConfigMapEdit,
    deleteConfigMap,
    openSecretDetail,
    openSecretEdit,
    deleteSecret,
    openStorageDetail
  }
}
