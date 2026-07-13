<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  batchUpdateMonitorSilenceRules,
  deleteMonitorSilenceRule,
  monitorSilenceRuleInfo,
  queryMonitorAlertRuleList,
  queryMonitorSilenceRuleList,
  saveMonitorSilenceRule
} from '../../api/monitor'

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const templateDialogVisible = ref(false)
const templateType = ref('metric')
const selectedRuleIds = ref([])
const rows = ref([])
const total = ref(0)
const ruleOptions = ref([])
const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', status: '' })
const timeRange = ref([])

const silenceTemplates = [
  { id: 'metric-maintenance', type: 'metric', name: '监控告警 - 维护窗口', matchMode: 'select', ruleIds: [], severity: '', matchersJson: '{}', description: '维护期间屏蔽全部监控告警，请设置屏蔽时间。' },
  { id: 'metric-node', type: 'metric', name: '监控告警 - 主机资源维护', matchMode: 'regex', ruleNamePattern: '^主机.*(CPU|内存|磁盘|负载).*', severity: '', matchersJson: '{}', description: '屏蔽主机资源维护相关监控告警。' },
  { id: 'metric-k8s', type: 'metric', name: '监控告警 - Kubernetes 发布窗口', matchMode: 'regex', ruleNamePattern: '^(Kubernetes|Pod|Deployment|PVC).*', severity: '', matchersJson: '{}', description: '发布窗口内屏蔽 Kubernetes 工作负载告警。' },
  { id: 'metric-instance', type: 'metric', name: '监控告警 - 指定主机维护', matchMode: 'select', ruleIds: [], severity: '', matchersJson: '{"instance":"10.0.0.1"}', description: '替换 instance 后屏蔽指定主机的监控告警。' },
  { id: 'metric-low-priority', type: 'metric', name: '监控告警 - 低优先级静默', matchMode: 'select', ruleIds: [], severity: 'P3', matchersJson: '{}', description: '临时静默 P3 低优先级监控告警。' },
  { id: 'log-maintenance', type: 'log', name: '日志告警 - 维护窗口', matchMode: 'select', ruleIds: [], severity: '', matchersJson: '{"alert_type":"log"}', description: '维护期间屏蔽所有日志告警。' },
  { id: 'log-error', type: 'log', name: '日志告警 - ERROR 发布噪声', matchMode: 'regex', ruleNamePattern: '.*(ERROR|异常|失败).*', severity: '', matchersJson: '{"alert_type":"log"}', description: '发布期间屏蔽 ERROR 与异常类日志告警。' },
  { id: 'log-namespace', type: 'log', name: '日志告警 - 指定命名空间', matchMode: 'select', ruleIds: [], severity: '', matchersJson: '{"namespace":"default","alert_type":"log"}', description: '替换 namespace 后屏蔽指定命名空间日志告警。' },
  { id: 'log-service', type: 'log', name: '日志告警 - 指定服务', matchMode: 'select', ruleIds: [], severity: '', matchersJson: '{"service":"game","alert_type":"log"}', description: '替换 service 后屏蔽指定服务日志告警。' },
  { id: 'log-low-priority', type: 'log', name: '日志告警 - 低优先级静默', matchMode: 'select', ruleIds: [], severity: 'P3', matchersJson: '{"alert_type":"log"}', description: '临时静默 P3 低优先级日志告警。' }
]

const visibleSilenceTemplates = computed(() => silenceTemplates.filter((item) => item.type === templateType.value))

const form = reactive({
  id: undefined,
  name: '',
  matchMode: 'select',
  ruleIds: [],
  ruleNamePattern: '',
  severity: '',
  matchersJson: '{}',
  startsAt: 0,
  endsAt: 0,
  status: 1,
  description: ''
})

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    matchMode: 'select',
    ruleIds: [],
    ruleNamePattern: '',
    severity: '',
    matchersJson: '{}',
    startsAt: 0,
    endsAt: 0,
    status: 1,
    description: ''
  })
  timeRange.value = []
}

function toUnixSeconds(value) {
  return value ? Math.floor(new Date(value).getTime() / 1000) : 0
}

function normalizePayload() {
  const [start, end] = timeRange.value || []
  return {
    ...form,
    ruleIds: form.matchMode === 'select' ? form.ruleIds : [],
    ruleNamePattern: form.matchMode === 'regex' ? form.ruleNamePattern : '',
    startsAt: toUnixSeconds(start),
    endsAt: toUnixSeconds(end)
  }
}

function selectedRuleNames(ids = []) {
  if (!ids.length) return '全部规则'
  return ids
    .map((id) => ruleOptions.value.find((item) => Number(item.id) === Number(id))?.name || id)
    .join('、')
}

async function loadRuleOptions() {
  const data = await queryMonitorAlertRuleList({ pageNum: 1, pageSize: 1000 })
  ruleOptions.value = data.list || []
}

async function loadData() {
  loading.value = true
  try {
    const data = await queryMonitorSilenceRuleList(query)
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

function openTemplateDialog() {
  templateType.value = 'metric'
  templateDialogVisible.value = true
}

function applySilenceTemplate(template) {
  resetForm()
  Object.assign(form, {
    name: template.name,
    matchMode: template.matchMode,
    ruleIds: template.ruleIds || [],
    ruleNamePattern: template.ruleNamePattern || '',
    severity: template.severity || '',
    matchersJson: template.matchersJson,
    description: template.description
  })
  templateDialogVisible.value = false
  dialogVisible.value = true
}

function handleSelectionChange(selection) {
  selectedRuleIds.value = selection.map((item) => item.id)
}

async function handleBatchAction(action) {
  if (!selectedRuleIds.value.length) return
  const labels = { enable: '启用', disable: '禁用', delete: '删除' }
  await ElMessageBox.confirm(`确认批量${labels[action]}已选中的 ${selectedRuleIds.value.length} 条屏蔽规则吗？`, '批量操作确认', { type: action === 'delete' ? 'warning' : 'info' })
  await batchUpdateMonitorSilenceRules({ ids: selectedRuleIds.value, action })
  ElMessage.success(`已批量${labels[action]}`)
  selectedRuleIds.value = []
  await loadData()
}

async function openEdit(row) {
  isEdit.value = true
  const data = await monitorSilenceRuleInfo(row.id)
  Object.assign(form, {
    ...data,
    matchMode: data.matchMode || 'regex',
    ruleIds: data.ruleIds || [],
    matchersJson: data.matchersJson || '{}'
  })
  timeRange.value = data.startsAt && data.endsAt ? [data.startsAt, data.endsAt] : []
  dialogVisible.value = true
}

async function submit() {
  if (!form.name.trim()) {
    ElMessage.warning('请输入屏蔽规则名称')
    return
  }
  if (form.matchMode === 'regex' && !form.ruleNamePattern.trim()) {
    ElMessage.warning('请输入规则名正则')
    return
  }
  saving.value = true
  try {
    await saveMonitorSilenceRule(normalizePayload())
    ElMessage.success('保存成功')
    dialogVisible.value = false
    await loadData()
  } finally {
    saving.value = false
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`确认删除屏蔽规则「${row.name}」吗？`, '提示', { type: 'warning' })
  await deleteMonitorSilenceRule(row.id)
  ElMessage.success('删除成功')
  await loadData()
}

onMounted(async () => {
  await loadRuleOptions()
  await loadData()
})
</script>

<template>
  <div class="monitor-page">
    <div class="page-header">
      <div>
        <h2>告警屏蔽规则</h2>
        <p>按具体告警规则或规则名正则静默告警，命中后只记录事件，不发送通知。</p>
      </div>
      <div class="header-actions"><el-button @click="openTemplateDialog">导入常用模板</el-button><el-button type="primary" @click="openCreate">新增屏蔽</el-button></div>
    </div>

    <div class="toolbar">
      <el-input v-model="query.keyword" clearable placeholder="搜索名称 / 规则名" style="width: 260px" @keyup.enter="loadData" />
      <el-select v-model="query.status" clearable placeholder="状态" style="width: 120px">
        <el-option label="启用" value="1" />
        <el-option label="禁用" value="2" />
      </el-select>
      <el-button type="primary" @click="loadData">搜索</el-button>
    </div>

    <div v-if="selectedRuleIds.length" class="batch-toolbar">
      <span>已选择 <b>{{ selectedRuleIds.length }}</b> 条屏蔽规则</span>
      <el-button size="small" type="success" @click="handleBatchAction('enable')">批量启用</el-button>
      <el-button size="small" type="warning" @click="handleBatchAction('disable')">批量禁用</el-button>
      <el-button size="small" type="danger" plain @click="handleBatchAction('delete')">批量删除</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border @selection-change="handleSelectionChange">
      <el-table-column type="selection" width="52" fixed="left" />
      <el-table-column prop="name" label="名称" min-width="180" />
      <el-table-column label="规则匹配" min-width="240" show-overflow-tooltip>
        <template #default="{ row }">
          <span v-if="row.matchMode === 'select'">多选：{{ selectedRuleNames(row.ruleIds) }}</span>
          <span v-else>正则：{{ row.ruleNamePattern || '-' }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="severity" label="等级" width="90">
        <template #default="{ row }">{{ row.severity || '全部' }}</template>
      </el-table-column>
      <el-table-column prop="matchersJson" label="Label 匹配" min-width="220" show-overflow-tooltip />
      <el-table-column prop="startsAt" label="开始时间" width="180" />
      <el-table-column prop="endsAt" label="结束时间" width="180" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '启用' : '禁用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="150" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="danger" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" :total="total" layout="total, sizes, prev, pager, next" @current-change="loadData" @size-change="loadData" />
    </div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑屏蔽规则' : '新增屏蔽规则'" width="820px">
      <el-form label-width="120px">
        <el-form-item label="名称" required><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="规则匹配">
          <el-radio-group v-model="form.matchMode">
            <el-radio-button label="select">下拉多选规则</el-radio-button>
            <el-radio-button label="regex">正则匹配规则名</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="form.matchMode === 'select'" label="告警规则">
          <el-select v-model="form.ruleIds" multiple filterable clearable placeholder="为空表示全部告警规则" style="width: 100%">
            <el-option v-for="item in ruleOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item v-else label="规则名正则" required>
          <el-input v-model="form.ruleNamePattern" placeholder="例如：^skzy-sh.* 表示匹配 skzy-sh 开头的所有告警规则" />
        </el-form-item>
        <el-form-item label="等级">
          <el-select v-model="form.severity" clearable placeholder="全部等级" style="width: 100%">
            <el-option v-for="item in ['P0','P1','P2','P3']" :key="item" :label="item" :value="item" />
          </el-select>
        </el-form-item>
        <el-form-item label="Label 匹配">
          <el-input v-model="form.matchersJson" type="textarea" :rows="3" placeholder='例如：{"instance":"10.0.0.1","job":"node"}' />
        </el-form-item>
        <el-form-item label="屏蔽时间">
          <el-date-picker v-model="timeRange" type="datetimerange" start-placeholder="开始时间" end-placeholder="结束时间" value-format="YYYY-MM-DD HH:mm:ss" style="width: 100%" />
        </el-form-item>
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

    <el-dialog v-model="templateDialogVisible" title="导入常用屏蔽模板" width="780px">
      <div class="template-head"><span>模板会带入规则匹配、等级和 Label 匹配条件，请按实际环境修改。</span><el-radio-group v-model="templateType"><el-radio-button label="metric">监控告警模板</el-radio-button><el-radio-button label="log">日志告警模板</el-radio-button></el-radio-group></div>
      <div class="template-grid">
        <button v-for="item in visibleSilenceTemplates" :key="item.id" type="button" class="template-card" @click="applySilenceTemplate(item)">
          <el-tag :type="item.type === 'log' ? 'warning' : 'primary'" size="small">{{ item.type === 'log' ? '日志告警' : '监控告警' }}</el-tag>
          <strong>{{ item.name }}</strong><p>{{ item.description }}</p><code>{{ item.matchMode === 'regex' ? item.ruleNamePattern : item.matchersJson }}</code>
        </button>
      </div>
      <template #footer><el-button @click="templateDialogVisible = false">取消</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.monitor-page { display: flex; flex-direction: column; gap: 18px; padding: 24px; background: #fff; border-radius: 18px; box-shadow: 0 12px 30px rgba(36, 54, 90, 0.08); }
.page-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.header-actions { display: flex; gap: 10px; }
.page-header h2 { margin: 0 0 8px; font-size: 26px; color: #10213f; }
.page-header p { margin: 0; color: #7282a0; }
.toolbar { display: flex; flex-wrap: wrap; gap: 12px; }
.pager { display: flex; justify-content: flex-end; }
.batch-toolbar { display: flex; align-items: center; gap: 10px; padding: 10px 12px; border: 1px solid #d9e5fb; border-radius: 8px; background: #f5f8ff; color: #52637f; }
.batch-toolbar b { color: #4265d5; }
.template-head { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 16px; color: #7282a0; font-size: 13px; }
.template-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.template-card { min-height: 138px; padding: 14px; text-align: left; border: 1px solid #dbe5f4; border-radius: 8px; background: #fff; cursor: pointer; }
.template-card:hover { border-color: #5b72f2; box-shadow: 0 8px 18px rgba(65, 92, 201, .12); }
.template-card strong { display: block; margin-top: 10px; color: #1d3154; }
.template-card p { min-height: 35px; margin: 6px 0; color: #7282a0; font-size: 13px; line-height: 1.4; }
.template-card code { display: block; overflow: hidden; color: #4567c7; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
</style>
