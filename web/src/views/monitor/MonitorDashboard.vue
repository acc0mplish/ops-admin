<script setup>
import { uiT } from '../../utils/english-hardcoding-i18n'
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  deleteMonitorDashboard,
  deleteMonitorDashboardPanel,
  monitorDashboardInfo,
  queryMonitorDashboardList,
  queryMonitorDashboardPanel,
  queryMonitorDatasourceOptions,
  saveMonitorDashboard,
  saveMonitorDashboardPanel
} from '../../api/monitor'

const loading = ref(false)
const route = useRoute()
const dashboards = ref([])
const datasourceOptions = ref([])
const selectedDatasourceId = ref()
const activeDashboardId = ref()
const activeDashboard = ref(null)
const panels = ref([])
const panelResults = reactive({})
const panelPending = reactive({})
const dashboardDialogVisible = ref(false)
const panelDialogVisible = ref(false)
const editingDashboard = ref(false)
const editingPanel = ref(false)
const activeTemplate = ref('blank')
const autoRefreshSeconds = ref(30)
const timeRangeSeconds = ref(3600)
const isFullscreen = ref(false)
const lastRefreshAt = ref(new Date())
let refreshTimer = null
let panelRefreshVersion = 0
const panelResultCache = new Map()
const PANEL_QUERY_CONCURRENCY = 4
const PANEL_CACHE_TTL = 15 * 1000
const inspectionFilter = ref('all')

const pageMode = computed(() => route.path.includes('/monitor/inspections') ? 'inspection' : 'dashboard')
const pageLayout = computed(() => pageMode.value === 'inspection' ? 'list' : 'grid')
const pageTitle = computed(() => pageMode.value === 'inspection' ? 'Inspection Dashboard' : uiT('monitoringDashboard'))
const pageDescription = computed(() => (
  pageMode.value === 'inspection'
    ? 'Inspection Checklist 방식으로 PromQL Panel 상태를 점검하고 Inspection Report PDF Export를 지원합니다.'
    : 'Grid Dashboard 방식으로 Metric Card, Trend, Ranking, Gauge를 표시하며 대형 화면 관제에 적합합니다.'
))

const dashboardForm = reactive({
  id: undefined,
  name: '',
  layout: 'grid',
  status: 1,
  description: ''
})

const panelForm = reactive({
  id: undefined,
  dashboardId: undefined,
  title: '',
  datasourceId: undefined,
  promql: '',
  unit: '',
  chartType: 'stat',
  span: 12,
  sort: 0,
  status: 1,
  description: ''
})

const dashboardTemplates = [
  {
    key: 'blank',
    name: '빈 Dashboard',
    description: '빈 Dashboard를 생성하고 PromQL Panel을 직접 추가합니다.',
    panels: []
  },
  {
    key: 'host',
    name: 'Host Resource Dashboard',
    description: 'Node Exporter Dashboard를 기준으로 Online 상태, CPU, Memory, Disk, Network, System Load를 표시합니다.',
    panels: [
      { title: '전체 Host', chartType: 'stat', unit: '대', span: 6, promql: 'count(up{job=~"node.*|node-exporter"})' },
      { title: 'Online Host', chartType: 'stat', unit: '대', span: 6, promql: 'sum(up{job=~"node.*|node-exporter"} == 1)' },
      { title: 'Offline Host', chartType: 'stat', unit: '대', span: 6, promql: 'sum(up{job=~"node.*|node-exporter"} == 0)' },
      { title: '평균 CPU 사용률', chartType: 'gauge', unit: '%', span: 6, promql: '100 - (avg(irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)' },
      { title: '평균 Memory 사용률', chartType: 'gauge', unit: '%', span: 6, promql: '(1 - sum(node_memory_MemAvailable_bytes) / sum(node_memory_MemTotal_bytes)) * 100' },
      { title: '평균 Disk 사용률', chartType: 'gauge', unit: '%', span: 6, promql: '100 - (sum(node_filesystem_avail_bytes{fstype!~"tmpfs|overlay",mountpoint!~"/run.*|/boot.*"}) / sum(node_filesystem_size_bytes{fstype!~"tmpfs|overlay",mountpoint!~"/run.*|/boot.*"}) * 100)' },
      { title: 'CPU 사용률 Top', chartType: 'bar', unit: '%', span: 12, promql: 'topk(10, 100 - (avg by (instance) (irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100))' },
      { title: 'Memory 사용률 Top', chartType: 'bar', unit: '%', span: 12, promql: 'topk(10, (1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes) * 100)' },
      { title: 'Disk 사용률 Top', chartType: 'bar', unit: '%', span: 12, promql: 'topk(10, 100 - (node_filesystem_avail_bytes{fstype!~"tmpfs|overlay",mountpoint!~"/run.*|/boot.*"} / node_filesystem_size_bytes{fstype!~"tmpfs|overlay",mountpoint!~"/run.*|/boot.*"} * 100))' },
      { title: 'System Load Top', chartType: 'bar', unit: '', span: 12, promql: 'topk(10, node_load1)' },
      { title: 'Network 수신 Rate Top', chartType: 'bar', unit: 'B/s', span: 12, promql: 'topk(10, sum by (instance) (rate(node_network_receive_bytes_total{device!~"lo|veth.*|docker.*|br.*"}[5m])))' },
      { title: 'Network 송신 Rate Top', chartType: 'bar', unit: 'B/s', span: 12, promql: 'topk(10, sum by (instance) (rate(node_network_transmit_bytes_total{device!~"lo|veth.*|docker.*|br.*"}[5m])))' },
      { title: 'Disk Read Rate Top', chartType: 'bar', unit: 'B/s', span: 12, promql: 'topk(10, sum by (instance) (rate(node_disk_read_bytes_total[5m])))' },
      { title: 'Disk Write Rate Top', chartType: 'bar', unit: 'B/s', span: 12, promql: 'topk(10, sum by (instance) (rate(node_disk_written_bytes_total[5m])))' },
      { title: 'CPU 사용 Trend', chartType: 'line', unit: '%', span: 12, promql: '100 - (avg by (instance) (irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)' },
      { title: 'Memory 사용 Trend', chartType: 'line', unit: '%', span: 12, promql: '(1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes) * 100' },
      { title: 'File Handle 사용률', chartType: 'gauge', unit: '%', span: 6, promql: 'sum(node_filefd_allocated) / sum(node_filefd_maximum) * 100' },
      { title: '실행 Process 수', chartType: 'stat', unit: '개', span: 6, promql: 'sum(node_procs_running)' },
      { title: 'System Uptime Top', chartType: 'bar', unit: '일', span: 12, promql: 'topk(10, (time() - node_boot_time_seconds) / 86400)' },
      { title: 'Host 정보', chartType: 'table', unit: '', span: 12, promql: 'node_uname_info' }
    ]
  },
  {
    key: 'k8s',
    name: 'Kubernetes Dashboard',
    description: 'Kubernetes Dashboard 스타일로 Cluster Resource, Capacity, Workload, Network Metric을 표시합니다.',
    panels: [
      { title: 'Pod 수', chartType: 'stat', unit: '개', span: 8, promql: 'count(kube_pod_info)' },
      { title: 'Running Pod', chartType: 'stat', unit: '개', span: 8, promql: 'sum(kube_pod_status_phase{phase="Running"})' },
      { title: '이상 Pod', chartType: 'stat', unit: '개', span: 8, promql: 'sum(kube_pod_status_phase{phase=~"Failed|Unknown|Pending"})' },
      { title: 'Ready Node', chartType: 'stat', unit: '개', span: 8, promql: 'sum(kube_node_status_condition{condition="Ready",status="true"})' },
      { title: 'Namespace', chartType: 'stat', unit: '개', span: 8, promql: 'count(kube_namespace_created)' },
      { title: 'Deployment', chartType: 'stat', unit: '개', span: 8, promql: 'count(kube_deployment_created)' },
      { title: 'Service', chartType: 'stat', unit: '개', span: 8, promql: 'count(kube_service_info)' },
      { title: 'Ingress', chartType: 'stat', unit: '개', span: 8, promql: 'count(kube_ingress_info)' },
      { title: 'PVC', chartType: 'stat', unit: '개', span: 8, promql: 'count(kube_persistentvolumeclaim_info)' },
      { title: 'CPU Request 사용률', chartType: 'gauge', unit: '%', span: 12, promql: 'sum(kube_pod_container_resource_requests{resource="cpu"}) / sum(kube_node_status_allocatable{resource="cpu"}) * 100' },
      { title: 'Memory Request 사용률', chartType: 'gauge', unit: '%', span: 12, promql: 'sum(kube_pod_container_resource_requests{resource="memory"}) / sum(kube_node_status_allocatable{resource="memory"}) * 100' },
      { title: 'Namespace별 Pod 분포', chartType: 'bar', unit: '개', span: 12, promql: 'sum by (namespace) (kube_pod_info)' },
      { title: 'Node별 Pod 분포', chartType: 'bar', unit: '개', span: 12, promql: 'sum by (node) (kube_pod_info)' },
      { title: 'Workload Replica 가용률', chartType: 'bar', unit: '%', span: 12, promql: 'sum by (deployment) (kube_deployment_status_replicas_available) / sum by (deployment) (kube_deployment_spec_replicas) * 100' },
      { title: '이상 Reason Top', chartType: 'bar', unit: '개', span: 12, promql: 'sum by (reason) (kube_pod_container_status_waiting_reason)' },
      { title: 'Pod 누적 Restart Top', chartType: 'bar', unit: '회', span: 12, promql: 'topk(10, sum by (namespace, pod) (kube_pod_container_status_restarts_total{pod!=""}))' },
      { title: '최근 1시간 Pod Restart 증가 Top', chartType: 'bar', unit: '회', span: 12, promql: 'topk(10, sum by (namespace, pod) (increase(kube_pod_container_status_restarts_total{pod!=""}[1h])))' },
      { title: 'Container CPU 사용 Trend', chartType: 'line', unit: 'Core', span: 12, promql: 'sum by (namespace) (rate(container_cpu_usage_seconds_total{container!="",pod!=""}[5m]))' },
      { title: 'Container Memory 사용 Trend', chartType: 'line', unit: 'B', span: 12, promql: 'sum by (namespace) (container_memory_working_set_bytes{container!="",pod!=""})' },
      { title: 'Pod Network 수신 Rate', chartType: 'line', unit: 'B/s', span: 12, promql: 'sum by (namespace) (rate(container_network_receive_bytes_total[5m]))' },
      { title: 'Pod Network 송신 Rate', chartType: 'line', unit: 'B/s', span: 12, promql: 'sum by (namespace) (rate(container_network_transmit_bytes_total[5m]))' },
      { title: 'Pod 상세', chartType: 'table', unit: '', span: 24, promql: 'kube_pod_info' }
    ]
  }
]

const activePanels = computed(() => panels.value.filter((item) => item.status === 1))
const inspectionSummary = computed(() => {
  const summary = { healthy: 0, warning: 0, danger: 0, disabled: 0 }
  panels.value.forEach((panel) => { summary[panelStateKey(panel)] += 1 })
  return summary
})
const inspectionPanels = computed(() => {
  const priority = { danger: 0, warning: 1, healthy: 2, disabled: 3 }
  return panels.value
    .filter((panel) => inspectionFilter.value === 'all' || panelStateKey(panel) === inspectionFilter.value)
    .slice()
    .sort((left, right) => priority[panelStateKey(left)] - priority[panelStateKey(right)] || Number(left.sort || 0) - Number(right.sort || 0))
})
const visibleDashboards = computed(() => dashboards.value.filter((item) => (item.layout || 'grid') === pageLayout.value))
const isListLayout = computed(() => pageMode.value === 'inspection')
const isK8sDashboard = computed(() => {
  if (isListLayout.value) return false
  const name = `${activeDashboard.value?.name || ''} ${activeDashboard.value?.description || ''}`.toLowerCase()
  return name.includes('k8s') || name.includes('kubernetes') || panels.value.some((panel) => String(panel.promql || '').includes('kube_'))
})
const dashboardHealth = computed(() => {
  const errors = activePanels.value.filter((panel) => panelResults[panel.id]?.error).length
  if (!activeDashboard.value) return { text: '미선택', type: 'info' }
  if (errors > 0) return { text: `${errors} 개 이상`, type: 'danger' }
  return { text: '정상', type: 'success' }
})
const defaultDatasourceId = computed(() => selectedDatasourceId.value || datasourceOptions.value[0]?.id)
const currentTemplate = computed(() => dashboardTemplates.find((item) => item.key === activeTemplate.value) || dashboardTemplates[0])
const currentDatasourceName = computed(() => datasourceOptions.value.find((item) => item.id === selectedDatasourceId.value)?.name || 'Datasource 미선택')
const lastRefreshText = computed(() => lastRefreshAt.value.toLocaleTimeString('zh-CN', { hour12: false }))

function resetDashboardForm() {
  Object.assign(dashboardForm, {
    id: undefined,
    name: '',
    layout: pageLayout.value,
    status: 1,
    description: ''
  })
  activeTemplate.value = 'blank'
}

function resetPanelForm() {
  Object.assign(panelForm, {
    id: undefined,
    dashboardId: activeDashboardId.value,
    title: '',
    datasourceId: defaultDatasourceId.value,
    promql: '',
    unit: '',
    chartType: 'stat',
    span: 12,
    sort: panels.value.length + 1,
    status: 1,
    description: ''
  })
}

function metricText(metric) {
  return Object.entries(metric || {})
    .map(([key, value]) => `${key}="${value}"`)
    .join(', ')
}

function metricName(metric) {
  if (!metric) return 'metric'
  return metric.instance || metric.pod || metric.namespace || metric.job || metric.__name__ || 'metric'
}

function numberValue(row) {
	const raw = row?.value?.[1] ?? row?.values?.[row.values.length - 1]?.[1]
	const value = Number(raw)
  return Number.isFinite(value) ? value : 0
}

function formatByUnit(value, unit = '') {
  if (!Number.isFinite(value)) return '-'
  if (unit === 'B/s') {
    const units = ['B/s', 'KB/s', 'MB/s', 'GB/s', 'TB/s']
    let current = Math.abs(value)
    let index = 0
    while (current >= 1024 && index < units.length - 1) {
      current /= 1024
      index += 1
    }
    const signed = value < 0 ? -current : current
    return `${signed.toFixed(current >= 100 ? 0 : current >= 10 ? 1 : 2)} ${units[index]}`
  }
  if (unit === '일') return `${value.toFixed(value >= 10 ? 0 : 1)}일`
  if (unit === '%') return `${value.toFixed(value >= 10 ? 1 : 2).replace(/\.?0+$/, '')}%`
  if (unit && unit !== '개' && unit !== '대') return `${value.toFixed(value >= 100 ? 0 : 2).replace(/\.?0+$/, '')}${unit}`
  return value.toLocaleString(undefined, { maximumFractionDigits: value >= 100 ? 0 : 2 })
}

function panelRows(panel) {
  return panelResults[panel.id]?.result || []
}

function panelValue(panel) {
	const value = numberValue(panelRows(panel)[0])
  if (!Number.isFinite(value)) return '-'
  const formatted = formatByUnit(value, panel.unit)
  return panel.unit && formatted.endsWith(panel.unit) ? formatted.slice(0, -panel.unit.length).trim() : formatted
}

function panelTrend(panel) {
	const firstSeries = panelRows(panel)[0]
	if (firstSeries?.values?.length) return firstSeries.values.map((item) => Number(item?.[1])).filter(Number.isFinite)
	return panelRows(panel).slice(0, 24).map((item) => numberValue(item))
}

function sparklinePoints(panel) {
  const values = panelTrend(panel)
  if (!values.length) return ''
  const max = Math.max(...values)
  const min = Math.min(...values)
  return trendPoints(values, min, max)
}

function trendPoints(values, min, max) {
  const range = max - min || 1
  return values.map((value, index) => {
    const x = values.length === 1 ? 100 : (index / (values.length - 1)) * 100
    const y = 48 - ((value - min) / range) * 40
    return `${x},${y}`
  }).join(' ')
}

const lineColors = ['#3b82f6', '#14b8a6', '#f59e0b', '#8b5cf6', '#ef4444', '#06b6d4', '#84cc16', '#ec4899']

function panelLineSeries(panel) {
  const series = panelRows(panel)
    .map((row) => ({ name: metricName(row.metric), values: (row.values || []).map((item) => Number(item?.[1])).filter(Number.isFinite) }))
    .filter((item) => item.values.length)
    .slice(0, 8)
  const allValues = series.flatMap((item) => item.values)
  if (!allValues.length) return []
  const min = Math.min(...allValues)
  const max = Math.max(...allValues)
  return series.map((item, index) => ({
    ...item,
    color: lineColors[index % lineColors.length],
    points: trendPoints(item.values, min, max)
  }))
}

function sparklineAreaPoints(panel) {
  const points = panelLineSeries(panel)[0]?.points || sparklinePoints(panel)
  return points ? `0,52 ${points} 100,52` : ''
}

function panelStats(panel) {
  const values = panelTrend(panel)
  if (!values.length) return { min: '-', max: '-', avg: '-' }
  const min = Math.min(...values)
  const max = Math.max(...values)
  const avg = values.reduce((sum, value) => sum + value, 0) / values.length
  return {
    min: formatByUnit(min, panel.unit),
    max: formatByUnit(max, panel.unit),
    avg: formatByUnit(avg, panel.unit)
  }
}

function panelChartLabel(chartType) {
  return ({ stat: 'Metric', gauge: 'Gauge', bar: 'Ranking', line: 'Trend', table: '상세' })[chartType] || chartType
}

function barRows(panel) {
  const rows = panelRows(panel)
    .map((row) => ({ name: metricName(row.metric), value: numberValue(row), displayValue: formatByUnit(numberValue(row), panel.unit) }))
    .sort((a, b) => b.value - a.value)
    .slice(0, 8)
  const max = Math.max(...rows.map((item) => item.value), 1)
  return rows.map((item) => ({ ...item, percent: Math.max(4, (item.value / max) * 100) }))
}

function gaugePercent(panel) {
	const value = numberValue(panelRows(panel)[0])
  if (!Number.isFinite(value)) return 0
  return Math.max(0, Math.min(100, value))
}

function panelSpan(panel) {
  const span = Number(panel.span || 12)
  if (isK8sDashboard.value) {
    return Math.max(1, Math.min(6, Math.round(span / 4)))
  }
  return Math.max(1, Math.min(4, Math.round(span / 6)))
}

function panelState(panel) {
  if (panel.status !== 1) return '비활성화됨'
  if (panelResults[panel.id]?.error) return 'Query 실패'
  if (!panelRows(panel).length) return 'Data 없음'
  return '실시간'
}

function panelStateKey(panel) {
  if (panel.status !== 1) return 'disabled'
  if (panelResults[panel.id]?.error) return 'danger'
  if (!panelRows(panel).length) return 'warning'
  return 'healthy'
}

function panelStateType(panel) {
  if (panel.status !== 1) return 'info'
  if (panelResults[panel.id]?.error) return 'danger'
  if (!panelRows(panel).length) return 'warning'
  return 'success'
}

function panelResultCount(panel) {
  return panelRows(panel).length
}

function panelDisplayValue(panel) {
	const value = numberValue(panelRows(panel)[0])
  if (!Number.isFinite(value)) return '-'
  return formatByUnit(value, panel.unit)
}

function escapeHtml(value) {
  return String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function exportInspectionReportPdf() {
  if (!activeDashboard.value) {
    ElMessage.warning('Inspection Dashboard를 먼저 선택하십시오.')
    return
  }
  const now = new Date().toLocaleString()
  const currentDatasourceName = datasourceOptions.value.find((item) => item.id === selectedDatasourceId.value)?.name || '-'
  const rows = panels.value.map((panel, index) => {
    const error = panelResults[panel.id]?.error || ''
    return `
      <tr>
        <td>${index + 1}</td>
        <td>${escapeHtml(panel.title)}</td>
        <td><span class="status ${panelStateType(panel)}">${escapeHtml(panelState(panel))}</span></td>
        <td>${escapeHtml(panelDisplayValue(panel))}</td>
        <td>${panelResultCount(panel)}</td>
        <td>${escapeHtml(currentDatasourceName)}</td>
        <td>${escapeHtml(panel.chartType)}</td>
        <td><code>${escapeHtml(panel.promql)}</code>${error ? `<div class="error">${escapeHtml(error)}</div>` : ''}</td>
      </tr>
    `
  }).join('')
  const win = window.open('', '_blank')
  if (!win) {
    ElMessage.warning('Browser가 Report Window를 차단했습니다. Popup을 허용한 뒤 다시 시도하십시오.')
    return
  }
  win.document.write(`
    <!doctype html>
    <html>
      <head>
        <meta charset="utf-8" />
        <title>${escapeHtml(activeDashboard.value.name)} - Inspection Report</title>
        <style>
          * { box-sizing: border-box; }
          body { margin: 0; padding: 28px; color: #10213f; font-family: Arial, "Microsoft YaHei", sans-serif; background: #fff; }
          h1 { margin: 0 0 8px; font-size: 26px; }
          .meta { display: flex; gap: 20px; margin-bottom: 22px; color: #64748b; font-size: 13px; }
          .summary { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; margin-bottom: 22px; }
          .card { padding: 14px; border: 1px solid #dce7f7; border-radius: 10px; background: #f8fbff; }
          .card span { display: block; color: #64748b; font-size: 12px; }
          .card strong { display: block; margin-top: 8px; font-size: 22px; }
          table { width: 100%; border-collapse: collapse; font-size: 12px; }
          th, td { padding: 10px; border: 1px solid #dce7f7; text-align: left; vertical-align: top; }
          th { background: #edf4ff; color: #334155; }
          code { display: block; max-width: 420px; white-space: pre-wrap; word-break: break-all; color: #1d4ed8; font-family: Consolas, Monaco, monospace; }
          .status { display: inline-block; padding: 3px 8px; border-radius: 999px; }
          .success { color: #15803d; background: #dcfce7; }
          .warning { color: #a16207; background: #fef9c3; }
          .danger { color: #b91c1c; background: #fee2e2; }
          .info { color: #475569; background: #e2e8f0; }
          .error { margin-top: 6px; color: #b91c1c; }
          @media print { body { padding: 16px; } .no-print { display: none; } }
        </style>
      </head>
      <body>
        <button class="no-print" onclick="window.print()" style="float:right;padding:8px 14px;">인쇄 / PDF 저장</button>
        <h1>${escapeHtml(activeDashboard.value.name)} Inspection Report</h1>
        <div class="meta">
          <span>생성 시각: ${escapeHtml(now)}</span>
          <span>Query Datasource: ${escapeHtml(currentDatasourceName)}</span>
          <span>Dashboard 상태: ${escapeHtml(dashboardHealth.value.text)}</span>
          <span>Refresh 주기: ${autoRefreshSeconds.value ? `${autoRefreshSeconds.value}s` : '꺼짐'}</span>
        </div>
        <div class="summary">
          <div class="card"><span>Panel 수</span><strong>${panels.value.length}</strong></div>
          <div class="card"><span>활성 Panel</span><strong>${activePanels.value.length}</strong></div>
          <div class="card"><span>이상 Panel</span><strong>${activePanels.value.filter((panel) => panelResults[panel.id]?.error).length}</strong></div>
          <div class="card"><span>Inspection Type</span><strong>List Inspection</strong></div>
        </div>
        <table>
          <thead>
            <tr>
              <th>#</th><th>{{ uiT('panel') }}</th><th>상태</th><th>현재 값</th><th>Series</th><th>Datasource</th><th>Type</th><th>PromQL / Error</th>
            </tr>
          </thead>
          <tbody>${rows || '<tr><td colspan="8">Inspection Panel이 없습니다.</td></tr>'}</tbody>
        </table>
      </body>
    </html>
  `)
  win.document.close()
  win.focus()
  setTimeout(() => win.print(), 300)
}

async function loadBase() {
  const [dashboardData, datasources] = await Promise.all([
    queryMonitorDashboardList({ pageNum: 1, pageSize: 100 }),
    queryMonitorDatasourceOptions()
  ])
  dashboards.value = dashboardData.list || []
  datasourceOptions.value = (datasources || []).filter((item) => ['prometheus', 'victoriametrics'].includes(item.type))
  if (!datasourceOptions.value.some((item) => item.id === selectedDatasourceId.value)) {
    selectedDatasourceId.value = datasourceOptions.value.find((item) => item.isDefault)?.id || datasourceOptions.value[0].id
  }
  if (!visibleDashboards.value.some((item) => item.id === activeDashboardId.value)) {
    activeDashboardId.value = visibleDashboards.value[0]?.id
  }
}

async function loadDashboard(id = activeDashboardId.value) {
  if (!id) {
    activeDashboard.value = null
    panels.value = []
    return
  }
  loading.value = true
  try {
    const data = await monitorDashboardInfo(id)
    activeDashboard.value = data.dashboard
    panels.value = data.panels || []
    activeDashboardId.value = id
    // Render the dashboard shell immediately. Panels fill in progressively so a
    // large K8s dashboard does not block the first paint on dozens of queries.
    Object.keys(panelResults).forEach((key) => delete panelResults[key])
    Object.keys(panelPending).forEach((key) => delete panelPending[key])
    void refreshAllPanels({ progressive: true, force: false })
  } finally {
    loading.value = false
  }
}

function openCreateDashboard() {
  editingDashboard.value = false
  resetDashboardForm()
  dashboardDialogVisible.value = true
}

function openEditDashboard() {
  if (!activeDashboard.value) return
  editingDashboard.value = true
  Object.assign(dashboardForm, { ...activeDashboard.value, layout: pageLayout.value })
  activeTemplate.value = 'blank'
  dashboardDialogVisible.value = true
}

async function createTemplatePanels(dashboardId) {
  if (editingDashboard.value || !currentTemplate.value.panels.length) return
  if (!defaultDatasourceId.value) return
  await Promise.all(currentTemplate.value.panels.map((panel, index) => saveMonitorDashboardPanel({
    dashboardId,
    datasourceId: defaultDatasourceId.value,
    title: panel.title,
    promql: panel.promql,
    unit: panel.unit,
    chartType: panel.chartType,
    span: panel.span,
    sort: index + 1,
    status: 1,
    description: currentTemplate.value.name
  })))
}

async function submitDashboard() {
  if (!dashboardForm.name.trim()) {
    ElMessage.warning(`${pageTitle.value} 이름을 입력하십시오.`)
    return
  }
  dashboardForm.layout = pageLayout.value
  if (!editingDashboard.value && currentTemplate.value.panels.length && !defaultDatasourceId.value) {
    ElMessage.warning('Prometheus Datasource를 먼저 생성한 뒤 Template으로 Dashboard를 생성하십시오.')
    return
  }
  const savedDashboard = await saveMonitorDashboard(dashboardForm)
  ElMessage.success('저장했습니다.')
  dashboardDialogVisible.value = false
  await loadBase()
  const target = dashboards.value.find((item) => Number(item.id) === Number(savedDashboard?.id)) || savedDashboard || dashboards.value[0]
  if (target) {
    activeDashboardId.value = target.id
    await createTemplatePanels(target.id)
    await loadBase()
    await loadDashboard(target.id)
  }
}

async function handleDeleteDashboard() {
  if (!activeDashboard.value) return
  await ElMessageBox.confirm(`${pageTitle.value} “${activeDashboard.value.name}”을(를) 삭제하시겠습니까? 연결된 Panel도 함께 삭제됩니다.`, '확인', { type: 'warning' })
  await deleteMonitorDashboard(activeDashboard.value.id)
  ElMessage.success('삭제했습니다.')
  activeDashboardId.value = undefined
  activeDashboard.value = null
  panels.value = []
  await loadBase()
  await loadDashboard(activeDashboardId.value)
}

function openCreatePanel() {
  if (!activeDashboardId.value) {
    ElMessage.warning('Monitoring Dashboard를 생성하거나 선택하십시오.')
    return
  }
  editingPanel.value = false
  resetPanelForm()
  panelDialogVisible.value = true
}

function openEditPanel(row) {
  editingPanel.value = true
  Object.assign(panelForm, row)
  panelDialogVisible.value = true
}

async function submitPanel() {
  if (!panelForm.title.trim() || !panelForm.datasourceId || !panelForm.promql.trim()) {
    ElMessage.warning('Panel 제목, Datasource 및 PromQL을 입력하십시오.')
    return
  }
  await saveMonitorDashboardPanel(panelForm)
  ElMessage.success('저장했습니다.')
  panelDialogVisible.value = false
  await loadDashboard(activeDashboardId.value)
}

async function handleDeletePanel(row) {
  await ElMessageBox.confirm(`Panel “${row.title}”을(를) 삭제하시겠습니까?`, '확인', { type: 'warning' })
  await deleteMonitorDashboardPanel(row.id)
  ElMessage.success('삭제했습니다.')
  await loadDashboard(activeDashboardId.value)
}

async function refreshPanel(row) {
  if (!selectedDatasourceId.value) {
    ElMessage.warning('Datasource 관리에서 Prometheus 또는 VictoriaMetrics Datasource를 먼저 구성하십시오.')
    return
  }
  await loadPanel(row, { force: true })
  lastRefreshAt.value = new Date()
}

async function refreshProblemPanels() {
  const items = activePanels.value.filter((panel) => ['danger', 'warning'].includes(panelStateKey(panel)))
  if (!items.length) return ElMessage.success('현재 재검토할 이상 Panel이 없습니다.')
  const version = ++panelRefreshVersion
  await runPanelQueue(items, version, true)
  if (version === panelRefreshVersion) lastRefreshAt.value = new Date()
}

async function copyPromql(promql) {
  try {
    await navigator.clipboard.writeText(promql || '')
    ElMessage.success('PromQL을 복사했습니다.')
  } catch {
    ElMessage.warning('복사에 실패했습니다. PromQL을 직접 복사하십시오.')
  }
}

function panelQueryPayload(id) {
	const endAt = Math.floor(Date.now() / 1000)
	const startAt = endAt - timeRangeSeconds.value
	return {
		id,
		datasourceId: selectedDatasourceId.value,
		startAt,
		endAt,
		stepSeconds: Math.max(15, Math.ceil(timeRangeSeconds.value / 120))
	}
}

function panelCacheKey(panel) {
  return `${selectedDatasourceId.value}:${timeRangeSeconds.value}:${panel.id}`
}

async function loadPanel(panel, { force = true, version = panelRefreshVersion } = {}) {
  const key = panelCacheKey(panel)
  const cached = panelResultCache.get(key)
  if (!force && cached && cached.expiresAt > Date.now()) {
    panelResults[panel.id] = cached.data
    return
  }

  panelPending[panel.id] = true
  try {
    const data = await queryMonitorDashboardPanel(panelQueryPayload(panel.id))
    if (version !== panelRefreshVersion) return
    panelResults[panel.id] = data
    panelResultCache.set(key, { data, expiresAt: Date.now() + PANEL_CACHE_TTL })
  } catch (error) {
    if (version === panelRefreshVersion) {
      panelResults[panel.id] = { error: error.message || 'query failed' }
    }
  } finally {
    if (version === panelRefreshVersion) delete panelPending[panel.id]
  }
}

async function runPanelQueue(items, version, force) {
  let cursor = 0
  const worker = async () => {
    while (cursor < items.length && version === panelRefreshVersion) {
      const panel = items[cursor]
      cursor += 1
      await loadPanel(panel, { force, version })
    }
  }
  await Promise.all(Array.from({ length: Math.min(PANEL_QUERY_CONCURRENCY, items.length) }, worker))
}

async function refreshAllPanels({ progressive = false, force = true } = {}) {
  if (!selectedDatasourceId.value) {
    const message = 'Datasource 관리에서 Prometheus 또는 VictoriaMetrics Datasource를 먼저 구성하십시오.'
    for (const panel of panels.value.filter((item) => item.status === 1)) panelResults[panel.id] = { error: message }
    return
  }
  const enabledPanels = panels.value.filter((item) => item.status === 1)
  const version = ++panelRefreshVersion
  const firstScreenPanels = enabledPanels.slice(0, 6)
  const remainingPanels = enabledPanels.slice(6)
  const loadRemaining = () => runPanelQueue(remainingPanels, version, force)

  if (progressive) {
    await runPanelQueue(firstScreenPanels, version, force)
    if (version === panelRefreshVersion && remainingPanels.length) {
      window.setTimeout(() => { void loadRemaining() }, 0)
    }
    return
  }

  await runPanelQueue(firstScreenPanels, version, force)
  if (version === panelRefreshVersion) await loadRemaining()
  if (version === panelRefreshVersion) lastRefreshAt.value = new Date()
}

async function handleDatasourceChange() {
  if (activePanels.value.length) {
    await refreshAllPanels()
  }
}

function restartAutoRefresh() {
  if (refreshTimer) window.clearInterval(refreshTimer)
  refreshTimer = null
  if (!autoRefreshSeconds.value) return
  refreshTimer = window.setInterval(() => {
    if (activePanels.value.length) refreshAllPanels()
  }, autoRefreshSeconds.value * 1000)
}

async function toggleFullscreen() {
  const el = document.querySelector('.dashboard-main')
  if (!document.fullscreenElement && el?.requestFullscreen) {
    await el.requestFullscreen()
  } else if (document.exitFullscreen) {
    await document.exitFullscreen()
  }
}

function onFullscreenChange() {
  isFullscreen.value = Boolean(document.fullscreenElement)
}

onMounted(async () => {
  await loadBase()
  await loadDashboard(activeDashboardId.value)
  restartAutoRefresh()
  document.addEventListener('fullscreenchange', onFullscreenChange)
})

watch(pageLayout, async () => {
  activeDashboardId.value = undefined
  activeDashboard.value = null
  panels.value = []
  await loadBase()
  await loadDashboard(activeDashboardId.value)
})

onBeforeUnmount(() => {
  if (refreshTimer) window.clearInterval(refreshTimer)
  document.removeEventListener('fullscreenchange', onFullscreenChange)
})
</script>

<template>
  <div class="dashboard-page">
    <section class="dashboard-workspace">
      <div class="workspace-title">
        <span class="brand-mark">M</span>
        <div>
          <strong>{{ uiT('monitoringDashboard') }}</strong>
          <p>{{ pageDescription }}</p>
        </div>
      </div>

      <div class="dashboard-switcher">
        <el-scrollbar>
          <div class="dashboard-list">
          <button
            v-for="item in visibleDashboards"
            :key="item.id"
            class="dashboard-item"
            :class="{ active: item.id === activeDashboardId }"
            @click="loadDashboard(item.id)"
          >
            <strong>{{ item.name }}</strong>
            <span>{{ item.panelCount || 0 }} 개Panel</span>
          </button>
          <div v-if="!visibleDashboards.length" class="empty-switcher">{{ pageTitle }} 없음</div>
          </div>
        </el-scrollbar>
      </div>

      <el-button class="create-screen-btn" type="primary" @click="openCreateDashboard">{{ pageTitle }} 생성</el-button>
    </section>

    <main class="dashboard-main observability-canvas" v-loading="loading">
      <section class="dashboard-hero">
        <div>
          <div class="eyebrow">{{ uiT('monitoringDashboard') }}</div>
          <h2>{{ activeDashboard?.name || pageTitle }}</h2>
          <p>{{ activeDashboard?.description || pageDescription }}</p>
          <div v-if="activeDashboard" class="layout-hint">
            {{ isListLayout ? '현재 Layout: Inspection List. 일상 Troubleshooting과 항목별 점검에 적합합니다.' : '현재 Layout: Grid Dashboard. 대형 화면 표시와 전체 관제에 적합합니다.' }}
          </div>
          <div v-if="activeDashboard" class="dashboard-context">
            <span><i class="health-dot"></i>{{ currentDatasourceName }}</span>
            <span>최근 업데이트 {{ lastRefreshText }}</span>
            <span>{{ activePanels.length }} 개활성 Panel</span>
          </div>
        </div>
		<div class="hero-actions">
          <el-select v-model="selectedDatasourceId" placeholder="Datasource 선택" style="width: 180px" @change="handleDatasourceChange">
            <el-option v-for="item in datasourceOptions" :key="item.id" :label="item.name" :value="item.id" />
		  </el-select>
		  <el-select v-model="timeRangeSeconds" style="width: 130px" @change="refreshAllPanels">
			<el-option label="최근 15분" :value="900" />
			<el-option label="최근 1시간" :value="3600" />
			<el-option label="최근 6시간" :value="21600" />
			<el-option label="최근 24시간" :value="86400" />
		  </el-select>
          <el-select v-model="autoRefreshSeconds" style="width: 130px" @change="restartAutoRefresh">
            <el-option label="자동 Refresh 끄기" :value="0" />
            <el-option label="10초 Refresh" :value="10" />
            <el-option label="30초 Refresh" :value="30" />
            <el-option label="60초 Refresh" :value="60" />
          </el-select>
          <el-button @click="refreshAllPanels" :disabled="!activePanels.length">전체 Refresh</el-button>
          <el-button v-if="!isListLayout" @click="toggleFullscreen" :disabled="!activeDashboard">{{ isFullscreen ? '전체 화면 종료' : 'Dashboard 전체 화면' }}</el-button>
          <el-button v-else @click="exportInspectionReportPdf" :disabled="!activeDashboard">{{ uiT('inspectionReportPdfExport') }}</el-button>
          <el-button v-if="!isFullscreen" @click="openEditDashboard" :disabled="!activeDashboard">{{ pageTitle }} 수정</el-button>
          <el-button v-if="!isFullscreen" type="danger" plain @click="handleDeleteDashboard" :disabled="!activeDashboard">{{ pageTitle }} 삭제</el-button>
          <el-button v-if="!isFullscreen" type="primary" @click="openCreatePanel">Panel 추가</el-button>
        </div>
      </section>

      <section class="dashboard-summary">
        <div class="summary-card">
          <span>Panel 수</span>
          <strong>{{ panels.length }}</strong>
        </div>
        <div class="summary-card">
          <span>활성 Panel</span>
          <strong>{{ activePanels.length }}</strong>
        </div>
        <div class="summary-card">
          <span>Refresh 주기</span>
          <strong>{{ autoRefreshSeconds ? `${autoRefreshSeconds}s` : '꺼짐' }}</strong>
        </div>
        <div class="summary-card">
          <span>Dashboard 상태</span>
          <strong :class="`state-${dashboardHealth.type}`">{{ dashboardHealth.text }}</strong>
        </div>
      </section>

      <el-empty v-if="!activeDashboard" description="Monitoring Dashboard가 없습니다. 먼저 생성하십시오." />
      <el-empty v-else-if="!panels.length" description="현재 Dashboard에 Panel이 없습니다. Panel을 추가하거나 Template으로 생성하십시오." />

      <section v-else-if="isListLayout" class="inspection-list">
        <div class="inspection-command-bar">
          <div class="inspection-filter">
            <button :class="{ active: inspectionFilter === 'all' }" @click="inspectionFilter = 'all'">전체 {{ panels.length }}</button>
            <button :class="{ active: inspectionFilter === 'danger' }" @click="inspectionFilter = 'danger'">이상 {{ inspectionSummary.danger }}</button>
            <button :class="{ active: inspectionFilter === 'warning' }" @click="inspectionFilter = 'warning'">검토 필요 {{ inspectionSummary.warning }}</button>
            <button :class="{ active: inspectionFilter === 'healthy' }" @click="inspectionFilter = 'healthy'">정상 {{ inspectionSummary.healthy }}</button>
          </div>
          <div class="inspection-command-actions">
            <span>이상 항목 우선 정렬 · 현재 {{ inspectionPanels.length }}개 표시</span>
            <el-button @click="refreshProblemPanels" :disabled="!activePanels.length">이상 재검토</el-button>
            <el-button type="primary" @click="refreshAllPanels" :disabled="!activePanels.length">Inspection 실행</el-button>
          </div>
        </div>
        <div class="inspection-head">
          <div>
            <h3>{{ uiT('inspectionPanel') }}</h3>
            <p>Panel별 Query 상태, 현재 값, 반환 Series, PromQL을 확인하며 일상 Troubleshooting Inspection에 적합합니다.</p>
          </div>
          <el-button @click="refreshAllPanels" :disabled="!activePanels.length">Inspection 실행</el-button>
        </div>
        <el-table :data="inspectionPanels" class="inspection-table" row-key="id" empty-text="현재 Filter 조건에 해당하는 Panel이 없습니다.">
          <el-table-column :label="uiT('panel')" min-width="180">
            <template #default="{ row }">
              <div class="inspection-name">
                <strong>{{ row.title }}</strong>
                <span>{{ row.description || '설명 없음' }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="상태" width="110">
            <template #default="{ row }">
              <el-tag :type="panelStateType(row)" effect="light">{{ panelState(row) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="현재 값" width="150">
            <template #default="{ row }">
              <strong class="inspection-value">{{ panelDisplayValue(row) }}</strong>
            </template>
          </el-table-column>
          <el-table-column label="반환 Series" width="110">
            <template #default="{ row }">{{ panelResultCount(row) }}</template>
          </el-table-column>
          <el-table-column label="Datasource / Type" width="180">
            <template #default="{ row }">
              <div class="inspection-meta">
                <span>{{ datasourceOptions.find((item) => item.id === selectedDatasourceId)?.name || row.datasourceName || '-' }}</span>
                <b>{{ row.chartType }}</b>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="PromQL" min-width="320" show-overflow-tooltip>
            <template #default="{ row }">
              <code class="inspection-promql">{{ row.promql }}</code>
            </template>
          </el-table-column>
          <el-table-column label="작업" width="190" fixed="right">
            <template #default="{ row }">
              <el-button link type="primary" @click="refreshPanel(row)">Refresh</el-button>
              <el-button link type="primary" @click="openEditPanel(row)">수정</el-button>
              <el-button link type="danger" @click="handleDeletePanel(row)">삭제</el-button>
            </template>
          </el-table-column>
        </el-table>
      </section>

      <section v-else class="dashboard-grid-shell">
        <div class="dashboard-grid-toolbar">
          <div class="dashboard-grid-status">
            <span class="status-chip healthy"><i></i>정상 {{ inspectionSummary.healthy }}</span>
            <span class="status-chip warning"><i></i>확인 필요 {{ inspectionSummary.warning }}</span>
            <span class="status-chip danger"><i></i>이상 {{ inspectionSummary.danger }}</span>
            <span class="status-chip muted"><i></i>비활성 {{ inspectionSummary.disabled }}</span>
          </div>
          <div class="dashboard-grid-actions">
            <span>최근 Refresh {{ lastRefreshText }}</span>
            <el-button size="small" @click="refreshProblemPanels" :disabled="!activePanels.length">이상 재검토</el-button>
          </div>
        </div>
        <div class="panel-grid" :class="{ 'k8s-panel-grid': isK8sDashboard }">
        <div
          v-for="panel in panels"
          :key="panel.id"
          class="metric-panel chart-panel"
           :class="[`panel-${panel.chartType}`, { disabled: panel.status !== 1 }]"
           :style="{ gridColumn: `span ${panelSpan(panel)}` }"
           v-loading="panelPending[panel.id]"
           :element-loading-background="isFullscreen ? 'rgba(8, 15, 28, 0.82)' : 'rgba(255, 255, 255, 0.72)'"
        >
          <div class="panel-glow"></div>
          <div class="panel-head">
            <div class="panel-identity">
              <div class="panel-title-row">
                <i class="panel-signal"></i>
                <strong>{{ panel.title }}</strong>
              </div>
              <span>{{ panelChartLabel(panel.chartType) }} · {{ panelResultCount(panel) }} 개 Series · {{ currentDatasourceName }}</span>
            </div>
            <div class="panel-actions">
              <span class="panel-state" :class="`is-${panelStateType(panel)}`"><i></i>{{ panelState(panel) }}</span>
              <el-button link type="primary" @click="refreshPanel(panel)">Refresh</el-button>
              <el-button link type="primary" @click="openEditPanel(panel)">수정</el-button>
              <el-button link type="danger" @click="handleDeletePanel(panel)">삭제</el-button>
            </div>
          </div>

          <div v-if="panelResults[panel.id]?.error" class="panel-error">{{ panelResults[panel.id].error }}</div>

          <template v-else-if="panel.chartType === 'table'">
            <el-table :data="panelRows(panel)" size="small" :height="isFullscreen ? 126 : 250">
              <el-table-column label="Metric" min-width="240" show-overflow-tooltip>
                <template #default="{ row }">{{ metricText(row.metric) }}</template>
              </el-table-column>
              <el-table-column label="Value" width="130">
                <template #default="{ row }">{{ row.value?.[1] ?? '-' }}</template>
              </el-table-column>
            </el-table>
          </template>

          <template v-else-if="panel.chartType === 'bar'">
            <div class="bar-chart">
              <div v-for="item in barRows(panel)" :key="item.name" class="bar-row">
                <span>{{ item.name }}</span>
                <div><i :style="{ width: `${item.percent}%` }"></i></div>
                <b>{{ item.displayValue }}</b>
              </div>
            </div>
            <div class="panel-query" :title="panel.promql">{{ panel.promql }}</div>
          </template>

          <template v-else-if="panel.chartType === 'gauge'">
            <div class="gauge-wrap">
              <div class="gauge" :style="{ '--value': `${gaugePercent(panel) * 3.6}deg` }">
                <div>
                  <strong>{{ panelValue(panel) }}</strong>
                  <span>{{ panel.unit }}</span>
                </div>
              </div>
            </div>
            <div class="panel-query" :title="panel.promql">{{ panel.promql }}</div>
          </template>

          <template v-else-if="panel.chartType === 'line'">
            <div class="trend-summary">
              <div class="trend-current">
                <span>현재 값</span>
                <strong>{{ panelDisplayValue(panel) }}</strong>
              </div>
              <div class="trend-stats">
                <span>최소 <b>{{ panelStats(panel).min }}</b></span>
                <span>평균 <b>{{ panelStats(panel).avg }}</b></span>
                <span>최대 <b>{{ panelStats(panel).max }}</b></span>
              </div>
            </div>
            <div class="trend-chart">
              <svg class="sparkline" viewBox="0 0 100 52" preserveAspectRatio="none">
                <defs>
                  <linearGradient :id="`trend-area-${panel.id}`" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stop-color="#4f8cff" stop-opacity="0.28" />
                    <stop offset="100%" stop-color="#4f8cff" stop-opacity="0.02" />
                  </linearGradient>
                </defs>
                <line v-for="y in [8, 18, 28, 38, 48]" :key="y" x1="0" :y1="y" x2="100" :y2="y" class="chart-grid-line" />
                <polygon :points="sparklineAreaPoints(panel)" :fill="`url(#trend-area-${panel.id})`" />
                <polyline
                  v-for="(series, index) in panelLineSeries(panel)"
                  :key="`${series.name}-${index}`"
                  class="line-series"
                  :points="series.points"
                  :style="{ stroke: series.color }"
                />
              </svg>
              <div class="trend-axis"><span>시작</span><span>현재</span></div>
            </div>
            <div v-if="panelLineSeries(panel).length > 1" class="trend-legend">
              <span v-for="(series, index) in panelLineSeries(panel).slice(0, 4)" :key="`${series.name}-legend-${index}`">
                <i :style="{ background: series.color }"></i>{{ series.name }}
              </span>
              <span v-if="panelLineSeries(panel).length > 4">+{{ panelLineSeries(panel).length - 4 }}</span>
            </div>
            <div class="panel-query" :title="panel.promql">{{ panel.promql }}</div>
          </template>

          <template v-else>
            <div class="stat-row">
              <div>
                <div class="stat-value">{{ panelValue(panel) }}<small>{{ panel.unit }}</small></div>
                <div class="stat-caption">
                  <span></span>
                  실시간 Sampling
                </div>
              </div>
              <svg v-if="panel.chartType === 'line'" class="sparkline" viewBox="0 0 100 52" preserveAspectRatio="none">
                <polyline :points="sparklinePoints(panel)" />
              </svg>
            </div>
            <div class="promql">{{ panel.promql }}</div>
          </template>
        </div>
        </div>
      </section>
    </main>

    <el-dialog v-model="dashboardDialogVisible" :title="editingDashboard ? `${pageTitle} 수정` : `${pageTitle} 생성`" width="760px">
      <el-form label-width="100px">
        <el-form-item label="이름" required><el-input v-model="dashboardForm.name" :placeholder="`예: Production ${pageTitle}`" /></el-form-item>
        <el-form-item v-if="!editingDashboard" label="Dashboard Template">
          <div class="template-grid">
            <button v-for="item in dashboardTemplates" :key="item.key" type="button" class="template-card" :class="{ active: activeTemplate === item.key }" @click="activeTemplate = item.key">
              <strong>{{ item.name }}</strong>
              <span>{{ item.description }}</span>
            </button>
          </div>
        </el-form-item>
        <el-form-item label="Type">
          <el-tag type="primary" effect="light">{{ isListLayout ? 'Inspection Dashboard / List Inspection' : 'Monitoring Dashboard / Grid Layout' }}</el-tag>
        </el-form-item>
        <el-form-item label="상태">
          <el-radio-group v-model="dashboardForm.status">
            <el-radio :value="1">활성화</el-radio>
            <el-radio :value="2">비활성화</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="설명"><el-input v-model="dashboardForm.description" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dashboardDialogVisible = false">취소</el-button>
        <el-button type="primary" @click="submitDashboard">{{ editingDashboard ? '저장' : '생성' }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="panelDialogVisible" :title="editingPanel ? 'Panel 수정' : 'Panel 추가'" width="820px">
      <el-form label-width="110px">
        <el-form-item label="제목" required><el-input v-model="panelForm.title" /></el-form-item>
        <el-form-item label="Datasource" required>
          <el-select v-model="panelForm.datasourceId" filterable style="width: 100%">
            <el-option v-for="item in datasourceOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="PromQL" required><el-input v-model="panelForm.promql" type="textarea" :rows="4" placeholder="예: up 또는 sum(rate(http_requests_total[5m]))" /></el-form-item>
        <el-row :gutter="12">
          <el-col :span="8">
            <el-form-item label="Chart Type">
              <el-select v-model="panelForm.chartType">
                <el-option label="Metric Card" value="stat" />
                <el-option label="Line Trend" value="line" />
                <el-option label="Bar Ranking" value="bar" />
                <el-option label="Gauge" value="gauge" />
                <el-option label="Table" value="table" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8"><el-form-item :label="uiT('unit')"><el-input v-model="panelForm.unit" placeholder="%, ms, 개" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="Width"><el-input-number v-model="panelForm.span" :min="6" :max="24" :step="6" style="width: 100%" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="8"><el-form-item label="Sort"><el-input-number v-model="panelForm.sort" style="width: 100%" /></el-form-item></el-col>
          <el-col :span="16">
            <el-form-item label="상태">
              <el-radio-group v-model="panelForm.status">
                <el-radio :value="1">활성화</el-radio>
                <el-radio :value="2">비활성화</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="설명"><el-input v-model="panelForm.description" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="panelDialogVisible = false">취소</el-button>
        <el-button type="primary" @click="submitPanel">저장</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.dashboard-page {
  display: flex;
  flex-direction: column;
  gap: 18px;
  min-height: calc(100vh - 170px);
}
.dashboard-workspace {
  display: grid;
  grid-template-columns: minmax(260px, 360px) minmax(0, 1fr) auto;
  align-items: center;
  gap: 16px;
  padding: 16px 18px;
  border-radius: 18px;
  background: #fff;
  border: 1px solid #dce7f7;
  box-shadow: 0 10px 24px rgba(36, 54, 90, 0.07);
}
.workspace-title {
  display: flex;
  align-items: center;
  gap: 12px;
}
.brand-mark {
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  width: 40px;
  height: 40px;
  border-radius: 10px;
  background: linear-gradient(135deg, #2563eb, #60a5fa);
  color: #fff;
  font-weight: 800;
}
.workspace-title strong,
.workspace-title p {
  display: block;
  margin: 0;
}
.workspace-title strong {
  color: #10213f;
  font-size: 16px;
}
.workspace-title p {
  margin-top: 4px;
  color: #7282a0;
  font-size: 13px;
}
.create-screen-btn {
  min-width: 132px;
  height: 42px;
}
.dashboard-switcher {
  min-width: 0;
  padding: 8px;
  border-radius: 14px;
  background: #f3f7fd;
  border: 1px solid #e2ebf7;
}
.dashboard-list {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 54px;
}
.dashboard-item {
  flex: 0 0 190px;
  padding: 10px 12px;
  border: 1px solid transparent;
  border-radius: 12px;
  background: transparent;
  color: #41516e;
  text-align: left;
  cursor: pointer;
  transition: 0.18s ease;
}
.dashboard-item strong,
.dashboard-item span {
  display: block;
}
.dashboard-item span {
  margin-top: 5px;
  color: #8a99b4;
  font-size: 12px;
}
.dashboard-item.active,
.dashboard-item:hover {
  background: #fff;
  border-color: #d7e4f5;
  color: #17335f;
  box-shadow: 0 8px 18px rgba(47, 99, 191, 0.1);
}
.dashboard-item.active span,
.dashboard-item:hover span {
  color: #6b7b95;
}
.empty-switcher {
  padding: 0 14px;
  color: #8a99b4;
  white-space: nowrap;
}
.dashboard-main {
  position: relative;
  padding: 22px;
  min-height: 760px;
  border-radius: 18px;
  background:
    radial-gradient(circle at 12% 8%, rgba(37, 99, 235, 0.14), transparent 28%),
    linear-gradient(180deg, #edf4ff 0%, #e7eef9 100%);
  box-shadow: 0 12px 30px rgba(36, 54, 90, 0.08);
  overflow: auto;
}
.dashboard-main:fullscreen {
  border-radius: 0;
  background: #07111f;
  padding: 28px;
}
.dashboard-main:fullscreen .dashboard-hero,
.dashboard-main:fullscreen .dashboard-summary,
.dashboard-main:fullscreen .metric-panel {
  background: rgba(15, 23, 42, 0.92);
  border-color: rgba(96, 165, 250, 0.28);
  color: #dbeafe;
}
.dashboard-hero {
  display: flex;
  justify-content: space-between;
  gap: 18px;
  padding: 24px;
  margin-bottom: 16px;
  border: 1px solid #dce7f7;
  border-radius: 16px;
  background:
    linear-gradient(135deg, rgba(255, 255, 255, 0.96) 0%, rgba(248, 251, 255, 0.92) 58%, rgba(230, 241, 255, 0.92) 100%),
    radial-gradient(circle at right top, rgba(37, 99, 235, 0.18), transparent 36%);
}
.eyebrow {
  margin-bottom: 8px;
  color: #2f63bf;
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}
.dashboard-hero h2 {
  margin: 0 0 8px;
  color: #0f1f3d;
  font-size: 30px;
}
.dashboard-hero p {
  margin: 0;
  color: #6f7f9b;
}
.layout-hint {
  display: inline-flex;
  margin-top: 12px;
  padding: 6px 10px;
  border-radius: 999px;
  color: #2f63bf;
  background: rgba(47, 99, 191, 0.09);
  font-size: 12px;
  font-weight: 700;
}
.hero-actions {
  display: flex;
  align-items: flex-start;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 10px;
  min-width: 520px;
}
.dashboard-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(160px, 1fr));
  gap: 14px;
  margin-bottom: 16px;
}
.summary-card {
  position: relative;
  padding: 16px;
  border: 1px solid #dce7f7;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.9);
  overflow: hidden;
}
.summary-card::after {
  content: '';
  position: absolute;
  right: 14px;
  bottom: 12px;
  width: 54px;
  height: 8px;
  border-radius: 999px;
  background: linear-gradient(90deg, rgba(37, 99, 235, 0.16), rgba(34, 197, 94, 0.22));
}
.summary-card span {
  display: block;
  color: #7888a6;
}
.summary-card strong {
  display: block;
  margin-top: 8px;
  color: #0f1f3d;
  font-size: 24px;
}
.state-success { color: #16a34a !important; }
.state-danger { color: #dc2626 !important; }
.state-info { color: #64748b !important; }
.inspection-list {
  padding: 18px;
  border: 1px solid rgba(169, 190, 222, 0.72);
  border-radius: 18px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.96), rgba(250, 253, 255, 0.92)),
    radial-gradient(circle at 90% 0%, rgba(37, 99, 235, 0.12), transparent 32%);
  box-shadow: 0 14px 32px rgba(31, 54, 92, 0.08);
}
.inspection-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}
.inspection-head h3 {
  margin: 0 0 6px;
  color: #10213f;
  font-size: 20px;
}
.inspection-head p {
  margin: 0;
  color: #7282a0;
}
.inspection-command-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  margin: -2px 0 18px;
  padding: 10px 12px;
  border: 1px solid #e0e9f6;
  border-radius: 12px;
  background: linear-gradient(100deg, #f8fbff, #f1f6ff);
}
.inspection-command-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 8px;
  color: #7282a0;
  font-size: 12px;
}
.inspection-filter {
  display: inline-flex;
  padding: 3px;
  border: 1px solid #dce7f7;
  border-radius: 10px;
  background: #eef4fc;
}
.inspection-filter button {
  padding: 6px 9px;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: #667895;
  cursor: pointer;
  font-size: 12px;
}
.inspection-filter button:hover,
.inspection-filter button.active {
  background: #fff;
  color: #2463d4;
  box-shadow: 0 2px 6px rgba(36, 99, 212, 0.12);
}
.inspection-table {
  border-radius: 14px;
  overflow: hidden;
}
.inspection-name strong,
.inspection-name span,
.inspection-meta span,
.inspection-meta b {
  display: block;
}
.inspection-name strong {
  color: #10213f;
}
.inspection-name span {
  margin-top: 4px;
  color: #8a99b4;
  font-size: 12px;
}
.inspection-value {
  color: #1554d1;
  font-size: 18px;
}
.inspection-meta span {
  color: #334155;
}
.inspection-meta b {
  margin-top: 3px;
  color: #8a99b4;
  font-size: 12px;
  font-weight: 600;
}
.inspection-promql {
  display: inline-block;
  max-width: 100%;
  padding: 5px 8px;
  border-radius: 8px;
  background: #0f172a;
  color: #b9d6ff;
  font-family: Consolas, Monaco, monospace;
  font-size: 12px;
}
.panel-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(240px, 1fr));
  gap: 14px;
}
.dashboard-grid-shell {
  min-width: 0;
}
.dashboard-grid-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
  padding: 10px 12px;
  border: 1px solid rgba(169, 190, 222, 0.72);
  border-radius: 13px;
  background: rgba(255, 255, 255, 0.74);
  backdrop-filter: blur(12px);
}
.dashboard-grid-status,
.dashboard-grid-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}
.dashboard-grid-actions {
  justify-content: flex-end;
  color: #71829e;
  font-size: 12px;
}
.status-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 9px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 700;
}
.status-chip i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}
.status-chip.healthy { color: #16834b; background: #eaf8ef; }
.status-chip.warning { color: #a56608; background: #fff7df; }
.status-chip.danger { color: #c24141; background: #fff0f0; }
.status-chip.muted { color: #64748b; background: #eef2f7; }
.k8s-dashboard-main {
  background:
    radial-gradient(circle at 10% 8%, rgba(34, 197, 94, 0.12), transparent 24%),
    radial-gradient(circle at 78% 0%, rgba(59, 130, 246, 0.16), transparent 26%),
    linear-gradient(180deg, #0b1117 0%, #111820 100%);
  border: 1px solid #26313c;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04), 0 18px 34px rgba(2, 6, 23, 0.28);
}
.k8s-dashboard-main .dashboard-hero,
.k8s-dashboard-main .dashboard-summary {
  border-color: #26313c;
  background:
    linear-gradient(180deg, rgba(20, 29, 38, 0.96), rgba(15, 23, 31, 0.94)),
    radial-gradient(circle at right top, rgba(34, 197, 94, 0.12), transparent 32%);
  box-shadow: none;
}
.k8s-dashboard-main .eyebrow {
  color: #93c5fd;
}
.k8s-dashboard-main .dashboard-hero h2,
.k8s-dashboard-main .panel-head strong,
.k8s-dashboard-main .summary-card strong {
  color: #dbeafe;
}
.k8s-dashboard-main .dashboard-hero p,
.k8s-dashboard-main .layout-hint,
.k8s-dashboard-main .panel-head span,
.k8s-dashboard-main .summary-card span {
  color: #94a3b8;
}
.k8s-dashboard-main .layout-hint {
  background: rgba(34, 197, 94, 0.1);
  color: #86efac;
}
.k8s-dashboard-main .summary-card {
  border-color: #26313c;
  background: linear-gradient(180deg, rgba(18, 27, 36, 0.92), rgba(12, 18, 26, 0.92));
}
.k8s-dashboard-main .summary-card::after {
  background: linear-gradient(90deg, #22c55e, #eab308, #3b82f6);
  opacity: 0.72;
}
.k8s-panel-grid {
  grid-template-columns: repeat(6, minmax(180px, 1fr));
  gap: 6px;
}
.metric-panel {
  position: relative;
  min-height: 260px;
  padding: 18px;
  border: 1px solid rgba(169, 190, 222, 0.72);
  border-radius: 18px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.96), rgba(250, 253, 255, 0.92)),
    radial-gradient(circle at 80% 0%, rgba(37, 99, 235, 0.16), transparent 32%);
  box-shadow: 0 14px 32px rgba(31, 54, 92, 0.08);
  overflow: hidden;
}
.metric-panel::before {
  content: '';
  position: absolute;
  inset: 0;
  border-top: 3px solid rgba(37, 99, 235, 0.72);
  pointer-events: none;
}
.panel-glow {
  position: absolute;
  right: -36px;
  top: -46px;
  width: 128px;
  height: 128px;
  border-radius: 50%;
  background: rgba(37, 99, 235, 0.12);
  filter: blur(2px);
  pointer-events: none;
}
.panel-gauge::before {
  border-top-color: rgba(34, 197, 94, 0.78);
}
.panel-bar::before {
  border-top-color: rgba(14, 165, 233, 0.82);
}
.panel-line::before {
  border-top-color: rgba(99, 102, 241, 0.82);
}
.panel-table::before {
  border-top-color: rgba(100, 116, 139, 0.76);
}
.k8s-metric-panel {
  min-height: 168px;
  padding: 11px;
  border-radius: 4px;
  border-color: #293541;
  background:
    linear-gradient(180deg, rgba(21, 30, 38, 0.96), rgba(13, 19, 26, 0.98)),
    radial-gradient(circle at 80% 0%, rgba(34, 197, 94, 0.08), transparent 34%);
  box-shadow: none;
}
.k8s-metric-panel::before {
  border-top-width: 2px;
  border-top-color: #22c55e;
}
.k8s-metric-panel.panel-gauge::before {
  border-top-color: #ef4444;
}
.k8s-metric-panel.panel-bar::before {
  border-top-color: #eab308;
}
.k8s-metric-panel.panel-line::before {
  border-top-color: #3b82f6;
}
.k8s-metric-panel .panel-glow {
  display: none;
}
.k8s-metric-panel .panel-head {
  margin-bottom: 10px;
}
.k8s-metric-panel .panel-head strong {
  font-size: 13px;
}
.k8s-metric-panel .panel-head span {
  margin-top: 3px;
  font-size: 11px;
}
.k8s-metric-panel .panel-actions {
  opacity: 0;
  transition: opacity 0.15s ease;
}
.k8s-metric-panel:hover .panel-actions {
  opacity: 1;
}
.k8s-metric-panel .stat-row {
  display: block;
}
.k8s-metric-panel .stat-value {
  color: #4ade80;
  font-size: 32px;
  text-shadow: 0 0 18px rgba(74, 222, 128, 0.18);
}
.k8s-metric-panel.panel-gauge .stat-value,
.k8s-metric-panel.panel-gauge .gauge strong {
  color: #f87171;
}
.k8s-metric-panel .stat-caption {
  margin-top: 10px;
  color: #94a3b8;
  background: rgba(148, 163, 184, 0.1);
}
.k8s-metric-panel .promql {
  margin-top: 10px;
  padding: 7px 9px;
  border: 1px solid #26313c;
  border-radius: 4px;
  background: #070b10;
  color: #93c5fd;
  font-size: 11px;
}
.k8s-metric-panel .bar-chart {
  gap: 8px;
}
.k8s-metric-panel .bar-row {
  grid-template-columns: 118px 1fr 64px;
  gap: 8px;
  color: #a9b7c6;
  font-size: 11px;
}
.k8s-metric-panel .bar-row div {
  height: 8px;
  background: #1d2732;
}
.k8s-metric-panel .bar-row i {
  background: linear-gradient(90deg, #22c55e, #eab308, #ef4444);
  box-shadow: none;
}
.k8s-metric-panel .bar-row b {
  color: #dbeafe;
}
.k8s-metric-panel .gauge-wrap {
  min-height: 118px;
}
.k8s-metric-panel .gauge {
  width: 118px;
  height: 118px;
  background: conic-gradient(#ef4444 var(--value), #1d2732 0deg);
  box-shadow: none;
}
.k8s-metric-panel .gauge > div {
  width: 82px;
  height: 82px;
  background: #0e141b;
}
.k8s-metric-panel .gauge strong {
  font-size: 23px;
}
.k8s-metric-panel .sparkline {
  height: 72px;
  margin-top: 10px;
}
.k8s-metric-panel .sparkline polyline {
  stroke: #4ade80;
  stroke-width: 2.5;
}
.k8s-metric-panel .el-table {
  --el-table-bg-color: #0e141b;
  --el-table-tr-bg-color: #0e141b;
  --el-table-header-bg-color: #111c27;
  --el-table-border-color: #26313c;
  --el-table-text-color: #cbd5e1;
  --el-table-header-text-color: #93c5fd;
}
.k8s-metric-panel.panel-table {
  min-height: 260px;
}
.k8s-dashboard-main:fullscreen {
  background: #080d13;
}
.k8s-dashboard-main:fullscreen .k8s-metric-panel {
  background: linear-gradient(180deg, rgba(21, 30, 38, 0.96), rgba(13, 19, 26, 0.98));
  border-color: #293541;
}
.metric-panel.disabled {
  opacity: 0.55;
}
.panel-head {
  position: relative;
  z-index: 1;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 10px;
  margin-bottom: 18px;
}
.panel-head strong {
  display: block;
  color: #10213f;
  font-size: 16px;
}
.panel-head span {
  display: block;
  margin-top: 5px;
  color: #8391aa;
  font-size: 12px;
}
.panel-actions {
  display: flex;
  gap: 4px;
  white-space: nowrap;
}
.stat-row {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: 180px 1fr;
  align-items: center;
  gap: 18px;
}
.stat-value {
  color: #1554d1;
  font-size: 44px;
  font-weight: 850;
  line-height: 1.1;
  text-shadow: 0 8px 24px rgba(37, 99, 235, 0.18);
}
.stat-value small {
  margin-left: 6px;
  color: #73829f;
  font-size: 14px;
}
.stat-caption {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  margin-top: 12px;
  padding: 5px 10px;
  border-radius: 999px;
  color: #64748b;
  background: #eef5ff;
  font-size: 12px;
}
.stat-caption span {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #22c55e;
  box-shadow: 0 0 0 4px rgba(34, 197, 94, 0.14);
}
.sparkline {
  width: 100%;
  height: 92px;
}
.sparkline polyline {
  fill: none;
  stroke: #3b82f6;
  stroke-width: 4;
  stroke-linecap: round;
  stroke-linejoin: round;
}
.promql {
  position: relative;
  z-index: 1;
  margin-top: 18px;
  padding: 10px;
  border-radius: 10px;
  background: #0f172a;
  color: #b9d6ff;
  font-family: Consolas, Monaco, monospace;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
}
.panel-error {
  padding: 12px;
  border-radius: 10px;
  color: #b91c1c;
  background: #fff1f2;
  word-break: break-all;
}
.bar-chart {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.bar-row {
  display: grid;
  grid-template-columns: 160px 1fr 84px;
  align-items: center;
  gap: 10px;
  color: #566781;
  font-size: 13px;
}
.bar-row span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.bar-row div {
  height: 11px;
  overflow: hidden;
  border-radius: 999px;
  background: rgba(226, 235, 247, 0.9);
}
.bar-row i {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, #2563eb, #06b6d4 48%, #22c55e);
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.18);
}
.bar-row b {
  color: #10213f;
  text-align: right;
}
.gauge-wrap {
  position: relative;
  z-index: 1;
  display: grid;
  place-items: center;
  min-height: 185px;
}
.gauge {
  position: relative;
  display: grid;
  place-items: center;
  width: 180px;
  height: 180px;
  border-radius: 50%;
  background: conic-gradient(#2563eb var(--value), #e5edf8 0deg);
  box-shadow: inset 0 0 0 1px rgba(37, 99, 235, 0.08), 0 18px 32px rgba(37, 99, 235, 0.16);
}
.gauge::before {
  content: '';
  position: absolute;
}
.gauge > div {
  display: grid;
  place-items: center;
  width: 126px;
  height: 126px;
  border-radius: 50%;
  background: #fff;
}
.gauge strong {
  color: #10213f;
  font-size: 34px;
}
.gauge span {
  color: #75859f;
  font-size: 13px;
}
.template-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  width: 100%;
}
.template-card {
  min-height: 118px;
  padding: 14px;
  border: 1px solid #dce7f7;
  border-radius: 12px;
  background: #f8fbff;
  color: #263b5f;
  text-align: left;
  cursor: pointer;
}
.template-card strong,
.template-card span {
  display: block;
}
.template-card span {
  margin-top: 8px;
  color: #7282a0;
  line-height: 1.5;
}
.template-card.active {
  border-color: #3b82f6;
  background: #eff6ff;
  box-shadow: inset 0 0 0 1px #3b82f6;
}

/* Grafana/Nightingale inspired observability workspace. */
.dashboard-page {
  gap: 14px;
}
.dashboard-workspace {
  padding: 12px 14px;
  border-color: #d8e0eb;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(15, 23, 42, 0.04);
}
.dashboard-switcher {
  padding: 4px;
  border-color: #dce4ef;
  border-radius: 6px;
  background: #f4f7fb;
}
.dashboard-list {
  min-height: 46px;
  gap: 4px;
}
.dashboard-item {
  flex-basis: 176px;
  padding: 8px 10px;
  border-radius: 5px;
}
.dashboard-item.active,
.dashboard-item:hover {
  box-shadow: 0 1px 4px rgba(30, 64, 175, 0.08);
}
.observability-canvas {
  padding: 14px;
  border: 1px solid #d9e1ec;
  border-radius: 8px;
  background: #f2f5f9;
  box-shadow: none;
}
.observability-canvas .dashboard-hero {
  align-items: flex-start;
  padding: 16px 18px;
  margin-bottom: 10px;
  border-color: #d9e1ec;
  border-radius: 6px;
  background: #fff;
  box-shadow: none;
}
.observability-canvas .eyebrow {
  margin-bottom: 4px;
  color: #2563eb;
  font-size: 10px;
  letter-spacing: 0.06em;
}
.observability-canvas .dashboard-hero h2 {
  margin-bottom: 4px;
  font-size: 22px;
  letter-spacing: 0;
}
.observability-canvas .dashboard-hero p {
  font-size: 13px;
}
.observability-canvas .layout-hint {
  display: none;
}
.dashboard-context {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-top: 10px;
  color: #64748b;
  font-size: 12px;
}
.dashboard-context span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.health-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #22c55e;
  box-shadow: 0 0 0 3px rgba(34, 197, 94, 0.12);
}
.observability-canvas .hero-actions {
  align-items: center;
  min-width: 0;
  max-width: 760px;
  gap: 6px;
}
.observability-canvas .hero-actions :deep(.el-button + .el-button) {
  margin-left: 0;
}
.observability-canvas .dashboard-summary {
  grid-template-columns: repeat(4, minmax(130px, 1fr));
  gap: 8px;
  margin-bottom: 10px;
}
.observability-canvas .summary-card {
  min-height: 72px;
  padding: 12px 14px;
  border-color: #d9e1ec;
  border-radius: 6px;
  background: #fff;
}
.observability-canvas .summary-card::after {
  right: 12px;
  bottom: 11px;
  width: 34px;
  height: 3px;
  background: #3b82f6;
  opacity: 0.55;
}
.observability-canvas .summary-card span {
  font-size: 12px;
}
.observability-canvas .summary-card strong {
  margin-top: 5px;
  font-size: 21px;
}
.panel-grid {
  gap: 10px;
}
.chart-panel {
  min-height: 238px;
  padding: 0 0 34px;
  border: 1px solid #d8e0eb;
  border-radius: 6px;
  background: #fff;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.035);
  transition: border-color 0.16s ease, box-shadow 0.16s ease;
}
.chart-panel:hover {
  border-color: #b9c8dc;
  box-shadow: 0 4px 12px rgba(15, 23, 42, 0.07);
}
.chart-panel::before {
  display: none;
}
.chart-panel .panel-glow {
  display: none;
}
.chart-panel .panel-head {
  align-items: center;
  min-height: 55px;
  margin: 0;
  padding: 10px 12px;
  border-bottom: 1px solid #e4e9f0;
  background: #fbfcfe;
}
.panel-identity {
  min-width: 0;
}
.panel-title-row {
  display: flex;
  align-items: center;
  gap: 7px;
}
.panel-title-row strong {
  overflow: hidden;
  color: #172033;
  font-size: 14px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.panel-signal {
  width: 3px;
  height: 15px;
  border-radius: 2px;
  background: #3b82f6;
}
.chart-panel.panel-gauge .panel-signal { background: #f59e0b; }
.chart-panel.panel-bar .panel-signal { background: #14b8a6; }
.chart-panel.panel-table .panel-signal { background: #64748b; }
.chart-panel .panel-head span {
  margin-top: 3px;
  color: #8491a5;
  font-size: 11px;
}
.chart-panel .panel-actions {
  align-items: center;
  opacity: 1;
}
.chart-panel .panel-actions :deep(.el-button) {
  opacity: 0;
  transition: opacity 0.15s ease;
}
.chart-panel:hover .panel-actions :deep(.el-button) {
  opacity: 1;
}
.panel-state {
  display: inline-flex !important;
  align-items: center;
  gap: 5px;
  margin: 0 5px 0 0 !important;
  color: #64748b !important;
  white-space: nowrap;
}
.panel-state i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #94a3b8;
}
.panel-state.is-success i { background: #22c55e; }
.panel-state.is-warning i { background: #f59e0b; }
.panel-state.is-danger i { background: #ef4444; }
.chart-panel .bar-chart,
.chart-panel .gauge-wrap,
.chart-panel .stat-row,
.chart-panel .panel-error {
  margin: 14px;
}
.chart-panel .bar-chart {
  max-height: 172px;
  gap: 9px;
  overflow: auto;
}
.chart-panel .bar-row {
  grid-template-columns: minmax(90px, 132px) 1fr 76px;
  font-size: 12px;
}
.chart-panel .bar-row div {
  height: 7px;
  border-radius: 2px;
  background: #edf1f6;
}
.chart-panel .bar-row i {
  border-radius: 2px;
  background: linear-gradient(90deg, #2563eb, #14b8a6);
  box-shadow: none;
}
.chart-panel .gauge-wrap {
  min-height: 142px;
}
.chart-panel .gauge {
  width: 138px;
  height: 138px;
  box-shadow: none;
}
.chart-panel .gauge > div {
  width: 100px;
  height: 100px;
}
.chart-panel .gauge strong {
  font-size: 28px;
}
.chart-panel .stat-row {
  display: block;
}
.chart-panel .stat-value {
  font-size: 36px;
  text-shadow: none;
}
.trend-summary {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 14px 0;
}
.trend-current span,
.trend-current strong {
  display: block;
}
.trend-current span {
  color: #7c899d;
  font-size: 11px;
}
.trend-current strong {
  margin-top: 2px;
  color: #172033;
  font-size: 24px;
}
.trend-stats {
  display: flex;
  gap: 14px;
  color: #8a96a8;
  font-size: 10px;
}
.trend-stats b {
  margin-left: 3px;
  color: #475569;
  font-weight: 600;
}
.trend-chart {
  padding: 5px 14px 0;
}
.trend-chart .sparkline {
  display: block;
  height: 104px;
}
.trend-chart .sparkline polyline {
  stroke-width: 1.8;
  vector-effect: non-scaling-stroke;
}
.chart-grid-line {
  stroke: #e7ecf3;
  stroke-width: 0.5;
  vector-effect: non-scaling-stroke;
}
.trend-axis {
  display: flex;
  justify-content: space-between;
  margin-top: 2px;
  color: #a0aaba;
  font-size: 9px;
}
.trend-legend {
  display: flex;
  gap: 10px;
  min-height: 16px;
  padding: 2px 14px 0;
  overflow: hidden;
  color: #64748b;
  font-size: 9px;
  white-space: nowrap;
}
.trend-legend span {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  max-width: 150px;
  overflow: hidden;
  text-overflow: ellipsis;
}
.trend-legend i {
  flex: 0 0 auto;
  width: 7px;
  height: 7px;
  border-radius: 50%;
}
.panel-query,
.chart-panel .promql {
  position: absolute;
  right: 12px;
  bottom: 8px;
  left: 12px;
  z-index: 1;
  margin: 0;
  padding: 4px 7px;
  overflow: hidden;
  border: 1px solid #e0e7f0;
  border-radius: 3px;
  background: #f6f8fb;
  color: #64748b;
  font-family: Consolas, Monaco, monospace;
  font-size: 10px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.chart-panel.panel-table {
  padding-bottom: 10px;
}
.chart-panel.panel-table .el-table {
  padding: 10px 12px 0;
}

/* Fullscreen is a dedicated Grafana-style presentation mode. */
.observability-canvas:fullscreen {
  padding: 12px;
  border: 0;
  border-radius: 0;
  background: #0b1017;
  color: #d7e0ea;
}
.observability-canvas:fullscreen .dashboard-hero {
  align-items: center;
  min-height: 94px;
  padding: 12px 16px;
  border-color: #283442;
  background: #111923;
}
.observability-canvas:fullscreen .eyebrow {
  color: #60a5fa;
}
.observability-canvas:fullscreen .dashboard-hero h2 {
  color: #f1f5f9;
}
.observability-canvas:fullscreen .dashboard-hero p,
.observability-canvas:fullscreen .dashboard-context {
  color: #8291a5;
}
.observability-canvas:fullscreen .hero-actions {
  max-width: none;
}
.observability-canvas:fullscreen .hero-actions :deep(.el-input__wrapper),
.observability-canvas:fullscreen .hero-actions :deep(.el-select__wrapper) {
  border: 1px solid #334155;
  background: #0c131c;
  box-shadow: none;
}
.observability-canvas:fullscreen .hero-actions :deep(.el-input__inner),
.observability-canvas:fullscreen .hero-actions :deep(.el-select__selected-item),
.observability-canvas:fullscreen .hero-actions :deep(.el-select__placeholder) {
  color: #cbd5e1;
}
.observability-canvas:fullscreen .hero-actions :deep(.el-button) {
  border-color: #334155;
  background: #17202c;
  color: #d6e0eb;
}
.observability-canvas:fullscreen .hero-actions :deep(.el-button:hover) {
  border-color: #4f8cff;
  background: #1c2a3d;
  color: #fff;
}
.observability-canvas:fullscreen .dashboard-summary {
  gap: 6px;
}
.observability-canvas:fullscreen .summary-card {
  min-height: 64px;
  border-color: #283442;
  background: #111923;
}
.observability-canvas:fullscreen .summary-card::after {
  display: none;
}
.observability-canvas:fullscreen .summary-card span {
  color: #7f8ea3;
}
.observability-canvas:fullscreen .summary-card strong {
  color: #e7edf5;
}
.observability-canvas:fullscreen .panel-grid {
  gap: 6px;
}
.observability-canvas:fullscreen .chart-panel {
  min-height: 224px;
  padding-bottom: 8px;
  border-color: #283442;
  background: #111923;
  box-shadow: none;
}
.observability-canvas:fullscreen .chart-panel:hover {
  border-color: #3b4c60;
  box-shadow: none;
}
.observability-canvas:fullscreen .chart-panel .panel-head {
  min-height: 50px;
  padding: 8px 10px;
  border-color: #283442;
  background: #151e29;
}
.observability-canvas:fullscreen .panel-title-row strong,
.observability-canvas:fullscreen .trend-current strong,
.observability-canvas:fullscreen .chart-panel .bar-row b {
  color: #e5edf6;
}
.observability-canvas:fullscreen .chart-panel .panel-head span,
.observability-canvas:fullscreen .trend-current span,
.observability-canvas:fullscreen .trend-stats,
.observability-canvas:fullscreen .trend-axis,
.observability-canvas:fullscreen .trend-legend,
.observability-canvas:fullscreen .chart-panel .bar-row {
  color: #8493a7;
}
.observability-canvas:fullscreen .trend-stats b {
  color: #bdc9d8;
}
.observability-canvas:fullscreen .chart-panel .panel-actions :deep(.el-button) {
  display: none;
}
.observability-canvas:fullscreen .chart-panel .stat-value {
  color: #60a5fa;
}
.observability-canvas:fullscreen .chart-panel .stat-value small {
  color: #7f8ea3;
}
.observability-canvas:fullscreen .chart-panel.panel-stat .stat-row {
  margin-top: 34px;
}
.observability-canvas:fullscreen .chart-panel .stat-caption {
  color: #9aabc0;
  background: #182330;
}
.observability-canvas:fullscreen .chart-panel .gauge {
  background: conic-gradient(#3b82f6 var(--value), #263241 0deg);
}
.observability-canvas:fullscreen .chart-panel .gauge > div {
  background: #111923;
}
.observability-canvas:fullscreen .chart-panel .gauge strong {
  color: #e8eef6;
}
.observability-canvas:fullscreen .chart-panel .gauge span {
  color: #7f8ea3;
}
.observability-canvas:fullscreen .chart-panel .bar-row div {
  background: #263241;
}
.observability-canvas:fullscreen .chart-grid-line {
  stroke: #263241;
}
.observability-canvas:fullscreen .panel-query,
.observability-canvas:fullscreen .chart-panel .promql {
  display: none;
}
.observability-canvas:fullscreen .chart-panel .el-table {
  --el-table-bg-color: #111923;
  --el-table-tr-bg-color: #111923;
  --el-table-header-bg-color: #151e29;
  --el-table-border-color: #283442;
  --el-table-text-color: #c7d2df;
  --el-table-header-text-color: #8fa0b5;
}
.observability-canvas:fullscreen .panel-error {
  border: 1px solid rgba(239, 68, 68, 0.26);
  background: rgba(127, 29, 29, 0.2);
  color: #fca5a5;
}

/* Dense fullscreen layout: prioritize the monitoring surface over page chrome. */
.observability-canvas:fullscreen {
  padding: 8px;
}
.observability-canvas:fullscreen .dashboard-hero {
  min-height: 62px;
  margin-bottom: 6px;
  padding: 8px 12px;
}
.observability-canvas:fullscreen .dashboard-hero p,
.observability-canvas:fullscreen .eyebrow {
  display: none;
}
.observability-canvas:fullscreen .dashboard-hero h2 {
  margin: 0;
  font-size: 18px;
}
.observability-canvas:fullscreen .dashboard-context {
  gap: 10px;
  margin-top: 5px;
  font-size: 10px;
}
.observability-canvas:fullscreen .hero-actions {
  flex-wrap: nowrap;
  gap: 4px;
}
.observability-canvas:fullscreen .hero-actions :deep(.el-select) {
  width: 128px !important;
}
.observability-canvas:fullscreen .hero-actions :deep(.el-select:first-child) {
  width: 150px !important;
}
.observability-canvas:fullscreen .hero-actions :deep(.el-select__wrapper),
.observability-canvas:fullscreen .hero-actions :deep(.el-button) {
  min-height: 30px;
  height: 30px;
  padding-top: 4px;
  padding-bottom: 4px;
  font-size: 12px;
}
.observability-canvas:fullscreen .dashboard-summary {
  display: none;
}
.observability-canvas:fullscreen .dashboard-grid-toolbar {
  margin-bottom: 6px;
  padding: 5px 8px;
  border-color: rgba(96, 165, 250, 0.24);
  background: rgba(15, 23, 42, 0.9);
}
.observability-canvas:fullscreen .status-chip {
  padding: 3px 7px;
  font-size: 10px;
}
.observability-canvas:fullscreen .panel-grid {
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 5px;
}
.observability-canvas:fullscreen .chart-panel {
  min-height: 164px;
}
.observability-canvas:fullscreen .chart-panel.panel-table {
  grid-column: span 3 !important;
  min-height: 174px;
}
.observability-canvas:fullscreen .chart-panel .panel-head {
  min-height: 40px;
  padding: 6px 8px;
}
.observability-canvas:fullscreen .panel-title-row {
  gap: 5px;
}
.observability-canvas:fullscreen .panel-title-row strong {
  font-size: 12px;
}
.observability-canvas:fullscreen .panel-signal {
  height: 13px;
}
.observability-canvas:fullscreen .panel-identity > span {
  display: none;
}
.observability-canvas:fullscreen .panel-state {
  margin-right: 0 !important;
  font-size: 9px !important;
}
.observability-canvas:fullscreen .chart-panel.panel-stat .stat-row,
.observability-canvas:fullscreen .chart-panel .stat-row {
  margin: 20px 10px 8px;
}
.observability-canvas:fullscreen .chart-panel .stat-value {
  font-size: 30px;
}
.observability-canvas:fullscreen .chart-panel .stat-caption {
  margin-top: 7px;
  padding: 3px 7px;
  font-size: 9px;
}
.observability-canvas:fullscreen .chart-panel .gauge-wrap {
  min-height: 112px;
  margin: 6px;
}
.observability-canvas:fullscreen .chart-panel .gauge {
  width: 96px;
  height: 96px;
}
.observability-canvas:fullscreen .chart-panel .gauge > div {
  width: 68px;
  height: 68px;
}
.observability-canvas:fullscreen .chart-panel .gauge strong {
  font-size: 20px;
}
.observability-canvas:fullscreen .chart-panel .gauge span {
  font-size: 10px;
}
.observability-canvas:fullscreen .chart-panel .bar-chart {
  max-height: 112px;
  margin: 8px 10px;
  gap: 5px;
}
.observability-canvas:fullscreen .chart-panel .bar-row {
  grid-template-columns: minmax(70px, 112px) 1fr 58px;
  gap: 6px;
  font-size: 10px;
}
.observability-canvas:fullscreen .chart-panel .bar-row div {
  height: 5px;
}
.observability-canvas:fullscreen .trend-summary {
  padding: 7px 9px 0;
}
.observability-canvas:fullscreen .trend-current strong {
  font-size: 17px;
}
.observability-canvas:fullscreen .trend-stats {
  gap: 7px;
  font-size: 8px;
}
.observability-canvas:fullscreen .trend-chart {
  padding: 2px 9px 0;
}
.observability-canvas:fullscreen .trend-chart .sparkline {
  height: 70px;
}
.observability-canvas:fullscreen .trend-legend {
  display: none;
}
.observability-canvas:fullscreen .chart-panel.panel-table .el-table {
  padding: 5px 7px 0;
  font-size: 10px;
}
@media (max-width: 1280px) {
  .dashboard-workspace {
    grid-template-columns: 1fr;
    align-items: stretch;
  }
  .hero-actions {
    min-width: 0;
  }
  .create-screen-btn {
    width: 100%;
  }
  .panel-grid,
  .dashboard-summary {
    grid-template-columns: repeat(2, minmax(220px, 1fr));
  }
  .dashboard-grid-toolbar {
    align-items: flex-start;
    flex-direction: column;
  }
  .dashboard-grid-actions { justify-content: flex-start; }
}
@media (max-width: 760px) {
  .inspection-command-bar {
    align-items: flex-start;
    flex-direction: column;
  }
  .inspection-command-actions { justify-content: flex-start; }
}
</style>
