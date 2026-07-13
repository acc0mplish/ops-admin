<script setup>
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { opsAppReleaseInfo, queryOpsApplicationOptions, queryOpsAppReleaseList } from '../../api/ops'

const route = useRoute()
const loading = ref(false)
const rows = ref([])
const total = ref(0)
const appOptions = ref([])
const logVisible = ref(false)
const autoScroll = ref(true)
const currentLog = ref({ title: '', content: '' })
const logBodyRef = ref()

const query = reactive({ pageNum: 1, pageSize: 10, appId: undefined, env: '', keyword: '', status: '' })

async function loadApps() {
  appOptions.value = await queryOpsApplicationOptions()
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryOpsAppReleaseList(query)
    rows.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

function statusType(status) {
  if (status === 'success') return 'success'
  if (status === 'running') return 'warning'
  return 'danger'
}

function statusText(status) {
  return { success: '成功', running: '执行中', failed: '失败', waiting: '等待' }[status] || status || '-'
}

function durationText(ms) {
  if (!ms) return '-'
  const seconds = Math.round(ms / 1000)
  if (seconds < 60) return `${seconds}秒`
  return `${Math.floor(seconds / 60)}分${seconds % 60}秒`
}

function releaseTitle(row) {
  return `${row.buildTaskName || row.appName || '构建任务'} #${row.id}`
}

function stageList(row) {
  const failed = row.status === 'failed'
  const stages = [
    { name: 'Git Clone', status: failed && row.stage === 'checkout' ? 'failed' : row.status === 'running' && row.stage === 'checkout' ? 'running' : 'success', duration: '拉取代码' },
    { name: '应用构建', status: failed && row.stage === 'build' ? 'failed' : row.status === 'running' && row.stage === 'build' ? 'running' : row.stage === 'checkout' ? 'waiting' : 'success', duration: '执行构建脚本' }
  ]
  if (row.deployLog || row.stage === 'post_build') {
    stages.push({ name: '构建后操作', status: failed && row.stage === 'post_build' ? 'failed' : row.status === 'running' && row.stage === 'post_build' ? 'running' : 'success', duration: '执行构建后脚本' })
  }
  return stages
}

function buildParamsText(row) {
  if (!row.paramsJson) return '-'
  try {
    const params = JSON.parse(row.paramsJson)
    return Object.entries(params).map(([key, value]) => `${key}=${value}`).join('，') || '-'
  } catch { return '-' }
}

const logFileName = computed(() => `${currentLog.value.title || 'build-log'}.log`.replace(/[\\/:*?"<>|]/g, '_'))

async function openLog(row) {
  const detail = await opsAppReleaseInfo(row.id)
  const content = [
    `构建任务：${detail.buildTaskName || detail.appName || '-'}`,
    `版本：${detail.version || '-'}`,
    `分支：${detail.branch || '-'}`,
    `Git Commit：${detail.commitId || '-'}`,
    '',
    detail.buildLog || '',
    detail.deployLog || ''
  ].join('\n')
  currentLog.value = { title: releaseTitle(detail), content }
  logVisible.value = true
  await nextTick()
  if (autoScroll.value && logBodyRef.value) logBodyRef.value.scrollTop = logBodyRef.value.scrollHeight
}

function downloadLog() {
  const blob = new Blob([currentLog.value.content], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = logFileName.value
  link.click()
  URL.revokeObjectURL(url)
}

onMounted(async () => {
  if (route.query.appId) query.appId = Number(route.query.appId)
  if (route.query.env) query.env = String(route.query.env)
  if (route.query.keyword) query.keyword = String(route.query.keyword)
  await loadApps()
  await loadData()
})
</script>

<template>
  <div class="history-page">
    <div class="app-header">
      <div>
        <h1>构建历史</h1>
        <p>记录每次 Git/SVN 拉取、应用构建和构建后操作，可追溯版本、提交与完整执行日志。</p>
      </div>
    </div>

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
            <el-option label="prod" value="prod" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="query.status" clearable placeholder="全部状态">
            <el-option label="成功" value="success" />
            <el-option label="执行中" value="running" />
            <el-option label="失败" value="failed" />
          </el-select>
        </el-form-item>
        <el-form-item label="任务名称">
          <el-input v-model="query.keyword" clearable placeholder="搜索任务 / 版本 / 摘要" @keyup.enter="loadData" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="loadData">搜索</el-button>
          <el-button @click="Object.assign(query, { appId: undefined, env: '', status: '', keyword: '', pageNum: 1 }); loadData()">重置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <div v-loading="loading" class="history-list">
      <el-empty v-if="!rows.length" description="暂无构建历史" />
      <div v-for="row in rows" :key="row.id" class="history-card">
        <div class="timeline-dot" :class="row.status"></div>
        <div class="history-main">
          <div class="history-title">
            <div>
              <strong>{{ releaseTitle(row) }}</strong>
              <el-tag size="small" :type="statusType(row.status)">{{ statusText(row.status) }}</el-tag>
              <span class="branch">{{ row.branch || '-' }}</span>
            </div>
            <span>总耗时：{{ durationText(row.durationMs) }}</span>
          </div>
          <div class="meta-grid">
            <span>构建时间：{{ row.createTime || '-' }}</span>
            <span>构建版本：{{ row.version || '-' }}</span>
            <span>构建环境：{{ row.env || '-' }}</span>
            <span>Git Commit：{{ row.commitId || '-' }}</span>
            <span>所属应用：{{ row.appName || '-' }}</span>
            <span>构建摘要：{{ row.summary || '-' }}</span>
            <span>执行路径：{{ row.workspace || '-' }}</span>
            <span class="params-meta">构建参数：{{ buildParamsText(row) }}</span>
          </div>
          <div class="stage-panel">
            <div class="stage-head">
              <span>构建阶段</span>
              <span>状态：{{ statusText(row.status) }}</span>
            </div>
            <div class="stage-list">
              <div v-for="stage in stageList(row)" :key="stage.name" class="stage-item" :class="stage.status">
                <span class="stage-icon"></span>
                <strong>{{ stage.name }}</strong>
                <el-tag size="small" :type="stage.status === 'success' ? 'success' : stage.status === 'running' ? 'warning' : stage.status === 'failed' ? 'danger' : 'info'">
                  {{ statusText(stage.status) }}
                </el-tag>
                <span>{{ stage.duration }}</span>
              </div>
            </div>
          </div>
          <div class="history-actions">
            <el-button type="primary" @click="openLog(row)">查看日志</el-button>
          </div>
        </div>
      </div>
      <div class="pager">
        <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" layout="total, prev, pager, next" :total="total" @current-change="loadData" />
      </div>
    </div>

    <el-dialog v-model="logVisible" title="构建日志" width="960px" class="log-dialog">
      <div class="log-shell">
        <div class="log-head">
          <span>{{ currentLog.title }}</span>
          <el-button link type="primary" @click="downloadLog">下载日志</el-button>
        </div>
        <pre ref="logBodyRef" class="log-body">{{ currentLog.content }}</pre>
      </div>
      <template #footer>
        <el-checkbox v-model="autoScroll">自动滚动</el-checkbox>
        <el-button type="primary" @click="downloadLog">下载日志</el-button>
        <el-button type="danger" @click="logVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.history-page { padding: 24px; }
.app-header, .filter-panel, .history-list { background: #fff; border: 1px solid #e5edf8; border-radius: 12px; }
.app-header { padding: 24px; margin-bottom: 16px; }
.app-header h1 { margin: 0; font-size: 28px; color: #071b3d; }
.app-header p { margin: 8px 0 0; color: #6b7c9b; }
.filter-panel { padding: 18px 24px 0; margin-bottom: 16px; }
:deep(.filter-panel .el-select) { width: 220px; }
:deep(.filter-panel .el-input) { width: 280px; }
.history-list { padding: 20px 24px; }
.history-card { display: grid; grid-template-columns: 18px 1fr; gap: 18px; padding: 18px 0; border-bottom: 1px solid #edf2f9; }
.timeline-dot { width: 14px; height: 14px; border-radius: 50%; margin-top: 6px; background: #b8c5d9; }
.timeline-dot.success { background: #67c23a; }
.timeline-dot.failed { background: #ff6b6b; }
.timeline-dot.running { background: #e6a23c; }
.history-title { display: flex; justify-content: space-between; align-items: center; margin-bottom: 14px; color: #6b7c9b; }
.history-title div { display: flex; align-items: center; gap: 10px; }
.history-title strong { font-size: 18px; color: #071b3d; }
.branch { color: #62728c; }
.meta-grid { display: grid; grid-template-columns: repeat(2, minmax(220px, 1fr)); gap: 12px 28px; padding: 18px; border-radius: 10px; background: #fbfdff; color: #6b7c9b; }
.params-meta { grid-column: 1 / -1; overflow-wrap: anywhere; }
.stage-panel { margin-top: 16px; border-radius: 10px; background: #fbfdff; padding: 16px; }
.stage-head { display: flex; justify-content: space-between; margin-bottom: 12px; color: #6b7c9b; }
.stage-list { display: flex; gap: 14px; flex-wrap: wrap; }
.stage-item { display: flex; align-items: center; gap: 8px; min-width: 220px; padding: 12px; border-radius: 8px; background: #fff; border: 1px solid #e7eef9; }
.stage-icon { width: 10px; height: 10px; border-radius: 50%; background: #b8c5d9; }
.stage-item.success .stage-icon { background: #67c23a; }
.stage-item.failed .stage-icon { background: #ff6b6b; }
.stage-item.running .stage-icon { background: #e6a23c; }
.history-actions { margin-top: 16px; }
.pager { display: flex; justify-content: flex-end; padding-top: 18px; }
.log-shell { overflow: hidden; border-radius: 8px; background: #1f1f1f; color: #eee; }
.log-head { display: flex; justify-content: space-between; padding: 12px 16px; background: #2d2d2d; }
.log-body { min-height: 420px; max-height: 560px; margin: 0; padding: 16px; overflow: auto; font-family: Consolas, Monaco, monospace; font-size: 13px; line-height: 1.6; white-space: pre-wrap; }
</style>
