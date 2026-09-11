import http from './http'
import { executeInfraOperation, listInfraProviderConnections, listInfraResources, planInfraOperation } from './infra'

export const queryK8sClusterList = () => http.get('/api/v1/k8s/cluster/list')

export const queryK8sClusterInfo = (id) => http.get('/api/v1/k8s/cluster/info', { params: { id } })

export const queryK8sClusterOverview = (clusterId) =>
  http.get('/api/v1/k8s/cluster/detail', { params: { clusterId } })

export const queryK8sNodeDetail = (clusterId, nodeName) =>
  http.get('/api/v1/k8s/node/detail', { params: { clusterId, nodeName } })

export const queryK8sNodePods = (clusterId, nodeName) =>
  http.get('/api/v1/k8s/node/pods', { params: { clusterId, nodeName } })

export const queryK8sNamespaceDetail = (clusterId, namespace) =>
  http.get('/api/v1/k8s/namespace/detail', { params: { clusterId, namespace } })

export const queryK8sNamespaceEvents = (clusterId, namespace) =>
  http.get('/api/v1/k8s/namespace/events', { params: { clusterId, namespace } })

export const queryK8sServiceDetail = (clusterId, namespace, serviceName) =>
  http.get('/api/v1/k8s/service/detail', { params: { clusterId, namespace, serviceName } })

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

// ---- V2 mutations (plan §3.2 H1 — plan → execute(Idempotency-Key) → approve → poll) ----
// The v1 write routes die at H2; every mutation rides the §16.2 operation
// flow (E2 restart pattern §3.5). Create joined at I10 J2 (§16.1 connection-
// scoped face — D-15 repaid on the consumption side; the v1 route itself is
// removed at J3).
// The 10 operations (§3.2 — compose_k8s_ops.go opdef names are the single
// source; these constants mirror them for the view layer).
export const K8S_OPERATIONS = {
  restart: 'k8s.workload.restart',
  scale: 'k8s.workload.scale',
  imageUpdate: 'k8s.workload.image_update',
  resourcesUpdate: 'k8s.workload.resources_update',
  nodeLabelsUpdate: 'k8s.node.labels_update',
  serviceUpdate: 'k8s.service.update',
  resourceCreate: 'k8s.resource.create',
  resourceApply: 'k8s.resource.apply',
  resourceDelete: 'k8s.resource.delete',
  istioTrafficUpdate: 'k8s.istio.traffic_update',
  httpRouteTrafficUpdate: 'k8s.httproute.traffic_update'
}

// V2 inventory target specs per view resourceType — kind + URN-tail builder
// (buildURN §8.1: workload `:workload:{ns}/{type}/{name}`, other name-addressable
// kinds `:{singular}:{ns}/{name}`, cluster-scoped `:{singular}:{name}`). The
// uid-identity node matches by displayName instead. pod·virtualservice and the
// uncollected istio kinds have no V2 face and stay out of this table.
export const K8S_RESOURCE_TARGETS = {
  workload: (namespace, workloadType, name) => ({ kind: 'orchestration.workload', urnTail: `:workload:${namespace}/${workloadType}/${name}` }),
  namespace: (_namespace, _workloadType, name) => ({ kind: 'orchestration.namespace', urnTail: `:namespace:${name}` }),
  service: (namespace, _workloadType, name) => ({ kind: 'network.load_balancer', urnTail: `:service:${namespace}/${name}` }),
  ingress: (namespace, _workloadType, name) => ({ kind: 'network.load_balancer', urnTail: `:ingress:${namespace}/${name}` }),
  configmap: (namespace, _workloadType, name) => ({ kind: 'orchestration.configmap', urnTail: `:configmap:${namespace}/${name}` }),
  secret: (namespace, _workloadType, name) => ({ kind: 'orchestration.secret', urnTail: `:secret:${namespace}/${name}` }),
  pv: (_namespace, _workloadType, name) => ({ kind: 'storage.volume', urnTail: `:pv:${name}` }),
  pvc: (namespace, _workloadType, name) => ({ kind: 'storage.volume', urnTail: `:pvc:${namespace}/${name}` }),
  // Gateway API faces — the advanced-network tab's Gateway/HTTPRoute rows.
  gatewayapi: (namespace, _workloadType, name) => ({ kind: 'network.gateway', urnTail: `:gateway:${namespace}/${name}` }),
  httproute: (namespace, _workloadType, name) => ({ kind: 'network.http_route', urnTail: `:httproute:${namespace}/${name}` })
}

// Traffic ops face their own kinds — istio VirtualService (P2-D uid anchor)
// and the Gateway API HTTPRoute.
export const K8S_TRAFFIC_TARGETS = {
  virtualservice: (namespace, _workloadType, name) => ({ kind: 'network.virtual_service', urnTail: `:virtualservice:${namespace}/${name}` }),
  httproute: (namespace, _workloadType, name) => ({ kind: 'network.http_route', urnTail: `:httproute:${namespace}/${name}` })
}
// §13.5 closed 9-state vocabulary — the terminal set mirrors TaskDetail.vue.
export const K8S_OPERATION_TASK_TERMINAL_STATUSES = ['succeeded', 'failed', 'timed_out', 'cancelled']

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

// E0 connectionUid filter narrows to one cluster. A target is matched either
// by URN tail — `:workload:{ns}/{type}/{name}` is unique inside the cluster
// (§J4), and the other name-addressable kinds follow the same shape — or, for
// the uid-identity kinds (node), by display name (the URN tail carries the
// node uid, not the name the v1 UI holds).
// The backend caps pageSize at 100 and answers `{items, total, page, pageSize}`,
// so a cluster with more than one page of same-kind resources is walked with
// the total-based pagination until the target appears (no false "not found"
// on row 101+).
export async function resolveK8sResourceUid(connectionUid, { kind, urnTail, displayName }) {
  const pageSize = 100
  const seen = new Set()
  let page = 1
  let total = Infinity
  while (seen.size < total) {
    const response = await listInfraResources({ kind, connectionUid, pageSize, page })
    const items = response?.items || []
    const match = items.find((row) => (urnTail
      ? typeof row.externalUrn === 'string' && row.externalUrn.endsWith(urnTail)
      : row.displayName === displayName))
    if (match?.uid) {
      return match.uid
    }
    items.forEach((row) => seen.add(row.uid))
    total = Number(response?.total) || 0
    if (!items.length) {
      break
    }
    page += 1
  }
  const target = urnTail || `displayName=${displayName}`
  throw new Error(`resource not found in V2 inventory: ${kind} ${target}`)
}

// §3.5: one key per `<resourceUid>:<op>:<submit timestamp(ms)>`; repeated
// clicks on the same button reuse the key so replays never spawn new tasks
// (the §13.4 unique index is the server-side last line of defense).
const operationIdempotencyKeys = new Map()
export function k8sOperationIdempotencyKey(resourceUid, operation) {
  const mapKey = `${resourceUid}:${operation}`
  let key = operationIdempotencyKeys.get(mapKey)
  if (!key) {
    key = `${mapKey}:${Date.now()}`
    operationIdempotencyKeys.set(mapKey, key)
  }
  return key
}

// The server replays a finished task for a known key regardless of state, so a
// spent key must be dropped the moment its task reaches a terminal state —
// otherwise a later re-fire would replay the old task and report success
// without touching the cluster. In-flight double clicks still reuse the key.
export function clearK8sOperationIdempotencyKey(resourceUid, operation) {
  operationIdempotencyKeys.delete(`${resourceUid}:${operation}`)
}

// plan → execute with the §3.5 key. The payload rides the request body and is
// frozen at execute; restartedAt/resourceRevision come from the plan response
// (J1/J7). Returns the task uid plus the plan's permission string so the
// caller can decide whether this user may approve.
export async function createK8sOperationTask(resourceUid, operation, payload = {}) {
  const plan = await planInfraOperation(resourceUid, operation)
  const body = {
    ...payload,
    restartedAt: plan.restartedAt,
    ...(plan.resourceRevision !== null && plan.resourceRevision !== undefined
      ? { resourceRevision: plan.resourceRevision }
      : {})
  }
  const response = await executeInfraOperation(resourceUid, plan.operation || operation, body, k8sOperationIdempotencyKey(resourceUid, operation))
  const taskUid = response?.task?.uid
  if (!taskUid) {
    throw new Error(`${operation} execution returned no task uid`)
  }
  return { taskUid, permission: plan.permission || '' }
}

// ---- §16.1 connection-scoped create (I10 J2) ----
// targetKey — 프론트가 조립한 매니페스트의 `Kind:ns/name` 신원(§3.6). 제출 시점에
// 한 번만 도출되어 row에 보관되고, 키 생성·소각이 같은 문자열을 소비한다(§5 #17).
// 매니페스트 텍스트는 JSON 직렬화본(pv·configStorage)과 bare yaml(namespace·istio)
// 2형태가 있어 둘 다 읽는 완화 정규식으로 metadata 블록의 첫 매치를 취한다.
// 추출 실패 시 per-submit 유니크 폴백 — 실패 제출 간 슬롯 공유 재생 위험 없이
// 더블클릭 중복 제출만 남고(동일 활성 목표는 서버 RESOURCE_BUSY가 방어) 대상
// 검증 자체는 백엔드 plan이 소유한다.
export function k8sManifestTargetKey(yaml) {
  const pick = (field) => {
    const match = yaml.match(new RegExp(`^\\s*"?${field}"?:\\s*"?([A-Za-z0-9][A-Za-z0-9._/-]*)`, 'm'))
    return match ? match[1] : ''
  }
  const kind = pick('kind')
  const name = pick('name')
  if (!kind || !name) {
    return `unidentified:${Date.now()}`
  }
  const namespace = pick('namespace')
  return namespace ? `${kind}:${namespace}/${name}` : `${kind}:${name}`
}

// §5 #17 — 맵 키 템플릿은 이 한 곳만 소유한다. 생성(k8sConnectionIdempotencyKey)과
// 소각(clearK8sConnectionIdempotencyKey)이 같은 템플릿을 소비하지 않으면 슬롯이
// 어긋나 낡은 SUCCEEDED 재생이 silent false-success를 낳는다(§3.6 위험1). 기존
// resourceUid:op 쌍(k8sOperationIdempotencyKey)의 구조를 승계한다.
const connectionIdempotencyMapKey = (connectionUid, operation, targetKey) => `${connectionUid}:${operation}:${targetKey}`
const connectionIdempotencyKeys = new Map()

export function k8sConnectionIdempotencyKey(connectionUid, operation, targetKey) {
  const mapKey = connectionIdempotencyMapKey(connectionUid, operation, targetKey)
  let key = connectionIdempotencyKeys.get(mapKey)
  if (!key) {
    key = `${mapKey}:${Date.now()}`
    connectionIdempotencyKeys.set(mapKey, key)
  }
  return key
}

export function clearK8sConnectionIdempotencyKey(connectionUid, operation, targetKey) {
  connectionIdempotencyKeys.delete(connectionIdempotencyMapKey(connectionUid, operation, targetKey))
}

// §16.1 커넥션-스코프 create 제출 — plan(body={yaml}) → execute(공유 템플릿 키).
// plan은 restartedAt/resourceRevision을 발급하지 않는다(§3.5 — create의 동결
// 앵커는 매니페스트 자신) — execute 본문은 {yaml} 그대로다. 반환은 task uid와
// plan의 permission 문자열(승인 자동화 판정 입력).
export async function createK8sConnectionOperationTask(connectionUid, operation, payload, targetKey) {
  const plan = await http.post(
    `/api/v2/infra/provider-connections/${encodeURIComponent(connectionUid)}/operations/${encodeURIComponent(operation)}/plan`,
    payload
  )
  const response = await http.post(
    `/api/v2/infra/provider-connections/${encodeURIComponent(connectionUid)}/operations/${encodeURIComponent(operation)}/execute`,
    payload,
    { headers: { 'Idempotency-Key': k8sConnectionIdempotencyKey(connectionUid, operation, targetKey) } }
  )
  const taskUid = response?.task?.uid
  if (!taskUid) {
    throw new Error(`${operation} execution returned no task uid`)
  }
  return { taskUid, permission: plan.permission || '' }
}

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
