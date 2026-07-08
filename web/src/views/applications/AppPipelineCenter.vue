<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  copyOpsAppPipeline,
  deleteOpsAppPipeline,
  opsAppPipelineInfo,
  opsAppPipelineRunInfo,
  queryOpsAppPipelineList,
  queryOpsAppPipelineRunList,
  queryOpsAppPipelineTemplates,
  queryOpsApplicationOptions,
  runOpsAppPipeline,
  saveOpsAppPipeline,
  updateOpsAppPipelineStatus
} from '../../api/ops'
import { queryK8sClusterList } from '../../api/k8s'

const loading = ref(false)
const saving = ref(false)
const rows = ref([])
const total = ref(0)
const stats = ref({ total: 0, enabled: 0, failed: 0 })
const appOptions = ref([])
const k8sClusterOptions = ref([])
const templates = ref([])
const templateVisible = ref(false)
const editorVisible = ref(false)
const runVisible = ref(false)
const runDetailVisible = ref(false)
const runRows = ref([])
const runTotal = ref(0)
const selectedCategory = ref('全部模板')
const selectedTemplate = ref(null)
const activeTab = ref('pipelines')
const logBodyRef = ref()
const runRefreshTimer = ref()

const categories = ['全部模板', 'Java', 'Node.js', 'Go', 'Python', 'Vue', '空模板']

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

const currentRun = ref({ run: {}, stages: [] })

const currentApp = computed(() => appOptions.value.find((item) => Number(item.id) === Number(form.appId)))
const filteredTemplates = computed(() => {
  if (selectedCategory.value === '全部模板') return templates.value
  if (selectedCategory.value === '空模板') return []
  return templates.value.filter((item) => item.category === selectedCategory.value || item.techStack === selectedCategory.value)
})

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    appId: undefined,
    defaultBranch: '',
    env: 'test',
    techStack: 'custom',
    templateId: 0,
    status: 1,
    description: '',
    stages: []
  })
}

function parseStages(definitionJson) {
  if (!definitionJson) return []
  try {
    const data = JSON.parse(definitionJson)
    return Array.isArray(data.stages)
      ? data.stages.map((stage, index) => ({
          id: stage.id || `stage-${index + 1}`,
          name: stage.name || `阶段 ${index + 1}`,
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

function stringifyDefinition() {
  return JSON.stringify({ stages: form.stages }, null, 2)
}

function defaultStage(type = 'command') {
  const next = form.stages.length + 1
  const names = {
    checkout: '代码拉取',
    command: '构建命令',
    test: '单元测试',
    dockerBuild: 'Docker 镜像构建',
    dockerPush: '上传镜像仓库',
    k8sDeploy: 'K8s 发布',
    manual: '人工确认',
    notify: '消息通知'
  }
  const stage = {
    id: `${type}-${Date.now()}-${next}`,
    name: names[type] || `阶段 ${next}`,
    type,
    timeoutSeconds: 1800,
    failurePolicy: type === 'notify' ? 'ignore' : 'stop',
    config: {},
    env: {}
  }
  normalizeStageConfig(stage)
  return stage
}

function normalizeStageConfig(stage) {
  if (!stage.config || typeof stage.config !== 'object') stage.config = {}
  if (['command', 'test', 'build'].includes(stage.type) && stage.config.script === undefined) {
    stage.config.script = ''
  }
  if (stage.type === 'dockerBuild') {
    if (!stage.config.repository) stage.config.repository = ''
    if (!stage.config.dockerfile) stage.config.dockerfile = 'Dockerfile'
    if (!stage.config.context) stage.config.context = '.'
  }
  if (stage.type === 'dockerPush' && !stage.config.repository) {
    stage.config.repository = ''
  }
  if (stage.type === 'k8sDeploy') {
    if (!stage.config.clusterId) stage.config.clusterId = undefined
    if (!stage.config.workloadType) stage.config.workloadType = 'deployment'
    if (!stage.config.namespace) stage.config.namespace = ''
    if (!stage.config.workload) stage.config.workload = ''
    if (!stage.config.container) stage.config.container = ''
    if (!stage.config.repository) stage.config.repository = ''
  }
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

function openTemplateDialog() {
  selectedCategory.value = '全部模板'
  selectedTemplate.value = null
  templateVisible.value = true
}

function createBlankPipeline() {
  resetForm()
  form.stages = []
  templateVisible.value = false
  editorVisible.value = true
}

function useTemplate(template) {
  selectedTemplate.value = template
}

function confirmTemplate() {
  if (!selectedTemplate.value) {
    ElMessage.warning('请选择流水线模板，或使用空白流水线')
    return
  }
  resetForm()
  form.name = selectedTemplate.value.name.replace('通用模板', '流水线')
  form.techStack = selectedTemplate.value.techStack || 'custom'
  form.templateId = selectedTemplate.value.id
  form.description = selectedTemplate.value.description || ''
  form.stages = parseStages(selectedTemplate.value.definitionJson)
  templateVisible.value = false
  editorVisible.value = true
}

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
    status: item.status || 1,
    description: item.description || '',
    stages: detail.stages || parseStages(item.definitionJson)
  })
  editorVisible.value = true
}

function fillFromApp() {
  if (!currentApp.value) return
  if (!form.defaultBranch) form.defaultBranch = currentApp.value.branch || 'master'
  if (!form.env) form.env = currentApp.value.env || 'test'
}

function addStage(type) {
  form.stages.push(defaultStage(type))
}

function removeStage(index) {
  form.stages.splice(index, 1)
}

async function submitPipeline() {
  if (!form.name || !form.appId) {
    ElMessage.warning('请填写流水线名称并选择所属应用')
    return
  }
  saving.value = true
  try {
    await saveOpsAppPipeline({
      id: form.id,
      name: form.name,
      appId: form.appId,
      defaultBranch: form.defaultBranch,
      env: form.env,
      techStack: form.techStack,
      templateId: form.templateId,
      status: form.status,
      description: form.description,
      definitionJson: stringifyDefinition()
    })
    ElMessage.success('保存成功')
    editorVisible.value = false
    await loadData()
  } finally {
    saving.value = false
  }
}

function openRun(row) {
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

async function submitRun() {
  let params = {}
  if (runForm.paramsText.trim()) {
    try {
      params = JSON.parse(runForm.paramsText)
    } catch {
      ElMessage.warning('自定义参数需要是 JSON 对象')
      return
    }
  }
  const data = await runOpsAppPipeline({
    pipelineId: runForm.pipelineId,
    branch: runForm.branch,
    env: runForm.env,
    imageTag: runForm.imageTag,
    params
  })
  ElMessage.success(`流水线已开始执行：#${data.runId}`)
  runVisible.value = false
  await loadData()
  await openRunDetail(data.runId)
}

async function openRunDetail(id) {
  const data = await opsAppPipelineRunInfo(id)
  currentRun.value = { run: data.run || {}, stages: data.stages || [] }
  runDetailVisible.value = true
  startRunRefresh()
  await nextTick()
  if (logBodyRef.value) logBodyRef.value.scrollTop = logBodyRef.value.scrollHeight
}

function startRunRefresh() {
  stopRunRefresh()
  if (currentRun.value.run?.status !== 'running') return
  runRefreshTimer.value = window.setInterval(async () => {
    if (!currentRun.value.run?.id) return
    const data = await opsAppPipelineRunInfo(currentRun.value.run.id)
    currentRun.value = { run: data.run || {}, stages: data.stages || [] }
    if (currentRun.value.run.status !== 'running') {
      stopRunRefresh()
      await loadData()
      if (activeTab.value === 'runs') await loadRuns()
    }
  }, 2500)
}

function stopRunRefresh() {
  if (runRefreshTimer.value) {
    window.clearInterval(runRefreshTimer.value)
    runRefreshTimer.value = undefined
  }
}

async function toggleStatus(row) {
  const next = Number(row.status) === 1 ? 2 : 1
  await updateOpsAppPipelineStatus({ id: row.id, status: next })
  ElMessage.success(next === 1 ? '已启用' : '已禁用')
  await loadData()
}

async function copyPipeline(row) {
  await copyOpsAppPipeline(row.id)
  ElMessage.success('复制成功')
  await loadData()
}

async function removePipeline(row) {
  await ElMessageBox.confirm(`确认删除流水线「${row.name}」及其执行记录？`, '删除流水线', { type: 'warning' })
  await deleteOpsAppPipeline(row.id)
  ElMessage.success('删除成功')
  await loadData()
}

function statusType(status) {
  if (status === 'success' || Number(status) === 1) return 'success'
  if (status === 'running') return 'warning'
  if (status === 'failed') return 'danger'
  return 'info'
}

function statusText(status) {
  return { success: '成功', running: '执行中', failed: '失败', waiting: '等待中', 1: '启用', 2: '禁用' }[status] || status || '-'
}

function durationText(ms) {
  if (!ms) return '-'
  const seconds = Math.round(ms / 1000)
  if (seconds < 60) return `${seconds} 秒`
  return `${Math.floor(seconds / 60)} 分 ${seconds % 60} 秒`
}

function stageTypeText(type) {
  return {
    checkout: '代码拉取',
    command: '命令',
    test: '测试',
    build: '构建',
    dockerBuild: '镜像构建',
    dockerPush: '上传镜像仓库',
    k8sDeploy: 'K8s 发布',
    manual: '人工确认',
    notify: '消息通知'
  }[type] || type
}

const combinedLog = computed(() => {
  return (currentRun.value.stages || [])
    .map((stage) => `===== ${stage.stageName} / ${statusText(stage.status)} =====\n${stage.log || stage.summary || ''}`)
    .join('\n\n')
})

function downloadRunLog() {
  const name = `${currentRun.value.run.pipelineName || 'pipeline'}-${currentRun.value.run.id || 'run'}.log`.replace(/[\\/:*?"<>|]/g, '_')
  const blob = new Blob([combinedLog.value], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = name
  link.click()
  URL.revokeObjectURL(url)
}

watch(runDetailVisible, (visible) => {
  if (!visible) stopRunRefresh()
})

onMounted(async () => {
  await Promise.all([loadApps(), loadTemplates(), loadK8sClusters()])
  await loadData()
})

onBeforeUnmount(stopRunRefresh)
</script>

<template>
  <div class="pipeline-page">
    <div class="hero-panel">
      <div>
        <span class="eyebrow">Cloud Native Delivery</span>
        <h1>CI/CD 流水线</h1>
        <p>从代码拉取、构建、测试、制品、发布到通知，统一编排应用交付流程。</p>
      </div>
      <div class="hero-stats">
        <div><strong>{{ stats.total || 0 }}</strong><span>流水线总数</span></div>
        <div><strong>{{ stats.enabled || 0 }}</strong><span>启用中</span></div>
        <div><strong>{{ stats.failed || 0 }}</strong><span>最近失败</span></div>
      </div>
    </div>

    <el-tabs v-model="activeTab" class="pipeline-tabs" @tab-change="activeTab === 'runs' && loadRuns()">
      <el-tab-pane label="流水线列表" name="pipelines">
        <div class="filter-panel">
          <el-form inline>
            <el-form-item label="应用">
              <el-select v-model="query.appId" clearable filterable placeholder="全部应用">
                <el-option v-for="item in appOptions" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
            <el-form-item label="环境">
              <el-select v-model="query.env" clearable placeholder="全部环境">
                <el-option label="dev" value="dev" />
                <el-option label="test" value="test" />
                <el-option label="staging" value="staging" />
                <el-option label="prod" value="prod" />
              </el-select>
            </el-form-item>
            <el-form-item label="技术栈">
              <el-select v-model="query.techStack" clearable placeholder="全部技术栈">
                <el-option label="Go" value="go" />
                <el-option label="Maven Java" value="maven" />
                <el-option label="Vue" value="vue" />
                <el-option label="自定义" value="custom" />
              </el-select>
            </el-form-item>
            <el-form-item label="关键字">
              <el-input v-model="query.keyword" clearable placeholder="搜索流水线 / 应用 / 仓库" @keyup.enter="loadData" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="loadData">搜索</el-button>
              <el-button @click="Object.assign(query, { pageNum: 1, keyword: '', appId: undefined, env: '', status: '', techStack: '' }); loadData()">重置</el-button>
            </el-form-item>
          </el-form>
          <el-button type="primary" @click="openTemplateDialog">新建流水线</el-button>
        </div>

        <el-table v-loading="loading" :data="rows" class="pipeline-table">
          <el-table-column label="流水线" min-width="220">
            <template #default="{ row }">
              <div class="name-cell">
                <strong>{{ row.name }}</strong>
                <span>{{ row.appName || '-' }} / {{ row.defaultBranch || '-' }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="repoUrl" label="仓库地址" min-width="260" show-overflow-tooltip />
          <el-table-column prop="env" label="环境" width="100" />
          <el-table-column prop="techStack" label="技术栈" width="120" />
          <el-table-column prop="stageCount" label="阶段数" width="90" />
          <el-table-column label="状态" width="100">
            <template #default="{ row }"><el-tag :type="statusType(row.status)">{{ statusText(row.status) }}</el-tag></template>
          </el-table-column>
          <el-table-column label="最近执行" min-width="160">
            <template #default="{ row }">
              <el-tag v-if="row.lastStatus" :type="statusType(row.lastStatus)" size="small">{{ statusText(row.lastStatus) }}</el-tag>
              <span class="muted">{{ row.lastRunAt || '-' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="330" fixed="right">
            <template #default="{ row }">
              <el-button link type="primary" @click="openRun(row)">立即执行</el-button>
              <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
              <el-button link type="primary" @click="copyPipeline(row)">复制</el-button>
              <el-button link type="primary" @click="Object.assign(runQuery, { pipelineId: row.id }); activeTab = 'runs'; loadRuns()">历史</el-button>
              <el-button link :type="Number(row.status) === 1 ? 'warning' : 'success'" @click="toggleStatus(row)">
                {{ Number(row.status) === 1 ? '禁用' : '启用' }}
              </el-button>
              <el-button link type="danger" @click="removePipeline(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
        <div class="pager">
          <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" layout="total, prev, pager, next" :total="total" @current-change="loadData" />
        </div>
      </el-tab-pane>

      <el-tab-pane label="流水线模板" name="templates">
        <div class="template-grid static">
          <div v-for="item in templates" :key="item.id" class="template-card">
            <strong>{{ item.name }}</strong>
            <p>{{ item.description }}</p>
            <span>{{ item.techStack }} / {{ item.stageCount }} 个阶段</span>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="执行记录" name="runs">
        <div class="filter-panel">
          <el-form inline>
            <el-form-item label="应用">
              <el-select v-model="runQuery.appId" clearable filterable placeholder="全部应用">
                <el-option v-for="item in appOptions" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
            <el-form-item label="状态">
              <el-select v-model="runQuery.status" clearable placeholder="全部状态">
                <el-option label="成功" value="success" />
                <el-option label="执行中" value="running" />
                <el-option label="失败" value="failed" />
              </el-select>
            </el-form-item>
            <el-form-item label="关键字">
              <el-input v-model="runQuery.keyword" clearable placeholder="流水线 / 应用 / 镜像 Tag" @keyup.enter="loadRuns" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="loadRuns">搜索</el-button>
              <el-button @click="Object.assign(runQuery, { pageNum: 1, keyword: '', appId: undefined, pipelineId: undefined, env: '', status: '' }); loadRuns()">重置</el-button>
            </el-form-item>
          </el-form>
        </div>
        <el-table v-loading="loading" :data="runRows" class="pipeline-table">
          <el-table-column prop="id" label="执行编号" width="100" />
          <el-table-column prop="pipelineName" label="流水线" min-width="180" />
          <el-table-column prop="appName" label="应用" min-width="160" />
          <el-table-column prop="env" label="环境" width="90" />
          <el-table-column prop="branch" label="分支" width="130" />
          <el-table-column prop="imageTag" label="镜像 Tag" min-width="150" />
          <el-table-column label="状态" width="100">
            <template #default="{ row }"><el-tag :type="statusType(row.status)">{{ statusText(row.status) }}</el-tag></template>
          </el-table-column>
          <el-table-column label="耗时" width="120">
            <template #default="{ row }">{{ durationText(row.durationMs) }}</template>
          </el-table-column>
          <el-table-column prop="createTime" label="开始时间" min-width="180" />
          <el-table-column label="操作" width="110" fixed="right">
            <template #default="{ row }"><el-button link type="primary" @click="openRunDetail(row.id)">详情/日志</el-button></template>
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

    <el-dialog v-model="templateVisible" title="选择流水线模板" width="980px" class="template-dialog">
      <div class="template-picker">
        <aside>
          <button v-for="item in categories" :key="item" :class="{ active: selectedCategory === item }" @click="selectedCategory = item">
            {{ item }}
          </button>
        </aside>
        <main>
          <div v-if="selectedCategory === '空模板'" class="blank-template" @click="createBlankPipeline">
            <strong>空白流水线</strong>
            <p>从空白画布开始，自定义所有阶段。</p>
          </div>
          <div v-else class="template-grid">
            <div v-for="item in filteredTemplates" :key="item.id" class="template-card" :class="{ selected: selectedTemplate?.id === item.id }" @click="useTemplate(item)">
              <strong>{{ item.name }}</strong>
              <p>{{ item.description }}</p>
              <span>{{ item.techStack }} / {{ item.stageCount }} 个阶段</span>
            </div>
          </div>
        </main>
      </div>
      <template #footer>
        <el-button @click="templateVisible = false">取消</el-button>
        <el-button @click="createBlankPipeline">空白流水线</el-button>
        <el-button type="primary" @click="confirmTemplate">使用选中模板</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="editorVisible" :title="form.id ? '编辑流水线' : '新建流水线'" width="1180px" class="pipeline-editor">
      <el-form label-width="100px">
        <div class="form-grid">
          <el-form-item label="流水线名称" required><el-input v-model="form.name" placeholder="请输入流水线名称" /></el-form-item>
          <el-form-item label="所属应用" required>
            <el-select v-model="form.appId" filterable placeholder="请选择应用" @change="fillFromApp">
              <el-option v-for="item in appOptions" :key="item.id" :label="item.name" :value="item.id" />
            </el-select>
          </el-form-item>
          <el-form-item label="默认分支"><el-input v-model="form.defaultBranch" placeholder="默认使用应用分支" /></el-form-item>
          <el-form-item label="默认环境">
            <el-select v-model="form.env">
              <el-option label="dev" value="dev" />
              <el-option label="test" value="test" />
              <el-option label="staging" value="staging" />
              <el-option label="prod" value="prod" />
            </el-select>
          </el-form-item>
          <el-form-item label="技术栈">
            <el-select v-model="form.techStack">
              <el-option label="Go" value="go" />
              <el-option label="Maven Java" value="maven" />
              <el-option label="Vue" value="vue" />
              <el-option label="自定义" value="custom" />
            </el-select>
          </el-form-item>
          <el-form-item label="状态">
            <el-radio-group v-model="form.status">
              <el-radio :value="1">启用</el-radio>
              <el-radio :value="2">禁用</el-radio>
            </el-radio-group>
          </el-form-item>
        </div>
        <el-form-item label="描述"><el-input v-model="form.description" type="textarea" :rows="2" placeholder="说明流水线用途、环境和风险提示" /></el-form-item>
      </el-form>

      <div class="stage-toolbar">
        <strong>阶段编排</strong>
        <div>
          <el-button size="small" @click="addStage('checkout')">代码拉取</el-button>
          <el-button size="small" @click="addStage('command')">命令</el-button>
          <el-button size="small" @click="addStage('dockerBuild')">镜像构建</el-button>
          <el-button size="small" @click="addStage('dockerPush')">上传镜像仓库</el-button>
          <el-button size="small" @click="addStage('k8sDeploy')">K8s 发布</el-button>
          <el-button size="small" @click="addStage('manual')">人工确认</el-button>
          <el-button size="small" @click="addStage('notify')">消息通知</el-button>
        </div>
      </div>
      <div class="stage-editor">
        <el-empty v-if="!form.stages.length" description="当前为空白流水线，请添加阶段" />
        <div v-for="(stage, index) in form.stages" :key="stage.id" class="stage-config-card">
          <div class="stage-index">{{ index + 1 }}</div>
          <div class="stage-fields">
            <el-input v-model="stage.name" placeholder="阶段名称" />
            <el-select v-model="stage.type" @change="normalizeStageConfig(stage)">
              <el-option label="代码拉取" value="checkout" />
              <el-option label="命令" value="command" />
              <el-option label="测试" value="test" />
              <el-option label="构建" value="build" />
              <el-option label="镜像构建" value="dockerBuild" />
              <el-option label="上传镜像仓库" value="dockerPush" />
              <el-option label="K8s 发布" value="k8sDeploy" />
              <el-option label="人工确认" value="manual" />
              <el-option label="消息通知" value="notify" />
            </el-select>
            <el-input-number v-model="stage.timeoutSeconds" :min="10" :max="7200" controls-position="right" />
            <el-select v-model="stage.failurePolicy">
              <el-option label="失败终止" value="stop" />
              <el-option label="忽略继续" value="ignore" />
              <el-option label="人工确认" value="manual" />
            </el-select>
          </div>
          <el-input
            v-if="['command', 'test', 'build'].includes(stage.type)"
            v-model="stage.config.script"
            type="textarea"
            :rows="4"
            placeholder="请输入要执行的 Shell 命令，例如 npm run build / go test ./..."
          />
          <div v-else-if="stage.type === 'dockerBuild'" class="stage-config-grid">
            <el-input v-model="stage.config.repository" placeholder="镜像名，例如 registry.example.com/app/{{appCode}}，Tag 默认使用执行参数" />
            <el-input v-model="stage.config.dockerfile" placeholder="Dockerfile 路径，默认 Dockerfile" />
            <el-input v-model="stage.config.context" placeholder="构建上下文，默认 ." />
          </div>
          <div v-else-if="stage.type === 'dockerPush'" class="stage-config-grid">
            <el-input v-model="stage.config.repository" placeholder="镜像仓库地址，例如 registry.example.com/app/{{appCode}}，Tag 默认使用执行参数" />
          </div>
          <div v-else-if="stage.type === 'k8sDeploy'" class="stage-config-grid">
            <el-select v-model="stage.config.clusterId" filterable placeholder="选择 K8s 集群">
              <el-option
                v-for="cluster in k8sClusterOptions"
                :key="cluster.id"
                :label="`${cluster.name}（${cluster.statusText || cluster.status || '-'} / ${cluster.version || '-'}）`"
                :value="cluster.id"
              />
            </el-select>
            <el-select v-model="stage.config.workloadType" placeholder="工作负载类型">
              <el-option label="Deployment" value="deployment" />
              <el-option label="StatefulSet" value="statefulset" />
              <el-option label="DaemonSet" value="daemonset" />
            </el-select>
            <el-input v-model="stage.config.namespace" placeholder="命名空间，例如 default" />
            <el-input v-model="stage.config.workload" placeholder="工作负载名称，默认应用编码" />
            <el-input v-model="stage.config.container" placeholder="容器名称" />
            <el-input v-model="stage.config.repository" placeholder="镜像名，例如 registry.example.com/app/{{appCode}}" />
          </div>
          <el-alert
            v-else-if="stage.type === 'checkout'"
            type="info"
            show-icon
            :closable="false"
            title="代码拉取会使用所属应用的 Git/SVN 仓库地址和执行时选择的分支。"
          />
          <el-alert
            v-else
            type="info"
            show-icon
            :closable="false"
            title="该阶段当前会记录执行结果，后续可接入审批中心或消息通知规则。"
          />
          <el-button link type="danger" @click="removeStage(index)">删除阶段</el-button>
        </div>
      </div>
      <template #footer>
        <el-button @click="editorVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitPipeline">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="runVisible" title="立即执行流水线" width="680px">
      <el-form label-width="100px">
        <el-form-item label="流水线"><el-input v-model="runForm.pipelineName" disabled /></el-form-item>
        <el-form-item label="执行环境">
          <el-select v-model="runForm.env">
            <el-option label="dev" value="dev" />
            <el-option label="test" value="test" />
            <el-option label="staging" value="staging" />
            <el-option label="prod" value="prod" />
          </el-select>
        </el-form-item>
        <el-form-item label="分支/Tag"><el-input v-model="runForm.branch" /></el-form-item>
        <el-form-item label="镜像 Tag"><el-input v-model="runForm.imageTag" placeholder="默认自动生成时间戳" /></el-form-item>
        <el-form-item label="自定义参数"><el-input v-model="runForm.paramsText" type="textarea" :rows="4" placeholder='例如：{"version":"1.0.0"}' /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="runVisible = false">取消</el-button>
        <el-button type="primary" @click="submitRun">开始执行</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="runDetailVisible" title="流水线执行详情" width="1080px" class="run-detail-dialog">
      <div class="run-summary">
        <div><span>流水线</span><strong>{{ currentRun.run.pipelineName || '-' }}</strong></div>
        <div><span>状态</span><el-tag :type="statusType(currentRun.run.status)">{{ statusText(currentRun.run.status) }}</el-tag></div>
        <div><span>环境</span><strong>{{ currentRun.run.env || '-' }}</strong></div>
        <div><span>耗时</span><strong>{{ durationText(currentRun.run.durationMs) }}</strong></div>
      </div>
      <div class="run-detail-actions">
        <el-button type="primary" link @click="currentRun.run?.id && openRunDetail(currentRun.run.id)">刷新</el-button>
        <el-button type="primary" link @click="downloadRunLog">下载日志</el-button>
      </div>
      <div class="run-detail-grid">
        <div class="run-stages">
          <div v-for="stage in currentRun.stages" :key="stage.id" class="run-stage" :class="stage.status">
            <span></span>
            <div>
              <strong>{{ stage.stageName }}</strong>
              <p>{{ stageTypeText(stage.stageType) }} / {{ statusText(stage.status) }} / {{ durationText(stage.durationMs) }}</p>
            </div>
          </div>
        </div>
        <pre ref="logBodyRef" class="run-log">{{ combinedLog }}</pre>
      </div>
    </el-dialog>
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
.template-picker { display: grid; grid-template-columns: 180px 1fr; min-height: 520px; border-top: 1px solid #e5edf8; border-bottom: 1px solid #e5edf8; }
.template-picker aside { padding: 16px 12px; background: #f5f8fc; border-right: 1px solid #e5edf8; }
.template-picker aside button { display: block; width: 100%; height: 40px; padding: 0 14px; border: 0; border-radius: 6px; background: transparent; color: #49617f; text-align: left; cursor: pointer; }
.template-picker aside button.active { background: #e7f0ff; color: #1677ff; font-weight: 700; }
.template-picker main { padding: 22px; }
.template-grid { display: grid; grid-template-columns: repeat(2, minmax(280px, 1fr)); gap: 16px; }
.template-grid.static { grid-template-columns: repeat(3, minmax(260px, 1fr)); }
.template-card, .blank-template { min-height: 140px; padding: 20px; border: 1px solid #d7e4f5; border-radius: 8px; background: #fff; cursor: pointer; }
.template-card.selected, .template-card:hover, .blank-template:hover { border-color: #2f6be6; box-shadow: 0 8px 24px rgba(47, 107, 230, .12); }
.template-card strong, .blank-template strong { color: #10213d; font-size: 16px; }
.template-card p, .blank-template p { color: #6b7c9b; }
.template-card span { color: #1677ff; font-weight: 700; }
.form-grid { display: grid; grid-template-columns: repeat(2, minmax(320px, 1fr)); column-gap: 28px; }
.stage-toolbar { display: flex; align-items: center; justify-content: space-between; margin: 16px 0; padding-top: 14px; border-top: 1px solid #edf2f9; }
.stage-editor { max-height: 460px; overflow: auto; padding-right: 6px; }
.stage-config-card { position: relative; margin-bottom: 12px; padding: 16px 16px 16px 54px; border: 1px solid #e3ebf7; border-radius: 10px; background: #fbfdff; }
.stage-index { position: absolute; left: 16px; top: 16px; width: 26px; height: 26px; display: grid; place-items: center; border-radius: 50%; background: #2f6be6; color: #fff; font-weight: 700; }
.stage-fields { display: grid; grid-template-columns: 1.4fr 1fr 140px 130px; gap: 10px; margin-bottom: 10px; }
.stage-config-grid { display: grid; grid-template-columns: repeat(2, minmax(220px, 1fr)); gap: 10px; }
.run-summary { display: grid; grid-template-columns: 2fr 1fr 1fr 1fr; gap: 12px; margin-bottom: 16px; }
.run-summary div { padding: 14px; border: 1px solid #e3ebf7; border-radius: 8px; background: #fbfdff; }
.run-summary span { display: block; margin-bottom: 6px; color: #7d8daa; }
.run-detail-actions { display: flex; justify-content: flex-end; gap: 12px; margin: -4px 0 12px; }
.run-detail-grid { display: grid; grid-template-columns: 300px 1fr; gap: 16px; }
.run-stages { border: 1px solid #e3ebf7; border-radius: 10px; padding: 12px; }
.run-stage { display: flex; gap: 10px; padding: 12px; border-bottom: 1px solid #edf2f9; }
.run-stage span { width: 10px; height: 10px; margin-top: 6px; border-radius: 50%; background: #b8c5d9; }
.run-stage.success span { background: #67c23a; }
.run-stage.running span { background: #e6a23c; }
.run-stage.failed span { background: #f56c6c; }
.run-stage strong { color: #10213d; }
.run-stage p { margin: 4px 0 0; color: #7d8daa; }
.run-log { min-height: 480px; max-height: 560px; margin: 0; padding: 16px; overflow: auto; border-radius: 10px; background: #111827; color: #d6e2ff; font-family: Consolas, Monaco, monospace; font-size: 13px; line-height: 1.6; white-space: pre-wrap; }
</style>
