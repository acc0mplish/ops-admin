<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { batchDeleteSysOperationLog, cleanSysOperationLog, deleteSysOperationLog, querySysOperationLogList } from '../../api/system'
import { st } from '../../utils/system-i18n'

const loading = ref(false)
const selectedIds = ref([])
const tableData = ref([])
const total = ref(0)
const stats = ref({ total: 0, highRisk: 0, failed: 0, avgDuration: 0 })
const detailVisible = ref(false)
const currentDetail = ref({})
const query = reactive({ pageNum: 1, pageSize: 10, username: '', keyword: '', riskLevel: '', success: '' })

async function loadData() {
  loading.value = true
  try {
    const data = await querySysOperationLogList(query)
    tableData.value = data.list || []
    total.value = data.total || 0
    stats.value = data.stats || { total: total.value, highRisk: 0, failed: 0, avgDuration: 0 }
  } finally { loading.value = false }
}
function onSelectionChange(rows) { selectedIds.value = rows.map((item) => item.id) }
async function handleDelete(id) { await deleteSysOperationLog(id); ElMessage.success(st('deletedSuccess')); await loadData() }
async function handleBatchDelete() { if (!selectedIds.value.length) return; await batchDeleteSysOperationLog(selectedIds.value); ElMessage.success(st('deletedSuccess')); await loadData() }
async function handleClean() { await ElMessageBox.confirm(st('clearLogsConfirm'), st('deleteConfirm'), { type: 'warning' }); await cleanSysOperationLog(); ElMessage.success(st('deletedSuccess')); await loadData() }
function resetQuery() { Object.assign(query, { pageNum: 1, username: '', keyword: '', riskLevel: '', success: '' }); loadData() }
function openDetail(row) { currentDetail.value = row; detailVisible.value = true }
function riskType(value) { return { high: 'danger', medium: 'warning', normal: 'info' }[value] || 'info' }
function riskText(value) { return { high: st('highRisk'), medium: st('mediumRisk'), normal: st('normalRisk') }[value] || value || st('normalRisk') }
function durationText(value) { if (value === undefined || value === null) return '-'; if (value < 1000) return `${value} ms`; return `${(value / 1000).toFixed(2)} s` }
onMounted(loadData)
</script>

<template>
  <div class="page-card console-card-page">
    <h2 class="page-title">{{ st('operationLog') }}</h2>
    <div class="audit-stat-grid">
      <div class="audit-stat-card"><span>{{ st('auditTotal') }}</span><strong>{{ stats.total || 0 }}</strong><small>{{ st('auditTotalHint') }}</small></div>
      <div class="audit-stat-card danger"><span>{{ st('highRiskOperations') }}</span><strong>{{ stats.highRisk || 0 }}</strong><small>{{ st('highRiskHint') }}</small></div>
      <div class="audit-stat-card warning"><span>{{ st('failedOperations') }}</span><strong>{{ stats.failed || 0 }}</strong><small>{{ st('failedOperationsHint') }}</small></div>
      <div class="audit-stat-card"><span>{{ st('averageDuration') }}</span><strong>{{ durationText(stats.avgDuration || 0) }}</strong><small>{{ st('averageDurationHint') }}</small></div>
    </div>
    <div class="toolbar">
      <div class="toolbar-left">
        <el-input v-model="query.username" :placeholder="st('searchAccount')" clearable style="width:220px" />
        <el-input v-model="query.keyword" :placeholder="st('searchOperation')" clearable style="width:260px" />
        <el-select v-model="query.riskLevel" clearable :placeholder="st('riskLevel')" style="width:140px"><el-option :label="st('highRisk')" value="high" /><el-option :label="st('mediumRisk')" value="medium" /><el-option :label="st('normalRisk')" value="normal" /></el-select>
        <el-select v-model="query.success" clearable :placeholder="st('executionResult')" style="width:140px"><el-option :label="st('success')" value="true" /><el-option :label="st('failure')" value="false" /></el-select>
        <el-button type="primary" @click="loadData">{{ st('query') }}</el-button><el-button @click="resetQuery">{{ st('reset') }}</el-button>
      </div>
      <div class="toolbar-right"><el-button v-permission="'system:operationlog:delete'" :disabled="!selectedIds.length" @click="handleBatchDelete">{{ st('batchDelete') }}</el-button><el-button v-permission="'system:operationlog:clean'" type="danger" plain @click="handleClean">{{ st('clearLogs') }}</el-button></div>
    </div>
    <el-table v-loading="loading" :data="tableData" border @selection-change="onSelectionChange">
      <el-table-column type="selection" width="48" />
      <el-table-column prop="username" :label="st('account')" min-width="120" />
      <el-table-column :label="st('risk')" width="90"><template #default="{ row }"><el-tag :type="riskType(row.riskLevel)" effect="light">{{ riskText(row.riskLevel) }}</el-tag></template></el-table-column>
      <el-table-column :label="st('result')" width="90"><template #default="{ row }"><el-tag :type="row.success ? 'success' : 'danger'" effect="light">{{ row.success ? st('success') : st('failure') }}</el-tag></template></el-table-column>
      <el-table-column prop="method" :label="st('method')" width="90" />
      <el-table-column :label="st('duration')" width="100"><template #default="{ row }">{{ durationText(row.durationMs) }}</template></el-table-column>
      <el-table-column prop="statusCode" :label="st('statusCode')" width="90" />
      <el-table-column prop="ip" label="IP" min-width="120" />
      <el-table-column prop="description" :label="st('description')" min-width="180" />
      <el-table-column prop="url" label="URL" min-width="220" />
      <el-table-column prop="createTime" :label="st('operationTime')" min-width="180" />
      <el-table-column :label="st('actions')" width="140"><template #default="{ row }"><el-button link type="primary" @click="openDetail(row)">{{ st('detail') }}</el-button><el-button v-permission="'system:operationlog:delete'" link type="danger" @click="handleDelete(row.id)">{{ st('delete') }}</el-button></template></el-table-column>
    </el-table>
    <div style="margin-top:16px;display:flex;justify-content:flex-end;"><el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total" layout="total, prev, pager, next, sizes" @current-change="loadData" @size-change="loadData" /></div>
    <el-dialog v-model="detailVisible" :title="st('operationAuditDetail')" width="760px">
      <el-descriptions :column="2" border>
        <el-descriptions-item :label="st('account')">{{ currentDetail.username || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="st('sourceIp')">{{ currentDetail.ip || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="st('riskLevel')"><el-tag :type="riskType(currentDetail.riskLevel)" effect="light">{{ riskText(currentDetail.riskLevel) }}</el-tag></el-descriptions-item>
        <el-descriptions-item :label="st('executionResult')"><el-tag :type="currentDetail.success ? 'success' : 'danger'" effect="light">{{ currentDetail.success ? st('success') : st('failure') }}</el-tag></el-descriptions-item>
        <el-descriptions-item :label="st('requestMethod')">{{ currentDetail.method }}</el-descriptions-item>
        <el-descriptions-item :label="st('statusCode')">{{ currentDetail.statusCode || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="st('duration')">{{ durationText(currentDetail.durationMs) }}</el-descriptions-item>
        <el-descriptions-item :label="st('operationTime')">{{ currentDetail.createTime || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="st('description')" :span="2">{{ currentDetail.description || '-' }}</el-descriptions-item>
        <el-descriptions-item label="URL" :span="2">{{ currentDetail.url || '-' }}</el-descriptions-item>
      </el-descriptions>
      <div class="audit-summary"><strong>{{ st('requestSummary') }}</strong><pre>{{ currentDetail.requestSummary || st('noRequestParameters') }}</pre></div>
    </el-dialog>
  </div>
</template>

<style scoped>
.audit-summary { margin-top: 16px; }
.audit-stat-grid { display: grid; grid-template-columns: repeat(4, minmax(160px, 1fr)); gap: 14px; margin-bottom: 18px; }
.audit-stat-card { padding: 16px; border: 1px solid #e3ebf7; border-radius: 10px; background: linear-gradient(180deg, #ffffff 0%, #f8fbff 100%); }
.audit-stat-card span { color: #7182a0; font-size: 13px; }
.audit-stat-card strong { display: block; margin: 8px 0 4px; color: #0f1f3d; font-size: 28px; }
.audit-stat-card small { color: #8b9ab3; }
.audit-stat-card.danger strong { color: #ef4444; }
.audit-stat-card.warning strong { color: #f59e0b; }
.audit-summary strong { display: block; margin-bottom: 8px; color: #0f1f3d; }
.audit-summary pre { max-height: 280px; margin: 0; padding: 14px; overflow: auto; border-radius: 8px; background: #111827; color: #d6e2ff; font-family: Consolas, Monaco, monospace; font-size: 13px; line-height: 1.6; white-space: pre-wrap; }
</style>
