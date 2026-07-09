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
const editorTitle = computed(() => (isTemplateMode.value ? '作业模板编排' : '作业编排'))
const saveButtonText = computed(() => (isTemplateMode.value ? '保存模板' : '保存作业'))
const selectedCount = computed(() => selectedCellIds.value.length)
const selectedScriptTimeout = computed(() => {
  const script = scriptOptions.value.find((item) => Number(item.id) === Number(selectedNodeForm.config?.scriptId))
  return script?.timeoutSeconds || 300
})

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
      return '\u6587\u4ef6\u5206\u53d1'
    case 'approval':
      return '\u4eba\u5de5\u786e\u8ba4'
    case 'notify':
      return '\u6d88\u606f\u901a\u77e5'
    default:
      return '\u811a\u672c\u6267\u884c'
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
        message: '\u7b49\u5f85\u4eba\u5de5\u786e\u8ba4',
        content: '\u8bf7\u786e\u8ba4\u540e\u7ee7\u7eed\u6267\u884c\u8be5\u4f5c\u4e1a'
      }
    case 'notify':
      return {
        notifyRuleId: undefined,
        message: '\u4f5c\u4e1a\u901a\u77e5',
        content: '\u4f5c\u4e1a\u6267\u884c\u5230\u6d88\u606f\u901a\u77e5\u6b65\u9aa4'
      }
    default:
      return {
        scriptId: undefined,
        parameters: '',
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

function syncSelectionState() {
  if (!graph) return
  const selectedCells = graph.getSelectedCells()
  selectedCellIds.value = selectedCells.map((cell) => cell.id)
  const selectedNodes = selectedCells.filter((cell) => cell.isNode())
  if (selectedNodes.length === 1) {
    loadSelectedNode(selectedNodes[0].id)
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
    loadSelectedNode('')
  })
  graph.on('node:selected', ({ node }) => {
    setActiveNode(node)
  })
  graph.on('edge:selected', ({ edge }) => {
    selectedCellIds.value = [edge.id]
    loadSelectedNode('')
  })
  graph.on('blank:click', () => {
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
  await ElMessageBox.confirm('确认清空当前编排画布吗？', '提示', { type: 'warning' })
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
    ElMessage.warning('请先选择作业模板')
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
  ElMessage.success('模板已导入到编排画布')
}

async function save() {
  if (!form.name.trim()) {
    ElMessage.warning(isTemplateMode.value ? '请输入模板名称' : '请输入作业名称')
    return
  }
  const definition = serializeDefinition()
  if (!definition.nodes.length) {
    ElMessage.warning('请至少添加一个步骤')
    return
  }
  const invalidNotifyNode = definition.nodes.find((node) => node.type === 'notify' && !node.config?.notifyRuleId)
  if (invalidNotifyNode) {
    ElMessage.warning('\u8bf7\u4e3a\u6d88\u606f\u901a\u77e5\u6b65\u9aa4\u9009\u62e9\u901a\u77e5\u89c4\u5219')
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
      ElMessage.success('作业模板已保存')
      router.push('/ops/jobs/templates')
      return
    }
    if (form.id) {
      await updateOpsJob(payload)
    } else {
      await addOpsJob(payload)
    }
    ElMessage.success('作业已保存')
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
        <p class="page-desc">参考流程编排台风格，支持将脚本执行、文件分发和人工确认组合成可复用作业。</p>
      </div>
      <div class="head-actions">
        <el-button @click="importDialogVisible = true">导入作业模板</el-button>
        <el-button @click="removeSelectedNode" :disabled="!selectedCount && !selectedNodeId">
          {{ selectedCount > 1 ? `删除选中项（${selectedCount}）` : '删除选中步骤' }}
        </el-button>
        <el-button @click="clearCanvas" :disabled="!graphReady">清空画布</el-button>
        <el-button type="primary" :loading="saving" @click="save">{{ saveButtonText }}</el-button>
      </div>
    </div>

    <div class="page-card record-form">
      <el-form label-width="100px">
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item :label="isTemplateMode ? '模板名称' : '作业名称'" required>
              <el-input v-model="form.name" :placeholder="isTemplateMode ? '请输入模板名称' : '请输入作业名称'" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="状态">
              <el-radio-group v-model="form.status">
                <el-radio :value="1">启用</el-radio>
                <el-radio :value="2">禁用</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="8" v-if="!isTemplateMode">
            <el-form-item label="来源模板">
              <el-select v-model="form.templateId" clearable filterable placeholder="可选">
                <el-option v-for="item in templateOptions" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="描述">
              <el-input v-model="form.description" type="textarea" :rows="2" placeholder="用于说明作业用途、执行顺序与风险提示" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </div>

    <div class="designer-layout">
      <div class="page-card left-palette">
        <div class="panel-title">步骤库</div>
        <el-button class="palette-btn" @click="addStep('script')">脚本执行</el-button>
        <el-button class="palette-btn" @click="addStep('file')">文件分发</el-button>
        <el-button class="palette-btn" @click="addStep('approval')">人工确认</el-button>
        <el-button class="palette-btn" @click="addStep('notify')">&#x6d88;&#x606f;&#x901a;&#x77e5;</el-button>
        <div class="panel-tip">拖动节点可调整位置，连线决定执行顺序。</div>
      </div>

      <div class="page-card canvas-panel">
        <div class="panel-title">编排画布</div>
        <div ref="graphContainer" v-loading="loading" class="graph-container" />
      </div>

      <div class="page-card config-panel">
        <div class="panel-title">步骤配置</div>
        <el-empty v-if="!selectedNodeId" :image-size="68" description="点击画布中的步骤节点后，在这里编辑参数" />
        <el-form v-else label-position="top">
          <el-form-item label="步骤名称">
            <el-input v-model="selectedNodeForm.label" />
          </el-form-item>

          <template v-if="selectedNodeForm.type === 'script'">
            <el-form-item label="脚本">
              <el-select v-model="selectedNodeForm.config.scriptId" filterable placeholder="选择脚本">
                <el-option v-for="item in scriptOptions" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
            <el-form-item label="执行参数">
              <el-input v-model="selectedNodeForm.config.parameters" placeholder="例如：--env prod" />
            </el-form-item>
            <el-form-item label="并发数">
              <el-input-number v-model="selectedNodeForm.config.concurrency" :min="1" :max="10" style="width: 100%" />
            </el-form-item>
            <el-form-item label="脚本超时">
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
            <el-form-item label="源主机">
              <el-select v-model="selectedNodeForm.config.sourceHostId" filterable placeholder="选择源主机">
                <el-option v-for="item in hostOptions" :key="item.id" :label="item.hostName" :value="item.id" />
              </el-select>
            </el-form-item>
            <el-form-item label="源文件路径">
              <el-input v-model="selectedNodeForm.config.sourcePath" placeholder="/opt/app/config.yml" />
            </el-form-item>
            <el-form-item label="目标路径">
              <el-input v-model="selectedNodeForm.config.targetPath" placeholder="/opt/app/config.yml" />
            </el-form-item>
            <el-form-item label="并发数">
              <el-input-number v-model="selectedNodeForm.config.concurrency" :min="1" :max="10" style="width: 100%" />
            </el-form-item>
            <el-form-item label="超时秒数">
              <el-input-number v-model="selectedNodeForm.config.timeoutSeconds" :min="10" :max="3600" style="width: 100%" />
            </el-form-item>
            <el-form-item label="覆盖目标文件">
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
            <el-form-item label="&#x901a;&#x77e5;&#x89c4;&#x5219;" required>
              <el-select v-model="selectedNodeForm.config.notifyRuleId" filterable placeholder="&#x9009;&#x62e9;&#x901a;&#x77e5;&#x89c4;&#x5219;">
                <el-option v-for="item in notifyRuleOptions" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
            <el-form-item label="&#x901a;&#x77e5;&#x6458;&#x8981;">
              <el-input v-model="selectedNodeForm.config.message" placeholder="&#x4f8b;&#x5982;&#xff1a;&#x53d1;&#x5e03;&#x5b8c;&#x6210;&#x901a;&#x77e5;" />
            </el-form-item>
            <el-form-item label="&#x901a;&#x77e5;&#x5185;&#x5bb9;">
              <el-input v-model="selectedNodeForm.config.content" type="textarea" :rows="6" placeholder="&#x8fd9;&#x91cc;&#x4f1a;&#x4f5c;&#x4e3a; {{detail}} &#x53d8;&#x91cf;&#x4f20;&#x7ed9;&#x6d88;&#x606f;&#x6a21;&#x677f;" />
            </el-form-item>
          </template>

          <template v-else>
            <el-form-item label="确认提示">
              <el-input v-model="selectedNodeForm.config.message" placeholder="例如：请确认窗口维护已完成" />
            </el-form-item>
            <el-form-item label="确认说明">
              <el-input v-model="selectedNodeForm.config.content" type="textarea" :rows="6" placeholder="在这里填写人工确认节点的操作说明、注意事项和放行条件" />
            </el-form-item>
          </template>
        </el-form>
      </div>
    </div>

    <el-dialog v-model="importDialogVisible" title="导入作业模板" width="520px">
      <el-form label-width="90px">
        <el-form-item label="作业模板">
          <el-select v-model="importTemplateId" filterable placeholder="选择要导入的模板" style="width: 100%">
            <el-option v-for="item in templateOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="importDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="importTemplate">导入</el-button>
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
