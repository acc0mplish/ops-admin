<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { claimMonitorAlertEvent, queryMonitorAlertEventList, resolveMonitorAlertEvent, triggerMonitorAlertAction } from '../../api/monitor'
import { queryOpsJobList, queryOpsScriptList } from '../../api/ops'

const loading = ref(false)
const rows = ref([])
const total = ref(0)
const jobs = ref([])
const scripts = ref([])
const actionVisible = ref(false)
const current = ref({})
const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', status: '', severity: '' })
const actionForm = reactive({ actionType: 'job', targetId: undefined, operator: '系统管理员', summary: '' })

function statusType(status) {
  if (status === 'firing') return 'danger'
  if (status === 'claimed') return 'warning'
  if (status === 'silenced') return 'info'
  if (status === 'recovered') return 'success'
  return 'info'
}

function statusText(status) {
  return ({ firing: '触发中', claimed: '已认领', silenced: '已屏蔽', recovered: '已恢复', resolved: '已关闭' }[status] || status)
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

async function loadActionOptions() {
  const [jobData, scriptData] = await Promise.all([
    queryOpsJobList({ pageNum: 1, pageSize: 100, status: 1 }),
    queryOpsScriptList({ pageNum: 1, pageSize: 100, status: 1 })
  ])
  jobs.value = jobData.list || []
  scripts.value = scriptData.list || []
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

async function openAction(row) {
  current.value = row
  Object.assign(actionForm, { actionType: 'job', targetId: undefined, operator: '系统管理员', summary: `处理告警：${row.ruleName}` })
  await loadActionOptions()
  actionVisible.value = true
}

async function submitAction() {
  const source = actionForm.actionType === 'job' ? jobs.value : scripts.value
  const target = source.find((item) => item.id === actionForm.targetId)
  await triggerMonitorAlertAction({
    id: current.value.id,
    actionType: actionForm.actionType,
    targetId: actionForm.targetId,
    targetName: target?.name || '',
    operator: actionForm.operator,
    summary: actionForm.summary
  })
  ElMessage.success(actionForm.actionType === 'job' ? '已触发作业' : '已记录诊断脚本处置')
  actionVisible.value = false
  await loadData()
}

onMounted(loadData)
</script>

<template>
  <div class="monitor-page">
    <div class="page-header">
      <div>
        <h2>告警事件</h2>
        <p>集中查看触发、认领、屏蔽、恢复和关闭状态，并可直接触发诊断脚本或作业。</p>
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
        <template #default="{ row }"><el-tag :type="statusType(row.status)">{{ statusText(row.status) }}</el-tag></template>
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
      <el-table-column label="操作" width="240" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openAction(row)">联动处置</el-button>
          <el-button v-if="row.status === 'firing'" link type="primary" @click="handleClaim(row)">认领</el-button>
          <el-button v-if="row.status === 'firing' || row.status === 'claimed' || row.status === 'silenced'" link type="warning" @click="handleResolve(row)">关闭</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" />
    </div>

    <el-dialog v-model="actionVisible" title="告警联动处置" width="620px">
      <el-form :model="actionForm" label-width="100px">
        <el-form-item label="告警规则">
          <el-input :model-value="current.ruleName" disabled />
        </el-form-item>
        <el-form-item label="处置类型">
          <el-radio-group v-model="actionForm.actionType">
            <el-radio label="job">触发作业</el-radio>
            <el-radio label="script">诊断脚本</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="actionForm.actionType === 'job' ? '选择作业' : '选择脚本'" required>
          <el-select v-model="actionForm.targetId" filterable clearable style="width: 100%">
            <el-option
              v-for="item in actionForm.actionType === 'job' ? jobs : scripts"
              :key="item.id"
              :label="item.name"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="处理人">
          <el-input v-model="actionForm.operator" />
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model="actionForm.summary" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="actionVisible = false">取消</el-button>
        <el-button type="primary" @click="submitAction">确认处置</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.monitor-page {
  display: flex;
  flex-direction: column;
  gap: 18px;
  padding: 24px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 12px 30px rgba(36, 54, 90, 0.08);
}
.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}
.page-header h2 {
  margin: 0 0 8px;
  font-size: 26px;
  color: #10213f;
}
.page-header p {
  margin: 0;
  color: #7282a0;
}
.toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}
.pager {
  display: flex;
  justify-content: flex-end;
}
</style>
