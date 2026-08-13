<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { batchDeleteSysOperationLog, cleanSysOperationLog, deleteSysOperationLog, querySysOperationLogList } from '../../api/system'

const loading = ref(false)
const selectedIds = ref([])
const tableData = ref([])
const total = ref(0)
const stats = ref({ total: 0, highRisk: 0, failed: 0, avgDuration: 0 })
const detailVisible = ref(false)
const currentDetail = ref({})
const query = reactive({
  pageNum: 1,
  pageSize: 10,
  username: '',
  keyword: '',
  riskLevel: '',
  success: ''
})

async function loadData() {
  loading.value = true
  try {
    const data = await querySysOperationLogList(query)
    tableData.value = data.list || []
    total.value = data.total || 0
    stats.value = data.stats || { total: total.value, highRisk: 0, failed: 0, avgDuration: 0 }
  } finally {
    loading.value = false
  }
}

function onSelectionChange(rows) {
  selectedIds.value = rows.map((item) => item.id)
}

async function handleDelete(id) {
  await deleteSysOperationLog(id)
  ElMessage.success('删除成功')
  await loadData()
}

async function handleBatchDelete() {
  if (!selectedIds.value.length) return
  await batchDeleteSysOperationLog(selectedIds.value)
  ElMessage.success('批量删除成功')
  await loadData()
}

async function handleClean() {
  await ElMessageBox.confirm('确认清空操作日志吗？', '提示', { type: 'warning' })
  await cleanSysOperationLog()
  ElMessage.success('已清空')
  await loadData()
}

function resetQuery() {
  Object.assign(query, { pageNum: 1, username: '', keyword: '', riskLevel: '', success: '' })
  loadData()
}

function openDetail(row) {
  currentDetail.value = row
  detailVisible.value = true
}

function riskType(value) {
  return { high: 'danger', medium: 'warning', normal: 'info' }[value] || 'info'
}

function riskText(value) {
  return { high: '高危', medium: '中危', normal: '普通' }[value] || value || '普通'
}

function durationText(value) {
  if (value === undefined || value === null) return '-'
  if (value < 1000) return `${value} ms`
  return `${(value / 1000).toFixed(2)} s`
}

onMounted(loadData)
</script>

<template>
  <div class="page-card console-card-page">
    <h2 class="page-title">操作日志</h2>
    <div class="audit-stat-grid">
      <div class="audit-stat-card">
        <span>审计总数</span>
        <strong>{{ stats.total || 0 }}</strong>
        <small>当前筛选范围内的操作记录</small>
      </div>
      <div class="audit-stat-card danger">
        <span>高危操作</span>
        <strong>{{ stats.highRisk || 0 }}</strong>
        <small>删除、SQL、K8s、发布等敏感动作</small>
      </div>
      <div class="audit-stat-card warning">
        <span>失败操作</span>
        <strong>{{ stats.failed || 0 }}</strong>
        <small>接口返回失败或异常的操作</small>
      </div>
      <div class="audit-stat-card">
        <span>平均耗时</span>
        <strong>{{ durationText(stats.avgDuration || 0) }}</strong>
        <small>辅助定位慢操作和卡顿接口</small>
      </div>
    </div>
    <div class="toolbar">
      <div class="toolbar-left">
        <el-input v-model="query.username" placeholder="按账号搜索" clearable style="width:220px" />
        <el-input v-model="query.keyword" placeholder="描述 / URL / IP / 参数" clearable style="width:260px" />
        <el-select v-model="query.riskLevel" clearable placeholder="风险等级" style="width:140px">
          <el-option label="高危" value="high" />
          <el-option label="中危" value="medium" />
          <el-option label="普通" value="normal" />
        </el-select>
        <el-select v-model="query.success" clearable placeholder="执行结果" style="width:140px">
          <el-option label="成功" value="true" />
          <el-option label="失败" value="false" />
        </el-select>
        <el-button type="primary" @click="loadData">查询</el-button>
        <el-button @click="resetQuery">重置</el-button>
      </div>
      <div class="toolbar-right">
        <el-button v-permission="'system:operationlog:delete'" :disabled="!selectedIds.length" @click="handleBatchDelete">批量删除</el-button>
        <el-button v-permission="'system:operationlog:clean'" type="danger" plain @click="handleClean">清空日志</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="tableData" border @selection-change="onSelectionChange">
      <el-table-column type="selection" width="48" />
      <el-table-column prop="username" label="账号" min-width="120" />
      <el-table-column label="风险" width="90">
        <template #default="{ row }">
          <el-tag :type="riskType(row.riskLevel)" effect="light">{{ riskText(row.riskLevel) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="结果" width="90">
        <template #default="{ row }">
          <el-tag :type="row.success ? 'success' : 'danger'" effect="light">{{ row.success ? '成功' : '失败' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="method" label="方法" width="90" />
      <el-table-column label="耗时" width="100">
        <template #default="{ row }">{{ durationText(row.durationMs) }}</template>
      </el-table-column>
      <el-table-column prop="statusCode" label="状态码" width="90" />
      <el-table-column prop="ip" label="IP" min-width="120" />
      <el-table-column prop="description" label="描述" min-width="180" />
      <el-table-column prop="url" label="URL" min-width="220" />
      <el-table-column prop="createTime" label="操作时间" min-width="180" />
      <el-table-column label="操作" width="140">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDetail(row)">详情</el-button>
          <el-button v-permission="'system:operationlog:delete'" link type="danger" @click="handleDelete(row.id)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div style="margin-top:16px;display:flex;justify-content:flex-end;">
      <el-pagination
        v-model:current-page="query.pageNum"
        v-model:page-size="query.pageSize"
        :total="total"
        layout="total, prev, pager, next, sizes"
        @current-change="loadData"
        @size-change="loadData"
      />
    </div>

    <el-dialog v-model="detailVisible" title="操作审计详情" width="760px">
      <el-descriptions :column="2" border>
        <el-descriptions-item label="账号">{{ currentDetail.username || '-' }}</el-descriptions-item>
        <el-descriptions-item label="来源 IP">{{ currentDetail.ip || '-' }}</el-descriptions-item>
        <el-descriptions-item label="风险等级">
          <el-tag :type="riskType(currentDetail.riskLevel)" effect="light">{{ riskText(currentDetail.riskLevel) }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="执行结果">
          <el-tag :type="currentDetail.success ? 'success' : 'danger'" effect="light">{{ currentDetail.success ? '成功' : '失败' }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="请求方法">{{ currentDetail.method }}</el-descriptions-item>
        <el-descriptions-item label="状态码">{{ currentDetail.statusCode || '-' }}</el-descriptions-item>
        <el-descriptions-item label="耗时">{{ durationText(currentDetail.durationMs) }}</el-descriptions-item>
        <el-descriptions-item label="操作时间">{{ currentDetail.createTime || '-' }}</el-descriptions-item>
        <el-descriptions-item label="描述" :span="2">{{ currentDetail.description || '-' }}</el-descriptions-item>
        <el-descriptions-item label="URL" :span="2">{{ currentDetail.url || '-' }}</el-descriptions-item>
      </el-descriptions>
      <div class="audit-summary">
        <strong>请求摘要</strong>
        <pre>{{ currentDetail.requestSummary || '无请求参数' }}</pre>
      </div>
    </el-dialog>
  </div>
</template>

<style scoped>
.audit-summary {
  margin-top: 16px;
}

.audit-stat-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(160px, 1fr));
  gap: 14px;
  margin-bottom: 18px;
}

.audit-stat-card {
  padding: 16px;
  border: 1px solid #e3ebf7;
  border-radius: 10px;
  background: linear-gradient(180deg, #ffffff 0%, #f8fbff 100%);
}

.audit-stat-card span {
  color: #7182a0;
  font-size: 13px;
}

.audit-stat-card strong {
  display: block;
  margin: 8px 0 4px;
  color: #0f1f3d;
  font-size: 28px;
}

.audit-stat-card small {
  color: #8b9ab3;
}

.audit-stat-card.danger strong {
  color: #ef4444;
}

.audit-stat-card.warning strong {
  color: #f59e0b;
}

.audit-summary strong {
  display: block;
  margin-bottom: 8px;
  color: #0f1f3d;
}

.audit-summary pre {
  max-height: 280px;
  margin: 0;
  padding: 14px;
  overflow: auto;
  border-radius: 8px;
  background: #111827;
  color: #d6e2ff;
  font-family: Consolas, Monaco, monospace;
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
}
</style>
