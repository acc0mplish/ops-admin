<script setup>
import { onMounted, reactive, ref } from 'vue'
import { queryNotifySendLogList } from '../../api/ops'

const loading = ref(false)
const detailVisible = ref(false)
const rows = ref([])
const total = ref(0)
const current = ref({})

const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', status: '' })

async function loadData() {
  loading.value = true
  try {
    const data = await queryNotifySendLogList(query)
    rows.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

function openDetail(row) {
  current.value = row
  detailVisible.value = true
}

onMounted(loadData)
</script>

<template>
  <div class="page-card notify-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">发送日志</h2>
        <p class="page-desc">记录通知规则每次 Webhook 发送结果，方便排查机器人地址、签名和模板问题。</p>
      </div>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" clearable placeholder="搜索规则 / 媒介 / 目标" style="width: 280px" @keyup.enter="loadData" />
      <el-select v-model="query.status" clearable placeholder="状态" style="width: 120px">
        <el-option label="成功" value="success" />
        <el-option label="失败" value="failed" />
      </el-select>
      <el-button type="primary" @click="loadData">搜索</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="ruleName" label="规则" min-width="160" />
      <el-table-column prop="channelName" label="媒介" min-width="160" />
      <el-table-column prop="channelType" label="类型" width="120" />
      <el-table-column prop="scope" label="场景" width="100" />
      <el-table-column prop="event" label="事件" width="130" />
      <el-table-column prop="targetName" label="目标" min-width="180" show-overflow-tooltip />
      <el-table-column label="状态" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="row.status === 'success' ? 'success' : 'danger'">{{ row.status === 'success' ? '成功' : '失败' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="createTime" label="发送时间" width="180" />
      <el-table-column label="操作" width="100" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDetail(row)">详情</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" />
    </div>

    <el-dialog v-model="detailVisible" title="发送详情" width="860px">
      <el-descriptions :column="2" border>
        <el-descriptions-item label="规则">{{ current.ruleName }}</el-descriptions-item>
        <el-descriptions-item label="媒介">{{ current.channelName }}</el-descriptions-item>
        <el-descriptions-item label="事件">{{ current.event }}</el-descriptions-item>
        <el-descriptions-item label="状态">{{ current.status }}</el-descriptions-item>
        <el-descriptions-item label="目标">{{ current.targetName }}</el-descriptions-item>
        <el-descriptions-item label="时间">{{ current.createTime }}</el-descriptions-item>
      </el-descriptions>
      <div class="detail-block">
        <h3>请求体</h3>
        <pre>{{ current.requestBody }}</pre>
      </div>
      <div class="detail-block">
        <h3>响应</h3>
        <pre>{{ current.response || current.errorText || '-' }}</pre>
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
.detail-block { margin-top: 16px; }
.detail-block h3 { margin: 0 0 8px; font-size: 15px; color: #14213d; }
pre { margin: 0; padding: 12px; min-height: 96px; border-radius: 8px; background: #0f172a; color: #e5edff; white-space: pre-wrap; word-break: break-word; }
</style>
