<template>
  <div class="topology-page" v-loading="loading">
    <section class="topology-header">
      <div class="header-copy">
        <div class="topology-mark">AT</div>
        <div>
          <p class="eyebrow">APPLICATION TOPOLOGY</p>
          <h1>应用拓扑</h1>
          <p>从应用出发，追踪运行资源、交付链路与告警状态。</p>
        </div>
      </div>
      <div class="filters">
        <el-select v-model="query.appId" clearable filterable placeholder="选择应用" @change="loadData">
          <el-option v-for="item in appOptions" :key="item.id" :label="item.name" :value="item.id" />
        </el-select>
        <el-select v-model="query.env" clearable placeholder="全部环境" @change="loadData">
          <el-option v-for="item in environments" :key="item.code" :label="`${item.name} / ${item.code}`" :value="item.code" />
        </el-select>
        <el-button :icon="Refresh" @click="loadData">刷新</el-button>
      </div>
    </section>

    <section class="summary-grid" aria-label="应用资源概览">
      <div v-for="item in summaryCards" :key="item.label" class="summary-card" :class="`tone-${item.tone}`">
        <div class="summary-label">
          <i></i>
          <span>{{ item.label }}</span>
          <em>{{ item.code }}</em>
        </div>
        <div class="summary-value">
          <strong>{{ item.value }}</strong>
          <small>{{ item.note }}</small>
        </div>
      </div>
    </section>

    <section ref="workspaceRef" class="topology-workspace">
      <header class="workspace-toolbar">
        <div>
          <h2>资源关系图</h2>
          <p><span class="live-dot"></span>{{ selectedAppLabel }} · {{ selectedEnvLabel }}</p>
        </div>
        <div class="canvas-actions">
          <el-tooltip content="缩小" placement="top"><el-button :icon="ZoomOut" circle @click="zoom(-0.1)" /></el-tooltip>
          <span class="zoom-value">{{ zoomPercent }}%</span>
          <el-tooltip content="放大" placement="top"><el-button :icon="ZoomIn" circle @click="zoom(0.1)" /></el-tooltip>
          <el-tooltip content="适配画布" placement="top"><el-button :icon="Aim" circle @click="fitCanvas" /></el-tooltip>
          <el-tooltip content="全屏" placement="top"><el-button :icon="FullScreen" circle @click="toggleFullscreen" /></el-tooltip>
        </div>
      </header>

      <div class="workspace-body" :class="{ 'has-detail': activeNode }">
        <div class="canvas-shell">
          <div class="canvas-legend">
            <span><i class="legend-dot application"></i>应用</span>
            <span><i class="legend-dot resource"></i>运行资源</span>
            <span><i class="legend-dot delivery"></i>交付</span>
            <span><i class="legend-dot alert"></i>告警</span>
          </div>
          <div ref="graphContainer" class="topology-graph"></div>
          <div v-if="!data.app?.id" class="canvas-empty">
            <el-empty description="选择一个应用后查看完整资源关系" :image-size="88" />
          </div>
        </div>

        <aside v-if="activeNode" class="detail-panel">
          <template>
            <div class="detail-heading">
              <div class="detail-type" :class="`tone-${activeNode.tone}`">{{ activeNode.short }}</div>
              <div>
                <span>{{ activeNode.eyebrow }}</span>
                <h3>{{ activeNode.title }}</h3>
              </div>
              <el-button class="detail-close" text circle aria-label="关闭节点详情" @click="closeNodeDetail">
                <el-icon><Close /></el-icon>
              </el-button>
            </div>
            <div class="detail-metric">
              <span>关联数量</span>
              <strong>{{ activeNode.count }}</strong>
            </div>
            <div class="detail-list">
              <div v-for="(item, index) in activeNode.items" :key="item.id || index" class="detail-item">
                <i :class="statusClass(item, activeNode.type)"></i>
                <div>
                  <strong>{{ itemTitle(item, activeNode.type) }}</strong>
                  <span>{{ itemMeta(item, activeNode.type) }}</span>
                </div>
              </div>
              <el-empty v-if="!activeNode.items.length" description="暂无关联数据" :image-size="64" />
            </div>
          </template>
        </aside>
      </div>
    </section>

    <section class="activity-grid">
      <div class="activity-panel">
        <div class="panel-title">
          <div><span class="title-accent delivery"></span><h2>近期发布</h2></div>
          <span>最近 {{ data.releases?.length || 0 }} 条</span>
        </div>
        <el-table :data="data.releases || []" height="270" empty-text="暂无发布记录">
          <el-table-column prop="version" label="版本" min-width="140" show-overflow-tooltip />
          <el-table-column prop="env" label="环境" width="90" />
          <el-table-column label="状态" width="96">
            <template #default="scope"><el-tag :type="statusTagType(scope.row.status)" effect="light">{{ scope.row.status || '-' }}</el-tag></template>
          </el-table-column>
          <el-table-column prop="stage" label="阶段" width="110" />
          <el-table-column prop="createTime" label="时间" min-width="170" show-overflow-tooltip />
        </el-table>
      </div>
      <div class="activity-panel">
        <div class="panel-title">
          <div><span class="title-accent pipeline"></span><h2>流水线执行</h2></div>
          <span>最近 {{ data.pipelineRuns?.length || 0 }} 条</span>
        </div>
        <el-table :data="data.pipelineRuns || []" height="270" empty-text="暂无流水线记录">
          <el-table-column prop="pipelineName" label="流水线" min-width="150" show-overflow-tooltip />
          <el-table-column prop="env" label="环境" width="90" />
          <el-table-column prop="imageTag" label="镜像 Tag" min-width="130" show-overflow-tooltip />
          <el-table-column label="状态" width="96">
            <template #default="scope"><el-tag :type="statusTagType(scope.row.status)" effect="light">{{ scope.row.status || '-' }}</el-tag></template>
          </el-table-column>
          <el-table-column prop="createTime" label="时间" min-width="170" show-overflow-tooltip />
        </el-table>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  Aim,
  Close,
  FullScreen,
  Refresh,
  ZoomIn,
  ZoomOut
} from '@element-plus/icons-vue'
import { Graph } from '@antv/x6'
import { queryOpsApplicationOptions, queryOpsApplicationTopology, queryOpsEnvironmentList } from '../../api/ops'

const query = reactive({ appId: undefined, env: '' })
const appOptions = ref([])
const environments = ref([])
const data = ref({})
const loading = ref(false)
const graphContainer = ref(null)
const workspaceRef = ref(null)
const activeNode = ref(null)
const zoomPercent = ref(100)
let graph
let resizeObserver

const selectedAppLabel = computed(() => data.value.app?.name || '未选择应用')
const selectedEnvLabel = computed(() => {
  if (!query.env) return '全部环境'
  return environments.value.find((item) => item.code === query.env)?.name || query.env
})

const summaryCards = computed(() => {
  const summary = data.value.summary || {}
  return [
    { label: '关联主机', code: 'HOST', value: summary.hosts || 0, note: '计算与运行节点', tone: 'blue' },
    { label: 'K8s 集群', code: 'K8S', value: summary.k8sClusters || 0, note: '容器运行环境', tone: 'violet' },
    { label: '数据库', code: 'DB', value: summary.databases || 0, note: '应用数据依赖', tone: 'green' },
    { label: '活跃告警', code: 'ALERT', value: summary.alerts || 0, note: '关联监控事件', tone: 'red' },
    { label: '发布记录', code: 'RELEASE', value: summary.releases || 0, note: '版本变更轨迹', tone: 'amber' },
    { label: '流水线', code: 'PIPELINE', value: summary.pipelineRuns || 0, note: '自动交付执行', tone: 'cyan' }
  ]
})

const nodePalette = {
  application: { stroke: '#2563eb', fill: '#eff6ff', accent: '#2563eb', eyebrow: '#1d4ed8' },
  host: { stroke: '#3b82f6', fill: '#f0f7ff', accent: '#3b82f6', eyebrow: '#2563eb' },
  k8s: { stroke: '#7c3aed', fill: '#f5f3ff', accent: '#7c3aed', eyebrow: '#6d28d9' },
  database: { stroke: '#059669', fill: '#ecfdf5', accent: '#10b981', eyebrow: '#047857' },
  pipeline: { stroke: '#0891b2', fill: '#ecfeff', accent: '#06b6d4', eyebrow: '#0e7490' },
  release: { stroke: '#d97706', fill: '#fffbeb', accent: '#f59e0b', eyebrow: '#b45309' },
  alert: { stroke: '#e11d48', fill: '#fff1f2', accent: '#f43f5e', eyebrow: '#be123c' }
}

function registerTopologyNode() {
  Graph.registerNode('archify-topology-node', {
    inherit: 'rect',
    width: 236,
    height: 102,
    markup: [
      { tagName: 'rect', selector: 'body' },
      { tagName: 'rect', selector: 'accent' },
      { tagName: 'text', selector: 'eyebrow' },
      { tagName: 'text', selector: 'title' },
      { tagName: 'text', selector: 'meta' },
      { tagName: 'text', selector: 'count' }
    ],
    attrs: {
      body: { rx: 8, ry: 8, strokeWidth: 1.4 },
      accent: { x: 0, y: 0, width: 5, height: 102, rx: 3, ry: 3, stroke: 'none' },
      eyebrow: { refX: 20, refY: 24, fontSize: 10, fontWeight: 700, letterSpacing: 1.1, textAnchor: 'start' },
      title: { refX: 20, refY: 51, fontSize: 16, fontWeight: 700, fill: '#10213f', textAnchor: 'start' },
      meta: { refX: 20, refY: 77, fontSize: 11, fill: '#71809c', textAnchor: 'start' },
      count: { refX: 216, refY: 51, fontSize: 23, fontWeight: 800, fill: '#10213f', textAnchor: 'end' }
    }
  }, true)
}

function initGraph() {
  registerTopologyNode()
  graph = new Graph({
    container: graphContainer.value,
    autoResize: true,
    grid: { size: 16, visible: true, type: 'dot', args: { color: '#dbe6f5', thickness: 1 } },
    background: { color: '#fbfdff' },
    panning: true,
    interacting: { nodeMovable: true, edgeMovable: false },
    mousewheel: { enabled: true, modifiers: ['ctrl', 'meta'], minScale: 0.6, maxScale: 1.6 },
    selecting: { enabled: true, multiple: false, showNodeSelectionBox: true }
  })
  graph.on('node:click', ({ node }) => {
    const nodeData = node.getData()
    if (nodeData?.isBoundary) return
    graph.cleanSelection()
    graph.select(node)
    activeNode.value = nodeData
  })
  graph.on('blank:click', () => {
    graph.cleanSelection()
    activeNode.value = null
  })
  graph.on('scale', ({ sx }) => {
    zoomPercent.value = Math.round(sx * 100)
  })
  resizeObserver = new ResizeObserver(() => graph?.resize())
  resizeObserver.observe(graphContainer.value)
}

function addBoundary(id, label, x, y, width, height, color) {
  graph.addNode({
    id,
    shape: 'rect',
    x,
    y,
    width,
    height,
    zIndex: 0,
    selectable: false,
    movable: false,
    data: { isBoundary: true },
    attrs: {
      body: { fill: '#ffffff', fillOpacity: 0.68, stroke: color, strokeWidth: 1, strokeDasharray: '7 5', rx: 10, ry: 10 },
      label: { text: label, fill: color, fontSize: 11, fontWeight: 700, refX: 18, refY: 18, textAnchor: 'start' }
    }
  })
}

function addTopologyNode(config) {
  const palette = nodePalette[config.type]
  return graph.addNode({
    id: config.id,
    shape: 'archify-topology-node',
    x: config.x,
    y: config.y,
    zIndex: 3,
    data: config,
    attrs: {
      body: { fill: palette.fill, stroke: palette.stroke },
      accent: { fill: palette.accent },
      eyebrow: { text: config.eyebrow, fill: palette.eyebrow },
      title: { text: config.title },
      meta: { text: config.meta },
      count: { text: String(config.count) }
    }
  })
}

function addEdge(source, target, label, variant = 'default') {
  const styles = {
    default: { stroke: '#86a0c7', dash: '' },
    emphasis: { stroke: '#3b82f6', dash: '' },
    warning: { stroke: '#e11d48', dash: '7 5' }
  }
  const style = styles[variant]
  graph.addEdge({
    source: { cell: source, anchor: { name: 'right' } },
    target: { cell: target, anchor: { name: 'left' } },
    zIndex: 1,
    router: { name: 'manhattan', args: { padding: 18 } },
    connector: { name: 'rounded', args: { radius: 10 } },
    attrs: {
      line: {
        stroke: style.stroke,
        strokeWidth: variant === 'emphasis' ? 2 : 1.5,
        strokeDasharray: style.dash,
        targetMarker: { name: 'block', width: 9, height: 7 }
      }
    },
    labels: label ? [{ attrs: { label: { text: label, fill: '#64748b', fontSize: 10 }, body: { fill: '#fff', stroke: '#dce6f3', rx: 4, ry: 4 } } }] : []
  })
}

function renderGraph() {
  if (!graph) return
  graph.clearCells()
  activeNode.value = null
  const app = data.value.app || {}
  const resources = [
    { id: 'host', type: 'host', short: 'H', eyebrow: 'COMPUTE / 主机', title: '主机资源', meta: 'SSH 与运行节点', count: data.value.hosts?.length || 0, items: data.value.hosts || [], tone: 'blue', x: 390, y: 76 },
    { id: 'k8s', type: 'k8s', short: 'K8s', eyebrow: 'CONTAINER / K8S', title: 'K8s 集群', meta: '工作负载运行环境', count: data.value.k8sClusters?.length || 0, items: data.value.k8sClusters || [], tone: 'violet', x: 390, y: 232 },
    { id: 'database', type: 'database', short: 'DB', eyebrow: 'DATA / 数据库', title: '数据库', meta: '应用数据依赖', count: data.value.databases?.length || 0, items: data.value.databases || [], tone: 'green', x: 390, y: 388 },
    { id: 'pipeline', type: 'pipeline', short: 'CI', eyebrow: 'DELIVERY / 流水线', title: '流水线执行', meta: '构建、测试与交付', count: data.value.pipelineRuns?.length || 0, items: data.value.pipelineRuns || [], tone: 'cyan', x: 746, y: 76 },
    { id: 'release', type: 'release', short: 'CD', eyebrow: 'CHANGE / 发布', title: '发布记录', meta: '版本与环境变更', count: data.value.releases?.length || 0, items: data.value.releases || [], tone: 'amber', x: 746, y: 232 },
    { id: 'alert', type: 'alert', short: '!', eyebrow: 'OBSERVE / 告警', title: '监控告警', meta: '运行异常与事件', count: data.value.alerts?.length || 0, items: data.value.alerts || [], tone: 'red', x: 746, y: 388 }
  ]
  addBoundary('runtime-boundary', '运行资源', 358, 42, 300, 480, '#5b7db8')
  addBoundary('operation-boundary', '交付与可观测', 714, 42, 300, 480, '#7c6acb')
  addTopologyNode({
    id: 'application', type: 'application', short: 'APP', eyebrow: 'APPLICATION / 应用', title: app.name || '未选择应用',
    meta: [app.code, data.value.env || query.env].filter(Boolean).join(' · ') || '选择应用后加载关系', count: app.id ? 1 : 0,
    items: app.id ? [app] : [], tone: 'blue', x: 54, y: 232
  })
  resources.forEach(addTopologyNode)
  addEdge('application', 'host', '运行于', 'emphasis')
  addEdge('application', 'k8s', '部署于', 'emphasis')
  addEdge('application', 'database', '依赖')
  addEdge('application', 'pipeline', '持续交付', 'emphasis')
  addEdge('pipeline', 'release', '生成版本', 'emphasis')
  addEdge('application', 'alert', '监控', 'warning')
  nextTick(fitCanvas)
}

function zoom(delta) {
  if (!graph) return
  const current = graph.zoom()
  graph.zoomTo(Math.min(1.6, Math.max(0.6, current + delta)))
}

function fitCanvas() {
  if (!graph) return
  graph.zoomToFit({ padding: 28, maxScale: 1 })
  graph.centerContent()
  zoomPercent.value = Math.round(graph.zoom() * 100)
}

function closeNodeDetail() {
  graph?.cleanSelection()
  activeNode.value = null
  nextTick(fitCanvas)
}

async function toggleFullscreen() {
  if (!document.fullscreenElement) await workspaceRef.value?.requestFullscreen()
  else await document.exitFullscreen()
  setTimeout(fitCanvas, 120)
}

function itemTitle(item, type) {
  if (type === 'application') return item.name || item.code || '应用'
  if (type === 'host') return item.hostName || item.name || item.sshIp || '主机'
  if (type === 'k8s') return item.name || 'K8s 集群'
  if (type === 'database') return item.name || item.dbName || '数据库'
  if (type === 'alert') return item.ruleName || item.summary || '告警事件'
  if (type === 'release') return item.version || item.stage || '发布记录'
  return item.pipelineName || item.name || item.imageTag || '流水线执行'
}

function itemMeta(item, type) {
  if (type === 'application') return [item.code, item.env].filter(Boolean).join(' · ') || '-'
  if (type === 'host') return item.sshIp || item.privateIp || item.publicIp || '-'
  if (type === 'k8s') return [item.version, `${item.nodeCount || 0} 节点`].filter(Boolean).join(' · ')
  if (type === 'database') return `${item.host || '-'}:${item.port || '-'} · ${item.dbName || '-'}`
  if (type === 'alert') return [item.severity, item.status].filter(Boolean).join(' · ') || '-'
  if (type === 'release') return [item.env, item.status, item.stage].filter(Boolean).join(' · ') || '-'
  return [item.env, item.status, item.imageTag].filter(Boolean).join(' · ') || '-'
}

function statusClass(item, type) {
  const value = String(item.status || '').toLowerCase()
  if (type === 'alert' || ['failed', 'failure', 'error', 'firing'].includes(value)) return 'status-dot danger'
  if (['success', 'succeeded', 'running', 'online', 'active'].includes(value)) return 'status-dot success'
  return 'status-dot neutral'
}

function statusTagType(status) {
  const value = String(status || '').toLowerCase()
  if (['success', 'succeeded', 'running', 'completed'].includes(value)) return 'success'
  if (['failed', 'failure', 'error'].includes(value)) return 'danger'
  if (['pending', 'waiting', 'building'].includes(value)) return 'warning'
  return 'info'
}

async function loadOptions() {
  const [apps, envs] = await Promise.all([queryOpsApplicationOptions(), queryOpsEnvironmentList({ status: 1 })])
  appOptions.value = apps || []
  environments.value = envs || []
}

async function loadData() {
  loading.value = true
  try {
    data.value = await queryOpsApplicationTopology(query)
    renderGraph()
  } catch (error) {
    ElMessage.error(error?.message || '加载应用拓扑失败')
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await nextTick()
  initGraph()
  await loadOptions()
  await loadData()
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  graph?.dispose()
})
</script>

<style scoped>
.topology-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
  color: #10213f;
}
.topology-header,
.topology-workspace,
.activity-panel,
.summary-grid {
  background: #fff;
  border: 1px solid #dfe8f5;
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(35, 63, 112, .05);
}
.topology-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  padding: 16px 18px;
}
.header-copy {
  display: flex;
  align-items: center;
  gap: 14px;
}
.topology-mark,
.detail-type {
  display: grid;
  place-items: center;
  flex: none;
}
.topology-mark {
  width: 42px;
  height: 42px;
  color: #fff;
  background: #2563eb;
  border-radius: 8px;
  font: 800 15px/1 ui-monospace, SFMono-Regular, Menlo, monospace;
  box-shadow: 0 8px 18px rgba(37, 99, 235, .24);
}
.eyebrow {
  margin: 0 0 4px;
  color: #2f6eea;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 1.2px;
}
h1, h2, h3, p { margin-top: 0; }
h1 { margin-bottom: 4px; font-size: 23px; letter-spacing: 0; }
.header-copy p:last-child { margin-bottom: 0; color: #71809c; }
.filters { display: flex; align-items: center; gap: 10px; }
.filters .el-select { width: 210px; }
.summary-grid {
  display: grid;
  grid-template-columns: repeat(6, minmax(130px, 1fr));
  gap: 0;
  overflow: hidden;
}
.summary-card {
  min-width: 0;
  min-height: 72px;
  padding: 11px 14px;
  position: relative;
  border-right: 1px solid #e8eef7;
}
.summary-card:last-child { border-right: 0; }
.summary-label,
.summary-value { display: flex; align-items: center; }
.summary-label { gap: 6px; min-width: 0; }
.summary-label i { width: 6px; height: 6px; flex: none; background: var(--tone); border-radius: 2px; }
.summary-label span { color: #52637f; font-size: 11px; font-weight: 700; white-space: nowrap; }
.summary-label em {
  margin-left: auto;
  color: var(--tone);
  font: 700 9px/1 ui-monospace, SFMono-Regular, Menlo, monospace;
  font-style: normal;
  opacity: .72;
}
.summary-value { justify-content: space-between; gap: 8px; margin-top: 7px; }
.summary-value strong { color: #10213f; font-size: 22px; line-height: 1; }
.summary-value small { overflow: hidden; color: #8391a8; font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.tone-blue { --tone: #3b82f6; }
.tone-violet { --tone: #7c3aed; }
.tone-green { --tone: #10b981; }
.tone-red { --tone: #f43f5e; }
.tone-amber { --tone: #f59e0b; }
.tone-cyan { --tone: #06b6d4; }
.topology-workspace { overflow: hidden; }
.topology-workspace:fullscreen { width: 100vw; height: 100vh; background: #eef3fb; border-radius: 0; }
.topology-workspace:fullscreen .workspace-body { height: calc(100vh - 72px); }
.workspace-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 66px;
  padding: 0 18px;
  border-bottom: 1px solid #e3ebf6;
}
.workspace-toolbar h2 { margin: 0 0 5px; font-size: 17px; }
.workspace-toolbar p { margin: 0; color: #71809c; font-size: 12px; }
.live-dot { display: inline-block; width: 7px; height: 7px; margin-right: 7px; background: #22c55e; border-radius: 50%; box-shadow: 0 0 0 4px rgba(34, 197, 94, .1); }
.canvas-actions { display: flex; align-items: center; gap: 7px; }
.zoom-value { min-width: 42px; color: #64748b; font: 600 12px/1 ui-monospace, SFMono-Regular, Menlo, monospace; text-align: center; }
.workspace-body { display: grid; grid-template-columns: minmax(0, 1fr); height: 620px; transition: grid-template-columns .2s ease; }
.workspace-body.has-detail { grid-template-columns: minmax(0, 1fr) 310px; }
.canvas-shell { min-width: 0; position: relative; border-right: 1px solid #e3ebf6; overflow: hidden; }
.topology-graph { width: 100%; height: 100%; }
.canvas-legend {
  position: absolute;
  z-index: 6;
  left: 16px;
  bottom: 14px;
  display: flex;
  gap: 14px;
  padding: 8px 10px;
  color: #6b7b96;
  background: rgba(255, 255, 255, .92);
  border: 1px solid #e1e9f5;
  border-radius: 6px;
  font-size: 11px;
  backdrop-filter: blur(8px);
}
.canvas-legend span { display: flex; align-items: center; gap: 5px; }
.legend-dot { width: 7px; height: 7px; border-radius: 2px; }
.legend-dot.application { background: #2563eb; }
.legend-dot.resource { background: #10b981; }
.legend-dot.delivery { background: #f59e0b; }
.legend-dot.alert { background: #f43f5e; }
.canvas-empty { position: absolute; inset: 100px 0 0; z-index: 5; display: grid; place-items: center; pointer-events: none; }
.detail-panel { min-width: 0; padding: 18px; background: #fbfdff; border-left: 1px solid #e3ebf6; overflow: auto; }
.detail-heading { display: flex; align-items: center; gap: 12px; padding-bottom: 16px; border-bottom: 1px solid #e5edf7; }
.detail-heading > div:nth-child(2) { min-width: 0; }
.detail-close { margin-left: auto; flex: none; }
.detail-type { width: 42px; height: 42px; color: var(--tone); background: color-mix(in srgb, var(--tone) 10%, white); border: 1px solid color-mix(in srgb, var(--tone) 22%, white); border-radius: 7px; font: 800 12px/1 ui-monospace, SFMono-Regular, Menlo, monospace; }
.detail-heading span { color: #7a89a3; font-size: 10px; font-weight: 700; letter-spacing: .8px; }
.detail-heading h3 { margin: 4px 0 0; font-size: 17px; }
.detail-metric { display: flex; align-items: center; justify-content: space-between; margin: 14px 0; padding: 12px 14px; background: #f1f5fb; border-radius: 7px; }
.detail-metric span { color: #6f809d; font-size: 12px; }
.detail-metric strong { font-size: 22px; }
.detail-list { display: flex; flex-direction: column; gap: 8px; }
.detail-item { display: flex; align-items: flex-start; gap: 9px; padding: 10px; background: #fff; border: 1px solid #e3ebf6; border-radius: 7px; }
.detail-item > div { min-width: 0; }
.detail-item strong, .detail-item span { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.detail-item strong { font-size: 12px; }
.detail-item span { margin-top: 4px; color: #7888a3; font-size: 11px; }
.status-dot { width: 7px; height: 7px; margin-top: 4px; flex: none; border-radius: 50%; }
.status-dot.success { background: #22c55e; }
.status-dot.danger { background: #f43f5e; }
.status-dot.neutral { background: #94a3b8; }
.activity-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }
.activity-panel { padding: 16px; min-width: 0; }
.panel-title { display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; }
.panel-title > div { display: flex; align-items: center; gap: 8px; }
.panel-title h2 { margin: 0; font-size: 16px; }
.panel-title > span { color: #8090aa; font-size: 11px; }
.title-accent { width: 4px; height: 17px; border-radius: 2px; }
.title-accent.delivery { background: #f59e0b; }
.title-accent.pipeline { background: #06b6d4; }
:deep(.x6-widget-selection-box) { border: 1.5px solid #4f73ff; border-radius: 9px; box-shadow: 0 0 0 3px rgba(79, 115, 255, .1); }
:deep(.el-table) { --el-table-header-bg-color: #f5f8fc; --el-table-border-color: #e8eef7; }
@media (max-width: 1280px) {
  .summary-grid { grid-template-columns: repeat(3, 1fr); }
  .summary-card:nth-child(3) { border-right: 0; }
  .summary-card:nth-child(-n + 3) { border-bottom: 1px solid #e8eef7; }
  .workspace-body.has-detail { grid-template-columns: minmax(0, 1fr) 280px; }
}
@media (max-width: 900px) {
  .topology-header { align-items: flex-start; flex-direction: column; }
  .filters { width: 100%; flex-wrap: wrap; }
  .filters .el-select { flex: 1; min-width: 180px; }
  .summary-grid { grid-template-columns: repeat(2, 1fr); }
  .workspace-body, .workspace-body.has-detail { display: block; height: auto; }
  .canvas-shell { height: 520px; border-right: 0; border-bottom: 1px solid #e3ebf6; }
  .detail-panel { height: 300px; }
  .activity-grid { grid-template-columns: 1fr; }
}
</style>
