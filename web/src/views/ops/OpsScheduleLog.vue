<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { opsScheduleLogInfo, queryOpsScheduleLogList } from '../../api/ops'
import { ot } from '../../utils/ops-i18n'

const router = useRouter()
const loading = ref(false)
const detailLoading = ref(false)
const detailVisible = ref(false)
const rows = ref([])
const total = ref(0)
const detail = ref(null)

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
    const data = await queryOpsScheduleLogList(query)
    rows.value = data.list || []
    total.value = data.total || 0
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
    detail.value = await opsScheduleLogInfo(row.id)
  } finally {
    detailLoading.value = false
  }
}

function taskTypeLabel(value) {
  return value === 'http' ? ot('httpProbe') : 'Script Task'
}

function triggerLabel(value) {
  return value === 'manual' ? ot('manualTrigger') : ot('scheduledRun')
}

function statusTagType(value) {
  if (value === 'success') return 'success'
  if (value === 'running') return 'warning'
  if (value === 'partial') return 'warning'
  return 'danger'
}

async function copyText(value, message) {
  try {
    await navigator.clipboard.writeText(value || '')
    ElMessage.success(message)
  } catch {
    ElMessage.error(ot('copyFailed'))
  }
}

function openExecHistory(execTaskId) {
  if (!execTaskId) return
  router.push({ path: '/ops/quick-exec/history', query: { taskId: String(execTaskId) } })
}

onMounted(loadData)
</script>

<template>
  <div class="page-card ops-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">Task Log</h2>
        <p class="page-desc">{{ ot('scheduleLogDesc') }}</p>
      </div>
    </div>

    <div class="toolbar">
      <div class="toolbar-left">
        <el-input v-model="query.keyword" clearable :placeholder="ot('searchScheduleLog')" style="width: 280px" @keyup.enter="loadData" />
        <el-select v-model="query.taskType" clearable :placeholder="ot('taskTypeLabel')" style="width: 140px">
          <el-option label="Script Task" value="script" />
          <el-option :label="ot('httpProbe')" value="http" />
        </el-select>
        <el-select v-model="query.status" clearable :placeholder="ot('executionStatus')" style="width: 120px">
          <el-option :label="ot('success')" value="success" />
          <el-option :label="ot('failure')" value="failed" />
          <el-option :label="ot('partialSuccess')" value="partial" />
          <el-option :label="ot('running')" value="running" />
        </el-select>
        <el-button type="primary" @click="loadData">{{ ot('search') }}</el-button>
        <el-button @click="resetQuery">{{ ot('reset') }}</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="taskName" :label="ot('taskName')" min-width="180" />
      <el-table-column :label="ot('taskTypeLabel')" width="120">
        <template #default="{ row }">{{ taskTypeLabel(row.taskType) }}</template>
      </el-table-column>
      <el-table-column :label="ot('triggerType')" width="110">
        <template #default="{ row }">{{ triggerLabel(row.triggerType) }}</template>
      </el-table-column>
      <el-table-column :label="ot('executionStatus')" width="110" align="center">
        <template #default="{ row }">
          <el-tag :type="statusTagType(row.status)" effect="light">{{ row.status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="summary" :label="ot('executionSummaryLabel')" min-width="260" show-overflow-tooltip />
      <el-table-column prop="durationMs" :label="ot('durationMsLabel')" width="100" />
      <el-table-column prop="startedAt" :label="ot('startTime')" width="180" />
      <el-table-column prop="finishedAt" :label="ot('endTime')" width="180" />
      <el-table-column :label="ot('actions')" width="100" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDetail(row)">{{ ot('detail') }}</el-button>
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

    <el-dialog v-model="detailVisible" :title="ot('taskLogDetail')" width="960px">
      <div v-loading="detailLoading" class="log-detail">
        <template v-if="detail">
          <div class="detail-grid">
            <div><span>{{ ot('taskName') }}</span><strong>{{ detail.taskName }}</strong></div>
            <div><span>{{ ot('taskTypeLabel') }}</span><strong>{{ taskTypeLabel(detail.taskType) }}</strong></div>
            <div><span>{{ ot('executionStatus') }}</span><strong>{{ detail.status }}</strong></div>
            <div><span>{{ ot('triggerType') }}</span><strong>{{ triggerLabel(detail.triggerType) }}</strong></div>
            <div><span>{{ ot('startTime') }}</span><strong>{{ detail.startedAt || '-' }}</strong></div>
            <div><span>{{ ot('endTime') }}</span><strong>{{ detail.finishedAt || '-' }}</strong></div>
            <div><span>{{ ot('durationWord') }}</span><strong>{{ detail.durationMs }} ms</strong></div>
            <div><span>{{ ot('relatedExecTask') }}</span><strong>{{ detail.execTaskId || '-' }}</strong></div>
            <div><span>{{ ot('expectedStatusCodeLabel') }}</span><strong>{{ detail.expectedStatus || '-' }}</strong></div>
            <div><span>{{ ot('actualStatusCode') }}</span><strong>{{ detail.actualStatus || '-' }}</strong></div>
          </div>

          <el-alert :title="detail.summary || ot('noSummary')" type="info" :closable="false" />

          <div class="detail-actions">
            <el-button v-if="detail.execTaskId" type="primary" link @click="openExecHistory(detail.execTaskId)">{{ ot('goToExecDetail') }}</el-button>
            <el-button link @click="copyText(detail.detail || '', ot('detailOutputCopied'))">{{ ot('detailOutputCopy') }}</el-button>
            <el-button link @click="copyText(detail.responseBody || '', ot('httpResponseCopied'))">{{ ot('httpResponseCopy') }}</el-button>
          </div>

          <div class="detail-section">
            <h4>{{ ot('detailOutput') }}</h4>
            <pre>{{ detail.detail || '-' }}</pre>
          </div>
          <div class="detail-section">
            <h4>{{ ot('httpResponse') }}</h4>
            <pre>{{ detail.responseBody || '-' }}</pre>
          </div>
        </template>
      </div>
    </el-dialog>
  </div>
</template>

<style scoped>
.ops-page { display: flex; flex-direction: column; gap: 18px; }
.page-title { margin: 0 0 8px; font-size: 22px; font-weight: 700; color: #14213d; }
.page-desc { margin: 0; color: #7282a0; }
.toolbar-left { display: flex; gap: 12px; flex-wrap: wrap; }
.pager { display: flex; justify-content: flex-end; }
.log-detail { min-height: 280px; display: flex; flex-direction: column; gap: 16px; }
.detail-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px 20px; }
.detail-grid span { display: block; margin-bottom: 4px; color: #7282a0; font-size: 13px; }
.detail-grid strong { color: #14213d; font-weight: 600; }
.detail-actions { display: flex; gap: 16px; flex-wrap: wrap; }
.detail-section h4 { margin: 0 0 8px; color: #14213d; }
.detail-section pre { margin: 0; padding: 14px 16px; border-radius: 10px; background: #111827; color: #e5e7eb; white-space: pre-wrap; word-break: break-word; font-family: 'JetBrains Mono', 'Consolas', monospace; font-size: 13px; line-height: 1.6; }
</style>
