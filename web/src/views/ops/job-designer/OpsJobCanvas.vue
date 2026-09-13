<script setup>
// I5-J (i5-plan §3.8·CW-4) — OpsJobDesigner.vue 1,009행 분할 자식(오케스트레이션
// 캔버스 — X6 그래프 전담). 원본 좌표: 템플릿 :793-796(canvas-panel)·watch
// (선택 폼→노드 동기화) :84-104·cloneValue :123-125·defaultNodeLabel :164-175·
// createDefaultConfig :177-210·buildNodeData :212-220·createGraphNode :222-276·
// loadSelectedNode :278-294·loadSelectedEdge :296-299·showEdgeTools :301-307·
// removeSelectedEdge :309-318·syncSelectionState :320-335·setActiveNode
// :337-345·initGraph :347-456·serializeDefinition :458-486·getGraphJson
// :488-490·loadDefinition :492-548·addStep :550-558·removeSelectedNode
// :560-593·handleCanvasKeydown :595-605·clearCanvas :607-611·onMounted 그래프
// 분 :717-722·onBeforeUnmount :725-731·CSS .canvas-panel :955-961 분해·
// .panel-title :963-967(공유 복사)·.graph-container :996-1002.
// 그래프 변수(graph)와 graphContainer·키 리스너는 본 자식 귀속. 선택 상태
// (selectedNodeId·selectedEdgeId·selectedCellIds·selectedNodeForm)는 부모·형제
// (노드 폼)가 공유해 page로 재배선한다(§5 #11).
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Graph, Shape } from '@antv/x6'
import { ot } from '../../../utils/ops-i18n'

const props = defineProps({
  page: {
    type: Object,
    required: true
  }
})

const graphContainer = ref()

let graph

// 원본 :84-104 — 선택 폼 변경을 그래프 노드에 반영(선택 폼은 page 공유)
watch(
  () => [props.page.selectedNodeForm.label, JSON.stringify(props.page.selectedNodeForm.config || {})],
  () => {
    if (!graph || !props.page.selectedNodeId) return
    const node = graph.getCellById(props.page.selectedNodeId)
    if (!node) return
    const data = {
      id: props.page.selectedNodeForm.id,
      type: props.page.selectedNodeForm.type,
      label: props.page.selectedNodeForm.label,
      config: cloneValue(props.page.selectedNodeForm.config || {})
    }
    node.setData(data)
    node.setAttrs({
      label: {
        text: props.page.selectedNodeForm.label || defaultNodeLabel(props.page.selectedNodeForm.type)
      }
    })
  },
  { deep: true }
)

// 원본 :123-125
function cloneValue(value) {
  return JSON.parse(JSON.stringify(value))
}

// 원본 :164-175
function defaultNodeLabel(type) {
  switch (type) {
    case 'file':
      return ot('nodeFileDeploy')
    case 'approval':
      return 'Manual Approval'
    case 'notify':
      return ot('messageNotify')
    default:
      return ot('scriptExecution')
  }
}

// 원본 :177-210
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
        message: ot('approvalNodeMessage'),
        content: ot('approvalNodeContent')
      }
    case 'notify':
      return {
        notifyRuleId: undefined,
        message: ot('notifyNodeMessage'),
        content: ot('notifyNodeContent')
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

// 원본 :212-220
function buildNodeData(type) {
  const id = `job_node_${Date.now()}_${Math.random().toString(16).slice(2, 6)}`
  return {
    id,
    type,
    label: defaultNodeLabel(type),
    config: createDefaultConfig(type)
  }
}

// 원본 :222-276
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

// 원본 :278-294
function loadSelectedNode(nodeId) {
  props.page.selectedNodeId = nodeId || ''
  props.page.selectedEdgeId = ''
  if (!graph || !nodeId) {
    Object.assign(props.page.selectedNodeForm, { id: '', type: 'script', label: '', config: {} })
    return
  }
  const node = graph.getCellById(nodeId)
  if (!node) return
  const data = node.getData() || {}
  Object.assign(props.page.selectedNodeForm, {
    id: data.id || node.id,
    type: data.type || 'script',
    label: data.label || node.attr('label/text') || '',
    config: cloneValue(data.config || {})
  })
}

// 원본 :296-299
function loadSelectedEdge(edgeId) {
  props.page.selectedNodeId = ''
  props.page.selectedEdgeId = edgeId || ''
}

// 원본 :301-307
function showEdgeTools(edge) {
  if (!edge) return
  edge.addTools([
    { name: 'source-arrowhead' },
    { name: 'target-arrowhead' }
  ])
}

// 원본 :309-318
function removeSelectedEdge() {
  if (!graph || !props.page.selectedEdgeId) return
  const edge = graph.getCellById(props.page.selectedEdgeId)
  if (!edge?.isEdge?.()) return
  graph.removeCell(edge)
  graph.cleanSelection()
  props.page.selectedCellIds = []
  loadSelectedEdge('')
  ElMessage.success(ot('edgeDeleted'))
}

// 원본 :320-335
function syncSelectionState() {
  if (!graph) return
  const selectedCells = graph.getSelectedCells()
  props.page.selectedCellIds = selectedCells.map((cell) => cell.id)
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

// 원본 :337-345
function setActiveNode(node) {
  if (!node) {
    props.page.selectedCellIds = []
    loadSelectedNode('')
    return
  }
  props.page.selectedCellIds = [node.id]
  loadSelectedNode(node.id)
}

// 원본 :347-456
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
    props.page.selectedCellIds = [cell.id]
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
    props.page.selectedCellIds = [edge.id]
    loadSelectedEdge(edge.id)
    showEdgeTools(edge)
  })
  graph.on('edge:click', ({ edge }) => {
    graph.cleanSelection()
    graph.select(edge)
    props.page.selectedCellIds = [edge.id]
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
    props.page.selectedCellIds = [edge.id]
    loadSelectedEdge(edge.id)
    showEdgeTools(edge)
    if (props.page.selectedEdgeId === edge.id) loadSelectedEdge(edge.id)
  })
  props.page.graphReady = true
}

// 원본 :458-486
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

// 원본 :132-144 normalizeNodeTargets — serializeDefinition 전용(자식 귀속).
// normalizeTargetIds는 부모 귀속(page 공유 — 노드 폼 자식과 공통)
function normalizeNodeTargets(config = {}) {
  const normalized = cloneValue(config || {})
  const hostIds = props.page.normalizeTargetIds(normalized.hostIds)
  const groupIds = props.page.normalizeTargetIds(normalized.groupIds)
  if (hostIds.length) {
    normalized.hostIds = hostIds
    normalized.groupIds = []
  } else {
    normalized.hostIds = []
    normalized.groupIds = groupIds
  }
  return normalized
}

// 원본 :488-490
function getGraphJson() {
  return JSON.stringify(graph.toJSON())
}

// 원본 :492-548
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

// 원본 :550-558
function addStep(type) {
  if (!graph) return
  const nodeData = buildNodeData(type)
  createGraphNode(nodeData, {
    x: 100 + graph.getNodes().length * 24,
    y: 100 + graph.getNodes().length * 18
  })
  loadSelectedNode(nodeData.id)
}

// 원본 :560-593
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

  if (!cellsToRemove.length && props.page.selectedNodeId) {
    const node = graph.getCellById(props.page.selectedNodeId)
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

// 원본 :595-605
function handleCanvasKeydown(event) {
  if (!graph) return
  const isDelete = event.key === 'Delete' || event.key === 'Backspace'
  if (!isDelete || !props.page.selectedCellIds.length) return
  const target = event.target
  const tagName = target?.tagName?.toLowerCase?.() || ''
  const editable = target?.isContentEditable || ['input', 'textarea'].includes(tagName)
  if (editable) return
  event.preventDefault()
  removeSelectedNode()
}

// 원본 :607-611
async function clearCanvas() {
  await ElMessageBox.confirm(ot('clearCanvasConfirm'), ot('noticeTitle'), { type: 'warning' })
  graph.clearCells()
  loadSelectedNode('')
}

// 원본 watch(route.fullPath) :117의 graph.clearCells() 대체 위임용 얇은 래퍼
function clearCells() {
  graph?.clearCells()
}

// 원본 :717-722 그래프 분 — nextTick 후 그래프 초기화·키 리스너 등록.
// 자식 onMounted는 부모 onMounted보다 먼저 같은 마운트 플러시 내에서 실행된다.
onMounted(async () => {
  await nextTick()
  initGraph()
  window.addEventListener('keydown', handleCanvasKeydown)
})

// 원본 :725-731
onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleCanvasKeydown)
  if (graph) {
    graph.dispose()
    graph = null
  }
})

defineExpose({
  addStep,
  removeSelectedNode,
  removeSelectedEdge,
  clearCanvas,
  clearCells,
  loadSelectedNode,
  loadDefinition,
  serializeDefinition,
  getGraphJson
})
</script>

<template>
  <div class="page-card canvas-panel">
    <div class="panel-title">Orchestration Canvas</div>
    <div ref="graphContainer" v-loading="page.loading" class="graph-container" />
  </div>
</template>

<style scoped>
/* 원본 :955-961 합성 셀렉터 분해 — .canvas-panel 소속(자식 루트).
   .left-palette은 부모·.config-panel은 OpsJobNodeForm 자식 */
.canvas-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

/* 원본 :963-967 — left-palette(부모)·config-panel(OpsJobNodeForm)과 공유 복사 */
.panel-title {
  font-size: 18px;
  font-weight: 700;
  color: #14213d;
}

/* 원본 :996-1002 */
.graph-container {
  width: 100%;
  min-height: 640px;
  border: 1px solid #dbe4ff;
  border-radius: 14px;
  background: linear-gradient(180deg, #fbfdff 0%, #f6f9ff 100%);
}
</style>
