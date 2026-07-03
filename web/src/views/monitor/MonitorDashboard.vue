<script setup>
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
const panelLoading = ref(false)
const dashboards = ref([])
const datasourceOptions = ref([])
const selectedDatasourceId = ref()
const activeDashboardId = ref()
const activeDashboard = ref(null)
const panels = ref([])
const panelResults = reactive({})
const dashboardDialogVisible = ref(false)
const panelDialogVisible = ref(false)
const editingDashboard = ref(false)
const editingPanel = ref(false)
const activeTemplate = ref('blank')
const autoRefreshSeconds = ref(30)
const isFullscreen = ref(false)
let refreshTimer = null

const pageMode = computed(() => route.path.includes('/monitor/inspections') ? 'inspection' : 'dashboard')
const pageLayout = computed(() => pageMode.value === 'inspection' ? 'list' : 'grid')
const pageTitle = computed(() => pageMode.value === 'inspection' ? '巡检大屏' : '监控大屏')
const pageDescription = computed(() => (
  pageMode.value === 'inspection'
    ? '以巡检清单方式核查 PromQL 面板状态，并支持导出巡检报告 PDF。'
    : '以网格大屏方式展示指标卡、趋势、排行和仪表盘，适合投屏观测。'
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
    name: '空白大屏',
    description: '创建一个空白大屏，手动添加 PromQL 面板。',
    panels: []
  },
  {
    key: 'host',
    name: '主机资源大屏',
    description: '参考 Node Exporter 大屏，覆盖在线状态、CPU、内存、磁盘、网络和系统负载。',
    panels: [
      { title: '主机总数', chartType: 'stat', unit: '台', span: 6, promql: 'count(up{job=~"node.*|node-exporter"})' },
      { title: '在线主机', chartType: 'stat', unit: '台', span: 6, promql: 'sum(up{job=~"node.*|node-exporter"} == 1)' },
      { title: '离线主机', chartType: 'stat', unit: '台', span: 6, promql: 'sum(up{job=~"node.*|node-exporter"} == 0)' },
      { title: '平均 CPU 使用率', chartType: 'gauge', unit: '%', span: 6, promql: '100 - (avg(irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)' },
      { title: '平均内存使用率', chartType: 'gauge', unit: '%', span: 6, promql: '(1 - sum(node_memory_MemAvailable_bytes) / sum(node_memory_MemTotal_bytes)) * 100' },
      { title: '平均磁盘使用率', chartType: 'gauge', unit: '%', span: 6, promql: '100 - (sum(node_filesystem_avail_bytes{fstype!~"tmpfs|overlay",mountpoint!~"/run.*|/boot.*"}) / sum(node_filesystem_size_bytes{fstype!~"tmpfs|overlay",mountpoint!~"/run.*|/boot.*"}) * 100)' },
      { title: 'CPU 使用率 Top', chartType: 'bar', unit: '%', span: 12, promql: 'topk(10, 100 - (avg by (instance) (irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100))' },
      { title: '内存使用率 Top', chartType: 'bar', unit: '%', span: 12, promql: 'topk(10, (1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes) * 100)' },
      { title: '磁盘使用率 Top', chartType: 'bar', unit: '%', span: 12, promql: 'topk(10, 100 - (node_filesystem_avail_bytes{fstype!~"tmpfs|overlay",mountpoint!~"/run.*|/boot.*"} / node_filesystem_size_bytes{fstype!~"tmpfs|overlay",mountpoint!~"/run.*|/boot.*"} * 100))' },
      { title: '系统负载 Top', chartType: 'bar', unit: '', span: 12, promql: 'topk(10, node_load1)' },
      { title: '网络接收速率 Top', chartType: 'bar', unit: 'B/s', span: 12, promql: 'topk(10, sum by (instance) (rate(node_network_receive_bytes_total{device!~"lo|veth.*|docker.*|br.*"}[5m])))' },
      { title: '网络发送速率 Top', chartType: 'bar', unit: 'B/s', span: 12, promql: 'topk(10, sum by (instance) (rate(node_network_transmit_bytes_total{device!~"lo|veth.*|docker.*|br.*"}[5m])))' },
      { title: '磁盘读取速率 Top', chartType: 'bar', unit: 'B/s', span: 12, promql: 'topk(10, sum by (instance) (rate(node_disk_read_bytes_total[5m])))' },
      { title: '磁盘写入速率 Top', chartType: 'bar', unit: 'B/s', span: 12, promql: 'topk(10, sum by (instance) (rate(node_disk_written_bytes_total[5m])))' },
      { title: 'CPU 使用趋势', chartType: 'line', unit: '%', span: 12, promql: '100 - (avg by (instance) (irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)' },
      { title: '内存使用趋势', chartType: 'line', unit: '%', span: 12, promql: '(1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes) * 100' },
      { title: '文件句柄使用率', chartType: 'gauge', unit: '%', span: 6, promql: 'sum(node_filefd_allocated) / sum(node_filefd_maximum) * 100' },
      { title: '运行进程数', chartType: 'stat', unit: '个', span: 6, promql: 'sum(node_procs_running)' },
      { title: '系统运行时间 Top', chartType: 'bar', unit: '天', span: 12, promql: 'topk(10, (time() - node_boot_time_seconds) / 86400)' },
      { title: '主机信息', chartType: 'table', unit: '', span: 12, promql: 'node_uname_info' }
    ]
  },
  {
    key: 'k8s',
    name: 'Kubernetes 大屏',
    description: '参考 K8S Dashboard 大屏风格，覆盖集群资源、容量、工作负载和网络指标。',
    panels: [
      { title: 'Pod 数量', chartType: 'stat', unit: '个', span: 8, promql: 'count(kube_pod_info)' },
      { title: 'Running Pod', chartType: 'stat', unit: '个', span: 8, promql: 'sum(kube_pod_status_phase{phase="Running"})' },
      { title: '异常 Pod', chartType: 'stat', unit: '个', span: 8, promql: 'sum(kube_pod_status_phase{phase=~"Failed|Unknown|Pending"})' },
      { title: 'Ready 节点', chartType: 'stat', unit: '个', span: 8, promql: 'sum(kube_node_status_condition{condition="Ready",status="true"})' },
      { title: '命名空间', chartType: 'stat', unit: '个', span: 8, promql: 'count(kube_namespace_created)' },
      { title: 'Deployment', chartType: 'stat', unit: '个', span: 8, promql: 'count(kube_deployment_created)' },
      { title: 'Service', chartType: 'stat', unit: '个', span: 8, promql: 'count(kube_service_info)' },
      { title: 'Ingress', chartType: 'stat', unit: '个', span: 8, promql: 'count(kube_ingress_info)' },
      { title: 'PVC', chartType: 'stat', unit: '个', span: 8, promql: 'count(kube_persistentvolumeclaim_info)' },
      { title: 'CPU Request 使用率', chartType: 'gauge', unit: '%', span: 12, promql: 'sum(kube_pod_container_resource_requests{resource="cpu"}) / sum(kube_node_status_allocatable{resource="cpu"}) * 100' },
      { title: '内存 Request 使用率', chartType: 'gauge', unit: '%', span: 12, promql: 'sum(kube_pod_container_resource_requests{resource="memory"}) / sum(kube_node_status_allocatable{resource="memory"}) * 100' },
      { title: '命名空间 Pod 分布', chartType: 'bar', unit: '个', span: 12, promql: 'sum by (namespace) (kube_pod_info)' },
      { title: '节点 Pod 分布', chartType: 'bar', unit: '个', span: 12, promql: 'sum by (node) (kube_pod_info)' },
      { title: '工作负载副本可用率', chartType: 'bar', unit: '%', span: 12, promql: 'sum by (deployment) (kube_deployment_status_replicas_available) / sum by (deployment) (kube_deployment_spec_replicas) * 100' },
      { title: '异常原因 Top', chartType: 'bar', unit: '个', span: 12, promql: 'sum by (reason) (kube_pod_container_status_waiting_reason)' },
      { title: 'Pod 重启次数 Top', chartType: 'bar', unit: '次', span: 12, promql: 'topk(10, sum by (namespace, pod) (increase(kube_pod_container_status_restarts_total[1h])))' },
      { title: '容器 CPU 使用趋势', chartType: 'line', unit: 'Core', span: 12, promql: 'sum by (namespace) (rate(container_cpu_usage_seconds_total{container!="",pod!=""}[5m]))' },
      { title: '容器内存使用趋势', chartType: 'line', unit: 'B', span: 12, promql: 'sum by (namespace) (container_memory_working_set_bytes{container!="",pod!=""})' },
      { title: 'Pod 网络接收速率', chartType: 'line', unit: 'B/s', span: 12, promql: 'sum by (namespace) (rate(container_network_receive_bytes_total[5m]))' },
      { title: 'Pod 网络发送速率', chartType: 'line', unit: 'B/s', span: 12, promql: 'sum by (namespace) (rate(container_network_transmit_bytes_total[5m]))' },
      { title: 'Pod 明细', chartType: 'table', unit: '', span: 24, promql: 'kube_pod_info' }
    ]
  }
]

const activePanels = computed(() => panels.value.filter((item) => item.status === 1))
const visibleDashboards = computed(() => dashboards.value.filter((item) => (item.layout || 'grid') === pageLayout.value))
const isListLayout = computed(() => pageMode.value === 'inspection')
const isK8sDashboard = computed(() => {
  if (isListLayout.value) return false
  const name = `${activeDashboard.value?.name || ''} ${activeDashboard.value?.description || ''}`.toLowerCase()
  return name.includes('k8s') || name.includes('kubernetes') || panels.value.some((panel) => String(panel.promql || '').includes('kube_'))
})
const dashboardHealth = computed(() => {
  const errors = activePanels.value.filter((panel) => panelResults[panel.id]?.error).length
  if (!activeDashboard.value) return { text: '未选择', type: 'info' }
  if (errors > 0) return { text: `${errors} 个异常`, type: 'danger' }
  return { text: '正常', type: 'success' }
})
const defaultDatasourceId = computed(() => selectedDatasourceId.value || datasourceOptions.value[0]?.id)
const currentTemplate = computed(() => dashboardTemplates.find((item) => item.key === activeTemplate.value) || dashboardTemplates[0])

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
  const value = Number(row?.value?.[1])
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
  if (unit === '天') return `${value.toFixed(value >= 10 ? 0 : 1)} 天`
  if (unit === '%') return `${value.toFixed(value >= 10 ? 1 : 2).replace(/\.?0+$/, '')}%`
  if (unit && unit !== '个' && unit !== '台') return `${value.toFixed(value >= 100 ? 0 : 2).replace(/\.?0+$/, '')}${unit}`
  return value.toLocaleString(undefined, { maximumFractionDigits: value >= 100 ? 0 : 2 })
}

function panelRows(panel) {
  return panelResults[panel.id]?.result || []
}

function panelValue(panel) {
  const value = Number(panelRows(panel)[0]?.value?.[1])
  if (!Number.isFinite(value)) return '-'
  const formatted = formatByUnit(value, panel.unit)
  return panel.unit && formatted.endsWith(panel.unit) ? formatted.slice(0, -panel.unit.length).trim() : formatted
}

function panelTrend(panel) {
  return panelRows(panel).slice(0, 12).map((item) => numberValue(item))
}

function sparklinePoints(panel) {
  const values = panelTrend(panel)
  if (!values.length) return ''
  const max = Math.max(...values)
  const min = Math.min(...values)
  const range = max - min || 1
  return values.map((value, index) => {
    const x = values.length === 1 ? 100 : (index / (values.length - 1)) * 100
    const y = 48 - ((value - min) / range) * 40
    return `${x},${y}`
  }).join(' ')
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
  const value = Number(panelRows(panel)[0]?.value?.[1])
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
  if (panel.status !== 1) return '已禁用'
  if (panelResults[panel.id]?.error) return '查询失败'
  if (!panelRows(panel).length) return '暂无数据'
  return '实时'
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
  const value = Number(panelRows(panel)[0]?.value?.[1])
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
    ElMessage.warning('请先选择巡检大屏')
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
    ElMessage.warning('浏览器阻止了报告窗口，请允许弹窗后重试')
    return
  }
  win.document.write(`
    <!doctype html>
    <html>
      <head>
        <meta charset="utf-8" />
        <title>${escapeHtml(activeDashboard.value.name)} - 巡检报告</title>
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
        <button class="no-print" onclick="window.print()" style="float:right;padding:8px 14px;">打印 / 保存 PDF</button>
        <h1>${escapeHtml(activeDashboard.value.name)} 巡检报告</h1>
        <div class="meta">
          <span>生成时间：${escapeHtml(now)}</span>
          <span>查询数据源：${escapeHtml(currentDatasourceName)}</span>
          <span>大屏状态：${escapeHtml(dashboardHealth.value.text)}</span>
          <span>刷新周期：${autoRefreshSeconds.value ? `${autoRefreshSeconds.value}s` : '关闭'}</span>
        </div>
        <div class="summary">
          <div class="card"><span>面板数量</span><strong>${panels.value.length}</strong></div>
          <div class="card"><span>启用面板</span><strong>${activePanels.value.length}</strong></div>
          <div class="card"><span>异常面板</span><strong>${activePanels.value.filter((panel) => panelResults[panel.id]?.error).length}</strong></div>
          <div class="card"><span>巡检类型</span><strong>列表巡检</strong></div>
        </div>
        <table>
          <thead>
            <tr>
              <th>#</th><th>面板</th><th>状态</th><th>当前值</th><th>序列</th><th>数据源</th><th>类型</th><th>PromQL / 错误</th>
            </tr>
          </thead>
          <tbody>${rows || '<tr><td colspan="8">暂无巡检面板</td></tr>'}</tbody>
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
  datasourceOptions.value = datasources || []
  if (!selectedDatasourceId.value && datasourceOptions.value.length) {
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
    await refreshAllPanels()
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
    ElMessage.warning(`请输入${pageTitle.value}名称`)
    return
  }
  dashboardForm.layout = pageLayout.value
  if (!editingDashboard.value && currentTemplate.value.panels.length && !defaultDatasourceId.value) {
    ElMessage.warning('请先创建 Prometheus 数据源，再使用模板创建大屏')
    return
  }
  const savedDashboard = await saveMonitorDashboard(dashboardForm)
  ElMessage.success('保存成功')
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
  await ElMessageBox.confirm(`确认删除${pageTitle.value}「${activeDashboard.value.name}」吗？面板也会一起删除。`, '提示', { type: 'warning' })
  await deleteMonitorDashboard(activeDashboard.value.id)
  ElMessage.success('删除成功')
  activeDashboardId.value = undefined
  activeDashboard.value = null
  panels.value = []
  await loadBase()
  await loadDashboard(activeDashboardId.value)
}

function openCreatePanel() {
  if (!activeDashboardId.value) {
    ElMessage.warning('请先创建或选择监控大屏')
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
    ElMessage.warning('请填写面板标题、数据源和 PromQL')
    return
  }
  await saveMonitorDashboardPanel(panelForm)
  ElMessage.success('保存成功')
  panelDialogVisible.value = false
  await loadDashboard(activeDashboardId.value)
}

async function handleDeletePanel(row) {
  await ElMessageBox.confirm(`确认删除面板「${row.title}」吗？`, '提示', { type: 'warning' })
  await deleteMonitorDashboardPanel(row.id)
  ElMessage.success('删除成功')
  await loadDashboard(activeDashboardId.value)
}

async function refreshPanel(row) {
  panelLoading.value = true
  try {
    panelResults[row.id] = await queryMonitorDashboardPanel({ id: row.id, datasourceId: selectedDatasourceId.value })
  } catch (error) {
    panelResults[row.id] = { error: error.message || 'query failed' }
  } finally {
    panelLoading.value = false
  }
}

async function refreshAllPanels() {
  const enabledPanels = panels.value.filter((item) => item.status === 1)
  await Promise.all(enabledPanels.map((item) => queryMonitorDashboardPanel({ id: item.id, datasourceId: selectedDatasourceId.value })
    .then((data) => { panelResults[item.id] = data })
    .catch((error) => { panelResults[item.id] = { error: error.message || 'query failed' } })))
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
          <strong>监控大屏</strong>
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
            <span>{{ item.panelCount || 0 }} 个面板</span>
          </button>
          <div v-if="!visibleDashboards.length" class="empty-switcher">暂无{{ pageTitle }}</div>
          </div>
        </el-scrollbar>
      </div>

      <el-button class="create-screen-btn" type="primary" @click="openCreateDashboard">创建{{ pageTitle }}</el-button>
    </section>

    <main class="dashboard-main" :class="{ 'k8s-dashboard-main': isK8sDashboard }" v-loading="loading">
      <section class="dashboard-hero">
        <div>
          <div class="eyebrow">Monitoring Dashboard</div>
          <h2>{{ activeDashboard?.name || pageTitle }}</h2>
          <p>{{ activeDashboard?.description || pageDescription }}</p>
          <div v-if="activeDashboard" class="layout-hint">
            {{ isListLayout ? '当前布局：列表巡检，适合日常排障和逐项核查。' : '当前布局：网格大屏，适合投屏展示和整体观测。' }}
          </div>
        </div>
        <div class="hero-actions">
          <el-select v-model="selectedDatasourceId" placeholder="选择数据源" style="width: 180px" @change="handleDatasourceChange">
            <el-option v-for="item in datasourceOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
          <el-select v-model="autoRefreshSeconds" style="width: 130px" @change="restartAutoRefresh">
            <el-option label="关闭刷新" :value="0" />
            <el-option label="10 秒刷新" :value="10" />
            <el-option label="30 秒刷新" :value="30" />
            <el-option label="60 秒刷新" :value="60" />
          </el-select>
          <el-button @click="refreshAllPanels" :disabled="!activePanels.length">刷新全部</el-button>
          <el-button v-if="!isListLayout" @click="toggleFullscreen" :disabled="!activeDashboard">{{ isFullscreen ? '退出全屏' : '大屏展示' }}</el-button>
          <el-button v-else @click="exportInspectionReportPdf" :disabled="!activeDashboard">导出巡检报告 PDF</el-button>
          <el-button @click="openEditDashboard" :disabled="!activeDashboard">编辑{{ pageTitle }}</el-button>
          <el-button type="danger" plain @click="handleDeleteDashboard" :disabled="!activeDashboard">删除{{ pageTitle }}</el-button>
          <el-button type="primary" @click="openCreatePanel">新增面板</el-button>
        </div>
      </section>

      <section class="dashboard-summary">
        <div class="summary-card">
          <span>面板数量</span>
          <strong>{{ panels.length }}</strong>
        </div>
        <div class="summary-card">
          <span>启用面板</span>
          <strong>{{ activePanels.length }}</strong>
        </div>
        <div class="summary-card">
          <span>刷新周期</span>
          <strong>{{ autoRefreshSeconds ? `${autoRefreshSeconds}s` : '关闭' }}</strong>
        </div>
        <div class="summary-card">
          <span>大屏状态</span>
          <strong :class="`state-${dashboardHealth.type}`">{{ dashboardHealth.text }}</strong>
        </div>
      </section>

      <el-empty v-if="!activeDashboard" description="还没有监控大屏，请先创建一个" />
      <el-empty v-else-if="!panels.length" description="当前大屏还没有面板，可以新增面板或使用模板创建" />

      <section v-else-if="isListLayout" class="inspection-list" v-loading="panelLoading">
        <div class="inspection-head">
          <div>
            <h3>巡检面板</h3>
            <p>按面板逐项查看查询状态、当前值、返回序列和 PromQL，适合日常排障巡检。</p>
          </div>
          <el-button @click="refreshAllPanels" :disabled="!activePanels.length">执行巡检</el-button>
        </div>
        <el-table :data="panels" class="inspection-table" row-key="id">
          <el-table-column label="面板" min-width="180">
            <template #default="{ row }">
              <div class="inspection-name">
                <strong>{{ row.title }}</strong>
                <span>{{ row.description || '未填写描述' }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="110">
            <template #default="{ row }">
              <el-tag :type="panelStateType(row)" effect="light">{{ panelState(row) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="当前值" width="150">
            <template #default="{ row }">
              <strong class="inspection-value">{{ panelDisplayValue(row) }}</strong>
            </template>
          </el-table-column>
          <el-table-column label="返回序列" width="110">
            <template #default="{ row }">{{ panelResultCount(row) }}</template>
          </el-table-column>
          <el-table-column label="数据源 / 类型" width="180">
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
          <el-table-column label="操作" width="190" fixed="right">
            <template #default="{ row }">
              <el-button link type="primary" @click="refreshPanel(row)">刷新</el-button>
              <el-button link type="primary" @click="openEditPanel(row)">编辑</el-button>
              <el-button link type="danger" @click="handleDeletePanel(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </section>

      <section v-else class="panel-grid" :class="{ 'k8s-panel-grid': isK8sDashboard }" v-loading="panelLoading">
        <div
          v-for="panel in panels"
          :key="panel.id"
          class="metric-panel"
          :class="[`panel-${panel.chartType}`, { disabled: panel.status !== 1, 'k8s-metric-panel': isK8sDashboard }]"
          :style="{ gridColumn: `span ${panelSpan(panel)}` }"
        >
          <div class="panel-glow"></div>
          <div class="panel-head">
            <div>
              <strong>{{ panel.title }}</strong>
              <span>{{ datasourceOptions.find((item) => item.id === selectedDatasourceId)?.name || panel.datasourceName }} / {{ panel.chartType }} / {{ panelState(panel) }}</span>
            </div>
            <div class="panel-actions">
              <el-button link type="primary" @click="refreshPanel(panel)">刷新</el-button>
              <el-button link type="primary" @click="openEditPanel(panel)">编辑</el-button>
              <el-button link type="danger" @click="handleDeletePanel(panel)">删除</el-button>
            </div>
          </div>

          <div v-if="panelResults[panel.id]?.error" class="panel-error">{{ panelResults[panel.id].error }}</div>

          <template v-else-if="panel.chartType === 'table'">
            <el-table :data="panelRows(panel)" size="small" height="250">
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
          </template>

          <template v-else>
            <div class="stat-row">
              <div>
                <div class="stat-value">{{ panelValue(panel) }}<small>{{ panel.unit }}</small></div>
                <div class="stat-caption">
                  <span></span>
                  实时采样
                </div>
              </div>
              <svg v-if="panel.chartType === 'line'" class="sparkline" viewBox="0 0 100 52" preserveAspectRatio="none">
                <polyline :points="sparklinePoints(panel)" />
              </svg>
            </div>
            <div class="promql">{{ panel.promql }}</div>
          </template>
        </div>
      </section>
    </main>

    <el-dialog v-model="dashboardDialogVisible" :title="editingDashboard ? `编辑${pageTitle}` : `创建${pageTitle}`" width="760px">
      <el-form label-width="100px">
        <el-form-item label="名称" required><el-input v-model="dashboardForm.name" :placeholder="`例如：生产环境${pageTitle}`" /></el-form-item>
        <el-form-item v-if="!editingDashboard" label="大屏模板">
          <div class="template-grid">
            <button v-for="item in dashboardTemplates" :key="item.key" type="button" class="template-card" :class="{ active: activeTemplate === item.key }" @click="activeTemplate = item.key">
              <strong>{{ item.name }}</strong>
              <span>{{ item.description }}</span>
            </button>
          </div>
        </el-form-item>
        <el-form-item label="类型">
          <el-tag type="primary" effect="light">{{ isListLayout ? '巡检大屏 / 列表巡检' : '监控大屏 / 网格展示' }}</el-tag>
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="dashboardForm.status">
            <el-radio :value="1">启用</el-radio>
            <el-radio :value="2">禁用</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="描述"><el-input v-model="dashboardForm.description" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dashboardDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submitDashboard">{{ editingDashboard ? '保存' : '创建' }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="panelDialogVisible" :title="editingPanel ? '编辑面板' : '新增面板'" width="820px">
      <el-form label-width="110px">
        <el-form-item label="标题" required><el-input v-model="panelForm.title" /></el-form-item>
        <el-form-item label="数据源" required>
          <el-select v-model="panelForm.datasourceId" filterable style="width: 100%">
            <el-option v-for="item in datasourceOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="PromQL" required><el-input v-model="panelForm.promql" type="textarea" :rows="4" placeholder="例如：up 或 sum(rate(http_requests_total[5m]))" /></el-form-item>
        <el-row :gutter="12">
          <el-col :span="8">
            <el-form-item label="图表类型">
              <el-select v-model="panelForm.chartType">
                <el-option label="指标卡" value="stat" />
                <el-option label="折线趋势" value="line" />
                <el-option label="柱状排行" value="bar" />
                <el-option label="仪表盘" value="gauge" />
                <el-option label="表格" value="table" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8"><el-form-item label="单位"><el-input v-model="panelForm.unit" placeholder="%, ms, 个" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="宽度"><el-input-number v-model="panelForm.span" :min="6" :max="24" :step="6" style="width: 100%" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="8"><el-form-item label="排序"><el-input-number v-model="panelForm.sort" style="width: 100%" /></el-form-item></el-col>
          <el-col :span="16">
            <el-form-item label="状态">
              <el-radio-group v-model="panelForm.status">
                <el-radio :value="1">启用</el-radio>
                <el-radio :value="2">禁用</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="描述"><el-input v-model="panelForm.description" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="panelDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submitPanel">保存</el-button>
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
}
</style>
