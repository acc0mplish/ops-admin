<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { queryMonitorDatasourceOptions, queryMonitorJaegerOperations, queryMonitorJaegerServices, queryMonitorTraces } from '../../api/monitor'

const loading = ref(false)
const serviceLoading = ref(false)
const operationLoading = ref(false)
const datasources = ref([])
const services = ref([])
const operations = ref([])
const datasourceId = ref()
const service = ref('')
const operation = ref('')
const tags = ref('')
const traceId = ref('')
const timeRange = ref('1h')
const customDateRange = ref([])
const items = ref([])
const router = useRouter()

const activeDatasource = computed(() => datasources.value.find((item) => item.id === datasourceId.value))
function rangeTimestamps() {
  if (timeRange.value === 'custom' && customDateRange.value?.length === 2) {
    const [startAt, endAt] = customDateRange.value.map(Number)
    if (Number.isFinite(startAt) && Number.isFinite(endAt) && startAt < endAt) return { startAt, endAt }
  }
  const endAt = Date.now()
  const minutes = { '15m': 15, '1h': 60, '6h': 360, '24h': 1440, '7d': 10080 }[timeRange.value] || 60
  return { startAt: endAt - minutes * 60000, endAt }
}

function formatTime(value) {
  if (!value) return '-'
  const time = Number(value) > 100000000000000 ? Number(value) / 1000 : Number(value)
  const date = new Date(time)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString('zh-CN', { hour12: false })
}

function formatDuration(value) {
  const microseconds = Number(value || 0)
  if (microseconds >= 1000000) return `${(microseconds / 1000000).toFixed(2)} s`
  if (microseconds >= 1000) return `${(microseconds / 1000).toFixed(2)} ms`
  return `${microseconds} μs`
}

function traceServices(trace) {
  return [...new Set(Object.values(trace.processes || {}).map((item) => item.serviceName).filter(Boolean))].join(' → ') || '-'
}

function traceOperation(trace) {
  const spans = trace.spans || []
  const rootSpan = spans.find((span) => !(span.references || []).some((reference) => reference.refType === 'CHILD_OF'))
  return rootSpan?.operationName || spans[0]?.operationName || '-'
}

function traceDuration(trace) {
  const spans = trace.spans || []
  if (!spans.length) return '-'
  const started = Math.min(...spans.map((item) => Number(item.startTime || 0)))
  const ended = Math.max(...spans.map((item) => Number(item.startTime || 0) + Number(item.duration || 0)))
  return formatDuration(ended - started)
}

async function loadServices() {
  services.value = []
  operations.value = []
  service.value = ''
  operation.value = ''
  if (!datasourceId.value) return
  serviceLoading.value = true
  try {
    services.value = await queryMonitorJaegerServices({ datasourceId: datasourceId.value }) || []
  } finally {
    serviceLoading.value = false
  }
}

async function loadOperations() {
  operations.value = []
  operation.value = ''
  if (!datasourceId.value || !service.value) return
  operationLoading.value = true
  try {
    operations.value = await queryMonitorJaegerOperations({ datasourceId: datasourceId.value, service: service.value }) || []
  } finally {
    operationLoading.value = false
  }
}

async function loadDatasources() {
  const options = await queryMonitorDatasourceOptions()
  datasources.value = (options || []).filter((item) => item.type === 'jaeger')
  datasourceId.value = datasources.value[0]?.id
}

async function search() {
  if (!datasourceId.value) return ElMessage.warning('请先在数据源管理中配置 Jaeger 数据源')
  loading.value = true
  try {
    if (traceId.value.trim()) {
      router.push({ path: `/monitor/traces/${encodeURIComponent(traceId.value.trim())}`, query: { datasourceId: datasourceId.value } })
      return
    } else {
      if (!service.value) return ElMessage.warning('请选择服务，或直接输入 Trace ID 查询')
      if (timeRange.value === 'custom' && customDateRange.value?.length !== 2) return ElMessage.warning('请选择完整的起止时间')
      const { startAt, endAt } = rangeTimestamps()
      items.value = await queryMonitorTraces({ datasourceId: datasourceId.value, service: service.value, operation: operation.value, tags: tags.value, startAt, endAt, limit: 100 }) || []
    }
  } finally {
    loading.value = false
  }
}

function openDetail(trace) {
  router.push({ path: `/monitor/traces/${encodeURIComponent(trace.traceID)}`, query: { datasourceId: datasourceId.value } })
}

watch(datasourceId, loadServices)
watch(service, loadOperations)
onMounted(loadDatasources)
</script>

<template>
  <div class="trace-page">
    <div class="page-header"><div><h2>链路追踪</h2><p>通过 Jaeger 检索分布式调用链，定位服务调用耗时与异常 Span。</p></div></div>
    <el-alert v-if="!datasources.length" title="尚未配置 Jaeger 数据源" type="warning" :closable="false" show-icon>
      请先前往“数据源管理”新增并测试 Jaeger Query 服务地址。
    </el-alert>
    <div class="toolbar">
      <el-select v-model="datasourceId" filterable placeholder="选择 Jaeger 数据源" style="width: 220px"><el-option v-for="item in datasources" :key="item.id" :label="item.name" :value="item.id" /></el-select>
      <el-select v-model="service" filterable clearable :loading="serviceLoading" placeholder="选择服务" style="width: 220px"><el-option v-for="item in services" :key="item" :label="item" :value="item" /></el-select>
      <el-select v-model="operation" filterable clearable :loading="operationLoading" :disabled="!service" placeholder="选择 Operation（可选）" style="width: 230px"><el-option v-for="item in operations" :key="item" :label="item" :value="item" /></el-select>
      <el-select v-model="timeRange" style="width: 130px"><el-option label="最近 15 分钟" value="15m" /><el-option label="最近 1 小时" value="1h" /><el-option label="最近 6 小时" value="6h" /><el-option label="最近 24 小时" value="24h" /><el-option label="最近 7 天" value="7d" /><el-option label="自定义时间" value="custom" /></el-select>
      <el-date-picker v-if="timeRange === 'custom'" v-model="customDateRange" type="datetimerange" value-format="x" start-placeholder="开始时间" end-placeholder="结束时间" range-separator="至" style="width: 360px" />
      <el-input v-model="tags" clearable placeholder='标签 JSON，例如 {"http.status_code":"500"}' style="width: 300px" @keyup.enter="search" />
      <el-input v-model="traceId" clearable placeholder="Trace ID（可直接查询）" style="width: 280px" @keyup.enter="search" />
      <el-button type="primary" :loading="loading" @click="search">查询</el-button>
    </div>
    <div class="hint">当前数据源：{{ activeDatasource?.name || '-' }}。标签筛选需填写 Jaeger 格式的 JSON 对象。</div>
    <el-table v-loading="loading" :data="items" border empty-text="请选择服务并查询，或输入 Trace ID 直接定位调用链">
      <el-table-column label="Trace ID" min-width="270"><template #default="{ row }"><el-link type="primary" @click="openDetail(row)">{{ row.traceID }}</el-link></template></el-table-column>
      <el-table-column label="服务链路" min-width="260" show-overflow-tooltip><template #default="{ row }">{{ traceServices(row) }}</template></el-table-column>
      <el-table-column label="Operation" min-width="240" show-overflow-tooltip><template #default="{ row }">{{ traceOperation(row) }}</template></el-table-column>
      <el-table-column label="开始时间" width="190"><template #default="{ row }">{{ formatTime(row.spans?.[0]?.startTime) }}</template></el-table-column>
      <el-table-column label="总耗时" width="120"><template #default="{ row }">{{ traceDuration(row) }}</template></el-table-column>
      <el-table-column label="Span 数" width="110" align="center" sortable :sort-method="(left, right) => (left.spans?.length || 0) - (right.spans?.length || 0)"><template #default="{ row }">{{ row.spans?.length || 0 }}</template></el-table-column>
      <el-table-column label="操作" width="90"><template #default="{ row }"><el-button link type="primary" @click="openDetail(row)">详情</el-button></template></el-table-column>
    </el-table>
  </div>
</template>

<style scoped>
.trace-page { display: flex; flex-direction: column; gap: 18px; padding: 24px; background: #fff; border-radius: 18px; box-shadow: 0 12px 30px rgba(36, 54, 90, .08); }
.page-header h2 { margin: 0 0 8px; font-size: 26px; color: #10213f; }.page-header p, .hint { margin: 0; color: #7282a0; }.toolbar { display: flex; flex-wrap: wrap; gap: 12px; }.hint { font-size: 13px; }
</style>
