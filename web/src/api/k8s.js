import http from './http'
import { executeInfraOperation, listInfraProviderConnections, listInfraResources, planInfraOperation } from './infra'

export const queryK8sClusterList = () => http.get('/api/v1/k8s/cluster/list')

export const queryK8sClusterInfo = (id) => http.get('/api/v1/k8s/cluster/info', { params: { id } })

export const addK8sCluster = (data) => http.post('/api/v1/k8s/cluster/add', data)

export const updateK8sCluster = (data) => http.put('/api/v1/k8s/cluster/update', data)

export const deleteK8sCluster = (id) => http.delete('/api/v1/k8s/cluster/delete', { data: { id } })

export const queryK8sClusterOverview = (clusterId) =>
  http.get('/api/v1/k8s/cluster/detail', { params: { clusterId } })

export const queryK8sNodeDetail = (clusterId, nodeName) =>
  http.get('/api/v1/k8s/node/detail', { params: { clusterId, nodeName } })

export const queryK8sNodePods = (clusterId, nodeName) =>
  http.get('/api/v1/k8s/node/pods', { params: { clusterId, nodeName } })

export const updateK8sNodeLabels = (data) => http.put('/api/v1/k8s/node/labels', data)

export const queryK8sNamespaceDetail = (clusterId, namespace) =>
  http.get('/api/v1/k8s/namespace/detail', { params: { clusterId, namespace } })

export const queryK8sNamespaceEvents = (clusterId, namespace) =>
  http.get('/api/v1/k8s/namespace/events', { params: { clusterId, namespace } })

export const queryK8sServiceDetail = (clusterId, namespace, serviceName) =>
  http.get('/api/v1/k8s/service/detail', { params: { clusterId, namespace, serviceName } })

export const updateK8sService = (data) => http.put('/api/v1/k8s/service/update', data)

export const queryK8sIngressDetail = (clusterId, namespace, ingressName) =>
  http.get('/api/v1/k8s/ingress/detail', { params: { clusterId, namespace, ingressName } })

export const queryK8sIstioResourceDetail = (clusterId, resourceType, namespace, name) =>
  http.get('/api/v1/k8s/istio/detail', { params: { clusterId, resourceType, namespace, name } })

export const queryK8sConfigMapDetail = (clusterId, namespace, configMapName) =>
  http.get('/api/v1/k8s/configmap/detail', { params: { clusterId, namespace, configMapName } })

export const queryK8sSecretDetail = (clusterId, namespace, secretName) =>
  http.get('/api/v1/k8s/secret/detail', { params: { clusterId, namespace, secretName } })

export const queryK8sStorageDetail = (clusterId, kind, namespace, name) =>
  http.get('/api/v1/k8s/storage/detail', { params: { clusterId, kind, namespace, name } })

export const queryK8sPodDetail = (clusterId, namespace, podName) =>
  http.get('/api/v1/k8s/pod/detail', { params: { clusterId, namespace, podName } })

export const queryK8sPodMetrics = (clusterId, namespace, podName, range = '1h') =>
  http.get('/api/v1/k8s/pod/metrics', { params: { clusterId, namespace, podName, range } })

export const queryK8sWorkloadMetrics = (clusterId, namespace, workloadType, workloadName, range = '1h') =>
  http.get('/api/v1/k8s/workload/metrics', { params: { clusterId, namespace, workloadType, workloadName, range } })

export const queryK8sPodContainers = (clusterId, namespace, podName) =>
  http.get('/api/v1/k8s/pod/containers', { params: { clusterId, namespace, podName } })

export const queryK8sPodLogs = (clusterId, namespace, podName, container = '', tailLines = 200) =>
  http.get('/api/v1/k8s/pod/logs', { params: { clusterId, namespace, podName, container, tailLines } })

export const queryK8sPodEvents = (clusterId, namespace, podName) =>
  http.get('/api/v1/k8s/pod/events', { params: { clusterId, namespace, podName } })

export const queryK8sWorkloadDetail = (clusterId, namespace, workloadType, workloadName) =>
  http.get('/api/v1/k8s/workload/detail', { params: { clusterId, namespace, workloadType, workloadName } })

export const scaleK8sWorkload = (data) => http.post('/api/v1/k8s/workload/scale', data)

// ---- V2 workload restart (plan §3.5 / Phase E2 — 4-step operation flow) ----
// The v1 direct call is gone (E1 deletes the backend route); restarts go through
// plan → execute(Idempotency-Key) → approve → poll as §16.2 tasks.
const RESTART_OPERATION = 'k8s.workload.restart'
const RESTART_RESOURCE_KIND = 'orchestration.workload'
// §13.5 closed 9-state vocabulary — the terminal set mirrors TaskDetail.vue.
export const K8S_RESTART_TASK_TERMINAL_STATUSES = ['succeeded', 'failed', 'timed_out', 'cancelled']

// E2 §3.5: provider-connections → the row whose sourceModel is k8s_cluster and
// sourceId is the cluster id exposes the connection uid the E0 filter needs.
export async function resolveK8sClusterConnectionUid(clusterId) {
  const connections = await listInfraProviderConnections()
  const match = (connections?.items || []).find(
    (row) => row.sourceModel === 'k8s_cluster' && String(row.sourceId) === String(clusterId)
  )
  if (!match?.uid) {
    throw new Error(`k8s_cluster provider connection not found (clusterId=${clusterId})`)
  }
  return match.uid
}

// E0 connectionUid filter narrows to one cluster; the URN tail
// `:workload:{ns}/{type}/{name}` is unique inside that cluster (§J4).
export async function resolveK8sWorkloadResourceUid(connectionUid, namespace, workloadType, workloadName) {
  const response = await listInfraResources({ kind: RESTART_RESOURCE_KIND, connectionUid, pageSize: 100 })
  const tail = `:workload:${namespace}/${workloadType}/${workloadName}`
  const match = (response?.items || []).find(
    (row) => typeof row.externalUrn === 'string' && row.externalUrn.endsWith(tail)
  )
  if (!match?.uid) {
    throw new Error(`workload resource not found in V2 inventory: ${tail}`)
  }
  return match.uid
}

// §3.5: one key per `<resourceUid>:restart:<submit timestamp(ms)>`; repeated
// clicks on the same button reuse the key so replays never spawn new tasks
// (the §13.4 unique index is the server-side last line of defense).
const restartIdempotencyKeys = new Map()
export function k8sRestartIdempotencyKey(resourceUid) {
  let key = restartIdempotencyKeys.get(resourceUid)
  if (!key) {
    key = `${resourceUid}:restart:${Date.now()}`
    restartIdempotencyKeys.set(resourceUid, key)
  }
  return key
}

// The server replays a finished task for a known key regardless of state, so a
// spent key must be dropped the moment its task reaches a terminal state —
// otherwise a later re-fire would replay the old task and report success
// without touching the cluster. In-flight double clicks still reuse the key.
export function clearK8sRestartIdempotencyKey(resourceUid) {
  restartIdempotencyKeys.delete(resourceUid)
}

// plan → execute with the §3.5 key. Returns the task uid plus the plan's
// permission string so the caller can decide whether this user may approve.
export async function createK8sWorkloadRestartTask(resourceUid, idempotencyKey) {
  const plan = await planInfraOperation(resourceUid, RESTART_OPERATION)
  const payload = { restartedAt: plan.restartedAt }
  if (plan.resourceRevision !== null && plan.resourceRevision !== undefined) {
    payload.resourceRevision = plan.resourceRevision
  }
  const response = await executeInfraOperation(resourceUid, plan.operation || RESTART_OPERATION, payload, idempotencyKey)
  const taskUid = response?.task?.uid
  if (!taskUid) {
    throw new Error('restart execution returned no task uid')
  }
  return { taskUid, permission: plan.permission || '' }
}

export const updateK8sWorkloadImages = (data) => http.post('/api/v1/k8s/workload/images', data)

export const updateK8sWorkloadResources = (data) => http.put('/api/v1/k8s/workload/resources', data)

export const updateK8sIstioTraffic = (data) => http.post('/api/v1/k8s/istio/traffic', data)

export const updateK8sHTTPRouteTraffic = (data) => http.post('/api/v1/k8s/httproute/traffic', data)

export const createK8sResourceYAML = (data) => http.post('/api/v1/k8s/resource/yaml/create', data)

export const updateK8sResourceYAML = (data) => http.put('/api/v1/k8s/resource/yaml', data)

export const deleteK8sResource = (data) => http.delete('/api/v1/k8s/resource/delete', { data })

export const buildK8sPodTerminalWSUrl = ({
  clusterId,
  namespace,
  podName,
  container = '',
  command = '/bin/sh',
  rows = 32,
  cols = 120,
  ticket
}) => {
  const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
  const params = new URLSearchParams({
    clusterId: String(clusterId),
    namespace,
    podName,
    command,
    rows: String(rows),
    cols: String(cols)
  })
  params.set('ticket', ticket)
  if (container) {
    params.set('container', container)
  }
  return `${protocol}://${window.location.host}/api/v1/k8s/pod/terminal/ws?${params.toString()}`
}
