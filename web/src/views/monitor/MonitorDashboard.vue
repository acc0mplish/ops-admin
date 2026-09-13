<script setup>
import { uiT } from '../../utils/english-hardcoding-i18n'
import { mt } from '../../utils/monitor-i18n'
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  deleteMonitorDashboard,
  deleteMonitorDashboardPanel,
  monitorDashboardInfo,
  queryMonitorDashboardList,
  queryMonitorDatasourceOptions,
  saveMonitorDashboard,
  saveMonitorDashboardPanel
} from '../../api/monitor'
import { useMonitorDashboardPanels } from '../../composables/useMonitorDashboardPanels'
import MonitorDashboardWorkspace from './dashboard/MonitorDashboardWorkspace.vue'
import MonitorDashboardHero from './dashboard/MonitorDashboardHero.vue'
import MonitorDashboardPanelList from './dashboard/MonitorDashboardPanelList.vue'
import MonitorDashboardGrid from './dashboard/MonitorDashboardGrid.vue'
import MonitorDashboardPanelChart from './dashboard/MonitorDashboardPanelChart.vue'

// I5-I (i5-plan §3.8·CW-DB) — MonitorDashboard.vue 2,600행 분할 잔류본.
// 패널 쿼리 엔진·렌더 어휘는 composables/useMonitorDashboardPanels.js 1종,
// 템플릿/CSS 덩어리는 dashboard/ 자식 5종으로 이동했고 각 커밋 메시지에
// 원본 좌표가 있다. 자식은 `:page` 주입 패턴(§5 #11 승계)으로 상태를 받는다.
const loading = ref(false)
const route = useRoute()
const dashboards = ref([])
const datasourceOptions = ref([])
const selectedDatasourceId = ref()
const activeDashboardId = ref()
const activeDashboard = ref(null)
const panels = ref([])
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
const inspectionFilter = ref('all')

const pageMode = computed(() => route.path.includes('/monitor/inspections') ? 'inspection' : 'dashboard')
const pageLayout = computed(() => pageMode.value === 'inspection' ? 'list' : 'grid')
const pageTitle = computed(() => pageMode.value === 'inspection' ? 'Inspection Dashboard' : uiT('monitoringDashboard'))
const pageDescription = computed(() => (
  pageMode.value === 'inspection'
    ? mt('inspectionPageDesc')
    : mt('gridPageDesc')
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

const activePanels = computed(() => panels.value.filter((item) => item.status === 1))
// 원본 :154-159 — 컴포저블 주입 전제로 선언 위치만 선행 이동(게터는 지연 평가라 행동 동일)
const isListLayout = computed(() => pageMode.value === 'inspection')
const isK8sDashboard = computed(() => {
  if (isListLayout.value) return false
  const name = `${activeDashboard.value?.name || ''} ${activeDashboard.value?.description || ''}`.toLowerCase()
  return name.includes('k8s') || name.includes('kubernetes') || panels.value.some((panel) => String(panel.promql || '').includes('kube_'))
})
const {
  panelResults,
  panelPending,
  dashboardTemplates,
  metricText,
  unitText,
  panelRows,
  panelValue,
  sparklinePoints,
  panelLineSeries,
  sparklineAreaPoints,
  panelStats,
  panelChartLabel,
  barRows,
  gaugePercent,
  panelSpan,
  panelState,
  panelStateKey,
  panelStateType,
  panelResultCount,
  panelDisplayValue,
  exportInspectionReportPdf,
  refreshPanel,
  refreshProblemPanels,
  refreshAllPanels
} = useMonitorDashboardPanels({ panels, activePanels, selectedDatasourceId, timeRangeSeconds, lastRefreshAt, isK8sDashboard })
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
const dashboardHealth = computed(() => {
  const errors = activePanels.value.filter((panel) => panelResults[panel.id]?.error).length
  if (!activeDashboard.value) return { text: mt('healthNotSelected'), type: 'info' }
  if (errors > 0) return { text: mt('healthErrorsAbove', { count: errors }), type: 'danger' }
  return { text: mt('healthOk'), type: 'success' }
})
const defaultDatasourceId = computed(() => selectedDatasourceId.value || datasourceOptions.value[0]?.id)
const currentTemplate = computed(() => dashboardTemplates.find((item) => item.key === activeTemplate.value) || dashboardTemplates[0])
const currentDatasourceName = computed(() => datasourceOptions.value.find((item) => item.id === selectedDatasourceId.value)?.name || mt('datasourceNotSelected'))
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
    ElMessage.warning(mt('enterTitleName', { title: pageTitle.value }))
    return
  }
  dashboardForm.layout = pageLayout.value
  if (!editingDashboard.value && currentTemplate.value.panels.length && !defaultDatasourceId.value) {
    ElMessage.warning(mt('createDatasourceFirst'))
    return
  }
  const savedDashboard = await saveMonitorDashboard(dashboardForm)
  ElMessage.success(mt('savedMsg'))
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
  await ElMessageBox.confirm(mt('deleteDashboardConfirm', { title: pageTitle.value, name: activeDashboard.value.name }), mt('confirmTitle'), { type: 'warning' })
  await deleteMonitorDashboard(activeDashboard.value.id)
  ElMessage.success(mt('deletedMsg'))
  activeDashboardId.value = undefined
  activeDashboard.value = null
  panels.value = []
  await loadBase()
  await loadDashboard(activeDashboardId.value)
}

function openCreatePanel() {
  if (!activeDashboardId.value) {
    ElMessage.warning(mt('selectOrCreateDashboard'))
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
    ElMessage.warning(mt('enterPanelRequired'))
    return
  }
  await saveMonitorDashboardPanel(panelForm)
  ElMessage.success(mt('savedMsg'))
  panelDialogVisible.value = false
  await loadDashboard(activeDashboardId.value)
}

async function handleDeletePanel(row) {
  await ElMessageBox.confirm(mt('deletePanelConfirm', { name: row.title }), mt('confirmTitle'), { type: 'warning' })
  await deleteMonitorDashboardPanel(row.id)
  ElMessage.success(mt('deletedMsg'))
  await loadDashboard(activeDashboardId.value)
}

async function copyPromql(promql) {
  try {
    await navigator.clipboard.writeText(promql || '')
    ElMessage.success(mt('promqlCopied'))
  } catch {
    ElMessage.warning(mt('promqlCopyFailed'))
  }
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

// I5-I 자식 주입 번들 — reactive 래핑으로 ref/computed가 언랩되어
// 자식 템플릿의 page.x / v-model="page.x" 재배선이 동작한다(§5 #11).
const page = reactive({
  // 상태·computed
  loading,
  datasourceOptions,
  selectedDatasourceId,
  activeDashboardId,
  activeDashboard,
  panels,
  autoRefreshSeconds,
  timeRangeSeconds,
  isFullscreen,
  inspectionFilter,
  pageTitle,
  pageDescription,
  activePanels,
  inspectionSummary,
  inspectionPanels,
  dashboardHealth,
  visibleDashboards,
  isListLayout,
  isK8sDashboard,
  currentDatasourceName,
  lastRefreshText,
  panelResults,
  panelPending,
  metricText,
  unitText,
  panelRows,
  panelValue,
  sparklinePoints,
  panelLineSeries,
  sparklineAreaPoints,
  panelStats,
  panelChartLabel,
  barRows,
  gaugePercent,
  panelSpan,
  panelState,
  panelStateType,
  panelResultCount,
  panelDisplayValue,
  // 함수
  loadDashboard,
  openCreateDashboard,
  openEditDashboard,
  handleDeleteDashboard,
  openCreatePanel,
  openEditPanel,
  handleDeletePanel,
  handleDatasourceChange,
  restartAutoRefresh,
  toggleFullscreen,
  exportInspectionReportPdf,
  refreshPanel,
  refreshProblemPanels,
  refreshAllPanels
})


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
    <MonitorDashboardWorkspace :page="page" />

    <main class="dashboard-main observability-canvas" v-loading="loading">
      <MonitorDashboardHero :page="page" />

      <el-empty v-if="!activeDashboard" :description="mt('noDashboardsYet')" />
      <el-empty v-else-if="!panels.length" :description="mt('noPanelsYet')" />

      <MonitorDashboardPanelList v-else-if="isListLayout" :page="page" />

      <MonitorDashboardGrid v-else :page="page" />
    </main>

    <el-dialog v-model="dashboardDialogVisible" :title="editingDashboard ? mt('editTitle', { title: pageTitle }) : mt('createTitle', { title: pageTitle })" width="760px">
      <el-form label-width="100px">
        <el-form-item :label="mt('nameLabel')" required><el-input v-model="dashboardForm.name" :placeholder="mt('namePlaceholder', { title: pageTitle })" /></el-form-item>
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
        <el-form-item :label="mt('status')">
          <el-radio-group v-model="dashboardForm.status">
            <el-radio :value="1">{{ mt('enabledOption') }}</el-radio>
            <el-radio :value="2">{{ mt('disabledOption') }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="mt('descriptionLabel')"><el-input v-model="dashboardForm.description" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dashboardDialogVisible = false">{{ mt('cancel') }}</el-button>
        <el-button type="primary" @click="submitDashboard">{{ editingDashboard ? mt('save') : mt('create') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="panelDialogVisible" :title="editingPanel ? mt('editPanel') : mt('addPanel')" width="820px">
      <el-form label-width="110px">
        <el-form-item :label="mt('titleLabel')" required><el-input v-model="panelForm.title" /></el-form-item>
        <el-form-item label="Datasource" required>
          <el-select v-model="panelForm.datasourceId" filterable style="width: 100%">
            <el-option v-for="item in datasourceOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="PromQL" required><el-input v-model="panelForm.promql" type="textarea" :rows="4" :placeholder="mt('promqlPlaceholder')" /></el-form-item>
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
          <el-col :span="8"><el-form-item :label="uiT('unit')"><el-input v-model="panelForm.unit" :placeholder="mt('unitPlaceholder')" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="Width"><el-input-number v-model="panelForm.span" :min="6" :max="24" :step="6" style="width: 100%" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="8"><el-form-item label="Sort"><el-input-number v-model="panelForm.sort" style="width: 100%" /></el-form-item></el-col>
          <el-col :span="16">
            <el-form-item :label="mt('status')">
              <el-radio-group v-model="panelForm.status">
                <el-radio :value="1">{{ mt('enabledOption') }}</el-radio>
                <el-radio :value="2">{{ mt('disabledOption') }}</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item :label="mt('descriptionLabel')"><el-input v-model="panelForm.description" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="panelDialogVisible = false">{{ mt('cancel') }}</el-button>
        <el-button type="primary" @click="submitPanel">{{ mt('save') }}</el-button>
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
/* Grafana/Nightingale inspired observability workspace. */
.dashboard-page {
  gap: 14px;
}
.observability-canvas {
  padding: 14px;
  border: 1px solid #d9e1ec;
  border-radius: 8px;
  background: #f2f5f9;
  box-shadow: none;
}
/* Fullscreen is a dedicated Grafana-style presentation mode. */
.observability-canvas:fullscreen {
  padding: 12px;
  border: 0;
  border-radius: 0;
  background: #0b1017;
  color: #d7e0ea;
}
/* Dense fullscreen layout: prioritize the monitoring surface over page chrome. */
.observability-canvas:fullscreen {
  padding: 8px;
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
</style>
