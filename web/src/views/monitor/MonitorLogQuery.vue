<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  deleteMonitorLogShortcut,
  queryMonitorDatasourceOptions,
  queryMonitorElasticsearchIndices,
  queryMonitorLogFieldValues,
  queryMonitorLogShortcuts,
  queryMonitorLogs,
  queryMonitorVictoriaLogsStreams,
  saveMonitorLogShortcut
} from '../../api/monitor'

const loading = ref(false)
const datasources = ref([])
const datasourceId = ref()
const indexOptions = ref([])
const indexLoading = ref(false)
const index = ref('_all')
const keyword = ref('')
const fieldKeyword = ref('')
const fieldsCollapsed = ref(false)
const fieldValueVisible = ref(false)
const fieldValueLoading = ref(false)
const selectedField = ref('')
const selectedFieldQuery = ref('')
const fieldValues = ref([])
const fieldValueKeyword = ref('')
const timeRange = ref('1h')
const customDateRange = ref([])
const streamOptions = ref([])
const streamLoading = ref(false)
const selectedTopics = ref([])
const opLogFieldDrilldownEnabled = ref(false)
const items = ref([])
const histogram = ref([])
const total = ref(0)
const took = ref(0)
const activeLog = ref(null)
const detailVisible = ref(false)
const autoRefresh = ref(false)
const shortcuts = ref([])
const shortcutDialogVisible = ref(false)
const shortcutSaving = ref(false)
const shortcutForm = reactive({ id: undefined, name: '', query: '', indexName: '_all', timeRange: '1h', sort: 0 })
let refreshTimer

const elasticsearchFields = ['kubernetes.pod_namespace', 'kubernetes.pod_name', 'kubernetes.container_name', 'kafka_topic', 'level']
const victoriaLogsFields = ['kubernetes.pod_namespace', 'kubernetes.pod_name', 'kubernetes.container_name', 'kafka_topic', 'level']
const activeDatasource = computed(() => datasources.value.find((item) => item.id === datasourceId.value))
const isVictoriaLogs = computed(() => activeDatasource.value?.type === 'victorialogs')
const fallbackFields = computed(() => (isVictoriaLogs.value ? victoriaLogsFields : elasticsearchFields).map((name) => ({ name, type: '常用' })))
const opLogIgnoredFields = new Set(['@timestamp', 'kafka_topic', 'source_type', 'timestamp', 'ts'])
const hasSelectedOpLogStream = computed(() => isVictoriaLogs.value && selectedTopics.value.some((topic) => String(topic).toLowerCase().includes('op.log')))
const opLogFields = computed(() => {
  const counts = new Map()
  for (const item of items.value) {
    const source = item?.source
    if (!source || typeof source !== 'object') continue
    for (const [name, value] of Object.entries(source)) {
      if (opLogIgnoredFields.has(name) || value === null || value === undefined || typeof value === 'object') continue
      counts.set(name, (counts.get(name) || 0) + 1)
    }
  }
  return [...counts.entries()]
    .map(([name, hits]) => ({ name: `op.${name}`, queryName: name, type: 'OP 日志', hits }))
    .sort((left, right) => right.hits - left.hits || left.name.localeCompare(right.name))
})
const fields = computed(() => {
  const phrase = fieldKeyword.value.trim().toLowerCase()
  const base = opLogFieldDrilldownEnabled.value && hasSelectedOpLogStream.value
    ? [...fallbackFields.value, ...opLogFields.value.filter((item) => !fallbackFields.value.some((field) => field.name === item.name))]
    : fallbackFields.value
  return phrase ? base.filter((item) => item.name.toLowerCase().includes(phrase)) : base
})
const maxBucketCount = computed(() => Math.max(1, ...histogram.value.map((item) => Number(item.doc_count || 0))))

function rangeTimestamps() {
  if (timeRange.value === 'custom' && customDateRange.value?.length === 2) {
    const [start, end] = customDateRange.value
    const startAt = new Date(start).getTime()
    const endAt = new Date(end).getTime()
    if (Number.isFinite(startAt) && Number.isFinite(endAt) && startAt < endAt) return { startAt, endAt }
  }
  const endAt = Date.now()
  const hours = { '1h': 1, '6h': 6, '24h': 24, '3d': 72, '7d': 168 }[timeRange.value] || 1
  return { startAt: endAt - hours * 3600000, endAt }
}

function formatTime(value) {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN', { hour12: false })
}

function histogramTime(bucket) {
  return formatTime(bucket.key_as_string || bucket.key).slice(5, 16)
}

function levelClass(level) {
  const value = String(level || '').toUpperCase()
  if (value.includes('ERROR') || value.includes('FATAL')) return 'level-error'
  if (value.includes('WARN')) return 'level-warn'
  if (value.includes('DEBUG') || value.includes('TRACE')) return 'level-debug'
  return 'level-info'
}

function displayLogContent(row) {
  return row?.message || row?.messageRaw || '-'
}

function escapeLogsQLValue(value) {
  return `"${String(value).replaceAll('\\', '\\\\').replaceAll('"', '\\"')}"`
}

function effectiveQuery() {
  const baseQuery = keyword.value.trim()
  if (!isVictoriaLogs.value || !selectedTopics.value.length) return baseQuery
  const topicQuery = selectedTopics.value.map((topic) => `kafka_topic:${escapeLogsQLValue(topic)}`).join(' OR ')
  return baseQuery ? `(${baseQuery}) AND (${topicQuery})` : `(${topicQuery})`
}

async function loadDatasources() {
  const options = await queryMonitorDatasourceOptions()
  datasources.value = (options || []).filter((item) => ['elasticsearch', 'victorialogs'].includes(item.type))
  datasourceId.value = datasources.value.find((item) => item.isDefault)?.id || datasources.value[0]?.id
}

async function loadShortcuts() {
  shortcuts.value = await queryMonitorLogShortcuts({ datasourceType: isVictoriaLogs.value ? 'victorialogs' : 'elasticsearch' })
}

async function loadIndices() {
  indexOptions.value = []
  if (!datasourceId.value || isVictoriaLogs.value) return
  indexLoading.value = true
  try {
    indexOptions.value = await queryMonitorElasticsearchIndices({ datasourceId: datasourceId.value })
  } finally {
    indexLoading.value = false
  }
}

async function loadStreams() {
  streamOptions.value = []
  if (!datasourceId.value || !isVictoriaLogs.value) return
  streamLoading.value = true
  try {
    streamOptions.value = await queryMonitorVictoriaLogsStreams({
      datasourceId: datasourceId.value,
      field: 'kafka_topic',
      query: keyword.value.trim(),
      ...rangeTimestamps()
    })
  } finally {
    streamLoading.value = false
  }
}

async function search() {
  if (!datasourceId.value) return ElMessage.warning('请先配置并选择日志数据源')
  if (timeRange.value === 'custom' && customDateRange.value?.length !== 2) return ElMessage.warning('请选择完整的开始和结束日期时间')
  loading.value = true
  try {
    const range = rangeTimestamps()
    const data = await queryMonitorLogs({
      datasourceId: datasourceId.value,
      index: isVictoriaLogs.value ? '_all' : (index.value.trim() || '_all'),
      query: effectiveQuery(),
      pageSize: 100,
      ...range
    })
    items.value = data.items || []
    histogram.value = data.histogram || []
    total.value = Number(data.total?.value ?? data.total ?? 0)
    took.value = Number(data.took || 0)
  } finally {
    loading.value = false
  }
}

function drillDownOpLogFields() {
  if (!hasSelectedOpLogStream.value) return
  if (!items.value.length) {
    ElMessage.warning('请先查询包含 op.log 的日志，再进行字段下钻')
    return
  }
  opLogFieldDrilldownEnabled.value = true
  if (!opLogFields.value.length) {
    ElMessage.info('当前结果中未发现可用于筛选的 OP 日志字段')
    return
  }
  ElMessage.success(`已下钻 ${opLogFields.value.length} 个 OP 日志字段`)
}

function showDetail(row) {
  activeLog.value = row
  detailVisible.value = true
}

function insertField(field) {
  keyword.value = keyword.value ? `${keyword.value} AND ${field}:` : `${field}:`
}

async function openFieldValues(field) {
  if (!datasourceId.value) return ElMessage.warning('请先选择日志数据源')
  selectedField.value = field.name || field
  selectedFieldQuery.value = field.queryName || field.name || field
  fieldValueKeyword.value = ''
  fieldValues.value = []
  fieldValueVisible.value = true
  fieldValueLoading.value = true
  try {
    fieldValues.value = await queryMonitorLogFieldValues({
      datasourceId: datasourceId.value,
      index: isVictoriaLogs.value ? '_all' : index.value,
      field: selectedFieldQuery.value,
      query: effectiveQuery(),
      ...rangeTimestamps()
    })
  } finally {
    fieldValueLoading.value = false
  }
}

function applyFieldValue(value) {
  const expression = `${selectedFieldQuery.value}:${escapeLogsQLValue(value)}`
  keyword.value = keyword.value ? `(${keyword.value}) AND ${expression}` : expression
  fieldValueVisible.value = false
  search()
}

function applyShortcut(item) {
  keyword.value = item.query || ''
  index.value = item.indexName || '_all'
  timeRange.value = item.timeRange || '1h'
  customDateRange.value = []
  search()
}

function openShortcutDialog(item) {
  Object.assign(shortcutForm, item
    ? { id: item.id, name: item.name, query: item.query, indexName: item.indexName || '_all', timeRange: item.timeRange || '1h', sort: item.sort || 0 }
    : { id: undefined, name: '', query: keyword.value, indexName: index.value || '_all', timeRange: timeRange.value || '1h', sort: shortcuts.value.length + 1 })
  shortcutDialogVisible.value = true
}

async function submitShortcut() {
  if (!shortcutForm.name.trim()) return ElMessage.warning('请填写快捷语句名称')
  shortcutSaving.value = true
  try {
    await saveMonitorLogShortcut({ ...shortcutForm, datasourceType: isVictoriaLogs.value ? 'victorialogs' : 'elasticsearch' })
    shortcutDialogVisible.value = false
    ElMessage.success('快捷语句已保存')
    await loadShortcuts()
  } finally {
    shortcutSaving.value = false
  }
}

async function removeShortcut(item) {
  await ElMessageBox.confirm(`确认删除快捷语句“${item.name}”吗？`, '删除确认', { type: 'warning' })
  await deleteMonitorLogShortcut(item.id)
  ElMessage.success('快捷语句已删除')
  await loadShortcuts()
}

function updateAutoRefresh(enabled) {
  window.clearInterval(refreshTimer)
  refreshTimer = enabled ? window.setInterval(search, 15000) : undefined
}

watch(autoRefresh, updateAutoRefresh)
watch(timeRange, async (value) => {
  if (value !== 'custom') customDateRange.value = []
  if (isVictoriaLogs.value) {
    await loadStreams()
  }
})
watch(selectedTopics, () => {
  if (!hasSelectedOpLogStream.value) opLogFieldDrilldownEnabled.value = false
  search()
})
watch(datasourceId, async () => {
  selectedTopics.value = []
  opLogFieldDrilldownEnabled.value = false
  index.value = '_all'
  await loadIndices()
  await loadStreams()
  await loadShortcuts()
  await search()
})

onMounted(async () => {
  await loadDatasources()
  await loadIndices()
  await loadStreams()
  await loadShortcuts()
  await search()
})

onBeforeUnmount(() => window.clearInterval(refreshTimer))
</script>

<template>
  <div class="log-page">
    <header class="log-header">
      <div>
        <div class="eyebrow">{{ isVictoriaLogs ? 'VICTORIALOGS LOG EXPLORER' : 'ELASTICSEARCH LOG EXPLORER' }}</div>
        <h2>日志查询</h2>
        <p>统一检索 Kubernetes、应用与中间件日志，支持 Elasticsearch 的 Lucene 和 VictoriaLogs 的 LogsQL。</p>
      </div>
      <div class="log-header-actions">
        <el-switch v-model="autoRefresh" active-text="自动刷新" />
        <el-button :loading="loading" @click="search">刷新</el-button>
      </div>
    </header>

    <section class="log-search-panel">
      <div class="source-line">
        <label>数据源</label>
        <el-select v-model="datasourceId" filterable placeholder="选择日志数据源" style="width: 290px">
          <el-option v-for="item in datasources" :key="item.id" :label="`${item.name} (${item.type} / ${item.env || '-'})`" :value="item.id" />
        </el-select>
        <template v-if="isVictoriaLogs">
          <label>Stream</label>
          <el-select v-model="selectedTopics" multiple collapse-tags collapse-tags-tooltip filterable clearable :loading="streamLoading" placeholder="选择 Topic" style="width: 340px">
            <el-option v-for="stream in streamOptions" :key="stream.value" :label="stream.value" :value="stream.value">
              <div class="stream-option"><span>{{ stream.value }}</span><small>{{ Number(stream.hits || 0).toLocaleString() }} 条</small></div>
            </el-option>
          </el-select>
        </template>
        <template v-if="!isVictoriaLogs">
          <label>索引</label>
          <el-select v-model="index" :loading="indexLoading" filterable allow-create default-first-option placeholder="选择或输入索引 / 通配符" style="width: 310px" @change="search">
            <el-option label="全部索引 (_all)" value="_all" />
            <el-option v-for="item in indexOptions" :key="item.name" :label="item.name" :value="item.name">
              <div class="index-option"><span>{{ item.name }}</span><small>{{ item.docsCount || 0 }} docs / {{ item.storeSize || '-' }}</small></div>
            </el-option>
          </el-select>
        </template>
        <el-tag v-else effect="plain" type="success">LogsQL</el-tag>
        <el-button v-if="hasSelectedOpLogStream" type="primary" plain @click="drillDownOpLogFields">OP 日志下钻</el-button>
        <span class="source-status">{{ activeDatasource?.url || '未选择数据源' }}</span>
      </div>
      <div class="query-line">
        <el-input v-model="keyword" class="query-input" clearable :placeholder="isVictoriaLogs ? '支持 LogsQL，例如 kubernetes.pod_namespace:default AND _msg:error' : '支持 Lucene query_string，例如 kubernetes.pod_namespace:default AND (ERROR OR WARN)'" @keyup.enter="search" />
        <el-select v-model="timeRange" style="width: 140px"><el-option label="最近 1 小时" value="1h" /><el-option label="最近 6 小时" value="6h" /><el-option label="最近 24 小时" value="24h" /><el-option label="最近 3 天" value="3d" /><el-option label="最近 7 天" value="7d" /><el-option label="指定日期" value="custom" /></el-select>
        <el-date-picker v-if="timeRange === 'custom'" v-model="customDateRange" type="datetimerange" value-format="YYYY-MM-DD HH:mm:ss" start-placeholder="开始日期时间" end-placeholder="结束日期时间" style="width: 360px" @change="search" />
        <el-button type="primary" :loading="loading" @click="search">搜索</el-button>
      </div>
      <div class="shortcut-line">
        <span>快捷语句</span>
        <el-button size="small" type="primary" plain @click="openShortcutDialog()">新增快捷语句</el-button>
        <el-dropdown v-for="item in shortcuts" :key="item.id" trigger="click">
          <el-button size="small" class="shortcut-button" @click="applyShortcut(item)">{{ item.name }}</el-button>
          <template #dropdown><el-dropdown-menu><el-dropdown-item @click="openShortcutDialog(item)">编辑</el-dropdown-item><el-dropdown-item divided @click="removeShortcut(item)">删除</el-dropdown-item></el-dropdown-menu></template>
        </el-dropdown>
      </div>
    </section>

    <section class="log-workspace" :class="{ 'fields-collapsed': fieldsCollapsed }">
      <aside class="field-panel">
        <div class="panel-title">
          <strong v-if="!fieldsCollapsed">字段列表</strong><span v-if="!fieldsCollapsed">{{ fields.length }}</span>
          <el-button link class="collapse-button" :title="fieldsCollapsed ? '展开字段列表' : '收起字段列表'" @click="fieldsCollapsed = !fieldsCollapsed">{{ fieldsCollapsed ? '»' : '«' }}</el-button>
        </div>
        <template v-if="!fieldsCollapsed">
          <el-input v-model="fieldKeyword" size="small" clearable placeholder="搜索字段" />
          <div class="field-list">
            <button v-for="field in fields" :key="field.name" class="field-item" :title="`查看 ${field.name} 的可选值`" @click="openFieldValues(field)">
              <span>{{ field.name }}</span><b>›</b>
            </button>
          </div>
        </template>
      </aside>
      <main class="log-main">
        <div class="result-summary"><strong>原始日志</strong><span>命中 {{ total }} 条</span><span>耗时 {{ took }} ms</span></div>
        <div v-if="histogram.length" class="histogram-panel"><div class="histogram"><div v-for="bucket in histogram" :key="bucket.key" class="histogram-bar-wrap" :title="`${histogramTime(bucket)} / ${bucket.doc_count}`"><div class="histogram-bar" :style="{ height: `${Math.max(2, Number(bucket.doc_count || 0) / maxBucketCount * 100)}%` }" /></div></div></div>
        <div class="log-table-wrap">
          <el-table v-loading="loading" :data="items" height="520" @row-click="showDetail">
            <el-table-column label="时间" width="148"><template #default="{ row }">{{ formatTime(row.timestamp) }}</template></el-table-column>
            <el-table-column label="级别" width="64"><template #default="{ row }"><span class="level" :class="levelClass(row.level)">{{ row.level || '-' }}</span></template></el-table-column>
            <el-table-column prop="namespace" label="命名空间" width="126" show-overflow-tooltip />
            <el-table-column prop="container" label="容器" width="108" show-overflow-tooltip />
            <el-table-column prop="message" label="日志内容" min-width="620" show-overflow-tooltip><template #default="{ row }"><span class="log-message">{{ displayLogContent(row) }}</span></template></el-table-column>
            <el-table-column label="操作" width="64" fixed="right"><template #default="{ row }"><el-button link type="primary" @click.stop="showDetail(row)">详情</el-button></template></el-table-column>
          </el-table>
        </div>
      </main>
    </section>

    <el-drawer v-model="detailVisible" title="日志详情" size="min(760px, 90vw)">
      <template v-if="activeLog">
        <div class="detail-meta"><span>{{ formatTime(activeLog.timestamp) }}</span><span>{{ activeLog.namespace || '-' }}</span><span>{{ activeLog.pod || '-' }}</span><span>{{ activeLog.container || '-' }}</span></div>
        <div class="message-fields"><div><small>日志时间</small><strong>{{ activeLog.logTime || formatTime(activeLog.timestamp) }}</strong></div><div><small>级别</small><strong :class="levelClass(activeLog.level)">{{ activeLog.level || '-' }}</strong></div><div><small>进程 ID</small><strong>{{ activeLog.processId || '-' }}</strong></div></div>
        <h4>日志正文</h4><pre class="detail-message">{{ displayLogContent(activeLog) }}</pre>
        <details v-if="activeLog.messageRaw && activeLog.messageRaw !== activeLog.message" class="raw-log-line"><summary>查看原始日志行</summary><pre>{{ activeLog.messageRaw }}</pre></details>
        <h4>原始文档</h4><pre class="detail-json">{{ JSON.stringify(activeLog.source || {}, null, 2) }}</pre>
      </template>
    </el-drawer>

    <el-dialog v-model="fieldValueVisible" :title="`筛选字段：${selectedField}`" width="min(640px, 92vw)" destroy-on-close>
      <p class="field-value-tip">展示当前时间范围、数据源和查询条件下可用的字段值。点击任意值即可追加筛选条件。</p>
      <el-input v-model="fieldValueKeyword" clearable placeholder="搜索字段值" />
      <div v-loading="fieldValueLoading" class="field-value-list">
        <el-empty v-if="!fieldValueLoading && !fieldValues.length" description="当前条件下没有可选值" :image-size="72" />
        <button v-for="item in fieldValues.filter((value) => !fieldValueKeyword || String(value.value).toLowerCase().includes(fieldValueKeyword.toLowerCase()))" :key="item.value" class="field-value-item" @click="applyFieldValue(item.value)">
          <span :title="item.value">{{ item.value }}</span>
          <small>{{ item.hits }} 条</small>
        </button>
      </div>
    </el-dialog>

    <el-dialog v-model="shortcutDialogVisible" :title="shortcutForm.id ? '编辑快捷语句' : '新增快捷语句'" width="min(620px, 92vw)">
      <el-form label-width="105px">
        <el-form-item label="名称" required><el-input v-model="shortcutForm.name" placeholder="例如：错误日志" /></el-form-item>
        <el-form-item label="查询语句"><el-input v-model="shortcutForm.query" type="textarea" :rows="3" :placeholder="isVictoriaLogs ? 'LogsQL，留空表示全部日志' : 'Lucene query_string，留空表示全部日志'" /></el-form-item>
        <el-form-item v-if="!isVictoriaLogs" label="索引"><el-input v-model="shortcutForm.indexName" placeholder="_all 或 logs-*" /></el-form-item>
        <el-form-item label="时间范围"><el-select v-model="shortcutForm.timeRange" style="width: 100%"><el-option label="最近 1 小时" value="1h" /><el-option label="最近 6 小时" value="6h" /><el-option label="最近 24 小时" value="24h" /><el-option label="最近 3 天" value="3d" /><el-option label="最近 7 天" value="7d" /></el-select></el-form-item>
      </el-form>
      <template #footer><el-button @click="shortcutDialogVisible = false">取消</el-button><el-button type="primary" :loading="shortcutSaving" @click="submitShortcut">保存</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.log-page{display:flex;flex-direction:column;gap:16px;min-height:calc(100vh - 115px);padding:24px;background:#f6f8fc;color:#24364e}.log-header,.log-search-panel,.log-workspace{border:1px solid #e0e8f4;border-radius:12px;background:#fff;box-shadow:0 8px 22px rgba(49,78,125,.06)}.log-header{display:flex;justify-content:space-between;gap:24px;padding:22px 24px}.eyebrow{color:#4a70d8;font-size:12px;font-weight:700;letter-spacing:1px}.log-header h2{margin:7px 0;color:#10213f;font-size:27px}.log-header p{margin:0;color:#7282a0}.log-header-actions,.source-line,.query-line,.shortcut-line{display:flex;align-items:center;gap:12px}.log-header-actions{flex-shrink:0}.log-search-panel{padding:16px}.query-line,.shortcut-line{margin-top:12px}.source-line label,.shortcut-line>span{color:#51627f;font-weight:600;white-space:nowrap}.source-status{overflow:hidden;color:#8491a7;font-size:12px;text-overflow:ellipsis;white-space:nowrap}.query-input{flex:1}.shortcut-line{flex-wrap:wrap}.shortcut-button{border-color:#d9e3f3;color:#4f6384;background:#f7f9fd}.index-option,.stream-option{display:flex;justify-content:space-between;gap:16px}.index-option small,.stream-option small{color:#8491a7}.stream-option{width:100%;min-width:0}.stream-option span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.log-workspace{display:grid;grid-template-columns:230px minmax(0,1fr);overflow:hidden}.field-panel{display:flex;flex-direction:column;gap:8px;padding:14px;border-right:1px solid #e5ebf4;background:#fbfcff}.panel-title{display:flex;justify-content:space-between;color:#263a59}.panel-title span{color:#8291aa}.field-item{display:flex;justify-content:space-between;width:100%;padding:9px 8px;border:0;border-radius:5px;color:#536784;background:transparent;text-align:left;cursor:pointer}.field-item:hover{color:#3567d6;background:#eef4ff}.log-main{min-width:0}.result-summary{display:flex;align-items:center;gap:18px;padding:13px 16px;border-bottom:1px solid #e5ebf4;color:#71809b;font-size:13px}.result-summary strong{color:#203759;font-size:15px}.histogram-panel{height:150px;padding:12px 16px;border-bottom:1px solid #e5ebf4}.histogram{display:flex;align-items:flex-end;gap:4px;height:100%}.histogram-bar-wrap{display:flex;flex:1;align-items:flex-end;height:100%;min-width:4px}.histogram-bar{width:100%;min-height:2px;border-radius:2px 2px 0 0;background:#5a6df1;opacity:.88}.log-message{font-family:Consolas,Monaco,monospace;color:#3b4e6b}.detail-meta{display:flex;flex-wrap:wrap;gap:8px;color:#73849a;font-size:13px}.detail-meta span{padding:4px 8px;border-radius:4px;background:#f2f5f9}.message-fields{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:10px;margin-top:16px}.message-fields>div{display:flex;flex-direction:column;gap:4px;min-width:0;padding:10px;border:1px solid #e5ebf3;border-radius:6px}.message-fields small{color:#8491a7}.message-fields strong{overflow:hidden;color:#24364e;text-overflow:ellipsis;white-space:nowrap}.detail-message,.detail-json{overflow:auto;padding:14px;border-radius:6px;color:#dce6f5;background:#111827;font:12px/1.65 Consolas,Monaco,monospace;white-space:pre-wrap;word-break:break-word}.raw-log-line{margin:14px 0;color:#60748d}.raw-log-line summary{cursor:pointer}.raw-log-line pre{overflow:auto;max-height:180px;padding:12px;border-radius:6px;color:#c9d4e3;background:#202936;font:12px/1.6 Consolas,Monaco,monospace;white-space:pre-wrap;word-break:break-word}.level{display:inline-flex;min-width:48px;justify-content:center;padding:3px 5px;border-radius:3px;font-size:11px;font-weight:700}.level-error{color:#d94c5b;background:#fff0f1}.level-warn{color:#bd7c11;background:#fff7e8}.level-info{color:#3978c6;background:#eef6ff}.level-debug{color:#6c7788;background:#f1f4f7}:deep(.el-table){--el-table-header-bg-color:#f6f8fc;--el-table-header-text-color:#71809b;--el-table-row-hover-bg-color:#f4f7fc;--el-table-border-color:#e7edf6}@media(max-width:1000px){.log-workspace{grid-template-columns:1fr}.field-panel{display:none}.source-line,.query-line{align-items:stretch;flex-wrap:wrap}.query-input{flex-basis:100%}.log-header{flex-direction:column}.message-fields{grid-template-columns:1fr}}
.log-workspace.fields-collapsed{grid-template-columns:44px minmax(0,1fr)}.field-panel{min-width:0}.panel-title{align-items:center;gap:4px}.collapse-button{margin-left:auto;font-size:18px}.field-list{display:flex;min-height:80px;flex-direction:column;gap:3px}.field-item{gap:6px;min-width:0}.field-item span{overflow:hidden;flex:1;text-overflow:ellipsis;white-space:nowrap}.field-item small{flex:0 0 auto;color:#8b98aa;font-size:10px}.field-item b{flex:0 0 auto;color:#5c75a1;font-weight:500}.fields-collapsed .field-panel{padding:14px 8px}.fields-collapsed .collapse-button{margin:auto}.field-value-tip{margin:0 0 12px;color:#7282a0;font-size:13px}.field-value-list{display:flex;flex-direction:column;gap:4px;min-height:100px;max-height:420px;margin-top:12px;overflow:auto}.field-value-item{display:flex;justify-content:space-between;gap:16px;width:100%;padding:10px 12px;border:1px solid #e3e9f4;border-radius:6px;background:#fff;color:#354864;text-align:left;cursor:pointer}.field-value-item:hover{border-color:#9db8f4;background:#f3f7ff;color:#3567d6}.field-value-item span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.field-value-item small{flex:0 0 auto;color:#8291aa}
</style>
