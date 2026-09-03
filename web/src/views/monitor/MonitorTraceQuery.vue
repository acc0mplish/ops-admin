<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { queryMonitorDatasourceOptions, queryMonitorJaegerOperations, queryMonitorJaegerServices, queryMonitorTraces } from '../../api/monitor'
import { mt } from '../../utils/monitor-i18n'

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
const pageNum = ref(1)
const pageSize = ref(20)
const lastQueryWindow = ref(null)
const router = useRouter()
const route = useRoute()
let restoringQuery = false

const activeDatasource = computed(() => datasources.value.find((item) => item.id === datasourceId.value))
const pagedItems = computed(() => {
  const start = (pageNum.value - 1) * pageSize.value
  return items.value.slice(start, start + pageSize.value)
})
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
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString(undefined, { hour12: false })
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

function traceRootSpan(trace) {
  const spans = trace.spans || []
  return spans.find((span) => !(span.references || []).some((reference) => reference.refType === 'CHILD_OF')) || spans[0] || {}
}

function traceStatus(trace) {
  const span = traceRootSpan(trace)
  const spanTags = span.tags || []
  const httpTag = spanTags.find((item) => ['http.status_code', 'http.response.status_code', 'http.status', 'http.statusCode'].includes(item.key))
  if (httpTag) return String(httpTag.value ?? '-')
  const hasError = (trace.spans || []).some((item) => (item.tags || []).some((tag) => {
    if (tag.key === 'error') return String(tag.value).toLowerCase() !== 'false'
    if (tag.key === 'status.code') return String(tag.value).toUpperCase() === 'ERROR'
    return ['http.status_code', 'http.response.status_code'].includes(tag.key) && Number(tag.value) >= 400
  }))
  return hasError ? mt('abnormal') : mt('success')
}

function traceStatusTagType(trace) {
  const status = traceStatus(trace)
  const code = Number(status)
  if (code >= 200 && code < 400) return 'success'
  if (code >= 500 || status === mt('abnormal') || status.toUpperCase() === 'ERROR') return 'danger'
  if (code >= 400) return 'warning'
  return status === mt('success') ? 'success' : 'info'
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

function queryWindowFromRoute() {
  const startAt = Number(route.query.startAt)
  const endAt = Number(route.query.endAt)
  return Number.isFinite(startAt) && Number.isFinite(endAt) && startAt < endAt ? { startAt, endAt } : null
}

function detailRouteQuery() {
  const { startAt, endAt } = lastQueryWindow.value || rangeTimestamps()
  return {
    datasourceId: datasourceId.value, service: service.value || undefined, operation: operation.value || undefined,
    tags: tags.value || undefined, timeRange: timeRange.value, startAt, endAt, page: pageNum.value, pageSize: pageSize.value
  }
}

async function queryList(window = null, preservePage = false) {
  if (!service.value) return ElMessage.warning(mt('selectServiceWarning'))
  if (timeRange.value === 'custom' && customDateRange.value?.length !== 2) return ElMessage.warning(mt('selectFullTimeRange'))
  const { startAt, endAt } = window || rangeTimestamps()
  lastQueryWindow.value = { startAt, endAt }
  items.value = await queryMonitorTraces({ datasourceId: datasourceId.value, service: service.value, operation: operation.value, tags: tags.value, startAt, endAt, limit: 1000 }) || []
  if (!preservePage) pageNum.value = 1
  const pageCount = Math.max(1, Math.ceil(items.value.length / pageSize.value))
  pageNum.value = Math.min(Math.max(1, pageNum.value), pageCount)
}

async function restoreQueryState() {
  const state = route.query
  if (!state?.service) return false
  restoringQuery = true
  try {
    datasourceId.value = datasources.value.find((item) => Number(item.id) === Number(state.datasourceId))?.id || datasources.value[0]?.id
    await loadServices()
    service.value = state.service || ''
    await loadOperations()
    operation.value = state.operation || ''
    tags.value = state.tags || ''
    timeRange.value = state.timeRange || '1h'
    customDateRange.value = state.timeRange === 'custom' && queryWindowFromRoute() ? [String(state.startAt), String(state.endAt)] : []
    pageSize.value = [20, 50, 100].includes(Number(state.pageSize)) ? Number(state.pageSize) : 20
    pageNum.value = Math.max(1, Number(state.page) || 1)
  } finally {
    restoringQuery = false
  }
  return true
}

async function search() {
  if (!datasourceId.value) return ElMessage.warning(mt('configureJaegerFirst'))
  loading.value = true
  try {
    if (traceId.value.trim()) {
      router.push({ path: `/monitor/traces/${encodeURIComponent(traceId.value.trim())}`, query: detailRouteQuery() })
      return
    }
    await queryList()
  } finally {
    loading.value = false
  }
}

function openDetail(trace) {
  router.push({ path: `/monitor/traces/${encodeURIComponent(trace.traceID)}`, query: detailRouteQuery() })
}

watch(datasourceId, () => { if (!restoringQuery) loadServices() })
watch(service, () => { if (!restoringQuery) loadOperations() })
onMounted(async () => {
  await loadDatasources()
  if (await restoreQueryState()) {
    loading.value = true
    try { await queryList(queryWindowFromRoute(), true) } finally { loading.value = false }
  }
})
</script>

<template>
  <div class="trace-page trace-query-page">
    <div class="page-header"><div><h2>{{ mt('traceSearch') }}</h2><p>{{ mt('traceSearchDesc') }}</p></div></div>
    <el-alert v-if="!datasources.length" :title="mt('noJaegerDatasource')" type="warning" :closable="false" show-icon>
      {{ mt('noJaegerDatasourceDesc') }}
    </el-alert>
    <div class="toolbar">
      <el-select v-model="datasourceId" filterable :placeholder="mt('selectJaegerDatasource')" style="width: 220px"><el-option v-for="item in datasources" :key="item.id" :label="item.name" :value="item.id" /></el-select>
      <el-select v-model="service" filterable clearable :loading="serviceLoading" :placeholder="mt('selectService')" style="width: 220px"><el-option v-for="item in services" :key="item" :label="item" :value="item" /></el-select>
      <el-select v-model="operation" filterable clearable :loading="operationLoading" :disabled="!service" :placeholder="mt('selectOperationOptional')" style="width: 230px"><el-option v-for="item in operations" :key="item" :label="item" :value="item" /></el-select>
      <el-select v-model="timeRange" style="width: 130px"><el-option :label="mt('last15Minutes')" value="15m" /><el-option :label="mt('lastHour')" value="1h" /><el-option :label="mt('last6Hours')" value="6h" /><el-option :label="mt('last24Hours')" value="24h" /><el-option :label="mt('last7Days')" value="7d" /><el-option :label="mt('customTime')" value="custom" /></el-select>
      <el-date-picker v-if="timeRange === 'custom'" v-model="customDateRange" type="datetimerange" value-format="x" :start-placeholder="mt('startTime')" :end-placeholder="mt('endTime')" :range-separator="mt('to')" style="width: 360px" />
      <el-input v-model="tags" clearable :placeholder="mt('tagJsonExample')" style="width: 300px" @keyup.enter="search" />
      <el-input v-model="traceId" clearable :placeholder="mt('traceIdDirect')" style="width: 280px" @keyup.enter="search" />
      <el-button type="primary" :loading="loading" @click="search">{{ mt('query') }}</el-button>
    </div>
    <div class="hint">{{ mt('currentDatasourceHint', { name: activeDatasource?.name || '-' }) }}</div>
    <el-table v-loading="loading" :data="pagedItems" border :empty-text="mt('selectServiceOrTrace')">
      <el-table-column label="Trace ID" min-width="270"><template #default="{ row }"><el-link type="primary" @click="openDetail(row)">{{ row.traceID }}</el-link></template></el-table-column>
      <el-table-column :label="mt('servicePath')" min-width="260" show-overflow-tooltip><template #default="{ row }">{{ traceServices(row) }}</template></el-table-column>
      <el-table-column label="Operation" min-width="240" show-overflow-tooltip><template #default="{ row }">{{ traceOperation(row) }}</template></el-table-column>
      <el-table-column :label="mt('status')" width="90" align="center"><template #default="{ row }"><el-tag size="small" effect="light" :type="traceStatusTagType(row)">{{ traceStatus(row) }}</el-tag></template></el-table-column>
      <el-table-column :label="mt('startedAt')" width="190"><template #default="{ row }">{{ formatTime(row.spans?.[0]?.startTime) }}</template></el-table-column>
      <el-table-column :label="mt('totalDuration')" width="120"><template #default="{ row }">{{ traceDuration(row) }}</template></el-table-column>
      <el-table-column :label="mt('spanCount')" width="110" align="center" sortable :sort-method="(left, right) => (left.spans?.length || 0) - (right.spans?.length || 0)"><template #default="{ row }">{{ row.spans?.length || 0 }}</template></el-table-column>
      <el-table-column :label="mt('actions')" width="90"><template #default="{ row }"><el-button link type="primary" @click="openDetail(row)">{{ mt('detail') }}</el-button></template></el-table-column>
    </el-table>
    <div class="trace-pagination"><span>{{ mt('pageSize', { count: pageSize }) }}</span><el-pagination v-model:current-page="pageNum" v-model:page-size="pageSize" :page-sizes="[20, 50, 100]" :total="items.length" layout="total, sizes, prev, pager, next, jumper" @size-change="pageNum = 1" /></div>
  </div>
</template>

<style scoped>
.trace-page { display: flex; flex-direction: column; gap: 18px; padding: 24px; background: #fff; border-radius: 18px; box-shadow: 0 12px 30px rgba(36, 54, 90, .08); }
.page-header h2 { margin: 0 0 8px; font-size: 26px; color: #10213f; }.page-header p, .hint { margin: 0; color: #7282a0; }.toolbar { display: flex; flex-wrap: wrap; gap: 12px; }.hint { font-size: 13px; }.trace-pagination { display: flex; align-items: center; justify-content: flex-end; gap: 14px; padding: 14px 2px 0; color: #7b8ca5; font-size: 12px; } @media (max-width: 720px) { .trace-pagination { align-items: flex-end; flex-direction: column; } }
</style>
