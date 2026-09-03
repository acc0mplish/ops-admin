<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { deleteOpsJobTemplate, queryOpsJobTemplateList, updateOpsJobTemplateStatus } from '../../api/ops'
import { ot } from '../../utils/ops-i18n'

const router = useRouter()
const loading = ref(false)
const rows = ref([])
const total = ref(0)

const query = reactive({
  pageNum: 1,
  pageSize: 10,
  keyword: '',
  status: ''
})

async function loadData() {
  loading.value = true
  try {
    const data = await queryOpsJobTemplateList(query)
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

function openCreate() {
  router.push({ path: '/ops/jobs/designer', query: { mode: 'template' } })
}

function openEdit(row) {
  router.push({ path: '/ops/jobs/designer', query: { mode: 'template', id: String(row.id) } })
}

async function handleDelete(row) {
  await ElMessageBox.confirm(ot('deleteTemplateConfirm', { name: row.name }), ot('deleteTemplateTitle'), { type: 'warning' })
  await deleteOpsJobTemplate(row.id)
  ElMessage.success(ot('deleteSuccess'))
  loadData()
}

async function toggleStatus(row) {
  const enabled = Number(row.status) === 1
  const action = enabled ? ot('disableAction') : ot('enableAction')
  await ElMessageBox.confirm(ot('toggleTemplateConfirm', { action, name: row.name }), ot('toggleTemplateTitle'), { type: 'warning' })
  await updateOpsJobTemplateStatus({ id: row.id, status: enabled ? 2 : 1 })
  ElMessage.success(enabled ? ot('templateDisabled') : ot('templateEnabled'))
  await loadData()
}

function statusLabel(status) {
  return Number(status) === 1 ? ot('enabled') : ot('disabled')
}

onMounted(loadData)
</script>

<template>
  <div class="page-card ops-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">{{ ot('jobTemplates') }}</h2>
        <p class="page-desc">{{ ot('jobTemplatesDesc') }}</p>
      </div>
      <el-button type="primary" @click="openCreate">{{ ot('newTemplate') }}</el-button>
    </div>

    <div class="toolbar">
      <div class="toolbar-left">
        <el-input v-model="query.keyword" clearable :placeholder="ot('searchTemplate')" style="width: 280px" @keyup.enter="loadData" />
        <el-select v-model="query.status" clearable :placeholder="ot('status')" style="width: 120px">
          <el-option :label="ot('enabled')" value="1" />
          <el-option :label="ot('disabled')" value="2" />
        </el-select>
        <el-button type="primary" @click="loadData">{{ ot('search') }}</el-button>
        <el-button @click="resetQuery">{{ ot('reset') }}</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="name" :label="ot('templateName')" min-width="220" />
      <el-table-column prop="description" :label="ot('description')" min-width="260" show-overflow-tooltip />
      <el-table-column :label="ot('status')" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'" effect="light">{{ statusLabel(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="updateTime" :label="ot('updatedAt')" width="180" />
      <el-table-column :label="ot('actions')" width="230" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">{{ ot('orchestrate') }}</el-button>
          <el-button link :class="Number(row.status) === 1 ? 'job-action-disable' : 'job-action-enable'" @click="toggleStatus(row)">{{ Number(row.status) === 1 ? ot('disableAction') : ot('enableAction') }}</el-button>
          <el-button link type="danger" @click="handleDelete(row)">{{ ot('delete') }}</el-button>
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
  </div>
</template>

<style scoped>
.ops-page { display: flex; flex-direction: column; gap: 18px; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
.page-title { margin: 0 0 8px; font-size: 24px; font-weight: 700; color: #14213d; }
.page-desc { margin: 0; color: #7282a0; }
.toolbar-left { display: flex; gap: 12px; flex-wrap: wrap; }
.pager { display: flex; justify-content: flex-end; }
.job-action-disable { color: #c87506 !important; font-weight: 600; }
.job-action-disable:hover { color: #9a5a00 !important; }
.job-action-enable { color: #49a828 !important; font-weight: 600; }
</style>
