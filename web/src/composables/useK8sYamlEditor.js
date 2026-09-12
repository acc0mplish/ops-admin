import { computed, nextTick, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  K8S_OPERATIONS,
  K8S_RESOURCE_TARGETS,
  queryK8sConfigMapDetail,
  queryK8sIngressDetail,
  queryK8sIstioResourceDetail,
  queryK8sNamespaceDetail,
  queryK8sPodDetail,
  queryK8sSecretDetail,
  queryK8sServiceDetail,
  queryK8sStorageDetail,
  queryK8sWorkloadDetail
} from '../api/k8s'
import { kt } from '../utils/k8s-extra-i18n'
import { t } from '../utils/i18n'

// I5-H — extracted from views/assets/K8s.vue (behavior preservation + rewiring
// contract, i5-plan §3.8; useK8sYamlEditor = YAML 편집·검색·diff). Original
// coordinates per block:
//   233-239 yamlEditorTarget · 263-283 editor state · 400-422 diff computeds ·
//   810-920 istio YAML templates · 922-1065 editor·search·diff ·
//   1192-1201·1215-1226·1313-1325·1371-1405·2075-2085·2226-2315 open*YAML
//   entries · 2317-2422 refresh·submit · 2424-2451 resource labels·title
// Rewiring notes: buildIstioTemplate now takes the namespace as an argument
// (was: closed over resolveIstioNamespace(); caller resolves it — plan §3.8
// parameter-injection rewiring). yamlEditorTarget·yamlResourceLabel are
// module-level single sources shared with useK8sResourceActions.

// yamlEditor.resourceType → V2 대상 스펙. pod·virtualservice·istio 종은 V2
// apply/delete 면 밖이라 이 표에 없다 — submitYAMLUpdate가 이들을 guard한다.
export function yamlEditorTarget(resourceType, namespace, name, workloadType) {
  const spec = K8S_RESOURCE_TARGETS[resourceType]
  if (!spec) return null
  return spec(namespace, workloadType, name)
}

export function yamlResourceLabel(key) {
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

export function buildIstioTemplate(resourceType, namespace) {
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

export function useK8sYamlEditor({ cluster, runOpTasks, details }) {
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
    yaml: '',
    readOnly: false
  })
  const yamlSearch = reactive({
    keyword: '',
    matches: [],
    activeIndex: -1
  })
  const yamlEditorScrollTop = ref(0)
  const yamlCurrentLine = ref(1)
  const yamlLineHeight = 20

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

  function setYAMLEditor(payload) {
    yamlEditor.title = payload.title || kt('k8sYamlEditor')
    yamlEditor.resourceType = payload.resourceType || ''
    yamlEditor.namespace = payload.namespace || ''
    yamlEditor.name = payload.name || ''
    yamlEditor.workloadType = payload.workloadType || ''
    yamlEditor.originalYAML = payload.yaml || ''
    yamlEditor.yaml = payload.yaml || ''
    yamlEditor.readOnly = payload.readOnly || false
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

  function buildYAMLTitle(resourceKey, name) {
    return kt('k8sEditResourceYamlTitle', {
      resource: yamlResourceLabel(resourceKey),
      name
    })
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

  async function openPodYAML(row) {
    if (!cluster.value?.id) return
    const detail = await queryK8sPodDetail(cluster.value.id, row.namespace, row.name)
    setYAMLEditor({
      title: buildYAMLTitle('pod', detail.name),
      resourceType: 'pod',
      namespace: detail.namespace,
      name: detail.name,
      yaml: detail.yaml,
      // H1 — pod는 V2 apply 면 밖(uid 신원 제외). 조회는 유지, 편집 제출은 제거.
      readOnly: true
    })
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
        if (details.namespaceDetail.value?.name) {
          details.namespaceDetail.value = await queryK8sNamespaceDetail(cluster.value.id, details.namespaceDetail.value.name)
        }
        break
      case 'workload':
        if (details.workloadDetail.value?.name) {
          details.workloadDetail.value = await queryK8sWorkloadDetail(
            cluster.value.id,
            details.workloadDetail.value.namespace,
            details.workloadDetail.value.type,
            details.workloadDetail.value.name
          )
        }
        break
      case 'pod':
        if (details.podDetail.value?.name) {
          details.podDetail.value = await queryK8sPodDetail(cluster.value.id, details.podDetail.value.namespace, details.podDetail.value.name)
        }
        break
      case 'service':
        if (details.serviceDetail.value?.name) {
          details.serviceDetail.value = await queryK8sServiceDetail(cluster.value.id, details.serviceDetail.value.namespace, details.serviceDetail.value.name)
        }
        break
      case 'ingress':
        if (details.ingressDetail.value?.name) {
          details.ingressDetail.value = await queryK8sIngressDetail(cluster.value.id, details.ingressDetail.value.namespace, details.ingressDetail.value.name)
        }
        break
      case 'gateway':
      case 'virtualservice':
      case 'destinationrule':
      case 'serviceentry':
        if (details.istioDetail.value?.name) {
          details.istioDetail.value = await queryK8sIstioResourceDetail(
            cluster.value.id,
            yamlEditor.resourceType,
            details.istioDetail.value.namespace,
            details.istioDetail.value.name
          )
        }
        break
      case 'configmap':
        if (details.configMapDetail.value?.name) {
          details.configMapDetail.value = await queryK8sConfigMapDetail(cluster.value.id, details.configMapDetail.value.namespace, details.configMapDetail.value.name)
        }
        break
      case 'secret':
        if (details.secretDetail.value?.name) {
          details.secretDetail.value = await queryK8sSecretDetail(cluster.value.id, details.secretDetail.value.namespace, details.secretDetail.value.name)
        }
        break
      case 'pvc':
      case 'pv':
        if (details.storageDetail.value?.name) {
          details.storageDetail.value = await queryK8sStorageDetail(
            cluster.value.id,
            details.storageDetail.value.kind,
            details.storageDetail.value.kind === 'PV' ? '' : details.storageDetail.value.namespace,
            details.storageDetail.value.name
          )
        }
        break
    }
  }

  async function submitYAMLUpdate() {
    if (!cluster.value?.id) return
    const target = yamlEditorTarget(yamlEditor.resourceType, yamlEditor.namespace, yamlEditor.name, yamlEditor.workloadType)
    if (!target) {
      // H1 확정(2026-09-10) — pod는 V2 apply 면 밖(opdef uid-신원 제외 원칙).
      // 조회는 유지, 제출은 불가 안내. v1 라우트는 H2에서 삭제된다.
      ElMessage.warning(kt('k8sV2FaceUnavailable'))
      return
    }
    await ElMessageBox.confirm(
      kt('k8sConfirmYamlUpdateMessage', {
        added: String(yamlChangeSummary.value.added),
        removed: String(yamlChangeSummary.value.removed)
      }),
      kt('k8sConfirmYamlUpdateTitle'),
      {
        type: 'warning',
        confirmButtonText: kt('confirmChange'),
        cancelButtonText: kt('cancel')
      }
    )
    yamlSaving.value = true
    try {
      const ok = await runOpTasks(cluster.value.id, kt('k8sOpProgressTitle', { op: kt('k8sOpResourceApply') }), [{
        op: K8S_OPERATIONS.resourceApply,
        target,
        payload: { yaml: yamlEditor.yaml },
        display: `${yamlEditor.namespace || ''}${yamlEditor.namespace ? '/' : ''}${yamlEditor.name}`
      }])
      if (ok) ElMessage.success(kt('k8sYamlUpdatedSuccess'))
      yamlDialogVisible.value = false
      await refreshCurrentYAMLResource()
    } finally {
      yamlSaving.value = false
    }
  }

  return {
    yamlDialogVisible,
    yamlSaving,
    yamlTextareaRef,
    yamlEditor,
    yamlSearch,
    yamlEditorScrollTop,
    yamlCurrentLine,
    yamlDiffLines,
    yamlLineNumbers,
    yamlPreviewLineNumbers,
    yamlChangeSummary,
    yamlCurrentLineOffset,
    yamlResourceLabel,
    runYAMLSearch,
    searchYAMLPrev,
    searchYAMLNext,
    handleYAMLInput,
    updateYAMLCurrentLine,
    handleYAMLScroll,
    submitYAMLUpdate,
    openNamespaceYAML,
    openWorkloadYAML,
    openPodYAML,
    openServiceYAML,
    openIngressYAML,
    openIstioResourceYAML,
    openConfigMapYAML,
    openSecretYAML,
    openStorageYAML
  }
}
