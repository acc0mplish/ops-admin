<template>
  <div class="change-page">
    <section class="page-head">
      <div>
        <p class="eyebrow">CHANGE CENTER</p>
        <h1>变更中心</h1>
        <p>统一汇总作业、应用发布、SQL 执行、K8s YAML 修改和告警联动处置。</p>
      </div>
    </section>

    <section class="stats-grid">
      <div class="stat-card">
        <span>变更总数</span>
        <strong>{{ stats.total || 0 }}</strong>
      </div>
      <div class="stat-card danger">
        <span>失败变更</span>
        <strong>{{ stats.failed || 0 }}</strong>
      </div>
      <div class="stat-card warning">
        <span>高风险</span>
        <strong>{{ stats.highRisk || 0 }}</strong>
      </div>
    </section>

    <section class="toolbar">
      <el-input v-model="query.keyword" clearable placeholder="搜索标题 / 来源 / 应用" @keyup.enter="loadData" />
      <el-select v-model="query.env" clearable placeholder="环境">
        <el-option v-for="item in environments" :key="item.code" :label="item.name" :value="item.code" />
      </el-select>
      <el-select v-model="query.changeType" clearable placeholder="变更类型">
        <el-option label="作业" value="job" />
        <el-option label="发布" value="release" />
        <el-option label="流水线" value="pipeline" />
        <el-option label="SQL" value="sql" />
        <el-option label="K8s YAML" value="k8s_yaml_update" />
        <el-option label="告警处置" value="alert_action" />
      </el-select>
      <el-select v-model="query.status" clearable placeholder="状态">
        <el-option label="运行中" value="running" />
        <el-option label="成功" value="success" />
        <el-option label="失败" value="failed" />
      </el-select>
      <el-button type="primary" @click="loadData">查询</el-button>
      <el-button @click="resetQuery">重置</el-button>
    </section>

    <el-table :data="list" class="data-table">
      <el-table-column prop="title" label="变更标题" min-width="220" show-overflow-tooltip />
      <el-table-column prop="changeType" label="类型" width="120" />
      <el-table-column prop="env" label="环境" width="100" />
      <el-table-column prop="riskLevel" label="风险" width="100">
        <template #default="{ row }">
          <el-tag :type="riskType(row.riskLevel)">{{ riskText(row.riskLevel) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="status" label="状态" width="110">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status)">{{ statusText(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="appName" label="应用" min-width="150" show-overflow-tooltip />
      <el-table-column prop="sourceName" label="来源" min-width="180" show-overflow-tooltip />
      <el-table-column prop="summary" label="摘要" min-width="220" show-overflow-tooltip />
      <el-table-column prop="createTime" label="时间" width="190" />
      <el-table-column label="操作" width="90" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="showDetail(row)">详情</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination
      class="pager"
      background
      layout="total, prev, pager, next"
      :total="total"
      :page-size="query.pageSize"
      v-model:current-page="query.pageNum"
      @current-change="loadData"
    />

    <el-dialog v-model="detailVisible" title="变更详情" width="820px">
      <el-descriptions :column="2" border>
        <el-descriptions-item label="标题">{{ current.title }}</el-descriptions-item>
        <el-descriptions-item label="状态">{{ statusText(current.status) }}</el-descriptions-item>
        <el-descriptions-item label="类型">{{ current.changeType }}</el-descriptions-item>
        <el-descriptions-item label="环境">{{ current.env || '-' }}</el-descriptions-item>
        <el-descriptions-item label="应用">{{ current.appName || '-' }}</el-descriptions-item>
        <el-descriptions-item label="来源">{{ current.sourceName || '-' }}</el-descriptions-item>
        <el-descriptions-item label="摘要" :span="2">{{ current.summary || '-' }}</el-descriptions-item>
      </el-descriptions>
      <pre class="detail-box">{{ current.detail || '暂无详情' }}</pre>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { queryOpsChangeList, queryOpsEnvironmentList } from '../../api/ops'

const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', env: '', changeType: '', status: '' })
const list = ref([])
const total = ref(0)
const stats = ref({})
const environments = ref([])
const detailVisible = ref(false)
const current = ref({})

async function loadData() {
  const data = await queryOpsChangeList(query)
  list.value = data.list || []
  total.value = data.total || 0
  stats.value = data.stats || {}
}

async function loadEnvironments() {
  environments.value = await queryOpsEnvironmentList({ status: 1 })
}

function resetQuery() {
  Object.assign(query, { pageNum: 1, keyword: '', env: '', changeType: '', status: '' })
  loadData()
}

function showDetail(row) {
  current.value = row
  detailVisible.value = true
}

function riskType(value) {
  return value === 'high' || value === 'critical' ? 'danger' : value === 'medium' ? 'warning' : 'success'
}

function riskText(value) {
  return ({ low: '低', medium: '中', high: '高', critical: '严重' }[value] || value || '-')
}

function statusType(value) {
  return value === 'success' ? 'success' : value === 'running' ? 'warning' : 'danger'
}

function statusText(value) {
  return ({ success: '成功', running: '运行中', failed: '失败' }[value] || value || '-')
}

onMounted(async () => {
  await loadEnvironments()
  await loadData()
})
</script>

<style scoped>
.change-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.page-head,
.toolbar,
.data-table,
.stat-card {
  background: #fff;
  border: 1px solid #dfe8f6;
  border-radius: 8px;
}
.page-head {
  padding: 24px;
}
.eyebrow {
  margin: 0 0 8px;
  color: #2f6eea;
  font-size: 12px;
  font-weight: 800;
}
h1 {
  margin: 0;
  color: #071a3d;
}
.page-head p:last-child {
  margin: 8px 0 0;
  color: #6d7f9f;
}
.stats-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 14px;
}
.stat-card {
  padding: 18px;
}
.stat-card span {
  color: #7889a8;
}
.stat-card strong {
  display: block;
  margin-top: 8px;
  font-size: 28px;
  color: #071a3d;
}
.stat-card.danger strong {
  color: #ef4444;
}
.stat-card.warning strong {
  color: #f59e0b;
}
.toolbar {
  display: flex;
  gap: 12px;
  padding: 16px;
}
.toolbar .el-input {
  width: 300px;
}
.toolbar .el-select {
  width: 150px;
}
.pager {
  align-self: flex-end;
}
.detail-box {
  max-height: 360px;
  padding: 14px;
  margin-top: 14px;
  overflow: auto;
  color: #dbeafe;
  background: #101827;
  border-radius: 8px;
  font-family: Consolas, Monaco, monospace;
}
</style>
