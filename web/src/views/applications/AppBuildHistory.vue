<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { opsAppReleaseInfo, queryOpsApplicationOptions, queryOpsAppReleaseList, retryOpsAppRelease } from '../../api/ops'
import { apt } from '../../utils/application-i18n'

const route = useRoute()
const loading = ref(false)
const rows = ref([])
const total = ref(0)
const appOptions = ref([])
const detailVisible = ref(false)
const detailLoading = ref(false)
const currentDetail = ref({})
const logKeyword = ref('')
const dateRange = ref([])
const logBodyRef = ref()
let detailPollTimer = null
let refreshingDetail = false

const query = reactive({ pageNum: 1, pageSize: 10, appId: undefined, env: '', keyword: '', status: '' })

async function loadApps() { appOptions.value = await queryOpsApplicationOptions() }
async function loadData() {
  loading.value = true
  try {
    const data = await queryOpsAppReleaseList({ ...query, startTime: dateRange.value?.[0] || '', endTime: dateRange.value?.[1] || '' })
    rows.value = data.list || []
    total.value = data.total || 0
  } finally { loading.value = false }
}
function search() { query.pageNum = 1; loadData() }
function reset() { Object.assign(query, { pageNum: 1, appId: undefined, env: '', status: '', keyword: '' }); dateRange.value = []; loadData() }
function toggleStatus(status) { query.status = query.status === status ? '' : status; search() }
function statusType(status) { return ({ success: 'success', running: 'warning', failed: 'danger', waiting: 'info' })[status] || 'info' }
function statusText(status) { return ({ success: apt('statusSuccess'), running: apt('statusRunning'), failed: apt('statusFailed'), waiting: apt('statusWaitingShort') })[status] || status || '-' }
function stageText(stage) { return ({ checkout: 'Source Checkout', build: 'Build', post_build: apt('postBuildTitle'), prepare: apt('stagePrepare'), done: apt('stageDone') })[stage] || stage || '-' }
function sourceText(row) { return row.repoType === 'svn' ? 'SVN Update' : 'Git Checkout' }
function versionText(row) { return row.repoType === 'svn' ? (row.branch || 'HEAD') : (row.branch || '-') }
function durationText(ms) { if (!ms) return '-'; const seconds = Math.max(0, Math.round(ms / 1000)); return seconds < 60 ? apt('durationSecondsCompact', { seconds }) : apt('durationMinutesCompact', { minutes: Math.floor(seconds / 60), seconds: seconds % 60 }) }
function releaseTitle(row) { return `${row.buildTaskName || row.appName || 'Build Task'} #${row.id || ''}` }
function stages(row) {
  const stateFor = (name) => {
    if (row.status === 'failed' && row.stage === name) return 'failed'
    if (row.status === 'running' && row.stage === name) return 'running'
    if ((name === 'build' && row.stage === 'checkout') || (name === 'post_build' && ['checkout', 'build'].includes(row.stage))) return 'waiting'
    return row.status === 'waiting' ? 'waiting' : 'success'
  }
  const list = [{ key: 'checkout', name: sourceText(row) }, { key: 'build', name: 'Build' }]
  if (row.deployLog || row.stage === 'post_build') list.push({ key: 'post_build', name: apt('postBuildTitle') })
  return list.map((item) => ({ ...item, status: stateFor(item.key) }))
}
const detailLogs = computed(() => {
  const content = [currentDetail.value.buildLog || '', currentDetail.value.deployLog || ''].filter(Boolean).join('\n')
  const keyword = logKeyword.value.trim().toLowerCase()
  return keyword ? content.split('\n').filter((line) => line.toLowerCase().includes(keyword)).join('\n') : content
})
const logFileName = computed(() => `${releaseTitle(currentDetail.value)}.log`.replace(/[\\/:*?"<>|]/g, '_'))

function stopDetailPolling() {
  if (detailPollTimer) window.clearInterval(detailPollTimer)
  detailPollTimer = null
}
async function scrollLogToLatest() {
  await nextTick()
  if (!logKeyword.value && logBodyRef.value) logBodyRef.value.scrollTop = logBodyRef.value.scrollHeight
}
async function refreshDetail() {
  if (!currentDetail.value?.id || refreshingDetail) return
  refreshingDetail = true
  try {
    currentDetail.value = await opsAppReleaseInfo(currentDetail.value.id)
    await scrollLogToLatest()
    if (currentDetail.value.status !== 'running') {
      stopDetailPolling()
      await loadData()
    }
  } finally { refreshingDetail = false }
}
function startDetailPolling() {
  stopDetailPolling()
  if (currentDetail.value.status === 'running') detailPollTimer = window.setInterval(refreshDetail, 2000)
}
async function openDetail(row) {
  detailVisible.value = true
  detailLoading.value = true
  logKeyword.value = ''
  try {
    currentDetail.value = await opsAppReleaseInfo(row.id)
    await scrollLogToLatest()
    startDetailPolling()
  } finally { detailLoading.value = false }
}
async function retry(row) {
  await ElMessageBox.confirm(apt('retryConfirm', { title: releaseTitle(row) }), apt('retryConfirmTitle'), { type: 'warning', confirmButtonText: apt('retryNow') })
  await retryOpsAppRelease(row.id)
  ElMessage.success(apt('retryTaskCreated'))
  await loadData()
}
async function copyLog() { await navigator.clipboard.writeText(detailLogs.value || ''); ElMessage.success(apt('logCopied')) }
function downloadLog() { const blob = new Blob([detailLogs.value || ''], { type: 'text/plain;charset=utf-8' }); const url = URL.createObjectURL(blob); const link = document.createElement('a'); link.href = url; link.download = logFileName.value; link.click(); URL.revokeObjectURL(url) }
watch(detailVisible, (visible) => { if (!visible) stopDetailPolling() })
onBeforeUnmount(stopDetailPolling)
onMounted(async () => { if (route.query.appId) query.appId = Number(route.query.appId); if (route.query.env) query.env = String(route.query.env); if (route.query.keyword) query.keyword = String(route.query.keyword); await Promise.all([loadApps(), loadData()]) })
</script>

<template>
  <div class="history-page">
    <div class="app-header"><div><h1>Build History</h1><p>{{ apt('historyHeroDesc') }}</p></div><el-button @click="loadData">{{ apt('refresh') }}</el-button></div>
    <section class="filter-panel">
      <div class="filter-main">
        <el-select v-model="query.appId" clearable filterable :placeholder="apt('allApplicationsPlaceholder')"><el-option v-for="item in appOptions" :key="item.id" :label="item.name" :value="item.id" /></el-select>
        <el-select v-model="query.env" clearable :placeholder="apt('allEnvironmentsPlaceholder')"><el-option label="dev" value="dev" /><el-option label="test" value="test" /><el-option label="prod" value="prod" /></el-select>
        <el-date-picker v-model="dateRange" type="datetimerange" value-format="YYYY-MM-DD HH:mm:ss" range-separator="~" :start-placeholder="apt('startTimeCol')" :end-placeholder="apt('endTimeCol')" />
        <el-input v-model="query.keyword" clearable :placeholder="apt('historySearchPlaceholder')" @keyup.enter="search" />
        <el-button type="primary" @click="search">{{ apt('search') }}</el-button><el-button @click="reset">{{ apt('reset') }}</el-button>
      </div>
      <div class="quick-filters"><span>{{ apt('quickFilters') }}</span><el-button :type="query.status === 'running' ? 'warning' : 'default'" plain @click="toggleStatus('running')">{{ apt('statusRunning') }}</el-button><el-button :type="query.status === 'failed' ? 'danger' : 'default'" plain @click="toggleStatus('failed')">{{ apt('showFailuresOnly') }}</el-button><el-button :type="query.status === 'success' ? 'success' : 'default'" plain @click="toggleStatus('success')">{{ apt('showSuccessOnly') }}</el-button></div>
    </section>
    <section v-loading="loading" class="history-list">
      <el-empty v-if="!rows.length" :description="apt('emptyHistory')" />
      <el-table v-else :data="rows" class="history-table">
        <el-table-column :label="apt('status')" width="104"><template #default="{ row }"><el-tag :type="statusType(row.status)" effect="light">{{ statusText(row.status) }}</el-tag></template></el-table-column>
        <el-table-column label="Task / Application" min-width="220"><template #default="{ row }"><div class="task-cell"><strong>{{ releaseTitle(row) }}</strong><span>{{ row.appName || '-' }} · {{ row.env || '-' }}</span></div></template></el-table-column>
        <el-table-column label="Code Version" min-width="170"><template #default="{ row }"><div class="code-cell"><span>{{ row.repoType === 'svn' ? 'SVN' : 'Git' }} · {{ versionText(row) }}</span><code>{{ row.commitId || apt('commitUnknown') }}</code></div></template></el-table-column>
        <el-table-column :label="apt('currentStageLabel')" min-width="220"><template #default="{ row }"><div class="stage-cell"><strong>{{ stageText(row.stage) }}</strong><span>{{ row.summary || '-' }}</span></div></template></el-table-column>
        <el-table-column :label="apt('startTimeCol')" prop="createTime" width="190" /><el-table-column :label="apt('durationCol')" width="100"><template #default="{ row }">{{ durationText(row.durationMs) }}</template></el-table-column>
        <el-table-column :label="apt('actions')" width="156" fixed="right"><template #default="{ row }"><el-button link type="primary" @click="openDetail(row)">{{ apt('detailView') }}</el-button><el-button v-if="row.status === 'failed' && row.buildTaskId" link type="danger" @click="retry(row)">{{ apt('retry') }}</el-button></template></el-table-column>
      </el-table>
      <div class="pager"><el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :page-sizes="[10, 20, 50]" layout="total, sizes, prev, pager, next" :total="total" @current-change="loadData" @size-change="search" /></div>
    </section>
    <el-drawer v-model="detailVisible" :title="releaseTitle(currentDetail)" size="min(760px, 100vw)" class="build-detail-drawer">
      <div v-loading="detailLoading" class="detail-content"><template v-if="currentDetail.id">
        <div class="detail-top"><el-tag :type="statusType(currentDetail.status)">{{ statusText(currentDetail.status) }}</el-tag><span>{{ currentDetail.summary || '-' }}</span><strong>{{ apt('durationPrefix', { duration: durationText(currentDetail.durationMs) }) }}</strong></div>
        <div class="detail-meta"><span>Application<b>{{ currentDetail.appName || '-' }}</b></span><span>Environment<b>{{ currentDetail.env || '-' }}</b></span><span>Code Version<b>{{ currentDetail.repoType === 'svn' ? 'SVN ' : 'Git ' }}{{ versionText(currentDetail) }}</b></span><span>Commit Version<b>{{ currentDetail.commitId || '-' }}</b></span><span>{{ apt('executionPathLabel') }}<b>{{ currentDetail.workspace || '-' }}</b></span><span>{{ apt('startTimeCol') }}<b>{{ currentDetail.createTime || '-' }}</b></span></div>
        <section class="detail-section"><div class="section-head"><strong>Build Stage</strong><span>{{ apt('currentStageNow', { stage: stageText(currentDetail.stage) }) }}</span></div><div class="stage-track"><div v-for="stage in stages(currentDetail)" :key="stage.key" class="stage-node" :class="stage.status"><i></i><strong>{{ stage.name }}</strong><span>{{ statusText(stage.status) }}</span></div></div></section>
        <section class="detail-section"><div class="section-head"><strong>{{ apt('allLogsLabel') }}</strong><div><el-input v-model="logKeyword" clearable size="small" :placeholder="apt('logSearchPlaceholder')" /><el-button link type="primary" @click="copyLog">{{ apt('copyButton') }}</el-button><el-button link type="primary" @click="downloadLog">Download</el-button></div></div><pre ref="logBodyRef" class="log-body">{{ detailLogs || apt('noLogOutput') }}</pre></section>
        <div class="detail-actions"><el-button v-if="currentDetail.status === 'failed' && currentDetail.buildTaskId" type="danger" plain @click="retry(currentDetail)">{{ apt('retryOriginalConfig') }}</el-button></div>
      </template></div>
    </el-drawer>
  </div>
</template>

<style scoped>
.history-page { padding: 24px; }.app-header, .filter-panel, .history-list { border: 1px solid #e1eaf7; border-radius: 12px; background: #fff; }.app-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; padding: 24px; background: linear-gradient(120deg, #fff, #f4f8ff); }.app-header h1 { margin: 0; color: #10213d; font-size: 28px; }.app-header p { margin: 8px 0 0; color: #70819e; }.filter-panel { margin-bottom: 16px; padding: 16px 20px; }.filter-main, .quick-filters { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }.filter-main :deep(.el-select) { width: 180px; }.filter-main :deep(.el-input) { width: 240px; }.quick-filters { margin-top: 12px; color: #74839c; font-size: 13px; }.quick-filters .el-button { margin-left: 0; }.history-list { padding: 12px 16px 18px; }.history-table { width: 100%; }.task-cell, .code-cell, .stage-cell { display: grid; gap: 4px; }.task-cell strong, .stage-cell strong { color: #172a47; }.task-cell span, .stage-cell span, .code-cell code { overflow: hidden; color: #71819c; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }.code-cell code { color: #496484; }.pager { display: flex; justify-content: flex-end; padding-top: 16px; }.detail-content { min-height: 100%; }.detail-top { display: flex; align-items: center; gap: 10px; padding-bottom: 18px; border-bottom: 1px solid #e8eef7; color: #647691; }.detail-top strong { margin-left: auto; color: #1e3352; }.detail-meta { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; padding: 18px 0; }.detail-meta span { display: grid; gap: 4px; color: #74839c; font-size: 12px; }.detail-meta b { overflow-wrap: anywhere; color: #263b5b; font-size: 13px; font-weight: 500; }.detail-section { margin-top: 18px; }.section-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; color: #6e809b; }.section-head strong { color: #1a2f4f; }.section-head > div { display: flex; align-items: center; gap: 6px; }.section-head :deep(.el-input) { width: 160px; }.stage-track { display: flex; gap: 8px; margin-top: 12px; }.stage-node { flex: 1; min-width: 0; padding: 12px; border: 1px solid #e2eaf5; border-radius: 8px; background: #fafcff; }.stage-node i { display: inline-block; width: 8px; height: 8px; margin-right: 6px; border-radius: 50%; background: #b5c2d5; }.stage-node strong, .stage-node span { display: block; }.stage-node span { margin-top: 5px; color: #8290a6; font-size: 12px; }.stage-node.success i { background: #42b883; }.stage-node.running i { background: #e6a23c; }.stage-node.failed { border-color: #fecaca; background: #fff8f8; }.stage-node.failed i { background: #f56c6c; }.log-body { min-height: 320px; max-height: calc(100vh - 540px); margin: 12px 0 0; padding: 16px; overflow: auto; border-radius: 8px; background: #101b30; color: #d9e6f5; font-family: Consolas, Monaco, monospace; font-size: 12px; line-height: 1.65; white-space: pre-wrap; }.detail-actions { margin-top: 20px; }@media (max-width: 900px) { .history-page { padding: 14px; }.filter-main :deep(.el-select), .filter-main :deep(.el-input), .filter-main :deep(.el-date-editor) { width: 100%; }.detail-meta { grid-template-columns: 1fr; }.stage-track { flex-direction: column; } }
</style>
