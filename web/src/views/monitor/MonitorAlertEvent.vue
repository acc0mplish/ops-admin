<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { claimMonitorAlertEvent, queryMonitorAlertEventList, resolveMonitorAlertEvent } from '../../api/monitor'

const loading = ref(false)
const rows = ref([])
const total = ref(0)
const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', status: '', severity: '' })

function statusType(status) {
  if (status === 'firing') return 'danger'
  if (status === 'claimed') return 'warning'
  if (status === 'silenced') return 'info'
  if (status === 'recovered') return 'success'
  return 'info'
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryMonitorAlertEventList(query)
    rows.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

async function handleClaim(row) {
  const { value } = await ElMessageBox.prompt('请输入认领人或处理说明', '认领告警', { inputPlaceholder: '例如：张三 排查中' })
  await claimMonitorAlertEvent({ id: row.id, claimedBy: value, handleNote: value })
  ElMessage.success('已认领')
  await loadData()
}

async function handleResolve(row) {
  const { value } = await ElMessageBox.prompt('请输入处理说明', '关闭告警', { inputPlaceholder: '例如：服务已恢复' })
  await resolveMonitorAlertEvent({ id: row.id, handleNote: value })
  ElMessage.success('已关闭')
  await loadData()
}

onMounted(loadData)
</script>

<template>
  <div class="monitor-page">
    <div class="page-header">
      <div>
        <h2>告警事件</h2>
        <p>集中查看触发、认领、恢复和关闭状态，形成告警处理闭环。</p>
      </div>
      <el-button @click="loadData">刷新</el-button>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" clearable placeholder="搜索规则 / 指标 / 摘要" style="width: 280px" @keyup.enter="loadData" />
      <el-select v-model="query.status" clearable placeholder="状态" style="width: 140px">
        <el-option label="触发中" value="firing" />
        <el-option label="已认领" value="claimed" />
        <el-option label="已屏蔽" value="silenced" />
        <el-option label="已恢复" value="recovered" />
        <el-option label="已关闭" value="resolved" />
      </el-select>
      <el-select v-model="query.severity" clearable placeholder="等级" style="width: 120px">
        <el-option v-for="item in ['P0','P1','P2','P3']" :key="item" :label="item" :value="item" />
      </el-select>
      <el-button type="primary" @click="loadData">搜索</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="ruleName" label="规则" min-width="170" />
      <el-table-column prop="severity" label="等级" width="90" />
      <el-table-column label="状态" width="110">
        <template #default="{ row }"><el-tag :type="statusType(row.status)">{{ row.status }}</el-tag></template>
      </el-table-column>
      <el-table-column prop="metric" label="指标" min-width="170" show-overflow-tooltip />
      <el-table-column label="降噪命中" min-width="180" show-overflow-tooltip>
        <template #default="{ row }">
          <span v-if="row.silenced">屏蔽：{{ row.silenceRuleName || '-' }}</span>
          <span v-else-if="row.aggregateRuleName">聚合：{{ row.aggregateRuleName }}</span>
          <span v-else>-</span>
        </template>
      </el-table-column>
      <el-table-column label="当前值" width="120">
        <template #default="{ row }">{{ Number(row.currentValue || 0).toFixed(4) }}</template>
      </el-table-column>
      <el-table-column prop="threshold" label="阈值" width="100" />
      <el-table-column prop="summary" label="摘要" min-width="260" show-overflow-tooltip />
      <el-table-column prop="lastTriggerAt" label="最近触发" width="180" />
      <el-table-column label="操作" width="170" fixed="right">
        <template #default="{ row }">
          <el-button v-if="row.status === 'firing'" link type="primary" @click="handleClaim(row)">认领</el-button>
          <el-button v-if="row.status === 'firing' || row.status === 'claimed' || row.status === 'silenced'" link type="warning" @click="handleResolve(row)">关闭</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" />
    </div>
  </div>
</template>

<style scoped>
.monitor-page { display: flex; flex-direction: column; gap: 18px; padding: 24px; background: #fff; border-radius: 18px; box-shadow: 0 12px 30px rgba(36, 54, 90, 0.08); }
.page-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.page-header h2 { margin: 0 0 8px; font-size: 26px; color: #10213f; }
.page-header p { margin: 0; color: #7282a0; }
.toolbar { display: flex; flex-wrap: wrap; gap: 12px; }
.pager { display: flex; justify-content: flex-end; }
</style>
