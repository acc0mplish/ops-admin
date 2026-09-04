<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Graph, Shape } from '@antv/x6'
import { queryAssetHostGroupList, queryAssetHostList } from '../../api/asset'
import {
  addOpsJob,
  addOpsJobTemplate,
  opsJobInfo,
  opsJobTemplateInfo,
  queryNotifyRuleOptions,
  queryOpsJobTemplateOptions,
  queryOpsScriptOptions,
  updateOpsJob,
  updateOpsJobTemplate
} from '../../api/ops'
import OpsTargetScope from './components/OpsTargetScope.vue'

const router = useRouter()
const route = useRoute()

const graphContainer = ref()
const loading = ref(false)
const saving = ref(false)
const graphReady = ref(false)
const importDialogVisible = ref(false)
const importTemplateId = ref()
const selectedNodeId = ref('')
const selectedEdgeId = ref('')
const selectedCellIds = ref([])

const scriptOptions = ref([])
const hostOptions = ref([])
const groupOptions = ref([])
const templateOptions = ref([])
const notifyRuleOptions = ref([])

const form = reactive({
  id: undefined,
  name: '',
  description: '',
  status: 1,
  templateId: undefined,
  notifyEnabled: false,
  notifyRuleId: undefined
})

const selectedNodeForm = reactive({
  id: '',
  type: 'script',
  label: '',
  config: {}
})

let graph

const isTemplateMode = computed(() => String(route.query.mode || '') === 'template')
const editorTitle = computed(() => (isTemplateMode.value ? 'Job Template 편집' : 'Job Orchestration'))
const saveButtonText = computed(() => (isTemplateMode.value ? 'Template 저장' : 'Job 저장'))
const selectedCount = computed(() => selectedCellIds.value.length)
const selectedScriptTimeout = computed(() => {
  const script = scriptOptions.value.find((item) => Number(item.id) === Number(selectedNodeForm.config?.scriptId))
  return script?.timeoutSeconds || 300
})
const selectedScriptVariables = computed(() => {
  const script = scriptOptions.value.find((item) => Number(item.id) === Number(selectedNodeForm.config?.scriptId))
  return script?.variables || []
})

function syncSelectedScriptVariables(values = {}) {
  const variables = {}
  selectedScriptVariables.value.forEach((variable) => {
    variables[variable.name] = values[variable.name] ?? (variable.secret ? '' : (variable.defaultValue || ''))
  })
  selectedNodeForm.config = { ...selectedNodeForm.config, variables }
}

function handleSelectedScriptChange() {
  syncSelectedScriptVariables(selectedNodeForm.config?.variables || {})
}

watch(
  () => [selectedNodeForm.label, JSON.stringify(selectedNodeForm.config || {})],
  () => {
    if (!graph || !selectedNodeId.value) return
    const node = graph.getCellById(selectedNodeId.value)
    if (!node) return
    const data = {
      id: selectedNodeForm.id,
      type: selectedNodeForm.type,
      label: selectedNodeForm.label,
      config: cloneValue(selectedNodeForm.config || {})
    }
    node.setData(data)
    node.setAttrs({
      label: {
        text: selectedNodeForm.label || defaultNodeLabel(selectedNodeForm.type)
      }
    })
  },
  { deep: true }
)

watch(
  () => route.fullPath,
  async () => {
    if (!graph) return
    form.id = undefined
    form.name = ''
    form.description = ''
    form.status = 1
    form.templateId = undefined
    form.notifyEnabled = false
    form.notifyRuleId = undefined
    graph.clearCells()
    loadSelectedNode('')
    await loadCurrentRecord()
  }
)

function cloneValue(value) {
  return JSON.parse(JSON.stringify(value))
}

function normalizeTargetIds(value) {
  if (!Array.isArray(value)) return []
  return [...new Set(value.map((item) => Number(item)).filter((item) => item > 0))]
}

function normalizeNodeTargets(config = {}) {
  const normalized = cloneValue(config || {})
  const hostIds = normalizeTargetIds(normalized.hostIds)
  const groupIds = normalizeTargetIds(normalized.groupIds)
  if (hostIds.length) {
    normalized.hostIds = hostIds
    normalized.groupIds = []
  } else {
    normalized.hostIds = []
    normalized.groupIds = groupIds
  }
  return normalized
}

function updateSelectedHostIds(hostIds) {
  const normalizedHostIds = normalizeTargetIds(hostIds)
  selectedNodeForm.config = {
    ...selectedNodeForm.config,
    hostIds: normalizedHostIds,
    groupIds: normalizedHostIds.length ? [] : normalizeTargetIds(selectedNodeForm.config.groupIds)
  }
}

function updateSelectedGroupIds(groupIds) {
  const normalizedGroupIds = normalizeTargetIds(groupIds)
  selectedNodeForm.config = {
    ...selectedNodeForm.config,
    hostIds: normalizedGroupIds.length ? [] : normalizeTargetIds(selectedNodeForm.config.hostIds),
    groupIds: normalizedGroupIds
  }
}

function defaultNodeLabel(type) {
  switch (type) {
    case 'file':
      return '파일 배포'
    case 'approval':
      return 'Manual Approval'
    case 'notify':
      return '메시지 알림'
    default:
      return 'Script 실행'
  }
}

function createDefaultConfig(type) {
  switch (type) {
    case 'file':
      return {
        sourceHostId: undefined,
        sourcePath: '',
        targetPath: '',
        hostIds: [],
        groupIds: [],
        concurrency: 5,
        timeoutSeconds: 30,
        overwrite: true
      }
    case 'approval':
      return {
        message: 'Manual Approval 대기',
        content: '확인 후 해당 Job 실행을 계속하십시오.'
      }
    case 'notify':
      return {
        notifyRuleId: undefined,
        message: 'Job 알림',
        content: 'Job이 메시지 알림 Step에 도달했습니다'
      }
    default:
      return {
        scriptId: undefined,
        variables: {},
        hostIds: [],
        groupIds: [],
        concurrency: 5
      }
  }
}

function buildNodeData(type) {
  const id = `job_node_${Date.now()}_${Math.random().toString(16).slice(2, 6)}`
  return {
    id,
    type,
    label: defaultNodeLabel(type),
    config: createDefaultConfig(type)
  }
}

function createGraphNode(data, position = {}) {
  return graph.addNode({
    id: data.id,
    shape: 'job-step-node',
    x: position.x ?? 80,
    y: position.y ?? 80,
    width: position.width ?? 210,
    height: position.height ?? 72,
    data: cloneValue(data),
    attrs: {
      body: {
        fill: '#ffffff',
        stroke: '#dbe4ff',
        strokeWidth: 1.5,
        rx: 12,
        ry: 12
      },
      label: {
        text: data.label,
        fill: '#1f2a44',
        fontSize: 14,
        fontWeight: 600
      }
    },
    ports: {
      groups: {
        top: {
          position: 'top',
          attrs: {
            circle: {
              r: 5,
              magnet: true,
              stroke: '#4f73ff',
              strokeWidth: 2,
              fill: '#fff'
            }
          }
        },
        bottom: {
          position: 'bottom',
          attrs: {
            circle: {
              r: 5,
              magnet: true,
              stroke: '#4f73ff',
              strokeWidth: 2,
              fill: '#fff'
            }
          }
        }
      },
      items: [{ group: 'top' }, { group: 'bottom' }]
    }
  })
}

function loadSelectedNode(nodeId) {
  selectedNodeId.value = nodeId || ''
  selectedEdgeId.value = ''
  if (!graph || !nodeId) {
    Object.assign(selectedNodeForm, { id: '', type: 'script', label: '', config: {} })
    return
  }
  const node = graph.getCellById(nodeId)
  if (!node) return
  const data = node.getData() || {}
  Object.assign(selectedNodeForm, {
    id: data.id || node.id,
    type: data.type || 'script',
    label: data.label || node.attr('label/text') || '',
    config: cloneValue(data.config || {})
  })
}

function loadSelectedEdge(edgeId) {
  selectedNodeId.value = ''
  selectedEdgeId.value = edgeId || ''
}

function showEdgeTools(edge) {
  if (!edge) return
  edge.addTools([
    { name: 'source-arrowhead' },
    { name: 'target-arrowhead' }
  ])
}

function removeSelectedEdge() {
  if (!graph || !selectedEdgeId.value) return
  const edge = graph.getCellById(selectedEdgeId.value)
  if (!edge?.isEdge?.()) return
  graph.removeCell(edge)
  graph.cleanSelection()
  selectedCellIds.value = []
  loadSelectedEdge('')
  ElMessage.success('연결선을 삭제했습니다.')
}

function syncSelectionState() {
  if (!graph) return
  const selectedCells = graph.getSelectedCells()
  selectedCellIds.value = selectedCells.map((cell) => cell.id)
  const selectedNodes = selectedCells.filter((cell) => cell.isNode())
  if (selectedNodes.length === 1) {
    loadSelectedNode(selectedNodes[0].id)
    return
  }
  const selectedEdges = selectedCells.filter((cell) => cell.isEdge())
  if (selectedEdges.length === 1) {
    loadSelectedEdge(selectedEdges[0].id)
    return
  }
  loadSelectedNode('')
}

function setActiveNode(node) {
  if (!node) {
    selectedCellIds.value = []
    loadSelectedNode('')
    return
  }
  selectedCellIds.value = [node.id]
  loadSelectedNode(node.id)
}

function initGraph() {
  Graph.registerNode(
    'job-step-node',
    {
      inherit: 'rect'
    },
    true
  )
  graph = new Graph({
    container: graphContainer.value,
    grid: {
      size: 16,
      visible: true
    },
    background: {
      color: '#f8fbff'
    },
    panning: true,
    mousewheel: {
      enabled: true,
      modifiers: ['ctrl', 'meta']
    },
    selecting: {
      enabled: true,
      multiple: true,
      rubberband: true,
      filter: ['node', 'edge'],
      showNodeSelectionBox: true,
      showEdgeSelectionBox: true
    },
    connecting: {
      snap: true,
      allowBlank: false,
      allowLoop: false,
      highlight: true,
      connector: 'rounded',
      router: {
        name: 'manhattan'
      },
      createEdge() {
        return new Shape.Edge({
          attrs: {
            line: {
              stroke: '#4f73ff',
              strokeWidth: 2,
              cursor: 'pointer',
              targetMarker: {
                name: 'block',
                width: 12,
                height: 8
              }
            }
          }
        })
      }
    }
  })
  graph.on('cell:click', ({ cell }) => {
    if (cell.isNode()) {
      graph.cleanSelection()
      graph.select(cell)
      setActiveNode(cell)
      return
    }
    graph.cleanSelection()
    graph.select(cell)
    selectedCellIds.value = [cell.id]
    if (cell.isEdge()) {
      loadSelectedEdge(cell.id)
      return
    }
    loadSelectedNode('')
  })
  graph.on('node:selected', ({ node }) => {
    setActiveNode(node)
  })
  graph.on('edge:selected', ({ edge }) => {
    selectedCellIds.value = [edge.id]
    loadSelectedEdge(edge.id)
    showEdgeTools(edge)
  })
  graph.on('edge:click', ({ edge }) => {
    graph.cleanSelection()
    graph.select(edge)
    selectedCellIds.value = [edge.id]
    loadSelectedEdge(edge.id)
    showEdgeTools(edge)
  })
  graph.on('blank:click', () => {
    graph.getEdges().forEach((edge) => edge.removeTools())
    graph.cleanSelection()
    syncSelectionState()
  })
  graph.on('selection:changed', () => syncSelectionState())
  graph.on('edge:connected', ({ edge }) => {
    edge.setAttrs({
      line: {
        stroke: '#4f73ff',
        strokeWidth: 2
      }
    })
    graph.cleanSelection()
    graph.select(edge)
    selectedCellIds.value = [edge.id]
    loadSelectedEdge(edge.id)
    showEdgeTools(edge)
    if (selectedEdgeId.value === edge.id) loadSelectedEdge(edge.id)
  })
  graphReady.value = true
}

function serializeDefinition() {
  const nodes = graph.getNodes().map((node) => {
    const data = node.getData() || {}
    const position = node.position()
    const size = node.size()
    const config = ['script', 'file'].includes(data.type)
      ? normalizeNodeTargets(data.config)
      : cloneValue(data.config || {})
    return {
      id: data.id || node.id,
      type: data.type || 'script',
      label: data.label || node.attr('label/text') || '',
      config,
      meta: {
        x: position.x,
        y: position.y,
        width: size.width,
        height: size.height
      }
    }
  })
  const edges = graph.getEdges()
    .map((edge) => ({
      source: edge.getSourceCellId(),
      target: edge.getTargetCellId()
    }))
    .filter((item) => item.source && item.target)
  return { nodes, edges }
}

function getGraphJson() {
  return JSON.stringify(graph.toJSON())
}

function loadDefinition(definition, graphJson = '') {
  if (!graph) return
  graph.clearCells()
  const positionMap = {}
  try {
    const parsed = JSON.parse(graphJson || '{}')
    for (const cell of parsed.cells || []) {
      if (cell.shape === 'edge') continue
      positionMap[cell.id] = {
        x: cell.x,
        y: cell.y,
        width: cell.width,
        height: cell.height
      }
    }
  } catch (error) {
    console.warn(error)
  }
  let row = 0
  for (const node of definition.nodes || []) {
    const metaPosition = node.meta || {}
    const cell = createGraphNode(node, {
      x: positionMap[node.id]?.x ?? metaPosition.x ?? 80 + (row % 3) * 260,
      y: positionMap[node.id]?.y ?? metaPosition.y ?? 80 + Math.floor(row / 3) * 140,
      width: positionMap[node.id]?.width ?? metaPosition.width ?? 210,
      height: positionMap[node.id]?.height ?? metaPosition.height ?? 72
    })
    cell.setData({
      id: node.id,
      type: node.type,
      label: node.label,
      config: cloneValue(node.config || {})
    })
    row += 1
  }
  for (const edge of definition.edges || []) {
    if (!edge.source || !edge.target) continue
    if (!graph.getCellById(edge.source) || !graph.getCellById(edge.target)) continue
    graph.addEdge({
      source: { cell: edge.source },
      target: { cell: edge.target },
      attrs: {
        line: {
          stroke: '#4f73ff',
          strokeWidth: 2,
          cursor: 'pointer',
          targetMarker: {
            name: 'block',
            width: 12,
            height: 8
          }
        }
      }
    })
  }
  loadSelectedNode('')
}

function addStep(type) {
  if (!graph) return
  const nodeData = buildNodeData(type)
  createGraphNode(nodeData, {
    x: 100 + graph.getNodes().length * 24,
    y: 100 + graph.getNodes().length * 18
  })
  loadSelectedNode(nodeData.id)
}

function removeSelectedNode() {
  if (!graph) return
  const selectedCells = graph.getSelectedCells()
  const cellsToRemove = []
  const appended = new Set()

  const appendCell = (cell) => {
    if (!cell || appended.has(cell.id)) return
    appended.add(cell.id)
    cellsToRemove.push(cell)
  }

  for (const cell of selectedCells) {
    appendCell(cell)
    if (cell.isNode?.()) {
      const edges = graph.getConnectedEdges(cell) || []
      edges.forEach(appendCell)
    }
  }

  if (!cellsToRemove.length && selectedNodeId.value) {
    const node = graph.getCellById(selectedNodeId.value)
    if (node) {
      appendCell(node)
      const edges = graph.getConnectedEdges(node) || []
      edges.forEach(appendCell)
    }
  }

  if (!cellsToRemove.length) return
  graph.removeCells(cellsToRemove)
  graph.cleanSelection()
  syncSelectionState()
}

function handleCanvasKeydown(event) {
  if (!graph) return
  const isDelete = event.key === 'Delete' || event.key === 'Backspace'
  if (!isDelete || !selectedCellIds.value.length) return
  const target = event.target
  const tagName = target?.tagName?.toLowerCase?.() || ''
  const editable = target?.isContentEditable || ['input', 'textarea'].includes(tagName)
  if (editable) return
  event.preventDefault()
  removeSelectedNode()
}

async function clearCanvas() {
  await ElMessageBox.confirm('현재 Orchestration Canvas를 비우시겠습니까?', '알림', { type: 'warning' })
  graph.clearCells()
  loadSelectedNode('')
}

async function loadBaseOptions() {
  const [scripts, hosts, groups, templates, notifyRules] = await Promise.all([
    queryOpsScriptOptions(),
    queryAssetHostList({ pageNum: 1, pageSize: 1000 }),
    queryAssetHostGroupList(),
    queryOpsJobTemplateOptions(),
    queryNotifyRuleOptions({ scope: 'job' })
  ])
  scriptOptions.value = scripts || []
  hostOptions.value = hosts.list || []
  groupOptions.value = groups.tree || []
  templateOptions.value = templates || []
  notifyRuleOptions.value = notifyRules || []
}

async function loadCurrentRecord() {
  const id = Number(route.query.id || 0)
  if (!id) return
  loading.value = true
  try {
    const data = isTemplateMode.value ? await opsJobTemplateInfo(id) : await opsJobInfo(id)
    form.id = data.id
    form.name = data.name || ''
    form.description = data.description || ''
    form.status = data.status || 1
    form.templateId = data.templateId || undefined
    form.notifyEnabled = !!data.notifyEnabled
    form.notifyRuleId = data.notifyRuleId || undefined
    const definition = JSON.parse(data.definitionJson || '{"nodes":[],"edges":[]}')
    loadDefinition(definition, data.graphJson || '')
  } finally {
    loading.value = false
  }
}

async function importTemplate() {
  if (!importTemplateId.value) {
    ElMessage.warning('먼저 Job Template을 선택하십시오.')
    return
  }
  const data = await opsJobTemplateInfo(importTemplateId.value)
  form.templateId = data.id
  if (!form.name.trim()) {
    form.name = data.name || ''
  }
  if (!form.description.trim()) {
    form.description = data.description || ''
  }
  const definition = JSON.parse(data.definitionJson || '{"nodes":[],"edges":[]}')
  loadDefinition(definition, data.graphJson || '')
  importDialogVisible.value = false
  ElMessage.success('Template을 Orchestration Canvas로 가져왔습니다.')
}

async function save() {
  if (!form.name.trim()) {
    ElMessage.warning(isTemplateMode.value ? 'Template 이름을 입력하십시오.' : 'Job 이름을 입력하십시오.')
    return
  }
  const definition = serializeDefinition()
  if (!definition.nodes.length) {
    ElMessage.warning('Step을 하나 이상 추가하십시오.')
    return
  }
  const invalidNotifyNode = definition.nodes.find((node) => node.type === 'notify' && !node.config?.notifyRuleId)
  if (invalidNotifyNode) {
    ElMessage.warning('메시지 알림 Step의 Notification Rule을 선택하십시오.')
    return
  }
  saving.value = true
  try {
    const payload = {
      id: form.id,
      name: form.name,
      description: form.description,
      status: form.status,
      templateId: form.templateId,
      notifyEnabled: false,
      notifyRuleId: undefined,
      graphJson: getGraphJson(),
      definitionJson: JSON.stringify(definition)
    }
    if (isTemplateMode.value) {
      if (form.id) {
        await updateOpsJobTemplate(payload)
      } else {
        await addOpsJobTemplate(payload)
      }
      ElMessage.success('Job Template을 저장했습니다.')
      router.push('/ops/jobs/templates')
      return
    }
    if (form.id) {
      await updateOpsJob(payload)
    } else {
      await addOpsJob(payload)
    }
    ElMessage.success('Job을 저장했습니다.')
    router.push('/ops/jobs/list')
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  await nextTick()
  initGraph()
  window.addEventListener('keydown', handleCanvasKeydown)
  await loadBaseOptions()
  await loadCurrentRecord()
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleCanvasKeydown)
  if (graph) {
    graph.dispose()
    graph = null
  }
})
</script>

<template>
  <div class="ops-job-page">
    <div class="page-card page-head">
      <div>
        <h2 class="page-title">{{ editorTitle }}</h2>
        <p class="page-desc">Flow Orchestration 콘솔 스타일을 참고해 Script 실행, File Distribution, Manual Approval을 재사용 가능한 Job으로 조합할 수 있습니다.</p>
      </div>
      <div class="head-actions">
        <el-button @click="importDialogVisible = true">Job Template 가져오기</el-button>
        <el-button @click="selectedEdgeId ? removeSelectedEdge() : removeSelectedNode()" :disabled="!selectedCount && !selectedNodeId && !selectedEdgeId">
          {{ selectedEdgeId ? '연결선 삭제' : (selectedCount > 1 ? `선택 항목 삭제(${selectedCount})` : '선택한 Step 삭제') }}
        </el-button>
        <el-button @click="clearCanvas" :disabled="!graphReady">Canvas 비우기</el-button>
        <el-button type="primary" :loading="saving" @click="save">{{ saveButtonText }}</el-button>
      </div>
    </div>

    <div class="page-card record-form">
      <el-form label-width="100px">
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item :label="isTemplateMode ? 'Template 이름' : 'Job 이름'" required>
              <el-input v-model="form.name" :placeholder="isTemplateMode ? 'Template 이름을 입력하십시오.' : 'Job 이름을 입력하십시오.'" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="상태">
              <el-radio-group v-model="form.status">
                <el-radio :value="1">활성화</el-radio>
                <el-radio :value="2">비활성화</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="8" v-if="!isTemplateMode">
            <el-form-item label="Source Template">
              <el-select v-model="form.templateId" clearable filterable placeholder="선택 사항">
                <el-option v-for="item in templateOptions" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="설명">
              <el-input v-model="form.description" type="textarea" :rows="2" placeholder="Job 용도, 실행 순서, 위험 안내를 설명하는 데 사용합니다." />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </div>

    <div class="designer-layout">
      <div class="page-card left-palette">
        <div class="panel-title">Step Library</div>
        <el-button class="palette-btn" @click="addStep('script')">Script 실행</el-button>
        <el-button class="palette-btn" @click="addStep('file')">File Distribution</el-button>
        <el-button class="palette-btn" @click="addStep('approval')">Manual Approval</el-button>
        <el-button class="palette-btn" @click="addStep('notify')">메시지 알림</el-button>
        <div class="panel-tip">Node를 드래그해 위치를 조정할 수 있으며 연결선이 실행 순서를 결정합니다.</div>
      </div>

      <div class="page-card canvas-panel">
        <div class="panel-title">Orchestration Canvas</div>
        <div ref="graphContainer" v-loading="loading" class="graph-container" />
      </div>

      <div class="page-card config-panel">
        <div class="panel-title">{{ selectedEdgeId ? '연결선 설정' : 'Step 설정' }}</div>
        <el-empty v-if="!selectedNodeId && !selectedEdgeId" :image-size="68" description="Step Node를 클릭해 Parameter를 편집하거나 연결선을 클릭해 실행 관계를 변경하십시오." />
        <el-empty v-else-if="selectedEdgeId" :image-size="68" description="연결선 양 끝의 핸들을 드래그하면 Step을 다시 연결할 수 있습니다." />
        <el-form v-else-if="selectedNodeId" label-position="top">
          <el-form-item label="Step 이름">
            <el-input v-model="selectedNodeForm.label" />
          </el-form-item>

          <template v-if="selectedNodeForm.type === 'script'">
            <el-form-item label="Script">
              <el-select v-model="selectedNodeForm.config.scriptId" filterable placeholder="Script 선택" @change="handleSelectedScriptChange">
                <el-option v-for="item in scriptOptions" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
            <div class="job-variable-panel">
              <div class="job-variable-panel__title">Step 변수</div>
              <div class="job-variable-panel__hint">실행 시 <code>VARIABLE_변수명</code> 형태로 Script에 주입되며 민감 값은 저장한 뒤 다시 표시되지 않습니다.</div>
              <div v-if="!selectedScriptVariables.length" class="job-variable-panel__empty">이 Script는 변수를 선언하지 않았습니다.</div>
              <div v-else class="job-variable-list">
                <div v-for="variable in selectedScriptVariables" :key="variable.name" class="job-variable-field">
                  <div class="job-variable-field__label"><code>VARIABLE_{{ variable.name }}</code><el-tag v-if="variable.required" size="small" type="danger" effect="plain">필수</el-tag></div>
                  <el-input v-model="selectedNodeForm.config.variables[variable.name]" :type="variable.secret ? 'password' : 'text'" :show-password="variable.secret" :placeholder="variable.secret ? '비워두면 기존 값을 유지합니다' : (variable.defaultValue || '변수 값을 입력하십시오.')" />
                  <div v-if="variable.description" class="job-variable-field__desc">{{ variable.description }}</div>
                </div>
              </div>
            </div>
            <el-form-item label="Concurrency">
              <el-input-number v-model="selectedNodeForm.config.concurrency" :min="1" :max="10" style="width: 100%" />
            </el-form-item>
            <el-form-item label="Script Timeout">
              <el-input :model-value="selectedScriptTimeout" disabled>
                <template #append>s</template>
              </el-input>
            </el-form-item>
            <OpsTargetScope
              :host-options="hostOptions"
              :group-options="groupOptions"
              :host-ids="selectedNodeForm.config.hostIds || []"
              :group-ids="selectedNodeForm.config.groupIds || []"
              @update:host-ids="updateSelectedHostIds"
              @update:group-ids="updateSelectedGroupIds"
            />
          </template>

          <template v-else-if="selectedNodeForm.type === 'file'">
            <el-form-item label="Source Host">
              <el-select v-model="selectedNodeForm.config.sourceHostId" filterable placeholder="Source Host 선택">
                <el-option v-for="item in hostOptions" :key="item.id" :label="item.hostName" :value="item.id" />
              </el-select>
            </el-form-item>
            <el-form-item label="Source File Path">
              <el-input v-model="selectedNodeForm.config.sourcePath" placeholder="/opt/app/config.yml" />
            </el-form-item>
            <el-form-item label="Target Path">
              <el-input v-model="selectedNodeForm.config.targetPath" placeholder="/opt/app/config.yml" />
            </el-form-item>
            <el-form-item label="Concurrency">
              <el-input-number v-model="selectedNodeForm.config.concurrency" :min="1" :max="10" style="width: 100%" />
            </el-form-item>
            <el-form-item label="Timeout(초)">
              <el-input-number v-model="selectedNodeForm.config.timeoutSeconds" :min="10" :max="3600" style="width: 100%" />
            </el-form-item>
            <el-form-item label="Target File 덮어쓰기">
              <el-switch v-model="selectedNodeForm.config.overwrite" />
            </el-form-item>
            <OpsTargetScope
              :host-options="hostOptions"
              :group-options="groupOptions"
              :host-ids="selectedNodeForm.config.hostIds || []"
              :group-ids="selectedNodeForm.config.groupIds || []"
              @update:host-ids="updateSelectedHostIds"
              @update:group-ids="updateSelectedGroupIds"
            />
          </template>
          <template v-else-if="selectedNodeForm.type === 'notify'">
            <el-form-item label="Notification Rule" required>
              <el-select v-model="selectedNodeForm.config.notifyRuleId" filterable placeholder="Notification Rule 선택">
                <el-option v-for="item in notifyRuleOptions" :key="item.id" :label="`${item.name} · Job Orchestration`" :value="item.id" />
              </el-select>
              <div class="form-tip">Job Orchestration용 Notification Rule만 표시합니다. 카드에 Job과 현재 Step 정보가 자동으로 포함됩니다.</div>
            </el-form-item>
            <el-form-item label="알림 요약">
              <el-input v-model="selectedNodeForm.config.message" placeholder="예: 배포 완료 알림" />
            </el-form-item>
            <el-form-item label="알림 내용">
              <el-input v-model="selectedNodeForm.config.content" type="textarea" :rows="6" placeholder="이 값은 {{detail}} 변수로 메시지 Template에 전달됩니다" />
            </el-form-item>
          </template>

          <template v-else>
            <el-form-item label="확인 메시지">
              <el-input v-model="selectedNodeForm.config.message" placeholder="예: 유지보수 Window가 완료되었는지 확인하십시오." />
            </el-form-item>
            <el-form-item label="확인 설명">
              <el-input v-model="selectedNodeForm.config.content" type="textarea" :rows="6" placeholder="여기에 Manual Approval Node의 작업 설명, 주의 사항, 진행 조건을 입력하십시오." />
            </el-form-item>
          </template>
        </el-form>
      </div>
    </div>

    <el-dialog v-model="importDialogVisible" title="Job Template 가져오기" width="520px">
      <el-form label-width="90px">
        <el-form-item label="Job Template">
          <el-select v-model="importTemplateId" filterable placeholder="가져올 Template 선택" style="width: 100%">
            <el-option v-for="item in templateOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="importDialogVisible = false">취소</el-button>
        <el-button type="primary" @click="importTemplate">가져오기</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.ops-job-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.page-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.page-title {
  margin: 0 0 8px;
  font-size: 28px;
  font-weight: 700;
  color: #14213d;
}

.page-desc {
  margin: 0;
  color: #7485a7;
}

.head-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.designer-layout {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr) 380px;
  gap: 16px;
  min-height: 720px;
}

.left-palette,
.canvas-panel,
.config-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.panel-title {
  font-size: 18px;
  font-weight: 700;
  color: #14213d;
}

.palette-btn {
  justify-content: flex-start;
  height: 44px;
  border-radius: 10px;
}

.panel-tip {
  font-size: 12px;
  line-height: 1.7;
  color: #7d8cad;
}

.form-tip {
  margin-top: 6px;
  color: #8491a9;
  font-size: 12px;
  line-height: 1.6;
}

.job-variable-panel { padding: 13px; border: 1px solid #d9e6ff; border-radius: 10px; background: linear-gradient(135deg, #f8fbff, #fff); }
.job-variable-panel__title { color: #172744; font-size: 14px; font-weight: 700; }
.job-variable-panel__hint, .job-variable-field__desc { margin-top: 4px; color: #7282a0; font-size: 12px; line-height: 1.55; }
.job-variable-panel code, .job-variable-field__label code { color: #3869d9; }
.job-variable-panel__empty { margin-top: 12px; padding: 9px; color: #8190aa; border: 1px dashed #cbdcff; border-radius: 7px; font-size: 12px; }
.job-variable-list { display: grid; gap: 12px; margin-top: 12px; }
.job-variable-field__label { display: flex; align-items: center; gap: 7px; margin-bottom: 6px; font-size: 12px; font-weight: 600; }

.graph-container {
  width: 100%;
  min-height: 640px;
  border: 1px solid #dbe4ff;
  border-radius: 14px;
  background: linear-gradient(180deg, #fbfdff 0%, #f6f9ff 100%);
}

@media (max-width: 1440px) {
  .designer-layout {
    grid-template-columns: 200px minmax(0, 1fr) 340px;
  }
}
</style>
