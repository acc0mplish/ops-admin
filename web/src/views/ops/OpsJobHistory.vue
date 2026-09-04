<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { opsJobHistoryDetail, queryOpsJobHistoryList } from '../../api/ops'

const router = useRouter()
const loading = ref(false)
const rows = ref([])
const total = ref(0)
const detailVisible = ref(false)
const detailLoading = ref(false)
const detail = reactive({
  history: null,
  steps: []
})

const query = reactive({
  pageNum: 1,
  pageSize: 10,
  keyword: '',
  status: ''
})

async function loadData() {
  loading.value = true
  try {
    const data = await queryOpsJobHistoryList(query)
    rows.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

function resetQuery() {
  Object.assign(query, { pageNum: 1, pageSize: 10, keyword: '', status: '' })
  loadData()
}

async function openDetail(row) {
  detailVisible.value = true
  detailLoading.value = true
  try {
    const data = await opsJobHistoryDetail(row.id)
    detail.history = data.history
    detail.steps = data.steps || []
  } finally {
    detailLoading.value = false
  }
}

function openExecHistory(step) {
  if (!step.execTaskId) return
  router.push({ path: '/ops/quick-exec/history', query: { taskId: String(step.execTaskId) } })
}

function statusTagType(status) {
  switch (status) {
    case 'success':
      return 'success'
    case 'running':
      return 'warning'
    case 'waiting_approval':
      return 'warning'
    case 'rejected':
      return 'danger'
    default:
      return 'info'
  }
}

onMounted(loadData)
</script>

<template>
  <div class="page-card ops-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">Job History</h2>
        <p class="page-desc">Job의 각 실행별 전체 상태, Step 결과, 실행 출력을 확인합니다. Manual Approval은 별도 대기 페이지에서 처리하십시오.</p>
      </div>
    </div>

    <div class="toolbar">
      <div class="toolbar-left">
        <el-input v-model="query.keyword" clearable placeholder="Job 이름 / 요약 / 현재 Step 검색" style="width: 320px" @keyup.enter="loadData" />
        <el-select v-model="query.status" clearable placeholder="상태" style="width: 140px">
          <el-option label="실행 중" value="running" />
          <el-option label="성공" value="success" />
          <el-option label="실패" value="failed" />
          <el-option label="승인 대기" value="waiting_approval" />
          <el-option label="거부됨" value="rejected" />
        </el-select>
        <el-button type="primary" @click="loadData">검색</el-button>
        <el-button @click="resetQuery">초기화</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="jobName" label="Job 이름" min-width="220" />
      <el-table-column prop="triggerType" label="트리거 방식" width="100" />
      <el-table-column label="상태" width="120" align="center">
        <template #default="{ row }">
          <el-tag :type="statusTagType(row.status)" effect="light">{{ row.status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="summary" label="요약" min-width="260" show-overflow-tooltip />
      <el-table-column prop="currentStepName" label="현재 Step" width="180" show-overflow-tooltip />
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

    <el-drawer v-model="detailVisible" size="70%" title="Job 실행 상세">
      <div v-loading="detailLoading" class="history-detail">
        <div v-if="detail.history" class="history-summary">
          <div class="summary-item"><span>Job</span><strong>{{ detail.history.jobName }}</strong></div>
          <div class="summary-item"><span>상태</span><strong>{{ detail.history.status }}</strong></div>
          <div class="summary-item"><span>요약</span><strong>{{ detail.history.summary || '-' }}</strong></div>
        </div>

        <el-table :data="detail.steps" border>
          <el-table-column prop="stepName" label="Step 이름" min-width="180" />
          <el-table-column prop="stepType" label="Step 유형" width="120" />
          <el-table-column label="상태" width="140" align="center">
            <template #default="{ row }">
              <el-tag :type="statusTagType(row.status)" effect="light">{{ row.status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="summary" label="요약" min-width="220" show-overflow-tooltip />
          <el-table-column prop="durationMs" label="소요 시간(ms)" width="120" />
          <el-table-column prop="output" label="출력" min-width="280" show-overflow-tooltip />
          <el-table-column label="작업" width="120" fixed="right">
            <template #default="{ row }">
              <el-button v-if="row.execTaskId" link type="primary" @click="openExecHistory(row)">실행 상세</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </el-drawer>
  </div>
</template>

<style scoped>
.ops-page { display: flex; flex-direction: column; gap: 18px; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
.page-title { margin: 0 0 8px; font-size: 24px; font-weight: 700; color: #14213d; }
.page-desc { margin: 0; color: #7282a0; }
.toolbar-left { display: flex; gap: 12px; flex-wrap: wrap; }
.pager { display: flex; justify-content: flex-end; }
.history-detail { display: flex; flex-direction: column; gap: 16px; }
.history-summary { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; }
.summary-item { padding: 16px; border: 1px solid #e5ebff; border-radius: 12px; background: #f9fbff; }
.summary-item span { display: block; margin-bottom: 8px; color: #7485a7; font-size: 13px; }
.summary-item strong { color: #1b2a4b; font-size: 16px; }
</style>
