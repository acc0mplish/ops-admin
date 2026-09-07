<template>
  <div class="infra-tasks">
    <div class="page-header">
      <div><h2 class="page-title">{{ inft('tasksTitle') }}</h2><p class="page-desc">{{ inft('tasksDesc') }}</p></div>
      <el-button @click="loadData">{{ inft('refresh') }}</el-button>
    </div>

    <el-card shadow="never">
      <div class="toolbar">
        <span class="toolbar-label">{{ inft('statusFilterLabel') }}</span>
        <el-select v-model="status" clearable :placeholder="inft('statusAll')" style="width:200px" @change="onStatusChange">
          <el-option v-for="item in statusOptions" :key="item" :label="item" :value="item" />
        </el-select>
        <el-input v-model="lookupUid" clearable :placeholder="inft('uidLookupPlaceholder')" style="width:320px" @keyup.enter="openByUid" />
        <el-button @click="openByUid">{{ inft('lookup') }}</el-button>
        <span class="toolbar-total">{{ inft('paginationTotal', { count: total }) }}</span>
      </div>
      <el-table v-loading="loading" :data="tasks" size="small" @row-click="openDetail">
        <el-table-column prop="uid" :label="inft('taskUidCol')" min-width="260" show-overflow-tooltip />
        <el-table-column :label="inft('operationCol')" min-width="190">
          <template #default="{ row }">{{ row.operation }} @ {{ row.operationVersion }}</template>
        </el-table-column>
        <el-table-column prop="status" :label="inft('statusCol')" width="150">
          <template #default="{ row }"><el-tag :type="statusTagType(row.status)" size="small">{{ row.status }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="approvalStatus" :label="inft('approvalCol')" width="140" />
        <el-table-column prop="resourceUid" :label="inft('resourceCol')" min-width="220" show-overflow-tooltip />
        <el-table-column :label="inft('attemptsCol')" width="90">
          <template #default="{ row }">{{ row.attemptCount }}/{{ row.maxAttempts }}</template>
        </el-table-column>
        <el-table-column prop="createdAt" :label="inft('createdAtCol')" width="170" />
      </el-table>
    </el-card>

    <TaskDetail ref="detailRef" @updated="loadData" />
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { listInfraTasks } from '../../api/infra'
import { inft } from '../../utils/infra-i18n'
import TaskDetail from './TaskDetail.vue'

// §13.5 closed 9-state vocabulary — the filter mirrors the server-side set.
const statusOptions = [
  'planned', 'awaiting_approval', 'queued', 'running', 'succeeded',
  'failed', 'timed_out', 'cancelling', 'cancelled'
]

const route = useRoute()
const detailRef = ref(null)
const tasks = ref([])
const status = ref('')
const total = ref(0)
const loading = ref(false)
const lookupUid = ref('')

function statusTagType(value) {
  if (value === 'succeeded') return 'success'
  if (value === 'failed' || value === 'timed_out') return 'danger'
  if (value === 'cancelled' || value === 'cancelling') return 'info'
  if (value === 'running') return 'warning'
  return 'primary'
}

function onStatusChange() {
  loadData()
}

function openDetail(row) {
  detailRef.value?.open(row.uid)
}

function openByUid() {
  const uid = lookupUid.value.trim()
  if (!uid) return
  detailRef.value?.open(uid)
}

async function loadData() {
  loading.value = true
  try {
    const params = {}
    if (status.value) params.status = status.value
    const response = await listInfraTasks(params)
    tasks.value = response?.items || []
    total.value = response?.total ?? tasks.value.length
  } catch (error) {
    tasks.value = []
    total.value = 0
    ElMessage.error(inft('loadFailed'))
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadData()
  const uid = typeof route.query.uid === 'string' ? route.query.uid : ''
  if (uid) {
    lookupUid.value = uid
    detailRef.value?.open(uid)
  }
})
</script>

<style scoped>
.page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 16px; }
.page-title { margin: 0 0 4px; font-size: 20px; }
.page-desc { margin: 0; color: var(--el-text-color-secondary); font-size: 13px; }
.toolbar { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; }
.toolbar-label { color: var(--el-text-color-secondary); font-size: 13px; }
.toolbar-total { margin-left: auto; color: var(--el-text-color-secondary); font-size: 13px; }
</style>
