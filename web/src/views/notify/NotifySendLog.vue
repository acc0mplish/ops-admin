<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { queryNotifySendLogList, retryNotifySendLog } from '../../api/ops'

const loading = ref(false)
const detailVisible = ref(false)
const rows = ref([])
const total = ref(0)
const current = ref({})
const timeRange = ref([])

const query = reactive({
  pageNum: 1,
  pageSize: 10,
  keyword: '',
  status: '',
  channelType: '',
  scope: '',
  startTime: '',
  endTime: ''
})

const statusMap = {
  pending: { label: '待发送', type: 'info' },
  sending: { label: '发送中', type: 'primary' },
  retrying: { label: '等待重试', type: 'warning' },
  success: { label: '成功', type: 'success' },
  failed: { label: '失败', type: 'danger' }
}

const channelTypeMap = {
  dingtalk: '钉钉',
  wecom: '企业微信',
  feishu: '飞书',
  webhook: '自定义 Webhook'
}

const scopeMap = { all: '全部', job: '作业编排', pipeline: 'CI/CD 流水线', schedule: '定时任务', monitor: '监控告警' }
const eventMap = {
  notify: '测试通知', success: '成功', failed: '失败', waiting_approval: '等待确认',
  rejected: '已拒绝', firing: '告警触发', recovered: '告警恢复'
}

async function loadData() {
  loading.value = true
  try {
    query.startTime = timeRange.value?.[0] || ''
    query.endTime = timeRange.value?.[1] || ''
    const data = await queryNotifySendLogList(query)
    rows.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

function resetQuery() {
  Object.assign(query, { pageNum: 1, keyword: '', status: '', channelType: '', scope: '', startTime: '', endTime: '' })
  timeRange.value = []
  loadData()
}

function openDetail(row) {
  current.value = row
  detailVisible.value = true
}

async function handleRetry(row) {
  await ElMessageBox.confirm(
    `将基于投递「${row.deliveryId}」创建一条新的发送任务，原日志不会被覆盖。是否继续？`,
    '重新发送',
    { type: 'warning', confirmButtonText: '确认重发', cancelButtonText: '取消' }
  )
  const result = await retryNotifySendLog(row.id)
  ElMessage.success(`已创建新投递 ${result?.deliveryId || ''}`)
  await loadData()
}

function statusInfo(status) {
  return statusMap[status] || { label: status || '-', type: 'info' }
}

function formatTime(value) {
  if (!value) return '-'
  return String(value).replace('T', ' ').replace(/\.\d+(?=[+Z])/, '').replace('+08:00', '')
}

onMounted(loadData)
</script>

<template>
  <div class="page-card notify-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">发送日志</h2>
        <p class="page-desc">追踪每次通知的投递状态、重试过程、平台响应与最终结果。</p>
      </div>
      <el-button @click="loadData">刷新</el-button>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" clearable placeholder="投递编号 / 规则 / 媒介 / 目标 / 摘要" style="width: 310px" @keyup.enter="loadData" />
      <el-select v-model="query.status" clearable placeholder="投递状态" style="width: 140px">
        <el-option v-for="(item, key) in statusMap" :key="key" :label="item.label" :value="key" />
      </el-select>
      <el-select v-model="query.channelType" clearable placeholder="媒介类型" style="width: 150px">
        <el-option v-for="(label, key) in channelTypeMap" :key="key" :label="label" :value="key" />
      </el-select>
      <el-select v-model="query.scope" clearable placeholder="业务场景" style="width: 140px">
        <el-option v-for="(label, key) in scopeMap" :key="key" :label="label" :value="key" />
      </el-select>
      <el-date-picker
        v-model="timeRange"
        type="datetimerange"
        value-format="YYYY-MM-DD HH:mm:ss"
        start-placeholder="开始时间"
        end-placeholder="结束时间"
        range-separator="至"
        style="width: 360px"
      />
      <el-button type="primary" @click="loadData">搜索</el-button>
      <el-button @click="resetQuery">重置</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="deliveryId" label="投递编号" min-width="210" show-overflow-tooltip />
      <el-table-column label="通知路由" min-width="210">
        <template #default="{ row }">
          <div class="primary-cell">{{ row.ruleName || '-' }}</div>
          <div class="secondary-cell">{{ row.channelName }} · {{ channelTypeMap[row.channelType] || row.channelType }}</div>
        </template>
      </el-table-column>
      <el-table-column label="业务事件" min-width="190">
        <template #default="{ row }">
          <div class="primary-cell">{{ scopeMap[row.scope] || row.scope }} / {{ eventMap[row.event] || row.event }}</div>
          <div class="secondary-cell ellipsis">{{ row.targetName || '-' }}</div>
        </template>
      </el-table-column>
      <el-table-column prop="summary" label="摘要" min-width="200" show-overflow-tooltip />
      <el-table-column label="状态" width="105" align="center">
        <template #default="{ row }">
          <el-tag :type="statusInfo(row.status).type">{{ statusInfo(row.status).label }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="尝试次数" width="100" align="center">
        <template #default="{ row }">{{ row.attemptCount || 0 }} / {{ row.maxAttempts || 3 }}</template>
      </el-table-column>
      <el-table-column label="响应" width="125">
        <template #default="{ row }">
          <div>HTTP {{ row.httpStatus || '-' }}</div>
          <div class="secondary-cell">业务码 {{ row.businessCode || '-' }}</div>
        </template>
      </el-table-column>
      <el-table-column label="耗时" width="95" align="right">
        <template #default="{ row }">{{ row.durationMs || 0 }} ms</template>
      </el-table-column>
      <el-table-column label="创建时间" width="175">
        <template #default="{ row }">{{ formatTime(row.createTime) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="145" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDetail(row)">详情</el-button>
          <el-button v-if="row.status === 'failed'" link type="warning" @click="handleRetry(row)">重新发送</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" />
    </div>

    <el-dialog v-model="detailVisible" title="投递详情" width="960px">
      <div class="delivery-heading">
        <div>
          <div class="delivery-id">{{ current.deliveryId }}</div>
          <div class="secondary-cell">{{ current.ruleName }} → {{ current.channelName }}</div>
        </div>
        <el-tag :type="statusInfo(current.status).type" size="large">{{ statusInfo(current.status).label }}</el-tag>
      </div>
      <el-descriptions :column="3" border>
        <el-descriptions-item label="业务场景">{{ scopeMap[current.scope] || current.scope }}</el-descriptions-item>
        <el-descriptions-item label="事件">{{ eventMap[current.event] || current.event }}</el-descriptions-item>
        <el-descriptions-item label="目标">{{ current.targetName || '-' }}</el-descriptions-item>
        <el-descriptions-item label="尝试次数">{{ current.attemptCount || 0 }} / {{ current.maxAttempts || 3 }}</el-descriptions-item>
        <el-descriptions-item label="最近尝试">{{ formatTime(current.lastAttemptAt) }}</el-descriptions-item>
        <el-descriptions-item label="下次重试">{{ formatTime(current.nextRetryAt) }}</el-descriptions-item>
        <el-descriptions-item label="HTTP 状态">{{ current.httpStatus || '-' }}</el-descriptions-item>
        <el-descriptions-item label="业务码">{{ current.businessCode || '-' }}</el-descriptions-item>
        <el-descriptions-item label="耗时">{{ current.durationMs || 0 }} ms</el-descriptions-item>
        <el-descriptions-item label="原投递记录">{{ current.retryOfId || '-' }}</el-descriptions-item>
        <el-descriptions-item label="创建时间" :span="2">{{ formatTime(current.createTime) }}</el-descriptions-item>
      </el-descriptions>
      <el-alert v-if="current.errorText" class="error-alert" :title="current.errorText" type="error" :closable="false" show-icon />
      <div class="detail-grid">
        <div class="detail-block">
          <h3>请求体</h3>
          <pre>{{ current.requestBody || '-' }}</pre>
        </div>
        <div class="detail-block">
          <h3>平台响应</h3>
          <pre>{{ current.response || current.errorText || '-' }}</pre>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<style scoped>
.notify-page { display: flex; flex-direction: column; gap: 18px; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
.page-title { margin: 0 0 8px; font-size: 24px; font-weight: 700; color: #14213d; }
.page-desc { margin: 0; color: #7282a0; }
.toolbar { display: flex; gap: 12px; flex-wrap: wrap; }
.pager { display: flex; justify-content: flex-end; }
.primary-cell { color: #203656; font-weight: 600; }
.secondary-cell { margin-top: 4px; color: #8492aa; font-size: 12px; }
.ellipsis { max-width: 180px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.delivery-heading { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 16px; padding: 14px 16px; border: 1px solid #dce6f5; border-radius: 6px; background: #f7faff; }
.delivery-id { color: #14213d; font-family: Consolas, 'Courier New', monospace; font-size: 17px; font-weight: 700; }
.error-alert { margin-top: 16px; }
.detail-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; margin-top: 16px; }
.detail-block h3 { margin: 0 0 8px; font-size: 15px; color: #14213d; }
pre { box-sizing: border-box; min-height: 190px; max-height: 360px; margin: 0; padding: 14px; overflow: auto; border-radius: 6px; background: #0f172a; color: #e5edff; font-family: Consolas, 'Courier New', monospace; white-space: pre-wrap; word-break: break-word; }
@media (max-width: 900px) { .detail-grid { grid-template-columns: 1fr; } }
</style>
