<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { queryMonitorDatasourceOptions, queryMonitorPrometheus, queryMonitorQueryHistoryList } from '../../api/monitor'

const loading = ref(false)
const datasourceOptions = ref([])
const datasourceId = ref()
const promql = ref('up')
const resultType = ref('')
const rows = ref([])
const historyVisible = ref(false)
const historyLoading = ref(false)
const historyRows = ref([])
const historyTotal = ref(0)
const historyQuery = ref({ pageNum: 1, pageSize: 10, keyword: '', status: '' })

function metricText(metric) {
  return Object.entries(metric || {}).map(([key, value]) => `${key}="${value}"`).join(', ')
}

async function loadOptions() {
  datasourceOptions.value = await queryMonitorDatasourceOptions()
  datasourceId.value = datasourceOptions.value.find((item) => item.isDefault)?.id || datasourceOptions.value[0]?.id
}

async function executeQuery() {
  if (!datasourceId.value || !promql.value.trim()) {
    ElMessage.warning('请选择数据源并输入 PromQL')
    return
  }
  loading.value = true
  try {
    const data = await queryMonitorPrometheus({ datasourceId: datasourceId.value, query: promql.value })
    resultType.value = data.resultType || ''
    rows.value = data.result || []
    if (historyVisible.value) await loadHistory()
  } finally {
    loading.value = false
  }
}

async function loadHistory() {
  historyLoading.value = true
  try {
    const data = await queryMonitorQueryHistoryList(historyQuery.value)
    historyRows.value = data.list || []
    historyTotal.value = data.total || 0
  } finally {
    historyLoading.value = false
  }
}

async function openHistory() {
  historyVisible.value = true
  await loadHistory()
}

function useHistory(row) {
  const datasource = datasourceOptions.value.find((item) => item.name === row.datasourceName)
  datasourceId.value = row.datasourceId || datasource?.id || datasourceId.value
  promql.value = row.query || promql.value
  historyVisible.value = false
}

onMounted(async () => {
  await loadOptions()
})
</script>

<template>
  <div class="monitor-query">
    <div class="query-top">
      <div>
        <h2>即时查询</h2>
        <p>输入 PromQL 直接查询指标，用于调试告警规则和排查现场。</p>
      </div>
      <div class="query-top-actions">
        <el-select v-model="datasourceId" filterable placeholder="选择数据源" style="width: 260px">
          <el-option v-for="item in datasourceOptions" :key="item.id" :label="item.name" :value="item.id" />
        </el-select>
        <el-button @click="openHistory">查询历史</el-button>
      </div>
    </div>

    <div class="query-editor">
      <el-input v-model="promql" type="textarea" :rows="5" placeholder="例如：up == 0" />
      <div class="query-actions">
        <el-button type="primary" :loading="loading" @click="executeQuery">执行查询</el-button>
        <span>Result Type: {{ resultType || '-' }}</span>
      </div>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column label="Metric" min-width="420" show-overflow-tooltip>
        <template #default="{ row }">{{ metricText(row.metric) }}</template>
      </el-table-column>
      <el-table-column label="Value" width="180">
        <template #default="{ row }">{{ row.value?.[1] ?? '-' }}</template>
      </el-table-column>
      <el-table-column label="Timestamp" width="180">
        <template #default="{ row }">{{ row.value?.[0] ?? '-' }}</template>
      </el-table-column>
    </el-table>

    <el-drawer v-model="historyVisible" title="查询历史" size="640px">
      <div class="history-toolbar">
        <el-input v-model="historyQuery.keyword" clearable placeholder="搜索 PromQL / 数据源" @keyup.enter="loadHistory" />
        <el-select v-model="historyQuery.status" clearable placeholder="状态" style="width: 120px">
          <el-option label="成功" value="success" />
          <el-option label="失败" value="failed" />
        </el-select>
        <el-button type="primary" @click="loadHistory">搜索</el-button>
      </div>
      <el-table v-loading="historyLoading" :data="historyRows" border>
        <el-table-column prop="datasourceName" label="数据源" width="140" />
        <el-table-column prop="query" label="PromQL" min-width="240" show-overflow-tooltip />
        <el-table-column prop="status" label="状态" width="90" />
        <el-table-column prop="createTime" label="时间" width="170" />
        <el-table-column label="操作" width="80" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="useHistory(row)">使用</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="pager">
        <el-pagination v-model:current-page="historyQuery.pageNum" v-model:page-size="historyQuery.pageSize" :total="historyTotal" layout="total, prev, pager, next" @current-change="loadHistory" @size-change="loadHistory" />
      </div>
    </el-drawer>
  </div>
</template>

<style scoped>
.monitor-query { display: flex; flex-direction: column; gap: 18px; padding: 24px; background: #fff; border-radius: 18px; box-shadow: 0 12px 30px rgba(36, 54, 90, 0.08); }
.query-top { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
.query-top-actions { display: flex; gap: 10px; align-items: center; }
.query-top h2 { margin: 0 0 8px; font-size: 26px; color: #10213f; }
.query-top p { margin: 0; color: #7282a0; }
.query-editor { border: 1px solid #dfe7f2; border-radius: 12px; overflow: hidden; }
.query-editor :deep(.el-textarea__inner) { border: 0; border-radius: 0; font-family: Consolas, Monaco, monospace; font-size: 14px; }
.query-actions { display: flex; justify-content: space-between; align-items: center; padding: 12px; background: #f7f9fd; border-top: 1px solid #e5ecf6; color: #7282a0; }
.history-toolbar { display: flex; gap: 10px; margin-bottom: 14px; }
.pager { display: flex; justify-content: flex-end; margin-top: 14px; }
</style>
