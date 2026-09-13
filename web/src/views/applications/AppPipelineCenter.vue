<script setup>
import { uiT } from '../../utils/english-hardcoding-i18n'
import { apt } from '../../utils/application-i18n'
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  copyOpsAppPipeline,
  deleteOpsAppPipeline,
  opsAppPipelineInfo,
  queryOpsAppPipelineList,
  queryOpsAppPipelineRunList,
  queryOpsAppPipelineTemplates,
  queryOpsApplicationOptions,
  queryOpsImageRegistryList,
  queryNotifyRuleOptions,
  updateOpsAppPipelineStatus
} from '../../api/ops'
import { queryK8sClusterList } from '../../api/k8s'
import { queryAssetHostList } from '../../api/asset'
// I5-J (i5-plan §3.8 row 5·CW-4) — 1,076행 분할 잔류본. 템플릿 선택·파이프라인
// 편집 다이얼로그는 pipeline/AppPipelineEditorDialogs.vue, 실행 폼·실행 상세는
// AppPipelineRunDialogs.vue로 이동(원본 좌표는 커밋 메시지·자식 파일 헤더).
// 자식은 :page 주입(§5 #11 승계)·탭 구조(el-tab-pane)는 부모에 그대로 유지.
import AppPipelineEditorDialogs from './pipeline/AppPipelineEditorDialogs.vue'
import AppPipelineRunDialogs from './pipeline/AppPipelineRunDialogs.vue'

const loading = ref(false)
const saving = ref(false)
const rows = ref([])
const total = ref(0)
const stats = ref({ total: 0, enabled: 0, failed: 0 })
const appOptions = ref([])
const k8sClusterOptions = ref([])
const notifyRuleOptions = ref([])
const imageRegistryOptions = ref([])
const executorHostOptions = ref([])
const templates = ref([])
const templateVisible = ref(false)
const editorVisible = ref(false)
const runVisible = ref(false)
const runRows = ref([])
const runTotal = ref(0)
const selectedCategory = ref('all')
const selectedTemplate = ref(null)
const activeTab = ref('pipelines')

const query = reactive({
  pageNum: 1,
  pageSize: 10,
  keyword: '',
  appId: undefined,
  env: '',
  status: '',
  techStack: ''
})

const runQuery = reactive({
  pageNum: 1,
  pageSize: 10,
  keyword: '',
  appId: undefined,
  pipelineId: undefined,
  env: '',
  status: ''
})

const form = reactive({
  id: undefined,
  name: '',
  appId: undefined,
  defaultBranch: '',
  env: 'test',
  techStack: 'custom',
  templateId: 0,
  executorHostId: undefined,
  status: 1,
  description: '',
  stages: []
})

const runForm = reactive({
  pipelineId: undefined,
  pipelineName: '',
  branch: '',
  env: 'test',
  imageTag: '',
  paramsText: ''
})

const runDialogsRef = ref(null)

const currentApp = computed(() => appOptions.value.find((item) => Number(item.id) === Number(form.appId)))

// 원본 :124-142 parseStages — openEdit(부모)·confirmTemplate(자식)이 공유해
// 부모 귀속 유지
function parseStages(definitionJson) {
  if (!definitionJson) return []
  try {
    const data = JSON.parse(definitionJson)
    return Array.isArray(data.stages)
      ? data.stages.map((stage, index) => ({
          id: stage.id || `stage-${index + 1}`,
          name: stage.name || `Stage ${index + 1}`,
          type: stage.type || 'command',
          timeoutSeconds: stage.timeoutSeconds || 1800,
          failurePolicy: stage.failurePolicy || 'stop',
          config: stage.config && typeof stage.config === 'object' ? stage.config : {},
          env: stage.env && typeof stage.env === 'object' ? stage.env : {}
        }))
      : []
  } catch {
    return []
  }
}

// 원본 :359-372 validateStages — openRun(부모)·submitPipeline(자식)이 공유해
// 부모 귀속 유지
function validateStages(stages = form.stages, executorHostId = form.executorHostId) {
  if (!stages.length) return apt('stageRequired')
  const ids = new Set()
  for (let index = 0; index < stages.length; index += 1) {
    const stage = stages[index]
    if (!stage.id || ids.has(stage.id)) return apt('stageIdDuplicate', { index: index + 1 })
    ids.add(stage.id)
    if (!String(stage.name || '').trim()) return apt('stageNameRequired', { index: index + 1 })
    if (['command', 'test', 'build'].includes(stage.type) && !String(stage.config?.script || '').trim()) return apt('stageCommandRequired', { name: stage.name })
    if (stage.type === 'notify' && !stage.config?.notifyRuleId) return apt('stageNotifyRequired', { name: stage.name })
  }
  if (!executorHostId) return apt('executorHostRequired')
  return ''
}

async function loadApps() {
  appOptions.value = await queryOpsApplicationOptions()
}

async function loadTemplates() {
  templates.value = await queryOpsAppPipelineTemplates()
}

async function loadK8sClusters() {
  k8sClusterOptions.value = await queryK8sClusterList()
}

async function loadNotifyRules() {
  notifyRuleOptions.value = await queryNotifyRuleOptions({ scope: 'pipeline' })
}

async function loadImageRegistries() {
  imageRegistryOptions.value = await queryOpsImageRegistryList({ enabledOnly: 1 })
}

async function loadExecutorHosts() {
  const data = await queryAssetHostList({ pageNum: 1, pageSize: 1000 })
  executorHostOptions.value = data.list || []
}

function executorHostLabel(host) {
  const address = host.sshIp || host.privateIp || host.publicIp || '-'
  return `${host.hostName || `Host #${host.id}`} (${address})`
}

function executorHostName(id) {
  const host = executorHostOptions.value.find((item) => Number(item.id) === Number(id))
  return host ? executorHostLabel(host) : (id ? `Asset Host #${id}` : apt('hostNotConfigured'))
}


async function loadData() {
  loading.value = true
  try {
    const data = await queryOpsAppPipelineList(query)
    rows.value = data.list || []
    total.value = data.total || 0
    stats.value = data.stats || { total: total.value, enabled: 0, failed: 0 }
  } finally {
    loading.value = false
  }
}

async function loadRuns() {
  loading.value = true
  try {
    const data = await queryOpsAppPipelineRunList(runQuery)
    runRows.value = data.list || []
    runTotal.value = data.total || 0
  } finally {
    loading.value = false
  }
}

// 원본 :287-291
function openTemplateDialog() {
  selectedCategory.value = 'all'
  selectedTemplate.value = null
  templateVisible.value = true
}

// 원본 :319-336
async function openEdit(row) {
  const detail = await opsAppPipelineInfo(row.id)
  const item = detail.pipeline || row
  Object.assign(form, {
    id: item.id,
    name: item.name || '',
    appId: item.appId,
    defaultBranch: item.defaultBranch || '',
    env: item.env || 'test',
    techStack: item.techStack || 'custom',
    templateId: item.templateId || 0,
    executorHostId: item.executorHostId || undefined,
    status: item.status || 1,
    description: item.description || '',
    stages: detail.stages || parseStages(item.definitionJson)
  })
  editorVisible.value = true
}

// 원본 :404-417
async function openRun(row) {
  const detail = await opsAppPipelineInfo(row.id)
  const stageError = validateStages(detail.stages || [], detail.pipeline?.executorHostId || row.executorHostId)
  if (stageError) return ElMessage.warning(apt('cannotRun', { message: stageError }))
  Object.assign(runForm, {
    pipelineId: row.id,
    pipelineName: row.name,
    branch: row.defaultBranch || 'master',
    env: row.env || 'test',
    imageTag: '',
    paramsText: ''
  })
  runVisible.value = true
}

// 원본 :457-466 openRunDetail은 자식(실행 상세 다이얼로그) 귀속 — 실행 이력
// 테이블의 상세 로그 진입점은 위임 호출로 대체했다.
function openRunDetail(id) {
  runDialogsRef.value?.openRunDetail(id)
}

// 원본 :490-495
async function toggleStatus(row) {
  const next = Number(row.status) === 1 ? 2 : 1
  await updateOpsAppPipelineStatus({ id: row.id, status: next })
  ElMessage.success(next === 1 ? apt('enabledMessage') : apt('disabledMessage'))
  await loadData()
}

// 원본 :497-501
async function copyPipeline(row) {
  await copyOpsAppPipeline(row.id)
  ElMessage.success(apt('duplicateSuccess'))
  await loadData()
}

// 원본 :503-508
async function removePipeline(row) {
  await ElMessageBox.confirm(apt('pipelineDeleteConfirm', { name: row.name }), apt('pipelineDeleteTitle'), { type: 'warning' })
  await deleteOpsAppPipeline(row.id)
  ElMessage.success(apt('deleteSuccess'))
  await loadData()
}

// 원본 :510-515 statusType·:517-519 statusText·:528-540 stageTypeText·
// :521-526 durationText — 목록·실행 이력(부모)과 실행 상세(자식)가 공유해
// 부모 귀속 유지
function statusType(status) {
  if (status === 'success' || Number(status) === 1) return 'success'
  if (status === 'running' || status === 'waiting_approval') return 'warning'
  if (status === 'failed') return 'danger'
  return 'info'
}

function statusText(status) {
  return { success: apt('statusSuccess'), running: apt('statusRunning'), failed: apt('statusFailed'), waiting: apt('statusWaiting'), waiting_approval: apt('statusApprovalWaiting'), 1: apt('activate'), 2: apt('deactivate') }[status] || status || '-'
}

function durationText(ms) {
  if (!ms) return '-'
  const seconds = Math.round(ms / 1000)
  if (seconds < 60) return apt('durationSeconds', { seconds })
  return apt('durationMinutes', { minutes: Math.floor(seconds / 60), seconds: seconds % 60 })
}

function stageTypeText(type) {
  return {
    checkout: 'Source Checkout',
    command: 'Command',
    test: 'Test',
    build: 'Build',
    dockerBuild: 'Image Build',
    dockerPush: 'Image Registry Push',
    k8sDeploy: 'Kubernetes Deploy',
    manual: 'Manual Approval',
    notify: 'Notification'
  }[type] || type
}

// I5-J 자식 주입 번들 — reactive 래핑으로 ref/reactive/computed가 언랩되어
// 자식 템플릿의 page.x / v-model="page.x" 재배선이 동작한다(§5 #11).
const page = reactive({
  form,
  runForm,
  runVisible,
  editorVisible,
  templateVisible,
  saving,
  selectedCategory,
  selectedTemplate,
  templates,
  appOptions,
  k8sClusterOptions,
  notifyRuleOptions,
  imageRegistryOptions,
  executorHostOptions,
  currentApp,
  activeTab,
  validateStages,
  parseStages,
  executorHostLabel,
  statusType,
  statusText,
  durationText,
  stageTypeText,
  loadData,
  loadRuns
})

onMounted(async () => {
  await Promise.all([loadApps(), loadTemplates(), loadK8sClusters(), loadImageRegistries(), loadNotifyRules(), loadExecutorHosts()])
  await loadData()
})
</script>

<template>
  <div class="pipeline-page">
    <div class="hero-panel">
      <div>
        <span class="eyebrow">{{ uiT('cloudNativeDelivery') }}</span>
        <h1>{{ uiT('cicdPipeline') }}</h1>
        <p>{{ apt('heroDesc') }}</p>
      </div>
      <div class="hero-stats">
        <div><strong>{{ stats.total || 0 }}</strong><span>{{ apt('statTotalPipelines') }}</span></div>
        <div><strong>{{ stats.enabled || 0 }}</strong><span>{{ apt('statEnabled') }}</span></div>
        <div><strong>{{ stats.failed || 0 }}</strong><span>{{ apt('statRecentFailed') }}</span></div>
      </div>
    </div>

    <el-tabs v-model="activeTab" class="pipeline-tabs" @tab-change="activeTab === 'runs' && loadRuns()">
      <el-tab-pane :label="apt('tabPipelineList')" name="pipelines">
        <div class="filter-panel">
          <el-form inline>
            <el-form-item :label="uiT('application')">
              <el-select v-model="query.appId" clearable filterable :placeholder="apt('allApplicationsPlaceholder')">
                <el-option v-for="item in appOptions" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
            <el-form-item :label="uiT('environment')">
              <el-select v-model="query.env" clearable :placeholder="apt('allEnvironmentsPlaceholder')">
                <el-option label="dev" value="dev" />
                <el-option label="test" value="test" />
                <el-option label="staging" value="staging" />
                <el-option label="prod" value="prod" />
              </el-select>
            </el-form-item>
            <el-form-item :label="uiT('techStack')">
              <el-select v-model="query.techStack" clearable :placeholder="apt('allTechStacksPlaceholder')">
                <el-option label="Go" value="go" />
                <el-option label="Maven Java" value="maven" />
                <el-option label="Vue" value="vue" />
                <el-option :label="apt('customTechStack')" value="custom" />
              </el-select>
            </el-form-item>
            <el-form-item :label="uiT('keyword')">
              <el-input v-model="query.keyword" clearable :placeholder="apt('pipelineSearchPlaceholder')" @keyup.enter="loadData" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="loadData">{{ apt('search') }}</el-button>
              <el-button @click="Object.assign(query, { pageNum: 1, keyword: '', appId: undefined, env: '', status: '', techStack: '' }); loadData()">{{ apt('reset') }}</el-button>
            </el-form-item>
          </el-form>
          <el-button type="primary" @click="openTemplateDialog">{{ apt('newPipeline') }}</el-button>
        </div>

        <el-table v-loading="loading" :data="rows" class="pipeline-table">
          <el-table-column :label="uiT('pipeline')" min-width="220">
            <template #default="{ row }">
              <div class="name-cell">
                <strong>{{ row.name }}</strong>
                <span>{{ row.appName || '-' }} / {{ row.defaultBranch || '-' }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="repoUrl" label="Repository Address" min-width="260" show-overflow-tooltip />
          <el-table-column prop="env" :label="uiT('environment')" width="100" />
          <el-table-column prop="techStack" :label="uiT('techStack')" width="120" />
          <el-table-column :label="apt('executionNode')" min-width="180" show-overflow-tooltip><template #default="{ row }">{{ executorHostName(row.executorHostId) }}</template></el-table-column>
          <el-table-column prop="stageCount" :label="apt('stageCountCol')" width="90" />
          <el-table-column :label="apt('status')" width="100">
            <template #default="{ row }"><el-tag :type="statusType(row.status)">{{ statusText(row.status) }}</el-tag></template>
          </el-table-column>
          <el-table-column :label="apt('lastRun')" min-width="160">
            <template #default="{ row }">
              <el-tag v-if="row.lastStatus" :type="statusType(row.lastStatus)" size="small">{{ statusText(row.lastStatus) }}</el-tag>
              <span class="muted">{{ row.lastRunAt || '-' }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="apt('actions')" width="330" fixed="right">
            <template #default="{ row }">
              <el-button link type="primary" @click="openRun(row)">{{ apt('runNow') }}</el-button>
              <el-button link type="primary" @click="openEdit(row)">{{ apt('edit') }}</el-button>
              <el-button link type="primary" @click="copyPipeline(row)">{{ apt('duplicate') }}</el-button>
              <el-button link type="primary" @click="Object.assign(runQuery, { pipelineId: row.id }); activeTab = 'runs'; loadRuns()">History</el-button>
              <el-button link :type="Number(row.status) === 1 ? 'warning' : 'success'" @click="toggleStatus(row)">
                {{ Number(row.status) === 1 ? apt('deactivate') : apt('activate') }}
              </el-button>
              <el-button link type="danger" @click="removePipeline(row)">{{ apt('delete') }}</el-button>
            </template>
          </el-table-column>
        </el-table>
        <div class="pager">
          <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" layout="total, prev, pager, next" :total="total" @current-change="loadData" />
        </div>
      </el-tab-pane>

      <el-tab-pane :label="apt('tabPipelineTemplate')" name="templates">
        <div class="template-grid static">
          <div v-for="item in templates" :key="item.id" class="template-card">
            <strong>{{ item.name }}</strong>
            <p>{{ item.description }}</p>
            <span>{{ apt('stageCountSuffix', { techStack: item.techStack, count: item.stageCount }) }}</span>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane :label="apt('tabRunHistory')" name="runs">
        <div class="filter-panel">
          <el-form inline>
            <el-form-item :label="uiT('application')">
              <el-select v-model="runQuery.appId" clearable filterable :placeholder="apt('allApplicationsPlaceholder')">
                <el-option v-for="item in appOptions" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
            <el-form-item :label="apt('status')">
              <el-select v-model="runQuery.status" clearable :placeholder="apt('allStatusesPlaceholder')">
                <el-option :label="apt('statusSuccess')" value="success" />
                <el-option :label="apt('statusRunning')" value="running" />
                <el-option :label="apt('statusFailed')" value="failed" />
              </el-select>
            </el-form-item>
            <el-form-item :label="uiT('keyword')">
              <el-input v-model="runQuery.keyword" clearable :placeholder="apt('runSearchPlaceholder')" @keyup.enter="loadRuns" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="loadRuns">{{ apt('search') }}</el-button>
              <el-button @click="Object.assign(runQuery, { pageNum: 1, keyword: '', appId: undefined, pipelineId: undefined, env: '', status: '' }); loadRuns()">{{ apt('reset') }}</el-button>
            </el-form-item>
          </el-form>
        </div>
        <el-table v-loading="loading" :data="runRows" class="pipeline-table">
          <el-table-column prop="id" label="Run ID" width="100" />
          <el-table-column prop="pipelineName" :label="uiT('pipeline')" min-width="180" />
          <el-table-column prop="appName" :label="uiT('application')" min-width="160" />
          <el-table-column prop="env" :label="uiT('environment')" width="90" />
          <el-table-column prop="branch" label="Branch" width="130" />
          <el-table-column prop="imageTag" label="Image Tag" min-width="150" />
          <el-table-column :label="apt('status')" width="100">
            <template #default="{ row }"><el-tag :type="statusType(row.status)">{{ statusText(row.status) }}</el-tag></template>
          </el-table-column>
          <el-table-column label="Duration" width="120">
            <template #default="{ row }">{{ durationText(row.durationMs) }}</template>
          </el-table-column>
          <el-table-column prop="createTime" label="Start Time" min-width="180" />
          <el-table-column :label="apt('actions')" width="110" fixed="right">
            <template #default="{ row }"><el-button link type="primary" @click="openRunDetail(row.id)">{{ apt('detailLog') }}</el-button></template>
          </el-table-column>
        </el-table>
        <div class="pager">
          <el-pagination
            v-model:current-page="runQuery.pageNum"
            v-model:page-size="runQuery.pageSize"
            layout="total, prev, pager, next"
            :total="runTotal"
            @current-change="loadRuns"
          />
        </div>
      </el-tab-pane>
    </el-tabs>

    <AppPipelineEditorDialogs :page="page" />

    <AppPipelineRunDialogs ref="runDialogsRef" :page="page" />
  </div>
</template>

<style scoped>
.pipeline-page { padding: 24px; }
.hero-panel { display: flex; justify-content: space-between; gap: 24px; padding: 28px; margin-bottom: 18px; border: 1px solid #dbe7f7; border-radius: 14px; background: linear-gradient(135deg, #f8fbff 0%, #eef5ff 100%); }
.eyebrow { color: #2f6be6; font-size: 12px; font-weight: 800; letter-spacing: .08em; text-transform: uppercase; }
.hero-panel h1 { margin: 8px 0; color: #071b3d; font-size: 30px; }
.hero-panel p { margin: 0; color: #6b7c9b; }
.hero-stats { display: grid; grid-template-columns: repeat(3, 132px); gap: 12px; }
.hero-stats div { padding: 16px; border: 1px solid #dbe7f7; border-radius: 10px; background: #fff; }
.hero-stats strong { display: block; color: #2f6be6; font-size: 28px; }
.hero-stats span, .muted { color: #7d8daa; font-size: 13px; }
.pipeline-tabs { padding: 18px 22px; border: 1px solid #e3ebf7; border-radius: 14px; background: #fff; }
.filter-panel { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; margin-bottom: 18px; }
:deep(.filter-panel .el-select) { width: 220px; }
:deep(.filter-panel .el-input) { width: 280px; }
.pipeline-table { border: 1px solid #edf2f9; border-radius: 10px; overflow: hidden; }
.name-cell { display: flex; flex-direction: column; gap: 4px; }
.name-cell strong { color: #0b5ed7; }
.name-cell span { color: #7d8daa; font-size: 12px; }
.pager { display: flex; justify-content: flex-end; padding-top: 16px; }

/* 원본 :1005-1011 — 템플릿 다이얼로그(자식)와 공유 복사 6건.
   .template-grid.static만 탭 전용 부모 잔류 */
.template-grid { display: grid; grid-template-columns: repeat(2, minmax(280px, 1fr)); gap: 16px; }
.template-grid.static { grid-template-columns: repeat(3, minmax(260px, 1fr)); }
.template-card, .blank-template { min-height: 140px; padding: 20px; border: 1px solid #d7e4f5; border-radius: 8px; background: #fff; cursor: pointer; }
.template-card.selected, .template-card:hover, .blank-template:hover { border-color: #2f6be6; box-shadow: 0 8px 24px rgba(47, 107, 230, .12); }
.template-card strong, .blank-template strong { color: #10213d; font-size: 16px; }
.template-card p, .blank-template p { color: #6b7c9b; }
.template-card span { color: #1677ff; font-weight: 700; }
</style>
