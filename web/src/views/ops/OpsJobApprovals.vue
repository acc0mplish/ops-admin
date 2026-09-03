<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  approveOpsJobHistory,
  opsJobHistoryDetail,
  queryOpsJobHistoryList,
  rejectOpsJobHistory
} from '../../api/ops'
import { ot } from '../../utils/ops-i18n'

const loading = ref(false)
const rows = ref([])
const total = ref(0)
const detailVisible = ref(false)
const detailLoading = ref(false)
const approvalNote = ref('')
const detail = reactive({
  history: null,
  steps: []
})

const query = reactive({
  pageNum: 1,
  pageSize: 10,
  keyword: '',
  status: 'waiting_approval'
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
  Object.assign(query, {
    pageNum: 1,
    pageSize: 10,
    keyword: '',
    status: 'waiting_approval'
  })
  loadData()
}

async function openDetail(row) {
  detailVisible.value = true
  detailLoading.value = true
  approvalNote.value = ''
  try {
    const data = await opsJobHistoryDetail(row.id)
    detail.history = data.history
    detail.steps = data.steps || []
  } finally {
    detailLoading.value = false
  }
}

function currentApprovalStep() {
  return (detail.steps || []).find((item) => item.status === 'waiting_approval') || null
}

async function handleApprove() {
  const step = currentApprovalStep()
  if (!step || !detail.history) return
  await approveOpsJobHistory({
    historyId: detail.history.id,
    stepId: step.stepId,
    note: approvalNote.value
  })
  ElMessage.success(ot('approvalPassed'))
  detailVisible.value = false
  await loadData()
}

async function handleReject() {
  const step = currentApprovalStep()
  if (!step || !detail.history) return
  await rejectOpsJobHistory({
    historyId: detail.history.id,
    stepId: step.stepId,
    note: approvalNote.value
  })
  ElMessage.success(ot('approvalRejected'))
  detailVisible.value = false
  await loadData()
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
        <h2 class="page-title">{{ ot('approvals') }}</h2>
        <p class="page-desc">{{ ot('approvalsDesc') }}</p>
      </div>
    </div>

    <div class="toolbar">
      <div class="toolbar-left">
        <el-input v-model="query.keyword" clearable :placeholder="ot('searchApproval')" style="width: 320px" @keyup.enter="loadData" />
        <el-button type="primary" @click="loadData">{{ ot('search') }}</el-button>
        <el-button @click="resetQuery">{{ ot('reset') }}</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="jobName" :label="ot('jobName')" min-width="220" />
      <el-table-column :label="ot('status')" width="120" align="center">
        <template #default="{ row }">
          <el-tag :type="statusTagType(row.status)" effect="light">{{ row.status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="summary" :label="ot('summary')" min-width="260" show-overflow-tooltip />
      <el-table-column prop="currentStepName" :label="ot('pendingApprovalStep')" width="220" show-overflow-tooltip />
      <el-table-column prop="startedAt" :label="ot('startedAt')" width="180" />
      <el-table-column :label="ot('actions')" width="100" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDetail(row)">{{ ot('process') }}</el-button>
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

    <el-drawer v-model="detailVisible" size="70%" :title="ot('approvalDrawer')">
      <div v-loading="detailLoading" class="approval-detail">
        <div v-if="detail.history" class="detail-summary">
          <div class="summary-card"><span>{{ ot('jobName') }}</span><strong>{{ detail.history.jobName }}</strong></div>
          <div class="summary-card"><span>{{ ot('currentStatus') }}</span><strong>{{ detail.history.status }}</strong></div>
          <div class="summary-card"><span>{{ ot('currentStep') }}</span><strong>{{ detail.history.currentStepName || '-' }}</strong></div>
        </div>

        <el-alert type="warning" :closable="false" :title="ot('approvalWarning')" />

        <el-input v-model="approvalNote" type="textarea" :rows="3" :placeholder="ot('approvalNotePlaceholder')" />

        <el-table :data="detail.steps" border>
          <el-table-column prop="stepName" :label="ot('stepName')" min-width="180" />
          <el-table-column prop="stepType" :label="ot('stepType')" width="120" />
          <el-table-column :label="ot('status')" width="140" align="center">
            <template #default="{ row }">
              <el-tag :type="statusTagType(row.status)" effect="light">{{ row.status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="summary" :label="ot('summary')" min-width="220" show-overflow-tooltip />
          <el-table-column prop="output" :label="ot('output')" min-width="320" show-overflow-tooltip />
        </el-table>

        <div class="detail-actions">
          <el-button type="danger" plain @click="handleReject">{{ ot('reject') }}</el-button>
          <el-button type="primary" @click="handleApprove">{{ ot('approve') }}</el-button>
        </div>
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
.approval-detail { display: flex; flex-direction: column; gap: 16px; }
.detail-summary { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; }
.summary-card { padding: 16px; border: 1px solid #e5ebff; border-radius: 12px; background: #f9fbff; }
.summary-card span { display: block; margin-bottom: 8px; color: #7485a7; font-size: 13px; }
.summary-card strong { color: #1b2a4b; font-size: 16px; }
.detail-actions { display: flex; justify-content: flex-end; gap: 12px; }
</style>
