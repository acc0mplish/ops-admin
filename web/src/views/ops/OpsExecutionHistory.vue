<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { queryOpsExecHistory, queryOpsExecHistoryDetail, retryOpsExecTask } from '../../api/ops'

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
    command: '명령 실행',
    script: 'Script 실행',
    file: '파일 배포'
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
  await ElMessageBox.confirm(`“${row.title}”의 실패 또는 Timeout Host만 다시 실행하시겠습니까?`, '실패 대상 재시도', { type: 'warning' })
  const data = await retryOpsExecTask(row.id)
  ElMessage.success('재시도 Task를 생성했습니다')
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
        <h2 class="page-title">실행 History</h2>
        <p class="page-desc">모든 명령 실행, Script 실행, 파일 배포 Task의 실행 결과를 기록합니다.</p>
      </div>
    </div>

    <div class="toolbar">
      <div class="toolbar-left">
        <el-input v-model="query.keyword" clearable placeholder="Task 이름 / Script / 파일 / 명령 검색" style="width: 320px" @keyup.enter="loadData" />
        <el-select v-model="query.taskType" clearable placeholder="Task 유형" style="width: 140px">
          <el-option label="명령 실행" value="command" />
          <el-option label="Script 실행" value="script" />
          <el-option label="파일 배포" value="file" />
        </el-select>
        <el-select v-model="query.status" clearable placeholder="실행 상태" style="width: 140px">
          <el-option label="성공" value="success" />
          <el-option label="부분 성공" value="partial" />
          <el-option label="실패" value="failed" />
        </el-select>
        <el-button type="primary" @click="loadData">검색</el-button>
        <el-button @click="resetQuery">초기화</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="tableData" border>
      <el-table-column prop="title" label="Task 이름" min-width="220" />
      <el-table-column label="유형" width="110">
        <template #default="{ row }">{{ taskTypeLabel(row.taskType) }}</template>
      </el-table-column>
      <el-table-column prop="scriptName" label="Script" min-width="160" />
      <el-table-column prop="fileName" label="파일" min-width="160" />
      <el-table-column prop="hostCount" label="Target Host" width="100" />
      <el-table-column prop="successCount" label="성공" width="80" />
      <el-table-column prop="failedCount" label="실패" width="80" />
      <el-table-column label="상태" width="100">
        <template #default="{ row }">
          <el-tag :type="statusTagType(row.status)" effect="light">{{ row.status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="summary" label="요약" min-width="180" />
      <el-table-column prop="operator" label="작업자" width="120"><template #default="{ row }">{{ row.operator || 'system' }}</template></el-table-column>
      <el-table-column label="위험" width="90"><template #default="{ row }"><el-tag :type="row.riskLevel === 'high' ? 'danger' : 'info'" effect="plain">{{ row.riskLevel === 'high' ? '고위험' : '일반' }}</el-tag></template></el-table-column>
      <el-table-column prop="createTime" label="생성 시각" min-width="180" />
      <el-table-column label="작업" width="190" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDetail(row)">상세</el-button>
          <el-button v-if="row.status !== 'running' && row.failedCount > 0 && row.taskType !== 'file'" link type="warning" @click="retryFailed(row)">실패 재시도</el-button>
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

    <el-drawer v-model="detailVisible" size="72%" title="실행 상세">
      <div v-loading="detailLoading" class="detail-wrap">
        <div class="detail-actions">
          <el-button :disabled="!detailTask" @click="downloadExecutionResult">전체 결과 Download</el-button>
        </div>
        <div v-if="detailTask" class="detail-summary">
          <div class="summary-item"><span>Task 이름</span><strong>{{ detailTask.title }}</strong></div>
          <div class="summary-item"><span>Task 유형</span><strong>{{ taskTypeLabel(detailTask.taskType) }}</strong></div>
          <div class="summary-item"><span>실행 상태</span><strong>{{ detailTask.status }}</strong></div>
          <div class="summary-item"><span>실행 요약</span><strong>{{ detailTask.summary || '-' }}</strong></div>
          <div class="summary-item"><span>Concurrency</span><strong>{{ detailTask.concurrency }}</strong></div>
          <div class="summary-item"><span>생성 시각</span><strong>{{ detailTask.createTime }}</strong></div>
          <div class="summary-item"><span>작업자</span><strong>{{ detailTask.operator || 'system' }}</strong></div>
          <div class="summary-item"><span>Script Version</span><strong>{{ detailTask.scriptVersion ? `v${detailTask.scriptVersion}` : '-' }}</strong></div>
        </div>

        <el-table :data="detailResults" border>
          <el-table-column prop="hostName" label="Host" min-width="180" />
          <el-table-column prop="groupName" label="Host Group" min-width="140" />
          <el-table-column prop="sshIp" label="SSH IP" min-width="140" />
          <el-table-column prop="status" label="상태" width="100" />
          <el-table-column prop="exitCode" label="Exit Code" width="80" />
          <el-table-column prop="durationMs" label="소요 시간(ms)" width="110" />
          <el-table-column prop="stdout" label="표준 출력" min-width="260" show-overflow-tooltip />
          <el-table-column prop="stderr" label="오류 출력" min-width="220" show-overflow-tooltip />
          <el-table-column prop="errorText" label="오류 메시지" min-width="220" show-overflow-tooltip />
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
