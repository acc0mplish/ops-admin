<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { queryNotifySendLogList, retryNotifySendLog } from '../../api/ops'
import { nt } from '../../utils/notify-i18n'

const loading = ref(false)
const detailVisible = ref(false)
const rows = ref([])
const total = ref(0)
const current = ref({})
const timeRange = ref([])

const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', status: '', channelType: '', scope: '', startTime: '', endTime: '' })

const statusMap = {
  pending: { key: 'pending', type: 'info' },
  sending: { key: 'sending', type: 'primary' },
  retrying: { key: 'retrying', type: 'warning' },
  success: { key: 'success', type: 'success' },
  failed: { key: 'failed', type: 'danger' }
}

const channelTypeKeys = { dingtalk: 'dingtalk', wecom: 'wecom', feishu: 'feishu', webhook: 'webhook' }
const scopeKeys = { all: 'all', job: 'job', pipeline: 'pipeline', schedule: 'schedule', monitor: 'monitor' }
const eventKeys = { notify: 'notify', success: 'success', failed: 'failed', waiting_approval: 'waiting_approval', rejected: 'rejected', firing: 'firing', recovered: 'recovered' }

async function loadData() {
  loading.value = true
  try {
    query.startTime = timeRange.value?.[0] || ''
    query.endTime = timeRange.value?.[1] || ''
    const data = await queryNotifySendLogList(query)
    rows.value = data.list || []
    total.value = data.total || 0
  } finally { loading.value = false }
}

function resetQuery() {
  Object.assign(query, { pageNum: 1, keyword: '', status: '', channelType: '', scope: '', startTime: '', endTime: '' })
  timeRange.value = []
  loadData()
}

function openDetail(row) { current.value = row; detailVisible.value = true }

async function handleRetry(row) {
  await ElMessageBox.confirm(nt('resendConfirm', { id: row.deliveryId }), nt('resendTitle'), { type: 'warning', confirmButtonText: nt('confirmResend'), cancelButtonText: nt('cancel') })
  const result = await retryNotifySendLog(row.id)
  ElMessage.success(nt('resendCreated', { id: result?.deliveryId || '' }))
  await loadData()
}

function statusInfo(status) {
  const item = statusMap[status]
  return item ? { label: nt(item.key), type: item.type } : { label: status || '-', type: 'info' }
}
function channelLabel(type) { return channelTypeKeys[type] ? nt(channelTypeKeys[type]) : type }
function scopeLabel(scope) { return scopeKeys[scope] ? nt(scopeKeys[scope]) : scope }
function eventLabel(event) { return eventKeys[event] ? nt(eventKeys[event]) : event }
function formatTime(value) { return value ? String(value).replace('T', ' ').replace(/\.\d+(?=[+Z])/, '').replace('+08:00', '') : '-' }

onMounted(loadData)
</script>

<template>
  <div class="page-card notify-page">
    <div class="page-header">
      <div><h2 class="page-title">{{ nt('sendLogs') }}</h2><p class="page-desc">{{ nt('sendLogsDesc') }}</p></div>
      <el-button @click="loadData">{{ nt('refresh') }}</el-button>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" clearable :placeholder="nt('keywordPlaceholder')" style="width:310px" @keyup.enter="loadData" />
      <el-select v-model="query.status" clearable :placeholder="nt('deliveryStatus')" style="width:140px"><el-option v-for="(item, key) in statusMap" :key="key" :label="nt(item.key)" :value="key" /></el-select>
      <el-select v-model="query.channelType" clearable :placeholder="nt('channelType')" style="width:150px"><el-option v-for="(value, key) in channelTypeKeys" :key="key" :label="nt(value)" :value="key" /></el-select>
      <el-select v-model="query.scope" clearable :placeholder="nt('businessScope')" style="width:140px"><el-option v-for="(value, key) in scopeKeys" :key="key" :label="nt(value)" :value="key" /></el-select>
      <el-date-picker v-model="timeRange" type="datetimerange" value-format="YYYY-MM-DD HH:mm:ss" :start-placeholder="nt('startTime')" :end-placeholder="nt('endTime')" :range-separator="nt('to')" style="width:360px" />
      <el-button type="primary" @click="loadData">{{ nt('search') }}</el-button><el-button @click="resetQuery">{{ nt('reset') }}</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="deliveryId" :label="nt('deliveryId')" min-width="210" show-overflow-tooltip />
      <el-table-column :label="nt('route')" min-width="210"><template #default="{ row }"><div class="primary-cell">{{ row.ruleName || '-' }}</div><div class="secondary-cell">{{ row.channelName }} · {{ channelLabel(row.channelType) }}</div></template></el-table-column>
      <el-table-column :label="nt('businessEvent')" min-width="190"><template #default="{ row }"><div class="primary-cell">{{ scopeLabel(row.scope) }} / {{ eventLabel(row.event) }}</div><div class="secondary-cell ellipsis">{{ row.targetName || '-' }}</div></template></el-table-column>
      <el-table-column prop="summary" :label="nt('summary')" min-width="200" show-overflow-tooltip />
      <el-table-column :label="nt('status')" width="105" align="center"><template #default="{ row }"><el-tag :type="statusInfo(row.status).type">{{ statusInfo(row.status).label }}</el-tag></template></el-table-column>
      <el-table-column :label="nt('attempts')" width="100" align="center"><template #default="{ row }">{{ row.attemptCount || 0 }} / {{ row.maxAttempts || 3 }}</template></el-table-column>
      <el-table-column :label="nt('response')" width="125"><template #default="{ row }"><div>HTTP {{ row.httpStatus || '-' }}</div><div class="secondary-cell">{{ nt('businessCode') }} {{ row.businessCode || '-' }}</div></template></el-table-column>
      <el-table-column :label="nt('duration')" width="95" align="right"><template #default="{ row }">{{ row.durationMs || 0 }} ms</template></el-table-column>
      <el-table-column :label="nt('createdAt')" width="175"><template #default="{ row }">{{ formatTime(row.createTime) }}</template></el-table-column>
      <el-table-column :label="nt('actions')" width="145" fixed="right"><template #default="{ row }"><el-button link type="primary" @click="openDetail(row)">{{ nt('detail') }}</el-button><el-button v-if="row.status === 'failed'" link type="warning" @click="handleRetry(row)">{{ nt('resend') }}</el-button></template></el-table-column>
    </el-table>

    <div class="pager"><el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" /></div>

    <el-dialog v-model="detailVisible" :title="nt('deliveryDetail')" width="960px">
      <div class="delivery-heading"><div><div class="delivery-id">{{ current.deliveryId }}</div><div class="secondary-cell">{{ current.ruleName }} → {{ current.channelName }}</div></div><el-tag :type="statusInfo(current.status).type" size="large">{{ statusInfo(current.status).label }}</el-tag></div>
      <el-descriptions :column="3" border>
        <el-descriptions-item :label="nt('businessScope')">{{ scopeLabel(current.scope) }}</el-descriptions-item><el-descriptions-item :label="nt('businessEvent')">{{ eventLabel(current.event) }}</el-descriptions-item><el-descriptions-item :label="nt('target')">{{ current.targetName || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="nt('attempts')">{{ current.attemptCount || 0 }} / {{ current.maxAttempts || 3 }}</el-descriptions-item><el-descriptions-item :label="nt('recentAttempt')">{{ formatTime(current.lastAttemptAt) }}</el-descriptions-item><el-descriptions-item :label="nt('nextRetry')">{{ formatTime(current.nextRetryAt) }}</el-descriptions-item>
        <el-descriptions-item :label="nt('httpStatus')">{{ current.httpStatus || '-' }}</el-descriptions-item><el-descriptions-item :label="nt('businessCode')">{{ current.businessCode || '-' }}</el-descriptions-item><el-descriptions-item :label="nt('duration')">{{ current.durationMs || 0 }} ms</el-descriptions-item>
        <el-descriptions-item :label="nt('originalDelivery')">{{ current.retryOfId || '-' }}</el-descriptions-item><el-descriptions-item :label="nt('createdAt')" :span="2">{{ formatTime(current.createTime) }}</el-descriptions-item>
      </el-descriptions>
      <el-alert v-if="current.errorText" class="error-alert" :title="current.errorText" type="error" :closable="false" show-icon />
      <div class="detail-grid"><div class="detail-block"><h3>{{ nt('requestBody') }}</h3><pre>{{ current.requestBody || '-' }}</pre></div><div class="detail-block"><h3>{{ nt('platformResponse') }}</h3><pre>{{ current.response || current.errorText || '-' }}</pre></div></div>
    </el-dialog>
  </div>
</template>

<style scoped>
.notify-page { display:flex; flex-direction:column; gap:18px; }.page-header { display:flex; justify-content:space-between; align-items:flex-start; gap:16px; }.page-title { margin:0 0 8px; font-size:24px; font-weight:700; color:#14213d; }.page-desc { margin:0; color:#7282a0; }.toolbar { display:flex; gap:12px; flex-wrap:wrap; }.pager { display:flex; justify-content:flex-end; }.primary-cell { color:#203656; font-weight:600; }.secondary-cell { margin-top:4px; color:#8492aa; font-size:12px; }.ellipsis { max-width:180px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }.delivery-heading { display:flex; align-items:center; justify-content:space-between; gap:16px; margin-bottom:16px; padding:14px 16px; border:1px solid #dce6f5; border-radius:6px; background:#f7faff; }.delivery-id { color:#14213d; font-family:Consolas,'Courier New',monospace; font-size:17px; font-weight:700; }.error-alert { margin-top:16px; }.detail-grid { display:grid; grid-template-columns:1fr 1fr; gap:14px; margin-top:16px; }.detail-block h3 { margin:0 0 8px; font-size:15px; color:#14213d; }pre { box-sizing:border-box; min-height:190px; max-height:360px; margin:0; padding:14px; overflow:auto; border-radius:6px; background:#0f172a; color:#e5edff; font-family:Consolas,'Courier New',monospace; white-space:pre-wrap; word-break:break-word; }@media (max-width:900px){.detail-grid{grid-template-columns:1fr}}
</style>
