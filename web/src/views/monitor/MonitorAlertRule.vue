<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { queryNotifyRuleOptions } from '../../api/ops'
import {
  deleteMonitorAlertRule,
  monitorAlertRuleInfo,
  queryMonitorAlertRuleList,
  queryMonitorDatasourceOptions,
  runMonitorAlertRule,
  saveMonitorAlertRule,
  updateMonitorAlertRuleStatus
} from '../../api/monitor'

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const rows = ref([])
const total = ref(0)
const datasourceOptions = ref([])
const notifyRuleOptions = ref([])
const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', status: '', severity: '' })
const form = reactive({
  id: undefined,
  name: '',
  datasourceId: undefined,
  promql: '',
  comparator: '>',
  threshold: 0,
  forSeconds: 0,
  evalIntervalSeconds: 60,
  severity: 'P2',
  labelsJson: '{}',
  annotationsJson: '{}',
  notifyEnabled: false,
  notifyRuleId: undefined,
  notifyRecoveryEnabled: true,
  status: 1,
  description: ''
})

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    datasourceId: datasourceOptions.value[0]?.id,
    promql: '',
    comparator: '>',
    threshold: 0,
    forSeconds: 0,
    evalIntervalSeconds: 60,
    severity: 'P2',
    labelsJson: '{}',
    annotationsJson: '{}',
    notifyEnabled: false,
    notifyRuleId: undefined,
    notifyRecoveryEnabled: true,
    status: 1,
    description: ''
  })
}

async function loadOptions() {
  const [datasources, notifyRules] = await Promise.all([
    queryMonitorDatasourceOptions(),
    queryNotifyRuleOptions({ scope: 'monitor' })
  ])
  datasourceOptions.value = datasources || []
  notifyRuleOptions.value = notifyRules || []
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryMonitorAlertRuleList(query)
    rows.value = data.list || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

function openCreate() {
  isEdit.value = false
  resetForm()
  dialogVisible.value = true
}

async function openEdit(row) {
  isEdit.value = true
  const data = await monitorAlertRuleInfo(row.id)
  Object.assign(form, {
    ...data,
    labelsJson: data.labelsJson || '{}',
    annotationsJson: data.annotationsJson || '{}'
  })
  dialogVisible.value = true
}

async function submit() {
  if (!form.name.trim() || !form.datasourceId || !form.promql.trim()) {
    ElMessage.warning('请填写规则名称、数据源和 PromQL')
    return
  }
  if (form.notifyEnabled && !form.notifyRuleId) {
    ElMessage.warning('请选择通知规则')
    return
  }
  saving.value = true
  try {
    await saveMonitorAlertRule(form)
    ElMessage.success('保存成功')
    dialogVisible.value = false
    await loadData()
  } finally {
    saving.value = false
  }
}

async function handleStatus(row) {
  await updateMonitorAlertRuleStatus({ id: row.id, status: row.status === 1 ? 2 : 1 })
  ElMessage.success('状态已更新')
  await loadData()
}

async function handleRun(row) {
  await runMonitorAlertRule(row.id)
  ElMessage.success('已触发一次规则评估')
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`确认删除告警规则「${row.name}」吗？`, '提示', { type: 'warning' })
  await deleteMonitorAlertRule(row.id)
  ElMessage.success('删除成功')
  await loadData()
}

onMounted(async () => {
  await loadOptions()
  await loadData()
})
</script>

<template>
  <div class="monitor-page">
    <div class="page-header">
      <div>
        <h2>告警规则</h2>
        <p>基于 PromQL 周期评估指标阈值，触发后可通过通知规则发送到对应媒介。</p>
      </div>
      <el-button type="primary" @click="openCreate">新增规则</el-button>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" clearable placeholder="搜索规则 / PromQL" style="width: 280px" @keyup.enter="loadData" />
      <el-select v-model="query.severity" clearable placeholder="等级" style="width: 120px">
        <el-option v-for="item in ['P0','P1','P2','P3']" :key="item" :label="item" :value="item" />
      </el-select>
      <el-select v-model="query.status" clearable placeholder="状态" style="width: 120px">
        <el-option label="启用" value="1" />
        <el-option label="禁用" value="2" />
      </el-select>
      <el-button type="primary" @click="loadData">搜索</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="name" label="规则名称" min-width="180" />
      <el-table-column prop="datasourceName" label="数据源" width="170" />
      <el-table-column prop="severity" label="等级" width="90" />
      <el-table-column label="条件" min-width="320" show-overflow-tooltip>
        <template #default="{ row }">{{ row.promql }} {{ row.comparator }} {{ row.threshold }}</template>
      </el-table-column>
      <el-table-column prop="evalIntervalSeconds" label="评估间隔" width="110">
        <template #default="{ row }">{{ row.evalIntervalSeconds }}s</template>
      </el-table-column>
      <el-table-column prop="lastEvalStatus" label="最近评估" width="120" />
      <el-table-column label="通知" width="90">
        <template #default="{ row }">
          <el-tag :type="row.notifyEnabled ? 'success' : 'info'">{{ row.notifyEnabled ? '启用' : '关闭' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '启用' : '禁用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="260" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="handleRun(row)">运行</el-button>
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="warning" @click="handleStatus(row)">{{ row.status === 1 ? '禁用' : '启用' }}</el-button>
          <el-button link type="danger" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" />
    </div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑告警规则' : '新增告警规则'" width="860px">
      <el-form label-width="120px">
        <el-form-item label="规则名称" required><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="数据源" required>
          <el-select v-model="form.datasourceId" filterable style="width: 100%">
            <el-option v-for="item in datasourceOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="PromQL" required><el-input v-model="form.promql" type="textarea" :rows="4" /></el-form-item>
        <el-row :gutter="12">
          <el-col :span="6">
            <el-form-item label="比较符"><el-select v-model="form.comparator"><el-option v-for="item in ['>','>=','<','<=','==','!=']" :key="item" :label="item" :value="item" /></el-select></el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="阈值"><el-input-number v-model="form.threshold" :precision="4" style="width: 100%" /></el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="持续时间"><el-input-number v-model="form.forSeconds" :min="0" :max="86400" style="width: 100%" /></el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="评估间隔"><el-input-number v-model="form.evalIntervalSeconds" :min="15" :max="3600" style="width: 100%" /></el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="等级">
          <el-radio-group v-model="form.severity">
            <el-radio-button v-for="item in ['P0','P1','P2','P3']" :key="item" :label="item" />
          </el-radio-group>
        </el-form-item>
        <el-form-item label="标签 JSON"><el-input v-model="form.labelsJson" type="textarea" :rows="2" /></el-form-item>
        <el-form-item label="注解 JSON"><el-input v-model="form.annotationsJson" type="textarea" :rows="2" /></el-form-item>
        <el-form-item label="消息通知"><el-switch v-model="form.notifyEnabled" /></el-form-item>
        <el-form-item v-if="form.notifyEnabled" label="通知规则" required>
          <el-select v-model="form.notifyRuleId" filterable style="width: 100%">
            <el-option v-for="item in notifyRuleOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="form.notifyEnabled" label="恢复通知"><el-switch v-model="form.notifyRecoveryEnabled" /></el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">启用</el-radio>
            <el-radio :value="2">禁用</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="描述"><el-input v-model="form.description" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submit">保存</el-button>
      </template>
    </el-dialog>
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
