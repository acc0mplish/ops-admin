<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { opsScheduleLogInfo, queryOpsScheduleLogList } from '../../api/ops'

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
  return value === 'http' ? 'HTTP 프로브' : 'Script Task'
}

function triggerLabel(value) {
  return value === 'manual' ? '수동 트리거' : '스케줄 실행'
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
    ElMessage.error('복사에 실패했습니다.')
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
        <p class="page-desc">정기 Task의 각 스케줄 실행 결과, 요약, 상세 출력을 확인합니다.</p>
      </div>
    </div>

    <div class="toolbar">
      <div class="toolbar-left">
        <el-input v-model="query.keyword" clearable placeholder="Task 이름 / 요약 검색" style="width: 280px" @keyup.enter="loadData" />
        <el-select v-model="query.taskType" clearable placeholder="Task 유형" style="width: 140px">
          <el-option label="Script Task" value="script" />
          <el-option label="HTTP 프로브" value="http" />
        </el-select>
        <el-select v-model="query.status" clearable placeholder="실행 상태" style="width: 120px">
          <el-option label="성공" value="success" />
          <el-option label="실패" value="failed" />
          <el-option label="부분 성공" value="partial" />
          <el-option label="실행 중" value="running" />
        </el-select>
        <el-button type="primary" @click="loadData">검색</el-button>
        <el-button @click="resetQuery">초기화</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="taskName" label="Task 이름" min-width="180" />
      <el-table-column label="Task 유형" width="120">
        <template #default="{ row }">{{ taskTypeLabel(row.taskType) }}</template>
      </el-table-column>
      <el-table-column label="트리거 방식" width="110">
        <template #default="{ row }">{{ triggerLabel(row.triggerType) }}</template>
      </el-table-column>
      <el-table-column label="실행 상태" width="110" align="center">
        <template #default="{ row }">
          <el-tag :type="statusTagType(row.status)" effect="light">{{ row.status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="summary" label="실행 요약" min-width="260" show-overflow-tooltip />
      <el-table-column prop="durationMs" label="소요 시간(ms)" width="100" />
      <el-table-column prop="startedAt" label="시작 시각" width="180" />
      <el-table-column prop="finishedAt" label="종료 시각" width="180" />
      <el-table-column label="작업" width="100" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDetail(row)">상세</el-button>
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

    <el-dialog v-model="detailVisible" title="Task Log 상세" width="960px">
      <div v-loading="detailLoading" class="log-detail">
        <template v-if="detail">
          <div class="detail-grid">
            <div><span>Task 이름</span><strong>{{ detail.taskName }}</strong></div>
            <div><span>Task 유형</span><strong>{{ taskTypeLabel(detail.taskType) }}</strong></div>
            <div><span>실행 상태</span><strong>{{ detail.status }}</strong></div>
            <div><span>트리거 방식</span><strong>{{ triggerLabel(detail.triggerType) }}</strong></div>
            <div><span>시작 시각</span><strong>{{ detail.startedAt || '-' }}</strong></div>
            <div><span>종료 시각</span><strong>{{ detail.finishedAt || '-' }}</strong></div>
            <div><span>소요 시간</span><strong>{{ detail.durationMs }} ms</strong></div>
            <div><span>관련 실행 Task</span><strong>{{ detail.execTaskId || '-' }}</strong></div>
            <div><span>예상 Status Code</span><strong>{{ detail.expectedStatus || '-' }}</strong></div>
            <div><span>실제 Status Code</span><strong>{{ detail.actualStatus || '-' }}</strong></div>
          </div>

          <el-alert :title="detail.summary || '요약 없음'" type="info" :closable="false" />

          <div class="detail-actions">
            <el-button v-if="detail.execTaskId" type="primary" link @click="openExecHistory(detail.execTaskId)">Quick Execution 상세로 이동</el-button>
            <el-button link @click="copyText(detail.detail || '', '상세 출력을 복사했습니다.')">상세 출력 복사</el-button>
            <el-button link @click="copyText(detail.responseBody || '', 'HTTP 응답을 복사했습니다.')">HTTP 응답 복사</el-button>
          </div>

          <div class="detail-section">
            <h4>상세 출력</h4>
            <pre>{{ detail.detail || '-' }}</pre>
          </div>
          <div class="detail-section">
            <h4>HTTP 응답</h4>
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
