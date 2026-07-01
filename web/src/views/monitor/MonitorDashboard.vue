<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
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
const panelLoading = ref(false)
const dashboards = ref([])
const datasourceOptions = ref([])
const activeDashboardId = ref()
const activeDashboard = ref(null)
const panels = ref([])
const panelResults = reactive({})
const dashboardDialogVisible = ref(false)
const panelDialogVisible = ref(false)
const editingDashboard = ref(false)
const editingPanel = ref(false)

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
  span: 8,
  sort: 0,
  status: 1,
  description: ''
})

const activePanels = computed(() => panels.value.filter((item) => item.status === 1))

function resetDashboardForm() {
  Object.assign(dashboardForm, {
    id: undefined,
    name: '',
    layout: 'grid',
    status: 1,
    description: ''
  })
}

function resetPanelForm() {
  Object.assign(panelForm, {
    id: undefined,
    dashboardId: activeDashboardId.value,
    title: '',
    datasourceId: datasourceOptions.value[0]?.id,
    promql: '',
    unit: '',
    chartType: 'stat',
    span: 8,
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

function panelValue(result) {
  const rows = result?.result || []
  const value = rows[0]?.value?.[1]
  return value === undefined ? '-' : value
}

async function loadBase() {
  const [dashboardData, datasources] = await Promise.all([
    queryMonitorDashboardList({ pageNum: 1, pageSize: 100 }),
    queryMonitorDatasourceOptions()
  ])
  dashboards.value = dashboardData.list || []
  datasourceOptions.value = datasources || []
  if (!activeDashboardId.value && dashboards.value.length) {
    activeDashboardId.value = dashboards.value[0].id
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
  Object.assign(dashboardForm, activeDashboard.value)
  dashboardDialogVisible.value = true
}

async function submitDashboard() {
  if (!dashboardForm.name.trim()) {
    ElMessage.warning('请输入仪表盘名称')
    return
  }
  await saveMonitorDashboard(dashboardForm)
  ElMessage.success('保存成功')
  dashboardDialogVisible.value = false
  await loadBase()
  if (!activeDashboardId.value && dashboards.value.length) activeDashboardId.value = dashboards.value[0].id
  await loadDashboard(activeDashboardId.value)
}

async function handleDeleteDashboard() {
  if (!activeDashboard.value) return
  await ElMessageBox.confirm(`确认删除仪表盘「${activeDashboard.value.name}」吗？面板也会一起删除。`, '提示', { type: 'warning' })
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
    ElMessage.warning('请先创建或选择仪表盘')
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
    panelResults[row.id] = await queryMonitorDashboardPanel(row.id)
  } finally {
    panelLoading.value = false
  }
}

async function refreshAllPanels() {
  const enabledPanels = panels.value.filter((item) => item.status === 1)
  await Promise.all(enabledPanels.map((item) => queryMonitorDashboardPanel(item.id)
    .then((data) => { panelResults[item.id] = data })
    .catch((error) => { panelResults[item.id] = { error: error.message || 'query failed' } })))
}

onMounted(async () => {
  await loadBase()
  await loadDashboard(activeDashboardId.value)
})
</script>

<template>
  <div class="dashboard-page">
    <aside class="dashboard-sidebar">
      <div class="sidebar-title">
        <span>仪表盘</span>
        <el-button link type="primary" @click="openCreateDashboard">新增</el-button>
      </div>
      <el-scrollbar height="620px">
        <button
          v-for="item in dashboards"
          :key="item.id"
          class="dashboard-item"
          :class="{ active: item.id === activeDashboardId }"
          @click="loadDashboard(item.id)"
        >
          <strong>{{ item.name }}</strong>
          <span>{{ item.panelCount || 0 }} 个面板</span>
        </button>
      </el-scrollbar>
    </aside>

    <main class="dashboard-main" v-loading="loading">
      <section class="dashboard-header">
        <div>
          <h2>{{ activeDashboard?.name || '监控仪表盘' }}</h2>
          <p>{{ activeDashboard?.description || '创建仪表盘后，可以添加 PromQL 面板并快速刷新指标。' }}</p>
        </div>
        <div class="header-actions">
          <el-button @click="refreshAllPanels" :disabled="!activePanels.length">刷新全部</el-button>
          <el-button @click="openEditDashboard" :disabled="!activeDashboard">编辑仪表盘</el-button>
          <el-button type="danger" plain @click="handleDeleteDashboard" :disabled="!activeDashboard">删除仪表盘</el-button>
          <el-button type="primary" @click="openCreatePanel">新增面板</el-button>
        </div>
      </section>

      <el-empty v-if="!activeDashboard" description="还没有仪表盘，请先创建一个" />
      <el-empty v-else-if="!panels.length" description="当前仪表盘还没有面板" />

      <section v-else class="panel-grid" v-loading="panelLoading">
        <div v-for="panel in panels" :key="panel.id" class="metric-panel" :class="{ disabled: panel.status !== 1 }" :style="{ gridColumn: `span ${Math.max(1, Math.round((panel.span || 8) / 6))}` }">
          <div class="panel-head">
            <div>
              <strong>{{ panel.title }}</strong>
              <span>{{ panel.datasourceName }} · {{ panel.chartType }}</span>
            </div>
            <div>
              <el-button link type="primary" @click="refreshPanel(panel)">刷新</el-button>
              <el-button link type="primary" @click="openEditPanel(panel)">编辑</el-button>
              <el-button link type="danger" @click="handleDeletePanel(panel)">删除</el-button>
            </div>
          </div>

          <template v-if="panelResults[panel.id]?.error">
            <div class="panel-error">{{ panelResults[panel.id].error }}</div>
          </template>
          <template v-else-if="panel.chartType === 'table'">
            <el-table :data="panelResults[panel.id]?.result || []" size="small" border height="220">
              <el-table-column label="Metric" min-width="220" show-overflow-tooltip>
                <template #default="{ row }">{{ metricText(row.metric) }}</template>
              </el-table-column>
              <el-table-column label="Value" width="120">
                <template #default="{ row }">{{ row.value?.[1] ?? '-' }}</template>
              </el-table-column>
            </el-table>
          </template>
          <template v-else>
            <div class="stat-value">{{ panelValue(panelResults[panel.id]) }}<small>{{ panel.unit }}</small></div>
            <div class="promql">{{ panel.promql }}</div>
          </template>
        </div>
      </section>
    </main>

    <el-dialog v-model="dashboardDialogVisible" :title="editingDashboard ? '编辑仪表盘' : '新增仪表盘'" width="620px">
      <el-form label-width="100px">
        <el-form-item label="名称" required><el-input v-model="dashboardForm.name" /></el-form-item>
        <el-form-item label="布局">
          <el-radio-group v-model="dashboardForm.layout">
            <el-radio-button label="grid">网格</el-radio-button>
            <el-radio-button label="list">列表</el-radio-button>
          </el-radio-group>
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
        <el-button type="primary" @click="submitDashboard">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="panelDialogVisible" :title="editingPanel ? '编辑面板' : '新增面板'" width="760px">
      <el-form label-width="110px">
        <el-form-item label="标题" required><el-input v-model="panelForm.title" /></el-form-item>
        <el-form-item label="数据源" required>
          <el-select v-model="panelForm.datasourceId" filterable style="width: 100%">
            <el-option v-for="item in datasourceOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="PromQL" required><el-input v-model="panelForm.promql" type="textarea" :rows="4" /></el-form-item>
        <el-row :gutter="12">
          <el-col :span="8">
            <el-form-item label="图表类型">
              <el-select v-model="panelForm.chartType">
                <el-option label="指标卡" value="stat" />
                <el-option label="表格" value="table" />
                <el-option label="折线" value="line" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8"><el-form-item label="单位"><el-input v-model="panelForm.unit" placeholder="%, ms, 次" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="宽度"><el-input-number v-model="panelForm.span" :min="6" :max="24" style="width: 100%" /></el-form-item></el-col>
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
.dashboard-page { display: grid; grid-template-columns: 260px 1fr; gap: 18px; }
.dashboard-sidebar, .dashboard-main { background: #fff; border-radius: 18px; box-shadow: 0 12px 30px rgba(36, 54, 90, 0.08); }
.dashboard-sidebar { padding: 18px; }
.sidebar-title { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; font-weight: 700; color: #10213f; }
.dashboard-item { width: 100%; border: 1px solid #e3ebf7; background: #f8fbff; border-radius: 10px; padding: 12px; margin-bottom: 10px; text-align: left; cursor: pointer; color: #263b5f; }
.dashboard-item strong, .dashboard-item span { display: block; }
.dashboard-item span { margin-top: 6px; color: #8190aa; font-size: 12px; }
.dashboard-item.active { background: #2f63bf; color: #fff; border-color: #2f63bf; }
.dashboard-item.active span { color: rgba(255,255,255,.78); }
.dashboard-main { padding: 22px; min-height: 680px; }
.dashboard-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; margin-bottom: 18px; }
.dashboard-header h2 { margin: 0 0 8px; font-size: 26px; color: #10213f; }
.dashboard-header p { margin: 0; color: #7282a0; }
.header-actions { display: flex; gap: 10px; flex-wrap: wrap; justify-content: flex-end; }
.panel-grid { display: grid; grid-template-columns: repeat(4, minmax(220px, 1fr)); gap: 14px; }
.metric-panel { min-height: 210px; border: 1px solid #e5edf8; border-radius: 12px; padding: 16px; background: #fbfdff; overflow: hidden; }
.metric-panel.disabled { opacity: .55; }
.panel-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 10px; margin-bottom: 18px; }
.panel-head strong { display: block; color: #10213f; font-size: 16px; }
.panel-head span { display: block; margin-top: 5px; color: #8391aa; font-size: 12px; }
.stat-value { font-size: 34px; font-weight: 800; color: #1e4f9a; line-height: 1.2; }
.stat-value small { margin-left: 6px; font-size: 14px; color: #7888a4; }
.promql { margin-top: 18px; padding: 10px; border-radius: 8px; background: #0f172a; color: #b9d6ff; font-family: Consolas, Monaco, monospace; font-size: 12px; white-space: pre-wrap; word-break: break-all; }
.panel-error { padding: 12px; border-radius: 8px; color: #b91c1c; background: #fff1f2; word-break: break-all; }
@media (max-width: 1200px) {
  .dashboard-page { grid-template-columns: 1fr; }
  .panel-grid { grid-template-columns: repeat(2, minmax(220px, 1fr)); }
}
</style>
