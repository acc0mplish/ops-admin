<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { queryOpsExecHistory, queryOpsExecHistoryDetail, retryOpsExecTask } from '../../api/ops'
import { ot } from '../../utils/ops-i18n'

const route = useRoute()
const loading = ref(false)
const detailVisible = ref(false)
const detailLoading = ref(false)
const tableData = ref([])
const total = ref(0)
const detailTask = ref(null)
const detailResults = ref([])

const query = reactive({
  pageNum: 1,
  pageSize: 10,
  keyword: '',
  taskType: '',
  status: ''
})

async function loadData() {
  loading.value = true
  try {
    const data = await queryOpsExecHistory(query)
    tableData.value = data.list || []
    total.value = data.total || 0
    await openDetailFromQuery()
  } finally {
    loading.value = false
  }
}

function resetQuery() {
  Object.assign(query, {
    pageNum: 1,
    pageSize: 10,
    keyword: '',
    taskType: '',
    status: ''
  })
  loadData()
}

async function openDetail(row) {
  detailVisible.value = true
  detailLoading.value = true
  try {
    const data = await queryOpsExecHistoryDetail(row.id)
    detailTask.value = data.task || null
    detailResults.value = data.results || []
  } finally {
    detailLoading.value = false
  }
}

async function openDetailFromQuery() {
  const taskId = Number(route.query.taskId || 0)
  if (!taskId || detailVisible.value) return
  const hit = tableData.value.find((item) => Number(item.id) === taskId)
  if (hit) {
    await openDetail(hit)
  }
}

function taskTypeLabel(value) {
  const map = {
    command: ot('taskTypeCommand'),
    script: ot('scriptExecution'),
    file: ot('nodeFileDeploy')
  }
  return map[value] || value || '-'
}

function statusTagType(value) {
  if (value === 'success') return 'success'
  if (value === 'partial') return 'warning'
  if (value === 'running') return 'warning'
  return 'danger'
}

async function retryFailed(row) {
  await ElMessageBox.confirm(ot('retryFailedConfirm', { title: row.title }), ot('retryFailedTitle'), { type: 'warning' })
  const data = await retryOpsExecTask(row.id)
  ElMessage.success(ot('retryCreated'))
  await loadData()
  if (data?.task) await openDetail(data.task)
}

function downloadExecutionResult() {
  if (!detailTask.value) return
  const payload = {
    task: detailTask.value,
    results: detailResults.value
  }
  const blob = new Blob([JSON.stringify(payload, null, 2)], { type: 'application/json;charset=utf-8' })
  const link = document.createElement('a')
  link.href = URL.createObjectURL(blob)
  link.download = `ops-execution-${detailTask.value.id || 'result'}.json`
  link.click()
  URL.revokeObjectURL(link.href)
}

onMounted(loadData)
</script>

<template>
  <div class="page-card ops-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">{{ ot('executionHistory') }}</h2>
        <p class="page-desc">{{ ot('executionHistoryDesc') }}</p>
      </div>
    </div>

    <div class="toolbar">
      <div class="toolbar-left">
        <el-input v-model="query.keyword" clearable :placeholder="ot('searchExecHistory')" style="width: 320px" @keyup.enter="loadData" />
        <el-select v-model="query.taskType" clearable :placeholder="ot('taskTypeLabel')" style="width: 140px">
          <el-option :label="ot('taskTypeCommand')" value="command" />
          <el-option :label="ot('scriptExecution')" value="script" />
          <el-option :label="ot('nodeFileDeploy')" value="file" />
        </el-select>
        <el-select v-model="query.status" clearable :placeholder="ot('executionStatus')" style="width: 140px">
          <el-option :label="ot('success')" value="success" />
          <el-option :label="ot('partialSuccess')" value="partial" />
          <el-option :label="ot('failure')" value="failed" />
        </el-select>
        <el-button type="primary" @click="loadData">{{ ot('search') }}</el-button>
        <el-button @click="resetQuery">{{ ot('reset') }}</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="tableData" border>
      <el-table-column prop="title" :label="ot('taskName')" min-width="220" />
      <el-table-column :label="ot('type')" width="110">
        <template #default="{ row }">{{ taskTypeLabel(row.taskType) }}</template>
      </el-table-column>
      <el-table-column prop="scriptName" label="Script" min-width="160" />
      <el-table-column prop="fileName" :label="ot('fileColumn')" min-width="160" />
      <el-table-column prop="hostCount" label="Target Host" width="100" />
      <el-table-column prop="successCount" :label="ot('success')" width="80" />
      <el-table-column prop="failedCount" :label="ot('failure')" width="80" />
      <el-table-column :label="ot('status')" width="100">
        <template #default="{ row }">
          <el-tag :type="statusTagType(row.status)" effect="light">{{ row.status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="summary" :label="ot('summaryLabel')" min-width="180" />
      <el-table-column prop="operator" :label="ot('operator')" width="120"><template #default="{ row }">{{ row.operator || 'system' }}</template></el-table-column>
      <el-table-column :label="ot('riskColumn')" width="90"><template #default="{ row }"><el-tag :type="row.riskLevel === 'high' ? 'danger' : 'info'" effect="plain">{{ row.riskLevel === 'high' ? ot('highRisk') : ot('normal') }}</el-tag></template></el-table-column>
      <el-table-column prop="createTime" :label="ot('createdAt')" min-width="180" />
      <el-table-column :label="ot('actions')" width="190" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDetail(row)">{{ ot('detail') }}</el-button>
          <el-button v-if="row.status !== 'running' && row.failedCount > 0 && row.taskType !== 'file'" link type="warning" @click="retryFailed(row)">{{ ot('retryFailedAction') }}</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination
        v-model:current-page="query.pageNum"
        v-model:page-size="query.pageSize"
        :total="total"
        layout="total, sizes, prev, pager, next"
        @current-change="loadData"
        @size-change="loadData"
      />
    </div>

    <el-drawer v-model="detailVisible" size="72%" :title="ot('executionDetail')">
      <div v-loading="detailLoading" class="detail-wrap">
        <div class="detail-actions">
          <el-button :disabled="!detailTask" @click="downloadExecutionResult">{{ ot('downloadFullResult') }}</el-button>
        </div>
        <div v-if="detailTask" class="detail-summary">
          <div class="summary-item"><span>{{ ot('taskName') }}</span><strong>{{ detailTask.title }}</strong></div>
          <div class="summary-item"><span>{{ ot('taskTypeLabel') }}</span><strong>{{ taskTypeLabel(detailTask.taskType) }}</strong></div>
          <div class="summary-item"><span>{{ ot('executionStatus') }}</span><strong>{{ detailTask.status }}</strong></div>
          <div class="summary-item"><span>{{ ot('executionSummaryLabel') }}</span><strong>{{ detailTask.summary || '-' }}</strong></div>
          <div class="summary-item"><span>Concurrency</span><strong>{{ detailTask.concurrency }}</strong></div>
          <div class="summary-item"><span>{{ ot('createdAt') }}</span><strong>{{ detailTask.createTime }}</strong></div>
          <div class="summary-item"><span>{{ ot('operator') }}</span><strong>{{ detailTask.operator || 'system' }}</strong></div>
          <div class="summary-item"><span>Script Version</span><strong>{{ detailTask.scriptVersion ? `v${detailTask.scriptVersion}` : '-' }}</strong></div>
        </div>

        <el-table :data="detailResults" border>
          <el-table-column prop="hostName" label="Host" min-width="180" />
          <el-table-column prop="groupName" label="Host Group" min-width="140" />
          <el-table-column prop="sshIp" label="SSH IP" min-width="140" />
          <el-table-column prop="status" :label="ot('status')" width="100" />
          <el-table-column prop="exitCode" label="Exit Code" width="80" />
          <el-table-column prop="durationMs" :label="ot('durationMsLabel')" width="110" />
          <el-table-column prop="stdout" :label="ot('stdoutLabel')" min-width="260" show-overflow-tooltip />
          <el-table-column prop="stderr" :label="ot('stderrLabel')" min-width="220" show-overflow-tooltip />
          <el-table-column prop="errorText" :label="ot('errorMessage')" min-width="220" show-overflow-tooltip />
        </el-table>
      </div>
    </el-drawer>
  </div>
</template>

<style scoped>
.ops-page { display: flex; flex-direction: column; gap: 18px; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; }
.page-title { margin: 0 0 8px; font-size: 22px; font-weight: 700; color: #14213d; }
.page-desc { margin: 0; color: #7282a0; }
.toolbar-left { display: flex; flex-wrap: wrap; gap: 12px; }
.pager { display: flex; justify-content: flex-end; }
.detail-wrap { display: flex; flex-direction: column; gap: 16px; }
.detail-actions { display: flex; justify-content: flex-end; }
.detail-summary { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; }
.summary-item { padding: 14px 16px; border: 1px solid #e8edf5; border-radius: 8px; background: #f8fbff; }
.summary-item span { display: block; margin-bottom: 6px; color: #7282a0; font-size: 13px; }
.summary-item strong { color: #14213d; }
</style>
